package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/pattaradol9/game24/server/internal/crypto"
)

// TestLegacyExpBoostMigratesIntoBoosts rolls a store back to the pre-coins
// schema (single exp_boost table), then reopens it — migrate must carry the
// row over as the "exp" kind and leave no exp_boost behind.
func TestLegacyExpBoostMigratesIntoBoosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	cr, err := crypto.New(testKey)
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(path, cr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TABLE boosts`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TABLE exp_boost (
		id           INTEGER PRIMARY KEY CHECK (id = 1),
		multiplier   REAL NOT NULL DEFAULT 2,
		duration_min INTEGER NOT NULL DEFAULT 60,
		label        TEXT NOT NULL DEFAULT '',
		enabled      INTEGER NOT NULL DEFAULT 0,
		starts_at    TEXT,
		ends_at      TEXT,
		updated_by   TEXT NOT NULL DEFAULT '',
		updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO exp_boost (id, multiplier, duration_min, label, enabled, starts_at, ends_at, updated_by)
		VALUES (1, 3, 45, 'Legacy', 1, datetime('now'), datetime('now', '+45 minutes'), 'admin:old')`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := Open(path, cr)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	b, err := s2.BoostState(BoostKindExp)
	if err != nil {
		t.Fatal(err)
	}
	if b.Multiplier != 3 || b.DurationMin != 45 || b.Label != "Legacy" || !b.Enabled {
		t.Fatalf("migrated exp boost = %+v, want the legacy row", b)
	}
	if _, active := s2.ActiveBoost(BoostKindExp); !active {
		t.Fatal("a still-open legacy window must survive the migration")
	}
	// the coins kind is seeded fresh beside it, and the old table is gone
	coins, err := s2.BoostState(BoostKindCoins)
	if err != nil || coins.Multiplier != 2 || coins.Enabled {
		t.Fatalf("seeded coins boost = %+v err=%v, want a fresh disarmed draft", coins, err)
	}
	var n int
	if err := s2.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'exp_boost'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("exp_boost must be gone after migration (count=%d err=%v)", n, err)
	}
}

func TestBoostDefaultsAndValidation(t *testing.T) {
	s := openTest(t)

	for _, kind := range []BoostKind{BoostKindExp, BoostKindCoins} {
		b, err := s.BoostState(kind)
		if err != nil {
			t.Fatal(err)
		}
		if b.Multiplier != 2 || b.DurationMin != 60 || b.Enabled || !b.StartsAt.IsZero() {
			t.Fatalf("fresh %s boost = %+v, want 2x/60min disarmed draft", kind, b)
		}
		if _, active := s.ActiveBoost(kind); active {
			t.Fatalf("a fresh %s draft must not be active", kind)
		}
	}
	if _, err := s.BoostState("magic"); !errors.Is(err, ErrBoostKind) {
		t.Fatalf("unknown kind = %v, want ErrBoostKind", err)
	}

	for _, tc := range []struct {
		m float64
		d int
	}{
		{0, 60},
		{-2, 60},
		{float64(BoostMultiplierMax) + 0.5, 60},
		{2, 0},
		{2, BoostDurationMax + 1},
	} {
		if _, err := s.SetBoostConfig("admin:x", BoostKindExp, tc.m, tc.d, ""); !errors.Is(err, ErrBoostRange) {
			t.Fatalf("SetBoostConfig(%v, %d) = %v, want ErrBoostRange", tc.m, tc.d, err)
		}
	}
}

