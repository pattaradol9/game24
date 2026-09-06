// Package handler wires the REST API: players, auth, single-player rounds,
// leaderboard and room management.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/auth"
	"github.com/pattaradol9/game24/server/internal/config"
	"github.com/pattaradol9/game24/server/internal/game"
	"github.com/pattaradol9/game24/server/internal/progress"
	"github.com/pattaradol9/game24/server/internal/room"
	"github.com/pattaradol9/game24/server/internal/store"
)

type API struct {
	Store  *store.Store
	Google *auth.GoogleVerifier
	Hub    *room.Hub
	Cfg    config.Config
	WS     http.Handler // room websocket endpoint (mounted at /ws/room/{code})
}

type envelopeOut struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func ok(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(envelopeOut{Success: true, Data: data})
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(envelopeOut{Success: false, Message: msg})
}

type modeStatJSON struct {
	Exp           int64 `json:"exp"`
	HandsSolved   int64 `json:"handsSolved"`
	HandsSkipped  int64 `json:"handsSkipped"`
	BestStreak    int64 `json:"bestStreak"`
	CurrentStreak int64 `json:"currentStreak"`
}

type playerJSON struct {
	ID        string                  `json:"id"`
	Nickname  string                  `json:"nickname"`
	Email     string                  `json:"email,omitempty"`
	Picture   string                  `json:"picture,omitempty"`
	IsGuest   bool                    `json:"isGuest"`
	IsAdmin   bool                    `json:"isAdmin"`
	TotalExp  int64                   `json:"totalExp"`
	Level     int64                   `json:"level"`
	LevelInto int64                   `json:"levelExpInto"`
	LevelNext int64                   `json:"levelExpForNext"`
	Tier      string                  `json:"tier"`
	TierBonus int                     `json:"tierHintBonus"` // single-player hint bonus
	PerMode   map[string]modeStatJSON `json:"perMode"`
}

// isAdminPlayer reports whether the player's Google email is on the
// ADMIN_EMAILS allowlist (guests never are).
func (a *API) isAdminPlayer(p store.Player) bool {
	if p.IsGuest || p.Email == "" {
		return false
	}
	return slices.Contains(a.Cfg.AdminEmails, strings.ToLower(strings.TrimSpace(p.Email)))
}

func (a *API) toPlayerJSON(p store.Player) playerJSON {
	lv, into, next := progress.LevelProgress(p.TotalExp)
	tier := progress.TierFromLevel(lv)
	out := playerJSON{
		ID: p.ID, Nickname: p.Nickname, Email: p.Email, Picture: p.Picture,
		IsGuest: p.IsGuest, IsAdmin: a.isAdminPlayer(p), TotalExp: p.TotalExp,
		Level: lv, LevelInto: into, LevelNext: next,
		Tier: progress.TierName(tier), TierBonus: progress.TierBonusQuota(tier),
		PerMode: map[string]modeStatJSON{},
	}
	for mode, st := range p.Stats {
		out.PerMode[mode] = modeStatJSON{
			Exp: st.Exp, HandsSolved: st.HandsSolved, HandsSkipped: st.HandsSkipped,
			BestStreak: st.BestStreak, CurrentStreak: st.CurrentStreak,
		}
	}
	return out
}

// --- auth middleware ---

type ctxKey int

const playerKey ctxKey = 1

func (a *API) playerFrom(r *http.Request) (store.Player, error) {
	header := r.Header.Get("Authorization")
	token := strings.TrimPrefix(header, "Bearer ")
	if token == "" || token == header {
		return store.Player{}, errors.New("no bearer token")
	}
	p, err := a.Store.PlayerByToken(token)
	if err != nil {
		return store.Player{}, err
	}
	return p, nil
}

func requirePlayer(a *API) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := a.playerFrom(r)
			if errors.Is(err, store.ErrBanned) {
				fail(w, http.StatusForbidden, "account banned")
				return
			}
			if err != nil {
				fail(w, http.StatusUnauthorized, "sign in required")
				return
			}
			next.ServeHTTP(w, r.WithContext(contextWithPlayer(r, p)))
		})
	}
}

