// Admin portal API: dashboard overview, player administration (view, ban,
// unban, delete, rename, EXP adjustment, stat resets), full leaderboards,
// round listings and the event log. Everything is mounted under /admin and
// guarded by an email allowlist: admins sign in with Google like any player
// and their session token unlocks the admin API only while their Google
// email is on ADMIN_EMAILS. An empty allowlist disables the whole group
// (404) — fail closed.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/game"
	"github.com/pattaradol9/game24/server/internal/progress"
	"github.com/pattaradol9/game24/server/internal/store"
)

// --- middleware ---

// requireAdmin authenticates the player token, then checks the ADMIN_EMAILS
// allowlist. Banned admins lose access like anyone else. The acting admin is
// attached to the request context so mutations can audit who performed them.
func (a *API) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(a.Cfg.AdminEmails) == 0 {
			fail(w, http.StatusNotFound, "not found")
			return
		}
		p, err := a.playerFrom(r)
		if errors.Is(err, store.ErrBanned) {
			fail(w, http.StatusForbidden, "account banned")
			return
		}
		if err != nil {
			fail(w, http.StatusUnauthorized, "sign in required")
			return
		}
		if !a.isAdminPlayer(p) {
			fail(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithPlayer(r, p)))
	})
}

// actorOf returns the audit-log actor string for the acting admin, e.g.
// "admin:<player id>" (player ids are random hex — no PII in the log).
func actorOf(r *http.Request) string {
	if p, ok := playerOf(r); ok {
		return "admin:" + p.ID
	}
	return "admin"
}

func (a *API) adminRoutes(r chi.Router) {
	r.Get("/session", a.adminSession)
	r.Get("/overview", a.adminOverview)
	r.Get("/players", a.adminPlayers)
	r.Get("/players/{id}", a.adminPlayerDetail)
	r.Patch("/players/{id}", a.adminPlayerPatch)
	r.Delete("/players/{id}", a.adminPlayerDelete)
	r.Post("/players/{id}/exp", a.adminAdjustEXP)
	r.Post("/players/{id}/stats/reset", a.adminResetStats)
	r.Get("/players/{id}/rounds", a.adminPlayerRounds)
	r.Get("/leaderboard", a.adminLeaderboard)
	r.Get("/rounds", a.adminRounds)
	r.Get("/events", a.adminEvents)
	r.Get("/settings", a.adminSettings)
	r.Get("/db/backup", a.adminDbBackup)
	r.Post("/db/reset", a.adminDbReset)
}

// --- JSON mappers ---

type adminPlayerJSON struct {
	ID           string                  `json:"id"`
	Nickname     string                  `json:"nickname"`
	Email        string                  `json:"email,omitempty"`
	Picture      string                  `json:"picture,omitempty"`
	IsGuest      bool                    `json:"isGuest"`
	Banned       bool                    `json:"banned"`
	BanReason    string                  `json:"banReason,omitempty"`
	TotalExp     int64                   `json:"totalExp"`
	Level        int64                   `json:"level"`
	Tier         string                  `json:"tier"`
	HandsSolved  int64                   `json:"handsSolved"`
	HandsSkipped int64                   `json:"handsSkipped"`
	CreatedAt    string                  `json:"createdAt"`
	LastSeenAt   string                  `json:"lastSeenAt"`
	PerMode      map[string]modeStatJSON `json:"perMode,omitempty"`
}

func adminPlayerCore(p store.Player) adminPlayerJSON {
	lv, _, _ := progress.LevelProgress(p.TotalExp)
	return adminPlayerJSON{
		ID: p.ID, Nickname: p.Nickname, Email: p.Email, Picture: p.Picture,
		IsGuest: p.IsGuest, Banned: p.Banned, BanReason: p.BanReason,
		TotalExp: p.TotalExp, Level: lv, Tier: progress.TierName(progress.TierFromLevel(lv)),
		CreatedAt:  p.CreatedAt.UTC().Format(time.RFC3339),
		LastSeenAt: p.LastSeen.UTC().Format(time.RFC3339),
	}
}

