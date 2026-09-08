// The socket wiring needs a browser, but the backoff curve and the event
// application are pure — those are what these tests pin down.
import test from 'node:test'
import assert from 'node:assert/strict'
import { applyEvent, nextBackoffMs } from './realtime.js'
import { activeBoosts, banNotice, clearSession, currentPlayer, playerIdentityVersion } from './auth.js'

// auth.js touches localStorage inside clearSession; node tests have none
globalThis.localStorage ??= {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {},
}

test('nextBackoffMs grows exponentially, stays within the cap and jitters', () => {
  // full jitter: half to 1.5x the exponential step
  assert.equal(nextBackoffMs(0, () => 0), 500)
  assert.equal(nextBackoffMs(0, () => 1), 1500)
  assert.equal(nextBackoffMs(3, () => 0.5), 8000)
  // capped at 30s
  assert.equal(nextBackoffMs(20, () => 1), 30000)
})

test('applyEvent player updates the shared player state', () => {
  currentPlayer.value = { id: 'p1', nickname: 'Old', totalCoins: 5 }
  applyEvent('player', { player: { id: 'p1', nickname: 'Old', totalCoins: 777 } })
  assert.equal(currentPlayer.value.totalCoins, 777)
})

test('applyEvent player bumps the identity version on a rename', () => {
  currentPlayer.value = { id: 'p1', nickname: 'Old' }
  const before = playerIdentityVersion.value
  applyEvent('player', { player: { id: 'p1', nickname: 'New' } })
  assert.equal(currentPlayer.value.nickname, 'New')
  assert.equal(playerIdentityVersion.value, before + 1)
  // other changes do not invalidate cached nicknames
  applyEvent('player', { player: { id: 'p1', nickname: 'New', totalCoins: 1 } })
  assert.equal(playerIdentityVersion.value, before + 1)
})

test('applyEvent deleted clears the session', () => {
  currentPlayer.value = { id: 'p1', nickname: 'Doomed' }
  const before = playerIdentityVersion.value
  applyEvent('deleted', {})
  assert.equal(currentPlayer.value, null)
  assert.equal(playerIdentityVersion.value, before + 1)
  clearSession() // leave the shared refs clean for the next test run
})

test('applyEvent banned raises the notice and clears the session', () => {
  currentPlayer.value = { id: 'p1', nickname: 'Outlaw' }
  const before = playerIdentityVersion.value
  applyEvent('banned', { reason: 'cheating' })
  assert.deepEqual(banNotice.value, { reason: 'cheating' })
  assert.equal(currentPlayer.value, null)
  assert.equal(playerIdentityVersion.value, before + 1)
  banNotice.value = null
})

test('applyEvent error/account banned acts like a live ban push', () => {
  // the reconnect auth of a socket that missed the ban push gets this
  currentPlayer.value = { id: 'p1', nickname: 'Outlaw' }
  applyEvent('error', { message: 'account banned' })
  assert.deepEqual(banNotice.value, { reason: '' })
  assert.equal(currentPlayer.value, null)
  banNotice.value = null
  // any other error message leaves the session alone
  currentPlayer.value = { id: 'p1', nickname: 'Kept' }
  applyEvent('error', { message: 'sign in required' })
  assert.equal(banNotice.value, null)
  assert.equal(currentPlayer.value.nickname, 'Kept')
})

test('applyEvent boost replaces the shared active-boost view', () => {
  const exp = { kind: 'exp', multiplier: 2.5, endsAt: '2026-09-10T12:00:00Z' }
  applyEvent('boost', { boosts: { exp } })
  // the ref wraps objects in a reactive proxy — compare by structure
  assert.deepEqual(activeBoosts.value, { exp })
  // arming a second kind keeps the first; the payload is the full view
  const coins = { kind: 'coins', multiplier: 3, endsAt: '2026-09-11T12:00:00Z' }
  applyEvent('boost', { boosts: { exp, coins } })
  assert.deepEqual(activeBoosts.value, { exp, coins })
  // an empty view = every event ended
  applyEvent('boost', { boosts: {} })
  assert.deepEqual(activeBoosts.value, {})
  activeBoosts.value = {} // leave the shared refs clean for the next test run
})

test('applyEvent ignores unknown types and malformed payloads', () => {
  currentPlayer.value = { id: 'p1', nickname: 'Kept' }
  applyEvent('stray', {})
  applyEvent('player', {})
  assert.equal(currentPlayer.value.nickname, 'Kept')
})
