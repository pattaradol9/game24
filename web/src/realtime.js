// Live profile socket: one websocket per signed-in tab, mirroring the room
// socket's envelope ({type, data}). The server pushes profile snapshots
// whenever something changes behind the player's back — admin EXP/coin/tier
// adjustments, achievement grants, renames — plus a `banned` notice when an
// admin bans the account (reason included), a `deleted` notice when the
// account is gone, and `boost` when a server-wide payout event starts, ends
// or is retuned, so every view that reads `currentPlayer` converges without
// a page refresh.
//
// Module import has no side effects: `initRealtime()` wires the session
// watcher once from main.js; the rest stays pure for tests.
import { watch } from 'vue'
import {
  currentPlayer,
  clearSession,
  getToken,
  markBanned,
  playerIdentityVersion,
  serverBoosts,
  updatePlayer,
} from './auth.js'

const WS_PATH = '/api/v1/ws/player'
const MAX_BACKOFF_MS = 30_000
// a socket must stay up this long before a drop counts as "connection was
// good": one that dies instantly must never reset the backoff, or the
// reconnect loop runs at full speed and trips the shared rate limit
const STABLE_MS = 10_000

let socket = null
let closedByUs = false
let retryTimer = null
let retries = 0
let openedAt = 0

/**
 * Reconnect delay for the nth failed attempt: full jitter around
 * exponential growth — 0.5–1.5s, 1–3s, 2–6s … hard-capped at 30s.
 */
export function nextBackoffMs(retries, rand = Math.random) {
  return Math.min(MAX_BACKOFF_MS, 1000 * 2 ** retries * (0.5 + rand()))
}

/** Apply one server push to the shared session state. */
export function applyEvent(type, data = {}) {
  if (type === 'player') {
    const before = currentPlayer.value
    const player = data.player
    if (!player) return
    updatePlayer(player)
    // an admin rename must invalidate cached nicknames too (leaderboard)
    if (before && player.nickname !== before.nickname) playerIdentityVersion.value++
  } else if (type === 'banned') {
    // an admin just banned this account: the dialog explains, the session
    // dies (the reason travels along for display)
    markBanned(data.reason || '')
  } else if (type === 'error' && data.message === 'account banned') {
    // the ban landed while our socket was down and the reconnect auth just
    // got the verdict — act exactly like a live ban push (markBanned also
    // stops the reconnect loop by emptying the session)
    markBanned()
  } else if (type === 'boost') {
    // a server-wide boost was armed, stopped or retuned — the payload is the
    // full server view, so replace that half wholesale; personal item
    // boosts are per-player and only ever travel in player snapshots
    serverBoosts.value = data.boosts ?? {}
  } else if (type === 'deleted') {
    // account vanished under us (admin delete or self-delete elsewhere)
    clearSession()
    playerIdentityVersion.value++
  }
}

function scheduleReopen() {
  if (retryTimer || !currentPlayer.value) return
  retryTimer = setTimeout(() => {
    retryTimer = null
    open()
  }, nextBackoffMs(retries))
  retries++
}

function open() {
  if (socket || !currentPlayer.value) return
  const token = getToken()
  if (!token) return
  closedByUs = false
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}${WS_PATH}`)
  socket = ws
  openedAt = Date.now()
  ws.onopen = () => {
    // the token travels in the first message, never in the URL
    send(ws, { type: 'auth', data: { token: getToken() } })
  }
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      if (msg?.type) applyEvent(msg.type, msg.data || {})
    } catch {
      /* ignore malformed frames */
    }
  }
  ws.onclose = () => {
    socket = null
    if (closedByUs) return
    if (Date.now() - openedAt >= STABLE_MS) retries = 0
    scheduleReopen()
  }
  ws.onerror = () => ws.close()
}

function send(ws, msg) {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg))
  }
}

function close() {
  closedByUs = true
  if (retryTimer) {
    clearTimeout(retryTimer)
    retryTimer = null
  }
  retries = 0
  socket?.close()
  socket = null
}

/** Wire the socket to the session lifecycle; call once from main.js. */
export function initRealtime() {
  watch(currentPlayer, (player) => {
    if (player) open()
    else close()
  })
  if (currentPlayer.value) open()
  // mobile browsers quietly kill idle sockets: reconnect eagerly when the
  // tab wakes up or the network comes back
  const revive = () => {
    if (currentPlayer.value && !socket && !retryTimer) {
      retries = 0
      open()
    }
  }
  window.addEventListener('online', revive)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') revive()
  })
}
