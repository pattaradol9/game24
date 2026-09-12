package handler

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/auth"
	"github.com/pattaradol9/game24/server/internal/config"
	"github.com/pattaradol9/game24/server/internal/crypto"
	"github.com/pattaradol9/game24/server/internal/game"
	"github.com/pattaradol9/game24/server/internal/presence"
	"github.com/pattaradol9/game24/server/internal/progress"
	"github.com/pattaradol9/game24/server/internal/room"
	"github.com/pattaradol9/game24/server/internal/store"
)

func newTestAPI(t *testing.T) (*API, *chi.Mux) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cr, err := crypto.New(strings.Repeat("ab", 32)) // valid 64-hex-char key
	if err != nil {
		t.Fatal(err)
	}
	db.SetCrypter(cr)
	api := &API{
		Store:    db,
		Hub:      room.NewHub(nil),
		Cfg:      config.Config{Port: "0"},
		Presence: presence.NewBroker(),
	}
	r := chi.NewRouter()
	r.Mount("/api/v1", api.Routes())
	return api, r
}

func post(t *testing.T, mux *chi.Mux, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json %s: %v", res.Body.String(), err)
	}
	return res.Code, out
}

func get(t *testing.T, mux *chi.Mux, path, token string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	json.Unmarshal(res.Body.Bytes(), &out)
	return res.Code, out
}

func data(m map[string]any) map[string]any {
	d, _ := m["data"].(map[string]any)
	return d
}

func del(t *testing.T, mux *chi.Mux, path string, body any, token string) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest("DELETE", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var out map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json %s: %v", res.Body.String(), err)
	}
	return res.Code, out
}

func TestDeleteMe(t *testing.T) {
	api, mux := newTestAPI(t)

	// guests cannot self-delete: the account survives the attempt
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Doomed"}, "")
	guestToken := data(res)["token"].(string)
	if code, _ := del(t, mux, "/api/v1/me", map[string]string{"confirm": "DELETE"}, guestToken); code != 403 {
		t.Fatalf("guest delete = %d, want 403", code)
	}
	if code, _ := get(t, mux, "/api/v1/me", guestToken); code != 200 {
		t.Fatal("guest should survive a rejected delete")
	}

	// Google accounts delete through the same confirmation guard
	_, token, err := api.Store.GoogleLogin("sub-doomed", "doomed@example.com", "Doomed", "")
	if err != nil {
		t.Fatalf("google login: %v", err)
	}

	// without the exact confirmation the account survives
	if code, res := del(t, mux, "/api/v1/me", nil, token); code != 400 {
		t.Fatalf("unconfirmed delete = %d %v, want 400", code, res)
	}
	if code, res := del(t, mux, "/api/v1/me", map[string]string{"confirm": "delete"}, token); code != 400 {
		t.Fatalf("lowercase confirmation = %d %v, want 400", code, res)
	}
	if code, _ := get(t, mux, "/api/v1/me", token); code != 200 {
		t.Fatal("player should still exist after rejected deletes")
	}

	if code, res := del(t, mux, "/api/v1/me", map[string]string{"confirm": "DELETE"}, token); code != 200 || data(res)["deleted"] != true {
		t.Fatalf("delete me = %d %v, want 200 deleted", code, res)
	}
	// the session token dies with the account
	if code, _ := get(t, mux, "/api/v1/me", token); code != 401 {
		t.Fatalf("me after delete = %d, want 401", code)
	}
}

