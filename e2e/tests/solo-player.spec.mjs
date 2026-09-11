// Solo play as a signed-in Google player: item helpers, the 0:00 extend
// offer, EXP banking into the live profile and account deletion.
import { test, expect } from '../helpers/fixtures.mjs'
import {
  apiGoogleSignIn,
  establishSession,
  seedItem,
  fetchMe,
  winHand,
  waitBoardReady,
  readClock,
  letCelebrationPass,
} from '../helpers/harness.mjs'
import { stageCredential } from '../helpers/fake-google.mjs'
import { unique } from '../helpers/harness.mjs'

async function newPlayerPage(page, { items = {} } = {}) {
  const name = unique('Google-solo')
  const { token, player } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name })
  for (const [itemId, qty] of Object.entries(items)) seedItem(player.id, itemId, qty)
  await establishSession(page, token)
  return { token, player, name }
}

test.describe('Solo (Google player)', () => {
  test('add-time spends one Time Extension and tops the clock to the mode limit', async ({ page }) => {
    // two units: one gets spent, the second keeps the bag non-empty so the
    // dark button explains the per-session rule (not an empty bag)
    const { token } = await newPlayerPage(page, { items: { time30: 2 } })
    await page.goto('/solo?mode=jack')
    await waitBoardReady(page)
    // let a little time drain, then spend the item mid-play
    await expect
      .poll(() => readClock(page), { timeout: 30_000 })
      .toBeLessThan(120)
    await page.locator('[data-test="add-time"]').click()
    // clamped: a hand never runs past the mode's base window (the display
    // ticks once a second, so poll rather than read once)
    await expect.poll(() => readClock(page), { timeout: 5_000 }).toBeGreaterThanOrEqual(118)
    expect(await readClock(page)).toBeLessThanOrEqual(120)
    await expect(page.locator('.toast')).toContainText('ต่อเวลาอีก')
    // one extension per session: the button is dark now and explains itself
    await page.locator('.helper', { has: page.locator('[data-test="add-time"]') }).click()
    await expect(page.locator('.why').filter({ hasText: 'ต่อเวลาได้ 1 ครั้งต่อ' })).toBeVisible()
    // the bag went down by one
    const me = await fetchMe(page.request, token)
    expect((me.items ?? []).find((i) => i.id === 'time30')?.qty ?? 0).toBe(1)
  })

  test('a drained clock offers the Time Extension before folding', async ({ page }) => {
    test.setTimeout(260_000)
    await newPlayerPage(page, { items: { time30: 1 } })
    await page.goto('/solo?mode=ace') // 60s window
    await waitBoardReady(page)
    const modal = page.locator('.overlay .panel.box').filter({ hasText: 'ต่อเวลาไหม?' })
    await expect(modal).toBeVisible({ timeout: 90_000 })
    await expect(page.locator('.clock .mmss')).toHaveText('0:00')
    await modal.locator('[data-test="confirm-extend"]').click()
    // +30 fresh seconds on this hand (the countdown re-arms within a tick)
    await expect.poll(() => readClock(page), { timeout: 5_000 }).toBe(30)
    await expect(page.locator('.toast')).toContainText('ต่อเวลาอีก 30 วินาที!')
    // when the borrowed seconds run out, the plain fold takes over
    const result = page.locator('.overlay .panel.modal')
    await expect(result.locator('.verdict.lose')).toHaveText('หมดเวลา!', { timeout: 60_000 })
  })

  test('skip spends one Skip Pass and is spent for the rest of the session', async ({ page }) => {
    // two units: after the fold the bag still holds one, so the dark button
    // explains the per-session rule rather than an empty bag
    await newPlayerPage(page, { items: { skip1: 2 } })
    await page.goto('/solo?mode=queen')
    await waitBoardReady(page)
    await page.locator('[data-test="skip-hand"]').click()
    const modal = page.locator('.overlay .panel.modal')
    await expect(modal.locator('.verdict.lose')).toBeVisible()
    await expect(modal.locator('.solution code')).not.toBeEmpty()
    await modal.getByRole('button', { name: 'เริ่มเล่นใหม่' }).click()
    await waitBoardReady(page)
    // one fold per play session: the button explains itself from now on
    await page.locator('.helper', { has: page.locator('[data-test="skip-hand"]') }).click()
    await expect(page.locator('.why').filter({ hasText: 'ข้ามได้ 1 ครั้งต่อ' })).toBeVisible()
  })

  test('solving banks EXP into the live level bar', async ({ page }) => {
    const { token } = await newPlayerPage(page)
    await page.goto('/solo?mode=queen')
    await waitBoardReady(page)
    const parseCount = async () => {
      const raw = await page.locator('.xp-panel .xp .count').textContent()
      const [into] = raw.trim().split('/').map((p) => parseInt(p, 10))
      return into
    }
    const before = await parseCount()
    await winHand(page)
    await letCelebrationPass(page)
    await expect.poll(parseCount).toBeGreaterThan(before)
  })

  test('account deletion requires the exact word and clears the session', async ({ page }) => {
    const name = unique('Google-delete')
    await page.goto('/')
    await stageCredential(page, { sub: unique('sub'), name })
    await page.locator('.gsi-box .g-hit button').click()
    await expect(page.locator('button.profile-btn')).toContainText(name)
    await page.locator('button.profile-btn').click()
    await page.locator('.profile .menu').getByText('ลบบัญชีถาวร', { exact: true }).click()
    const modal = page.locator('.overlay .panel')
    await expect(modal.locator('h3')).toHaveText('ลบบัญชีของคุณถาวร?')
    const confirm = modal.locator('#del-confirm')
    const del = modal.getByRole('button', { name: 'ลบบัญชีถาวร' })
    await confirm.fill('DELETE')
    await expect(del).toBeEnabled()
    await confirm.fill('DELETEX')
    await expect(del).toBeDisabled()
    await confirm.fill('DELETE')
    await del.click()
    // back to the entry gate — the account (and token) is gone
    await expect(page.locator('.overlay .panel.modal h2')).toHaveText('ตั้งชื่อเล่นของคุณ')
    await page.reload()
    await expect(page.locator('.overlay .panel.modal h2')).toHaveText('ตั้งชื่อเล่นของคุณ')
  })
})
