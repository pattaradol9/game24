<script setup>
// 3D icon for the host-reconnecting notice: a toon-shaded power plug that
// keeps trying to reach its wall socket — it approaches, sparks on contact,
// falls back and retries, over and over. "Trying to connect" made literal.
//
// Same conventions as GameBackdrop: motion is dt-driven so it looks
// identical at any refresh rate, drops to a single still frame under
// prefers-reduced-motion, and everything is disposed on unmount. Falls
// back to the plain CSS pulse when WebGL or the three module is missing.
import { onMounted, onUnmounted, ref } from 'vue'
import { disposeDeep, toonGradient } from '../three/anim.js'

const W = 116
const H = 84
const CYCLE = 2.6 // seconds per connect attempt
const IN = 0.42 // fraction of the cycle spent approaching
const HOLD = 0.14 // fraction spent seated while the spark flashes
const START = -0.85 // plug rest position
const END = 0.24 // plug seated position: prong tips reach the holes
const PLATE_X = 1.12

const cv = ref(null)
const ready = ref(false)

let cleanup = null

onMounted(async () => {
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

  let THREE, RoundedBoxGeometry
  try {
    THREE = await import('three')
    ;({ RoundedBoxGeometry } = await import('three/examples/jsm/geometries/RoundedBoxGeometry.js'))
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
  camera.position.set(0, 0, 6)
  camera.lookAt(0, 0, 0)

  scene.add(new THREE.HemisphereLight(0xdfe8ff, 0x241a4d, 1.15))
  const key = new THREE.DirectionalLight(0xffffff, 1.9)
  key.position.set(3, 4, 5)
  scene.add(key)

  const gradient = toonGradient(THREE)
  const toon = (color, extra = {}) =>
    new THREE.MeshToonMaterial({ color, gradientMap: gradient, ...extra })

  /* ------------------------------------------------------------ socket */
  const plate = new THREE.Mesh(
    new RoundedBoxGeometry(0.26, 1.0, 0.55, 3, 0.07),
    toon(0x3b2f8f, { emissive: 0x241d5e })
  )
  plate.position.set(PLATE_X, 0, 0)
  scene.add(plate)

  // the two holes, each dressed with a gold rim
  const holeGeo = new THREE.CylinderGeometry(0.075, 0.075, 0.1, 16)
  const rimGeo = new THREE.TorusGeometry(0.105, 0.014, 8, 32)
  for (const y of [-0.19, 0.19]) {
    const hole = new THREE.Mesh(holeGeo, new THREE.MeshBasicMaterial({ color: 0x101327 }))
    hole.rotation.z = -Math.PI / 2
    hole.position.set(0.99, y, 0)
    scene.add(hole)
    const rim = new THREE.Mesh(rimGeo, toon(0xf6b73c))
    rim.rotation.y = Math.PI / 2
    rim.position.set(1.0, y, 0)
    scene.add(rim)
  }

/* -------------------------------------------------------------- plug */
  const plug = new THREE.Group()
  const body = new THREE.Mesh(new THREE.CylinderGeometry(0.3, 0.3, 0.52, 24), toon(0x8b5cf6))
  body.rotation.z = -Math.PI / 2
  plug.add(body)
  const neck = new THREE.Mesh(new THREE.CylinderGeometry(0.16, 0.3, 0.16, 24), toon(0x7c4fe0))
  neck.rotation.z = -Math.PI / 2
  neck.position.x = 0.34
  plug.add(neck)
  const prongGeo = new THREE.CylinderGeometry(0.045, 0.045, 0.36, 12)
  for (const y of [-0.19, 0.19]) {
    const prong = new THREE.Mesh(prongGeo, toon(0xe4e8ff))
    prong.rotation.z = -Math.PI / 2
    prong.position.set(0.58, y, 0)
    plug.add(prong)
  }
  // cable trailing out through the bottom of the frame
  const cable = new THREE.Mesh(
    new THREE.TubeGeometry(
      new THREE.CatmullRomCurve3([
        new THREE.Vector3(-0.24, 0, 0),
        new THREE.Vector3(-0.72, -0.85, 0.08),
        new THREE.Vector3(-0.85, -2.6, 0.12),
      ]),
      20, 0.075, 10
    ),
    toon(0x241d5e)
  )
  plug.add(cable)
  plug.position.x = START
  scene.add(plug)

  /* ------------------------------------------------------------- spark */
  const sparkGroup = new THREE.Group()
  sparkGroup.position.set(0.9, 0, 0.12)
  const sparkMats = []
  const sparkGeo = new THREE.IcosahedronGeometry(0.06, 0)
  for (let i = 0; i < 5; i++) {
    const mat = new THREE.MeshBasicMaterial({ color: 0xffd27a, transparent: true, opacity: 0 })
    sparkMats.push(mat)
    const shard = new THREE.Mesh(sparkGeo, mat)
    const a = (i / 5) * Math.PI * 2
    shard.position.set(Math.cos(a) * 0.22, Math.sin(a) * 0.22, (i % 2) * 0.12 - 0.06)
    shard.rotation.set(a, a * 1.7, 0)
    sparkGroup.add(shard)
  }
  scene.add(sparkGroup)

  /* -------------------------------------------------------------- pose */
  const easeInOut = (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2)

  // the pose is a pure function of time, so the reduced-motion still frame
  // and the animated loop share one code path
  function applyPose(t) {
    const k = (t / CYCLE) % 1
    let x
    let spark = 0
    if (k < IN) {
      x = START + (END - START) * easeInOut(k / IN)
    } else if (k < IN + HOLD) {
      x = END
      spark = Math.sin(Math.PI * ((k - IN) / HOLD))
    } else {
      x = END + (START - END) * easeInOut((k - IN - HOLD) / (1 - IN - HOLD))
    }
    plug.position.x = x
    plug.position.y = Math.sin(t * 1.8) * 0.05
    plug.rotation.z = Math.sin(t * 1.4) * 0.05
    plate.position.x = PLATE_X + spark * 0.05 // tiny kickback on contact
    sparkGroup.scale.setScalar(0.25 + spark * 0.9)
    sparkGroup.rotation.z = t * 2
    for (const m of sparkMats) m.opacity = spark
  }

  let elapsed = CYCLE * (IN + HOLD * 0.5) // start mid-contact, spark lit
  let last = 0

  function frame(now) {
    const dt = last ? Math.min((now - last) / 1000, 1 / 15) : 1 / 60
    last = now
    if (document.hidden) return
    elapsed += dt
    applyPose(elapsed)
    renderer.render(scene, camera)
  }
  const onVisible = () => { last = 0 } // discard time spent in a hidden tab

  if (reduced) {
    applyPose(elapsed)
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
