package handler

import (
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/pattaradol9/game24/server/internal/achv"
	"github.com/pattaradol9/game24/server/internal/game"
)

// googlePlayerToken signs a player in through the store directly (the Google
// endpoint needs a live credential) and returns their session token.
func googlePlayerToken(t *testing.T, api *API, sub string) string {
	t.Helper()
	_, token, err := api.Store.GoogleLogin(sub, sub+"@mail.com", "P"+sub, "")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func playerID(t *testing.T, api *API, token string) string {
	t.Helper()
	p, err := api.Store.PlayerByToken(token)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestAchievementsCatalogEndpoint(t *testing.T) {
	_, mux := newTestAPI(t)

	code, res := get(t, mux, "/api/v1/achievements", "")
	if code != 200 {
		t.Fatalf("catalog: %d %v", code, res)
	}
	d := data(res)
	list := d["achievements"].([]any)
	if len(list) != len(achv.Catalog) {
		t.Fatalf("catalog size = %d, want %d", len(list), len(achv.Catalog))
	}
	first := list[0].(map[string]any)
	for _, key := range []string{"id", "tier", "expReward", "coinReward", "target", "title", "desc"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("achievement entry missing %q: %v", key, first)
		}
	}
	if len(d["unlocked"].(map[string]any)) != 0 || len(d["progress"].(map[string]any)) != 0 {
		t.Fatal("anonymous fetch must not leak unlock state")
	}
}

// solveSteps turns the solver's trace into the JSON payload submit expects.
func solveSteps(t *testing.T, numbers []int) []map[string]any {
	t.Helper()
	sols := game.Solve(numbers)
	if len(sols) == 0 {
		t.Fatalf("unsolvable hand: %v", numbers)
	}
	steps := []map[string]any{}
	for _, st := range sols[0].Trace {
		step := map[string]any{"op": st.Op}
		for side, ref := range map[string]game.Ref{"left": st.Left, "right": st.Right} {
			if ref.IsStep {
				step[side] = map[string]any{"step": ref.Step, "isStep": true}
			} else {
				step[side] = map[string]any{"card": ref.Card}
			}
		}
		steps = append(steps, step)
	}
	return steps
}

// solveOneHand deals and solves a single hand for the token's player,
// returning the submit response data.
func solveOneHand(t *testing.T, mux *chi.Mux, token, mode string) map[string]any {
	t.Helper()
	code, res := post(t, mux, "/api/v1/rounds", map[string]string{"mode": mode}, token)
	if code != 200 {
		t.Fatalf("create round: %d %v", code, res)
	}
	roundID := data(res)["roundId"].(string)
	numsRaw := data(res)["numbers"].([]any)
	numbers := make([]int, 4)
	for i, n := range numsRaw {
		numbers[i] = int(n.(float64))
	}
	code, res = post(t, mux, "/api/v1/rounds/"+roundID+"/submit", map[string]any{"steps": solveSteps(t, numbers)}, token)
	if code != 200 {
		t.Fatalf("submit: %d %v", code, res)
	}
	return data(res)
}

func TestSubmitPaysCoinsAndAchievements(t *testing.T) {
	api, mux := newTestAPI(t)
	token := googlePlayerToken(t, api, "achv1")

	d := solveOneHand(t, mux, token, "queen")
	if coins := d["coins"].(float64); coins <= 0 {
		t.Fatalf("coins = %v, want > 0", coins)
	}
	newAch := d["newAchievements"].([]any)
	if len(newAch) == 0 {
		t.Fatal("first solve must unlock achievements inline")
	}
	first := newAch[0].(map[string]any)
	if _, ok := first["title"].(map[string]any)["en"]; !ok {
		t.Fatalf("achievement title not bilingual: %v", first)
	}
	player := d["player"].(map[string]any)
	if tc := player["totalCoins"].(float64); tc < d["coins"].(float64) {
		t.Fatalf("totalCoins %v below hand payout", tc)
	}
	if player["skin"] == nil {
		t.Fatal("player JSON missing skin field")
	}

	// the same unlocks appear in the achievements view
	code, res := get(t, mux, "/api/v1/achievements", token)
	if code != 200 {
		t.Fatalf("achievements: %d", code)
	}
	if got := len(data(res)["unlocked"].(map[string]any)); got == 0 {
		t.Fatal("unlocked map empty after solve")
	}
}

func TestSkinShopFlow(t *testing.T) {
	api, mux := newTestAPI(t)

	// guests see the catalog but can never buy
	code, res := get(t, mux, "/api/v1/skins", "")
	if code != 200 {
		t.Fatalf("skins catalog: %d %v", code, res)
	}
	if len(data(res)["skins"].([]any)) != 12 {
		t.Fatal("expected 12 skins in catalog")
	}
	if code, _ := post(t, mux, "/api/v1/skins/midnight/buy", map[string]any{}, ""); code != 401 {
		t.Fatalf("anonymous buy = %d, want 401", code)
	}

	guestTok := func() string {
		_, res := post(t, mux, "/api/v1/players", map[string]string{"nickname": "G"}, "")
		return data(res)["token"].(string)
	}()
	if code, _ := post(t, mux, "/api/v1/skins/midnight/buy", map[string]any{}, guestTok); code != 403 {
		t.Fatalf("guest buy = %d, want 403", code)
	}
	if code, _ := post(t, mux, "/api/v1/skins/midnight/equip", map[string]any{}, guestTok); code != 403 {
		t.Fatalf("guest equip = %d, want 403", code)
	}

	token := googlePlayerToken(t, api, "shop1")

	// broke: not enough coins
	if code, res := post(t, mux, "/api/v1/skins/midnight/buy", map[string]any{}, token); code != 400 {
		t.Fatalf("broke buy = %d %v, want 400", code, res)
	}
	// unknown skin
	if code, _ := post(t, mux, "/api/v1/skins/nope/buy", map[string]any{}, token); code != 404 {
		t.Fatalf("unknown skin buy = %d, want 404", code)
	}

	// fund the wallet: one big solve (EXP + coins) then buy midnight (1000)
	if _, _, err := api.Store.AwardSolve(playerID(t, api, token), "queen", 200000, false); err != nil {
		t.Fatal(err)
	}
	if code, res := post(t, mux, "/api/v1/skins/midnight/buy", map[string]any{}, token); code != 200 {
		t.Fatalf("buy: %d %v", code, res)
	}
	if code, _ := post(t, mux, "/api/v1/skins/midnight/buy", map[string]any{}, token); code != 409 {
		t.Fatalf("double buy = %d, want 409", code)
	}
	// classic is not purchasable
	if code, _ := post(t, mux, "/api/v1/skins/classic/buy", map[string]any{}, token); code != 404 {
		t.Fatalf("classic buy = %d, want 404", code)
	}

	// equip flows through and lands in the player JSON
	if code, res := post(t, mux, "/api/v1/skins/midnight/equip", map[string]any{}, token); code != 200 {
		t.Fatalf("equip: %d %v", code, res)
	} else if skin := data(res)["player"].(map[string]any)["skin"]; skin != "midnight" {
		t.Fatalf("equipped skin = %v, want midnight", skin)
	}
	if code, _ := post(t, mux, "/api/v1/skins/galaxy/equip", map[string]any{}, token); code != 400 {
		t.Fatalf("unowned equip = %d, want 400", code)
	}

	// catalog now reports ownership and the equipped id
	code, res = get(t, mux, "/api/v1/skins", token)
	if code != 200 {
		t.Fatalf("skins: %d", code)
	}
	d := data(res)
	owned := d["owned"].([]any)
	if len(owned) != 2 || owned[0] != "classic" || owned[1] != "midnight" {
		t.Fatalf("owned = %v", owned)
	}
	if d["equipped"] != "midnight" {
		t.Fatalf("equipped = %v", d["equipped"])
	}
}
