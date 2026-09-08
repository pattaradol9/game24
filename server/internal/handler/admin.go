// Admin portal API: dashboard overview, player administration (view, ban,
// unban, delete, rename, EXP/coin overrides, stat resets, achievement
// grants/revocations), the server-wide EXP boost (config, arm, stop), full
// leaderboards, round listings and the event log.
// Everything is mounted under /admin and guarded by an email allowlist:
// admins sign in with Google like any player and their session token unlocks
// the admin API only while their Google email is on ADMIN_EMAILS. An empty
// allowlist disables the whole group (404) — fail closed. Every player-facing
// mutation also pushes a fresh profile snapshot through the player's live
// /ws/player sockets, so adjustments land without a page refresh; boost
// changes broadcast to every connected player at once.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/achv"
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
	r.Post("/players/{id}/exp", a.adminSetEXP)
	r.Post("/players/{id}/stats/reset", a.adminResetStats)
	r.Get("/players/{id}/rounds", a.adminPlayerRounds)
	r.Post("/players/{id}/coins", a.adminSetCoins)
	r.Post("/players/{id}/tier", a.adminSetTier)
	r.Post("/players/{id}/achievements/grant", a.adminGrantAchievement)
	r.Post("/players/{id}/achievements/revoke", a.adminRevokeAchievement)
	r.Get("/leaderboard", a.adminLeaderboard)
	r.Get("/rounds", a.adminRounds)
	r.Get("/events", a.adminEvents)
	r.Get("/boosts", a.adminBoostsList)
	r.Get("/boosts/{kind}", a.adminBoostGet)
	r.Put("/boosts/{kind}", a.adminBoostPut)
	r.Post("/boosts/{kind}/enable", a.adminBoostEnable)
	r.Post("/boosts/{kind}/disable", a.adminBoostDisable)
	r.Get("/settings", a.adminSettings)
	r.Get("/db/backup", a.adminDbBackup)
	r.Get("/db/backups", a.adminDbBackups)
	r.Post("/db/restore", a.adminDbRestore)
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
	TotalCoins   int64                   `json:"totalCoins"`
	Level        int64                   `json:"level"`
	Tier         string                  `json:"tier"`
	TierOverride bool                    `json:"tierOverride,omitempty"`
	HandsSolved  int64                   `json:"handsSolved"`
	HandsSkipped int64                   `json:"handsSkipped"`
	CreatedAt    string                  `json:"createdAt"`
	LastSeenAt   string                  `json:"lastSeenAt"`
	PerMode      map[string]modeStatJSON `json:"perMode,omitempty"`
}

// tierOf resolves the display tier: the admin override when present,
// otherwise the tier derived from the level. The bool reports an override.
func tierOf(lv int64, override string) (string, bool) {
	if override != "" {
		if t, ok := progress.TierFromName(override); ok {
			return progress.TierName(t), true
		}
	}
	return progress.TierName(progress.TierFromLevel(lv)), false
}

