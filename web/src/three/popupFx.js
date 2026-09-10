// A shared WebGL "glint" layer for the solo helper popups: one transparent
// full-viewport canvas that plays a short golden spark burst from wherever a
// reason bubble pops. The math half is pure and pinned by tests; the scene
// half follows the GameBackdrop discipline — three.js is dynamically imported
// (code-split), motion is dt-driven, resolution drops before frames do
// (qualityGuard), a hidden tab costs nothing, reduced motion never boots the
// canvas, and disposeGlint() releases every GPU resource. The layer sleeps
// (no rAF) between bursts, so an idle game pays nothing for it.

import { disposeDeep, qualityGuard } from './anim.js'
import { softDotTexture } from './profileHalo.js'

export const GLINT_DEFAULTS = Object.freeze({
  pool: 30, // particles per burst — the whole pool re-ignites at the new origin
  duration: 0.72, // seconds one burst lives before the layer sleeps again
  speedMin: 70, // launch speed range, px/s …
  speedMax: 190,
  spread: 1.15, // total fan width around straight up, radians
  gravity: 320, // px/s² — pulls sparks back down after the launch
  drag: 2.4, // e-folds per second of air drag on the launch velocity
  maxDelay: 0.08, // per-spark launch stagger window, seconds
})

/**
 * One burst's launch state in DOM pixel space (y grows DOWNWARD, matching
 * getBoundingClientRect — the scene's camera is an orthographic that maps
 * world units 1:1 onto screen pixels). Every spark leaves from the burst
 * point, fanned around straight up; `delays` staggers the launches so the
 * pop reads as a trickle, not a single blob.
 */
export function burstSparks(count, opts = GLINT_DEFAULTS, rng = Math.random) {
  const { speedMin, speedMax, spread, maxDelay } = opts
  const vx = new Float32Array(count)
  const vy = new Float32Array(count)
  const delays = new Float32Array(count)
  for (let i = 0; i < count; i++) {
    const a = (rng() - 0.5) * spread
    const s = speedMin + rng() * (speedMax - speedMin)
    vx[i] = Math.sin(a) * s
    vy[i] = -Math.cos(a) * s // negative = upward in y-down space
    delays[i] = rng() * maxDelay
  }
  return { vx, vy, delays }
}

/**
 * Advance one burst by `dt` seconds (`elapsed` since ignition, for the launch
 * delays). Launch velocity decays exponentially (air drag) while gravity bends
 * every spark back toward the ground; a spark still waiting on its delay sits
 * at its origin. Rewrites `state.positions` in place — that buffer is what
 * the Points geometry reads.
 */
export function advanceSparks(state, dt, elapsed, opts = GLINT_DEFAULTS) {
  const { positions, vx, vy, delays } = state
  const air = Math.exp(-opts.drag * dt)
  for (let i = 0; i < vx.length; i++) {
    if (elapsed < delays[i]) continue
    vx[i] *= air
    vy[i] = vy[i] * air + opts.gravity * dt
    positions[i * 3] += vx[i] * dt
    positions[i * 3 + 1] += vy[i] * dt
  }
  return positions
}

// ---------------------------------------------------------------------------
// The layer singleton: at most one canvas for the whole app, booted by the
// first glint() and torn down by disposeGlint() (called on the game view's
// unmount). A failed boot (no WebGL, SSR) resolves to null and the popups
// simply stay CSS-only.
let layer = null
let booting = null
let gen = 0

const reducedMotion = () =>
  typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

/** Fire a golden spark burst from a viewport point (clientX/clientY space). */
export async function glint(x, y) {
  if (reducedMotion()) return
  const myGen = gen
  if (!layer) {
    booting ??= boot(myGen)
    layer = await booting
    booting = null
    if (!layer) return
  }
  if (myGen !== gen) return // disposed while the boot was in flight
  ignite(layer, x, y)
}

