// Exact rational arithmetic for the 24 game (mirrors server/internal/game/fraction.go)

function gcd(a, b) {
  while (b) [a, b] = [b, a % b]
  return Math.abs(a)
}

export function frac(num, den = 1) {
  if (den === 0) throw new Error('zero denominator')
  if (den < 0) { num = -num; den = -den }
  const g = gcd(num, den) || 1
  return { num: num / g, den: den / g }
}

export const int = (n) => frac(n, 1)

export function add(a, b) { return frac(a.num * b.den + b.num * a.den, a.den * b.den) }
export function sub(a, b) { return frac(a.num * b.den - b.num * a.den, a.den * b.den) }
export function mul(a, b) { return frac(a.num * b.num, a.den * b.den) }
export function div(a, b) {
  if (b.num === 0) throw new Error('division by zero')
  return frac(a.num * b.den, a.den * b.num)
}

export function apply(op, a, b) {
  switch (op) {
    case '+': return add(a, b)
    case '-': return sub(a, b)
    case '*': return mul(a, b)
    case '/': return div(a, b)
    default: throw new Error(`unknown op ${op}`)
  }
}

export const isInt = (f) => f.den === 1
export const is24 = (f) => f.num === 24 && f.den === 1

export function format(f) {
  return isInt(f) ? String(f.num) : `${f.num}/${f.den}`
}
