// Layout math for the profile-halo scene — the golden particle ring and the
// rising spark dust that live behind the avatar in the mobile profile sheet.
// Pure on purpose: the ring is a flat circle in the XZ plane (the THREE.Points
// object itself applies the tilt and lift), and sparks carry their own base
// column so the drift can be advanced deterministically frame by frame.

export const HALO_DEFAULTS = Object.freeze({
  ringCount: 110,
  ringRadius: 2.6,
  sparkCount: 70,
  width: 7.5,
  height: 5,
  depth: 3,
})

/** N points evenly spread on a circle of `radius` in the XZ plane, centred at the origin. */
export function ringPositions(count, radius) {
  const out = new Float32Array(count * 3)
  for (let i = 0; i < count; i++) {
    const a = (i / count) * Math.PI * 2
    out[i * 3] = Math.cos(a) * radius
    out[i * 3 + 1] = 0
    out[i * 3 + 2] = Math.sin(a) * radius
  }
  return out
}

/**
 * A drifting spark field: every spark owns a base column (x, z) inside the
 * box, a rise speed and a sway phase. `positions` is what the Points geometry
 * reads; `advanceSparks` rewrites it each frame from the base columns.
 */
export function sparkField(count, width, height, depth, rng = Math.random) {
  const positions = new Float32Array(count * 3)
  const baseX = new Float32Array(count)
  const baseZ = new Float32Array(count)
  const speeds = new Float32Array(count)
  const phases = new Float32Array(count)
  for (let i = 0; i < count; i++) {
    baseX[i] = (rng() - 0.5) * width
    baseZ[i] = (rng() - 0.5) * depth
    positions[i * 3] = baseX[i]
    positions[i * 3 + 1] = rng() * height
    positions[i * 3 + 2] = baseZ[i]
    speeds[i] = 0.15 + rng() * 0.35 // world units per second
    phases[i] = rng() * Math.PI * 2
  }
  return { positions, baseX, baseZ, speeds, phases, height, width }
}

/**
 * Advance the spark field by `dt` seconds. Each spark rises by its speed,
 * sways sideways on a per-spark sine (never more than `sway` world units off
 * its base column) and wraps from the ceiling back to the floor.
 */
export function advanceSparks(field, dt, elapsed, { sway = 0.35, freq = 0.9 } = {}) {
  const { positions, baseX, baseZ, speeds, phases, height } = field
  for (let i = 0; i < speeds.length; i++) {
    let y = positions[i * 3 + 1] + speeds[i] * dt
    if (y >= height) y -= height
    positions[i * 3] = baseX[i] + Math.sin(elapsed * freq + phases[i]) * sway
    positions[i * 3 + 1] = y
  }
  return positions
}

/** Soft radial dot sprite for the Points materials. Caller owns disposal. */
export function softDotTexture(THREE) {
  const cv = document.createElement('canvas')
  cv.width = cv.height = 64
  const c = cv.getContext('2d')
  const g = c.createRadialGradient(32, 32, 2, 32, 32, 30)
  g.addColorStop(0, 'rgba(255,255,255,1)')
  g.addColorStop(0.4, 'rgba(255,255,255,0.55)')
  g.addColorStop(1, 'rgba(255,255,255,0)')
  c.fillStyle = g
  c.fillRect(0, 0, 64, 64)
  const tex = new THREE.CanvasTexture(cv)
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}
