<script setup>
// SkinFx — WebGL life for premium card skins, in two layers:
//   • an ambience scene rendered on a canvas BEHIND the cards (per-skin
//     particle weather built by three/skinScenes.js), and
//   • a celebration scene rendered on a canvas ABOVE the cards — the ring /
//     flash / spark shower fired by burst() when a hand resolves.
// classic/mono stay quiet by design — the stock table has no fog.
//
// Built on the same discipline as GameBackdrop: three.js is dynamically
// imported (code-split), motion is dt-driven, resolution drops before frames
// do (qualityGuard), the tab being hidden costs nothing, reduced motion gets
// a single composed still frame (and never a burst), and every GPU resource
// is disposed on unmount.
import { onMounted, onUnmounted, ref } from 'vue'
import { clamp, damp, disposeDeep, qualityGuard } from '../three/anim.js'
import { buildSkinScene, makeTextures } from '../three/skinScenes.js'

const props = defineProps({
  skin: { type: String, default: '' },
})

// two stacked hosts so the celebration canvas can outrank the cards
// (.cards sits at z-index 1 inside .board); a component's root would be one
// stacking context and could never straddle them
const backHost = ref(null)
const frontHost = ref(null)
let cleanup = null
let fireBurst = null

// GameBoard calls this at the merge point (client coords, strength ≈ 0..1)
function burst(x, y, strength = 1) {
  fireBurst?.(x, y, strength)
}
defineExpose({ burst })

// skins without a 3D layer
const QUIET = new Set(['', 'classic', 'mono'])

