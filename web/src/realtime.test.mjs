// The socket wiring needs a browser, but the backoff curve and the event
// application are pure — those are what these tests pin down.
import test from 'node:test'
import assert from 'node:assert/strict'
import { applyEvent, nextBackoffMs } from './realtime.js'
import {
  activeBoosts,
  banNotice,
  boostColorKey,
  clearSession,
  currentPlayer,
  itemBoosts,
  playerIdentityVersion,
  serverBoosts,
  splitBoosts,
} from './auth.js'

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

test('applyEvent boost replaces the server half and keeps item boosts', () => {
  const exp = { kind: 'exp', multiplier: 2.5, endsAt: '2999-01-01T12:00:00Z' }
  applyEvent('boost', { boosts: { exp } })
  // the payload is the full server-wide view — that half is replaced wholesale
  assert.deepEqual(serverBoosts.value, { exp })
  // a personal item boost rides on top; the bonuses add, never compound
  const itemExp = { kind: 'exp', multiplier: 2, endsAt: '2999-01-02T12:00:00Z' }
  itemBoosts.value = { exp: itemExp }
  assert.equal(activeBoosts.value.exp.multiplier, 3.5)
  assert.deepEqual(activeBoosts.value.exp.sources, [
    { origin: 'server', multiplier: 2.5, endsAt: exp.endsAt },
    { origin: 'item', multiplier: 2, endsAt: itemExp.endsAt },
  ])
  // endsAt points at the next change (the source that expires first)
  assert.equal(activeBoosts.value.exp.endsAt, exp.endsAt)
  // arming a second kind keeps the first; the payload is the full server view
  const coins = { kind: 'coins', multiplier: 3, endsAt: '2999-01-03T12:00:00Z' }
  applyEvent('boost', { boosts: { exp, coins } })
  assert.deepEqual(serverBoosts.value, { exp, coins })
  assert.equal(activeBoosts.value.coins.multiplier, 3)
  // an empty view = every server event ended; the personal boost keeps running
  applyEvent('boost', { boosts: {} })
  assert.deepEqual(serverBoosts.value, {})
  assert.equal(activeBoosts.value.exp.multiplier, 2)
  itemBoosts.value = {} // leave the shared refs clean for the next test run
})

test('splitBoosts tears a player snapshot back into its two halves', () => {
  const snapshot = {
    exp: {
      kind: 'exp',
      multiplier: 3,
      endsAt: '2999-01-04T12:00:00Z',
      sources: [
        { origin: 'server', multiplier: 2, endsAt: '2999-01-01T12:00:00Z' },
        { origin: 'item', multiplier: 2, endsAt: '2999-01-04T12:00:00Z', item: 'exp2', rarity: 'common', name: { en: 'EXP Tonic', th: 'โทนิค EXP' } },
      ],
    },
  }
  const { server, item } = splitBoosts(snapshot)
  assert.deepEqual(server, { exp: { kind: 'exp', multiplier: 2, endsAt: '2999-01-01T12:00:00Z' } })
  assert.deepEqual(item, {
    exp: { kind: 'exp', multiplier: 2, endsAt: '2999-01-04T12:00:00Z', item: 'exp2', rarity: 'common', name: { en: 'EXP Tonic', th: 'โทนิค EXP' } },
  })
  // and a player push lands in both refs through updatePlayer
  currentPlayer.value = { id: 'p1', nickname: 'P' }
  applyEvent('player', { player: { id: 'p1', nickname: 'P', boosts: snapshot } })
  assert.equal(serverBoosts.value.exp.multiplier, 2)
  assert.equal(itemBoosts.value.exp.multiplier, 2)
  serverBoosts.value = {}
  itemBoosts.value = {}
})

test('boostColorKey: server teal unless an item feeds the stack', () => {
  const server = [{ origin: 'server', multiplier: 2, endsAt: '2999-01-01T00:00:00Z' }]
  assert.equal(boostColorKey(server), 'server')
  assert.equal(boostColorKey([]), 'server')
  // an item-driven stack wears the highest rarity among its item sources
  const mixed = [
    ...server,
    { origin: 'item', multiplier: 2, rarity: 'common', endsAt: '2999-01-02T00:00:00Z' },
    { origin: 'item', multiplier: 5, rarity: 'epic', endsAt: '2999-01-03T00:00:00Z' },
  ]
  assert.equal(boostColorKey(mixed), 'epic')
  // item sources without a rarity (catalog since removed) fall back to common
  assert.equal(boostColorKey([{ origin: 'item', multiplier: 2, endsAt: '2999-01-02T00:00:00Z' }]), 'common')
})

test('applyEvent ignores unknown types and malformed payloads', () => {
  currentPlayer.value = { id: 'p1', nickname: 'Kept' }
  applyEvent('stray', {})
  applyEvent('player', {})
  assert.equal(currentPlayer.value.nickname, 'Kept')
})