// optionalPlayer attaches the player when a valid token is present. Banned
// players are treated as anonymous (mutations still fail elsewhere).
func optionalPlayer(a *API) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p, err := a.playerFrom(r); err == nil {
				r = r.WithContext(contextWithPlayer(r, p))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func playerOf(r *http.Request) (store.Player, bool) {
	p, ok := r.Context().Value(playerKey).(store.Player)
	return p, ok
}

func contextWithPlayer(r *http.Request, p store.Player) context.Context {
	return context.WithValue(r.Context(), playerKey, p)
}

// --- handlers ---

func (a *API) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { ok(w, map[string]string{"status": "ok"}) })
	r.Handle("/ws/room/{code}", a.WS)
	r.Get("/meta", func(w http.ResponseWriter, _ *http.Request) {
		ok(w, map[string]any{"googleClientId": a.Cfg.GoogleClientID})
	})

	r.Post("/players", a.createPlayer)
	r.Post("/auth/google", a.googleAuth)
	r.Get("/leaderboard", a.leaderboard)
	r.Get("/rooms/{code}", a.roomInfo)

	r.Group(func(priv chi.Router) {
		priv.Use(requirePlayer(a))
		priv.Get("/me", a.me)
		priv.Post("/me/rename", a.renameMe)
		priv.Post("/rounds", a.createRound)
		priv.Post("/rounds/{id}/submit", a.submitRound)
		priv.Post("/rounds/{id}/skip", a.skipRound)
		priv.Post("/rounds/{id}/hint", a.hintRound)
	})

	r.Group(func(priv chi.Router) {
		priv.Use(optionalPlayer(a))
		priv.Post("/rooms", a.createRoom)
	})

	// admin portal — 404s entirely when the ADMIN_EMAILS allowlist is empty
	// (fail closed)
	r.Route("/admin", func(ar chi.Router) {
		ar.Use(a.requireAdmin)
		a.adminRoutes(ar)
	})
	return r
}

func (a *API) createPlayer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Nickname) == "" {
		fail(w, http.StatusBadRequest, "nickname required")
		return
	}
	name := sanitizeName(body.Nickname)
	if name == "" {
		fail(w, http.StatusBadRequest, "nickname required")
		return
	}
	p, token, err := a.Store.CreateGuest(name)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"player": a.toPlayerJSON(p), "token": token})
}

func (a *API) googleAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Credential == "" {
		fail(w, http.StatusBadRequest, "credential required")
		return
	}
	if a.Google == nil {
		fail(w, http.StatusServiceUnavailable, "google sign-in not configured")
		return
	}
	claims, err := a.Google.Verify(r.Context(), body.Credential)
	if err != nil {
		fail(w, http.StatusUnauthorized, "invalid google credential")
		return
	}
	p, token, err := a.Store.GoogleLogin(claims.Sub, claims.Email, claims.Name, claims.Picture)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"player": a.toPlayerJSON(p), "token": token})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	ok(w, map[string]any{"player": a.toPlayerJSON(p)})
}

func (a *API) renameMe(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	var body struct {
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "nickname required")
		return
	}
	name := sanitizeName(body.Nickname)
	if name == "" {
		fail(w, http.StatusBadRequest, "nickname required")
		return
	}
	updated, err := a.Store.RenamePlayer("player:"+p.ID, p.ID, name)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"player": a.toPlayerJSON(updated)})
}

func sanitizeName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range strings.TrimSpace(s) {
		if r == '\n' || r == '\t' {
			continue
		}
		out = append(out, r)
		if len(out) >= 24 {
			break
		}
	}
	return string(out)
}

// --- single player rounds ---

func (a *API) createRound(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	var body struct {
		Mode game.Mode `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "mode required")
		return
	}
	cfg, err := game.Config(body.Mode)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	hand, err := game.Deal(body.Mode, rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()>>32))))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	round, err := a.Store.CreateRound(p.ID, string(body.Mode), hand)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	lv := progress.LevelFromExp(p.TotalExp)
	ok(w, map[string]any{
		"roundId":    round.ID,
		"numbers":    hand,
		"timeLimit":  cfg.TimeLimitSec,
		"multiplier": cfg.Multiplier,
		"hintQuota":  progress.SingleHintQuota(progress.TierFromLevel(lv)),
	})
}

func (a *API) submitRound(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	roundID := chi.URLParam(r, "id")
	var body struct {
		Steps []game.Step `json:"steps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "steps required")
		return
	}
	round, err := a.Store.RoundByID(roundID, p.ID)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "round not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if round.Status != "open" {
		fail(w, http.StatusConflict, "round already finished")
		return
	}
	cfg, _ := game.Config(game.Mode(round.Mode))
	elapsed := time.Since(round.DealtAt)
	if elapsed > time.Duration(cfg.TimeLimitSec+10)*time.Second {
		a.Store.FinishRound(round.ID, "expired", 0, elapsed.Milliseconds())
		if !p.IsGuest {
			a.Store.AwardEXP(p.ID, round.Mode, 0, false)
		}
		fail(w, http.StatusRequestTimeout, "time over")
		return
	}
	if err := game.Verify(round.Numbers, body.Steps); err != nil {
		fail(w, http.StatusUnprocessableEntity, "wrong: "+err.Error())
		return
	}
	remaining := int64(cfg.TimeLimitSec) - int64(elapsed.Seconds())
	if remaining < 0 {
		remaining = 0
	}
	points := (10 + remaining) * int64(cfg.Multiplier)

	lvBefore := progress.LevelFromExp(p.TotalExp)
	if !p.IsGuest {
		if _, _, err := a.Store.AwardEXP(p.ID, round.Mode, points, true); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	a.Store.FinishRound(round.ID, "solved", points, elapsed.Milliseconds())

	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	lvAfter := progress.LevelFromExp(fresh.TotalExp)
	tierAfter := progress.TierFromLevel(lvAfter)

	ok(w, map[string]any{
		"expr":    game.ExprFromSteps(round.Numbers, body.Steps),
		"points":  points,
		"player":  a.toPlayerJSON(fresh),
		"levelUp": lvAfter > lvBefore,
		"tierUp":  tierAfter != progress.TierFromLevel(lvBefore),
	})
}

