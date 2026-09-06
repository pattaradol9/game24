// Level / Tier / hint-quota math — must mirror server/internal/progress.

export const TIERS = ['bronze', 'silver', 'gold', 'platinum', 'diamond', 'master']

export function expForLevel(n) {
  if (n < 1) n = 1
  return 30 * (n - 1) * n
}

export function levelFromExp(exp) {
  if (exp < 0) exp = 0
  let n = Math.floor((30 + Math.sqrt(900 + 120 * exp)) / 60)
  while (expForLevel(n) > exp) n--
  while (expForLevel(n + 1) <= exp) n++
  return Math.max(1, n)
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
  const next = expForLevel(lv + 1)
  return { lv, into: exp - base, forNext: next - base }
}
