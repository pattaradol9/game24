import { test } from 'node:test'
import assert from 'node:assert/strict'
import { frac, int, apply, format, is24 } from './fraction.js'
import { newHand, pickCard, setOperator, undo } from './checker.js'
import { expForLevel, levelFromExp, tierFromLevel, singleHintQuota, levelProgress, tierBonusQuota, scoreMultiplier, MAX_LEVEL } from './progress.js'

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
  // board now: [1/5, 5, 5] — the result sits at the pair's midpoint
  pickCard(st, st.cards[2]); setOperator(st, '-'); pickCard(st, st.cards[0])   // 5 - 1/5 = 24/5
  // board now: [24/5, 5]
  assert.equal(format(st.cards[0].value), '24/5')
  pickCard(st, st.cards[1]); setOperator(st, '*'); pickCard(st, st.cards[0])   // 5 * 24/5 = 24
  assert.equal(st.won, true)
})

test('checker: merged card lands at the midpoint of its pair', () => {
  // adjacent pair in the middle: result centers between the survivors
  const st = newHand([2, 10, 12, 4])
  pickCard(st, st.cards[1]); setOperator(st, '+'); pickCard(st, st.cards[2])   // 10+12 = 22
  assert.deepEqual(st.cards.map((c) => c.display), ['2', '22', '4'])

  // adjacent pair at the left edge: result takes the left flank
  const leftEdge = newHand([2, 10, 12, 4])
  pickCard(leftEdge, leftEdge.cards[0]); setOperator(leftEdge, '+'); pickCard(leftEdge, leftEdge.cards[1])   // 2+10 = 12
  assert.deepEqual(leftEdge.cards.map((c) => c.display), ['12', '12', '4'])

  // non-adjacent pair: result centers across the whole vacated span
  const spread = newHand([2, 10, 12, 4])
  pickCard(spread, spread.cards[0]); setOperator(spread, '+'); pickCard(spread, spread.cards[3])   // 2+4 = 6
  assert.deepEqual(spread.cards.map((c) => c.display), ['10', '6', '12'])

  // undo replays the steps and reproduces the same layout
  undo(spread)
  assert.deepEqual(spread.cards.map((c) => c.display), ['2', '10', '12', '4'])
})

test('checker: hand state survives a JSON round-trip (refresh resume)', () => {
  const hand = newHand([6, 2, 9, 4])
  pickCard(hand, hand.cards[0])
  setOperator(hand, '*')
  pickCard(hand, hand.cards[1])
  assert.equal(hand.cards.length, 3)
  // the refresh snapshot stores the hand as plain JSON and replays it —
  // the revived state must be identical and keep playing (undo included)
  const revived = JSON.parse(JSON.stringify(hand))
  assert.deepEqual(revived, hand)
  undo(revived)
  assert.equal(revived.cards.length, 4)
  assert.equal(revived.steps.length, 0)
})

test('checker: revived selection re-links by id, merges still collapse', () => {
  const hand = newHand([6, 2, 9, 4])
  pickCard(hand, hand.cards[0])
  const revived = JSON.parse(JSON.stringify(hand))
  // references don't survive JSON: re-link the selection into cards by id
  revived.selection = revived.cards.find((c) => c.id === revived.selection.id)
  // tapping the picked card again must cancel it, not duplicate a card
  pickCard(revived, revived.selection)
  assert.equal(revived.selection, null)
  assert.equal(revived.cards.length, 4)
  // and a full merge still collapses to three cards
  pickCard(revived, revived.cards[0])
  setOperator(revived, '*')
  pickCard(revived, revived.cards[1])
  assert.equal(revived.cards.length, 3)
})

test('progress: level curve matches server', () => {
  assert.equal(expForLevel(100), 297000)
  assert.equal(levelFromExp(0), 1)
  assert.equal(levelFromExp(59), 1)
  assert.equal(levelFromExp(60), 2)
  assert.equal(levelFromExp(296999), 99)
  assert.equal(levelFromExp(297000), 100)
})

test('progress: level caps at 100', () => {
  assert.equal(MAX_LEVEL, 100)
  assert.equal(levelFromExp(1e9), 100)
  // at the cap the ladder ends: forNext reads 0 so the bar renders full
  assert.deepEqual(levelProgress(297000), { lv: 100, into: 0, forNext: 0 })
  assert.deepEqual(levelProgress(297000 + 5000), { lv: 100, into: 5000, forNext: 0 })
})

test('progress: score multiplier +5% per level, capped', () => {
  assert.equal(scoreMultiplier(1), 1)
  assert.equal(scoreMultiplier(11), 1.5)
  assert.equal(scoreMultiplier(100), 5.95)
  assert.equal(scoreMultiplier(250), 5.95) // past the cap: clamped
  assert.equal(scoreMultiplier(0), 1)
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
