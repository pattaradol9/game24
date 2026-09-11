<script setup>
// 3D icon for the host-reconnecting notice: a broken chain drawn like the
// classic "link broken" glyph — two toon-shaded chain halves on a diagonal,
// their torn stubs facing across the gap. The loop tells the story: the
// halves drift apart, wind up, snap back together, then break apart again
// with a spark flash, a shockwave ring and the glyph's radiating burst
// strokes. "Trying to reconnect" made literal.
//
// Same conventions as GameBackdrop: motion is dt-driven so it looks
// identical at any refresh rate, drops to a single still frame under
// prefers-reduced-motion (frozen mid-burst, matching the glyph), and
// everything is disposed on unmount. Falls back to the plain CSS pulse
// when WebGL or the three module is missing.
import { onMounted, onUnmounted, ref } from 'vue'
import { disposeDeep, toonGradient } from '../three/anim.js'

const W = 150
const H = 96
const CYCLE = 2.8 // seconds per connect attempt
const ANG = Math.PI / 4 // the chain sits on the glyph's diagonal
const REST = 1.24 // half rest position: stubs well apart
const SEAT = 0.71 // half seated position: torn stubs just touch
// timeline fractions: drift apart → wind up → snap together → hold → break
const T_REST = 0.34
const T_PULL = 0.1
const T_SNAP = 0.1
const T_HOLD = 0.16
// the remaining 0.3 is the break recoil

const cv = ref(null)
const ready = ref(false)

let cleanup = null

// one soft round glow shared by the halo and every mote
function glowTexture(THREE) {
  const c = document.createElement('canvas')
  c.width = c.height = 64
  const g = c.getContext('2d')
  const grad = g.createRadialGradient(32, 32, 0, 32, 32, 32)
  grad.addColorStop(0, 'rgba(255,255,255,1)')
  grad.addColorStop(0.35, 'rgba(255,255,255,0.5)')
  grad.addColorStop(1, 'rgba(255,255,255,0)')
  g.fillStyle = grad
  g.fillRect(0, 0, 64, 64)
  const tex = new THREE.CanvasTexture(c)
  tex.colorSpace = THREE.SRGBColorSpace
  return tex
}

