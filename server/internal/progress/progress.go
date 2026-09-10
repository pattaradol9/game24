package progress

import "math"

// MaxLevel caps the level ladder for now: EXP past the cap still banks and
// shows on the bar, but the level itself never climbs past 100 (the top of
// the tier table, and the level-100 "Master of 24" achievement).
const MaxLevel = 100

// ExpForLevel returns the cumulative EXP required to reach level n (n >= 1).
func ExpForLevel(n int64) int64 {
	if n < 1 {
		n = 1
	}
	return 30 * (n - 1) * n
}

// LevelFromExp maps total EXP to a level, always at least 1, never past
// MaxLevel.
func LevelFromExp(exp int64) int64 {
	if exp < 0 {
		exp = 0
	}
	// Solve 30n(n-1) <= exp  =>  n <= (30 + sqrt(900 + 120*exp)) / 60
	disc := 900 + 120*exp
	if disc < 0 {
		return 1
	}
	n := (int64(30) + int64(math.Sqrt(float64(disc)))) / 60
	for ExpForLevel(n) > exp {
		n--
	}
	for ExpForLevel(n+1) <= exp {
		n++
	}
	if n < 1 {
		n = 1
	}
	if n > MaxLevel {
		n = MaxLevel
	}
	return n
}

// Tier tiers: 0=Bronze, 1=Silver, 2=Gold, 3=Platinum, 4=Diamond, 5=Master.
const (
	TierBronze = iota
	TierSilver
	TierGold
	TierPlatinum
	TierDiamond
	TierMaster
	numTiers
)

// TierFromLevel: one tier per 20 levels (Silver at 20, Master at 100), capped.
func TierFromLevel(lv int64) int {
	t := int(lv / 20)
	if t < 0 {
		t = 0
	}
	if t >= numTiers {
		t = numTiers - 1
	}
	return t
}

var tierNames = [...]string{"bronze", "silver", "gold", "platinum", "diamond", "master"}

func TierName(t int) string {
	if t < 0 || t >= numTiers {
		return tierNames[0]
	}
	return tierNames[t]
}

// TierFromName parses a tier name ("bronze" … "master"); ok is false for
// anything else. Backs the admin tier override stored on the player.
func TierFromName(name string) (int, bool) {
	for i, n := range tierNames {
		if n == name {
			return i, true
		}
	}
	return 0, false
}

// SingleHintQuota: hint budget for one single-player session (a visit to the
// game, spanning all its hands), tier bonus included.
func SingleHintQuota(tier int) int {
	return 1 + TierBonusQuota(tier)
}

// TierBonusQuota: the raw tier bonus (+0, +0, +1, +1, +2, +2).
func TierBonusQuota(tier int) int {
	if tier < 0 || tier >= numTiers {
		return 0
	}
	return tier / 2
}

// LevelProgress describes progress from level lv to lv+1. At the cap the
// ladder ends: expForNext reads 0 so the bar renders full.
func LevelProgress(exp int64) (lv int64, expInto, expForNext int64) {
	lv = LevelFromExp(exp)
	base := ExpForLevel(lv)
	if lv >= MaxLevel {
		return lv, exp - base, 0
	}
	next := ExpForLevel(lv + 1)
	return lv, exp - base, next - base
}

// ScoreMultiplier is the level handicap on a solved hand's score: +5% per
// level above the first, so a Lv.100 veteran banks 5.95× what a fresh face
// does. Display-facing helper pairing with ScoreForHand.
func ScoreMultiplier(lv int64) float64 {
	return 1 + 0.05*float64(clampLevel(lv)-1)
}

// ScoreForHand multiplies a solved hand's base score by the level handicap.
// Deterministic, applied server-side where the score is computed; the score
// is the session scoreboard figure only — EXP pays from its own curve below.
func ScoreForHand(base, lv int64) int64 {
	return int64(math.Round(float64(base) * ScoreMultiplier(lv)))
}

// ExpForHand is a solved hand's EXP payout base, deliberately a different
// curve from the score: solving pays a solid flat base, speed counts at half
// weight, the mode scales it — and the level handicap does NOT compound it
// (levels already raise the score; letting them raise EXP too would make
// progression feed itself). Coins follow this figure, 10:1.
func ExpForHand(remaining, modeMult int64) int64 {
	return int64(math.Round((8 + float64(remaining)/2) * float64(modeMult)))
}

func clampLevel(lv int64) int64 {
	if lv < 1 {
		return 1
	}
	if lv > MaxLevel {
		return MaxLevel
	}
	return lv
}