func (a *API) skipRound(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	roundID := chi.URLParam(r, "id")
	round, err := a.Store.RoundByID(roundID, p.ID)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "round not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if round.Status != "open" {
		fail(w, http.StatusConflict, "round already finished")
		return
	}
	a.Store.FinishRound(round.ID, "skipped", 0, time.Since(round.DealtAt).Milliseconds())
	if !p.IsGuest {
		a.Store.AwardEXP(p.ID, round.Mode, 0, false)
	}

	sols := game.Solve(round.Numbers)
	expr := ""
	if len(sols) > 0 {
		expr = sols[0].Expr
	}
	fresh, _ := a.Store.PlayerByToken(bearerToken(r))
	ok(w, map[string]any{
		"solution": expr,
		"player":   a.toPlayerJSON(fresh),
	})
}

func (a *API) hintRound(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	roundID := chi.URLParam(r, "id")
	round, err := a.Store.RoundByID(roundID, p.ID)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "round not found")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if round.Status != "open" {
		fail(w, http.StatusConflict, "round already finished")
		return
	}
	lv := progress.LevelFromExp(p.TotalExp)
	quota := int64(progress.SingleHintQuota(progress.TierFromLevel(lv)))
	if round.HintsUsed >= quota {
		fail(w, http.StatusForbidden, "no hints left for this hand")
		return
	}
	if _, err := a.Store.TryUseHint(round.ID, p.ID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	h, found := game.Hint(round.Numbers)
	if !found {
		fail(w, http.StatusConflict, "no hint available")
		return
	}
	ok(w, map[string]any{
		"leftCard": h.LeftCard, "rightCard": h.RightCard,
		"op": h.Op, "result": h.Result.String(),
		"leftForHintQuota": quota - round.HintsUsed - 1,
	})
}

// --- leaderboard ---

func (a *API) leaderboard(w http.ResponseWriter, r *http.Request) {
	mode := game.Mode(r.URL.Query().Get("mode"))
	if _, err := game.Config(mode); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, err := a.Store.Leaderboard(string(mode), limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	type rowJSON struct {
		Rank        int    `json:"rank"`
		PlayerID    string `json:"playerId"`
		Nickname    string `json:"nickname"`
		Picture     string `json:"picture,omitempty"`
		Exp         int64  `json:"exp"`
		Level       int64  `json:"level"`
		Tier        string `json:"tier"`
		HandsSolved int64  `json:"handsSolved"`
		BestStreak  int64  `json:"bestStreak"`
	}
	out := make([]rowJSON, 0, len(rows))
	for i, row := range rows {
		lv := progress.LevelFromExp(row.Exp)
		out = append(out, rowJSON{
			Rank: offset + i + 1, PlayerID: row.PlayerID, Nickname: row.Nickname, Picture: row.Picture,
			Exp: row.Exp, Level: lv, Tier: progress.TierName(progress.TierFromLevel(lv)),
			HandsSolved: row.HandsSolved, BestStreak: row.BestStreak,
		})
	}
	ok(w, map[string]any{"entries": out})
}

// --- rooms ---

func (a *API) createRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode       game.Mode `json:"mode"`
		Rounds     int       `json:"rounds"`
		HintQuota  int       `json:"hintQuota"`
		RegenQuota int       `json:"regenQuota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	rm, hostKey, err := a.Hub.Create(room.Config{
		Mode: body.Mode, Rounds: body.Rounds,
		HintQuota: body.HintQuota, RegenQuota: body.RegenQuota,
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w, map[string]any{"code": rm.Code(), "hostKey": hostKey})
}

func (a *API) roomInfo(w http.ResponseWriter, r *http.Request) {
	rm, err := a.Hub.Get(chi.URLParam(r, "code"))
	if err != nil {
		fail(w, http.StatusNotFound, "room not found")
		return
	}
	ok(w, rm.PublicInfo())
}

// --- helpers ---

func bearerToken(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
		if n > 1_000_000 {
			return fallback
		}
	}
	return n
}
