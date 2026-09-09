// The boost timers are pure string shaping — that is what these pin down.
import test from 'node:test'
import assert from 'node:assert/strict'
import { fmtDuration } from './duration.js'

test('fmtDuration shows whole hours while any remain', () => {
  assert.equal(fmtDuration(24 * 3600), '24h')
  assert.equal(fmtDuration(23 * 3600 + 59 * 60 + 59), '23h')
  assert.equal(fmtDuration(3600), '1h')
})

test('fmtDuration drops to minutes below the hour', () => {
  assert.equal(fmtDuration(3599), '59m')
  assert.equal(fmtDuration(90), '1m')
  assert.equal(fmtDuration(60), '1m')
})

test('fmtDuration drops to seconds below the minute', () => {
  assert.equal(fmtDuration(59), '59s')
  assert.equal(fmtDuration(1), '1s')
})

test('fmtDuration clamps expired and negative spans to zero', () => {
  assert.equal(fmtDuration(0), '0s')
  assert.equal(fmtDuration(-5), '0s')
})
