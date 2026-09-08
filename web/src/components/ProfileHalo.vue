<script setup>
// ProfileHalo — the golden particle ring + rising spark dust that plays
// behind the avatar in the mobile profile sheet. Same discipline as
// GameBackdrop/SkinFx: three.js is dynamically imported (code-split), motion
// is dt-driven, resolution drops before frames do (qualityGuard), a hidden
// tab costs nothing, reduced motion gets a single composed still frame, and
// every GPU resource is disposed on unmount.
import { onMounted, onUnmounted, ref } from 'vue'
import { disposeDeep, qualityGuard } from '../three/anim.js'
import {
  HALO_DEFAULTS,
  advanceSparks,
  ringPositions,
  softDotTexture,
  sparkField,
} from '../three/profileHalo.js'

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

  let renderer
  try {
    renderer = new THREE.WebGLRenderer({ canvas: cv, alpha: true, antialias: false, powerPreference: 'low-power' })
  } catch {
    cv.remove()
    return
  }
  const q = qualityGuard(renderer, { max: 1.5, min: 0.6 })

  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 50)
  camera.position.set(0, 0.6, 9)

  const GOLD = 0xf6b73c
  const dotTex = softDotTexture(THREE)
  const owned = [dotTex]

  // orbiting ring: a flat XZ circle of gold dust, tilted toward the camera
  const ring = new THREE.Points(
    new THREE.BufferGeometry().setAttribute('position', new THREE.BufferAttribute(ringPositions(HALO_DEFAULTS.ringCount, HALO_DEFAULTS.ringRadius), 3)),
    new THREE.PointsMaterial({
      color: GOLD, size: 0.085, map: dotTex, transparent: true, opacity: 0.9,
      blending: THREE.AdditiveBlending, depthWrite: false, sizeAttenuation: true,
    }),
  )
  ring.rotation.x = 1.2
  ring.position.y = -0.4
  scene.add(ring)

  // rising spark dust drifting up through the frame
  const field = sparkField(HALO_DEFAULTS.sparkCount, HALO_DEFAULTS.width, HALO_DEFAULTS.height, HALO_DEFAULTS.depth)
  const sparkGeo = new THREE.BufferGeometry()
  const posAttr = new THREE.BufferAttribute(field.positions, 3)
  sparkGeo.setAttribute('position', posAttr)
  const sparks = new THREE.Points(
    sparkGeo,
    new THREE.PointsMaterial({
      color: GOLD, size: 0.05, map: dotTex, transparent: true, opacity: 0.55,
      blending: THREE.AdditiveBlending, depthWrite: false, sizeAttenuation: true,
    }),
  )
  sparks.position.y = -HALO_DEFAULTS.height / 2 + 0.6
  scene.add(sparks)

  /* -------------------------------------------------- sizing + loop */
  const resize = () => {
    const w = host.value?.clientWidth || 1
    const h = host.value?.clientHeight || 1
    renderer.setSize(w, h, false)
    camera.aspect = w / h
    camera.updateProjectionMatrix()
  }
  const ro = new ResizeObserver(resize)
  ro.observe(host.value)
  resize()

  let raf = 0
  let last = performance.now()
  let elapsed = 0
  let lastFrame = last

  const frame = (now) => {
    raf = requestAnimationFrame(frame)
    if (document.hidden) {
      last = now
      return
    }
    const dt = Math.min(0.1, (now - last) / 1000)
    last = now
    elapsed += dt
    q.sample(now - lastFrame)
    lastFrame = now

    ring.rotation.y += dt * 0.35
    advanceSparks(field, dt, elapsed)
    posAttr.needsUpdate = true
    renderer.render(scene, camera)
  }

  if (reduced) {
    // one composed still frame, no loop
    renderer.render(scene, camera)
  } else {
    raf = requestAnimationFrame(frame)
  }

  cleanup = () => {
    cancelAnimationFrame(raf)
    raf = 0
    ro.disconnect()
    disposeDeep(scene)
    for (const o of owned) o.dispose?.()
    sparkGeo.dispose()
    renderer.dispose()
    cv.remove()
  }
})

onUnmounted(() => cleanup?.())
</script>

<template>
  <div ref="host" class="halo-host" aria-hidden="true" />
</template>

<style scoped>
.halo-host {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}
</style>
