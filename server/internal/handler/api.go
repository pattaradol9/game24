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
	"github.com/pattaradol9/game24/server/internal/items"
	"github.com/pattaradol9/game24/server/internal/presence"
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
	// Presence fans live profile pushes out to each player's open
	// /ws/player sockets; nil disables the player socket (503).
	Presence *presence.Broker
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

// boostSourceJSON names one running boost source: a server-wide campaign
// ("server") or the player's own consumable item ("item"). Item sources
// carry the item id, its bilingual name and rarity tier so the buff tray can
// colour the chip by rarity and explain itself on hover.
type boostSourceJSON struct {
	Origin     string    `json:"origin"`
	Multiplier float64   `json:"multiplier"`
	EndsAt     string    `json:"endsAt"` // RFC3339, UTC
	Item       string    `json:"item,omitempty"`
	Rarity     string    `json:"rarity,omitempty"`
	Name       *textJSON `json:"name,omitempty"`
}

// boostJSON is the player-facing view of one payout kind's running boosts.
// Multiplier is the additive stack of every listed source (a ×2 server boost
// plus a ×2 personal item reads ×3); EndsAt is the farthest source's end.
type boostJSON struct {
	Kind       string            `json:"kind"`
	Multiplier float64           `json:"multiplier"`
	EndsAt     string            `json:"endsAt"` // RFC3339, UTC
	Sources    []boostSourceJSON `json:"sources,omitempty"`
}

// boostViews collects every running server-wide boost (keyed by kind, e.g.
// "exp", "coins"); it travels as the ws/player "boost" push. Callers must
// not run it inside a store transaction.
func (a *API) boostViews() map[string]boostJSON {
	out := map[string]boostJSON{}
	for _, kind := range []store.BoostKind{store.BoostKindExp, store.BoostKindCoins} {
		if ab, active := a.Store.ActiveBoost(kind); active {
			out[string(kind)] = a.boostView("server", ab)
		}
	}
	return out
}

// serverSource renders a server-wide campaign as a tray source.
func (a *API) serverSource(ab store.ActiveBoost) boostSourceJSON {
	return boostSourceJSON{Origin: "server", Multiplier: ab.Multiplier, EndsAt: ab.EndsAt.UTC().Format(time.RFC3339)}
}

// itemSource renders a personal item boost as a tray source, enriched with
// the item's bilingual name and rarity tier (best-effort: an item since
// removed from the catalog keeps its window but loses the trimmings).
func (a *API) itemSource(ab store.ActiveBoost) boostSourceJSON {
	src := boostSourceJSON{Origin: "item", Multiplier: ab.Multiplier, EndsAt: ab.EndsAt.UTC().Format(time.RFC3339), Item: ab.ItemID}
	if def, ok := items.ByID(ab.ItemID); ok {
		src.Rarity = def.Rarity
		src.Name = &textJSON{En: def.Name.En, Th: def.Name.Th}
	}
	return src
}

// boostView renders one active boost as its single-source view.
func (a *API) boostView(origin string, ab store.ActiveBoost) boostJSON {
	var src boostSourceJSON
	if origin == "item" {
		src = a.itemSource(ab)
	} else {
		src = a.serverSource(ab)
	}
	return boostJSON{Kind: string(ab.Kind), Multiplier: ab.Multiplier, EndsAt: src.EndsAt, Sources: []boostSourceJSON{src}}
}

// playerBoostViews is the per-player active view for the player JSON: every
// running source of each kind — the server-wide campaigns and the player's
// own item boosts — stacked additively (a ×2 server boost plus a ×2 item
// pays ×3 total, never the compounded ×4). Item sources name their item,
// bilingual name and rarity for the tray. Callers must not run it inside a
// store transaction.
func (a *API) playerBoostViews(playerID string) map[string]boostJSON {
	type entry struct {
		ends    time.Time
		mult    float64
		sources []boostSourceJSON
	}
	entries := map[string]*entry{}
	add := func(src boostSourceJSON, ab store.ActiveBoost) {
		e := entries[string(ab.Kind)]
		if e == nil {
			e = &entry{}
			entries[string(ab.Kind)] = e
		}
		e.mult += ab.Multiplier - 1
		if ab.EndsAt.After(e.ends) {
			e.ends = ab.EndsAt
		}
		e.sources = append(e.sources, src)
	}
	for _, kind := range []store.BoostKind{store.BoostKindExp, store.BoostKindCoins} {
		if ab, active := a.Store.ActiveBoost(kind); active {
			add(a.serverSource(ab), ab)
		}
	}
	// fixed kind order keeps the source list deterministic
	for _, kind := range []store.BoostKind{store.BoostKindExp, store.BoostKindCoins} {
		if ab, ok := a.Store.ActivePlayerBoost(playerID, kind); ok {
			add(a.itemSource(ab), ab)
		}
	}
	out := map[string]boostJSON{}
	for k, e := range entries {
		// each source banked its bonus (multiplier − 1); the base 1 turns the
		// bonus sum back into the total payout multiplier
		out[k] = boostJSON{Kind: k, Multiplier: 1 + e.mult, EndsAt: e.ends.UTC().Format(time.RFC3339), Sources: e.sources}
	}
	return out
}

