// Shared formatting helpers for the admin tables.
export const fmt = (n) => (n ?? 0).toLocaleString('en-US')

export const fmtDate = (iso) => (iso ? new Date(iso).toLocaleString() : '—')

export const fmtDay = (day) => day // signup days arrive as YYYY-MM-DD already

export const fmtDuration = (ms) => {
  if (!ms && ms !== 0) return '—'
  if (ms < 1000) return `${ms}ms`
  const s = ms / 1000
  return s < 60 ? `${s.toFixed(1)}s` : `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`
}

export const shortId = (id) => (id ? `${id.slice(0, 8)}…` : '—')
