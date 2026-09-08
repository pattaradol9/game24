package progress

import "testing"

func TestLevelFromExp(t *testing.T) {
	tests := []struct {
		exp  int64
		want int64
	}{
		{0, 1}, {59, 1}, {60, 2}, {61, 2}, {179, 2}, {180, 3},
		{ExpForLevel(100), 100}, {ExpForLevel(100) - 1, 99}, {ExpForLevel(101), 101},
	}
	for _, tc := range tests {
		if got := LevelFromExp(tc.exp); got != tc.want {
			t.Errorf("LevelFromExp(%d) = %d, want %d", tc.exp, got, tc.want)
		}
	}
}

func TestExpForLevel100(t *testing.T) {
	if got := ExpForLevel(100); got != 297000 {
		t.Fatalf("ExpForLevel(100) = %d, want 297000", got)
	}
}

func TestTierFromLevel(t *testing.T) {
	tests := []struct {
		lv   int64
		want int
	}{
		{1, TierBronze}, {19, TierBronze}, {20, TierSilver}, {39, TierSilver},
		{40, TierGold}, {59, TierGold}, {60, TierPlatinum}, {79, TierPlatinum},
		{80, TierDiamond}, {99, TierDiamond}, {100, TierMaster}, {150, TierMaster},
	}
	for _, tc := range tests {
		got := TierFromLevel(tc.lv)
		if got != tc.want {
			t.Errorf("TierFromLevel(%d) = %d (%s), want %d", tc.lv, got, TierName(got), tc.want)
		}
	}
}

func TestSingleHintQuota(t *testing.T) {
	// tier order: bronze, silver, gold, platinum, diamond, master
	want := []int{1, 1, 2, 2, 3, 3}
	for tier, w := range want {
		if got := SingleHintQuota(tier); got != w {
			t.Errorf("SingleHintQuota(%s) = %d, want %d", TierName(tier), got, w)
		}
	}
}

func TestLevelProgress(t *testing.T) {
	lv, into, next := LevelProgress(60)
	if lv != 2 || into != 0 || next != 120 {
		t.Fatalf("LevelProgress(60) = lv%d into%d next%d", lv, into, next)
	}
	lv, into, next = LevelProgress(120)
	if lv != 2 || into != 60 || next != 120 {
		t.Fatalf("LevelProgress(120) = lv%d into%d next%d", lv, into, next)
	}
}

func TestTierFromName(t *testing.T) {
	for i := 0; i < numTiers; i++ {
		got, ok := TierFromName(TierName(i))
		if !ok || got != i {
			t.Errorf("TierFromName(%q) = %d, %v; want %d, true", TierName(i), got, ok, i)
		}
	}
	if _, ok := TierFromName("diamonds"); ok {
		t.Error("TierFromName should reject unknown names")
	}
	if _, ok := TierFromName(""); ok {
		t.Error("TierFromName should reject the empty name")
	}
}
