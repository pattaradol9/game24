// Pins the pure layout math behind the profile-halo scene.
import test from 'node:test'
import assert from 'node:assert/strict'
import { HALO_DEFAULTS, advanceSparks, ringPositions, sparkField } from './profileHalo.js'

test('ringPositions lays count points exactly on the circle', () => {
  const pos = ringPositions(48, 2.6)
  assert.equal(pos.length, 48 * 3)
  for (let i = 0; i < 48; i++) {
    const x = pos[i * 3]
    const z = pos[i * 3 + 2]
    assert.equal(pos[i * 3 + 1], 0)
    // Float32 storage (what the GPU reads) allows ~1e-7 slack
    assert.ok(Math.abs(Math.hypot(x, z) - 2.6) < 1e-6, `point ${i} off the circle`)
  }
})

test('sparkField stays inside its box with sane rise speeds', () => {
  const f = sparkField(200, HALO_DEFAULTS.width, HALO_DEFAULTS.height, HALO_DEFAULTS.depth)
  assert.equal(f.positions.length, 200 * 3)
  for (let i = 0; i < 200; i++) {
    assert.ok(Math.abs(f.positions[i * 3]) <= HALO_DEFAULTS.width / 2 + 1e-9)
    assert.ok(f.positions[i * 3 + 1] >= 0 && f.positions[i * 3 + 1] < HALO_DEFAULTS.height)
    assert.ok(Math.abs(f.positions[i * 3 + 2]) <= HALO_DEFAULTS.depth / 2 + 1e-9)
    assert.ok(f.speeds[i] >= 0.15 && f.speeds[i] <= 0.5)
    assert.ok(f.phases[i] >= 0 && f.phases[i] < Math.PI * 2)
  }
})

test('advanceSparks rises with sway and wraps at the ceiling', () => {
  const f = sparkField(1, 4, 1, 1, () => 0.5)
  const x0 = f.positions[0]
  const y0 = f.positions[1]
  // one second of rise moves the spark up by its speed and sways x by ≤0.35
  advanceSparks(f, 1, 0)
  assert.ok(Math.abs(f.positions[1] - (y0 + f.speeds[0])) < 1e-9)
  assert.ok(Math.abs(f.positions[0] - x0) <= 0.35 + 1e-9)
  // pushing past the ceiling wraps back near the floor
  f.positions[1] = 0.999
  advanceSparks(f, 1, 0)
  assert.ok(f.positions[1] < 1, `y = ${f.positions[1]} should have wrapped below the ceiling`)
  assert.ok(f.positions[1] >= 0)
})
