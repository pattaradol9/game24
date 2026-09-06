// Juice manager — the "game feel" layer: floating combat text, particle
// sparkles, screen shake, haptics. All zero-dep, DOM/canvas based, and every
// helper no-ops under prefers-reduced-motion where motion isn't essential.

const reduced = () =>
  window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false

/* ---------- floating combat text ---------- */
export function popText(x, y, text, cls = '') {
  const el = document.createElement('div')
  el.className = `fx-pop ${cls}`
  el.textContent = text
  el.style.left = `${x}px`
  el.style.top = `${y}px`
  document.body.appendChild(el)
  setTimeout(() => el.remove(), 1100)
}

/* ---------- sparkle burst at a screen position ---------- */
let fxCanvas = null
let fxParts = []
let fxRaf = 0

function ensureFxCanvas() {
  if (fxCanvas) return fxCanvas
  fxCanvas = document.createElement('canvas')
  Object.assign(fxCanvas.style, {
    position: 'fixed', inset: 0, width: '100vw', height: '100vh',
    pointerEvents: 'none', zIndex: 998,
  })
  document.body.appendChild(fxCanvas)
  return fxCanvas
}

const SPARK_COLORS = ['#ffd166', '#ff9f1c', '#4cc9f0', '#ff5d8f', '#06d6a0', '#fff']

// kind: 'stars' | 'ring' — small burst used on card merges / button pokes
export function sparkle(x, y, { count = 14, power = 1, kind = 'stars' } = {}) {
  const cv = ensureFxCanvas()
  cv.width = innerWidth
  cv.height = innerHeight
  for (let i = 0; i < count; i++) {
    const a = Math.random() * Math.PI * 2
    const sp = (2 + Math.random() * 4) * power
    fxParts.push({
      x, y,
      vx: Math.cos(a) * sp,
      vy: Math.sin(a) * sp - 1.5,
      life: 34 + Math.random() * 18,
      size: 3 + Math.random() * 4,
      color: SPARK_COLORS[(Math.random() * SPARK_COLORS.length) | 0],
      star: kind === 'stars' && Math.random() < 0.5,
    })
  }
  if (!fxRaf) fxRaf = requestAnimationFrame(fxTick)
}

function fxTick() {
  const ctx = fxCanvas.getContext('2d')
  ctx.clearRect(0, 0, fxCanvas.width, fxCanvas.height)
  fxParts = fxParts.filter((p) => p.life > 0)
  for (const p of fxParts) {
    p.x += p.vx
    p.y += p.vy
    p.vy += 0.18
    p.vx *= 0.98
    p.life--
    const alpha = Math.min(1, p.life / 22)
    ctx.save()
    ctx.globalAlpha = alpha
    ctx.translate(p.x, p.y)
    ctx.fillStyle = p.color
    if (p.star) {
      drawStar(ctx, p.size)
    } else {
      ctx.beginPath()
      ctx.arc(0, 0, p.size * 0.6, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.restore()
  }
  if (fxParts.length) fxRaf = requestAnimationFrame(fxTick)
  else { cancelAnimationFrame(fxRaf); fxRaf = 0 }
}

function drawStar(ctx, r) {
  ctx.beginPath()
  for (let i = 0; i < 10; i++) {
    const rad = i % 2 === 0 ? r : r * 0.45
    const a = (Math.PI / 5) * i - Math.PI / 2
    ctx.lineTo(Math.cos(a) * rad, Math.sin(a) * rad)
  }
  ctx.closePath()
  ctx.fill()
}

/* ---------- screen shake ---------- */
export function shake(intensity = 1) {
  if (reduced()) return
  const app = document.getElementById('app')
  if (!app) return
  app.classList.remove('fx-shake')
  // restart the animation even when the class is already there
  void app.offsetWidth
  app.style.setProperty('--shake', String(4 + intensity * 4))
  app.classList.add('fx-shake')
  setTimeout(() => app.classList.remove('fx-shake'), 420)
}

/* ---------- danger vignette (time running out) ---------- */
let vigEl = null
export function danger(level) {
  // level: 0..1 — 0 hides the vignette
  if (!vigEl) {
    vigEl = document.createElement('div')
    vigEl.className = 'fx-vignette'
    document.body.appendChild(vigEl)
  }
  vigEl.style.opacity = String(level)
}

/* ---------- haptics (mobile) ---------- */
export function haptic(pattern = 12) {
  try { navigator.vibrate?.(pattern) } catch { /* unsupported */ }
}

/* ---------- helpers for element positions ---------- */
export function centerOf(el) {
  if (!el) return null
  const r = el.getBoundingClientRect()
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 }
}

// find a card tile element on screen by its card id
export function cardEl(cardId) {
  return document.querySelector(`[data-card-id="${CSS.escape(cardId)}"]`)
}
