// Session persistence + Google Identity Services integration.
// Player state is a reactive ref so header/profile UI updates live.
import { computed, ref } from 'vue'
import { api } from './api.js'

const TOKEN_KEY = 'g24_token'

export const currentPlayer = ref(null)

/**
 * The running server-wide boosts, keyed by payout kind — `{ exp: {...},
 * coins: {...} }`, each `{ kind, multiplier, endsAt }`. Kept in sync from
 * every player snapshot and from `boost` pushes on the realtime socket, so
 * any open tab sees an event start and end without a refresh.
 */
export const serverBoosts = ref({})

/**
 * The player's own item boosts (same shape as the server-wide ones), armed
 * by using an inventory item. They ride in every player snapshot's
 * `boosts.sources`, so the split happens in `applyBoostSources`.
 */
export const itemBoosts = ref({})

/**
 * The combined active-boost view keyed by kind: every running source —
 * server-wide and personal — stacked additively (`multiplier` =
 * 1 + Σ(source − 1)), with `sources` naming where each bonus came from and
 * `endsAt` the nearest change point (when the stack next thins out).
 */
export const activeBoosts = computed(() => mergeBoosts(serverBoosts.value, itemBoosts.value))

/**
 * Split a player-JSON boosts map (`{ kind: { sources: [...] } }`) back into
 * its server-wide and personal halves. Item sources carry their item id,
 * rarity and bilingual name through, so the tray can colour and explain
 * itself. Pure, so tests can pin the shape.
 */
export function splitBoosts(map = {}) {
  const server = {}
  const item = {}
  for (const b of Object.values(map)) {
    for (const s of b.sources ?? []) {
      const view = { kind: b.kind, multiplier: s.multiplier, endsAt: s.endsAt }
      if (s.item) view.item = s.item
      if (s.rarity) view.rarity = s.rarity
      if (s.name) view.name = s.name
      ;(s.origin === 'item' ? item : server)[b.kind] = view
    }
  }
  return { server, item }
}

/**
 * Stack two boost maps of the same shape additively per kind. Pure — the
 * buff tray calls it every second with the still-alive entries so a source
 * that expires simply stops contributing while the rest keep running.
 */
export function mergeBoosts(server = {}, item = {}) {
  const out = {}
  for (const [origin, map] of [['server', server], ['item', item]]) {
    for (const [kind, b] of Object.entries(map)) {
      const e = out[kind] ??= { kind, multiplier: 1, endsAt: b.endsAt, sources: [] }
      e.multiplier += b.multiplier - 1
      if (b.endsAt < e.endsAt) e.endsAt = b.endsAt
      const src = { origin, multiplier: b.multiplier, endsAt: b.endsAt }
      if (b.item) src.item = b.item
      if (b.rarity) src.rarity = b.rarity
      if (b.name) src.name = b.name
      e.sources.push(src)
    }
  }
  return out
}

function applyBoostSources(map) {
  const { server, item } = splitBoosts(map)
  serverBoosts.value = server
  itemBoosts.value = item
}

/** Rarity tiers, low → high (mirrors the server's items catalog). */
export const RARITY_ORDER = ['common', 'rare', 'epic', 'legend']

/**
 * The tray colour identity of one stacked cell: pure server campaigns get
 * the reserved server teal, anything an item feeds takes the highest rarity
 * among its item sources — so a cell reads its pedigree at a glance.
 * Pure, for tests.
 */
export function boostColorKey(sources = []) {
  const items = sources.filter((s) => s.origin === 'item')
  if (!items.length) return 'server'
  return items
    .map((s) => s.rarity || 'common')
    .sort((a, b) => RARITY_ORDER.indexOf(b) - RARITY_ORDER.indexOf(a))[0]
}

/**
 * Bumped whenever the player's public identity changes (rename, Google
 * sign-in). Views that render cached copies of the nickname — the
 * leaderboard — watch this and refetch.
 */
export const playerIdentityVersion = ref(0)

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function getPlayer() {
  return currentPlayer.value
}

export function setSession(token, player) {
  localStorage.setItem(TOKEN_KEY, token)
  currentPlayer.value = player
  applyBoostSources(player?.boosts ?? {})
}

/** Refresh the signed-in player after a round banks its EXP. */
export function updatePlayer(player) {
  if (player) {
    currentPlayer.value = player
    applyBoostSources(player.boosts ?? {})
  }
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  currentPlayer.value = null
  serverBoosts.value = {}
  itemBoosts.value = {}
}

/**
 * Bumped when any chrome (the mobile bottom bar's profile item, for one)
 * needs the nickname entry modal but lives outside the page that owns it —
 * Home watches this and opens the modal.
 */
export const namePromptTick = ref(0)

export function requestPlayerName() {
  namePromptTick.value++
}

/**
 * Set while the ban dialog should be on screen: `{ reason }` after the live
 * socket announces a ban (or a session restore bumps into one), null
 * otherwise. Lives here — not in realtime.js — so session restore can set
 * it without a circular import; BannedModal renders it.
 */
export const banNotice = ref(null)

/** Surface the ban dialog and drop the (now dead) session. */
export function markBanned(reason = '') {
  banNotice.value = { reason }
  clearSession()
  playerIdentityVersion.value++
}

export async function restoreSession() {
  const token = getToken()
  if (!token) return null
  try {
    const data = await api.me(token)
    currentPlayer.value = data.player
    return currentPlayer.value
  } catch (err) {
    clearSession()
    // the token died with a ban while we were away — explain, don't shrug
    if (err?.message === 'account banned') markBanned()
    return null
  }
}

export async function signInAsGuest(nickname) {
  const { token, player } = await api.createGuest(nickname)
  setSession(token, player)
  return player
}

export async function renamePlayer(nickname) {
  const token = getToken()
  const data = await api.renameMe(nickname, token)
  currentPlayer.value = data.player
  playerIdentityVersion.value++
  return currentPlayer.value
}

/**
 * Permanently deletes the signed-in account server-side, then clears the
 * local session. Irreversible — the API demands the exact confirmation word.
 */
export async function deleteAccount() {
  await api.deleteMe('DELETE', getToken())
  clearSession()
  playerIdentityVersion.value++
}

export async function signInWithGoogle(credential) {
  try {
    const { token, player } = await api.googleAuth(credential)
    setSession(token, player)
    playerIdentityVersion.value++
    return player
  } catch (err) {
    // a banned account can't sign in at all — raise the same prominent ban
    // dialog as a live ban instead of a one-line error under the button
    if (err?.message === 'account banned') markBanned()
    throw err
  }
}

// Google Identity Services button; googleClientId comes from GET /meta.
let googleClientId = ''
export async function initGoogle() {
  if (googleClientId) return googleClientId
  const meta = await api.meta()
  googleClientId = meta.googleClientId || ''
  return googleClientId
}

export function renderGoogleButton(el, onCredential, options = {}) {
  if (!googleClientId || !window.google?.accounts?.id) return false
  window.google.accounts.id.initialize({
    client_id: googleClientId,
    callback: (res) => onCredential(res.credential),
  })
  window.google.accounts.id.renderButton(el, {
    // GoogleSignIn.vue hides this rendered button behind its own styled
    // layer — theme/text only matter for the invisible hit area; width
    // must match so clicks land everywhere on the drawn button
    theme: 'filled_black',
    size: 'large',
    shape: 'pill',
    text: 'signin_with',
    locale: document.documentElement.lang || 'th',
    ...options,
  })
  return true
}
