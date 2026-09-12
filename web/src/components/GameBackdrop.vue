<script setup>
// Ambient 3D backdrop: cel-shaded playing cards and floating operator glyphs
// drifting through deep space behind the UI.
//
// The whole scene runs inside a Web Worker on an OffscreenCanvas (see
// backdropWorker.js), so three.js evaluation and per-frame rendering never
// touch the main thread — page load and input latency stay clean. Browsers
// without OffscreenCanvas keep the plain dark background; the boot is also
// deferred until the document has loaded and the browser is idle, because
// the backdrop is pure ambience and must never compete with first paint.
import { onMounted, onUnmounted } from 'vue'
import { clamp } from '../three/anim.js'

const props = defineProps({
  density: { type: Number, default: 1 }, // drifter count multiplier
})

let worker = null
let canvas = null
let disposed = false

// Resolves once the document has fully loaded and the browser has an idle
// slot, so the worker chunk and its three.js import start off the
// critical path.
function whenIdle() {
  return new Promise((resolve) => {
    const idle = () => {
      const ric = window.requestIdleCallback || ((cb) => setTimeout(cb, 300))
      ric(() => resolve(), { timeout: 2000 })
    }
    if (document.readyState === 'complete') idle()
    else addEventListener('load', idle, { once: true })
  })
}

function onMove(e) {
  worker?.postMessage({
    type: 'pointer',
    x: (e.clientX / innerWidth) * 2 - 1,
    y: (e.clientY / innerHeight) * 2 - 1,
  })
}
const onResize = () => worker?.postMessage({ type: 'resize', width: innerWidth, height: innerHeight })
const onVisible = () => worker?.postMessage({ type: 'visibility', hidden: document.hidden })

async function boot() {
  canvas = document.createElement('canvas')
  Object.assign(canvas.style, {
    position: 'fixed', inset: 0, width: '100vw', height: '100vh',
    pointerEvents: 'none', zIndex: -1, opacity: '0.62',
  })
  document.body.appendChild(canvas)

  if (typeof canvas.transferControlToOffscreen !== 'function') {
    canvas.remove()
    canvas = null
    return
  }
  try {
    worker = new Worker(new URL('../three/backdropWorker.js', import.meta.url), { type: 'module' })
  } catch {
    canvas.remove()
    canvas = null
    return
  }

  const offscreen = canvas.transferControlToOffscreen()
  worker.postMessage(
    {
      type: 'init',
      canvas: offscreen,
      width: innerWidth,
      height: innerHeight,
      dpr: devicePixelRatio || 1,
      density: props.density,
      reduced: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false,
    },
    [offscreen],
  )

  addEventListener('pointermove', onMove, { passive: true })
  addEventListener('resize', onResize)
  document.addEventListener('visibilitychange', onVisible)
}

onMounted(async () => {
  await whenIdle()
  if (!disposed) boot()
})

onUnmounted(() => {
  disposed = true
  worker?.postMessage({ type: 'dispose' })
  worker?.terminate()
  worker = null
  removeEventListener('pointermove', onMove)
  removeEventListener('resize', onResize)
  document.removeEventListener('visibilitychange', onVisible)
  canvas?.remove()
  canvas = null
})
</script>

<template>
  <!-- purely decorative: renders into a fixed canvas appended to <body> -->
</template>
