<script setup>
// Ambient 3D backdrop: cel-shaded playing cards and floating operator glyphs
// drifting through deep space behind the UI.
//
// Every drifter moves on dt, and wraps only once it is genuinely outside the
// camera frustum for its own depth — so nothing ever pops in or out mid-screen.
// Renders into a fixed canvas appended to <body>, drops resolution instead of
// frames when a device struggles, and stays a single still image under
// prefers-reduced-motion.
import { onMounted, onUnmounted } from 'vue'
import { clamp, damp, disposeDeep, qualityGuard, toonGradient } from '../three/anim.js'

const props = defineProps({
  density: { type: Number, default: 1 }, // drifter count multiplier
})

const CARDS = [
  { n: 'A', s: '♠', red: false },
  { n: '7', s: '♥', red: true },
  { n: 'K', s: '♦', red: true },
  { n: '3', s: '♣', red: false },
  { n: '9', s: '♠', red: false },
  { n: 'Q', s: '♥', red: true },
]
const GLYPHS = [
  { text: '+', color: '#22d3ee' },
  { text: '×', color: '#ff7ea8' },
  { text: '−', color: '#a78bfa' },
  { text: '÷', color: '#ffd166' },
  { text: '=', color: '#38bdf8' },
]

const FOV = 50
const CAM_Z = 9

let cleanup = null

