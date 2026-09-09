// The atelier scene's shape lives in pure helpers — these pin the card field
// layout and its integration, so the ambience can be re-tuned without
// silently breaking it.
import test from 'node:test'
import assert from 'node:assert/strict'
import { advanceCards, cardField, CARD_TINTS } from './skinShopFx.js'

// deterministic rng so layout assertions are exact
function lcg(seed) {
  let s = seed
  return () => {
    s = (s * 1664525 + 1013904223) % 4294967296
    return s / 4294967296
  }
}

test('cardField scatters cards inside the pane with sane specs', () => {
  const w0 = { x: 6, y: 3 }
  const cards = cardField(12, w0, 4, lcg(7))
  assert.equal(cards.length, 12)
  for (const c of cards) {
    assert.ok(Math.abs(c.baseX) <= w0.x + 0.4 + 1e-9, `baseX ${c.baseX} outside the pane`)
    assert.ok(Math.abs(c.y) <= w0.y + 0.6 + 1e-9, `y ${c.y} outside the pane`)
    assert.ok(c.z <= -4 * 0.55 - 1e-9 && c.z >= -4 - 1e-9, `z ${c.z} outside the far slab`)
    assert.ok(c.vy >= 0.1 && c.vy <= 0.3, `vy ${c.vy} out of range`)
    assert.ok(c.opacity >= 0.1 && c.opacity <= 0.24, `opacity ${c.opacity} out of range`)
    assert.ok(c.tint >= 0 && c.tint < CARD_TINTS.length)
  }
})

test('advanceCards makes cards fall, sway around their column and tumble', () => {
  const w0 = { x: 6, y: 3 }
  const cards = cardField(3, w0, 4, lcg(11))
  const before = cards.map((c) => ({ ...c }))
  advanceCards(cards, 1, 5, w0)
  for (let i = 0; i < cards.length; i++) {
    const c = cards[i]
    const b = before[i]
    // falling: y moved down by exactly vy
    assert.ok(Math.abs(c.y - (b.y - c.vy)) < 1e-9)
    // sway: x hugs its base column formula, not a random walk
    assert.ok(Math.abs(c.x - (b.baseX + Math.sin(5 * c.swayF + c.phase) * c.sway)) < 1e-9)
    // tumbling: every axis advanced by its own velocity
    assert.ok(Math.abs(c.rx - (b.rx + c.vrx)) < 1e-9)
    assert.ok(Math.abs(c.ry - (b.ry + c.vry)) < 1e-9)
    assert.ok(Math.abs(c.rz - (b.rz + c.vrz)) < 1e-9)
  }
})

test('advanceCards wraps cards that fall past the floor back to the ceiling', () => {
  const w0 = { x: 6, y: 3 }
  const cards = cardField(2, w0, 4, lcg(3))
  // park a card just above the floor and let enough time pass
  cards[0].y = -w0.y - 0.7
  cards[0].vy = 0.2
  advanceCards(cards, 1, 0, w0)
  assert.equal(cards[0].y, w0.y + 0.8)
  // determinism: the same seed builds the same field
  const again = cardField(3, w0, 4, lcg(11))
  const first = cardField(3, w0, 4, lcg(11))
  assert.deepEqual(again, first)
})
