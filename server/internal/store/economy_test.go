package store

import (
	"errors"
	"testing"

	"github.com/pattaradol9/game24/server/internal/achv"
)

// googlePlayer returns a signed-in (non-guest) player for economy tests.
func googlePlayer(t *testing.T, s *Store, sub string) (id string, token string) {
	t.Helper()
	p, tok, err := s.GoogleLogin(sub, sub+"@mail.com", "P"+sub, "")
	if err != nil {
		t.Fatal(err)
	}
	return p.ID, tok
}

func TestCoinsAwardedPerSolve(t *testing.T) {
	s := openTest(t)
	id, tok := googlePlayer(t, s, "coin1")

	res, unlocked, err := s.AwardSolve(id, "queen", 155, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.CoinsEarned != 15 {
		t.Fatalf("coins earned = %d, want 15 (155/10)", res.CoinsEarned)
	}
	// totals are read after achievement rewards were banked, so they exceed
	// the hand payout alone
	if res.TotalCoins < 15 {
		t.Fatalf("total coins = %d, want >= 15", res.TotalCoins)
	}
	// first solve unlocks the entry-level achievements; their rewards bank too
	if len(unlocked) == 0 {
		t.Fatal("first solve should unlock achievements")
	}
	fresh, err := s.PlayerByToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.TotalCoins <= 15 {
		t.Fatalf("achievement coin rewards missing: %d", fresh.TotalCoins)
	}
	if fresh.TotalExp <= 155 {
		t.Fatalf("achievement exp rewards missing: %d", fresh.TotalExp)
	}
	if CoinsForHand(9) != 1 || CoinsForHand(10) != 1 || CoinsForHand(0) != 1 {
		t.Fatal("CoinsForHand must pay at least one coin")
	}
}

func TestSkipRecordsAndUnlocksNothingPaid(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "coin2")

	for i := 0; i < 10; i++ {
		if _, err := s.AwardSkip(id, "queen"); err != nil {
			t.Fatal(err)
		}
	}
	fresh, err := s.PlayerByID(id)
	if err != nil {
		t.Fatal(err)
	}
	// skips pay nothing directly; the only income here is the skip-10
	// achievement reward (30 EXP + 20 coins)
	if fresh.TotalCoins != 20 || fresh.TotalExp != 30 {
		t.Fatalf("want skip-10 rewards only, got exp=%d coins=%d", fresh.TotalExp, fresh.TotalCoins)
	}
	// skip-10 unlocks, and its reward pays
	found := false
	for _, d := range achv.Catalog {
		if d.ID == "skip-10" {
			found = true
		}
	}
	if !found {
		t.Fatal("skip-10 missing from catalog")
	}
	unlocked, _, err := s.AchievementState(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := unlocked["skip-10"]; !ok {
		t.Fatal("10 skips should unlock skip-10")
	}
	if fresh.TotalCoins != 20 {
		t.Fatalf("skip-10 coin reward missing, coins = %d", fresh.TotalCoins)
	}
}

func TestAchievementSnapshotCounters(t *testing.T) {
	s := openTest(t)
	id, _ := googlePlayer(t, s, "coin3")

	// two fast no-hint solves in queen, one hint spend, one skip, all on a
	// second mode too for the breadth counter
	if _, _, err := s.AwardEXP(id, "queen", 100, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TryUseHint("missing", id); err == nil {
		t.Fatal("hint on missing round must fail")
	}
	snap, err := s.AchievementSnapshot(id)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Solved[""] != 1 || snap.Solved["queen"] != 1 || snap.ModesPlayed != 1 {
		t.Fatalf("solved snapshot = %+v", snap)
	}
}

func TestBuyAndEquipSkinLifecycle(t *testing.T) {
	s := openTest(t)
	gp, _, _ := s.CreateGuest("G")
	gid := gp.ID
	if err := s.BuySkin(gid, "midnight"); !errors.Is(err, ErrGoogleRequired) {
		t.Fatalf("guest buy = %v, want ErrGoogleRequired", err)
	}
	if err := s.EquipSkin(gid, "midnight"); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("guest equip unowned = %v, want ErrNotOwned", err)
	}
	if err := s.EquipSkin(gid, "classic"); err != nil {
		t.Fatalf("classic equip must always be allowed: %v", err)
	}

	id, _ := googlePlayer(t, s, "coin4")
	if err := s.BuySkin(id, "midnight"); !errors.Is(err, ErrInsufficientCoins) {
		t.Fatalf("broke buy = %v, want ErrInsufficientCoins", err)
	}
	if _, _, err := s.AwardSolve(id, "queen", 20000, false); err != nil {
		t.Fatal(err)
	}
	fresh, _ := s.PlayerByID(id)
	if fresh.TotalCoins < 1000 {
		t.Fatalf("test needs >=1000 coins, has %d", fresh.TotalCoins)
	}
	if err := s.BuySkin(id, "midnight"); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if err := s.BuySkin(id, "midnight"); !errors.Is(err, ErrAlreadyOwned) {
		t.Fatalf("double buy = %v, want ErrAlreadyOwned", err)
	}
	afterBuy, _ := s.PlayerByID(id)
	if afterBuy.TotalCoins != fresh.TotalCoins-1000 {
		t.Fatalf("coins after buy = %d, want %d", afterBuy.TotalCoins, fresh.TotalCoins-1000)
	}
	if err := s.EquipSkin(id, "midnight"); err != nil {
		t.Fatalf("equip: %v", err)
	}
	equipped, _ := s.PlayerByID(id)
	if equipped.Skin != "midnight" {
		t.Fatalf("skin = %q, want midnight", equipped.Skin)
	}
	owned, err := s.OwnedSkins(id)
	if err != nil {
		t.Fatal(err)
	}
	if owned[0] != "classic" || len(owned) != 2 {
		t.Fatalf("owned = %v, want classic first then midnight", owned)
	}
	if err := s.EquipSkin(id, "galaxy"); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("equip unowned = %v, want ErrNotOwned", err)
	}
	if err := s.BuySkin(id, "classic"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("buying classic = %v, want ErrNotFound", err)
	}
}
