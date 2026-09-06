package progress

import "math"

// ExpForLevel returns the cumulative EXP required to reach level n (n >= 1).
func ExpForLevel(n int64) int64 {
	if n < 1 {
		n = 1
	}
	return 30 * (n - 1) * n
}

// LevelFromExp maps total EXP to a level, always at least 1.
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

// SingleHintQuota: hints per hand in single player, tier bonus included.
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

// LevelProgress describes progress from level lv to lv+1.
func LevelProgress(exp int64) (lv int64, expInto, expForNext int64) {
	lv = LevelFromExp(exp)
	base := ExpForLevel(lv)
	next := ExpForLevel(lv + 1)
	return lv, exp - base, next - base
}
