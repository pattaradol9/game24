// 24-game solver for driving the real UI.
//
// solve24() returns a merge plan expressed in the exact card identities the
// app uses: original cards are `c0..c3`, the result of merge k is `s{k}`
// (mirroring web/src/core/checker.js). Each step is one merge: pick card a,
// arm the operator, pick card b — the merge fires on the second pick.

const gcd = (a, b) => {
  while (b) { [a, b] = [b, a % b] }
  return Math.abs(a)
}

const frac = (n, d = 1) => {
  if (d < 0) { n = -n; d = -d }
  const g = gcd(n, d) || 1
  return { n: n / g, d: d / g }
}

const fAdd = (a, b) => frac(a.n * b.d + b.n * a.d, a.d * b.d)
const fSub = (a, b) => frac(a.n * b.d - b.n * a.d, a.d * b.d)
const fMul = (a, b) => frac(a.n * b.n, a.d * b.d)
const fDiv = (a, b) => (b.n === 0 ? null : frac(a.n * b.d, a.d * b.n))

const APPLY = {
  '+': fAdd,
  '-': fSub,
  '*': fMul,
  '/': fDiv,
}

// bounds keep the search tree from exploding on nonsense intermediates
const usable = (f) => f !== null && Math.abs(f.n) <= 1e9 && f.d <= 1e6

/**
 * Find a sequence of merges reaching `target`. When `integerOnly` is set,
 * every intermediate value must be a whole number (Jack-mode style play).
 * Returns steps `[{a, op, b}]` or null.
 */
export function solve24(numbers, target = 24, integerOnly = true) {
  const start = numbers.map((n, i) => ({ id: `c${i}`, val: frac(n) }))
  const steps = []
  const found = search(start, target, steps, integerOnly)
  return found ? steps.map((s) => ({ ...s })) : null
}

function search(items, target, steps, integerOnly) {
  if (items.length === 1) {
    const f = items[0].val
    return f.d === 1 || !integerOnly ? f.n === target * f.d : false
  }
  for (let i = 0; i < items.length; i++) {
    for (let j = 0; j < items.length; j++) {
      if (i === j) continue
      for (const op of ['+', '-', '*', '/']) {
        const val = APPLY[op](items[i].val, items[j].val)
        if (!usable(val)) continue
        if (integerOnly && val.d !== 1) continue
        const rest = items.filter((_, k) => k !== i && k !== j)
        const merged = { id: `s${steps.length}`, val }
        steps.push({ a: items[i].id, op, b: items[j].id })
        rest.push(merged)
        if (search(rest, target, steps, integerOnly)) return true
        steps.pop()
        rest.pop()
      }
    }
  }
  return false
}

/**
 * Any reachable single-card value other than 24 (for the wrong-merge test).
 * Prefers integer paths; returns {target, steps} or null.
 */
export function solveNon24(numbers) {
  for (const integerOnly of [true, false]) {
    for (let target = 1; target <= 40; target++) {
      if (target === 24) continue
      const steps = solve24(numbers, target, integerOnly)
      if (steps) return { target, steps }
    }
  }
  return null
}

export const OP_ARIA = { '+': 'plus', '-': 'minus', '*': 'times', '/': 'divide' }