onMounted(async () => {
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

  let THREE
  try {
    THREE = await import('three')
  } catch { return }

  let renderer
  try {
    renderer = new THREE.WebGLRenderer({ canvas: cv.value, alpha: true, antialias: true })
  } catch { return }
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  renderer.setSize(W, H, false)

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(38, W / H, 0.1, 20)
  camera.position.set(0, 0, 5.2)
  camera.lookAt(0, 0, 0)

  scene.add(new THREE.HemisphereLight(0xdfe8ff, 0x241a4d, 1.15))
  const key = new THREE.DirectionalLight(0xffffff, 1.9)
  key.position.set(3, 4, 5)
  scene.add(key)
  // warm rim from the gap side so the torn edges catch light
  const rim = new THREE.DirectionalLight(0xffc46b, 0.7)
  rim.position.set(0, -3, 4)
  scene.add(rim)

  const gradient = toonGradient(THREE)
  const toon = (color, extra = {}) =>
    new THREE.MeshToonMaterial({ color, gradientMap: gradient, ...extra })

  /* ------------------------------------------------------- chain halves */
  // one half = oval link + the broken stub of its bar, torn tip toward the
  // gap; the second half is the same build rotated a half turn
  const chain = new THREE.Group()
  chain.rotation.z = ANG
  scene.add(chain)

  const ringGeo = new THREE.TorusGeometry(0.27, 0.105, 14, 36)
  const stubGeo = new THREE.CylinderGeometry(0.075, 0.1, 0.42, 12)
  const jagGeo = new THREE.ConeGeometry(0.05, 0.13, 8)
  const ringMat = toon(0x8b5cf6, { emissive: 0x241d5e })
  const stubMat = toon(0x7c4fe0)

  function buildHalf() {
    const half = new THREE.Group()
    const ring = new THREE.Mesh(ringGeo, ringMat)
    ring.scale.set(1.5, 1, 1) // oval stretched along the chain axis
    ring.position.x = 0.15
    half.add(ring)
    // the stub: bar remnant pointing at the gap with a torn, jagged tip
    const stub = new THREE.Mesh(stubGeo, stubMat)
    stub.rotation.z = Math.PI / 2 // narrow end toward the gap
    stub.position.x = -0.51
    half.add(stub)
    const jags = [
      { p: [-0.73, -0.045, 0.02], r: 2.25, s: 1 },
      { p: [-0.75, 0.03, -0.02], r: 2.8, s: 1 },
      { p: [-0.71, 0.0, 0.03], r: 2.5, s: 1.25 },
    ]
    for (const j of jags) {
      const jag = new THREE.Mesh(jagGeo, stubMat)
      jag.position.set(...j.p)
      jag.rotation.z = j.r
      jag.scale.setScalar(j.s)
      half.add(jag)
    }
    return half
  }

  const halfR = buildHalf()
  chain.add(halfR)
  const halfL = buildHalf()
  halfL.rotation.z = Math.PI // mirror: stub now faces +x
  chain.add(halfL)

  /* --------------------------------------------- burst strokes & effects */
  // the glyph's radiating strokes: two fans of three in the gap, flashing
  // at the moment the chain breaks apart
  const strokeMat = new THREE.MeshBasicMaterial({
    color: 0xffd27a,
    transparent: true,
    opacity: 0,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
  })
  const strokeGeo = new THREE.CylinderGeometry(0.032, 0.032, 0.34, 8)
  const strokes = []
  for (const fan of [0, Math.PI]) {
    for (const off of [-0.55, 0, 0.55]) {
      const a = fan + off
      const s = new THREE.Mesh(strokeGeo, strokeMat)
      s.position.set(Math.cos(a) * 0.62, Math.sin(a) * 0.62, 0.12)
      s.rotation.z = a - Math.PI / 2 // long axis pointing radially
      strokes.push(s)
      chain.add(s)
    }
  }

  // shockwave ring expanding over the break point
  const ringFxMat = new THREE.MeshBasicMaterial({
    color: 0xffd27a,
    transparent: true,
    opacity: 0,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
  })
  const ringFx = new THREE.Mesh(new THREE.RingGeometry(0.3, 0.38, 40), ringFxMat)
  ringFx.position.z = 0.18
  scene.add(ringFx)

  // flash light that pops the whole scene for the instant of the break
  const flash = new THREE.PointLight(0xffc46b, 0, 5, 2)
  flash.position.set(0, 0, 0.9)
  scene.add(flash)

  /* ------------------------------------------------- halo & drifting motes */
  const glowTex = glowTexture(THREE)
  const makeSprite = (color, opacity) => {
    const s = new THREE.Sprite(
      new THREE.SpriteMaterial({
        map: glowTex,
        color,
        transparent: true,
        opacity,
        blending: THREE.AdditiveBlending,
        depthWrite: false,
      })
    )
    scene.add(s)
    return s
  }
  const halo = makeSprite(0x7c5cf0, 0.3)
  halo.position.set(0, 0, -0.9)
  halo.scale.set(5.2, 3.4, 1)
  const breakGlow = makeSprite(0xffb347, 0)
  breakGlow.position.set(0, 0, -0.2)
  breakGlow.scale.set(2.3, 1.8, 1)

  const motes = []
  for (let i = 0; i < 8; i++) {
    const size = 0.05 + Math.random() * 0.08
    const m = makeSprite(i % 3 ? 0x9f8bff : 0xffd27a, 0.1 + Math.random() * 0.16)
    m.scale.set(size, size, 1)
    motes.push({
      s: m,
      x: -2.6 + Math.random() * 5.2,
      y: -1.6 + Math.random() * 3.2,
      v: 0.08 + Math.random() * 0.14,
      amp: 0.05 + Math.random() * 0.1,
      ph: Math.random() * Math.PI * 2,
    })
  }

  /* -------------------------------------------------------------- pose */
  const easeOut = (t) => 1 - Math.pow(1 - t, 3)
  // recoil: flies out fast, overshoots the rest point, settles back
  const easeOutBack = (t) => {
    const c = 1.2
    return 1 + (c + 1) * Math.pow(t - 1, 3) + c * Math.pow(t - 1, 2)
  }

  // x position of the right half across the cycle (the left half mirrors it)
  function halfX(k) {
    if (k < T_REST) return REST
    if (k < T_REST + T_PULL) {
      const q = (k - T_REST) / T_PULL
      return REST + 0.08 * easeOut(q) // wind up: pull a little further apart
    }
    if (k < T_REST + T_PULL + T_SNAP) {
      const q = (k - T_REST - T_PULL) / T_SNAP
      const from = REST + 0.08
      return from - (from - SEAT) * q * q // accelerating snap into contact
    }
    if (k < T_REST + T_PULL + T_SNAP + T_HOLD) return SEAT
    const q = (k - T_REST - T_PULL - T_SNAP - T_HOLD) / (1 - T_REST - T_PULL - T_SNAP - T_HOLD)
    return SEAT + (REST - SEAT) * easeOutBack(q) // the break
  }

  // the pose is a pure function of time, so the reduced-motion still frame
  // and the animated loop share one code path
  function applyPose(t) {
    const k = (t / CYCLE) % 1
    const x = halfX(k)

    // spark bell over the first 55% of the break recoil
    let spark = 0
    if (k >= T_REST + T_PULL + T_SNAP + T_HOLD) {
      const q = (k - T_REST - T_PULL - T_SNAP - T_HOLD) / (1 - T_REST - T_PULL - T_SNAP - T_HOLD)
      spark = Math.sin(Math.PI * Math.min(q / 0.55, 1))
    }
    // faint warm breathing while the halves sit connected
    const seated =
      k >= T_REST + T_PULL + T_SNAP && k < T_REST + T_PULL + T_SNAP + T_HOLD
        ? Math.sin(
            Math.PI * ((k - T_REST - T_PULL - T_SNAP) / T_HOLD)
          ) * 0.14
        : 0

    halfR.position.x = x
    halfL.position.x = -x
    halfR.position.y = Math.sin(t * 1.7) * 0.045
    halfL.position.y = Math.sin(t * 1.7 + 2.1) * 0.045
    halfR.rotation.z = Math.sin(t * 1.3) * 0.05
    halfL.rotation.z = Math.PI + Math.sin(t * 1.3 + 1.4) * 0.05
    const sx = 1 + spark * 0.06
    const sy = 1 - spark * 0.04
    halfR.scale.set(sx, sy, sy)
    halfL.scale.set(sx, sy, sy)

    const bursting = spark > 0.01
    for (const s of strokes) {
      s.visible = bursting
      s.scale.set(1, 0.5 + spark * 0.6, 1)
    }
    strokeMat.opacity = spark * 0.95
    ringFx.visible = bursting
    const e = easeOut(Math.min(q5Of(k) / 0.55, 1))
    ringFx.scale.setScalar(0.35 + e * 2.3)
    ringFxMat.opacity = (1 - e) * 0.8
    flash.intensity = spark * 14
    halo.material.opacity = 0.26 + Math.sin(t * 1.3) * 0.07 + spark * 0.16
    breakGlow.material.opacity = seated + spark * 0.5
  }

  // break-recoil progress 0..1 for the current cycle phase
  function q5Of(k) {
    const start = T_REST + T_PULL + T_SNAP + T_HOLD
    return k < start ? 0 : (k - start) / (1 - start)
  }

  // motes are the one dt-driven layer: they drift on even between attempts
  function driftMotes(dt, t) {
    for (const m of motes) {
      m.y += m.v * dt
      if (m.y > 1.8) {
        m.y = -1.8
        m.x = -2.6 + Math.random() * 5.2
      }
      m.s.position.set(m.x + Math.sin(t * 0.7 + m.ph) * m.amp, m.y, -0.2)
    }
  }

  // start mid-burst: the animated loop opens on the break, and the
  // reduced-motion still frame freezes on the glyph's iconic moment
  let elapsed = CYCLE * (T_REST + T_PULL + T_SNAP + T_HOLD + 0.0825)
  let last = 0

  function frame(now) {
    const dt = last ? Math.min((now - last) / 1000, 1 / 15) : 1 / 60
    last = now
    if (document.hidden) return
    elapsed += dt
    applyPose(elapsed)
    driftMotes(dt, elapsed)
    renderer.render(scene, camera)
  }
  const onVisible = () => { last = 0 } // discard time spent in a hidden tab

  if (reduced) {
    applyPose(elapsed)
    driftMotes(0, elapsed)
    renderer.render(scene, camera)
  } else {
    document.addEventListener('visibilitychange', onVisible)
    renderer.setAnimationLoop(frame)
  }

  cleanup = () => {
    renderer.setAnimationLoop(null)
    document.removeEventListener('visibilitychange', onVisible)
    disposeDeep(scene)
    gradient.dispose()
    glowTex.dispose()
    renderer.dispose()
  }
  ready.value = true
})

onUnmounted(() => cleanup?.())
</script>

<template>
  <span class="beacon" :style="{ width: W + 'px', height: H + 'px' }" aria-hidden="true">
    <canvas ref="cv" v-show="ready" class="beacon-cv" />
    <span v-if="!ready" class="pulse" />
  </span>
</template>

<style scoped>
.beacon { display: block; flex: none; }
.beacon-cv { display: block; width: 100%; height: 100%; }
.pulse {
  display: block;
  width: 12px;
  height: 12px;
  margin: 36px auto;
  border-radius: var(--r-full);
  background: var(--accent);
  animation: beacon-pulse 1.1s var(--ease) infinite;
}
@keyframes beacon-pulse {
  0%, 100% { opacity: 0.35; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1.15); }
}
</style>