func TestGuestRoundFlow(t *testing.T) {
	api, mux := newTestAPI(t)
	_ = api

	// 1. guest signs in with a nickname
	code, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "TestNick"}, "")
	if code != 200 || res["success"] != true {
		t.Fatalf("create player: %d %v", code, res)
	}
	player := data(res)["player"].(map[string]any)
	token := data(res)["token"].(string)
	if player["nickname"] != "TestNick" || player["isGuest"] != true {
		t.Fatalf("player = %v", player)
	}

	// 2. deal a hand
	code, res = post(t, mux, "/api/v1/rounds", map[string]string{"mode": "queen"}, token)
	if code != 200 {
		t.Fatalf("create round: %d %v", code, res)
	}
	roundID := data(res)["roundId"].(string)
	numsRaw := data(res)["numbers"].([]any)
	numbers := make([]int, 4)
	for i, n := range numsRaw {
		numbers[i] = int(n.(float64))
	}
	hintQuota := int(data(res)["hintQuota"].(float64))
	if hintQuota != 1 { // bronze guest: 1 hint per hand
		t.Fatalf("hint quota = %v", hintQuota)
	}

	// 3. solve it
	sols := game.Solve(numbers)
	if len(sols) == 0 {
		t.Fatalf("unsolvable hand dealt: %v", numbers)
	}
	steps := []map[string]any{}
	for _, st := range sols[0].Trace {
		step := map[string]any{"op": st.Op}
		if st.Left.IsStep {
			step["left"] = map[string]any{"step": st.Left.Step, "isStep": true}
		} else {
			step["left"] = map[string]any{"card": st.Left.Card}
		}
		if st.Right.IsStep {
			step["right"] = map[string]any{"step": st.Right.Step, "isStep": true}
		} else {
			step["right"] = map[string]any{"card": st.Right.Card}
		}
		steps = append(steps, step)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/submit", map[string]any{"steps": steps}, token)
	if code != 200 {
		t.Fatalf("submit: %d %v", code, res)
	}
	if pts := data(res)["points"].(float64); pts <= 0 {
		t.Fatalf("points = %v", pts)
	}
	if expr, _ := data(res)["expr"].(string); expr == "" {
		t.Fatal("no expr returned")
	}
	// the submit's fresh profile carries the per-mode high score the
	// leaderboard ranks
	solved := data(res)["player"].(map[string]any)
	hs, ok := solved["highScores"].(map[string]any)
	if !ok {
		t.Fatalf("no highScores in player json: %v", solved)
	}
	if v, ok := hs["queen"].(float64); !ok || v <= 0 {
		t.Fatalf("queen high score = %v, want > 0", hs["queen"])
	}

	// 4. the guest's solved hand ranks on the leaderboard (score ledger
	// only: the player JSON above shows no exp/coins)
	code, res = get(t, mux, "/api/v1/leaderboard?mode=queen", "")
	if code != 200 {
		t.Fatalf("leaderboard: %d", code)
	}
	entries := data(res)["entries"].([]any)
	if len(entries) != 1 {
		t.Fatalf("board entries = %v, want the guest's row", entries)
	}
	entry := entries[0].(map[string]any)
	if entry["nickname"] != "TestNick" {
		t.Fatalf("board row = %v, want the guest", entry)
	}
	if pts, ok := entry["score"].(float64); !ok || pts <= 0 {
		t.Fatalf("guest score = %v, want > 0", entry["score"])
	}

	// 5. hint quota enforcement on a fresh round
	code, res = post(t, mux, "/api/v1/rounds", map[string]string{"mode": "jack"}, token)
	roundID = data(res)["roundId"].(string)
	_, res = post(t, mux, "/api/v1/rounds/"+roundID+"/hint", map[string]any{}, token)
	if res["success"] != true {
		t.Fatalf("first hint failed: %v", res)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/hint", map[string]any{}, token)
	if code != 403 {
		t.Fatalf("second hint should be 403, got %d", code)
	}
}

func TestRoomCreation(t *testing.T) {
	_, mux := newTestAPI(t)
	code, res := post(t, mux, "/api/v1/rooms", map[string]any{
		"mode": "queen", "rounds": 24, "hintQuota": 3, "regenQuota": 2,
	}, "")
	if code != 200 {
		t.Fatalf("create room: %d %v", code, res)
	}
	roomCode := data(res)["code"].(string)
	if len(roomCode) != 6 {
		t.Fatalf("code = %q", roomCode)
	}
	if data(res)["hostKey"] == "" {
		t.Fatal("missing host key")
	}

	code, res = get(t, mux, "/api/v1/rooms/"+roomCode, "")
	if code != 200 || data(res)["state"] != "lobby" {
		t.Fatalf("room info: %d %v", code, res)
	}

	if code, _ := get(t, mux, "/api/v1/rooms/ZZZZZZ", ""); code != 404 {
		t.Fatalf("missing room should 404, got %d", code)
	}

	// invalid config rejected
	if code, _ := post(t, mux, "/api/v1/rooms", map[string]any{"mode": "queen", "rounds": 10, "hintQuota": 1, "regenQuota": 1}, ""); code != 400 {
		t.Fatalf("invalid rounds accepted")
	}
}

func TestGoogleAuthDisabled(t *testing.T) {
	api, mux := newTestAPI(t)
	api.Google = auth.NewGoogleVerifier("unused")
	// verification will fail because the credential is garbage — that's fine,
	// we only assert the endpoint exists and rejects.
	code, _ := post(t, mux, "/api/v1/auth/google", map[string]string{"credential": "abc"}, "")
	if code != 401 {
		t.Fatalf("bad credential should 401, got %d", code)
	}
}

// One play session grants the full tier quota, shared across every hand of
// that session; a fresh session starts with the quota refilled.
func TestHintSessionQuota(t *testing.T) {
	api, mux := newTestAPI(t)

	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "Hinter"}, "")
	if res["success"] != true {
		t.Fatalf("create player: %v", res)
	}
	token := data(res)["token"].(string)
	p, err := api.Store.PlayerByToken(token)
	if err != nil {
		t.Fatal(err)
	}
	// gold tier (level 40): quota = 1 base + 1 tier bonus = 2 per session
	if _, _, err := api.Store.AwardEXP(p.ID, "queen", progress.ExpForLevel(40), true); err != nil {
		t.Fatal(err)
	}

	// newRound creates a hand in the session and reports the hints remaining.
	newRound := func(session string) (roundID string, hintsLeft int) {
		code, res := post(t, mux, "/api/v1/rounds", map[string]any{"mode": "queen", "sessionId": session}, token)
		if code != 200 {
			t.Fatalf("create round: %d %v", code, res)
		}
		if got := int(data(res)["hintQuota"].(float64)); got != 2 {
			t.Fatalf("gold hint quota = %d, want 2", got)
		}
		return data(res)["roundId"].(string), int(data(res)["hintsLeft"].(float64))
	}
	hint := func(roundID string) int {
		code, _ := post(t, mux, "/api/v1/rounds/"+roundID+"/hint", map[string]any{}, token)
		return code
	}

	// session 1: the full quota is usable, spread over different hands
	r1, left := newRound("sess-1")
	if left != 2 {
		t.Fatalf("fresh session hintsLeft = %d, want 2", left)
	}
	if code := hint(r1); code != 200 {
		t.Fatalf("first hint: %d", code)
	}
	r2, left := newRound("sess-1")
	if left != 1 {
		t.Fatalf("hintsLeft after one use = %d, want 1", left)
	}
	if code := hint(r2); code != 200 {
		t.Fatalf("second hint: %d", code)
	}
	// quota spent: the next hand of the same session gets no hint
	r3, left := newRound("sess-1")
	if left != 0 {
		t.Fatalf("hintsLeft after two uses = %d, want 0", left)
	}
	if code := hint(r3); code != 403 {
		t.Fatalf("hint past quota = %d, want 403", code)
	}

	// session 2: a fresh visit refills the budget
	r4, left := newRound("sess-2")
	if left != 2 {
		t.Fatalf("new session hintsLeft = %d, want 2", left)
	}
	if code := hint(r4); code != 200 {
		t.Fatalf("hint in new session: %d", code)
	}
}

