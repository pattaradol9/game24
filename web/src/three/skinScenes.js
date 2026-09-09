// skinScenes — per-skin Three.js scene builders for SkinFx.
//
// Every premium skin gets an ambience scene (rendered behind the cards) and a
// celebration kit (shockwave ring + flash + spark shower, rendered above the
// cards when SkinFx.burst() fires at the merge point). Builders are plain
// functions over a shared `api`:
//
//   { skin, THREE, back, front, owned, rand, DEPTH, w0, tex, pxScale, onScale }
//
// `back`/`front` are the two scenes, `w0` the visible half-extents at build
// time, `tex` the shared canvas textures. Builders return
// `{ update(t, dt, w), burst(x, y, strength) }`.
//
// The pure layout/curve helpers at the top are exported for node tests, so
// this module must stay free of top-level three imports.

import { clamp } from './anim.js'

/* ============================================================ pure helpers */

/**
 * Point layout for a 2-arm spiral galaxy: radius grows with the square root of
 * the index (uniform disc density), each arm winds around the centre, and the
 * `rng`-driven jitter blurs the arms as the radius shrinks them. Returns
 * Float32Array of [x, y] pairs.
 */
export function spiralLayout(count, arms, rMax, rng = Math.random) {
  const out = new Float32Array(count * 2)
  for (let i = 0; i < count; i++) {
    const r = Math.sqrt((i + 0.5) / count) * rMax
    const arm = (i % arms) * ((Math.PI * 2) / arms)
    const wind = r * 1.85
    const jitter = (rng() * 2 - 1) * (0.42 / (0.35 + r / rMax))
    const a = arm + wind + jitter
    out[i * 2] = Math.cos(a) * r
    out[i * 2 + 1] = Math.sin(a) * r
  }
  return out
}

/** Evenly spaced points on a circle of radius `r` — [x, y] pairs. */
export function circleLayout(count, r) {
  const out = new Float32Array(count * 2)
  for (let i = 0; i < count; i++) {
    const a = (i / count) * Math.PI * 2
    out[i * 2] = Math.cos(a) * r
    out[i * 2 + 1] = Math.sin(a) * r
  }
  return out
}

/** Glint envelope over a normalised cycle position: 0 → 0, 0.5 → 1, 1 → 0. */
export function glintCurve(p) {
  if (p <= 0 || p >= 1) return 0
  return Math.pow(Math.sin(p * Math.PI), 1.5)
}

/** Burst particle alpha from remaining life: quick fade-in, smooth fade-out. */
export function burstFade(life, ttl) {
  const age = ttl - life
  return clamp(Math.min(age * 7, life / (ttl * 0.7)), 0, 1)
}

/* ================================================================ GLSL */

const CLOUD_VERT = /* glsl */ `
  attribute float aPhase;
  attribute float aSize;
  attribute float aSpeed;
  attribute float aSway;
  attribute float aSwayF;
  attribute float aTwF;
  attribute vec3 aColor;
  uniform float uTime;
  uniform float uBound;
  uniform float uScale;
  varying vec3 vColor;
  varying float vTw;
  void main() {
    vec3 p = position;
#ifdef GALAXY_SPIN
    // differential rotation: inner points orbit faster, like a real disc
    float rC = length(p.xy);
    float ang = uTime * SPIN_SPEED * (1.7 / (0.32 + rC));
    float cS = cos(ang);
    float sS = sin(ang);
    p.xy = mat2(cS, -sS, sS, cS) * p.xy;
#endif
    // vertical drift with wrap-around, so embers/bubbles/dust recycle on GPU
    p.y = mod(p.y - uTime * aSpeed + uBound, 2.0 * uBound) - uBound;
    p.x += sin(uTime * aSwayF + aPhase) * aSway;
    vTw = 0.6 + 0.4 * sin(uTime * aTwF + aPhase * 3.7);
    vec4 mv = modelViewMatrix * vec4(p, 1.0);
    gl_PointSize = aSize * (0.75 + 0.5 * vTw) * uScale / max(0.12, -mv.z);
    vColor = aColor;
    gl_Position = projectionMatrix * mv;
  }
`

const CLOUD_FRAG = /* glsl */ `
  uniform sampler2D uMap;
  uniform float uOpacity;
  varying vec3 vColor;
  varying float vTw;
  void main() {
    float a = texture2D(uMap, gl_PointCoord).a * uOpacity * (0.55 + 0.45 * vTw);
    if (a < 0.012) discard;
    gl_FragColor = vec4(vColor, a);
  }
`

const BURST_VERT = /* glsl */ `
  attribute float aSize;
  attribute float aFade;
  attribute vec3 aColor;
  uniform float uScale;
  varying vec3 vColor;
  varying float vFade;
  void main() {
    vColor = aColor;
    vFade = aFade;
    vec4 mv = modelViewMatrix * vec4(position, 1.0);
    gl_PointSize = aSize * uScale / max(0.12, -mv.z);
    gl_Position = projectionMatrix * mv;
  }
`

const BURST_FRAG = /* glsl */ `
  uniform sampler2D uMap;
  varying vec3 vColor;
  varying float vFade;
  void main() {
    float a = texture2D(uMap, gl_PointCoord).a * vFade;
    if (a < 0.012) discard;
    gl_FragColor = vec4(vColor, a);
  }
`

const PLANE_VERT = /* glsl */ `
  varying vec2 vUv;
  void main() {
    vUv = uv;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  }
`

const NOISE_GLSL = /* glsl */ `
  float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123); }
  float vnoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(
      mix(hash(i), hash(i + vec2(1.0, 0.0)), u.x),
      mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), u.x),
      u.y
    );
  }
  float fbm(vec2 p) {
    float v = 0.0;
    float a = 0.5;
    for (int k = 0; k < 4; k++) {
      v += a * vnoise(p);
      p = p * 2.03 + vec2(17.3, 9.1);
      a *= 0.55;
    }
    return v;
  }
`

