// Session persistence + Google Identity Services integration.
// Player state is a reactive ref so header/profile UI updates live.
import { ref } from 'vue'
import { api } from './api.js'

const TOKEN_KEY = 'g24_token'

export const currentPlayer = ref(null)

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
}

/** Refresh the signed-in player after a round banks its EXP. */
export function updatePlayer(player) {
  if (player) currentPlayer.value = player
}

export function clearSession() {
  localStorage.removeItem(TOKEN_KEY)
  currentPlayer.value = null
}

export async function restoreSession() {
  const token = getToken()
  if (!token) return null
  try {
    const data = await api.me(token)
    currentPlayer.value = data.player
    return currentPlayer.value
  } catch {
    clearSession()
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

export async function signInWithGoogle(credential) {
  const { token, player } = await api.googleAuth(credential)
  setSession(token, player)
  playerIdentityVersion.value++
  return player
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
    theme: 'outline',
    size: 'large',
    shape: 'pill',
    text: 'signin_with',
    locale: document.documentElement.lang || 'th',
    ...options,
  })
  return true
}
