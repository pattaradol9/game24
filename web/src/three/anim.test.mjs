// The point of these helpers is that motion looks the same no matter how often
// frames arrive — so that is exactly what the tests pin down.
import test from 'node:test'
import assert from 'node:assert/strict'
import { clamp, damp, qualityGuard, toonGradient } from './anim.js'

// exact step count, so two frame rates simulate exactly the same amount of time
const run = (fps, seconds, fn) => {
  const dt = 1 / fps
  for (let i = Math.round(fps * seconds); i > 0; i--) fn(dt)
}

test('clamp stays in range', () => {
  assert.equal(clamp(5, 0, 1), 1)
  assert.equal(clamp(-5, 0, 1), 0)
  assert.equal(clamp(0.5, 0, 1), 0.5)
})

test('damp lands in the same place at 30, 60 and 144 fps', () => {
  const settle = (fps) => {
    let v = 0
    run(fps, 1, (dt) => { v = damp(v, 10, 6, dt) })
    return v
  }
  const a = settle(30)
  const b = settle(60)
  const c = settle(144)
  assert.ok(Math.abs(a - b) < 1e-9, `30 vs 60 fps: ${a} vs ${b}`)
  assert.ok(Math.abs(b - c) < 1e-9, `60 vs 144 fps: ${b} vs ${c}`)
  assert.ok(Math.abs(a - 10 * (1 - Math.exp(-6))) < 1e-9)
})

test('damp with a zero step changes nothing', () => {
  assert.equal(damp(3, 99, 12, 0), 3)
})

test('toonGradient builds a single-row ramp texture', () => {
  const stub = {
    DataTexture: class { constructor(data, w, h, fmt) { Object.assign(this, { image: { data, width: w, height: h }, format: fmt }) } },
    RedFormat: 'red',
    NearestFilter: 'nearest',
  }
  const tex = toonGradient(stub, [0, 128, 255])
  assert.equal(tex.image.width, 3)
  assert.equal(tex.image.height, 1)
  assert.equal(tex.magFilter, 'nearest')
  assert.equal(tex.needsUpdate, true)
})

test('qualityGuard drops resolution on slow frames and restores it on fast ones', () => {
  const seen = []
  const renderer = { setPixelRatio: (v) => seen.push(v) }
  global.window = { devicePixelRatio: 2 }
  const guard = qualityGuard(renderer, { max: 2, min: 1 })
  assert.equal(guard.dpr, 2)

  for (let i = 0; i < 400; i++) guard.sample(40) // a device that cannot keep up
  assert.ok(guard.dpr < 2, `expected a drop, still at ${guard.dpr}`)

  for (let i = 0; i < 600; i++) guard.sample(16) // comfortable again
  assert.equal(guard.dpr, 2)
  delete global.window
})