const AURORA_FRAG = /* glsl */ `
  uniform float uTime;
  uniform float uOpacity;
  varying vec2 vUv;
  void main() {
    float x = vUv.x * 6.2831;
    float band = sin(x * 1.7 + uTime * 0.6 + sin(x * 0.9 - uTime * 0.37) * 1.4);
    float band2 = sin(x * 2.6 - uTime * 0.43 + 2.1);
    float curtain = 0.5 + 0.28 * band + 0.22 * band2;
    float falloff = smoothstep(0.0, 0.28, vUv.y) * smoothstep(1.0, 0.55, vUv.y);
    float v = clamp(curtain, 0.0, 1.0) * falloff;
    vec3 col = mix(vec3(0.35, 0.88, 0.75), vec3(0.56, 0.48, 1.0), vUv.x + 0.2 * sin(uTime * 0.2));
    gl_FragColor = vec4(col * v, v * uOpacity);
  }
`


/* ============================================================ textures */

/** Canvas textures shared by every scene. Caller owns disposal. */
export function makeTextures(THREE) {
  function texture(size, paint) {
    const cv = document.createElement('canvas')
    cv.width = cv.height = size
    paint(cv.getContext('2d'))
    const tex = new THREE.CanvasTexture(cv)
    tex.colorSpace = THREE.SRGBColorSpace
    return tex
  }
  const dot = texture(64, (c) => {
    const g = c.createRadialGradient(32, 32, 2, 32, 32, 30)
    g.addColorStop(0, 'rgba(255,255,255,1)')
    g.addColorStop(0.4, 'rgba(255,255,255,0.55)')
    g.addColorStop(1, 'rgba(255,255,255,0)')
    c.fillStyle = g
    c.fillRect(0, 0, 64, 64)
  })
  const petal = texture(64, (c) => {
    const g = c.createRadialGradient(32, 26, 4, 32, 32, 30)
    g.addColorStop(0, 'rgba(255,235,244,0.98)')
    g.addColorStop(1, 'rgba(240,150,190,0.85)')
    c.fillStyle = g
    c.beginPath()
    c.ellipse(32, 32, 26, 17, 0.5, 0, Math.PI * 2)
    c.fill()
  })
  const streak = texture(128, (c) => {
    const g = c.createLinearGradient(0, 0, 128, 0)
    g.addColorStop(0, 'rgba(255,255,255,0)')
    g.addColorStop(0.5, 'rgba(255,255,255,1)')
    g.addColorStop(1, 'rgba(255,255,255,0)')
    c.fillStyle = g
    c.fillRect(0, 60, 128, 8)
  })
  // lumpy cloud puff for nebulas / mist / storm clouds
  const puff = texture(128, (c) => {
    for (const [x, y, r, a] of [[50, 60, 34, 0.5], [78, 52, 30, 0.45], [64, 74, 36, 0.5], [60, 44, 22, 0.35]]) {
      const g = c.createRadialGradient(x, y, 2, x, y, r)
      g.addColorStop(0, `rgba(255,255,255,${a})`)
      g.addColorStop(1, 'rgba(255,255,255,0)')
      c.fillStyle = g
      c.fillRect(0, 0, 128, 128)
    }
  })
  // four-point star glint
  const glint = texture(128, (c) => {
    c.translate(64, 64)
    const spike = (rot) => {
      c.save()
      c.rotate(rot)
      c.scale(1, 0.13)
      const g = c.createRadialGradient(0, 0, 0, 0, 0, 62)
      g.addColorStop(0, 'rgba(255,255,255,1)')
      g.addColorStop(0.2, 'rgba(255,255,255,0.85)')
      g.addColorStop(1, 'rgba(255,255,255,0)')
      c.fillStyle = g
      c.beginPath()
      c.arc(0, 0, 62, 0, Math.PI * 2)
      c.fill()
      c.restore()
    }
    spike(0)
    spike(Math.PI / 2)
    const g = c.createRadialGradient(0, 0, 0, 0, 0, 14)
    g.addColorStop(0, 'rgba(255,255,255,1)')
    g.addColorStop(1, 'rgba(255,255,255,0)')
    c.fillStyle = g
    c.beginPath()
    c.arc(0, 0, 14, 0, Math.PI * 2)
    c.fill()
  })
  const moon = texture(128, (c) => {
    let g = c.createRadialGradient(64, 64, 30, 64, 64, 62)
    g.addColorStop(0, 'rgba(226,233,255,0)')
    g.addColorStop(0.86, 'rgba(226,233,255,0.55)')
    g.addColorStop(0.94, 'rgba(226,233,255,0.18)')
    g.addColorStop(1, 'rgba(226,233,255,0)')
    c.fillStyle = g
    c.fillRect(0, 0, 128, 128)
    g = c.createRadialGradient(58, 58, 8, 64, 64, 40)
    g.addColorStop(0, 'rgba(250,252,255,1)')
    g.addColorStop(0.75, 'rgba(214,224,248,1)')
    g.addColorStop(1, 'rgba(190,205,240,0.9)')
    c.fillStyle = g
    c.beginPath()
    c.arc(64, 64, 40, 0, Math.PI * 2)
    c.fill()
    c.fillStyle = 'rgba(150,165,205,0.5)'
    for (const [x, y, r] of [[52, 50, 7], [76, 70, 5], [66, 44, 4], [80, 52, 3.4]]) {
      c.beginPath()
      c.arc(x, y, r, 0, Math.PI * 2)
      c.fill()
    }
  })
  // soft light shafts, faded on BOTH axes so they never read as hard bars.
  // `beam` is bright at its canvas top (for shafts hanging from an anchor
  // above, like ocean god rays); `ray` is bright at its bottom (for shafts
  // rising from a source below, like sunset sun rays).
  const beamGradient = (c, dir) => {
    const g = c.createLinearGradient(0, 0, 0, 128)
    if (dir === 'down') {
      g.addColorStop(0, 'rgba(255,255,255,0.85)')
      g.addColorStop(0.55, 'rgba(255,255,255,0.22)')
      g.addColorStop(1, 'rgba(255,255,255,0)')
    } else {
      g.addColorStop(0, 'rgba(255,255,255,0)')
      g.addColorStop(0.45, 'rgba(255,255,255,0.22)')
      g.addColorStop(1, 'rgba(255,255,255,0.85)')
    }
    c.fillStyle = g
    c.fillRect(0, 0, 128, 128)
    c.globalCompositeOperation = 'destination-in'
    const h = c.createLinearGradient(0, 0, 128, 0)
    h.addColorStop(0, 'rgba(0,0,0,0)')
    h.addColorStop(0.5, 'rgba(0,0,0,1)')
    h.addColorStop(1, 'rgba(0,0,0,0)')
    c.fillStyle = h
    c.fillRect(0, 0, 128, 128)
    c.globalCompositeOperation = 'source-over'
  }
  const beam = texture(128, (c) => beamGradient(c, 'down'))
  const ray = texture(128, (c) => beamGradient(c, 'up'))
  return { dot, petal, streak, puff, glint, moon, beam, ray }
}

