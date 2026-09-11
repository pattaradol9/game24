package progress

import "testing"

func TestLevelFromExp(t *testing.T) {
	tests := []struct {
		exp  int64
		want int64
	}{
		{0, 1}, {59, 1}, {60, 2}, {61, 2}, {179, 2}, {180, 3},
		{ExpForLevel(100), 100}, {ExpForLevel(100) - 1, 99},
		// the ladder caps at 100: EXP past the cap keeps banking but the
		// level never climbs further
		{ExpForLevel(101), 100}, {ExpForLevel(150), 100}, {1 << 40, 100},
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
	// at the cap the ladder ends: expForNext reads 0 so the bar renders full
	lv, into, next = LevelProgress(ExpForLevel(100) + 5000)
	if lv != 100 || into != 5000 || next != 0 {
		t.Fatalf("LevelProgress(at cap) = lv%d into%d next%d, want lv100 into5000 next0", lv, into, next)
	}
}

func TestScoreForHand(t *testing.T) {
	if got := ScoreForHand(100, 1); got != 100 {
		t.Fatalf("ScoreForHand(100, 1) = %d, want 100", got)
	}
	if got := ScoreForHand(100, 11); got != 150 {
		t.Fatalf("ScoreForHand(100, 11) = %d, want 150", got)
	}
	if got := ScoreForHand(100, 100); got != 595 {
		t.Fatalf("ScoreForHand(100, 100) = %d, want 595", got)
	}
	if got := ScoreForHand(100, 250); got != 595 {
		t.Fatalf("ScoreForHand(100, 250) = %d, want 595 (level clamped)", got)
	}
	if got := ScoreForHand(7, 13); got != 11 { // 7 × 1.60 = 11.2 → 11
		t.Fatalf("ScoreForHand(7, 13) = %d, want 11", got)
	}
	if m := ScoreMultiplier(13); m != 1.6 {
		t.Fatalf("ScoreMultiplier(13) = %v, want 1.6", m)
	}
}

func TestExpForHand(t *testing.T) {
	tests := []struct {
		remaining, modeMult, want int64
	}{
		{45, 2, 61}, // queen: (8 + 22.5) × 2 = 61
		{90, 1, 53}, // jack: (8 + 45) × 1 = 53
		{0, 5, 40},  // ace at the buzzer: the solve's flat base carries it
		{80, 2, 96}, // (8 + 40) × 2
		{13, 3, 44}, // rounding: (8 + 6.5) × 3 = 43.5 → 44
	}
	for _, tc := range tests {
		if got := ExpForHand(tc.remaining, tc.modeMult); got != tc.want {
			t.Errorf("ExpForHand(%d, %d) = %d, want %d", tc.remaining, tc.modeMult, got, tc.want)
		}
	}
	// the point of the split: EXP never inherits the level handicap, so the
	// two currencies drift apart as the player levels (score here is what
	// ScoreForHand would pay the same hand at the cap)
	if ScoreForHand((10+45)*2, 100) == ExpForHand(45, 2) {
		t.Fatal("score and exp curves coincide; they must stay distinct")
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
