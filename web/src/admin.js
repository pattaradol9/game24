// Admin portal API client. Admins authenticate like any player (Google
// sign-in); the regular player session token is what unlocks /admin while
// the account's email is on the server's ADMIN_EMAILS allowlist.
import { request } from './api.js'
import { getToken } from './auth.js'

function adminRequest(path, { method = 'GET', body } = {}) {
  return request(path, { method, body, token: getToken() })
}

function query(params) {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== '' && v !== null && v !== undefined) q.set(k, v)
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}

export const adminApi = {
  session: () => adminRequest('/admin/session'),
  overview: () => adminRequest('/admin/overview'),
  players: ({ search = '', filter = '', limit = 50, offset = 0 } = {}) =>
    adminRequest('/admin/players' + query({ search, filter, limit, offset })),
  playerDetail: (id) => adminRequest(`/admin/players/${id}`),
  playerRounds: (id, { limit = 50, offset = 0 } = {}) =>
    adminRequest(`/admin/players/${id}/rounds` + query({ limit, offset })),
  patchPlayer: (id, body) => adminRequest(`/admin/players/${id}`, { method: 'PATCH', body }),
  deletePlayer: (id) => adminRequest(`/admin/players/${id}`, { method: 'DELETE' }),
  adjustExp: (id, mode, delta) =>
    adminRequest(`/admin/players/${id}/exp`, { method: 'POST', body: { mode, delta } }),
  resetStats: (id, mode) =>
    adminRequest(`/admin/players/${id}/stats/reset`, { method: 'POST', body: mode ? { mode } : {} }),
  leaderboard: (mode, { limit = 100, offset = 0 } = {}) =>
    adminRequest('/admin/leaderboard' + query({ mode, limit, offset })),
  rounds: ({ mode = '', status = '', limit = 50, offset = 0 } = {}) =>
    adminRequest('/admin/rounds' + query({ mode, status, limit, offset })),
  events: ({ action = '', limit = 50, offset = 0 } = {}) =>
    adminRequest('/admin/events' + query({ action, limit, offset })),
  settings: () => adminRequest('/admin/settings'),
  resetDb: () => adminRequest('/admin/db/reset', { method: 'POST', body: { confirm: 'RESET' } }),
}

/** Streams a database snapshot as a Blob download (binary, not JSON). */
export async function downloadDbBackup() {
  const res = await fetch('/api/v1/admin/db/backup', {
    headers: { Authorization: `Bearer ${getToken()}` },
  })
  if (!res.ok) throw new Error(`backup failed: HTTP ${res.status}`)
  const blob = await res.blob()
  const cd = res.headers.get('Content-Disposition') || ''
  const name = cd.match(/filename="?([^";]+)"?/)?.[1] || 'game24-backup.db'
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
  return name
}
