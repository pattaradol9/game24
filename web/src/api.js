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
  createRound: (mode, token) => request('/rounds', { method: 'POST', body: { mode }, token }),
  submitRound: (roundId, steps, token) =>
    request(`/rounds/${roundId}/submit`, { method: 'POST', body: { steps }, token }),
  skipRound: (roundId, token) => request(`/rounds/${roundId}/skip`, { method: 'POST', body: {}, token }),
  hintRound: (roundId, token) => request(`/rounds/${roundId}/hint`, { method: 'POST', body: {}, token }),
  leaderboard: (mode, limit = 50) => request(`/leaderboard?mode=${mode}&limit=${limit}`),
  createRoom: (cfg, token) => request('/rooms', { method: 'POST', body: cfg, token }),
  roomInfo: (code) => request(`/rooms/${code}`),
}
