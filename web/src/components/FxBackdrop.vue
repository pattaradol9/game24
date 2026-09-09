<script setup>
// FxBackdrop — WebGL ambience behind a shop tab: ONE canvas for the whole
// pane (a context per card would blow the browser's context limit), built by
// a per-variant scene module on the shared skinScenes primitives:
//   • "vault"   — rising gold/amber motes + halo ring (boost items & inventory)
//   • "atelier" — falling ghost cards + glitter curtain (card skins)
//
// Same discipline as SkinFx/GameBackdrop: three.js is dynamically imported
// (code-split), motion is dt-driven, resolution drops before frames do
// (qualityGuard), the hidden tab and an off-screen pane (v-show tabs!) cost
// nothing, reduced motion gets a single composed still frame, and every GPU
// resource is disposed on unmount.
import { onMounted, onUnmounted, ref } from 'vue'
import { clamp, damp, disposeDeep, qualityGuard } from '../three/anim.js'
import { makeTextures } from '../three/skinScenes.js'
import { createItemScene } from '../three/itemFx.js'
import { createSkinShopScene } from '../three/skinShopFx.js'

const props = defineProps({
  variant: { type: String, default: 'vault' },
})

const BUILDERS = {
  vault: createItemScene,
  atelier: createSkinShopScene,
}

const host = ref(null)
let cleanup = null

onMounted(async () => {
  if (!host.value) return
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

  let THREE
  try {
    THREE = await import('three')
  } catch {
    return
  }

  const cv = document.createElement('canvas')
  cv.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;'
  host.value.appendChild(cv)

  let r
  try {
    r = new THREE.WebGLRenderer({ canvas: cv, alpha: true, antialias: true, powerPreference: 'high-performance' })
  } catch {
    cv.remove()
    return
  }
  r.outputColorSpace = THREE.SRGBColorSpace
  const q = qualityGuard(r, { max: 1.75, min: 0.7 })

  const FOV = 45
  const CAM_Z = 9
  const scene = new THREE.Scene()
  const cam = new THREE.PerspectiveCamera(FOV, 1, 0.1, 40)
  cam.position.set(0, 0, CAM_Z)

  // half-extents of the visible plane at the particle slab depth
  let aspect = 2
  function extent(z) {
    const h = Math.tan((FOV * Math.PI) / 360) * (CAM_Z - z) + 0.5
    return { x: h * aspect + 0.5, y: h }
  }
  const rand = (a, b) => a + Math.random() * (b - a)

  const owned = []
  const tex = makeTextures(THREE)
  owned.push(...Object.values(tex))
  const scaleRegs = []
  const pxScale = () => cv.height / (2 * Math.tan((FOV * Math.PI) / 360))
  const api = {
    THREE, back: scene, owned, rand, DEPTH: 4, tex,
    w0: { x: 0, y: 0 },
    pxScale,
    onScale: (fn) => scaleRegs.push(fn),
  }

  // damped pointer parallax, like every other scene in the app
  const pointer = { x: 0, y: 0 }
  const par = { x: 0, y: 0 }
  function onMove(e) {
    pointer.x = clamp((e.clientX / innerWidth) * 2 - 1, -1, 1)
    pointer.y = clamp((e.clientY / innerHeight) * 2 - 1, -1, 1)
  }
  if (!reduced) addEventListener('pointermove', onMove, { passive: true })

  function resize() {
    const rect = host.value.getBoundingClientRect()
    if (!rect.width || !rect.height) return
    aspect = rect.width / rect.height
    r.setSize(rect.width, rect.height, false)
    cam.aspect = aspect
    cam.updateProjectionMatrix()
    const s = pxScale()
    for (const fn of scaleRegs) fn(s)
  }
  const ro = new ResizeObserver(resize)
  ro.observe(host.value)
  resize()
  // the builder scatters particles with the first known frustum
  api.w0 = extent(-2)
  const fx = BUILDERS[props.variant]?.(api)
  if (!fx) {
    cleanup = () => {
      ro.disconnect()
      disposeDeep(scene)
      for (const o of owned) o.dispose?.()
      r.dispose()
      cv.remove()
    }
    return
  }

  // v-show tabs keep the canvas mounted but off-screen — skip those frames
  let onScreen = true
  const io = new IntersectionObserver(([e]) => {
    onScreen = e.isIntersecting
    last = 0
  })
  io.observe(host.value)

  let elapsed = 0
  let last = 0
  function frame(now) {
    const dt = last ? Math.min((now - last) / 1000, 1 / 15) : 1 / 60
    last = now
    if (document.hidden || !onScreen) return
    elapsed += dt
    const w = extent(-2)
    fx.update(elapsed, dt, w)
    par.x = damp(par.x, pointer.x * 0.32, 2.2, dt)
    par.y = damp(par.y, -pointer.y * 0.2, 2.2, dt)
    cam.position.x = par.x + Math.sin(elapsed * 0.3) * 0.05
    cam.position.y = par.y + Math.cos(elapsed * 0.23) * 0.04
    cam.lookAt(0, 0, -2)
    r.render(scene, cam)
    q.sample(dt * 1000)
  }

  if (reduced) {
    // one composed still frame; nothing loops
    fx.update(1.5, 0, extent(-2))
    r.render(scene, cam)
  } else {
    r.setAnimationLoop(frame)
  }
  const onVisible = () => { last = 0 }
  document.addEventListener('visibilitychange', onVisible)

  cleanup = () => {
    r.setAnimationLoop(null)
    io.disconnect()
    if (!reduced) removeEventListener('pointermove', onMove)
    document.removeEventListener('visibilitychange', onVisible)
    ro.disconnect()
    disposeDeep(scene)
    for (const o of owned) o.dispose?.()
    r.dispose()
    cv.remove()
  }
})

onUnmounted(() => cleanup?.())
</script>

<template>
  <!-- decorative WebGL layer behind the cards; pointer-events pass through -->
  <div ref="host" class="fx-backdrop" aria-hidden="true" />
</template>

<style scoped>
.fx-backdrop {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}
</style>