onMounted(async () => {
  const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

  let THREE, RoundedBoxGeometry
  try {
    THREE = await import('three')
    ;({ RoundedBoxGeometry } = await import('three/examples/jsm/geometries/RoundedBoxGeometry.js'))
  } catch { return }

  const el = document.createElement('canvas')
  Object.assign(el.style, {
    position: 'fixed', inset: 0, width: '100vw', height: '100vh',
    pointerEvents: 'none', zIndex: -1, opacity: '0.62',
  })
  document.body.appendChild(el)

  let renderer
  try {
    renderer = new THREE.WebGLRenderer({ canvas: el, alpha: true, antialias: true, powerPreference: 'high-performance' })
  } catch { el.remove(); return }
  renderer.outputColorSpace = THREE.SRGBColorSpace
  const quality = qualityGuard(renderer, { max: 1.5, min: 0.7 })

  const scene = new THREE.Scene()
  scene.fog = new THREE.Fog(0x0c0f16, 10, 25)
  const camera = new THREE.PerspectiveCamera(FOV, 1, 0.1, 40)
  camera.position.set(0, 0, CAM_Z)

  scene.add(new THREE.HemisphereLight(0xdfe8ff, 0x241a4d, 1.1))
  const key = new THREE.DirectionalLight(0xffffff, 1.9)
  key.position.set(3, 5, 6)
  scene.add(key)
  const rim = new THREE.DirectionalLight(0x8b5cf6, 1.3)
  rim.position.set(-5, 2, -3)
  scene.add(rim)

  const gradient = toonGradient(THREE)

  /* ---------------------------------------------------------- textures */
  function texture(w, h, paint) {
    const cv = document.createElement('canvas')
    cv.width = w
    cv.height = h
    paint(cv.getContext('2d'))
    const tex = new THREE.CanvasTexture(cv)
    tex.colorSpace = THREE.SRGBColorSpace
    tex.anisotropy = 4
    return tex
  }

  const cardTex = (card) => texture(220, 308, (c) => {
    c.fillStyle = '#f7f8ff'
    c.fillRect(0, 0, 220, 308)
    const ink = card.red ? '#e11d48' : '#312e81'
    c.fillStyle = ink
    c.textAlign = 'center'
    c.textBaseline = 'middle'
    c.font = '700 96px Kanit, "Trebuchet MS", sans-serif'
    c.fillText(card.n, 110, 140)
    c.font = '700 56px Kanit, "Trebuchet MS", sans-serif'
    c.fillText(card.s, 110, 224)
    c.textAlign = 'left'
    c.font = '700 32px Kanit, "Trebuchet MS", sans-serif'
    c.fillText(card.n, 16, 34)
    c.font = '28px Kanit, "Trebuchet MS", sans-serif'
    c.fillText(card.s, 18, 70)
  })

  const glyphTex = (g) => texture(128, 128, (c) => {
    c.textAlign = 'center'
    c.textBaseline = 'middle'
    c.font = '700 88px Kanit, "Trebuchet MS", sans-serif'
    c.shadowColor = g.color
    c.shadowBlur = 22
    c.fillStyle = g.color
    c.fillText(g.text, 64, 70)
    c.fillText(g.text, 64, 70)
  })

  const backTex = texture(220, 308, (c) => {
    c.fillStyle = '#3b2f8f'
    c.fillRect(0, 0, 220, 308)
    c.strokeStyle = 'rgba(255,255,255,0.16)'
    c.lineWidth = 4
    for (let i = -308; i < 220; i += 26) {
      c.beginPath(); c.moveTo(i, 0); c.lineTo(i + 308, 308); c.stroke()
      c.beginPath(); c.moveTo(i + 308, 0); c.lineTo(i, 308); c.stroke()
    }
    c.strokeStyle = '#d9d4ff'
    c.lineWidth = 10
    c.strokeRect(12, 12, 196, 284)
  })

  /* ---------------------------------------------------------- drifters */
  let aspect = innerWidth / innerHeight
  const drifters = []
  const owned = [gradient, backTex] // textures/materials to dispose by hand

  function spawn(mesh, { depth, speed = 1 }) {
    mesh.position.set(0, 0, depth)
    mesh.rotation.set(Math.random() * 0.6 - 0.3, (Math.random() - 0.5) * 1.7, Math.random() * 0.5 - 0.25)
    scene.add(mesh)
    const d = {
      mesh,
      vy: (0.1 + Math.random() * 0.16) * speed,
      rx: (Math.random() - 0.5) * 0.12,
      ry: (Math.random() - 0.5) * 0.14,
      rz: (Math.random() - 0.5) * 0.16,
      sway: 0.1 + Math.random() * 0.25,
      swayF: 0.2 + Math.random() * 0.35,
      phase: Math.random() * Math.PI * 2,
    }
    drifters.push(d)
    place(d, Math.random()) // scatter along the loop so nothing starts in a row
    return d
  }

  // half-height / half-width of the frustum at a drifter's depth, plus margin
  const extent = (z) => {
    const h = Math.tan((FOV * Math.PI) / 360) * (CAM_Z - z)
    return { y: h + 1.6, x: h * aspect + 1.6 }
  }
  function place(d, at) {
    const e = extent(d.mesh.position.z)
    d.mesh.position.y = -e.y + at * e.y * 2
    d.mesh.position.x = (Math.random() - 0.5) * e.x * 1.9
  }

  const cardGeo = new RoundedBoxGeometry(1.05, 1.47, 0.07, 3, 0.08)
  const edgeMat = new THREE.MeshToonMaterial({ color: 0xe4e8ff, gradientMap: gradient })
  const backMat = new THREE.MeshToonMaterial({ map: backTex, gradientMap: gradient, emissive: 0x241d5e })
  const faceMats = CARDS.map((card) => {
    const tex = cardTex(card)
    owned.push(tex)
    // emissive keeps a card readable even when it drifts edge-on to the key light
    return new THREE.MeshToonMaterial({
      map: tex, gradientMap: gradient, emissive: 0xffffff, emissiveMap: tex, emissiveIntensity: 0.42,
    })
  })

  const cardCount = Math.round(9 * props.density)
  for (let i = 0; i < cardCount; i++) {
    // BoxGeometry group order: +X −X +Y −Y +Z(front) −Z(back)
    const mats = [edgeMat, edgeMat, edgeMat, edgeMat, faceMats[i % faceMats.length], backMat]
    spawn(new THREE.Mesh(cardGeo, mats), { depth: -3.5 - Math.random() * 8.5 })
  }

  const glyphCount = Math.round(6 * props.density)
  for (let i = 0; i < glyphCount; i++) {
    const g = GLYPHS[i % GLYPHS.length]
    const tex = glyphTex(g)
    owned.push(tex)
    const mat = new THREE.SpriteMaterial({ map: tex, transparent: true, opacity: 0.55, depthWrite: false, fog: true })
    const sprite = new THREE.Sprite(mat)
    sprite.scale.setScalar(0.7 + Math.random() * 0.5)
    const d = spawn(sprite, { depth: -1.5 - Math.random() * 7, speed: 0.8 })
    d.rx = d.ry = 0 // sprites always face the camera; spinning them does nothing
    d.rz = 0
  }

  /* ------------------------------------------------------------ input */
  const pointer = { x: 0, y: 0 }
  const cam = { x: 0, y: 0 }
  function onMove(e) {
    pointer.x = clamp((e.clientX / innerWidth) * 2 - 1, -1, 1)
    pointer.y = clamp((e.clientY / innerHeight) * 2 - 1, -1, 1)
  }
  addEventListener('pointermove', onMove, { passive: true })

  function resize() {
    aspect = innerWidth / innerHeight
    renderer.setSize(innerWidth, innerHeight, false)
    camera.aspect = aspect
    camera.updateProjectionMatrix()
  }
  addEventListener('resize', resize)
  resize()

  /* ------------------------------------------------------------- loop */
  let elapsed = 0
  let last = 0

  function frame(now) {
    const dt = last ? Math.min((now - last) / 1000, 1 / 15) : 1 / 60
    last = now
    if (document.hidden) return
    elapsed += dt

    for (const d of drifters) {
      const m = d.mesh
      m.position.y += d.vy * dt
      m.position.x += Math.sin(elapsed * d.swayF + d.phase) * d.sway * dt
      m.rotation.x += d.rx * dt
      m.rotation.y += d.ry * dt
      m.rotation.z += d.rz * dt
      // wrap only once fully outside this drifter's own frustum slice
      if (m.position.y > extent(m.position.z).y) place(d, 0)
    }

    cam.x = damp(cam.x, pointer.x * 0.6, 2.4, dt)
    cam.y = damp(cam.y, -pointer.y * 0.4, 2.4, dt)
    camera.position.x = cam.x
    camera.position.y = cam.y + Math.sin(elapsed * 0.24) * 0.22
    camera.lookAt(0, 0, -4)

    renderer.render(scene, camera)
    quality.sample(dt * 1000)
  }

  if (reduced) {
    renderer.render(scene, camera) // one still frame, no motion
  } else {
    renderer.setAnimationLoop(frame)
  }
  const onVisible = () => { last = 0 } // discard time spent in a hidden tab
  document.addEventListener('visibilitychange', onVisible)

  cleanup = () => {
    renderer.setAnimationLoop(null)
    removeEventListener('pointermove', onMove)
    removeEventListener('resize', resize)
    document.removeEventListener('visibilitychange', onVisible)
    disposeDeep(scene)
    cardGeo.dispose()
    for (const o of owned) o.dispose?.()
    renderer.dispose()
    el.remove()
  }
})

onUnmounted(() => cleanup?.())
</script>

<template>
  <!-- purely decorative: renders into a fixed canvas appended to <body> -->
</template>
