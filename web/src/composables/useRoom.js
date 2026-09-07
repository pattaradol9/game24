import { ref, computed } from 'vue'
import { connectRoom } from '../ws.js'
import { getToken, getPlayer } from '../auth.js'
import { newHand, pickCard as doPickCard, setOperator as doSetOperator, undo as doUndo } from '../core/checker.js'
import { sfx } from '../audio.js'
import { burst } from '../confetti.js'

// Multiplayer room state machine driven by websocket events.
export function useRoom() {
  const state = ref('connecting') // connecting | lobby | round | summary | finished | closed
  const code = ref('')
  const you = ref('')
  const players = ref([])
  const host = ref('')
  const config = ref(null)
  const roundNo = ref(0)
  const totalRounds = ref(0)
  const numbers = ref([])
  const endsAt = ref(0)
  const remaining = ref(0)
  const hand = ref(null) // personal checker state (multi)
  const roundResult = ref(null)
  const matchResult = ref(null)
  const hint = ref(null)
  const wrongFlash = ref(false)
  const error = ref('')
  const combo = ref(0) // consecutive merges this round (sound pitch ladder)
  const intro = ref(false) // 3-2-1 intro overlay showing; input blocked
  const timeLimit = ref(0)
  const isHost = computed(() => you.value !== '' && you.value === host.value)

  let sock = null
  let timer = null
  let hostKey = ''

  function open(code_, hostKey_) {
    code.value = code_
    hostKey = hostKey_ || ''
    const name = getPlayer()?.nickname || `Guest${Math.floor(Math.random() * 90) + 10}`
    sock = connectRoom(code.value, { name, token: getToken() }, {
      onMessage: handle,
      onClose: () => {
        if (state.value !== 'finished' && state.value !== 'closed') {
          state.value = state.value // keep; reconnect handled inside ws.js
        }
      },
    })
  }

  function handle(env) {
    const d = env.data ?? {}
    switch (env.type) {
      case 'joined':
        you.value = d.sessionId
        break
      case 'welcome':
        applyState(d.state)
        break
      case 'room_state':
        applyState(d)
        break
      case 'round_start':
        roundNo.value = d.roundNo
        totalRounds.value = d.total
        numbers.value = d.numbers
        endsAt.value = d.endsAt
        hand.value = newHand(d.numbers)
        roundResult.value = null
        hint.value = null
        combo.value = 0
        state.value = 'round'
        remaining.value = d.timeLimit
        timeLimit.value = d.timeLimit
        intro.value = true
        startTimer()
        break
      case 'round_result':
        stopTimer()
        // the round is over: the hint served its purpose
        hint.value = null
        roundResult.value = d
        state.value = 'summary'
        if (d.standings) players.value = d.standings
        if (d.winner) sfx.win()
        else sfx.lose()
        break
      case 'match_end':
        state.value = 'finished'
        matchResult.value = d.standings ?? []
        break
      case 'hint':
        // stays on the board until the round ends
        hint.value = { ...d, at: Date.now() }
        sfx.click()
        break
      case 'regen':
        numbers.value = d.numbers
        hand.value = newHand(d.numbers)
        // the old hint describes the replaced hand
        hint.value = null
        break
      case 'error':
        error.value = d.message ?? 'error'
        if (String(d.message).startsWith('wrong')) flashWrong()
        setTimeout(() => (error.value = ''), 2500)
        break
    }
  }

  function applyState(s) {
    if (!s) return
    state.value = s.state === 'round' ? 'round' : s.state
    players.value = s.players ?? []
    host.value = s.host ?? ''
    config.value = s.config ?? null
    roundNo.value = s.roundNo ?? 0
  }

  function startTimer() {
    stopTimer()
    let lastBeat = -1
    timer = setInterval(() => {
      remaining.value = Math.max(0, Math.ceil((endsAt.value - Date.now()) / 1000))
      if (remaining.value <= 10 && remaining.value > 0 && remaining.value !== lastBeat) {
        lastBeat = remaining.value
        sfx.heartbeat()
      }
    }, 250)
  }

  function stopTimer() {
    if (timer) clearInterval(timer)
    timer = null
  }

  function flashWrong() {
    wrongFlash.value = true
    sfx.lose()
    setTimeout(() => (wrongFlash.value = false), 500)
  }

  function start() {
    sock?.send('start', { hostKey })
  }

  function introDone() {
    intro.value = false
  }

  function pickCard(card) {
    if (state.value !== 'round' || intro.value || !hand.value) return
    const before = hand.value.cards.length
    doPickCard(hand.value, card)
    if (hand.value.cards.length < before) {
      sfx.merge(combo.value)
      combo.value += 1
      if (hand.value.won) sock?.send('submit', { steps: hand.value.steps })
    }
  }

  function setOperator(op) {
    if (state.value !== 'round' || intro.value || !hand.value) return
    doSetOperator(hand.value, op)
  }

  function undo() {
    if (state.value !== 'round' || intro.value || !hand.value) return
    doUndo(hand.value)
    combo.value = 0
    sfx.cardSlide()
  }

  function askHint() {
    sock?.send('hint')
  }

  function askRegen() {
    sock?.send('regen')
  }

  function leave() {
    stopTimer()
    sock?.close()
    state.value = 'closed'
  }

  return {
    state, code, you, players, host, config, roundNo, totalRounds,
    numbers, endsAt, remaining, timeLimit, hand, roundResult, matchResult, hint,
    wrongFlash, error, isHost, combo, intro,
    open, start, pickCard, setOperator, undo, askHint, askRegen, leave, introDone,
  }
}
