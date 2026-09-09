// Boost-item endpoints: catalog visibility, guest gates, buy/use against the
// inventory, and the additive stacking reflected in the player JSON.
package handler

import (
	"testing"

	"github.com/pattaradol9/game24/server/internal/items"
)

func TestItemsCatalogAndBuyUse(t *testing.T) {
	api, mux := newTestAPI(t)

	// the public catalog carries every item and no ownership state
	code, res := get(t, mux, "/api/v1/items", "")
	if code != 200 {
		t.Fatalf("catalog: %d %v", code, res)
	}
	d := data(res)
	if list := d["items"].([]any); len(list) != len(items.Catalog) {
		t.Fatalf("catalog size = %d, want %d", len(list), len(items.Catalog))
	}
	if len(d["owned"].(map[string]any)) != 0 {
		t.Fatal("anonymous fetch must not leak inventory")
	}

	// guests cannot buy or use
	guestCode, guestRes := post(t, mux, "/api/v1/players", map[string]any{"nickname": "Temp"}, "")
	if guestCode != 200 {
		t.Fatalf("guest create: %d %v", guestCode, guestRes)
	}
	guestToken := data(guestRes)["token"].(string)
	if code, _ := post(t, mux, "/api/v1/items/exp2/buy", map[string]any{}, guestToken); code != 403 {
		t.Fatalf("guest buy = %d, want 403", code)
	}
	if code, _ := post(t, mux, "/api/v1/items/exp2/use", map[string]any{}, guestToken); code != 403 {
		t.Fatalf("guest use = %d, want 403", code)
	}

	// a Google player funds up, buys an item and pops it
	token := googlePlayerToken(t, api, "itemshop")
	if _, _, err := api.Store.AwardEXP(playerID(t, api, token), "queen", 5000, true); err != nil {
		t.Fatal(err)
	}
	code, res = post(t, mux, "/api/v1/items/exp2/buy", map[string]any{}, token)
	if code != 200 {
		t.Fatalf("buy: %d %v", code, res)
	}
	player := data(res)["player"].(map[string]any)
	itemsOwned := player["items"].([]any)
	if len(itemsOwned) != 1 || itemsOwned[0].(map[string]any)["id"] != "exp2" {
		t.Fatalf("player.items after buy = %v, want one exp2", itemsOwned)
	}

	code, res = post(t, mux, "/api/v1/items/exp2/use", map[string]any{}, token)
	if code != 200 {
		t.Fatalf("use: %d %v", code, res)
	}
	player = data(res)["player"].(map[string]any)
	if _, ok := player["items"]; ok {
		t.Fatalf("player.items after spending the last unit = %v, want omitted", player["items"])
	}
	boosts := player["boosts"].(map[string]any)
	exp, ok := boosts["exp"].(map[string]any)
	if !ok {
		t.Fatalf("player.boosts after use = %v, want an exp entry", boosts)
	}
	if exp["multiplier"].(float64) != 2 {
		t.Fatalf("boost multiplier = %v, want 2", exp["multiplier"])
	}
	sources := exp["sources"].([]any)
	if len(sources) != 1 || sources[0].(map[string]any)["origin"] != "item" {
		t.Fatalf("boost sources = %v, want one item source", sources)
	}

	// the inventory read mirrors what the player JSON said
	code, res = get(t, mux, "/api/v1/items", token)
	if code != 200 {
		t.Fatalf("catalog with token: %d %v", code, res)
	}
	d = data(res)
	if owned := d["owned"].(map[string]any); len(owned) != 0 {
		t.Fatalf("owned after spending the last unit = %v, want empty", owned)
	}
	active := d["active"].(map[string]any)
	if _, ok := active["exp2"]; !ok {
		t.Fatalf("active windows = %v, want exp2 running", active)
	}

	// unknown items 404
	if code, _ = post(t, mux, "/api/v1/items/nope/buy", map[string]any{}, token); code != 404 {
		t.Fatalf("unknown buy = %d, want 404", code)
	}
	if code, _ = post(t, mux, "/api/v1/items/nope/use", map[string]any{}, token); code != 404 {
		t.Fatalf("unknown use = %d, want 404", code)
	}
}

func TestPlayerJSONBoostsStackAdditively(t *testing.T) {
	api, mux := newTestAPI(t)

	// arm a server-wide ×2 EXP boost, then hand the player a personal ×2 item
	if _, err := api.Store.SetBoostConfig("admin:x", "exp", 2, 60, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Store.EnableBoost("admin:x", "exp"); err != nil {
		t.Fatal(err)
	}
	token := googlePlayerToken(t, api, "stackjson")
	id := playerID(t, api, token)
	if _, _, err := api.Store.AwardEXP(id, "queen", 5000, true); err != nil {
		t.Fatal(err)
	}
	if err := api.Store.BuyItem(id, "exp2"); err != nil {
		t.Fatal(err)
	}
	if _, action, _, err := api.Store.UseItem(id, "exp2", 1); err != nil || action != "fresh" {
		t.Fatalf("first use: action=%q err=%v, want fresh", action, err)
	}

	code, res := get(t, mux, "/api/v1/me", token)
	if code != 200 {
		t.Fatalf("me: %d %v", code, res)
	}
	boosts := data(res)["player"].(map[string]any)["boosts"].(map[string]any)
	exp := boosts["exp"].(map[string]any)
	if exp["multiplier"].(float64) != 3 {
		t.Fatalf("stacked multiplier = %v, want 3 (×2 server + ×2 item, additive)", exp["multiplier"])
	}
	sources := exp["sources"].([]any)
	if len(sources) != 2 {
		t.Fatalf("sources = %v, want server + item", sources)
	}
	if sources[0].(map[string]any)["origin"] != "server" || sources[1].(map[string]any)["origin"] != "item" {
		t.Fatalf("source order = %v, want server first then item", sources)
	}
}

func TestUseItemCountValidationAndMultiUse(t *testing.T) {
	api, mux := newTestAPI(t)
	token := googlePlayerToken(t, api, "countapi")
	id := playerID(t, api, token)
	if _, _, err := api.Store.AwardEXP(id, "queen", 30000, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if err := api.Store.BuyItem(id, "exp2"); err != nil {
			t.Fatal(err)
		}
	}
	// broken counts are refused before the bag is ever touched
	for _, n := range []any{0, -1, 2.5, 201, "3"} {
		code, _ := post(t, mux, "/api/v1/items/exp2/use", map[string]any{"count": n}, token)
		if code != 400 {
			t.Fatalf("count %v = %d, want 400", n, code)
		}
	}
	// a bodyless post still means one unit
	if code, _ := post(t, mux, "/api/v1/items/exp2/use", map[string]any{}, token); code != 200 {
		t.Fatal("bodyless use should spend one unit")
	}
	// a 3-unit use spends 3 and echoes the count
	code, res := post(t, mux, "/api/v1/items/exp2/use", map[string]any{"count": 3}, token)
	if code != 200 {
		t.Fatalf("count 3 use: %d %v", code, res)
	}
	if n := data(res)["count"].(float64); n != 3 {
		t.Fatalf("response count = %v, want 3", n)
	}
	// 6 bought − 1 − 3 = 2 left
	stacks, err := api.Store.PlayerItems(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) != 1 || stacks[0].Qty != 2 {
		t.Fatalf("inventory = %+v, want one exp2 stack of 2", stacks)
	}
	// asking for more than the bag holds spends nothing
	if code, _ = post(t, mux, "/api/v1/items/exp2/use", map[string]any{"count": 99}, token); code != 400 {
		t.Fatalf("use 99 of 2 = %d, want 400", code)
	}
}
