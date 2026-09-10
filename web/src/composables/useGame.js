import { onUnmounted, ref } from 'vue'
import { api } from '../api.js'
import { getToken, getPlayer, updatePlayer } from '../auth.js'
import { newHand, pickCard as doPickCard, setOperator as doSetOperator, undo as doUndo } from '../core/checker.js'
import { singleHintQuota } from '../core/progress.js'
import { sfx } from '../audio.js'
import { burst } from '../confetti.js'

// the catalog's time item (kind "time"): spent mid-game to extend a hand
const TIME_ITEM_ID = 'time30'
// the catalog's skip item (kind "skip"): spent by the Skip button, one per
// play session — the plain timeout folds the hand for free instead
const SKIP_ITEM_ID = 'skip1'

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
  // the game's running tallies: every solved hand banks its score, EXP and
  // coins — the refresh snapshot owns them so a reload lands back on them
  const sessionScore = ref(0)
  const sessionExp = ref(0)
  const sessionCoins = ref(0)
  // a refresh-restored hand resumes silently, skipping the countdown intro
  const restored = ref(false)
  // Time Extension offer: while set, the clock is stopped at 0:00 and the
  // "spend the item for +30s?" dialog is up — the round continues only if
  // the player confirms
  const extendOffer = ref(null)
  const extendBusy = ref(false)
  const extendUsed = ref(false) // one bailout per play session
  const skipUsed = ref(false) // the Skip Pass folds one hand per session

  // One hint budget per visit to the game: every hand dealt while this view
  // stays mounted shares the same session, so hints never refill between
  // rounds — only leaving and coming back grants a fresh budget. A same-tab
  // refresh reuses the stored session id, so the visit survives reloads.
  let sessionId = crypto.randomUUID
    ? crypto.randomUUID()
    : `s-${Date.now()}-${Math.random().toString(36).slice(2)}`

  let deadline = 0
  let timer = null
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
    hintCard.value = null
    combo.value = 0
    clearSnapshot() // leaving (or ending) the game kills the stored session
  }
  onUnmounted(stop)

  // ---------------------------------------------------------------------------
  // Refresh-resume: the live session mirrors into sessionStorage — a same-tab
  // reload replays it (board, tallies, quotas, remaining time), while closing
  // the tab ends the session for good. The server clock stays the sole judge
  // of the hand's window: the snapshot only carries what the server already
  // knows (roundId, sessionId) plus client-side display state.
  const SNAPSHOT_KEY = 'game24:solo-session'

  function saveSnapshot() {
    if (disposed || !round.value || phase.value === 'idle' || phase.value === 'loading') return
    try {
      sessionStorage.setItem(SNAPSHOT_KEY, JSON.stringify({
        sessionId,
        mode: mode.value,
        roundId: round.value.roundId,
        numbers: round.value.numbers,
        timeLimit: timeLimit.value,
        deadline,
        phase: phase.value,
        win: result.value?.win ?? null,
        hand: hand.value,
        hintLeft: hintLeft.value,
        extendUsed: extendUsed.value,
        skipUsed: skipUsed.value,
        tallies: { score: sessionScore.value, exp: sessionExp.value, coins: sessionCoins.value },
      }))
    } catch { /* storage unavailable — a refresh then simply restarts */ }
  }

  function loadSnapshot() {
    try {
      const s = JSON.parse(sessionStorage.getItem(SNAPSHOT_KEY))
      return s && s.roundId ? s : null
    } catch { return null }
  }

  function clearSnapshot() {
    try { sessionStorage.removeItem(SNAPSHOT_KEY) } catch { /* noop */ }
  }

  // resume a refresh-surviving hand: replay the stored board, tallies and
  // quotas and restart the clock from the stored absolute deadline
  function restoreLive(snap) {
    mode.value = snap.mode
    round.value = { roundId: snap.roundId, numbers: snap.numbers }
    hand.value = snap.hand
    // JSON round-trips break card identity: the armed selection must point
    // back into the revived cards array (the checker merges by reference)
    if (hand.value.selection) {
      hand.value.selection = hand.value.cards.find((c) => c.id === hand.value.selection.id) ?? null
    }
    timeLimit.value = snap.timeLimit
    deadline = snap.deadline
    remaining.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
    hintLeft.value = snap.hintLeft ?? 0
    extendUsed.value = !!snap.extendUsed
    skipUsed.value = !!snap.skipUsed
    sessionScore.value = snap.tallies?.score ?? 0
    sessionExp.value = snap.tallies?.exp ?? 0
    sessionCoins.value = snap.tallies?.coins ?? 0
    result.value = null
    hintCard.value = null
    combo.value = 0
    error.value = ''
    busy.value = false
    paused.value = false
    restored.value = true
    phase.value = 'playing'
    lastBeat = -1
    stopTimer()
    timer = setInterval(tick, 250)
    saveSnapshot()
  }

  async function start(m = mode.value) {
    // a same-tab refresh lands here with the previous game in storage:
    // reuse its session id so the server-side hint/extend/skip quotas treat
    // the resumed game as one continuing visit
    const snap = loadSnapshot()
    if (snap?.sessionId) sessionId = snap.sessionId
    if (snap && snap.phase === 'result') {
      // refreshed on a result dialog: a win keeps the tallies (that hand is
      // over and banked), a game-over wipes them for the next run
      sessionScore.value = snap.win ? snap.tallies?.score ?? 0 : 0
      sessionExp.value = snap.win ? snap.tallies?.exp ?? 0 : 0
      sessionCoins.value = snap.win ? snap.tallies?.coins ?? 0 : 0
      clearSnapshot()
    } else if (snap?.roundId && snap.hand) {
      restoreLive(snap)
      return
    } else {
      clearSnapshot()
    }
    mode.value = m
    phase.value = 'loading'
    error.value = ''
    busy.value = true
    combo.value = 0
    extendOffer.value = null
    try {
      const data = await api.createRound(m, getToken(), sessionId)
      round.value = data
      hand.value = newHand(data.numbers)
      remaining.value = data.timeLimit
      timeLimit.value = data.timeLimit
      hintLeft.value = data.hintsLeft ?? singleHintQuota(getPlayer()?.level ?? 1)
      hintCard.value = null
      result.value = null
      deadline = Date.now() + data.timeLimit * 1000
      phase.value = 'playing'
      paused.value = true
      pausedAt = Date.now()
      stopTimer()
      lastBeat = -1
      timer = setInterval(tick, 250)
      saveSnapshot()
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
    saveSnapshot()
  }

  function tick() {
    if (disposed || paused.value) return
    remaining.value = Math.max(0, Math.ceil((deadline - Date.now()) / 1000))
    saveSnapshot()
    if (remaining.value <= 10 && remaining.value > 0 && remaining.value !== lastBeat) {
      lastBeat = remaining.value
      sfx.heartbeat()
    }
    if (remaining.value <= 0) finishByTimeout()
  }

  // how many Time Extension units the bag holds (0 for guests — they never
  // own items)
  function timeItemsInBag() {
    const p = getPlayer()
    if (!p || p.isGuest) return 0
    return (p.items ?? []).find((i) => i.id === TIME_ITEM_ID)?.qty ?? 0
  }

  // same read for the Skip Pass the Skip button spends
  function skipItemsInBag() {
    const p = getPlayer()
    if (!p || p.isGuest) return 0
    return (p.items ?? []).find((i) => i.id === SKIP_ITEM_ID)?.qty ?? 0
  }

  async function finishByTimeout() {
    stopTimer()
    if (disposed) return
    // holding a Time Extension buys one 30-second bailout per session: park
    // the round on 0:00 and ask before spending it
    if (!extendUsed.value && timeItemsInBag() > 0) {
      extendOffer.value = { roundId: round.value.roundId }
      return
    }
    await concedeTimeout()
  }

  // the plain timeout flow the offer falls back to: skip the hand, show the
  // solution — the fold goes through the free timeout endpoint, no item
  async function concedeTimeout() {
    sfx.lose()
    try {
      const data = await api.timeoutRound(round.value.roundId, getToken())
      if (disposed) return
      updatePlayer(data.player)
      result.value = { win: false, solution: data.solution, player: data.player, points: 0, coins: data.coins ?? 0, newAchievements: data.newAchievements ?? [] }
    } catch {
      if (disposed) return
      result.value = { win: false, solution: '', points: 0 }
    }
    hintCard.value = null
    combo.value = 0
    phase.value = 'result'
    saveSnapshot()
  }

  // the dialog's answer. use=true spends one unit through the extend
  // endpoint and hands the player 30 fresh seconds on THIS hand; declining
  // (or a refused extend — empty bag, session already used, round gone)
  // falls through to the plain timeout flow. Resolves to the seconds
  // actually gained (falsy when the hand went on).
  async function resolveExtend(use) {
    if (!extendOffer.value || extendBusy.value) return 0
    if (!use) {
      extendOffer.value = null
      await concedeTimeout()
      return 0
    }
    extendBusy.value = true
    try {
      const data = await api.extendRound(round.value.roundId, getToken())
      if (disposed) return 0
      extendUsed.value = true
      updatePlayer(data.player)
      extendOffer.value = null
      const extra = data.extraSeconds ?? 30
      deadline = Date.now() + extra * 1000
      remaining.value = extra
      lastBeat = -1
      stopTimer()
      timer = setInterval(tick, 250)
      saveSnapshot()
      sfx.pop()
      return extra
    } catch {
      if (disposed) return 0
      extendOffer.value = null
      await concedeTimeout()
      return 0
    } finally {
      extendBusy.value = false
    }
  }

  // the Add-time button: spends one Time Extension mid-play, no need to wait
  // for the clock to die. The server clamps the extra so a hand's remaining
  // time never passes the mode's base window; the client mirrors that clamp
  // so the countdown tops out exactly at timeLimit. Returns the seconds
  // actually gained (0 when the press was refused or added nothing).
  async function addTime() {
    if (phase.value !== 'playing' || paused.value || extendOffer.value || extendBusy.value) return 0
    if (extendUsed.value || timeItemsInBag() <= 0) return 0
    if (remaining.value >= timeLimit.value) return 0
    extendBusy.value = true
    const before = remaining.value
    try {
      const data = await api.extendRound(round.value.roundId, getToken())
      if (disposed) return 0
      extendUsed.value = true
      updatePlayer(data.player)
      const added = Math.max(0, Math.min(data.extraSeconds ?? 0, timeLimit.value - before))
      remaining.value = before + added
      deadline = Date.now() + remaining.value * 1000
      lastBeat = -1
      saveSnapshot()
      sfx.pop()
      return added
    } catch (e) {
      if (!disposed) error.value = e.message
      return 0
    } finally {
      extendBusy.value = false
    }
  }

  function pickCard(card) {
    if (phase.value !== 'playing' || paused.value || extendOffer.value || !hand.value) return false
    const before = hand.value.cards.length
    doPickCard(hand.value, card)
    if (hand.value.cards.length < before) {
      sfx.merge(combo.value)
      combo.value += 1
      saveSnapshot()
      if (hand.value.won) submitWin()
      return true
    }
    saveSnapshot()
    return false
  }

  function setOperator(op) {
    if (phase.value !== 'playing' || paused.value || extendOffer.value) return
    doSetOperator(hand.value, op)
    saveSnapshot()
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
      // bank this hand's payout into the run's tallies — the side panel and
      // the game-over summary both read them (timeout/skip bank nothing);
      // must precede saveSnapshot so a refresh on the dialog keeps them
      sessionScore.value += data.points ?? 0
      sessionExp.value += data.exp ?? data.points ?? 0
      sessionCoins.value += data.coins ?? 0
      // `exp` is what the hand banked server-side (boost multiplier
      // applied); it falls back to the game points when absent
      result.value = { win: true, expr: data.expr, points: data.points, exp: data.exp ?? data.points, player: data.player, levelUp: data.levelUp, tierUp: data.tierUp, coins: data.coins ?? 0, newAchievements: data.newAchievements ?? [], remaining: remaining.value, timeLimit: timeLimit.value }
      hintCard.value = null
      combo.value = 0
      phase.value = 'result'
      saveSnapshot()
    } catch (e) {
      error.value = e.message
      // server rejected (shouldn't normally happen): keep playing
      phase.value = 'playing'
    } finally {
      busy.value = false
    }
  }

  function undo() {
    if (phase.value !== 'playing' || paused.value || extendOffer.value || !hand.value) return
    doUndo(hand.value)
    combo.value = 0
    saveSnapshot()
    sfx.cardSlide()
  }

  // the Skip button folds the hand mid-play — it costs one Skip Pass and
  // only one fold per play session is allowed (the server enforces both; a
  // refused skip leaves the round open and surfaces the reason)
  async function skip() {
    if (phase.value !== 'playing' || paused.value || busy.value || extendOffer.value) return
    if (skipUsed.value || skipItemsInBag() <= 0) return
    busy.value = true
    stopTimer()
    try {
      const data = await api.skipRound(round.value.roundId, getToken())
      if (disposed) return
      updatePlayer(data.player)
      skipUsed.value = true
      sfx.lose()
      result.value = { win: false, solution: data.solution, player: data.player, points: 0, coins: data.coins ?? 0, newAchievements: data.newAchievements ?? [], remaining: remaining.value, timeLimit: timeLimit.value }
      hintCard.value = null
      combo.value = 0
      phase.value = 'result'
      saveSnapshot()
    } catch (e) {
      error.value = e.message
      // nothing was consumed: the clock keeps running
      if (!disposed && !timer) timer = setInterval(tick, 250)
    } finally {
      busy.value = false
    }
  }

  async function hint() {
    if (phase.value !== 'playing' || paused.value || extendOffer.value || hintLeft.value <= 0 || busy.value) return
    busy.value = true
    try {
      const h = await api.hintRound(round.value.roundId, getToken())
      if (disposed) return
      hintLeft.value = Math.max(0, hintLeft.value - 1)
      // the hint stays on the board until the round ends
      hintCard.value = { ...h, at: Date.now() }
      saveSnapshot()
      sfx.pop()
    } catch (e) {
      error.value = e.message
    } finally {
      busy.value = false
    }
  }

  function next() {
    // a game-over resets the tallies; a win chains into the next hand
    if (result.value && !result.value.win) {
      sessionScore.value = 0
      sessionExp.value = 0
      sessionCoins.value = 0
    }
    start(mode.value)
  }

  return {
    phase, mode, hand, round, remaining, timeLimit, hintLeft, hintCard, result, error, busy, paused, combo,
    extendOffer, extendBusy, extendUsed, skipUsed,
    sessionScore, sessionExp, sessionCoins, restored,
    start, resume, stop, pickCard, setOperator, undo, skip, hint, next, resolveExtend, addTime,
  }
}
