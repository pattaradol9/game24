// Inventory and personal-boost item flows: buying debits coins and stacks
// qty, using consumes stock and arms a window, same-kind windows REPLACE
// each other, and — the headline rule — personal boosts stack additively
// with the server-wide campaigns instead of compounding.
package store

import (
	"errors"
	"testing"
	"time"
)

func TestBuyItemGatesAndStacks(t *testing.T) {
	s := openTest(t)

	// guests never hold items
	g, _, err := s.CreateGuest("Temp")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BuyItem(g.ID, "exp2"); !errors.Is(err, ErrGoogleRequired) {
		t.Fatalf("guest buy = %v, want ErrGoogleRequired", err)
	}
	if _, _, _, err := s.UseItem(g.ID, "exp2", 1); !errors.Is(err, ErrGoogleRequired) {
		t.Fatalf("guest use = %v, want ErrGoogleRequired", err)
	}
	if err := s.BuyItem(g.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown item = %v, want ErrNotFound", err)
	}

	// a Google player buys until the balance runs dry; the stack holds every
	// successful purchase
	id, _ := googlePlayer(t, s, "buyitem")
	if _, _, err := s.AwardEXP(id, "queen", 5000, true); err != nil {
		t.Fatal(err)
	}
	bought := 0
	for {
		err := s.BuyItem(id, "coin2")
		if errors.Is(err, ErrInsufficientCoins) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		bought++
		if bought > 10_000 {
			t.Fatal("buy loop never ran dry")
		}
	}
	if bought == 0 {
		t.Fatal("a funded player should afford at least one coin2")
	}
	items, err := s.PlayerItems(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "coin2" || items[0].Qty != int64(bought) {
		t.Fatalf("inventory = %+v, want one coin2 stack of %d", items, bought)
	}
}

func TestUseItemConsumesStockAndArmsWindow(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "useitem")
	if _, _, _, err := s.UseItem(id, "exp2", 1); !errors.Is(err, ErrNoItems) {
		t.Fatalf("use with empty bag = %v, want ErrNoItems", err)
	}
	if err := s.BuyItem(id, "exp2"); !errors.Is(err, ErrInsufficientCoins) {
		t.Fatalf("buy without coins = %v, want ErrInsufficientCoins", err)
	}
	if _, _, err := s.AwardEXP(id, "queen", 5000, true); err != nil {
		t.Fatal(err)
	}
	if err := s.BuyItem(id, "exp2"); err != nil {
		t.Fatal(err)
	}
	boost, action, _, err := s.UseItem(id, "exp2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if action != "fresh" {
		t.Fatalf("first use action = %q, want fresh", action)
	}
	if boost.ItemID != "exp2" {
		t.Fatalf("armed window item = %q, want exp2", boost.ItemID)
	}
	if _, ok := s.ActivePlayerBoost(id, BoostKindExp); !ok {
		t.Fatal("personal EXP boost should be live right after use")
	}
	if _, ok := s.ActivePlayerBoost(id, BoostKindCoins); ok {
		t.Fatal("coin kind must stay untouched by an EXP item")
	}
	// the stack is spent; the row is cleaned up
	items, err := s.PlayerItems(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("inventory = %+v, want empty after using the last unit", items)
	}
}

