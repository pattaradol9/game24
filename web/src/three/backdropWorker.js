// Ambient backdrop scene, rendered inside a Web Worker on an OffscreenCanvas
// so three.js evaluation and per-frame rendering never touch the main thread
// (page load and input latency stay clean).
//
// Protocol (postMessage from GameBackdrop.vue):
//   {type:'init', canvas, width, height, dpr, density, reduced} — canvas is
//     a transferred OffscreenCanvas; builds the scene and starts (or, under
//     reduced motion, renders exactly one still frame)
//   {type:'pointer', x, y} — pointer position normalized to [-1, 1]
//   {type:'resize', width, height}
//   {type:'visibility', hidden} — the loop parks while the tab is hidden
//   {type:'dispose'} — release every GPU resource, then close
import { RoundedBoxGeometry } from 'three/examples/jsm/geometries/RoundedBoxGeometry.js'
import * as THREE from 'three'
import { clamp, damp, disposeDeep, qualityGuard, toonGradient } from './anim.js'

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
let hidden = false
let pointer = { x: 0, y: 0 }
let cam = { x: 0, y: 0 }
let drifters = []
let owned = [] // textures/materials to dispose by hand
let scene, camera, renderer, quality, aspect, reduced
let timer = 0
let last = 0
let elapsed = 0

// The scene textures paint card ranks with Kanit; a worker has no document
// stylesheet, so the needed faces are fetched and registered by hand. Best
// effort: any failure leaves the canvas fallback font (as before).
async function loadFonts() {
  const specs = [
    { weight: '400', url: '/fonts/kanit-400-latin.woff2' },
    { weight: '400', url: '/fonts/kanit-400-thai.woff2' },
    { weight: '700', url: '/fonts/kanit-700-latin.woff2' },
    { weight: '700', url: '/fonts/kanit-700-thai.woff2' },
  ]
  await Promise.all(
    specs.map(async ({ weight, url }) => {
      const buf = await fetch(url).then((r) => r.arrayBuffer())
      const face = new FontFace('Kanit', buf, { weight })
      self.fonts.add(face)
      await face.load()
    }),
  )
}

// half-height / half-width of the frustum at a drifter's depth, plus margin
function extent(z) {
  const h = Math.tan((FOV * Math.PI) / 360) * (CAM_Z - z)
  return { y: h + 1.6, x: h * aspect + 1.6 }
}

function place(d, at) {
  const e = extent(d.mesh.position.z)
  d.mesh.position.y = -e.y + at * e.y * 2
  d.mesh.position.x = (Math.random() - 0.5) * e.x * 1.9
}

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

function texture(w, h, paint) {
  const cv = new OffscreenCanvas(w, h)
  paint(cv.getContext('2d'))
  const tex = new THREE.CanvasTexture(cv)
  tex.colorSpace = THREE.SRGBColorSpace
  tex.anisotropy = 4
  return tex
}

function frame() {
  const now = performance.now()
  const dt = last ? Math.min((now - last) / 1000, 1 / 15) : 1 / 60
  last = now
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

// Workers have no requestAnimationFrame; a tight setTimeout chain paces the
// ambient loop instead. Parked while the tab is hidden via 'visibility'.
function schedule() {
  timer = setTimeout(() => {
    if (hidden) {
      timer = 0
      return
    }
    frame()
    schedule()
  }, 16)
}

function start() {
  if (reduced || timer) return
  last = 0 // discard time parked while hidden
  schedule()
}

function stop() {
  if (timer) {
    clearTimeout(timer)
    timer = 0
  }
}

async function build({ canvas, width, height, dpr, density, reduced: still }) {
  reduced = still
  aspect = width / height

  renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true, powerPreference: 'high-performance' })
  renderer.outputColorSpace = THREE.SRGBColorSpace
  quality = qualityGuard(renderer, { max: 1.5, min: 0.7, dpr })

  scene = new THREE.Scene()
  scene.fog = new THREE.Fog(0x0c0f16, 10, 25)
  camera = new THREE.PerspectiveCamera(FOV, aspect, 0.1, 40)
  camera.position.set(0, 0, CAM_Z)

  scene.add(new THREE.HemisphereLight(0xdfe8ff, 0x241a4d, 1.1))
  const key = new THREE.DirectionalLight(0xffffff, 1.9)
  key.position.set(3, 5, 6)
  scene.add(key)
  const rim = new THREE.DirectionalLight(0x8b5cf6, 1.3)
  rim.position.set(-5, 2, -3)
  scene.add(rim)

  const gradient = toonGradient(THREE)

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
  owned.push(gradient, backTex)

  const cardCount = Math.round(9 * density)
  for (let i = 0; i < cardCount; i++) {
    // BoxGeometry group order: +X −X +Y −Y +Z(front) −Z(back)
    const mats = [edgeMat, edgeMat, edgeMat, edgeMat, faceMats[i % faceMats.length], backMat]
    spawn(new THREE.Mesh(cardGeo, mats), { depth: -3.5 - Math.random() * 8.5 })
  }

  const glyphCount = Math.round(6 * density)
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

  renderer.setSize(width, height, false)

  cleanup = () => {
    stop()
    disposeDeep(scene)
    cardGeo.dispose()
    for (const o of owned) o.dispose?.()
    owned = []
    drifters = []
    renderer.dispose()
  }

  if (reduced) {
    renderer.render(scene, camera) // one still frame, no motion
  } else {
    start()
  }
}

self.onmessage = async (e) => {
  const msg = e.data
  switch (msg.type) {
    case 'init':
      try {
        await Promise.race([loadFonts(), new Promise((r) => setTimeout(r, 1500))])
      } catch { /* textures fall back to the canvas default font */ }
      try {
        await build(msg)
      } catch (err) {
        // no WebGL (or a failed context) in this worker — the page just
        // keeps its plain dark background
        console.warn('backdrop: scene unavailable', err)
        self.close()
      }
      break
    case 'pointer':
      pointer.x = clamp(msg.x, -1, 1)
      pointer.y = clamp(msg.y, -1, 1)
      break
    case 'resize':
      aspect = msg.width / msg.height
      renderer?.setSize(msg.width, msg.height, false)
      if (camera) {
        camera.aspect = aspect
        camera.updateProjectionMatrix()
      }
      break
    case 'visibility':
      hidden = msg.hidden
      if (!hidden) start()
      else stop()
      break
    case 'dispose':
      cleanup?.()
      self.close()
      break
  }
}