/* ====================================================== shared primitives */

/** GPU-driven Points cloud: drift + wrap + sway + twinkle all live in the
 *  vertex shader, so the CPU touches nothing per frame. Exported for the
 *  sibling scene modules (itemFx) that want the same weather. */
export function gpuCloud(api, o) {
  const { THREE, back, owned, rand, w0 } = api
  const {
    count, colors,
    size = [0.06, 0.1], zSpread = api.DEPTH,
    speed = [0, 0], sway = [0.1, 0.4], swayF = [0.3, 0.8], twF = [0.6, 1.6],
    opacity = 1, spin = 0, bound = null, layout = null, offset = [0, 0],
  } = o
  const mx = w0.x + 0.6
  const my = bound ?? w0.y + 0.6
  const pos = new Float32Array(count * 3)
  const col = new Float32Array(count * 3)
  const ph = new Float32Array(count)
  const sz = new Float32Array(count)
  const sp = new Float32Array(count)
  const sw = new Float32Array(count)
  const sf = new Float32Array(count)
  const tf = new Float32Array(count)
  const c = new THREE.Color()
  for (let i = 0; i < count; i++) {
    if (layout) {
      pos[i * 3] = layout[i * 2] + offset[0]
      pos[i * 3 + 1] = layout[i * 2 + 1] + offset[1]
      pos[i * 3 + 2] = -1.2 - Math.abs(layout[i * 2 + 1]) * 0.18
    } else {
      pos[i * 3] = rand(-mx, mx) + offset[0]
      pos[i * 3 + 1] = rand(-my, my) + offset[1]
      pos[i * 3 + 2] = rand(-zSpread, -0.4)
    }
    c.set(colors[Math.floor(rand(0, colors.length))])
    col[i * 3] = c.r
    col[i * 3 + 1] = c.g
    col[i * 3 + 2] = c.b
    ph[i] = rand(0, Math.PI * 2)
    sz[i] = rand(size[0], size[1])
    sp[i] = rand(speed[0], speed[1])
    sw[i] = rand(sway[0], sway[1])
    sf[i] = rand(swayF[0], swayF[1])
    tf[i] = rand(twF[0], twF[1])
  }
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(pos, 3))
  geo.setAttribute('aColor', new THREE.BufferAttribute(col, 3))
  geo.setAttribute('aPhase', new THREE.BufferAttribute(ph, 1))
  geo.setAttribute('aSize', new THREE.BufferAttribute(sz, 1))
  geo.setAttribute('aSpeed', new THREE.BufferAttribute(sp, 1))
  geo.setAttribute('aSway', new THREE.BufferAttribute(sw, 1))
  geo.setAttribute('aSwayF', new THREE.BufferAttribute(sf, 1))
  geo.setAttribute('aTwF', new THREE.BufferAttribute(tf, 1))
  const defines = {}
  if (spin) {
    defines.GALAXY_SPIN = ''
    defines.SPIN_SPEED = spin.toFixed(4)
  }
  const mat = new THREE.ShaderMaterial({
    defines,
    uniforms: {
      uTime: { value: 0 },
      uBound: { value: my },
      uScale: { value: api.pxScale() },
      uMap: { value: api.tex.dot },
      uOpacity: { value: opacity },
    },
    vertexShader: CLOUD_VERT,
    fragmentShader: CLOUD_FRAG,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
  api.onScale((s) => { mat.uniforms.uScale.value = s })
  const pts = new THREE.Points(geo, mat)
  pts.frustumCulled = false
  back.add(pts)
  owned.push(geo, mat)
  return { tick: (t) => { mat.uniforms.uTime.value = t }, mat }
}

/** Soft billboard (Sprite). Returns { obj, mat } for per-frame tweaks. */
export function sprite(api, { tex, color = 0xffffff, scale = 2, scaleY = scale, pos = [0, 0, -2], opacity = 1, blending = 'additive', scene = null }) {
  const { THREE, back, owned } = api
  const mat = new THREE.SpriteMaterial({
    map: tex,
    color,
    transparent: true,
    opacity,
    depthWrite: false,
    blending: blending === 'additive' ? THREE.AdditiveBlending : THREE.NormalBlending,
  })
  const s = new THREE.Sprite(mat)
  s.position.set(pos[0], pos[1], pos[2])
  s.scale.set(scale, scaleY, 1)
  ;(scene ?? back).add(s)
  owned.push(mat)
  return { obj: s, mat }
}

/** Full-screen-ish shader quad with the shared pass-through vertex stage. */
function shaderPlane(api, { w, h, pos = [0, 0, -2], rot = [0, 0, 0], frag, uniforms = {}, opacity = 1, blending = 'additive' }) {
  const { THREE, back, owned } = api
  const geo = new THREE.PlaneGeometry(w, h)
  const mat = new THREE.ShaderMaterial({
    uniforms,
    vertexShader: PLANE_VERT,
    fragmentShader: frag,
    transparent: true,
    depthWrite: false,
    blending: blending === 'additive' ? THREE.AdditiveBlending : THREE.NormalBlending,
  })
  if (opacity !== 1 && uniforms.uOpacity === undefined) uniforms.uOpacity = { value: opacity }
  const m = new THREE.Mesh(geo, mat)
  m.position.set(pos[0], pos[1], pos[2])
  m.rotation.set(rot[0], rot[1], rot[2])
  back.add(m)
  owned.push(geo, mat)
  return { obj: m, mat }
}

/** Stars that flare up at random spots on a slow cycle (gold/royal/galaxy). */
export function glintCycler(api, { n = 3, color = 0xffffff, scale = 0.85, anchors = null, period = 11, z = -1.2 }) {
  const { rand, w0 } = api
  const items = []
  for (let i = 0; i < n; i++) {
    const a = anchors ? anchors[i % anchors.length] : [rand(-0.8, 0.8), rand(-0.7, 0.7)]
    items.push({
      ...sprite(api, { tex: api.tex.glint, color, scale, pos: [a[0] * w0.x, a[1] * w0.y, z], opacity: 0 }),
      seed: rand(0, 1),
    })
  }
  return {
    tick(t) {
      for (let i = 0; i < n; i++) {
        const g = items[i]
        const p = (t / period + g.seed + i / n) % 1
        if (p > 0.22) {
          g.mat.opacity = 0
          continue
        }
        const k = p / 0.22
        g.mat.opacity = glintCurve(k)
        const sc = scale * (0.55 + 0.8 * k)
        g.obj.scale.set(sc, sc, 1)
        g.mat.rotation = t * 0.6 + i * 2.1
      }
    },
  }
}

/* ========================================================== celebration kit */

/**
 * The solve celebration, rendered on the FRONT scene above the cards:
 * a per-skin spark shower (GPU point sprites with per-particle colour/fade),
 * an expanding shockwave ring and a quick core flash.
 */
function makeBurstKit(api, o = {}) {
  const { THREE, front, owned, rand, tex } = api
  const {
    count = 150, colors = [0xffffff], pointTex = tex.dot,
    ring = true, ringColor = 0xffffff,
    flash = true, flashColor = 0xffffff,
    gravity = 2.1, drag = 0.88, up = 0, speed = [1.3, 3.8],
  } = o

  const pos = new Float32Array(count * 3)
  const vel = new Float32Array(count * 2)
  const life = new Float32Array(count)
  const ttl = new Float32Array(count)
  const col = new Float32Array(count * 3)
  const siz = new Float32Array(count)
  const fade = new Float32Array(count)
  const c = new THREE.Color()

  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(pos, 3))
  geo.setAttribute('aColor', new THREE.BufferAttribute(col, 3))
  geo.setAttribute('aSize', new THREE.BufferAttribute(siz, 1))
  geo.setAttribute('aFade', new THREE.BufferAttribute(fade, 1))
  const mat = new THREE.ShaderMaterial({
    uniforms: { uScale: { value: api.pxScale() }, uMap: { value: pointTex } },
    vertexShader: BURST_VERT,
    fragmentShader: BURST_FRAG,
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
  api.onScale((s) => { mat.uniforms.uScale.value = s })
  const points = new THREE.Points(geo, mat)
  points.frustumCulled = false
  points.visible = false
  front.add(points)
  owned.push(geo, mat)

  let cursor = 0
  let active = false

  const ringGeo = new THREE.RingGeometry(0.46, 0.5, 72)
  const ringMat = new THREE.MeshBasicMaterial({
    color: ringColor, transparent: true, opacity: 0, depthWrite: false,
    blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
  })
  const ringMesh = new THREE.Mesh(ringGeo, ringMat)
  ringMesh.visible = false
  front.add(ringMesh)
  owned.push(ringGeo, ringMat)

  const flashMat = new THREE.SpriteMaterial({
    map: tex.dot, color: flashColor, transparent: true, opacity: 0,
    depthWrite: false, blending: THREE.AdditiveBlending,
  })
  const flashSpr = new THREE.Sprite(flashMat)
  flashSpr.visible = false
  front.add(flashSpr)
  owned.push(flashMat)

  let ringT = -1
  let ringS = 1
  let flashT = -1
  let flashS = 1

  function emit(x, y, s = 1) {
    const n = Math.min(count, Math.round(count * (0.3 + 0.55 * Math.min(s, 1.5))))
    for (let k = 0; k < n; k++) {
      const i = cursor
      cursor = (cursor + 1) % count
      const a = rand(0, Math.PI * 2)
      const sp = rand(speed[0], speed[1]) * Math.pow(Math.max(s, 0.2), 0.7)
      vel[i * 2] = Math.cos(a) * sp
      vel[i * 2 + 1] = Math.sin(a) * sp * 0.85 + up * s
      pos[i * 3] = x + rand(-0.16, 0.16)
      pos[i * 3 + 1] = y + rand(-0.1, 0.1)
      pos[i * 3 + 2] = rand(-1.7, -0.7)
      ttl[i] = life[i] = rand(0.5, 1.1) * (0.7 + 0.45 * s)
      siz[i] = rand(0.05, 0.115) * (0.75 + 0.55 * s)
      c.set(colors[Math.floor(rand(0, colors.length))])
      col[i * 3] = c.r
      col[i * 3 + 1] = c.g
      col[i * 3 + 2] = c.b
    }
    active = true
    points.visible = true
    geo.attributes.position.needsUpdate = true
    geo.attributes.aColor.needsUpdate = true
    geo.attributes.aSize.needsUpdate = true
    if (ring) {
      ringT = 0
      ringS = s
      ringMesh.visible = true
      ringMesh.position.set(x, y, -0.9)
    }
    if (flash) {
      flashT = 0
      flashS = s
      flashSpr.visible = true
      flashSpr.position.set(x, y, -0.8)
    }
  }

  function update(dt) {
    if (active) {
      const dragF = Math.pow(drag, dt * 60)
      let alive = 0
      for (let i = 0; i < count; i++) {
        if (life[i] <= 0) continue
        life[i] -= dt
        if (life[i] <= 0) {
          fade[i] = 0
          continue
        }
        alive++
        vel[i * 2 + 1] -= gravity * dt
        vel[i * 2] *= dragF
        vel[i * 2 + 1] *= dragF
        pos[i * 3] += vel[i * 2] * dt
        pos[i * 3 + 1] += vel[i * 2 + 1] * dt
        fade[i] = burstFade(life[i], ttl[i])
      }
      geo.attributes.position.needsUpdate = true
      geo.attributes.aFade.needsUpdate = true
      if (!alive) {
        active = false
        points.visible = false
      }
    }
    if (ringT >= 0) {
      ringT += dt
      const k = ringT / 0.55
      if (k >= 1) {
        ringT = -1
        ringMesh.visible = false
      } else {
        const e = 1 - Math.pow(1 - k, 3)
        ringMesh.scale.setScalar((0.35 + 3.4 * e) * (0.6 + 0.55 * ringS))
        ringMat.opacity = (1 - k) * (1 - k) * 0.75
      }
    }
    if (flashT >= 0) {
      flashT += dt
      const k = flashT / 0.3
      if (k >= 1) {
        flashT = -1
        flashSpr.visible = false
      } else {
        const sc = (0.5 + 2.4 * k) * (0.5 + 0.6 * flashS)
        flashSpr.scale.set(sc, sc, 1)
        flashMat.opacity = 0.9 * Math.pow(1 - k, 1.6)
      }
    }
  }

  return { emit, update }
}

/* ================================================================ scenes */

function galaxy(api) {
  const { THREE, back, owned, rand, tex, w0 } = api
  // rotating spiral disc — spin lives in the vertex shader
  const disk = gpuCloud(api, {
    count: 820,
    colors: [0xffffff, 0xcfc3f2, 0xef9fd8, 0x9db8ee],
    size: [0.045, 0.1],
    spin: 0.16,
    bound: 1000, // a disc, not a drift — never wrap
    opacity: 0.9,
    layout: spiralLayout(820, 2, 3.8),
  })
  const far = gpuCloud(api, { count: 240, colors: [0xffffff, 0xcfc3f2], size: [0.03, 0.06], speed: [-0.05, -0.14], opacity: 0.7 })
  const core = sprite(api, { tex: tex.puff, color: 0xb9a5ff, scale: 4.2, scaleY: 3.2, pos: [0, 0, -2.6], opacity: 0.4 })
  const neb1 = sprite(api, { tex: tex.puff, color: 0xef9fd8, scale: 5.5, scaleY: 3.4, pos: [-w0.x * 0.4, w0.y * 0.3, -3.2], opacity: 0.2 })
  const neb2 = sprite(api, { tex: tex.puff, color: 0x6a5acd, scale: 6, scaleY: 3.8, pos: [w0.x * 0.45, -w0.y * 0.35, -3.4], opacity: 0.18 })
  // shooting star: an additive streak that periodically rips across
  const shootMat = new THREE.MeshBasicMaterial({
    map: tex.streak, transparent: true, opacity: 0, depthWrite: false,
    blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
  })
  const shoot = new THREE.Mesh(new THREE.PlaneGeometry(3.4, 0.09), shootMat)
  shoot.visible = false
  shoot.rotation.z = -0.42
  back.add(shoot)
  owned.push(shoot.geometry, shootMat)
  let nextShoot = 2.5
  let shootT = -1
  const glints = glintCycler(api, { n: 2, scale: 0.7, period: 13 })
  const kit = makeBurstKit(api, {
    colors: [0xffffff, 0xb9a5ff, 0xef9fd8, 0xffd166],
    ringColor: 0xb9a5ff, flashColor: 0xffffff,
  })
  return {
    update(t, dt, w) {
      disk.tick(t)
      far.tick(t)
      glints.tick(t)
      core.mat.opacity = 0.32 + Math.sin(t * 1.1) * 0.12
      neb1.mat.opacity = 0.16 + Math.sin(t * 0.7 + 1) * 0.08
      neb2.mat.opacity = 0.14 + Math.sin(t * 0.55 + 3) * 0.07
      nextShoot -= dt
      if (shootT < 0 && nextShoot <= 0) {
        shootT = 0
        nextShoot = rand(4.5, 9)
        shoot.position.set(rand(-w.x, w.x * 0.3), rand(w.y * 0.15, w.y * 0.85), -1.5)
      }
      if (shootT >= 0) {
        shootT += dt
        shoot.visible = true
        shoot.position.x += dt * 9
        shoot.position.y -= dt * 3.6
        shootMat.opacity = Math.max(0, 0.9 - shootT * 1.5)
        if (shootT > 0.8) {
          shootT = -1
          shoot.visible = false
        }
      }
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function midnight(api) {
  const { back, owned, rand, tex, w0 } = api
  const moon = sprite(api, { tex: tex.moon, scale: 1.7, pos: [w0.x * 0.52, w0.y * 0.5, -3], opacity: 0.95 })
  const halo = sprite(api, { tex: tex.dot, color: 0x8fb2f5, scale: 4.6, scaleY: 4.2, pos: [w0.x * 0.52, w0.y * 0.5, -3.05], opacity: 0.2 })
  const aurora = shaderPlane(api, {
    w: 12, h: 3.6, pos: [0, w0.y * 0.42, -3.6],
    frag: AURORA_FRAG,
    uniforms: { uTime: { value: 0 }, uOpacity: { value: 0.34 } },
  })
  const stars = gpuCloud(api, { count: 260, colors: [0xffffff, 0xcdd8f5, 0x9db8ee], size: [0.03, 0.07], speed: [-0.02, 0.05], opacity: 0.85 })
  // dark storm puffs drift over the stars (normal blending so they occlude)
  const clouds = []
  for (const [fy, sc, sp] of [[0.1, 6, 0.05], [-0.55, 4.4, -0.035]]) {
    clouds.push({
      ...sprite(api, { tex: tex.puff, color: 0x0c1326, scale: sc, scaleY: sc * 0.5, pos: [rand(-w0.x, w0.x), fy * w0.y, -2.8], opacity: 0.5, blending: 'normal' }),
      sp,
    })
  }
  const glints = glintCycler(api, { n: 2, color: 0xcdd8f5, scale: 0.6, period: 12 })
  const kit = makeBurstKit(api, {
    colors: [0xffffff, 0xcdd8f5, 0x8fb2f5],
    ringColor: 0x8fb2f5, flashColor: 0xe6eeff,
    gravity: 1.6, drag: 0.9,
  })
  return {
    update(t, dt, w) {
      stars.tick(t)
      glints.tick(t)
      aurora.mat.uniforms.uTime.value = t
      moon.mat.opacity = 0.88 + Math.sin(t * 0.9) * 0.07
      halo.mat.opacity = 0.16 + Math.sin(t * 1.3) * 0.08
      for (const c of clouds) {
        c.obj.position.x += c.sp * dt
        if (c.obj.position.x > w.x + 3.2) c.obj.position.x = -w.x - 3.2
        if (c.obj.position.x < -w.x - 3.2) c.obj.position.x = w.x + 3.2
      }
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function inferno(api) {
  const { tex } = api
  // ember storm: sparks drift across the whole board over a breathing molten
  // glow — deliberately shapeless, so no UI layer can ever slice it
  const embers = gpuCloud(api, {
    count: 210,
    colors: [0xffb347, 0xff7a33, 0xffcf6b, 0xff5c33],
    size: [0.04, 0.11],
    speed: [0.4, 1.3],
    sway: [0.25, 0.7],
    twF: [0.7, 1.8],
    opacity: 0.95,
  })
  const glow = sprite(api, { tex: tex.puff, color: 0xff5c33, scale: 9, scaleY: 4.2, pos: [0, -3.3, -1.6], opacity: 0.28 })
  const haze = sprite(api, { tex: tex.puff, color: 0xff7a33, scale: 6.5, scaleY: 3.2, pos: [0, 2.6, -2.2], opacity: 0.12 })
  const kit = makeBurstKit(api, {
    colors: [0xffd166, 0xffb347, 0xff5c33, 0xffffff],
    ringColor: 0xff7a33, flashColor: 0xffc178,
    gravity: 1.1, drag: 0.9, up: 1.1,
  })
  return {
    update(t, dt) {
      embers.tick(t)
      glow.mat.opacity = 0.24 + Math.sin(t * 1.4) * 0.08
      haze.mat.opacity = 0.1 + Math.sin(t * 0.9 + 2) * 0.04
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function sakura(api) {
  const { THREE, back, owned, rand, tex, w0, DEPTH } = api
  const petalMat = new THREE.MeshBasicMaterial({
    map: tex.petal, transparent: true, opacity: 0.85,
    depthWrite: false, side: THREE.DoubleSide,
  })
  const geo = new THREE.PlaneGeometry(0.34, 0.22)
  owned.push(geo, petalMat)
  const petals = []
  for (let i = 0; i < 40; i++) {
    const m = new THREE.Mesh(geo, petalMat)
    m.position.set(rand(-w0.x, w0.x), rand(-w0.y, w0.y), rand(-DEPTH, -0.4))
    back.add(m)
    petals.push({
      mesh: m,
      vy: rand(0.3, 0.62),
      sway: rand(0.25, 0.7),
      swayF: rand(0.5, 1.2),
      phase: rand(0, Math.PI * 2),
      rx: rand(-1.4, 1.4),
      ry: rand(-1.8, 1.8),
    })
  }
  const bokeh1 = sprite(api, { tex: tex.dot, color: 0xf3a9c6, scale: 3.6, pos: [-w0.x * 0.5, w0.y * 0.35, -3], opacity: 0.12 })
  const bokeh2 = sprite(api, { tex: tex.dot, color: 0xe56d9c, scale: 2.8, pos: [w0.x * 0.55, -w0.y * 0.4, -3.2], opacity: 0.1 })
  const kit = makeBurstKit(api, {
    count: 170,
    colors: [0xf3a9c6, 0xffffff, 0xe56d9c],
    ringColor: 0xe56d9c, flashColor: 0xffe4ef,
    gravity: 1.5, drag: 0.92, up: 0.6,
  })
  return {
    update(t, dt, w) {
      const gust = Math.sin(t * 0.5) * Math.sin(t * 0.13) * 1.3
      for (const p of petals) {
        p.mesh.position.y -= p.vy * dt
        p.mesh.position.x += (Math.sin(t * p.swayF + p.phase) * p.sway + gust) * dt
        p.mesh.rotation.x += p.rx * dt
        p.mesh.rotation.y += p.ry * dt
        if (p.mesh.position.y < -w.y - 0.4) p.mesh.position.set(rand(-w.x, w.x), w.y + 0.3, rand(-DEPTH, -0.4))
        if (p.mesh.position.x > w.x + 0.6) p.mesh.position.x = -w.x - 0.5
        if (p.mesh.position.x < -w.x - 0.6) p.mesh.position.x = w.x + 0.5
      }
      bokeh1.mat.opacity = 0.09 + Math.sin(t * 0.9) * 0.05
      bokeh2.mat.opacity = 0.08 + Math.sin(t * 1.2 + 2) * 0.04
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function forest(api) {
  const { tex, w0 } = api
  const flies = gpuCloud(api, {
    count: 46,
    colors: [0xffe9a3, 0xfff3c4, 0xd8e26f],
    size: [0.1, 0.18],
    speed: [-0.06, 0.06],
    sway: [0.5, 1.2],
    swayF: [0.25, 0.7],
    twF: [0.5, 1.4],
    opacity: 0.95,
  })
  const pollen = gpuCloud(api, {
    count: 90,
    colors: [0xd8e26f, 0xf3ecd6],
    size: [0.02, 0.045],
    speed: [0.03, 0.14],
    opacity: 0.55,
  })
  const mist = sprite(api, { tex: tex.puff, color: 0x7f9b5e, scale: 7.5, scaleY: 4.5, pos: [0, -w0.y * 0.5, -3.4], opacity: 0.14 })
  const kit = makeBurstKit(api, {
    colors: [0xffe9a3, 0xd8e26f, 0xffffff],
    ringColor: 0xd8e26f, flashColor: 0xfff3c4,
    gravity: 1.1, drag: 0.9, up: 0.5,
  })
  return {
    update(t, dt) {
      flies.tick(t)
      pollen.tick(t)
      mist.mat.opacity = 0.1 + Math.sin(t * 0.6) * 0.05
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function ocean(api) {
  const { THREE, back, owned, rand, tex, w0 } = api
  // god rays leaning in from the top edge
  const beamGeo = new THREE.PlaneGeometry(0.8, 8)
  beamGeo.translate(0, -4, 0)
  owned.push(beamGeo)
  const beams = []
  for (const [x, base, sw] of [[-w0.x * 0.4, 0.16, 0.06], [w0.x * 0.1, -0.08, 0.05], [w0.x * 0.5, 0.3, 0.07]]) {
    const mat = new THREE.MeshBasicMaterial({
      map: tex.beam, color: 0x9fe8ff, transparent: true, opacity: 0.1,
      depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
    })
    const m = new THREE.Mesh(beamGeo, mat)
    m.position.set(x, w0.y + 0.6, -2.8)
    m.rotation.z = base
    back.add(m)
    owned.push(mat)
    beams.push({ m, mat, base, sw })
  }
  const bubbles = gpuCloud(api, {
    count: 130,
    colors: [0x7fdcff, 0xbfefff, 0xffffff],
    size: [0.035, 0.085],
    speed: [0.16, 0.55],
    sway: [0.1, 0.35],
    opacity: 0.7,
  })
  const caustic1 = sprite(api, { tex: tex.puff, color: 0x5fd4ff, scale: 5, scaleY: 3, pos: [-w0.x * 0.35, w0.y * 0.2, -3], opacity: 0.1 })
  const caustic2 = sprite(api, { tex: tex.puff, color: 0x5fd4ff, scale: 4, scaleY: 2.6, pos: [w0.x * 0.4, -w0.y * 0.3, -3.2], opacity: 0.09 })
  const kit = makeBurstKit(api, {
    colors: [0x7fdcff, 0xbfefff, 0xffffff],
    ringColor: 0x5fd4ff, flashColor: 0xd6f6ff,
    gravity: 0.7, drag: 0.9,
  })
  return {
    update(t, dt) {
      bubbles.tick(t)
      for (const b of beams) {
        b.m.rotation.z = b.base + Math.sin(t * 0.3 + b.base * 7) * b.sw
        b.mat.opacity = 0.1 + Math.sin(t * 0.7 + b.base * 9) * 0.045
      }
      caustic1.obj.position.x = -w0.x * 0.35 + Math.sin(t * 0.22) * 0.8
      caustic2.obj.position.x = w0.x * 0.4 + Math.sin(t * 0.17 + 2) * 0.7
      caustic1.mat.opacity = 0.08 + Math.sin(t * 1.1) * 0.04
      caustic2.mat.opacity = 0.07 + Math.sin(t * 0.9 + 1.6) * 0.035
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function neon(api) {
  const { THREE, back, owned, rand, tex } = api
  const grid = shaderPlane(api, {
    w: 16, h: 7, pos: [0, -1.9, -2.4], rot: [-1.15, 0, 0],
    frag: GRID_FRAG,
    uniforms: { uTime: { value: 0 }, uGlitch: { value: 0 }, uOpacity: { value: 0.6 } },
  })
  const green = gpuCloud(api, { count: 95, colors: [0x3dff8f], size: [0.05, 0.09], speed: [-0.2, -0.05], opacity: 0.85 })
  const pink = gpuCloud(api, { count: 95, colors: [0xff3d8f], size: [0.05, 0.09], speed: [0.05, 0.2], opacity: 0.85 })
  // a slow scanline sweeping down the board, like a bad-idea laser
  const scanMat = new THREE.MeshBasicMaterial({
    map: tex.streak, color: 0x3dff8f, transparent: true, opacity: 0,
    depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
  })
  const scan = new THREE.Mesh(new THREE.PlaneGeometry(20, 0.5), scanMat)
  back.add(scan)
  owned.push(scan.geometry, scanMat)
  const kit = makeBurstKit(api, {
    colors: [0x3dff8f, 0xff3d8f, 0xffffff],
    ringColor: 0x3dff8f, flashColor: 0xbfffd9,
    gravity: 1.8,
  })
  let nextGlitch = 3
  let glitchT = -1
  return {
    update(t, dt, w) {
      grid.mat.uniforms.uTime.value = t
      green.tick(t)
      pink.tick(t)
      const beat = (Math.sin(t * 1.8) + 1) / 2
      green.mat.uniforms.uOpacity.value = 0.45 + beat * 0.5
      pink.mat.uniforms.uOpacity.value = 0.95 - beat * 0.5
      const cyc = (t % 6) / 6
      if (cyc < 0.18) {
        scan.visible = true
        scan.position.y = w.y - (cyc / 0.18) * w.y * 2
        scanMat.opacity = 0.22 * Math.sin((cyc / 0.18) * Math.PI)
      } else scan.visible = false
      // scheduled glitch tears
      nextGlitch -= dt
      if (glitchT < 0 && nextGlitch <= 0) {
        glitchT = 0.12
        nextGlitch = rand(3.5, 7.5)
      }
      if (glitchT >= 0) {
        glitchT -= dt
        grid.mat.uniforms.uGlitch.value = Math.random() * 0.5
        if (glitchT <= 0) grid.mat.uniforms.uGlitch.value = 0
      }
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function gold(api) {
  const { THREE, back, owned, tex, w0 } = api
  const glitter = gpuCloud(api, {
    count: 190,
    colors: [0xffe08a, 0xffd166, 0xfff3c4],
    size: [0.03, 0.075],
    speed: [-0.42, -0.12],
    sway: [0.05, 0.2],
    twF: [1.2, 3],
    opacity: 0.85,
  })
  const glints = glintCycler(api, {
    n: 3, scale: 0.9, period: 10,
    anchors: [[-0.55, 0.45], [0.42, -0.2], [0.08, 0.58]],
  })
  // metallic sheen sweeping across the whole table
  const sheenMat = new THREE.MeshBasicMaterial({
    map: tex.streak, color: 0xfff3c4, transparent: true, opacity: 0,
    depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
  })
  const sheen = new THREE.Mesh(new THREE.PlaneGeometry(3.4, 7), sheenMat)
  sheen.rotation.z = -0.55
  back.add(sheen)
  owned.push(sheen.geometry, sheenMat)
  const kit = makeBurstKit(api, {
    colors: [0xffe08a, 0xffd166, 0xffffff, 0xffb347],
    ringColor: 0xffe08a, flashColor: 0xfff3c4,
    gravity: 3.2, up: 1.2, drag: 0.92,
  })
  return {
    update(t, dt, w) {
      glitter.tick(t)
      glints.tick(t)
      const cyc = (t % 6) / 6
      if (cyc < 0.22) {
        sheen.visible = true
        sheen.position.x = -w.x * 1.4 + (cyc / 0.22) * w.x * 2.8
        sheenMat.opacity = 0.22 * Math.sin((cyc / 0.22) * Math.PI)
      } else sheen.visible = false
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function royal(api) {
  const { THREE, back, owned, tex, w0 } = api
  const mist1 = sprite(api, { tex: tex.puff, color: 0x6a4fae, scale: 6.5, scaleY: 4, pos: [-w0.x * 0.35, -w0.y * 0.2, -3.2], opacity: 0.2 })
  const mist2 = sprite(api, { tex: tex.puff, color: 0x8f5fc0, scale: 5, scaleY: 3.2, pos: [w0.x * 0.4, w0.y * 0.3, -3.4], opacity: 0.16 })
  const dust = gpuCloud(api, {
    count: 170,
    colors: [0xc9a7ff, 0xf3d98b, 0xffffff],
    size: [0.03, 0.07],
    speed: [-0.2, -0.05],
    twF: [0.8, 2.2],
    opacity: 0.8,
  })
  // a slowly orbiting gold halo ring behind the cards
  const halo = gpuCloud(api, {
    count: 64,
    colors: [0xf3d98b, 0xffe9a8],
    size: [0.05, 0.09],
    spin: 0.22,
    bound: 1000,
    opacity: 0.85,
    layout: circleLayout(64, 2.7),
    offset: [0, 0.55],
  })
  const glints = glintCycler(api, { n: 2, color: 0xf3d98b, scale: 0.75, period: 11 })
  const kit = makeBurstKit(api, {
    colors: [0xf3d98b, 0xc9a7ff, 0xffffff],
    ringColor: 0xf3d98b, flashColor: 0xefe2ff,
    gravity: 1.7, drag: 0.89,
  })
  return {
    update(t, dt) {
      dust.tick(t)
      halo.tick(t)
      glints.tick(t)
      mist1.mat.opacity = 0.16 + Math.sin(t * 0.8) * 0.07
      mist2.mat.opacity = 0.13 + Math.sin(t * 0.65 + 2) * 0.06
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

function sunset(api) {
  const { THREE, back, owned, rand, tex, w0 } = api
  const sunY = -w0.y * 0.52
  const halo = sprite(api, { tex: tex.dot, color: 0xff9a5c, scale: 6.5, scaleY: 4.6, pos: [0, sunY, -3.2], opacity: 0.3 })
  const sun = sprite(api, { tex: tex.dot, color: 0xffd166, scale: 2.6, scaleY: 2, pos: [0, sunY, -3.1], opacity: 0.9 })
  // sun rays pivoting at the disc (soft on both axes; bright end at the sun)
  const beamGeo = new THREE.PlaneGeometry(0.9, 7.5)
  beamGeo.translate(0, 3.75, 0)
  owned.push(beamGeo)
  const beams = []
  for (const [ang, sw] of [[-0.55, 0.08], [-0.12, 0.06], [0.28, 0.07], [0.62, 0.05]]) {
    const mat = new THREE.MeshBasicMaterial({
      map: tex.ray, color: 0xffc178, transparent: true, opacity: 0.16,
      depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide,
    })
    const m = new THREE.Mesh(beamGeo, mat)
    m.position.set(0, sunY + 0.4, -2.9)
    m.rotation.z = ang
    back.add(m)
    owned.push(mat)
    beams.push({ m, mat, base: ang, sw })
  }
  const clouds = []
  for (const [fy, colr, sc, sp] of [[0.42, 0xff9a5c, 5.5, 0.1], [0.05, 0xf4739e, 4.2, -0.07], [0.66, 0xffc178, 3.4, 0.05]]) {
    clouds.push({
      ...sprite(api, { tex: tex.puff, color: colr, scale: sc, scaleY: sc * 0.55, pos: [rand(-1, 1) * w0.x, fy * w0.y, -2.6], opacity: 0.3, blending: 'normal' }),
      sp,
    })
  }
  const motes = gpuCloud(api, {
    count: 120,
    colors: [0xffd166, 0xffb473, 0xfff1c4],
    size: [0.035, 0.07],
    speed: [0.05, 0.22],
    opacity: 0.7,
  })
  const kit = makeBurstKit(api, {
    colors: [0xffd166, 0xff9a5c, 0xffffff, 0xf4739e],
    ringColor: 0xffb473, flashColor: 0xffe3a1,
    gravity: 1.8, up: 0.5,
  })
  return {
    update(t, dt, w) {
      motes.tick(t)
      sun.mat.opacity = 0.86 + Math.sin(t * 1.6) * 0.12
      halo.mat.opacity = 0.24 + Math.sin(t * 1.2) * 0.09
      for (const b of beams) {
        b.m.rotation.z = b.base + Math.sin(t * 0.4 + b.base * 5) * b.sw
        b.mat.opacity = 0.13 + Math.sin(t * 0.6 + b.base * 8) * 0.05
      }
      for (const c of clouds) {
        c.obj.position.x += c.sp * dt
        if (c.obj.position.x > w.x + 3) c.obj.position.x = -w.x - 3
        if (c.obj.position.x < -w.x - 3) c.obj.position.x = w.x + 3
      }
      kit.update(dt)
    },
    burst: kit.emit,
  }
}

const SCENES = { galaxy, midnight, inferno, sakura, forest, ocean, neon, gold, royal, sunset }

/** Build the scene set for a skin id. Returns { update, burst } ({} if quiet). */
export function buildSkinScene(api) {
  const fn = SCENES[api.skin]
  return fn ? fn(api) : {}
}
