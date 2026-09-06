// Canvas confetti burst — cartoon celebration, zero deps.
let canvas = null
let particles = []
let rafId = null

const COLORS = ['#ff9f1c', '#ff5d8f', '#06d6a0', '#4cc9f0', '#9b5de5', '#ffd166']

function ensureCanvas() {
  if (canvas) return canvas
  canvas = document.createElement('canvas')
  Object.assign(canvas.style, {
    position: 'fixed', inset: 0, width: '100vw', height: '100vh',
    pointerEvents: 'none', zIndex: 999,
  })
  document.body.appendChild(canvas)
  return canvas
}

export function burst(count = 120) {
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return
  const cv = ensureCanvas()
  cv.width = innerWidth
  cv.height = innerHeight
  for (let i = 0; i < count; i++) {
    particles.push({
      x: innerWidth / 2 + (Math.random() - 0.5) * 200,
      y: innerHeight * 0.35,
      vx: (Math.random() - 0.5) * 14,
      vy: -Math.random() * 13 - 4,
      rot: Math.random() * Math.PI,
      vr: (Math.random() - 0.5) * 0.3,
      size: 6 + Math.random() * 8,
      color: COLORS[Math.floor(Math.random() * COLORS.length)],
      life: 90 + Math.random() * 40,
    })
  }
  if (!rafId) rafId = requestAnimationFrame(tick)
}

function tick() {
  const ctx = canvas.getContext('2d')
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  particles = particles.filter((p) => p.life > 0)
  for (const p of particles) {
    p.x += p.vx
    p.y += p.vy
    p.vy += 0.35
    p.rot += p.vr
    p.life--
    ctx.save()
    ctx.translate(p.x, p.y)
    ctx.rotate(p.rot)
    ctx.fillStyle = p.color
    ctx.strokeStyle = 'rgba(32,35,58,.8)'
    ctx.lineWidth = 1.5
    ctx.fillRect(-p.size / 2, -p.size / 2, p.size, p.size * 0.7)
    ctx.strokeRect(-p.size / 2, -p.size / 2, p.size, p.size * 0.7)
    ctx.restore()
  }
  if (particles.length > 0) {
    rafId = requestAnimationFrame(tick)
  } else {
    cancelAnimationFrame(rafId)
    rafId = null
  }
}
