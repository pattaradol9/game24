import { onUnmounted, ref } from 'vue'
import { api } from '../api.js'
import { getToken, getPlayer, updatePlayer } from '../auth.js'
import { newHand, pickCard as doPickCard, setOperator as doSetOperator, undo as doUndo } from '../core/checker.js'
import { singleHintQuota } from '../core/progress.js'
import { sfx } from '../audio.js'
import { burst } from '../confetti.js'

// Single-player game state machine.
export function useGame() {
  const phase = ref('idle') // idle | loading | playing | result
  const mode = ref('queen')
  const hand = ref(null)
  const round = ref(null)
  const remaining = ref(0)
  const hintLeft = ref(0)
  const hintCard = ref(null)
  const result = ref(null)
  const error = ref('')
  const busy = ref(false)
  const paused = ref(false) // countdown intro: clock frozen, input blocked
  const combo = ref(0) // consecutive merges in this hand (sound pitch ladder)
  const timeLimit = ref(0)

  let deadline = 0
  let timer = null
  let hintTimer = 0
  let pausedAt = 0
  let lastBeat = -1
  let disposed = false

  function stopTimer() {
    if (timer) clearInterval(timer)
    timer = null
  }

  /**
   * Tear everything down. Without this the round's interval outlived the view:
   * leaving mid-game kept the clock ticking on the home screen, kept firing the
   * last-ten-seconds heartbeat, and eventually timed the round out in the
   * background. Any in-flight request that lands afterwards is ignored too, so
   * it can't play a win/lose sting into another screen.
   */
  function stop() {
    disposed = true
    stopTimer()
    clearTimeout(hintTimer)
    hintCard.value = null
  }
  onUnmounted(stop)

  async function start(m = mode.value) {
    mode.value = m
    phase.value = 'loading'
    error.value = ''
    busy.value = true
    combo.value = 0
    try {
      const data = await api.createRound(m, getToken())
      round.value = data
      hand.value = newHand(data.numbers)
      remaining.value = data.timeLimit
      timeLimit.value = data.timeLimit
      hintLeft.value = data.hintQuota ?? singleHintQuota(getPlayer()?.level ?? 1)
      hintCard.value = null
      result.value = null
      deadline = Date.now() + data.timeLimit * 1000
      phase.value = 'playing'
      paused.value = true
      pausedAt = Date.now()
      stopTimer()
      lastBeat = -1
      timer = setInterval(tick, 250)
    } catch (e) {
      error.value = e.message
      phase.value = 'idle'
    } finally {
      busy.value = false
    }
  }

  // called when the countdown intro finishes
  function resume() {
    if (!paused.value) return
    deadline += Date.now() - pausedAt
    paused.value = false
  }

  function tick() {
    if (disposed || paused.value) return
    remaining.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
    if (remaining.value <= 10 && remaining.value > 0 && remaining.value !== lastBeat) {
      lastBeat = remaining.value
      sfx.heartbeat()
    }
    if (remaining.value <= 0) finishByTimeout()
  }

  async function finishByTimeout() {
    stopTimer()
    if (disposed) return
    sfx.lose()
    try {
      const data = await api.skipRound(round.value.roundId, getToken())
      if (disposed) return
      updatePlayer(data.player)
      result.value = { win: false, solution: data.solution, player: data.player, points: 0 }
    } catch {
      if (disposed) return
      result.value = { win: false, solution: '', points: 0 }
    }
    phase.value = 'result'
  }

  function pickCard(card) {
    if (phase.value !== 'playing' || paused.value || !hand.value) return false
    const before = hand.value.cards.length
    doPickCard(hand.value, card)
    if (hand.value.cards.length < before) {
      sfx.merge(combo.value)
      combo.value += 1
      if (hand.value.won) submitWin()
      return true
    }
    return false
  }

  function setOperator(op) {
    if (phase.value !== 'playing' || paused.value) return
    doSetOperator(hand.value, op)
  }

  async function submitWin() {
    stopTimer()
    busy.value = true
    try {
      const data = await api.submitRound(round.value.roundId, hand.value.steps, getToken())
      if (disposed) return
      updatePlayer(data.player)
      burst()
      sfx.win()
      result.value = { win: true, expr: data.expr, points: data.points, player: data.player, levelUp: data.levelUp, tierUp: data.tierUp, remaining: remaining.value, timeLimit: timeLimit.value }
      phase.value = 'result'
    } catch (e) {
      error.value = e.message
      // server rejected (shouldn't normally happen): keep playing
      phase.value = 'playing'
    } finally {
      busy.value = false
    }
  }

  function undo() {
    if (phase.value !== 'playing' || paused.value || !hand.value) return
    doUndo(hand.value)
    combo.value = 0
    sfx.cardSlide()
  }

  async function skip() {
    if (phase.value !== 'playing' || paused.value || busy.value) return
    busy.value = true
    stopTimer()
    try {
      const data = await api.skipRound(round.value.roundId, getToken())
      if (disposed) return
      updatePlayer(data.player)
      sfx.lose()
      result.value = { win: false, solution: data.solution, player: data.player, points: 0, remaining: remaining.value, timeLimit: timeLimit.value }
      phase.value = 'result'
    } catch (e) {
      error.value = e.message
    } finally {
      busy.value = false
    }
  }

  async function hint() {
    if (phase.value !== 'playing' || paused.value || hintLeft.value <= 0 || busy.value) return
    busy.value = true
    try {
      const h = await api.hintRound(round.value.roundId, getToken())
      if (disposed) return
      hintLeft.value = Math.max(0, hintLeft.value - 1)
      hintCard.value = h
      sfx.pop()
      clearTimeout(hintTimer)
      hintTimer = setTimeout(() => (hintCard.value = null), 4000)
    } catch (e) {
      error.value = e.message
    } finally {
      busy.value = false
    }
  }

  function next() {
    start(mode.value)
  }

  return {
    phase, mode, hand, round, remaining, timeLimit, hintLeft, hintCard, result, error, busy, paused, combo,
    start, resume, stop, pickCard, setOperator, undo, skip, hint, next,
  }
}