// stepsFor turns the solver's trace of a dealt hand into a submit body.
func stepsFor(t *testing.T, numbers []int) []map[string]any {
	t.Helper()
	sols := game.Solve(numbers)
	if len(sols) == 0 {
		t.Fatalf("unsolvable hand dealt: %v", numbers)
	}
	steps := []map[string]any{}
	for _, st := range sols[0].Trace {
		step := map[string]any{"op": st.Op}
		if st.Left.IsStep {
			step["left"] = map[string]any{"step": st.Left.Step, "isStep": true}
		} else {
			step["left"] = map[string]any{"card": st.Left.Card}
		}
		if st.Right.IsStep {
			step["right"] = map[string]any{"step": st.Right.Step, "isStep": true}
		} else {
			step["right"] = map[string]any{"card": st.Right.Card}
		}
		steps = append(steps, step)
	}
	return steps
}

func TestRoundTimeExtend(t *testing.T) {
	api, mux := newTestAPI(t)

	// guests are gated out like every other item action
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "ExtGuest"}, "")
	guestToken := data(res)["token"].(string)
	_, res = post(t, mux, "/api/v1/rounds", map[string]string{"mode": "queen"}, guestToken)
	guestRound := data(res)["roundId"].(string)
	if code, _ := post(t, mux, "/api/v1/rounds/"+guestRound+"/extend", map[string]any{}, guestToken); code != 403 {
		t.Fatalf("guest extend = %d, want 403", code)
	}

	// fund a Google player with two time items
	p, token, err := api.Store.GoogleLogin("sub-extend", "extend@mail.com", "Ext", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := api.Store.AwardEXP(p.ID, "queen", 5000, true); err != nil {
		t.Fatal(err)
	}
	if err := api.Store.BuyItem(p.ID, "time30", 2); err != nil {
		t.Fatal(err)
	}

	// first hand of session s1: the extend widens the window 90 → 120s
	code, res := post(t, mux, "/api/v1/rounds", map[string]any{"mode": "queen", "sessionId": "s1"}, token)
	if code != 200 {
		t.Fatalf("create round: %d %v", code, res)
	}
	roundID := data(res)["roundId"].(string)
	numsRaw := data(res)["numbers"].([]any)
	numbers := make([]int, 4)
	for i, n := range numsRaw {
		numbers[i] = int(n.(float64))
	}
	// a mid-play press: 40s of the 90s window played away leaves room for
	// the full +30 (the clamp only bites near the top of the clock)
	if err := api.Store.ShiftRoundDealt(roundID, -40*time.Second); err != nil {
		t.Fatal(err)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/extend", map[string]any{}, token)
	if code != 200 {
		t.Fatalf("extend: %d %v", code, res)
	}
	if tl := int(data(res)["timeLimit"].(float64)); tl != 120 {
		t.Fatalf("timeLimit after extend = %d, want 120 (queen 90 + 30)", tl)
	}
	if extra := int(data(res)["extraSeconds"].(float64)); extra != 30 {
		t.Fatalf("extraSeconds = %d, want 30", extra)
	}
	player := data(res)["player"].(map[string]any)
	items := player["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("inventory after extend = %v, want one time30 stack", items)
	}
	stack := items[0].(map[string]any)
	if stack["id"] != "time30" || int(stack["qty"].(float64)) != 1 {
		t.Fatalf("stack = %v, want time30 ×1", stack)
	}

	// the generic use endpoint refuses time items with a clear reason
	if code, _ := post(t, mux, "/api/v1/items/time30/use", map[string]any{}, token); code != 400 {
		t.Fatalf("time item via /use should be 400")
	}

	// the once-per-session rule: the NEXT hand of s1 is refused
	code, res = post(t, mux, "/api/v1/rounds", map[string]any{"mode": "queen", "sessionId": "s1"}, token)
	if code != 200 {
		t.Fatalf("second round: %d %v", code, res)
	}
	r2 := data(res)["roundId"].(string)
	if code, _ := post(t, mux, "/api/v1/rounds/"+r2+"/extend", map[string]any{}, token); code != 400 {
		t.Fatalf("second extend in session = %d, want 400", code)
	}

	// the widened window really pays: backdate the deal to 100s ago — past
	// the plain queen window (90s + 10s grace) but inside the extended one
	// (90 + 30 + 10) — then a correct submit still banks points
	if err := api.Store.ShiftRoundDealt(roundID, -100*time.Second); err != nil {
		t.Fatal(err)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/submit", map[string]any{"steps": stepsFor(t, numbers)}, token)
	if code != 200 {
		t.Fatalf("submit 100s into an extended window = %d %v, want 200", code, res)
	}
	if pts := data(res)["points"].(float64); pts <= 0 {
		t.Fatalf("points = %v, want a positive score", pts)
	}
	// the score/EXP split: EXP banks from its own (level-unmultiplied) curve,
	// so it must sit strictly below the level-multiplied score for a leveled
	// player
	if exp := data(res)["exp"].(float64); exp <= 0 || exp >= data(res)["points"].(float64) {
		t.Fatalf("exp = %v, want a positive payout strictly below the score %v", exp, data(res)["points"])
	}

	// a plain (never-extended) hand is long dead at 100s: the same backdate
	// closes it as expired
	code, res = post(t, mux, "/api/v1/rounds", map[string]any{"mode": "queen", "sessionId": "s2"}, token)
	if code != 200 {
		t.Fatalf("third round: %d %v", code, res)
	}
	r3 := data(res)["roundId"].(string)
	numsRaw = data(res)["numbers"].([]any)
	for i, n := range numsRaw {
		numbers[i] = int(n.(float64))
	}
	if err := api.Store.ShiftRoundDealt(r3, -100*time.Second); err != nil {
		t.Fatal(err)
	}
	if code, _ := post(t, mux, "/api/v1/rounds/"+r3+"/submit", map[string]any{"steps": stepsFor(t, numbers)}, token); code != 408 {
		t.Fatalf("submit 100s into a plain window = %d, want 408", code)
	}
}

// The Skip button is item-gated — one Skip Pass per play session — while the
// plain timeout endpoint folds hands for free only after their window
// genuinely ran out (verified server-side, extension included).
func TestRoundSkipItemAndTimeout(t *testing.T) {
	api, mux := newTestAPI(t)

	// guests hold no items: their skip is the Google gate
	_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "SkipGuest"}, "")
	guestToken := data(res)["token"].(string)
	_, res = post(t, mux, "/api/v1/rounds", map[string]string{"mode": "queen"}, guestToken)
	guestRound := data(res)["roundId"].(string)
	if code, _ := post(t, mux, "/api/v1/rounds/"+guestRound+"/skip", map[string]any{}, guestToken); code != 403 {
		t.Fatalf("guest skip = %d, want 403", code)
	}

	// a Google player, first hand BEFORE owning any pass
	p, token, err := api.Store.GoogleLogin("sub-skip", "skip@mail.com", "Skipper", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := api.Store.AwardEXP(p.ID, "queen", 5000, true); err != nil {
		t.Fatal(err)
	}
	newRound := func(session string) string {
		code, res := post(t, mux, "/api/v1/rounds", map[string]any{"mode": "queen", "sessionId": session}, token)
		if code != 200 {
			t.Fatalf("create round: %d %v", code, res)
		}
		return data(res)["roundId"].(string)
	}
	r0 := newRound("s0")
	if code, _ := post(t, mux, "/api/v1/rounds/"+r0+"/skip", map[string]any{}, token); code != 400 {
		t.Fatalf("skip with empty bag = %d, want 400", code)
	}

	// two passes in the bag; the generic use endpoint refuses play helpers
	if err := api.Store.BuyItem(p.ID, "skip1", 2); err != nil {
		t.Fatal(err)
	}
	if code, _ := post(t, mux, "/api/v1/items/skip1/use", map[string]any{}, token); code != 400 {
		t.Fatalf("skip item via /use should be 400")
	}

	// the first fold spends one pass and returns the solution
	code, res := post(t, mux, "/api/v1/rounds/"+r0+"/skip", map[string]any{}, token)
	if code != 200 {
		t.Fatalf("item skip: %d %v", code, res)
	}
	if sol, _ := data(res)["solution"].(string); sol == "" {
		t.Fatal("item skip returned no solution")
	}
	if stack := bagStack(t, data(res)["player"], "skip1"); stack != 1 {
		t.Fatalf("skip1 in bag after skip = %d, want 1", stack)
	}

	// the once-per-session rule: the NEXT hand of s0 is refused even with a
	// pass left, and the refusal consumes nothing
	r1 := newRound("s0")
	if code, _ := post(t, mux, "/api/v1/rounds/"+r1+"/skip", map[string]any{}, token); code != 400 {
		t.Fatalf("second skip in session = %d, want 400", code)
	}
	_, res = get(t, mux, "/api/v1/me", token)
	if stack := bagStack(t, data(res)["player"], "skip1"); stack != 1 {
		t.Fatalf("skip1 in bag after refused skip = %d, want 1", stack)
	}

	// a fresh session skips again — the bag runs dry
	r2 := newRound("s1")
	if code, res := post(t, mux, "/api/v1/rounds/"+r2+"/skip", map[string]any{}, token); code != 200 {
		t.Fatalf("skip in fresh session: %d %v", code, res)
	}
	_, res = get(t, mux, "/api/v1/me", token)
	if stack := bagStack(t, data(res)["player"], "skip1"); stack != 0 {
		t.Fatalf("skip1 in bag after both passes = %d, want 0", stack)
	}

	// the timeout endpoint refuses a live hand…
	r3 := newRound("s2")
	if code, _ := post(t, mux, "/api/v1/rounds/"+r3+"/timeout", map[string]any{}, token); code != 400 {
		t.Fatalf("timeout on a live hand = %d, want 400", code)
	}
	// …then folds it once the window (90s + grace) is truly gone
	if err := api.Store.ShiftRoundDealt(r3, -120*time.Second); err != nil {
		t.Fatal(err)
	}
	code, res = post(t, mux, "/api/v1/rounds/"+r3+"/timeout", map[string]any{}, token)
	if code != 200 {
		t.Fatalf("timeout on a dead hand: %d %v", code, res)
	}
	if sol, _ := data(res)["solution"].(string); sol == "" {
		t.Fatal("timeout returned no solution")
	}
	if code, _ := post(t, mux, "/api/v1/rounds/"+r3+"/timeout", map[string]any{}, token); code != 409 {
		t.Fatalf("second timeout = %d, want 409", code)
	}
}

// bagStack reads one item's qty out of a player JSON (0 when absent).
func bagStack(t *testing.T, player any, itemID string) int {
	t.Helper()
	p, ok := player.(map[string]any)
	if !ok {
		t.Fatalf("player JSON = %v", player)
	}
	items, _ := p["items"].([]any)
	for _, raw := range items {
		it := raw.(map[string]any)
		if it["id"] == itemID {
			return int(it["qty"].(float64))
		}
	}
	return 0
}