func TestExpBoostAppliesToSolveEXPOnly(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "boost1")

	// base payout before anything is armed
	if _, _, err := s.AwardEXP(id, "queen", 100, true); err != nil {
		t.Fatal(err)
	}
	st, err := s.ModeStats(id)
	if err != nil {
		t.Fatal(err)
	}
	if st["queen"].Exp != 100 {
		t.Fatalf("queen exp = %d, want 100 (no boost)", st["queen"].Exp)
	}

	if _, err := s.SetBoostConfig("admin:x", BoostKindExp, 2.5, 30, "Weekend"); err != nil {
		t.Fatal(err)
	}
	b, err := s.EnableBoost("admin:x", BoostKindExp)
	if err != nil {
		t.Fatal(err)
	}
	if !b.Enabled || b.Multiplier != 2.5 || b.DurationMin != 30 || b.Label != "Weekend" {
		t.Fatalf("armed boost = %+v", b)
	}
	ab, active := s.ActiveBoost(BoostKindExp)
	if !active || ab.Multiplier != 2.5 {
		t.Fatalf("active boost = %+v, active=%v", ab, active)
	}
	// the window closes in ~30 minutes (seconds-resolution arithmetic)
	if left := time.Until(ab.EndsAt); left < 29*time.Minute || left > 31*time.Minute {
		t.Fatalf("boost ends in %v, want ~30m", left)
	}
	// the coin boost is a separate row and stays idle
	if _, active := s.ActiveBoost(BoostKindCoins); active {
		t.Fatal("coin boost must not inherit the exp boost's window")
	}

	res, _, err := s.AwardSolve(id, "queen", 155, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := BoostAmount(155, 2.5); res.ExpEarned != want || want != 388 {
		t.Fatalf("exp earned = %d, want %d (155×2.5 rounded)", res.ExpEarned, want)
	}
	// coins keep paying on the base points
	if res.CoinsEarned != 15 {
		t.Fatalf("coins earned = %d, want 15 (155/10, unboosted)", res.CoinsEarned)
	}
	st, err = s.ModeStats(id)
	if err != nil {
		t.Fatal(err)
	}
	if st["queen"].Exp != 100+388 {
		t.Fatalf("queen exp = %d, want 488", st["queen"].Exp)
	}

	// disable stops the payout at once
	if _, err := s.DisableBoost("admin:x", BoostKindExp); err != nil {
		t.Fatal(err)
	}
	if _, active := s.ActiveBoost(BoostKindExp); active {
		t.Fatal("disabled boost must not be active")
	}
	res, _, err = s.AwardSolve(id, "queen", 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpEarned != 100 {
		t.Fatalf("exp earned after disable = %d, want 100", res.ExpEarned)
	}
}

func TestCoinBoostAppliesToSolveCoinsOnly(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "boost4")

	if _, err := s.SetBoostConfig("admin:x", BoostKindCoins, 3, 60, "Coin rain"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableBoost("admin:x", BoostKindCoins); err != nil {
		t.Fatal(err)
	}

	// 155 base points → 15 base coins → ×3 = 45; EXP pays unboosted
	res, _, err := s.AwardSolve(id, "queen", 155, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpEarned != 155 {
		t.Fatalf("exp earned = %d, want 155 (unboosted)", res.ExpEarned)
	}
	if want := BoostAmount(15, 3); res.CoinsEarned != want || want != 45 {
		t.Fatalf("coins earned = %d, want %d (15×3)", res.CoinsEarned, want)
	}

	// the boost multiplier must not distort CoinsForHand's floor: a small
	// hand pays at least one base coin, so 4×1 still rounds to 4
	res, _, err = s.AwardSolve(id, "queen", 9, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.CoinsEarned != 3 {
		t.Fatalf("small-hand coins = %d, want 3 (1 base coin ×3)", res.CoinsEarned)
	}
}

func TestBothBoostsStackIndependently(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "boost5")

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
	res, _, err := s.AwardSolve(id, "queen", 200, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpEarned != 400 || res.CoinsEarned != 40 {
		t.Fatalf("payout = exp %d / coins %d, want 400 / 40", res.ExpEarned, res.CoinsEarned)
	}
	// stopping one kind leaves the other running
	if _, err := s.DisableBoost("admin:x", BoostKindCoins); err != nil {
		t.Fatal(err)
	}
	res, _, err = s.AwardSolve(id, "queen", 200, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExpEarned != 400 || res.CoinsEarned != 20 {
		t.Fatalf("payout = exp %d / coins %d, want 400 / 20", res.ExpEarned, res.CoinsEarned)
	}
}

func TestBoostExpiryAndReArm(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "boost2")

	if _, err := s.SetBoostConfig("admin:x", BoostKindExp, 3, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableBoost("admin:x", BoostKindExp); err != nil {
		t.Fatal(err)
	}
	// simulate the window closing: pull ends_at into the past
	if _, err := s.db.Exec(`UPDATE boosts SET ends_at = datetime('now', '-1 minute') WHERE kind = 'exp'`); err != nil {
		t.Fatal(err)
	}
	if _, active := s.ActiveBoost(BoostKindExp); active {
		t.Fatal("expired boost must not be active")
	}
	st, _, _, err := s.awardEXP(id, "queen", 50, true)
	if err != nil {
		t.Fatal(err)
	}
	if st.Exp != 50 {
		t.Fatalf("exp after expiry = %d, want 50 (base)", st.Exp)
	}

	// re-arming starts a fresh window from now
	if _, err := s.EnableBoost("admin:x", BoostKindExp); err != nil {
		t.Fatal(err)
	}
	ab, active := s.ActiveBoost(BoostKindExp)
	if !active {
		t.Fatal("re-armed boost must be active")
	}
	if left := time.Until(ab.EndsAt); left < 59*time.Second || left > 61*time.Second {
		t.Fatalf("re-armed boost ends in %v, want ~1m", left)
	}
	if _, _, _, err := s.awardEXP(id, "queen", 50, true); err != nil {
		t.Fatal(err)
	}
	sts, err := s.ModeStats(id)
	if err != nil {
		t.Fatal(err)
	}
	if sts["queen"].Exp != 50+150 {
		t.Fatalf("queen exp = %d, want 200 (50 base + 50×3)", sts["queen"].Exp)
	}
}

func TestBoostAmountRounding(t *testing.T) {
	if got := BoostAmount(25, 1.5); got != 38 {
		t.Fatalf("BoostAmount(25, 1.5) = %d, want 38 (half rounds away from zero)", got)
	}
	if got := BoostAmount(101, 2); got != 202 {
		t.Fatalf("BoostAmount(101, 2) = %d, want 202", got)
	}
	if got := BoostAmount(7, 1); got != 7 {
		t.Fatalf("BoostAmount(7, 1) = %d, want 7 (1× is a no-op)", got)
	}
}

func TestBoostConfigEditableWhileActive(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "boost3")

	if _, err := s.SetBoostConfig("admin:x", BoostKindExp, 2, 60, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnableBoost("admin:x", BoostKindExp); err != nil {
		t.Fatal(err)
	}
	// retune mid-event: later hands pay the new multiplier
	if _, err := s.SetBoostConfig("admin:y", BoostKindExp, 4, 120, "Double double"); err != nil {
		t.Fatal(err)
	}
	b, err := s.BoostState(BoostKindExp)
	if err != nil {
		t.Fatal(err)
	}
	if b.Multiplier != 4 || b.DurationMin != 120 || b.UpdatedBy != "admin:y" {
		t.Fatalf("retuned boost = %+v", b)
	}
	// the already-running window is untouched by a config save
	abBefore, _ := s.ActiveBoost(BoostKindExp)
	if _, err := s.SetBoostConfig("admin:z", BoostKindExp, 4, 120, ""); err != nil {
		t.Fatal(err)
	}
	abAfter, active := s.ActiveBoost(BoostKindExp)
	if !active || !abAfter.EndsAt.Equal(abBefore.EndsAt) {
		t.Fatalf("config save must not move the window: before=%+v after=%+v", abBefore, abAfter)
	}
	if _, _, _, err := s.awardEXP(id, "queen", 10, true); err != nil {
		t.Fatal(err)
	}
	sts, err := s.ModeStats(id)
	if err != nil {
		t.Fatal(err)
	}
	if sts["queen"].Exp != 40 {
		t.Fatalf("queen exp = %d, want 40 (10×4)", sts["queen"].Exp)
	}
	// both rows answer the overview in a stable order
	all, err := s.AllBoosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Kind != BoostKindExp || all[1].Kind != BoostKindCoins {
		t.Fatalf("AllBoosts = %+v", all)
	}
}