func adminPlayerFull(p store.Player) adminPlayerJSON {
	out := adminPlayerCore(p)
	out.PerMode = map[string]modeStatJSON{}
	for mode, st := range p.Stats {
		out.PerMode[mode] = modeStatJSON{
			Exp: st.Exp, HandsSolved: st.HandsSolved, HandsSkipped: st.HandsSkipped,
			BestStreak: st.BestStreak, CurrentStreak: st.CurrentStreak,
		}
	}
	return out
}

func adminPlayerRowJSON(r store.AdminPlayerRow) adminPlayerJSON {
	lv, _, _ := progress.LevelProgress(r.TotalExp)
	return adminPlayerJSON{
		ID: r.ID, Nickname: r.Nickname, Email: r.Email, Picture: r.Picture,
		IsGuest: r.IsGuest, Banned: r.Banned, BanReason: r.BanReason,
		TotalExp: r.TotalExp, Level: lv, Tier: progress.TierName(progress.TierFromLevel(lv)),
		HandsSolved: r.HandsSolved, HandsSkipped: r.HandsSkipped,
		CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
		LastSeenAt: r.LastSeen.UTC().Format(time.RFC3339),
	}
}

type adminRoundJSON struct {
	ID         string `json:"id"`
	PlayerID   string `json:"playerId"`
	Nickname   string `json:"nickname"`
	Mode       string `json:"mode"`
	Status     string `json:"status"`
	Points     int64  `json:"points"`
	ElapsedMs  int64  `json:"elapsedMs"`
	HintsUsed  int64  `json:"hintsUsed"`
	DealtAt    string `json:"dealtAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
}

func adminRoundJSONOf(r store.AdminRoundRow) adminRoundJSON {
	out := adminRoundJSON{
		ID: r.ID, PlayerID: r.PlayerID, Nickname: r.Nickname, Mode: r.Mode,
		Status: r.Status, Points: r.Points, ElapsedMs: r.ElapsedMs, HintsUsed: r.HintsUsed,
		DealtAt: r.DealtAt.UTC().Format(time.RFC3339),
	}
	if r.FinishedAt != nil {
		out.FinishedAt = r.FinishedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// --- handlers ---

func (a *API) adminSession(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	ok(w, map[string]any{"ok": true, "admin": map[string]any{
		"playerId": p.ID, "nickname": p.Nickname,
	}})
}

func (a *API) adminOverview(w http.ResponseWriter, _ *http.Request) {
	o, err := a.Store.Overview()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	modes := make([]map[string]any, 0, len(o.Modes))
	for _, m := range o.Modes {
		modes = append(modes, map[string]any{
			"mode": m.Mode, "exp": m.Exp, "handsSolved": m.HandsSolved,
			"handsSkipped": m.HandsSkipped, "bestStreak": m.BestStreak,
		})
	}
	signups := make([]map[string]any, 0, len(o.Signups))
	for _, d := range o.Signups {
		signups = append(signups, map[string]any{"day": d.Day, "count": d.Count})
	}
	ok(w, map[string]any{
		"players":     o.Players,
		"rounds":      o.Rounds,
		"roundsTotal": o.RoundTotal,
		"totalExp":    o.TotalExp,
		"modes":       modes,
		"signups":     signups,
		"eventsTotal": o.Events,
	})
}

func (a *API) adminPlayers(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	filter := r.URL.Query().Get("filter")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, total, err := a.Store.AdminListPlayers(search, filter, limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	players := make([]adminPlayerJSON, 0, len(rows))
	for _, row := range rows {
		players = append(players, adminPlayerRowJSON(row))
	}
	ok(w, map[string]any{"players": players, "total": total})
}

func (a *API) adminPlayerDetail(w http.ResponseWriter, r *http.Request) {
	p, err := a.Store.PlayerByID(chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

// adminPlayerPatch applies partial updates: nickname (admin rename) and/or
// ban state. banned=true records an optional reason (max 200 chars).
func (a *API) adminPlayerPatch(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Nickname  *string `json:"nickname"`
		Banned    *bool   `json:"banned"`
		BanReason *string `json:"banReason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Nickname == nil && body.Banned == nil {
		fail(w, http.StatusBadRequest, "nothing to update")
		return
	}
	var current store.Player
	if body.Banned != nil {
		reason := ""
		if body.BanReason != nil {
			reason = sanitizeBanReason(*body.BanReason)
		}
		var err error
		if current, err = a.Store.SetPlayerBanned(actorOf(r), id, *body.Banned, reason); errors.Is(err, store.ErrNotFound) {
			fail(w, http.StatusNotFound, "player not found")
			return
		} else if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if body.Nickname != nil {
		name := sanitizeName(*body.Nickname)
		if name == "" {
			fail(w, http.StatusBadRequest, "nickname required")
			return
		}
		var err error
		if current, err = a.Store.RenamePlayer(actorOf(r), id, name); errors.Is(err, store.ErrNotFound) {
			fail(w, http.StatusNotFound, "player not found")
			return
		} else if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	ok(w, map[string]any{"player": adminPlayerFull(current)})
}

func sanitizeBanReason(s string) string {
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return -1
		}
		return r
	}, s))
}

