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
  // achievements unlocked by winning a round; rendered as non-blocking
  // toasts that dismiss themselves (server pushes one message per award)
  const achievementPops = ref([])
  let popTimers = []
  // the room's helper group, all backed by the quotas the host set at room
  // creation (solution / add-time / new-hand — identical for every seat,
  // never the players' personal inventory). extendEndsAt is the seat's live
  // private deadline (0 = none); extendExtra carries the seconds the last
  // Add-time press granted so the view can toast it.
  const extendEndsAt = ref(0)
  const extendExtra = ref(0)
  // every helper also works once per round per seat (server-enforced);
  // a fresh round resets them, a refresh restores them through the
  // welcome payload's hintUsed / extendUsed / regenUsed flags
  const hintUsedRound = ref(false)
  const extendUsedRound = ref(false)
  const regenUsedRound = ref(false)
  // epoch ms when the summary advances itself (60s stall guard) — the
  // summary shows the countdown; the host's press stays the fast path
  const autoNextAt = ref(0)
  // true once the server ended the match because the host left; the final
  // podium wears the "host left" note while it shows
  const hostLeftEnd = ref(false)
  const isHost = computed(() => you.value !== '' && you.value === host.value)

  let sock = null
  let timer = null
  let hostKey = ''
  let joinedOk = false

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
        joinedOk = true
        you.value = d.sessionId
        break
      case 'round_start':
        // a fresh round clears everyone's solved status and solve order
        players.value = (players.value ?? []).map((p) => ({ ...p, solved: false, solveOrder: 0, timedOut: false }))
        startRound(d, true)
        break
      case 'welcome': {
        const st = d.state ?? {}
        applyState(st)
        if (d.resume) storeResume(code.value, d.resume)
        // resuming after a refresh: rebuild the live round, or the podium
        if (d.round) startRound(d.round, false)
        else if (st.state === 'finished') matchResult.value = st.players ?? []
        // a refresh landing in a summary re-arms a local countdown — the
        // exact server stamp is gone with the old socket
        else if (st.state === 'summary') autoNextAt.value = Date.now() + 60_000
        // members joining mid-absence inherit the reconnect countdown;
        // the reclaiming host must not — host_back already lifted it and
        // re-arming it would flash the wait dialog on the host's screen
        if (d.hostReconnecting && d.you !== (st.host ?? '')) hostDeadline.value = d.hostReconnecting.endsAt
        break
      }
      case 'room_state':
        applyState(d)
        break
      case 'extended': {
        // this seat's clock widens: re-arm the countdown off the private
        // deadline and mirror the widened limit on the clock fill
        endsAt.value = d.endsAt
        timeLimit.value += d.extraSeconds ?? 0
        extendEndsAt.value = d.endsAt
        extendExtra.value = d.extraSeconds ?? 0
        extendUsedRound.value = true
        break
      }
      case 'round_result':
        stopTimer()
        // the round is over: the hint served its purpose and the merge
        // streak belongs to the finished round only
        hint.value = null
        combo.value = 0
        roundResult.value = d
        // the summary's stall-guard deadline: trust the server's stamp,
        // fall back to the known 60s window counted locally so the
        // countdown always renders
        autoNextAt.value = d.autoNextAt ?? Date.now() + 60_000
        state.value = 'summary'
        if (d.standings) players.value = mergePlayers(players.value, d.standings)
        // this seat's own solve decides the sting — winning the round now
        // means being among the ones who solved before the clock died
        {
          const mine = (d.standings ?? []).find((p) => p.id === you.value)
          if (mine?.gained > 0) sfx.win()
          else sfx.lose()
        }
        break
      case 'match_end':
        stopTimer()
        hostDeadline.value = 0
        state.value = 'finished'
        matchResult.value = d.standings ?? []
        hostLeftEnd.value = d.reason === 'host_left'
        // the host-left teardown removes the room server-side; the podium
        // is local from here on, so cut the socket instead of digging a
        // reconnect grave into a room that no longer exists
        if (hostLeftEnd.value) {
          storeResume(code.value, '')
          sock?.close()
        }
        break
      case 'hint':
        // stays on the board until the round ends — one reveal per round
        hintUsedRound.value = true
        hint.value = { ...d, at: Date.now() }
        sfx.click()
        break
      case 'regen':
        numbers.value = d.numbers
        hand.value = newHand(d.numbers)
        // the old hint describes the replaced hand
        hint.value = null
        regenUsedRound.value = true
        break
      case 'host_reconnecting':
        hostDeadline.value = d.endsAt
        break
      case 'achievements': {
        const list = Array.isArray(d.unlocked) ? d.unlocked : []
        if (!list.length) break
        const at = Date.now()
        const pops = list.map((a, i) => ({ ...a, key: `${a.id}:${at}:${i}` }))
        achievementPops.value = [...achievementPops.value, ...pops]
        for (const p of pops) {
          popTimers.push(setTimeout(() => {
            achievementPops.value = achievementPops.value.filter((x) => x.key !== p.key)
          }, 4600))
        }
        break
      }
      case 'host_back':
        hostDeadline.value = 0
        break
      case 'room_closed':
        // the host left for good: stop cleanly, show the closed notice —
        // unless the final podium is already up (the host-left match_end
        // lands right before the teardown and outranks the plain notice)
        stopTimer()
        hostDeadline.value = 0
        storeResume(code.value, '')
        sock?.close()
        if (state.value !== 'finished') state.value = 'gone'
        break
      case 'error':
        error.value = d.message ?? 'error'
        // a refusal before the seat even joined (e.g. the room is full)
        // ends the visit here — there is nothing to retry into
        if (!joinedOk) state.value = 'gone'
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
      const unchanged = old && ['name', 'guest', 'level', 'tier', 'score', 'wins', 'hintsLeft', 'extendsLeft', 'regensLeft', 'solved', 'solveOrder', 'timedOut', 'host', 'absent']
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
    extendExtra.value = 0
    // a fresh round_start carries no flags (new round, nothing spent yet);
    // a welcome resume carries the seat's per-round usage so a refresh
    // cannot unlock a helper twice in the same round
    hintUsedRound.value = !!d.hintUsed
    extendUsedRound.value = !!d.extendUsed
    regenUsedRound.value = !!d.regenUsed
    endsAt.value = d.endsAt
    timeLimit.value = d.timeLimit
    if (d.extendEndsAt) {
      // a private extension survived the refresh: play on the widened clock
      endsAt.value = d.extendEndsAt
      timeLimit.value += d.extraSeconds ?? 0
      extendEndsAt.value = d.extendEndsAt
    } else {
      extendEndsAt.value = 0
    }
    hand.value = newHand(d.numbers)
    roundResult.value = null
    hint.value = null
    combo.value = 0
    // a fresh round wipes any achievement toasts still on screen
    popTimers.forEach(clearTimeout)
    popTimers = []
    achievementPops.value = []
    state.value = 'round'
    remaining.value = Math.max(0, Math.ceil((endsAt.value - Date.now()) / 1000))
    intro.value = withIntro
    startTimer()
  }

  function startTimer() {
    stopTimer()
    let lastBeat = -1
    timer = setInterval(() => {
      remaining.value = Math.max(0, Math.ceil((endsAt.value - Date.now()) / 1000))
      // a seat that already solved keeps its clock for display only —
      // no more countdown beats while it waits out the round
      const done = !!players.value.find((p) => p.id === you.value)?.solved
      if (!done && remaining.value <= 10 && remaining.value > 0 && remaining.value !== lastBeat) {
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

  // the host's go signal between rounds: the server refuses anyone else
  function nextRound() {
    sock?.send('next')
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

  // Add-time: the room's own quota — the host set it at creation, identical
  // for every seat — and the server answers with `extended` (refusals arrive
  // as `error` messages). No inventory is touched in multiplayer.
  function extend() {
    if (state.value !== 'round' || intro.value || !hand.value) return
    sock?.send('extend')
  }

  function leave() {
    // announce the goodbye before cutting the socket: when the HOST's
    // leave lands, the server ends the match for every seat at once — a
    // refresh or a dropped socket never sends it, so the reconnect grace
    // stays untouched
    stopTimer()
    sock?.send('leave')
    sock?.close()
    popTimers.forEach(clearTimeout)
    popTimers = []
    achievementPops.value = []
    hint.value = null
    combo.value = 0
    state.value = 'closed'
  }

  // the host's confirmed goodbye mid-match: announce it and stay connected
  // — the server seals the room with the final podium (or the closure
  // notice), which the incoming messages render
  function leaveMatch() {
    sock?.send('leave')
  }

  return {
    state, code, you, players, host, config, roundNo, totalRounds,
    numbers, endsAt, remaining, timeLimit, hand, roundResult, matchResult, hint,
    wrongFlash, error, isHost, combo, intro, hostDeadline, achievementPops,
    extendEndsAt, extendExtra, hintUsedRound, extendUsedRound, regenUsedRound,
    autoNextAt, hostLeftEnd,
    open, start, nextRound, rename, pickCard, setOperator, undo, askHint, askRegen,
    extend, leave, leaveMatch, introDone,
  }
}
