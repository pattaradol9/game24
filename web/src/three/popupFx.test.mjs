// Pins the pure burst math behind the popup glint layer.
import test from 'node:test'
import assert from 'node:assert/strict'
import { GLINT_DEFAULTS, advanceSparks, burstSparks } from './popupFx.js'

// deterministic LCG so every run sees the same "random" draw
function seeded(seed) {
  let s = seed
  return () => (s = (s * 16807) % 2147483647) / 2147483647
}

test('burstSparks launches every spark upward inside the fan', () => {
  const b = burstSparks(64, GLINT_DEFAULTS, seeded(7))
  const minRise = GLINT_DEFAULTS.speedMin * Math.cos(GLINT_DEFAULTS.spread / 2)
  for (let i = 0; i < 64; i++) {
    // y-down space: upward means negative vy, never flatter than the fan edge
    assert.ok(b.vy[i] <= -minRise + 1e-9, `spark ${i} rises at only ${b.vy[i]}`)
    assert.ok(Math.hypot(b.vx[i], b.vy[i]) <= GLINT_DEFAULTS.speedMax + 1e-9)
    assert.ok(b.delays[i] >= 0 && b.delays[i] < GLINT_DEFAULTS.maxDelay)
  }
})

test('burstSparks is deterministic for a given rng', () => {
  const a = burstSparks(16, GLINT_DEFAULTS, seeded(42))
  const b = burstSparks(16, GLINT_DEFAULTS, seeded(42))
  assert.deepEqual([...a.vx], [...b.vx])
  assert.deepEqual([...a.vy], [...b.vy])
  assert.deepEqual([...a.delays], [...b.delays])
})

test('advanceSparks bends sparks back down and honours the launch delay', () => {
  const state = {
    positions: new Float32Array(6), // two sparks at the origin
    vx: new Float32Array([10, 0]),
    vy: new Float32Array([-100, -100]),
    delays: new Float32Array([0, 0.5]),
  }
  // a tenth of a second: spark 0 flies up-left of its launch, spark 1 waits
  advanceSparks(state, 0.1, 0.1)
  assert.ok(state.positions[1] < -4, 'spark 0 climbed (y-down space)')
  assert.ok(Math.abs(state.positions[1] + 4.7) < 1, `y = ${state.positions[1]}`)
  assert.ok(state.positions[4] === 0 && state.positions[5] === 0, 'delayed spark stayed put')
  // given enough time gravity wins and the spark falls back downward
  for (let t = 0.2; t <= 4; t += 0.2) advanceSparks(state, 0.2, t)
  assert.ok(state.vy[0] > 0, 'gravity turned the spark around')
  assert.ok(state.positions[1] > 0, 'spark fell below its launch point')
})

test('advanceSparks never moves a spark while its delay holds', () => {
  const state = {
    positions: new Float32Array(3),
    vx: new Float32Array([500]),
    vy: new Float32Array([-500]),
    delays: new Float32Array([1]),
  }
  advanceSparks(state, 0.25, 0.25)
  advanceSparks(state, 0.25, 0.5)
  advanceSparks(state, 0.25, 0.75)
  assert.equal(state.positions[0], 0)
  assert.equal(state.positions[1], 0)
})