onMounted(async () => {
  if (QUIET.has(props.skin) || !backHost.value || !frontHost.value) return
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

  let THREE
  try {
    THREE = await import('three')
  } catch {
    return
  }

  /* --------------------------------------------------------- canvases */
  const mk = (z) => {
    const el = document.createElement('canvas')
    el.style.cssText = `position:absolute;inset:0;width:100%;height:100%;display:block;z-index:${z};`
    return el
  }
  const backCv = mk(0)
  const frontCv = mk(2)
  backHost.value.appendChild(backCv)
  frontHost.value.appendChild(frontCv)

  let backR, frontR
  try {
    backR = new THREE.WebGLRenderer({ canvas: backCv, alpha: true, antialias: true, powerPreference: 'high-performance' })
    frontR = new THREE.WebGLRenderer({ canvas: frontCv, alpha: true, antialias: false, powerPreference: 'high-performance' })
  } catch {
    backCv.remove()
    frontCv.remove()
    return
  }
  backR.outputColorSpace = THREE.SRGBColorSpace
  frontR.outputColorSpace = THREE.SRGBColorSpace
  const qBack = qualityGuard(backR, { max: 1.75, min: 0.7 })
  const qFront = qualityGuard(frontR, { max: 1.5, min: 0.7 })

  /* ----------------------------------------------------------- stages */
  const FOV = 45
  const CAM_Z = 9
  const DEPTH = 5.5 // particle slab: z ∈ [-DEPTH, -0.4]
  const back = new THREE.Scene()
  const front = new THREE.Scene()
  const camBack = new THREE.PerspectiveCamera(FOV, 1, 0.1, 40)
  const camFront = new THREE.PerspectiveCamera(FOV, 1, 0.1, 40)
  camBack.position.set(0, 0, CAM_Z)
  camFront.position.set(0, 0, CAM_Z)

  // half-extents of the visible plane at depth z (with a small margin):
  // half-height is tan(fov/2) × distance from the camera
  let aspect = 2
  function extent(z) {
    const h = Math.tan((FOV * Math.PI) / 360) * (CAM_Z - z) + 0.5
    return { x: h * aspect + 0.5, y: h }
  }
  const rand = (a, b) => a + Math.random() * (b - a)

  const owned = []
  const tex = makeTextures(THREE)
  owned.push(...Object.values(tex))

  // materials register here so resize can reproject point sizes
  const scaleRegs = []
  const pxScale = () => frontCv.height / (2 * Math.tan((FOV * Math.PI) / 360))

  const api = {
    skin: props.skin,
    THREE, back, front, owned, rand, DEPTH, tex,
    w0: { x: 0, y: 0 },
    pxScale,
    onScale: (fn) => scaleRegs.push(fn),
  }

  /* ------------------------------------------------------ input + loop */
  const pointer = { x: 0, y: 0 }
  const cam = { x: 0, y: 0 }
  function onMove(e) {
    pointer.x = clamp((e.clientX / innerWidth) * 2 - 1, -1, 1)
    pointer.y = clamp((e.clientY / innerHeight) * 2 - 1, -1, 1)
  }
  if (!reduced) addEventListener('pointermove', onMove, { passive: true })

  function resize() {
    const r = backHost.value.getBoundingClientRect()
    if (!r.width || !r.height) return
    aspect = r.width / r.height
    backR.setSize(r.width, r.height, false)
    frontR.setSize(r.width, r.height, false)
    camBack.aspect = aspect
    camFront.aspect = aspect
    camBack.updateProjectionMatrix()
    camFront.updateProjectionMatrix()
    const s = pxScale()
    for (const fn of scaleRegs) fn(s)
  }
  const ro = new ResizeObserver(resize)
  ro.observe(backHost.value)
  resize()
  // the builders scatter particles with the first known frustum
  api.w0 = extent(-2)
  const sceneApi = buildSkinScene(api)

  // client coords → world point on the celebration slab (z = -1.2)
  const v3 = new THREE.Vector3()
  function toWorld(cx, cy) {
    const r = frontHost.value.getBoundingClientRect()
    const nx = ((cx - r.left) / r.width) * 2 - 1
    const ny = -(((cy - r.top) / r.height) * 2 - 1)
    v3.set(nx, ny, 0.5).unproject(camFront).sub(camFront.position).normalize()
    const t = (-1.2 - camFront.position.z) / v3.z
    return { x: camFront.position.x + v3.x * t, y: camFront.position.y + v3.y * t }
  }
  fireBurst = (cx, cy, s) => {
    if (reduced || !sceneApi.burst) return
    const p = toWorld(cx, cy)
    sceneApi.burst(clamp(p.x, -14, 14), clamp(p.y, -9, 9), clamp(s ?? 1, 0.25, 1.5))
  }

  let elapsed = 0
  let last = 0
  function frame(now) {
    const dt = last ? Math.min((now - last) / 1000, 1 / 15) : 1 / 60
    last = now
    if (document.hidden) return
    elapsed += dt
    const w = extent(-2)
    sceneApi.update?.(elapsed, dt, w)
    // damped pointer parallax + a faint autonomous sway so it never freezes
    cam.x = damp(cam.x, pointer.x * 0.35, 2.2, dt)
    cam.y = damp(cam.y, -pointer.y * 0.22, 2.2, dt)
    camBack.position.x = cam.x + Math.sin(elapsed * 0.3) * 0.06
    camBack.position.y = cam.y + Math.cos(elapsed * 0.23) * 0.05
    camBack.lookAt(0, 0, -2)
    backR.render(back, camBack)
    camFront.lookAt(0, 0, -2) // celebration layer stays fixed: coords must match the pointer
    frontR.render(front, camFront)
    qBack.sample(dt * 1000)
    qFront.sample(dt * 1000)
  }

  if (reduced) {
    // one composed still frame; bursts stay off
    sceneApi.update?.(1.5, 0, extent(-2))
    backR.render(back, camBack)
    frontR.render(front, camFront)
  } else {
    backR.setAnimationLoop(frame)
  }
  const onVisible = () => { last = 0 }
  document.addEventListener('visibilitychange', onVisible)

  cleanup = () => {
    backR.setAnimationLoop(null)
    if (!reduced) removeEventListener('pointermove', onMove)
    document.removeEventListener('visibilitychange', onVisible)
    ro.disconnect()
    disposeDeep(back)
    disposeDeep(front)
    for (const o of owned) o.dispose?.()
    backR.dispose()
    frontR.dispose()
    backCv.remove()
    frontCv.remove()
  }
})

onUnmounted(() => cleanup?.())
</script>

<template>
  <!-- decorative WebGL layers; pointer-events pass straight through -->
  <div ref="backHost" class="skin-fx back" aria-hidden="true" />
  <div ref="frontHost" class="skin-fx front" aria-hidden="true" />
</template>

<style scoped>
.skin-fx {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  overflow: hidden;
  pointer-events: none;
}
.skin-fx.back { z-index: 0; }
.skin-fx.front { z-index: 2; /* above the .cards row (z-index 1) */ }
</style>
