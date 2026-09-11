// Test harness: session bootstrap, DB seeding, and board-driving helpers
// shared by every spec.
import { DatabaseSync } from 'node:sqlite'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect } from './fixtures.mjs'
import { mintToken, GSI_STUB, blockGoogleScript } from './fake-google.mjs'
import { solve24, solveNon24, OP_ARIA } from './solver.mjs'

const helpersDir = path.dirname(fileURLToPath(import.meta.url))
const e2eDir = path.resolve(helpersDir, '..')
export const DB_PATH = path.join(e2eDir, '.tmp', 'game24-e2e.db')
// the isolated API instance the deps orchestrator starts
export const API = process.env.G24_E2E_API || 'http://127.0.0.1:18080/api/v1'

let counter = 0

/** A unique, stable-per-run nickname/sub so tests never collide. */
export function unique(prefix) {
  counter += 1
  return `${prefix}-${Date.now().toString(36)}-${counter}`
}

// ---------------------------------------------------------------------------
// sessions

// every API response rides in a { success, message, data } envelope
function unwrap(json) {
  if (!json?.success) throw new Error(json?.message || 'api error')
  return json.data
}

/** Create a guest through the real API; returns {token, player}. */
export async function apiCreateGuest(request, nickname = unique('guest')) {
  const res = await request.post(`${API}/players`, { data: { nickname } })
  expect(res.ok(), `create guest: ${res.status()}`).toBeTruthy()
  return unwrap(await res.json())
}

/** Create (or re-login) a Google player through the real API. */
export async function apiGoogleSignIn(request, { sub = unique('sub'), email, name = unique('Player') } = {}) {
  const credential = await mintToken({ sub, email: email ?? `${sub}@e2e.game24.test`, name })
  const res = await request.post(`${API}/auth/google`, { data: { credential } })
  expect(res.ok(), `google sign-in: ${res.status()}`).toBeTruthy()
  return unwrap(await res.json())
}

/** Plant a session token before navigation — restoreSession picks it up. */
export async function establishSession(page, token) {
  await page.addInitScript((t) => {
    try { localStorage.setItem('g24_token', t) } catch { /* ignore */ }
  }, token)
}

export async function fetchMe(request, token) {
  const res = await request.get(`${API}/me`, { headers: { Authorization: `Bearer ${token}` } })
  expect(res.ok(), `GET /me: ${res.status()}`).toBeTruthy()
  return unwrap(await res.json()).player
}

// ---------------------------------------------------------------------------
// direct DB seeding (coins / items / bans) — never through the admin API

function withDb(fn) {
  const db = new DatabaseSync(DB_PATH)
  try {
    for (let attempt = 0; ; attempt++) {
      try {
        return fn(db)
      } catch (err) {
        if (attempt >= 5 || !String(err).includes('SQLITE_BUSY')) throw err
        // the server holds WAL locks from time to time — back off briefly
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 200)
      }
    }
  } finally {
    db.close()
  }
}

export function seedCoins(playerId, coins) {
  withDb((db) => db.prepare('UPDATE players SET total_coins = ? WHERE id = ?').run(coins, playerId))
}

export function seedItem(playerId, itemId, qty) {
  withDb((db) =>
    db
      .prepare(`INSERT INTO player_items (player_id, item_id, qty) VALUES (?, ?, ?)
                ON CONFLICT(player_id, item_id) DO UPDATE SET qty = excluded.qty`)
      .run(playerId, itemId, qty)
  )
}

export function banPlayer(playerId, reason = 'e2e ban') {
  withDb((db) => db.prepare('UPDATE players SET banned = 1, ban_reason = ? WHERE id = ?').run(reason, playerId))
}

// ---------------------------------------------------------------------------
// board driving

/** Original dealt numbers, in board order (card ids c0..c3). */
export async function readNumbers(page) {
  const raw = await page.locator('.cards .slot[data-slot^="c"] .card .rank').allTextContents()
  return raw.map((t) => parseInt(t.trim(), 10))
}

/** The hand is interactable: four cards present, intro countdown gone. */
export async function waitBoardReady(page) {
  await expect(page.locator('.cards .slot[data-slot^="c"]')).toHaveCount(4)
  await expect(page.locator('.countdown')).toHaveCount(0)
}

/**
 * Play the current hand through real clicks. Reads the dealt numbers, solves
 * for `target` and merges accordingly; on target 24 the app auto-submits.
 * `numbers` may be passed to skip the DOM read.
 */
export async function playHand(page, target = 24, numbers = null) {
  const nums = numbers ?? (await readNumbers(page))
  const steps = solve24(nums, target)
  expect(steps, `no merge plan reaches ${target} for [${nums}]`).not.toBeNull()
  for (let k = 0; k < steps.length; k++) {
    const { a, op, b } = steps[k]
    await page.click(`[data-card-id="${a}"]`)
    await page.click(`.pad button.op[aria-label="${OP_ARIA[op]}"]`)
    await page.click(`[data-card-id="${b}"]`)
    if (k < steps.length - 1) {
      await expect(page.locator(`[data-card-id="s${k}"]`)).toBeVisible()
    }
  }
  return { nums, steps }
}

/** Convenience: wait for the board, then win the hand. */
export async function winHand(page, numbers = null) {
  await waitBoardReady(page)
  return playHand(page, 24, numbers)
}

/** Parse the solo clock text (`M:SS`) into seconds. */
export function clockSeconds(text) {
  const [m, s] = text.trim().split(':').map((p) => parseInt(p, 10))
  return m * 60 + s
}

export async function readClock(page) {
  return clockSeconds(await page.locator('.clock .mmss').textContent())
}

/** Dismiss any celebration banner blocking the UI (self-dismisses anyway). */
export async function letCelebrationPass(page) {
  const cele = page.locator('.cele')
  if (await cele.count()) {
    await cele.waitFor({ state: 'detached', timeout: 10_000 }).catch(() => {})
  }
}

// ---------------------------------------------------------------------------
// extra browser contexts (multiplayer): a guest session on a fresh page with
// the same stubs the shared fixtures apply

/** A fresh context+page signed in as a unique guest. Caller closes the ctx. */
export async function guestPage(browser, { viewport = { width: 1280, height: 800 } } = {}) {
  const motion = process.env.G24_E2E_MOTION ? 'no-preference' : 'reduce'
  const ctx = await browser.newContext({ viewport, reducedMotion: motion, locale: 'th-TH' })
  await blockGoogleScript(ctx)
  const page = await ctx.newPage()
  await page.addInitScript(GSI_STUB)
  await page.addInitScript(() => {
    try { localStorage.setItem('g24_sound', 'off') } catch { /* ignore */ }
  })
  const { token, player } = await apiCreateGuest(ctx.request)
  await establishSession(page, token)
  return { ctx, page, token, player }
}
