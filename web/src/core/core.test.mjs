import { test } from 'node:test'
import assert from 'node:assert/strict'
import { frac, int, apply, format, is24 } from './fraction.js'
import { newHand, pickCard, setOperator, undo } from './checker.js'
import { expForLevel, levelFromExp, tierFromLevel, singleHintQuota, levelProgress, tierBonusQuota } from './progress.js'

test('fraction: 5 * (5 - 1/5) = 24', () => {
  const one5th = apply('/', int(1), int(5))
  const result = apply('*', int(5), apply('-', int(5), one5th))
  assert.equal(is24(result), true)
  assert.equal(format(one5th), '1/5')
})

test('fraction: division by zero throws', () => {
  assert.throws(() => apply('/', int(1), int(0)))
})

test('fraction: format simplifies', () => {
  assert.equal(format(apply('/', int(8), int(8))), '1')
  assert.equal(format(apply('/', int(1), int(2))), '1/2')
  assert.equal(format(frac(4, -8)), '-1/2')
})

test('checker: number→operator→number merges instantly (8/8 then 7-1)', () => {
  const st = newHand([4, 7, 8, 8])
  pickCard(st, st.cards[2])            // 8
  setOperator(st, '/')
  assert.equal(st.operator, '/')
  pickCard(st, st.cards[3])            // 8 → merges immediately
  assert.equal(st.cards.length, 3)
  assert.equal(st.selection, null)
  assert.equal(st.operator, null)
  assert.deepEqual(st.steps, [{ left: { card: 2 }, right: { card: 3 }, op: '/' }])
})

test('checker: full win on [4,7,8,8] with new flow', () => {
  const st = newHand([4, 7, 8, 8])
  pickCard(st, st.cards[2]); setOperator(st, '/'); pickCard(st, st.cards[3])   // 8/8 = 1
  // board now: [4, 7, 1]
  pickCard(st, st.cards[1]); setOperator(st, '-'); pickCard(st, st.cards[2])   // 7-1 = 6
  // board now: [4, 6]
  pickCard(st, st.cards[1]); setOperator(st, '*'); pickCard(st, st.cards[0])   // 6*4 = 24
  assert.equal(st.won, true)
  assert.equal(st.cards.length, 1)
  assert.deepEqual(st.steps, [
    { left: { card: 2 }, right: { card: 3 }, op: '/' },
    { left: { card: 1 }, right: { step: 0 }, op: '-' },
    { left: { step: 1 }, right: { card: 0 }, op: '*' },
  ])
})

test('checker: operator before number is ignored', () => {
  const st = newHand([1, 2, 3, 4])
  setOperator(st, '+')
  assert.equal(st.operator, null)
  pickCard(st, st.cards[0])
  setOperator(st, '+')
  assert.equal(st.operator, '+')
})

test('checker: tapping the picked card cancels selection and operator', () => {
  const st = newHand([4, 7, 8, 8])
  pickCard(st, st.cards[2])
  setOperator(st, '/')
  pickCard(st, st.cards[2])
  assert.equal(st.selection, null)
  assert.equal(st.operator, null)
})

test('checker: undo restores prior state', () => {
  const st = newHand([4, 7, 8, 8])
  pickCard(st, st.cards[2]); setOperator(st, '/'); pickCard(st, st.cards[3])
  assert.equal(st.cards.length, 3)
  undo(st)
  assert.equal(st.cards.length, 4)
  assert.deepEqual(st.steps, [])
  assert.equal(st.won, false)
  assert.equal(st.selection, null)
})

test('checker: 5*(5-1/5) wins via UI flow (fraction path)', () => {
  const st = newHand([1, 5, 5, 5])
  pickCard(st, st.cards[0]); setOperator(st, '/'); pickCard(st, st.cards[1])   // 1/5
  // board now: [5, 5, 1/5]
  pickCard(st, st.cards[0]); setOperator(st, '-'); pickCard(st, st.cards[2])   // 5 - 1/5 = 24/5
  // board now: [5, 24/5]
  assert.equal(format(st.cards[1].value), '24/5')
  pickCard(st, st.cards[0]); setOperator(st, '*'); pickCard(st, st.cards[1])   // 5 * 24/5 = 24
  assert.equal(st.won, true)
})

test('progress: level curve matches server', () => {
  assert.equal(expForLevel(100), 297000)
  assert.equal(levelFromExp(0), 1)
  assert.equal(levelFromExp(59), 1)
  assert.equal(levelFromExp(60), 2)
  assert.equal(levelFromExp(296999), 99)
  assert.equal(levelFromExp(297000), 100)
})

test('progress: tier ladder every 20 levels', () => {
  assert.equal(tierFromLevel(1), 0)
  assert.equal(tierFromLevel(20), 1)
  assert.equal(tierFromLevel(40), 2)
  assert.equal(tierFromLevel(60), 3)
  assert.equal(tierFromLevel(80), 4)
  assert.equal(tierFromLevel(100), 5)
  assert.equal(tierFromLevel(150), 5)
})

test('progress: hint quota per tier', () => {
  assert.deepEqual([1, 20, 40, 60, 80, 100].map((lv) => singleHintQuota(lv)), [1, 1, 2, 2, 3, 3])
  assert.equal(tierBonusQuota(1), 0)
})

test('progress: level progress fields', () => {
  const p = levelProgress(60)
  assert.deepEqual(p, { lv: 2, into: 0, forNext: 120 })
})
