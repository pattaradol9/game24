// Level / Tier / hint-quota math — must mirror server/internal/progress.

export const TIERS = ['bronze', 'silver', 'gold', 'platinum', 'diamond', 'master']

// the level ladder ends here: EXP past the cap still banks but the level
// never climbs further (mirrors progress.MaxLevel server-side)
export const MAX_LEVEL = 100

export function expForLevel(n) {
  if (n < 1) n = 1
  return 30 * (n - 1) * n
}

export function levelFromExp(exp) {
  if (exp < 0) exp = 0
  let n = Math.floor((30 + Math.sqrt(900 + 120 * exp)) / 60)
  while (expForLevel(n) > exp) n--
  while (expForLevel(n + 1) <= exp) n++
  return Math.min(MAX_LEVEL, Math.max(1, n))
}

// the level handicap on a solved hand's score: +5% per level above the
// first (Lv.1 ×1.00 → Lv.100 ×5.95); mirrors progress.ScoreForHand
export function scoreMultiplier(lv) {
  const l = Math.min(MAX_LEVEL, Math.max(1, lv))
  return 1 + 0.05 * (l - 1)
}

export function tierFromLevel(lv) {
  return Math.min(5, Math.max(0, Math.floor(lv / 20)))
}

export function tierName(lv) {
  return TIERS[tierFromLevel(lv)]
}

export function tierBonusQuota(tier) {
  if (tier < 0 || tier > 5) return 0
  return Math.floor(tier / 2)
}

// hints per hand in single player
export function singleHintQuota(lv) {
  return 1 + tierBonusQuota(tierFromLevel(lv))
}

export function levelProgress(exp) {
  const lv = levelFromExp(exp)
  const base = expForLevel(lv)
  // at the cap the ladder ends: forNext reads 0 so the bar renders full
  if (lv >= MAX_LEVEL) return { lv, into: exp - base, forNext: 0 }
  const next = expForLevel(lv + 1)
  return { lv, into: exp - base, forNext: next - base }
}
