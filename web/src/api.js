// REST client: all responses use { success, message, data }
const BASE = '/api/v1'

export async function request(path, { method = 'GET', body, token } = {}) {
  const headers = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })
  let json
  try {
    json = await res.json()
  } catch {
    throw new Error(`HTTP ${res.status}`)
  }
  if (!json.success) {
    const err = new Error(json.message || `HTTP ${res.status}`)
    err.status = res.status
    throw err
  }
  return json.data
}

export const api = {
  meta: () => request('/meta'),
  createGuest: (nickname) => request('/players', { method: 'POST', body: { nickname } }),
  googleAuth: (credential) => request('/auth/google', { method: 'POST', body: { credential } }),
  me: (token) => request('/me', { token }),
  renameMe: (nickname, token) => request('/me/rename', { method: 'POST', body: { nickname }, token }),
  deleteMe: (confirm, token) => request('/me', { method: 'DELETE', body: { confirm }, token }),
  createRound: (mode, token, sessionId) =>
    request('/rounds', { method: 'POST', body: { mode, sessionId }, token }),
  submitRound: (roundId, steps, token) =>
    request(`/rounds/${roundId}/submit`, { method: 'POST', body: { steps }, token }),
  skipRound: (roundId, token) => request(`/rounds/${roundId}/skip`, { method: 'POST', body: {}, token }),
  // free fold for a hand whose countdown genuinely ran out (the Skip button
  // itself goes through the item-gated skipRound)
  timeoutRound: (roundId, token) => request(`/rounds/${roundId}/timeout`, { method: 'POST', body: {}, token }),
  hintRound: (roundId, token) => request(`/rounds/${roundId}/hint`, { method: 'POST', body: {}, token }),
  extendRound: (roundId, token) => request(`/rounds/${roundId}/extend`, { method: 'POST', body: {}, token }),
  leaderboard: (mode, period = 'alltime', limit = 50) =>
    request(`/leaderboard?mode=${mode}&period=${period}&limit=${limit}`),
  createRoom: (cfg, token) => request('/rooms', { method: 'POST', body: cfg, token }),
  roomInfo: (code) => request(`/rooms/${code}`),
  achievements: (token) => request('/achievements', { token }),
  skins: (token) => request('/skins', { token }),
  buySkin: (id, token) => request('/skins/' + id + '/buy', { method: 'POST', body: {}, token }),
  equipSkin: (id, token) => request('/skins/' + id + '/equip', { method: 'POST', body: {}, token }),
  items: (token) => request('/items', { token }),
  buyItem: (id, token, count = 1) => request('/items/' + id + '/buy', { method: 'POST', body: { count }, token }),
  useItem: (id, token, count = 1) => request('/items/' + id + '/use', { method: 'POST', body: { count }, token }),
}
