// The vault scene's shape lives in a handful of pure helpers — these pin
// them, so the ambience can be re-tuned without silently breaking layout.
// (The scene builder itself is glue over skinScenes' primitives, same as the
// per-skin builders: only the pure math is node-testable.)
import test from 'node:test'
import assert from 'node:assert/strict'
import { breath, haloRadius, sweep } from './itemFx.js'

test('haloRadius hugs the narrow axis and caps out', () => {
  assert.equal(haloRadius({ x: 4, y: 2 }), 1) // half of the narrow axis
  assert.equal(haloRadius({ x: 2, y: 6 }), 1)
  assert.equal(haloRadius({ x: 40, y: 30 }), 3.2) // never grows past the cap
})

test('sweep runs only for the duty slice, gliding in and out', () => {
  const r = (t) => sweep(t, { period: 10, duty: 0.2, span: 5 })
  // inside the duty window: progress advances, x sweeps -span → +span
  const a = r(0)
  assert.equal(a.k, 0)
  assert.equal(a.x, -5)
  assert.equal(a.alpha, 0)
  const mid = r(1) // halfway through the 2s duty slice
  assert.equal(mid.k, 0.5)
  assert.equal(mid.x, 0)
  assert.equal(mid.alpha, 1)
  const end = r(1.999)
  assert.ok(end.k > 0.99)
  assert.ok(Math.abs(end.x - 5) < 0.01)
  assert.ok(end.alpha < 0.02)
  // exactly at the duty edge (and beyond): dark — the window is half-open
  assert.equal(r(2), null)
  // and the cycle restarts
  assert.equal(r(10).k, 0)
})

test('breath oscillates around its base with the given amplitude', () => {
  assert.equal(breath(0, 0.5, 0.17, 0.06), 0.17) // sin(0) = 0 → exactly base
  const peak = breath(Math.PI / (2 * 0.5), 0.5, 0.17, 0.06) // quarter cycle → +amp
  assert.ok(Math.abs(peak - 0.23) < 1e-9)
  const trough = breath((3 * Math.PI) / (2 * 0.5), 0.5, 0.17, 0.06) // ¾ cycle → −amp
  assert.ok(Math.abs(trough - 0.11) < 1e-9)
})