func (a *API) adminPlayerDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeletePlayer(actorOf(r), chi.URLParam(r, "id")); errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	} else if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"deleted": true})
}

// adminAdjustEXP grants (positive) or revokes (negative) EXP for one mode;
// total EXP and the derived level always resync to the per-mode sum.
func (a *API) adminAdjustEXP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode  string `json:"mode"`
		Delta int64  `json:"delta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if _, err := game.Config(game.Mode(body.Mode)); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Delta == 0 {
		fail(w, http.StatusBadRequest, "delta must not be zero")
		return
	}
	p, err := a.Store.AdjustPlayerEXP(actorOf(r), chi.URLParam(r, "id"), body.Mode, body.Delta)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

// adminResetStats wipes stats for one mode (mode omitted = all modes).
func (a *API) adminResetStats(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	// an empty body is allowed and means "reset every mode"
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Mode != "" {
		if _, err := game.Config(game.Mode(body.Mode)); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	p, err := a.Store.ResetPlayerStats(actorOf(r), chi.URLParam(r, "id"), body.Mode)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player or mode stats not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

func (a *API) adminPlayerRounds(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, total, err := a.Store.AdminPlayerRounds(chi.URLParam(r, "id"), limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	rounds := make([]adminRoundJSON, 0, len(rows))
	for _, row := range rows {
		rounds = append(rounds, adminRoundJSONOf(row))
	}
	ok(w, map[string]any{"rounds": rounds, "total": total})
}

func (a *API) adminLeaderboard(w http.ResponseWriter, r *http.Request) {
	mode := game.Mode(r.URL.Query().Get("mode"))
	if _, err := game.Config(mode); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, err := a.Store.AdminLeaderboard(string(mode), limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	type rowJSON struct {
		Rank        int    `json:"rank"`
		PlayerID    string `json:"playerId"`
		Nickname    string `json:"nickname"`
		Exp         int64  `json:"exp"`
		Level       int64  `json:"level"`
		Tier        string `json:"tier"`
		HandsSolved int64  `json:"handsSolved"`
		BestStreak  int64  `json:"bestStreak"`
		IsGuest     bool   `json:"isGuest"`
		Banned      bool   `json:"banned"`
	}
	out := make([]rowJSON, 0, len(rows))
	for i, row := range rows {
		lv := progress.LevelFromExp(row.Exp)
		out = append(out, rowJSON{
			Rank: offset + i + 1, PlayerID: row.PlayerID, Nickname: row.Nickname,
			Exp: row.Exp, Level: lv, Tier: progress.TierName(progress.TierFromLevel(lv)),
			HandsSolved: row.HandsSolved, BestStreak: row.BestStreak,
			IsGuest: row.IsGuest, Banned: row.Banned,
		})
	}
	ok(w, map[string]any{"entries": out})
}

func (a *API) adminRounds(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	status := r.URL.Query().Get("status")
	if mode != "" {
		if _, err := game.Config(game.Mode(mode)); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if status != "" {
		switch status {
		case "open", "solved", "skipped", "expired":
		default:
			fail(w, http.StatusBadRequest, "unknown status")
			return
		}
	}
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, total, err := a.Store.AdminListRounds(mode, status, limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	rounds := make([]adminRoundJSON, 0, len(rows))
	for _, row := range rows {
		rounds = append(rounds, adminRoundJSONOf(row))
	}
	ok(w, map[string]any{"rounds": rounds, "total": total})
}

func (a *API) adminEvents(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, total, err := a.Store.AdminEvents(action, limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	type rowJSON struct {
		ID     string `json:"id"`
		TS     string `json:"ts"`
		Actor  string `json:"actor"`
		Action string `json:"action"`
		Target string `json:"target"`
		Detail string `json:"detail"`
	}
	events := make([]rowJSON, 0, len(rows))
	for _, e := range rows {
		events = append(events, rowJSON{
			ID: e.ID, TS: e.TS.UTC().Format(time.RFC3339), Actor: e.Actor,
			Action: e.Action, Target: e.Target, Detail: e.Detail,
		})
	}
	ok(w, map[string]any{"events": events, "total": total})
}

// --- settings: database backup & reset ---

// dbBackupPath names the snapshot file written next to the database before
// (and as part of) a reset, or streamed out as a manual download.
func (a *API) dbBackupPath() string {
	dir := filepath.Dir(a.Cfg.DBPath)
	name := fmt.Sprintf("game24-backup-%s.db", time.Now().UTC().Format("20060102-150405"))
	return filepath.Join(dir, name)
}

// adminSettings returns what the Settings page shows: row counts and the
// on-disk size of the database (WAL included, since recent writes live there).
func (a *API) adminSettings(w http.ResponseWriter, _ *http.Request) {
	var players, rounds, events int64
	a.Store.QueryCounts(&players, &rounds, &events)
	size := fileSize(a.Cfg.DBPath) + fileSize(a.Cfg.DBPath+"-wal") + fileSize(a.Cfg.DBPath+"-shm")
	ok(w, map[string]any{
		"dbFile":  filepath.Base(a.Cfg.DBPath),
		"dbSize":  size,
		"players": players,
		"rounds":  rounds,
		"events":  events,
	})
}

func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}

// adminDbBackup streams a consistent snapshot of the database as a download.
// The snapshot contains encrypted PII — it is only as safe as wherever it is
// stored, and only the ENCRYPTION_KEY can unlock it.
func (a *API) adminDbBackup(w http.ResponseWriter, r *http.Request) {
	path := a.dbBackupPath()
	if err := a.Store.BackupTo(path); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	data, err := os.ReadFile(path)
	os.Remove(path) // the download travels to the admin; no need to keep a copy
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	w.Write(data)
}

// adminDbReset wipes the whole database. It always snapshots to a backup file
// next to the DB first; body must confirm with {"confirm": "RESET"}. Every
// session — including this admin's — dies with the data.
func (a *API) adminDbReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Confirm != "RESET" {
		fail(w, http.StatusBadRequest, `confirmation required: {"confirm":"RESET"}`)
		return
	}
	backupPath := a.dbBackupPath()
	st, err := a.Store.Reset(actorOf(r), backupPath)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{
		"reset":      true,
		"backupFile": filepath.Base(backupPath),
		"wiped": map[string]int64{
			"players": st.Players, "rounds": st.Rounds, "events": st.Events,
		},
	})
}
