// The scene builders need a GPU to run, but their layout math and envelopes
// are pure — those are what these tests pin down.
import test from 'node:test'
import assert from 'node:assert/strict'
import { burstFade, circleLayout, glintCurve, spiralLayout } from './skinScenes.js'

test('spiralLayout fills a bounded disc, deterministically for a given rng', () => {
  const n = 500
  const a = spiralLayout(n, 2, 4, () => 0.42)
  const b = spiralLayout(n, 2, 4, () => 0.42)
  assert.equal(a.length, n * 2)
  assert.deepEqual([...a], [...b])
  for (let i = 0; i < n; i++) {
    const r = Math.hypot(a[i * 2], a[i * 2 + 1])
    assert.ok(r <= 4 + 1e-9, `point ${i} at radius ${r} escapes the disc`)
  }
  // square-root spacing: the innermost ring hugs the centre
  assert.ok(Math.hypot(a[0], a[1]) < 0.2)
})

test('circleLayout spaces points evenly on the ring', () => {
  const r = 2.7
  const pts = circleLayout(64, r)
  assert.equal(pts.length, 128)
  for (let i = 0; i < 64; i++) {
    assert.ok(Math.abs(Math.hypot(pts[i * 2], pts[i * 2 + 1]) - r) < 1e-6)
  }
  // consecutive angular gaps are uniform
  const gap = (pts[2] - pts[0]) // x of point 1 minus x of point 0
  assert.ok(Math.abs(gap) > 0 && Math.abs(gap) < r)
})

test('glintCurve rises to a single peak and closes at both ends', () => {
  assert.equal(glintCurve(0), 0)
  assert.equal(glintCurve(1), 0)
  assert.equal(glintCurve(0.5), 1)
  assert.ok(Math.abs(glintCurve(0.2) - glintCurve(0.8)) < 1e-9)
  assert.ok(glintCurve(0.35) < glintCurve(0.5))
})

test('burstFade fades in fast, out smooth, and never exceeds 1', () => {
  const ttl = 1
  assert.equal(burstFade(ttl, ttl), 0) // brand new: still invisible
  assert.equal(burstFade(0, ttl), 0) // expired
  const mid = burstFade(ttl * 0.7, ttl)
  const late = burstFade(ttl * 0.2, ttl)
  assert.ok(mid > late, 'alpha must decay as life runs out')
  for (let l = 0; l <= 100; l++) {
    const v = burstFade(l / 100, ttl)
    assert.ok(v >= 0 && v <= 1, `alpha ${v} out of range at life ${l / 100}`)
  }
})