/** Release the canvas and every GPU resource. Safe to call any time. */
export function disposeGlint() {
  gen++
  booting = null
  const L = layer
  layer = null
  if (!L) return
  cancelAnimationFrame(L.raf)
  window.removeEventListener('resize', L.resize)
  disposeDeep(L.scene)
  L.dotTex.dispose()
  L.renderer.dispose()
  L.cv.remove()
}

async function boot(myGen) {
  let THREE
  try {
    THREE = await import('three')
  } catch {
    return null
  }
  if (myGen !== gen) return null

  const cv = document.createElement('canvas')
  cv.style.cssText =
    'position:fixed;inset:0;width:100%;height:100%;display:block;pointer-events:none;z-index:80;'
  cv.setAttribute('aria-hidden', 'true')
  document.body.appendChild(cv)

  let renderer
  try {
    renderer = new THREE.WebGLRenderer({ canvas: cv, alpha: true, antialias: false, powerPreference: 'low-power' })
  } catch {
    cv.remove()
    return null
  }
  renderer.setClearAlpha(0)
  if (myGen !== gen) {
    renderer.dispose()
    cv.remove()
    return null
  }

  const scene = new THREE.Scene()
  // orthographic over the viewport with top=0/bottom=height: world (x, y) IS
  // DOM pixel space, so a burst origin is a clientX/clientY pair verbatim
  const camera = new THREE.OrthographicCamera(0, 1, 0, 1, -10, 10)

  const dotTex = softDotTexture(THREE)
  const state = { positions: new Float32Array(GLINT_DEFAULTS.pool * 3), ...burstSparks(GLINT_DEFAULTS.pool) }
  const geo = new THREE.BufferGeometry()
  const posAttr = new THREE.BufferAttribute(state.positions, 3)
  geo.setAttribute('position', posAttr)
  const mat = new THREE.PointsMaterial({
    color: 0xf6b73c,
    size: 7,
    map: dotTex,
    transparent: true,
    opacity: 0,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
    sizeAttenuation: false, // pixel-space scene: points keep their screen size
  })
  const points = new THREE.Points(geo, mat)
  points.frustumCulled = false
  points.visible = false
  scene.add(points)

  const sizePoints = (dpr) => (mat.size = 7 * dpr)
  const resize = () => {
    renderer.setSize(window.innerWidth, window.innerHeight, false)
    camera.right = window.innerWidth
    camera.bottom = window.innerHeight
    camera.updateProjectionMatrix()
  }
  window.addEventListener('resize', resize)
  resize()
  sizePoints(renderer.getPixelRatio())
  const q = qualityGuard(renderer, { max: 1.5, min: 0.6, onChange: sizePoints })

  const L = {
    renderer, scene, camera, geo, mat, points, posAttr, state, q, dotTex, cv, resize,
    raf: 0, last: 0, elapsed: 0,
  }

  L.frame = (now) => {
    if (layer !== L) return // disposed mid-flight
    L.raf = 0
    if (document.hidden) {
      L.last = now
      L.raf = requestAnimationFrame(L.frame)
      return
    }
    const dt = Math.min(0.1, (now - L.last) / 1000)
    L.last = now
    L.elapsed += dt
    L.q.sample(dt * 1000)
    advanceSparks(L.state, dt, L.elapsed)
    L.posAttr.needsUpdate = true
    const t = L.elapsed / GLINT_DEFAULTS.duration
    L.mat.opacity = Math.max(0, 1 - t) * 0.95
    L.renderer.render(L.scene, L.camera)
    if (t >= 1) {
      // burst over: sleep until the next ignite — zero rAF cost while idle
      L.points.visible = false
      L.mat.opacity = 0
      return
    }
    L.raf = requestAnimationFrame(L.frame)
  }

  return L
}

function ignite(L, x, y) {
  const { positions } = L.state
  for (let i = 0; i < GLINT_DEFAULTS.pool; i++) {
    positions[i * 3] = x
    positions[i * 3 + 1] = y
    positions[i * 3 + 2] = 0
  }
  L.elapsed = 0
  L.points.visible = true
  if (!L.raf) {
    L.last = performance.now()
    L.raf = requestAnimationFrame(L.frame)
  }
}
