package achv

import (
	"testing"
)

func TestCatalogSanity(t *testing.T) {
	if len(Catalog) != 50 {
		t.Fatalf("catalog has %d entries, want 50", len(Catalog))
	}
	seen := map[string]bool{}
	tiers := map[Tier]bool{
		TierBronze: true, TierSilver: true, TierGold: true,
		TierPlatinum: true, TierLegend: true,
	}
	modes := map[string]bool{"": true, "jack": true, "queen": true, "king": true, "ace": true}
	for _, d := range Catalog {
		if seen[d.ID] {
			t.Errorf("duplicate achievement id %q", d.ID)
		}
		seen[d.ID] = true
		if !tiers[d.Tier] {
			t.Errorf("%s: unknown tier %q", d.ID, d.Tier)
		}
		if d.ExpReward <= 0 || d.CoinReward <= 0 {
			t.Errorf("%s: rewards must be positive (exp=%d coins=%d)", d.ID, d.ExpReward, d.CoinReward)
		}
		if d.Target <= 0 {
			t.Errorf("%s: target must be positive", d.ID)
		}
		if _, ok := modes[d.Param]; !ok {
			t.Errorf("%s: unknown mode param %q", d.ID, d.Param)
		}
		if d.Title.En == "" || d.Title.Th == "" || d.Desc.En == "" || d.Desc.Th == "" {
			t.Errorf("%s: bilingual title/desc required", d.ID)
		}
	}
}

func TestEvaluateUnlocksInOrder(t *testing.T) {
	s := Snapshot{
		TotalExp: 100, Level: 5, TotalCoins: 500, ModesPlayed: 4,
		Solved:     map[string]int64{"": 100, "jack": 100, "queen": 100, "king": 100, "ace": 100},
		BestStreak: map[string]int64{"": 10},
	}
	got := Evaluate(s, map[string]bool{})
	var ids []string
	for _, d := range got {
		ids = append(ids, d.ID)
	}
	want := []string{
		"solve-1", "solve-25", "solve-100",
		"streak-3", "streak-5", "streak-10",
		"level-5",
		"jack-first", "queen-first", "king-first", "ace-first",
		"jack-master", "queen-master", "king-master", "ace-master",
		"all-rounder",
		"coin-500",
	}
	if len(ids) != len(want) {
		t.Fatalf("Evaluate returned %d unlocks, want %d: %v", len(ids), len(want), ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("unlock[%d] = %s, want %s", i, ids[i], want[i])
		}
	}
}

func TestEvaluateSkipsAlreadyUnlocked(t *testing.T) {
	// 30 solves would unlock both solve-1 and solve-25, but solve-1 is
	// already banked: only solve-25 may come back
	s := Snapshot{Solved: map[string]int64{"": 30}, BestStreak: map[string]int64{}}
	got := Evaluate(s, map[string]bool{"solve-1": true})
	if len(got) != 1 || got[0].ID != "solve-25" {
		t.Fatalf("want only solve-25 pending at 30 solves, got %v", got)
	}
}

func TestBestTimeLowerIsBetter(t *testing.T) {
	d, ok := ByID("swift-15s")
	if !ok {
		t.Fatal("swift-15s missing from catalog")
	}
	// no solve yet: never unlocked, even though 0 <= target
	for _, d2 := range Evaluate(Snapshot{BestStreak: map[string]int64{}}, map[string]bool{}) {
		if d2.ID == d.ID {
			t.Fatal("best-time achievement unlocked with no solve recorded")
		}
	}
	if !unlockedWhen(d, 15000) {
		t.Error("boundary value must unlock (<=)")
	}
	if unlockedWhen(d, 15001) {
		t.Error("slower than target must not unlock")
	}
	if !unlockedWhen(d, 4000) {
		t.Error("faster than target must unlock")
	}
}

func TestRewardsScaleWithTier(t *testing.T) {
	// the reward of the top entry of each family must exceed the bottom one
	top := ByIDOr(t, "solve-5000")
	bottom := ByIDOr(t, "solve-1")
	if top.ExpReward <= bottom.ExpReward || top.CoinReward <= bottom.CoinReward {
		t.Error("legend rewards must exceed bronze rewards within a family")
	}
}

func ByIDOr(t *testing.T, id string) Def {
	t.Helper()
	d, ok := ByID(id)
	if !ok {
		t.Fatalf("%s missing from catalog", id)
	}
	return d
}
