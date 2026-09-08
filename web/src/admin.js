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
  setExp: (id, mode, value) =>
    adminRequest(`/admin/players/${id}/exp`, { method: 'POST', body: { mode, value } }),
  setCoins: (id, value) =>
    adminRequest(`/admin/players/${id}/coins`, { method: 'POST', body: { value } }),
  setTier: (id, tier) =>
    adminRequest(`/admin/players/${id}/tier`, { method: 'POST', body: { tier } }),
  grantAchievement: (id, achievementId) =>
    adminRequest(`/admin/players/${id}/achievements/grant`, { method: 'POST', body: { id: achievementId } }),
  revokeAchievement: (id, achievementId) =>
    adminRequest(`/admin/players/${id}/achievements/revoke`, { method: 'POST', body: { id: achievementId } }),
  achievementCatalog: () => adminRequest('/achievements'),
  resetStats: (id, mode) =>
    adminRequest(`/admin/players/${id}/stats/reset`, { method: 'POST', body: mode ? { mode } : {} }),
  leaderboard: (mode, { limit = 100, offset = 0 } = {}) =>
    adminRequest('/admin/leaderboard' + query({ mode, limit, offset })),
  rounds: ({ mode = '', status = '', limit = 50, offset = 0 } = {}) =>
    adminRequest('/admin/rounds' + query({ mode, status, limit, offset })),
  events: ({ action = '', limit = 50, offset = 0 } = {}) =>
    adminRequest('/admin/events' + query({ action, limit, offset })),
  boosts: () => adminRequest('/admin/boosts'),
  boost: (kind) => adminRequest(`/admin/boosts/${kind}`),
  saveBoost: (kind, { multiplier, durationMinutes, label }) =>
    adminRequest(`/admin/boosts/${kind}`, { method: 'PUT', body: { multiplier, durationMinutes, label } }),
  enableBoost: (kind) => adminRequest(`/admin/boosts/${kind}/enable`, { method: 'POST', body: {} }),
  disableBoost: (kind) => adminRequest(`/admin/boosts/${kind}/disable`, { method: 'POST', body: {} }),
  settings: () => adminRequest('/admin/settings'),
  listDbBackups: () => adminRequest('/admin/db/backups'),
  restoreDb: (name) => adminRequest('/admin/db/restore', { method: 'POST', body: { name, confirm: 'RESTORE' } }),
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

/** Uploads a snapshot file (multipart) and restores the database from it. */
export async function restoreDbUpload(file) {
  const form = new FormData()
  form.set('confirm', 'RESTORE')
  form.set('file', file)
  const res = await fetch('/api/v1/admin/db/restore', {
    method: 'POST',
    headers: { Authorization: `Bearer ${getToken()}` },
    body: form,
  })
  let json
  try {
    json = await res.json()
  } catch {
    throw new Error(`restore failed: HTTP ${res.status}`)
  }
  if (!json.success) {
    const err = new Error(json.message || `HTTP ${res.status}`)
    err.status = res.status
    throw err
  }
  return json.data
}