func TestSameKindUseReplacesRunningWindow(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "replaceitem")
	// four hands fund the ×2 / ×5 / ×2 shopping list with change to spare
	for i := 0; i < 4; i++ {
		if _, _, err := s.AwardEXP(id, "queen", 9000, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, idem := range []string{"exp2", "exp5"} {
		if err := s.BuyItem(id, idem); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := s.UseItem(id, "exp2", 1); err != nil {
		t.Fatal(err)
	}
	// the fresh window restarts from the swap moment: swapping ≈30min from
	// now — extending the old window would instead land a full 30 minutes
	// later
	pre := time.Now()
	if _, action, _, err := s.UseItem(id, "exp5", 1); err != nil || action != "replaced" {
		t.Fatalf("use over a live window: action=%q err=%v, want replaced", action, err)
	}
	second, ok := s.ActivePlayerBoost(id, BoostKindExp)
	if !ok {
		t.Fatal("a window should still be live after the swap")
	}
	if second.Multiplier != 5 || second.ItemID != "exp5" {
		t.Fatalf("window after swap = ×%v (%s), want ×5 (exp5)", second.Multiplier, second.ItemID)
	}
	// the new window runs from the swap moment for the new item's duration —
	// neither extended by the old end nor kept from it
	if wantLo := pre.Add(29 * time.Minute); second.EndsAt.Before(wantLo) {
		t.Fatalf("swap end = %v, want ≈ 30min from the swap (>= %v)", second.EndsAt, wantLo)
	}
	if wantHi := pre.Add(31 * time.Minute); second.EndsAt.After(wantHi) {
		t.Fatalf("swap end = %v, want ≈ 30min from the swap (<= %v)", second.EndsAt, wantHi)
	}
	// popping the same ×5 again stacks time onto the live window instead
	if err := s.BuyItem(id, "exp5"); err != nil {
		t.Fatal(err)
	}
	if _, action, _, err := s.UseItem(id, "exp5", 1); err != nil || action != "extended" {
		t.Fatalf("re-use of the live item: action=%q err=%v, want extended", action, err)
	}
}

func TestSameMultiplierStacksDuration(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "stacktime")
	for i := 0; i < 4; i++ {
		if _, _, err := s.AwardEXP(id, "queen", 9000, true); err != nil {
			t.Fatal(err)
		}
	}
	// a ×2/30min and a ×2/60min feed the same window: 30 + 60 = 90min of ×2
	for _, idem := range []string{"exp2", "exp2h"} {
		if err := s.BuyItem(id, idem); err != nil {
			t.Fatal(err)
		}
	}
	if _, action, _, err := s.UseItem(id, "exp2", 1); err != nil || action != "fresh" {
		t.Fatalf("first use: action=%q err=%v, want fresh", action, err)
	}
	pre := time.Now()
	if _, action, _, err := s.UseItem(id, "exp2h", 1); err != nil || action != "extended" {
		t.Fatalf("second same-tier use: action=%q err=%v, want extended", action, err)
	}
	live, ok := s.ActivePlayerBoost(id, BoostKindExp)
	if !ok {
		t.Fatal("window should be live after stacking")
	}
	if live.Multiplier != 2 {
		t.Fatalf("multiplier after stacking = %v, want 2", live.Multiplier)
	}
	// the 60 minutes stack onto the old end: ≈ 90 minutes from the first use
	if wantLo := pre.Add(89 * time.Minute); live.EndsAt.Before(wantLo) {
		t.Fatalf("stacked end = %v, want ≈ 90min total (>= %v)", live.EndsAt, wantLo)
	}
	if wantHi := pre.Add(91 * time.Minute); live.EndsAt.After(wantHi) {
		t.Fatalf("stacked end = %v, want ≈ 90min total (<= %v)", live.EndsAt, wantHi)
	}
	// the window still names the latest item used
	if live.ItemID != "exp2h" {
		t.Fatalf("window item = %q, want exp2h", live.ItemID)
	}
}

func TestDurationCapBoundaryAndRefusal(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "capitem")
	for i := 0; i < 6; i++ {
		if _, _, err := s.AwardEXP(id, "queen", 9000, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, idem := range []string{"exp2", "exp2h", "exp2"} {
		if err := s.BuyItem(id, idem); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := s.UseItem(id, "exp2", 1); err != nil {
		t.Fatal(err)
	}
	// park the window at ≈23h remaining, exactly like a long chain of stacks
	if _, err := s.db.Exec(`UPDATE player_boosts SET ends_at = datetime('now', '+23 hours') WHERE player_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	// +60min lands right on the 24h mark — the boundary itself is allowed
	if _, action, capped, err := s.UseItem(id, "exp2h", 1); err != nil || action != "extended" || capped {
		t.Fatalf("use at the cap boundary: action=%q capped=%v err=%v, want extended/false", action, capped, err)
	}
	// a window already running past the cap gains nothing from another use:
	// refused outright, and the item stays in the bag
	if _, err := s.db.Exec(`UPDATE player_boosts SET ends_at = datetime('now', '+24 hours', '+30 minutes') WHERE player_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.UseItem(id, "exp2", 1); !errors.Is(err, ErrBoostDurationCap) {
		t.Fatalf("use over the cap = %v, want ErrBoostDurationCap", err)
	}
	items, err := s.PlayerItems(id)
	if err != nil {
		t.Fatal(err)
	}
	// two exp2 were bought, one was spent before the refused attempt
	if len(items) != 1 || items[0].ID != "exp2" || items[0].Qty != 1 {
		t.Fatalf("inventory = %+v, want the refused exp2 still in the bag", items)
	}
	live, ok := s.ActivePlayerBoost(id, BoostKindExp)
	if !ok {
		t.Fatal("window should still be live after the refused use")
	}
	if !live.EndsAt.After(time.Now().Add(24 * time.Hour)) {
		t.Fatalf("window end = %v, want the untouched >24h window", live.EndsAt)
	}
}

func TestUseItemCountSpendsStacksAndClamps(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "countitem")
	for i := 0; i < 14; i++ {
		if _, _, err := s.AwardEXP(id, "queen", 9000, true); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		if err := s.BuyItem(id, "exp2"); err != nil {
			t.Fatal(err)
		}
	}
	// zero and more-than-the-bag asks are refused with nothing consumed
	if _, _, _, err := s.UseItem(id, "exp2", 0); !errors.Is(err, ErrNoItems) {
		t.Fatalf("count 0 = %v, want ErrNoItems", err)
	}
	if _, _, _, err := s.UseItem(id, "exp2", 99); !errors.Is(err, ErrNoItems) {
		t.Fatalf("use 99 of 30 = %v, want ErrNoItems", err)
	}
	// a 3-unit fresh use arms 90 minutes at once
	pre := time.Now()
	boost, action, capped, err := s.UseItem(id, "exp2", 3)
	if err != nil || action != "fresh" || capped {
		t.Fatalf("3-unit use: action=%q capped=%v err=%v, want fresh/false", action, capped, err)
	}
	if boost.EndsAt.Before(pre.Add(89*time.Minute)) || boost.EndsAt.After(pre.Add(91*time.Minute)) {
		t.Fatalf("3-unit window = %v, want ≈ 90min from the use", boost.EndsAt)
	}
	// 4 more units (2h) onto a parked ≈23h window trims at the 24h cap —
	// the use succeeds and the units are still spent
	if _, err := s.db.Exec(`UPDATE player_boosts SET ends_at = datetime('now', '+23 hours') WHERE player_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	pre = time.Now()
	boost, action, capped, err = s.UseItem(id, "exp2", 4)
	if err != nil || action != "extended" || !capped {
		t.Fatalf("clamped use: action=%q capped=%v err=%v, want extended/true", action, capped, err)
	}
	if !boost.EndsAt.After(pre.Add(23*time.Hour)) || boost.EndsAt.After(pre.Add(24*time.Hour+time.Minute)) {
		t.Fatalf("clamped window = %v, want trimmed to ≈24h from the use", boost.EndsAt)
	}
	// 30 bought − 3 − 4 = 23 left
	items, err := s.PlayerItems(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "exp2" || items[0].Qty != 23 {
		t.Fatalf("inventory = %+v, want one exp2 stack of 23", items)
	}
}

func TestPersonalBoostStacksAdditivelyWithServerBoost(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "stackitem")

	// server-wide ×2 on both payouts
	if _, err := s.SetBoostConfig("admin:x", BoostKindExp, 2, 60, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetBoostConfig("admin:x", BoostKindCoins, 2, 60, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableBoost("admin:x", BoostKindExp); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableBoost("admin:x", BoostKindCoins); err != nil {
		t.Fatal(err)
	}
	// the player pops a personal ×2 EXP item on top
	if _, _, err := s.AwardEXP(id, "queen", 9000, true); err != nil {
		t.Fatal(err)
	}
	if err := s.BuyItem(id, "exp2"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.UseItem(id, "exp2", 1); err != nil {
		t.Fatal(err)
	}

	// base 100 EXP → server ×2 plus personal ×2 stack additively: ×3, never ×4
	_, _, payout, err := s.awardEXP(id, "queen", 100, true)
	if err != nil {
		t.Fatal(err)
	}
	if payout.ExpGain != 300 {
		t.Fatalf("exp payout = %d, want 300 (100 × (2+2−1)); compounding would pay 400", payout.ExpGain)
	}
	// coins have only the server ×2 running
	if payout.CoinGain != 20 {
		t.Fatalf("coin payout = %d, want 20 (10 × 2)", payout.CoinGain)
	}

	// a personal ×5 coin item stacks with the server ×2: ×6 total, never the
	// compounded ×12
	if err := s.BuyItem(id, "coin5"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.UseItem(id, "coin5", 1); err != nil {
		t.Fatal(err)
	}
	_, _, payout, err = s.awardEXP(id, "queen", 100, true)
	if err != nil {
		t.Fatal(err)
	}
	if payout.CoinGain != 60 {
		t.Fatalf("coin payout = %d, want 60 (10 × (2+5−1)); compounding would pay 100", payout.CoinGain)
	}
}

func TestExpiredPersonalBoostStopsPaying(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "expireitem")
	if _, _, err := s.AwardEXP(id, "queen", 9000, true); err != nil {
		t.Fatal(err)
	}
	if err := s.BuyItem(id, "exp2"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.UseItem(id, "exp2", 1); err != nil {
		t.Fatal(err)
	}
	// force the window into the past, the way real time would
	if _, err := s.db.Exec(`UPDATE player_boosts SET ends_at = datetime('now', '-1 minute') WHERE player_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.ActivePlayerBoost(id, BoostKindExp); ok {
		t.Fatal("expired personal boost must not apply")
	}
	_, _, payout, err := s.awardEXP(id, "queen", 100, true)
	if err != nil {
		t.Fatal(err)
	}
	if payout.ExpGain != 100 {
		t.Fatalf("exp payout = %d, want base 100 after expiry", payout.ExpGain)
	}
}
