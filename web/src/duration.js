/**
 * Compact countdown label used by every boost timer (buff tray, boosts menu,
 * inventory chips): whole hours while there are any — "24h", "3h" — then
 * whole minutes ("59m"), then whole seconds in the last minute ("59s").
 * Zero/negative spans collapse to "0s" so an expired window reads clean and
 * the label never jumps between units mid-tick.
 */
export function fmtDuration(seconds) {
  const s = Math.max(0, Math.floor(seconds))
  if (s >= 3600) return `${Math.floor(s / 3600)}h`
  if (s >= 60) return `${Math.floor(s / 60)}m`
  return `${s}s`
}
