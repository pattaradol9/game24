// The solo ladder's client half is display placement only — the server owns
// the numbers. These pin the rung math and the drop readout.
import test from 'node:test'
import assert from 'node:assert/strict'
import { LADDER_STEP, LADDER_MIN_TIME, ladderStageFor, ladderDropPct } from './modes.js'

test('ladderStageFor climbs one rung per LADDER_STEP hands', () => {
  assert.equal(ladderStageFor(1), 0)
  assert.equal(ladderStageFor(LADDER_STEP), 0)
  assert.equal(ladderStageFor(LADDER_STEP + 1), 1)
  assert.equal(ladderStageFor(2 * LADDER_STEP), 1)
  assert.equal(ladderStageFor(141), 14)
})

test('ladderStageFor clamps nonsense hand numbers to stage 0', () => {
  assert.equal(ladderStageFor(0), 0)
  assert.equal(ladderStageFor(-3), 0)
})

test('ladderDropPct reads the countdown squeeze off the run base', () => {
  assert.equal(ladderDropPct(120, 120), 0)
  assert.equal(ladderDropPct(108, 120), 10)
  assert.equal(ladderDropPct(97, 120), 19)
  // the 30s floor caps a jack run's squeeze at 75%
  assert.equal(ladderDropPct(LADDER_MIN_TIME, 120), 75)
})

test('ladderDropPct stays silent without a base or a drop', () => {
  assert.equal(ladderDropPct(90, 0), 0)
  assert.equal(ladderDropPct(0, 120), 0)
  assert.equal(ladderDropPct(120, 90), 0)
})