func adminPlayerCore(p store.Player) adminPlayerJSON {
	lv, _, _ := progress.LevelProgress(p.TotalExp)
	tier, overridden := tierOf(lv, p.Tier)
	return adminPlayerJSON{
		ID: p.ID, Nickname: p.Nickname, Email: p.Email, Picture: p.Picture,
		IsGuest: p.IsGuest, Banned: p.Banned, BanReason: p.BanReason,
		TotalExp: p.TotalExp, TotalCoins: p.TotalCoins,
		Level: lv, Tier: tier, TierOverride: overridden,
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
	tier, overridden := tierOf(lv, r.TierOverride)
	return adminPlayerJSON{
		ID: r.ID, Nickname: r.Nickname, Email: r.Email, Picture: r.Picture,
		IsGuest: r.IsGuest, Banned: r.Banned, BanReason: r.BanReason,
		TotalExp: r.TotalExp, TotalCoins: r.TotalCoins,
		Level: lv, Tier: tier, TierOverride: overridden,
		HandsSolved: r.HandsSolved, HandsSkipped: r.HandsSkipped,
		CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
		LastSeenAt: r.LastSeen.UTC().Format(time.RFC3339),
	}
}

// adminAchievementJSON is one unlocked achievement in a player's detail
// view: catalog facts plus the unlock timestamp (RFC3339, UTC).
type adminAchievementJSON struct {
	ID         string   `json:"id"`
	Tier       string   `json:"tier"`
	Title      textJSON `json:"title"`
	ExpReward  int64    `json:"expReward"`
	CoinReward int64    `json:"coinReward"`
	UnlockedAt string   `json:"unlockedAt"`
}

// adminPlayerAchievements joins the player's unlock timestamps with the
// catalog, newest unlock first.
func adminPlayerAchievements(unlocked map[string]string) []adminAchievementJSON {
	out := make([]adminAchievementJSON, 0, len(unlocked))
	for id, ts := range unlocked {
		def, ok := achv.ByID(id)
		if !ok {
			continue // catalog entry retired; keep the rest viewable
		}
		unlockedAt := ts
		if t, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
			unlockedAt = t.UTC().Format(time.RFC3339)
		}
		out = append(out, adminAchievementJSON{
			ID: def.ID, Tier: string(def.Tier),
			Title:     textJSON{En: def.Title.En, Th: def.Title.Th},
			ExpReward: def.ExpReward, CoinReward: def.CoinReward,
			UnlockedAt: unlockedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UnlockedAt != out[j].UnlockedAt {
			return out[i].UnlockedAt > out[j].UnlockedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
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
	id := chi.URLParam(r, "id")
	p, err := a.Store.PlayerByID(id)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	unlocked, err := a.Store.PlayerAchievements(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{
		"player":       adminPlayerFull(p),
		"achievements": adminPlayerAchievements(unlocked),
	})
}

// adminPlayerPatch applies partial updates: nickname (admin rename) and/or
// ban state. banned=true records an optional reason (max 200 chars). An
// admin can never ban their own account — the portal would lock them out
// with no one left to lift it.
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
	if body.Banned != nil && *body.Banned {
		if actor, ok := playerOf(r); ok && actor.ID == id {
			fail(w, http.StatusBadRequest, "cannot ban your own admin account")
			return
		}
	}
	var (
		current        store.Player
		bannedNow      bool
		banReasonValue string
	)
	if body.Banned != nil {
		reason := ""
		if body.BanReason != nil {
			reason = sanitizeBanReason(*body.BanReason)
		}
		bannedNow = *body.Banned
		banReasonValue = reason
		var err error
		if current, err = a.Store.SetPlayerBanned(actorOf(r), id, bannedNow, reason); errors.Is(err, store.ErrNotFound) {
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
	// a ban lands as its own terminal event (reason included) so the
	// player's open tabs throw the ban dialog; everything else is a plain
	// profile push
	if bannedNow {
		a.notifyPlayerBanned(id, banReasonValue)
	} else {
		a.notifyPlayer(current)
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
	id := chi.URLParam(r, "id")
	if err := a.Store.DeletePlayer(actorOf(r), id); errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	} else if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// the player's open tabs drop the dead session immediately
	a.notifyPlayerDeleted(id)
	ok(w, map[string]any{"deleted": true})
}

// adminSetEXP overwrites one mode's EXP with the requested absolute value;
// total EXP and the derived level always resync to the per-mode sum.
func (a *API) adminSetEXP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode  string `json:"mode"`
		Value int64  `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if _, err := game.Config(game.Mode(body.Mode)); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Value < 0 {
		fail(w, http.StatusBadRequest, "value must not be negative")
		return
	}
	p, err := a.Store.SetPlayerEXP(actorOf(r), chi.URLParam(r, "id"), body.Mode, body.Value)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notifyPlayer(p)
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
	a.notifyPlayer(p)
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

// adminSetCoins overwrites the player's coin balance. No achievement
// cascade here — coin-hold entries re-evaluate on the player's next hand,
// like every other metric.
func (a *API) adminSetCoins(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value int64 `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if body.Value < 0 {
		fail(w, http.StatusBadRequest, "value must not be negative")
		return
	}
	p, err := a.Store.SetPlayerCoins(actorOf(r), chi.URLParam(r, "id"), body.Value)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notifyPlayer(p)
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

// adminSetTier stores a tier override (empty tier clears it). The override
// replaces the level-derived tier everywhere: profile display, leaderboards
// and the tier hint quota.
func (a *API) adminSetTier(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Tier string `json:"tier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	tier := strings.ToLower(strings.TrimSpace(body.Tier))
	if tier != "" {
		if _, ok := progress.TierFromName(tier); !ok {
			fail(w, http.StatusBadRequest, "unknown tier")
			return
		}
	}
	p, err := a.Store.SetPlayerTier(actorOf(r), chi.URLParam(r, "id"), tier)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notifyPlayer(p)
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

// adminGrantAchievement records an unlock by hand and pays the entry's
// standard EXP/coin rewards; adminRevokeAchievement removes the unlock while
// banked rewards stay. Both are wired through one shared implementation.
func (a *API) adminGrantAchievement(w http.ResponseWriter, r *http.Request) {
	a.adminAchievementMutation(w, r, true)
}

func (a *API) adminRevokeAchievement(w http.ResponseWriter, r *http.Request) {
	a.adminAchievementMutation(w, r, false)
}

func (a *API) adminAchievementMutation(w http.ResponseWriter, r *http.Request, grant bool) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	body.ID = strings.TrimSpace(body.ID)
	if _, known := achv.ByID(body.ID); !known {
		fail(w, http.StatusNotFound, "unknown achievement")
		return
	}
	var (
		p   store.Player
		err error
	)
	if grant {
		p, err = a.Store.GrantAchievement(actorOf(r), chi.URLParam(r, "id"), body.ID)
	} else {
		p, err = a.Store.RevokeAchievement(actorOf(r), chi.URLParam(r, "id"), body.ID)
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, "player or achievement not found")
		return
	case errors.Is(err, store.ErrAlreadyUnlocked):
		fail(w, http.StatusConflict, "achievement already unlocked")
		return
	case err != nil:
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notifyPlayer(p)
	ok(w, map[string]any{"player": adminPlayerFull(p)})
}

// --- server-wide boosts (EXP / coin payouts) ---

// adminBoostJSON is one boost row: the admin-authored config (multiplier,
// duration, label) plus its run state (armed? window? time left?).
type adminBoostJSON struct {
	Kind            string  `json:"kind"`
	Multiplier      float64 `json:"multiplier"`
	DurationMinutes int     `json:"durationMinutes"`
	Label           string  `json:"label,omitempty"`
	Enabled         bool    `json:"enabled"`
	Active          bool    `json:"active"`
	StartsAt        string  `json:"startsAt,omitempty"` // RFC3339, UTC
	EndsAt          string  `json:"endsAt,omitempty"`   // RFC3339, UTC
	RemainingSec    int64   `json:"remainingSec,omitempty"`
	UpdatedBy       string  `json:"updatedBy,omitempty"`
	UpdatedAt       string  `json:"updatedAt,omitempty"`
}

func adminBoostJSONOf(b store.Boost, active bool, remainingSec int64) adminBoostJSON {
	out := adminBoostJSON{
		Kind: string(b.Kind), Multiplier: b.Multiplier, DurationMinutes: b.DurationMin,
		Label: b.Label, Enabled: b.Enabled, Active: active, UpdatedBy: b.UpdatedBy,
	}
	if !b.StartsAt.IsZero() {
		out.StartsAt = b.StartsAt.UTC().Format(time.RFC3339)
	}
	if !b.EndsAt.IsZero() {
		out.EndsAt = b.EndsAt.UTC().Format(time.RFC3339)
	}
	if !b.UpdatedAt.IsZero() {
		out.UpdatedAt = b.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if active {
		out.RemainingSec = remainingSec
		if out.RemainingSec < 1 {
			out.RemainingSec = 1
		}
	}
	return out
}

// activeBoostJSON resolves one kind's run state for the JSON view.
func (a *API) activeBoostJSON(kind store.BoostKind) (bool, int64) {
	ab, active := a.Store.ActiveBoost(kind)
	if !active {
		return false, 0
	}
	left := int64(time.Until(ab.EndsAt).Seconds())
	if left < 1 {
		left = 1
	}
	return true, left
}

// adminBoostsList returns every kind's config and run state — what the
// Server Boosts page and the portal header's active strip render.
func (a *API) adminBoostsList(w http.ResponseWriter, _ *http.Request) {
	all, err := a.Store.AllBoosts()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]adminBoostJSON, 0, len(all))
	for _, b := range all {
		active, left := a.activeBoostJSON(b.Kind)
		out = append(out, adminBoostJSONOf(b, active, left))
	}
	ok(w, map[string]any{"boosts": out})
}

func (a *API) adminBoostGet(w http.ResponseWriter, r *http.Request) {
	kind := store.BoostKind(chi.URLParam(r, "kind"))
	b, err := a.Store.BoostState(kind)
	if errors.Is(err, store.ErrBoostKind) {
		fail(w, http.StatusNotFound, "unknown boost kind")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	active, left := a.activeBoostJSON(kind)
	ok(w, map[string]any{"boost": adminBoostJSONOf(b, active, left)})
}

// adminBoostPut saves one kind's config (multiplier, duration, label)
// without arming it — the draft can be prepared days before the event
// starts. Values out of range → 400.
func (a *API) adminBoostPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Multiplier      float64 `json:"multiplier"`
		DurationMinutes int     `json:"durationMinutes"`
		Label           string  `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	b, err := a.Store.SetBoostConfig(actorOf(r), store.BoostKind(chi.URLParam(r, "kind")),
		body.Multiplier, body.DurationMinutes, strings.TrimSpace(body.Label))
	a.adminBoostMutationResult(w, b, err)
}

// adminBoostEnable arms one kind: the window runs from now for the saved
// duration (re-arming an active boost restarts it).
func (a *API) adminBoostEnable(w http.ResponseWriter, r *http.Request) {
	b, err := a.Store.EnableBoost(actorOf(r), store.BoostKind(chi.URLParam(r, "kind")))
	a.adminBoostMutationResult(w, b, err)
}

// adminBoostDisable stops one kind immediately; the saved config stays for
// the next run.
func (a *API) adminBoostDisable(w http.ResponseWriter, r *http.Request) {
	b, err := a.Store.DisableBoost(actorOf(r), store.BoostKind(chi.URLParam(r, "kind")))
	a.adminBoostMutationResult(w, b, err)
}

// adminBoostMutationResult answers a boost mutation: unknown kinds are a
// 404, out-of-range values a 400, anything else a 500 — and every success
// pushes the fresh boost state to all open player sockets so running tabs
// see it at once.
func (a *API) adminBoostMutationResult(w http.ResponseWriter, b store.Boost, err error) {
	switch {
	case errors.Is(err, store.ErrBoostKind):
		fail(w, http.StatusNotFound, "unknown boost kind")
		return
	case errors.Is(err, store.ErrBoostRange):
		fail(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.notifyBoosts()
	active, left := a.activeBoostJSON(b.Kind)
	ok(w, map[string]any{"boost": adminBoostJSONOf(b, active, left)})
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
		tier, _ := tierOf(lv, row.TierOverride)
		out = append(out, rowJSON{
			Rank: offset + i + 1, PlayerID: row.PlayerID, Nickname: row.Nickname,
			Exp: row.Exp, Level: lv, Tier: tier,
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
// (and as part of) a reset or restore, or streamed out as a manual download.
// Timestamps have one-second resolution, so a same-second collision grows a
// numeric suffix (-1, -2, …) instead of refusing to snapshot.
func (a *API) dbBackupPath() string {
	dir := filepath.Dir(a.Cfg.DBPath)
	base := time.Now().UTC().Format("20060102-150405")
	for i := 0; ; i++ {
		name := fmt.Sprintf("game24-backup-%s.db", base)
		if i > 0 {
			name = fmt.Sprintf("game24-backup-%s-%d.db", base, i)
		}
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}
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

// --- settings: database restore ---

// backupNameRE constrains restore sources to files the server itself wrote
// (VACUUM INTO snapshots named by dbBackupPath, including same-second
// suffixed ones) — no path traversal, no arbitrary file reads from the
// server's disk.
var backupNameRE = regexp.MustCompile(`^game24-backup-\d{8}-\d{6}(-\d{1,4})?\.db$`)

// maxRestoreUpload caps the snapshot a browser may push to the restore
// endpoint — generous for this database's size, tight enough to be irrelevant
// as an attack surface.
const maxRestoreUpload = 1 << 30 // 1 GiB

// adminDbBackups lists the safety snapshots sitting next to the database —
// files written by resets, restores, or a crash-truncated nothing; only the
// timestamped names the server itself produces are shown.
func (a *API) adminDbBackups(w http.ResponseWriter, _ *http.Request) {
	ok(w, map[string]any{"backups": a.listBackups()})
}

func (a *API) listBackups() []map[string]any {
	dir := filepath.Dir(a.Cfg.DBPath)
	matches, _ := filepath.Glob(filepath.Join(dir, "game24-backup-*.db"))
	sort.Sort(sort.Reverse(sort.StringSlice(matches))) // names sort chronologically
	out := make([]map[string]any, 0, len(matches))
	for _, path := range matches {
		fi, err := os.Stat(path)
		if err != nil {
			continue // raced with a delete; skip it
		}
		out = append(out, map[string]any{
			"name":     filepath.Base(path),
			"size":     fi.Size(),
			"modified": fi.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return out
}

// adminDbRestore replaces the live database with a snapshot: a server-side
// safety backup by name (JSON body {"name", "confirm"}) or an uploaded
// snapshot file (multipart form: file + confirm). The word RESTORE is
// required. A safety snapshot of the current data is kept next to the
// database first; every session the backup does not contain dies with the
// restore — usually including the acting admin's.
func (a *API) adminDbRestore(w http.ResponseWriter, r *http.Request) {
	var confirm, name string
	var upload multipart.File
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxRestoreUpload)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			fail(w, http.StatusBadRequest, "invalid upload")
			return
		}
		confirm = r.FormValue("confirm")
		name = strings.TrimSpace(r.FormValue("name"))
		if f, _, err := r.FormFile("file"); err == nil {
			upload = f
		}
	} else {
		var body struct {
			Name    string `json:"name"`
			Confirm string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(w, http.StatusBadRequest, "invalid payload")
			return
		}
		confirm, name = body.Confirm, strings.TrimSpace(body.Name)
	}
	if confirm != "RESTORE" {
		fail(w, http.StatusBadRequest, `confirmation required: {"confirm":"RESTORE"}`)
		return
	}

	// resolve the source: an uploaded snapshot staged next to the database,
	// or a server-side safety backup by exact validated name
	var srcPath string
	switch {
	case upload != nil:
		defer upload.Close()
		tmp, err := os.CreateTemp(filepath.Dir(a.Cfg.DBPath), "game24-upload-*.db")
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer func() {
			tmp.Close()
			os.Remove(tmp.Name()) // the staged copy never outlives the request
		}()
		if _, err := io.Copy(tmp, upload); err != nil {
			fail(w, http.StatusBadRequest, "upload failed")
			return
		}
		srcPath = tmp.Name()
	case name != "":
		if !backupNameRE.MatchString(name) {
			fail(w, http.StatusBadRequest, "unknown backup file")
			return
		}
		path := filepath.Join(filepath.Dir(a.Cfg.DBPath), name)
		if filepath.Base(path) != name { // belt and braces beside the regexp
			fail(w, http.StatusBadRequest, "unknown backup file")
			return
		}
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			fail(w, http.StatusNotFound, "backup not found")
			return
		}
		srcPath = path
	default:
		fail(w, http.StatusBadRequest, "provide a backup name or an uploaded file")
		return
	}

	preRestore := a.dbBackupPath()
	st, err := a.Store.RestoreFrom(actorOf(r), srcPath, preRestore)
	if errors.Is(err, store.ErrInvalidBackup) {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{
		"restored":     true,
		"backupFile":   filepath.Base(srcPath),
		"safetyBackup": filepath.Base(preRestore),
		"restoredRows": map[string]int64{
			"players": st.Players, "rounds": st.Rounds, "events": st.Events,
		},
	})
}
