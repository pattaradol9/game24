// Client-side play state: cards on the board, merge interactions and the
// step trace for server verification.
//
// Interaction order: number → operator → number. The merge fires the moment
// the second number is picked.
import { frac, int, apply, format, is24 } from './fraction.js'

// makeCards turns dealt numbers into playable cards.
// origin: {card: i} = original card i; {step: k} = result of step k.
export function makeCards(numbers) {
  return numbers.map((n, i) => ({
    id: `c${i}`,
    value: int(n),
    display: String(n),
    origin: { card: i },
    expr: String(n),
  }))
}

export function newHand(numbers) {
  return {
    numbers,
    cards: makeCards(numbers),
    steps: [],
    history: [],
    selection: null, // first picked card
    operator: null, // operator armed after the first card
    won: false,
  }
}

// pickCard handles a card click in the number→operator→number flow.
export function pickCard(state, card) {
  if (state.selection === card) {
    // tapping the picked card cancels it (and the armed operator with it)
    state.selection = null
    state.operator = null
    return state
  }
  if (state.selection && state.operator) {
    // second operand: run the operation right away
    return merge(state, state.selection, card)
  }
  state.selection = card
  return state
}

// setOperator arms an operator; it only registers after a number is picked.
// Tapping the same operator again disarms it.
export function setOperator(state, op) {
  if (!state.selection) return state
  state.operator = state.operator === op ? null : op
  return state
}

function merge(state, a, b) {
  const op = state.operator
  if (!op) return state
  const stepIndex = state.steps.length
  const value = apply(op, a.value, b.value)
  const card = {
    id: `s${stepIndex}`,
    value,
    display: format(value),
    origin: { step: stepIndex },
    expr: `(${a.expr}${op}${b.expr})`,
  }
  state.cards = state.cards.filter((c) => c !== a && c !== b)
  state.cards.push(card)
  state.steps.push({ left: a.origin, right: b.origin, op })
  state.selection = null
  state.operator = null
  state.won = state.cards.length === 1 && is24(card.value)
  state.history.push(`(${stripOuter(a.expr)}${op}${stripOuter(b.expr)}) = ${format(value)}`)
  return state
}

// undo pops the last merge and replays the remaining steps.
export function undo(state) {
  if (!state.steps.length) return state
  state.steps.pop()
  state.history.pop()
  state.won = false
  state.selection = null
  state.operator = null
  return rebuild(state, state.numbers, state.steps)
}

export function rebuild(state, numbers, steps) {
  state.cards = makeCards(numbers)
  state.selection = null
  state.operator = null
  const byOrigin = new Map(state.cards.map((c) => [JSON.stringify(c.origin), c]))
  steps.forEach((st, k) => {
    const a = byOrigin.get(JSON.stringify(st.left))
    const b = byOrigin.get(JSON.stringify(st.right))
    if (!a || !b) throw new Error('corrupt step history')
    const value = apply(st.op, a.value, b.value)
    const card = {
      id: `s${k}`,
      value,
      display: format(value),
      origin: { step: k },
      expr: `(${a.expr}${st.op}${b.expr})`,
    }
    state.cards = state.cards.filter((c) => c !== a && c !== b)
    state.cards.push(card)
    byOrigin.set(JSON.stringify(card.origin), card)
  })
  state.won = state.cards.length === 1 && is24(state.cards[0].value)
  return state
}

function stripOuter(s) {
  return s.startsWith('(') && s.endsWith(')') ? s.slice(1, -1) : s
}

export { frac, int, apply, format, is24 }
