import { ref, computed } from 'vue'
import { connectRoom } from '../ws.js'
import { getToken, getPlayer } from '../auth.js'
import { api } from '../api.js'
import { newHand, pickCard as doPickCard, setOperator as doSetOperator, undo as doUndo } from '../core/checker.js'
import { sfx } from '../audio.js'
import { burst } from '../confetti.js'

// Multiplayer room state machine driven by websocket events.
export function useRoom() {
  const state = ref('connecting') // connecting | lobby | round | summary | finished | closed | gone | lost
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
  // epoch ms deadline while the host seat is empty and the reconnect grace
  // is running; 0 means the host is connected
  const hostDeadline = ref(0)
  const isHost = computed(() => you.value !== '' && you.value === host.value)

  let sock = null
  let timer = null
  let hostKey = ''

  // the server hands out a resume secret per seat; keeping it in
  // sessionStorage (survives refreshes, dies with the tab) lets the same
  // browser reattach its old seat instead of forking a new one
  const resumeKeyFor = (roomCode) => `game24:resume:${roomCode}`
  const storedResume = (roomCode) => {
    try { return sessionStorage.getItem(resumeKeyFor(roomCode)) || '' } catch { return '' }
  }
  const storeResume = (roomCode, key) => {
    try {
      if (key) sessionStorage.setItem(resumeKeyFor(roomCode), key)
      else sessionStorage.removeItem(resumeKeyFor(roomCode))
    } catch { /* storage unavailable — a refresh just forks a fresh seat */ }
  }

  async function open(code_, hostKey_) {
    code.value = code_
    hostKey = hostKey_ || ''
    // fail fast into a clear screen when the room no longer exists (e.g.
    // refreshing after it was reaped) instead of retry-storming the socket
    try {
      await api.roomInfo(code.value)
    } catch (e) {
      if (e?.status === 404) {
        state.value = 'gone'
        return
      }
      // network hiccup — let the websocket retries handle it
    }
    const name = getPlayer()?.nickname || `Guest${Math.floor(Math.random() * 90) + 10}`
    sock = connectRoom(code.value, { name, token: getToken(), hostKey, resume: storedResume(code.value) }, {
      onMessage: handle,
      onClose: () => {
        if (state.value !== 'finished' && state.value !== 'closed') {
          state.value = state.value // keep; reconnect handled inside ws.js
        }
      },
      onGiveUp: async () => {
        if (state.value === 'finished' || state.value === 'closed') return
        // reconnects dried up: a 404 means the room is gone for good,
        // anything else is a connection problem worth a retry button
        try {
          await api.roomInfo(code.value)
          state.value = 'lost'
        } catch {
          state.value = 'gone'
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
      case 'welcome': {
        const st = d.state ?? {}
        applyState(st)
        if (d.resume) storeResume(code.value, d.resume)
        // resuming after a refresh: rebuild the live round, or the podium
        if (d.round) startRound(d.round, false)
        else if (st.state === 'finished') matchResult.value = st.players ?? []
        // members joining mid-absence inherit the reconnect countdown;
        // the reclaiming host must not — host_back already lifted it and
        // re-arming it would flash the wait dialog on the host's screen
        if (d.hostReconnecting && d.you !== (st.host ?? '')) hostDeadline.value = d.hostReconnecting.endsAt
        break
      }
      case 'room_state':
        applyState(d)
        break
      case 'round_start':
        startRound(d, true)
        break
      case 'round_result':
        stopTimer()
        // the round is over: the hint served its purpose and the merge
        // streak belongs to the finished round only
        hint.value = null
        combo.value = 0
        roundResult.value = d
        state.value = 'summary'
        if (d.standings) players.value = mergePlayers(players.value, d.standings)
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
      case 'host_reconnecting':
        hostDeadline.value = d.endsAt
        break
      case 'host_back':
        hostDeadline.value = 0
        break
      case 'room_closed':
        // the host left for good: stop cleanly, show the closed notice
        stopTimer()
        hostDeadline.value = 0
        storeResume(code.value, '')
        sock?.close()
        state.value = 'gone'
        break
      case 'error':
        error.value = d.message ?? 'error'
        if (String(d.message).startsWith('wrong')) flashWrong()
        setTimeout(() => (error.value = ''), 2500)
        break
    }
  }

  // fold an incoming roster into the current one, reusing the previous
  // player objects wherever nothing changed — stable identities keep Vue
  // from re-rendering (and re-animating) untouched rows, so one player's
  // refresh never flickers anyone else's screen
  function mergePlayers(prev, incoming) {
    if (!Array.isArray(incoming)) return []
    const before = new Map((prev ?? []).map((p) => [p.id, p]))
    return incoming.map((p) => {
      const old = before.get(p.id)
      const unchanged = old && ['name', 'guest', 'level', 'tier', 'score', 'wins', 'hintsLeft', 'regensLeft', 'host', 'absent']
        .every((k) => old[k] === p[k])
      return unchanged ? old : p
    })
  }

  function applyState(s) {
    if (!s) return
    state.value = s.state === 'round' ? 'round' : s.state
    players.value = mergePlayers(players.value, s.players ?? [])
    host.value = s.host ?? ''
    config.value = s.config ?? null
    roundNo.value = s.roundNo ?? 0
  }

  // one code path for a fresh round_start and for resuming a live round
  // after a refresh; without the intro the rejoined player keeps every
  // remaining second of the round
  function startRound(d, withIntro) {
    roundNo.value = d.roundNo
    totalRounds.value = d.total
    numbers.value = d.numbers
    endsAt.value = d.endsAt
    hand.value = newHand(d.numbers)
    roundResult.value = null
    hint.value = null
    combo.value = 0
    state.value = 'round'
    remaining.value = Math.max(0, Math.ceil((d.endsAt - Date.now()) / 1000))
    timeLimit.value = d.timeLimit
    intro.value = withIntro
    startTimer()
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

  // push a nickname change to everyone in the room without rejoining
  function rename(name) {
    sock?.send('rename', { name })
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
    hint.value = null
    combo.value = 0
    state.value = 'closed'
  }

  return {
    state, code, you, players, host, config, roundNo, totalRounds,
    numbers, endsAt, remaining, timeLimit, hand, roundResult, matchResult, hint,
    wrongFlash, error, isHost, combo, intro, hostDeadline,
    open, start, rename, pickCard, setOperator, undo, askHint, askRegen, leave, introDone,
  }
}
