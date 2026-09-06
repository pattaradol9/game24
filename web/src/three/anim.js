// Frame-rate independent animation helpers for the Three.js backdrop.
// Everything here is driven by dt (seconds) so motion looks identical on a
// 60Hz laptop, a 120Hz phone and a machine dropping frames.

export const clamp = (v, lo, hi) => (v < lo ? lo : v > hi ? hi : v)

/**
 * Exponential smoothing that is stable at any frame rate.
 * `lambda` is the decay rate in e-folds per second: 4 is lazy, 20 is snappy.
 */
export function damp(current, target, lambda, dt) {
  return target + (current - target) * Math.exp(-lambda * dt)
}

/**
 * Cel-shading ramp for MeshToonMaterial: a 1px-tall lookup texture that turns
 * smooth lighting into a few flat bands.
 */
export function toonGradient(THREE, steps = [118, 156, 192, 224, 255]) {
  const tex = new THREE.DataTexture(new Uint8Array(steps), steps.length, 1, THREE.RedFormat)
  tex.minFilter = THREE.NearestFilter
  tex.magFilter = THREE.NearestFilter
  tex.generateMipmaps = false
  tex.needsUpdate = true
  return tex
}

/**
 * Trims render resolution when frames stop arriving on time — the cheapest way
 * to turn a stuttering scene into a smooth one, because dropping pixels is far
 * less visible than dropping frames.
 *
 * Fed the interval *between* frames rather than our own CPU time, since GPU
 * cost never shows up in a performance.now() reading taken after render().
 * The thresholds are absolute (≈38fps down, ≈55fps up); the only case they get
 * wrong is a genuine 30Hz display, which settles at the floor and looks fine.
 */
export function qualityGuard(renderer, { max = 2, min = 0.75, onChange } = {}) {
  const cap = Math.min(window.devicePixelRatio || 1, max)
  let dpr = cap
  let avg = 16
  let hold = 60 // ignore the first second: startup frames are always slow
  renderer.setPixelRatio(dpr)
  return {
    get dpr() { return dpr },
    /** @param ms milliseconds since the previous rendered frame */
    sample(ms) {
      avg += (clamp(ms, 1, 40) - avg) * 0.05
      if (hold-- > 0) return
      const want = avg > 26 ? dpr * 0.8 : avg < 18 ? dpr * 1.15 : dpr
      const next = clamp(Math.round(want * 20) / 20, min, cap)
      if (Math.abs(next - dpr) < 0.04) return
      dpr = next
      renderer.setPixelRatio(dpr)
      onChange?.(dpr)
      hold = 90 // let the change settle before judging again
    },
  }
}

/** Releases every GPU resource under a node. Safe to call twice. */
export function disposeDeep(root) {
  root.traverse((n) => {
    n.geometry?.dispose?.()
    const mats = Array.isArray(n.material) ? n.material : n.material ? [n.material] : []
    for (const m of mats) {
      for (const key of ['map', 'gradientMap', 'alphaMap', 'emissiveMap']) m[key]?.dispose?.()
      m.dispose?.()
    }
  })
}