// itemQtyJSON is one inventory stack in the player JSON.
type itemQtyJSON struct {
	ID  string `json:"id"`
	Qty int64  `json:"qty"`
}

type playerJSON struct {
	ID         string                  `json:"id"`
	Nickname   string                  `json:"nickname"`
	Email      string                  `json:"email,omitempty"`
	Picture    string                  `json:"picture,omitempty"`
	IsGuest    bool                    `json:"isGuest"`
	IsAdmin    bool                    `json:"isAdmin"`
	TotalExp   int64                   `json:"totalExp"`
	TotalCoins int64                   `json:"totalCoins"`
	Skin       string                  `json:"skin"`
	Level      int64                   `json:"level"`
	LevelInto  int64                   `json:"levelExpInto"`
	LevelNext  int64                   `json:"levelExpForNext"`
	Tier       string                  `json:"tier"`
	TierBonus  int                     `json:"tierHintBonus"` // single-player hint bonus
	Boosts     map[string]boostJSON    `json:"boosts,omitempty"`
	Items      []itemQtyJSON           `json:"items,omitempty"` // inventory stacks (signed-in players)
	PerMode    map[string]modeStatJSON `json:"perMode"`
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
	tier := p.EffectiveTier()
	out := playerJSON{
		ID: p.ID, Nickname: p.Nickname, Email: p.Email, Picture: p.Picture,
		IsGuest: p.IsGuest, IsAdmin: a.isAdminPlayer(p), TotalExp: p.TotalExp,
		TotalCoins: p.TotalCoins, Skin: p.Skin,
		Level: lv, LevelInto: into, LevelNext: next,
		Tier: progress.TierName(tier), TierBonus: progress.TierBonusQuota(tier),
		PerMode: map[string]modeStatJSON{},
	}
	// the active-boost view stacks this player's item boosts on top of the
	// server-wide ones; guests have no personal boosts to look up. The
	// inventory rides along so every open tab sees its item counts without
	// another endpoint.
	if p.IsGuest {
		out.Boosts = a.boostViews()
	} else {
		out.Boosts = a.playerBoostViews(p.ID)
		if items, err := a.Store.PlayerItems(p.ID); err == nil {
			for _, it := range items {
				out.Items = append(out.Items, itemQtyJSON{ID: it.ID, Qty: it.Qty})
			}
		}
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
	r.Handle("/ws/player", a.playerWS())
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
		priv.Delete("/me", a.deleteMe)
		priv.Post("/rounds", a.createRound)
		priv.Post("/rounds/{id}/submit", a.submitRound)
		priv.Post("/rounds/{id}/skip", a.skipRound)
		priv.Post("/rounds/{id}/timeout", a.timeoutRound)
		priv.Post("/rounds/{id}/hint", a.hintRound)
		priv.Post("/rounds/{id}/extend", a.extendRound)
		priv.Post("/skins/{id}/buy", a.buySkin)
		priv.Post("/skins/{id}/equip", a.equipSkin)
		priv.Post("/items/{id}/buy", a.buyItem)
		priv.Post("/items/{id}/use", a.useItem)
	})

	r.Group(func(priv chi.Router) {
		priv.Use(optionalPlayer(a))
		priv.Post("/rooms", a.createRoom)
		priv.Get("/achievements", a.achievements)
		priv.Get("/skins", a.skinCatalog)
		priv.Get("/items", a.itemCatalog)
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
	if errors.Is(err, store.ErrBanned) {
		fail(w, http.StatusForbidden, "account banned")
		return
	}
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
	a.notifyPlayer(updated)
	ok(w, map[string]any{"player": a.toPlayerJSON(updated)})
}

// deleteMe permanently deletes the signed-in player's own account: the
// player row and every dependent row (mode stats, rounds, achievements,
// owned skins) vanish through cascades and the session token dies with the
// account. The body must confirm the intent, mirroring the admin DB reset.
// Google accounts only — guests are ephemeral, cannot self-delete and the
// profile menu hides the action for them.
func (a *API) deleteMe(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	if p.IsGuest {
		fail(w, http.StatusForbidden, "google sign-in required")
		return
	}
	var body struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Confirm != "DELETE" {
		fail(w, http.StatusBadRequest, `confirmation required: {"confirm":"DELETE"}`)
		return
	}
	if err := a.Store.DeleteOwnPlayer(p.ID); errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "player not found")
		return
	} else if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	// any other open tab of the account learns about the deletion live
	a.notifyPlayerDeleted(p.ID)
	ok(w, map[string]any{"deleted": true})
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
		Mode      game.Mode `json:"mode"`
		SessionID string    `json:"sessionId"`
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
	body.SessionID = strings.TrimSpace(body.SessionID)
	if len(body.SessionID) > 64 {
		body.SessionID = body.SessionID[:64]
	}
	hand, err := game.Deal(body.Mode, rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(time.Now().UnixNano()>>32))))
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	round, err := a.Store.CreateRound(p.ID, string(body.Mode), hand, body.SessionID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	quota := int64(progress.SingleHintQuota(p.EffectiveTier()))
	used, err := a.Store.HintsUsedInSession(p.ID, body.SessionID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{
		"roundId":    round.ID,
		"numbers":    hand,
		"timeLimit":  cfg.TimeLimitSec,
		"multiplier": cfg.Multiplier,
		"hintQuota":  quota,
		"hintsLeft":  quota - used,
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
	// a time-extension item widens this hand's window: the expiry check and
	// the remaining-time payout both count the seconds the item banked
	elapsed := time.Since(round.DealtAt)
	if elapsed > time.Duration(cfg.TimeLimitSec+int(round.ExtendSec)+10)*time.Second {
		a.Store.FinishRound(round.ID, "expired", 0, elapsed.Milliseconds())
		// no payout on an expired submit; the skip still counts toward
		// achievements and will surface on the player's next fetch
		if !p.IsGuest {
			a.Store.AwardSkip(p.ID, round.Mode)
		}
		fail(w, http.StatusRequestTimeout, "time over")
		return
	}
	if err := game.Verify(round.Numbers, body.Steps); err != nil {
		fail(w, http.StatusUnprocessableEntity, "wrong: "+err.Error())
		return
	}
	remaining := int64(cfg.TimeLimitSec+int(round.ExtendSec)) - int64(elapsed.Seconds())
	if remaining < 0 {
		remaining = 0
	}
	// the hand's two currencies, deliberately different curves:
	//   score — speed-heavy and level-multiplied, the session scoreboard
	//   exp   — the progression payout: solving pays a solid base, speed
	//           counts at half weight, the mode scales it, and the level
	//           handicap does not compound it (coins follow the EXP, 10:1)
	lvBefore := progress.LevelFromExp(p.TotalExp)
	score := progress.ScoreForHand((10+remaining)*int64(cfg.Multiplier), lvBefore)
	expBase := progress.ExpForHand(remaining, int64(cfg.Multiplier))

	var coins, expEarned int64
	var unlocked []achievementJSON
	if !p.IsGuest {
		res, freshUnlocked, err := a.Store.AwardSolve(p.ID, round.Mode, expBase, false)
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		coins = res.CoinsEarned
		expEarned = res.ExpEarned
		for _, d := range freshUnlocked {
			unlocked = append(unlocked, toAchievementJSON(d))
		}
	}
	a.Store.FinishRound(round.ID, "solved", score, elapsed.Milliseconds())

	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	lvAfter := progress.LevelFromExp(fresh.TotalExp)
	tierAfter := fresh.EffectiveTier()

	if unlocked == nil {
		unlocked = []achievementJSON{}
	}
	ok(w, map[string]any{
		"expr":            game.ExprFromSteps(round.Numbers, body.Steps),
		"points":          score,
		"exp":             expEarned, // what the hand banked, boost multiplier included
		"coins":           coins,
		"newAchievements": unlocked,
		"player":          a.toPlayerJSON(fresh),
		"levelUp":         lvAfter > lvBefore,
		"tierUp":          tierAfter != p.EffectiveTier(),
	})
}

// skipRound is the item-gated escape hatch behind the Skip button: folding
// an open hand mid-play spends ONE Skip Pass (kind "skip") and only one
// fold per play session is allowed — the store enforces both inside the
// consumption transaction. The hand finishes as "skipped" (no payout, skip
// achievements still evaluated) and the response carries the solution plus
// the fresh player JSON so every open tab sees the new bag count.
func (a *API) skipRound(w http.ResponseWriter, r *http.Request) {
	p, _ := playerOf(r)
	roundID := chi.URLParam(r, "id")
	skipped, err := a.Store.SkipRound(p.ID, roundID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrGoogleRequired):
			a.googleOnly(w)
		case errors.Is(err, store.ErrNoItems):
			fail(w, http.StatusBadRequest, "no skip item left in inventory")
		case errors.Is(err, store.ErrSessionSkipUsed):
			fail(w, http.StatusBadRequest, "skip already used this session")
		case errors.Is(err, store.ErrRoundClosed):
			fail(w, http.StatusConflict, "round already finished")
		case errors.Is(err, store.ErrNotFound):
			fail(w, http.StatusNotFound, "round not found")
		default:
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	var unlocked []achievementJSON
	if !p.IsGuest {
		if u, err := a.Store.AwardSkip(p.ID, skipped.Mode); err == nil {
			for _, d := range u {
				unlocked = append(unlocked, toAchievementJSON(d))
			}
		}
	}
	if unlocked == nil {
		unlocked = []achievementJSON{}
	}

	sols := game.Solve(skipped.Numbers)
	expr := ""
	if len(sols) > 0 {
		expr = sols[0].Expr
	}
	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	// the bag count changed behind the player's other tabs
	a.notifyPlayer(fresh)
	ok(w, map[string]any{
		"solution":        expr,
		"newAchievements": unlocked,
		"player":          a.toPlayerJSON(fresh),
	})
}

// timeoutRound is the free end of the countdown: when a solo hand's clock
// genuinely runs out the client folds it here instead of the item-gated
// skip. The expiry is verified server-side against the dealt time (base
// window + any extension), so it can never stand in for a mid-play skip —
// only a truly dead hand folds for free. No payout either way; skip
// achievements still evaluated.
func (a *API) timeoutRound(w http.ResponseWriter, r *http.Request) {
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
	cfg, _ := game.Config(game.Mode(round.Mode))
	// the client folds the hand the moment its countdown hits zero (an intro
	// pause and network latency always keep elapsed past the base limit), so
	// the base window itself is the fence — only the +10s SUBMIT grace stays
	// exclusive to submits
	if elapsed := time.Since(round.DealtAt); elapsed < time.Duration(cfg.TimeLimitSec+int(round.ExtendSec))*time.Second {
		fail(w, http.StatusBadRequest, "time not up yet")
		return
	}
	a.Store.FinishRound(round.ID, "skipped", 0, time.Since(round.DealtAt).Milliseconds())
	var unlocked []achievementJSON
	if !p.IsGuest {
		if u, err := a.Store.AwardSkip(p.ID, round.Mode); err == nil {
			for _, d := range u {
				unlocked = append(unlocked, toAchievementJSON(d))
			}
		}
	}
	if unlocked == nil {
		unlocked = []achievementJSON{}
	}

	sols := game.Solve(round.Numbers)
	expr := ""
	if len(sols) > 0 {
		expr = sols[0].Expr
	}
	fresh, _ := a.Store.PlayerByToken(bearerToken(r))
	ok(w, map[string]any{
		"solution":        expr,
		"newAchievements": unlocked,
		"player":          a.toPlayerJSON(fresh),
	})
}

// extendRound is the play-helper behind the Add-time button: a solo player
// holding a time item (kind "time") spends one unit to add the item's
// ExtraSeconds to THIS hand — any time mid-play, not just at zero. The store
// clamps the extra so the hand's remaining time never passes the mode's base
// window (a press with the full window still ahead → 400, nothing consumed)
// and enforces the once-per-session rule in the same transaction; the
// response echoes the widened window (timeLimit = base + extra) and the
// fresh player JSON so every open tab sees the new bag count.
func (a *API) extendRound(w http.ResponseWriter, r *http.Request) {
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
	timeDef, haveTime := items.TimeItem()
	if !haveTime {
		fail(w, http.StatusServiceUnavailable, "no time item in catalog")
		return
	}
	updated, err := a.Store.ExtendRound(p.ID, roundID, timeDef.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrGoogleRequired):
			a.googleOnly(w)
		case errors.Is(err, store.ErrNoItems):
			fail(w, http.StatusBadRequest, "no time extension item left in inventory")
		case errors.Is(err, store.ErrSessionExtendUsed):
			fail(w, http.StatusBadRequest, "time extension already used this session")
		case errors.Is(err, store.ErrRoundTimeFull):
			fail(w, http.StatusBadRequest, "round time is already at this mode's limit")
		case errors.Is(err, store.ErrRoundClosed):
			fail(w, http.StatusConflict, "round already finished")
		case errors.Is(err, store.ErrNotFound):
			fail(w, http.StatusNotFound, "round not found")
		default:
			fail(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	cfg, _ := game.Config(game.Mode(updated.Mode))
	fresh, err := a.Store.PlayerByToken(bearerToken(r))
	if err != nil {
		fresh = p
	}
	// the bag count changed behind the player's other tabs
	a.notifyPlayer(fresh)
	ok(w, map[string]any{
		"roundId":      updated.ID,
		"timeLimit":    cfg.TimeLimitSec + int(updated.ExtendSec),
		"extraSeconds": updated.ExtendSec,
		"player":       a.toPlayerJSON(fresh),
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
	quota := int64(progress.SingleHintQuota(p.EffectiveTier()))
	// the budget is per play session: hands dealt after this one draw from the
	// same pool, so count every hint spent under this session id
	used := round.HintsUsed
	if round.SessionID != "" {
		if used, err = a.Store.HintsUsedInSession(p.ID, round.SessionID); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if used >= quota {
		fail(w, http.StatusForbidden, "no hints left for this session")
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
		"leftCard": h.Step.LeftCard, "rightCard": h.Step.RightCard,
		"op": h.Step.Op, "result": h.Step.Result.String(),
		"expr":             h.Expr,
		"alternatives":     h.Alternatives,
		"solutionCount":    h.Count,
		"leftForHintQuota": quota - used - 1,
	})
}

// --- leaderboard ---

// leaderboard ranks the gameplay-score ledger by HIGH SCORE: each entry's
// score is the player's best single hand for the mode inside the window, not
// a running sum. Score is separate from EXP/coins and payout boosts
// (server-wide or personal items) never touch it, so the board reflects pure
// play. period=weekly ranks hands solved in the current ISO week (since
// Monday 00:00 UTC) and answers resetsAt; period=alltime (default) ranks
// everything ever scored.
func (a *API) leaderboard(w http.ResponseWriter, r *http.Request) {
	mode := game.Mode(r.URL.Query().Get("mode"))
	if _, err := game.Config(mode); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	weekly := r.URL.Query().Get("period") == "weekly"
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	rows, err := a.Store.ScoreLeaderboard(string(mode), weekly, limit, offset)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	type rowJSON struct {
		Rank        int    `json:"rank"`
		PlayerID    string `json:"playerId"`
		Nickname    string `json:"nickname"`
		Picture     string `json:"picture,omitempty"`
		Score       int64  `json:"score"`
		Level       int64  `json:"level"`
		Tier        string `json:"tier"`
		HandsSolved int64  `json:"handsSolved"`
		BestStreak  int64  `json:"bestStreak"`
	}
	out := make([]rowJSON, 0, len(rows))
	for i, row := range rows {
		lv := progress.LevelFromExp(row.TotalExp)
		tier, _ := tierOf(lv, row.TierOverride)
		out = append(out, rowJSON{
			Rank: offset + i + 1, PlayerID: row.PlayerID, Nickname: row.Nickname, Picture: row.Picture,
			Score: row.Score, Level: lv, Tier: tier,
			HandsSolved: row.HandsSolved, BestStreak: row.BestStreak,
		})
	}
	res := map[string]any{"entries": out, "period": "alltime"}
	if weekly {
		res["period"] = "weekly"
		// the window closes when the next ISO week begins
		res["resetsAt"] = store.WeekStartUTC(time.Now()).AddDate(0, 0, 7).UTC().Format(time.RFC3339)
	}
	ok(w, res)
}

// --- rooms ---

func (a *API) createRoom(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode        game.Mode `json:"mode"`
		Rounds      int       `json:"rounds"`
		HintQuota   int       `json:"hintQuota"`
		ExtendQuota int       `json:"extendQuota"`
		RegenQuota  int       `json:"regenQuota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, "invalid payload")
		return
	}
	rm, hostKey, err := a.Hub.Create(room.Config{
		Mode: body.Mode, Rounds: body.Rounds,
		HintQuota: body.HintQuota, ExtendQuota: body.ExtendQuota,
		RegenQuota: body.RegenQuota,
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
