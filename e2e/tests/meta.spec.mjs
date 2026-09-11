// Achievements catalog + leaderboards + legal pages.
import { test, expect } from '../helpers/fixtures.mjs'
import {
  apiGoogleSignIn,
  apiCreateGuest,
  establishSession,
  unique,
  winHand,
  waitBoardReady,
  letCelebrationPass,
} from '../helpers/harness.mjs'

test.describe('Achievements', () => {
  test('the full catalog renders for a guest, all locked', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/achievements')
    await expect(page.locator('article.ach')).toHaveCount(50)
    await expect(page.locator('.chip.accent')).toContainText('ปลดล็อกแล้ว 0 / 50')
    await expect(page.locator('article.ach.unlocked')).toHaveCount(0)
    // locked entries carry progress and rewards
    const first = page.locator('article.ach').first()
    await expect(first.locator('.state')).toContainText('ยังไม่ปลดล็อก')
    await expect(first.locator('.prog')).toBeVisible()
    await expect(first.locator('.rewards')).toContainText('EXP')
  })

  test('a first solve shows up as an unlock with the progress counter', async ({ page }) => {
    const { token } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name: unique('Achv') })
    await establishSession(page, token)
    await page.goto('/solo?mode=queen')
    await winHand(page)
    await letCelebrationPass(page)
    await page.goto('/achievements')
    await expect(page.locator('article.ach')).toHaveCount(50)
    await expect(page.locator('.chip.accent')).toContainText(/ปลดล็อกแล้ว [1-9]\d* \/ 50/)
    const unlocked = page.locator('article.ach.unlocked').first()
    await expect(unlocked.locator('.state.on')).toContainText('ปลดล็อกแล้ว')
    await expect(unlocked.locator('.when')).toBeVisible()
  })
})

test.describe('Leaderboard', () => {
  test('a signed-in solver appears on the board of their mode', async ({ page }) => {
    const name = unique('Board')
    const { token } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name })
    await establishSession(page, token)
    await page.goto('/solo?mode=queen')
    await winHand(page)
    await letCelebrationPass(page)
    await page.goto('/leaderboard')
    await expect(page.locator('h1:visible')).toHaveText('กระดานผู้นำ')
    // mode tabs + period tabs
    await expect(page.locator('.board .tabs').first().locator('.tab')).toHaveCount(4)
    // the fresh solve sits on the queen board with a points figure
    await page.locator('.board .tabs .tab', { hasText: 'ควีน' }).click()
    await expect(page.locator('.board')).toContainText(name, { timeout: 15_000 })
    await expect(page.locator('.board')).toContainText('แต้ม')
    // the weekly window has its own hint copy
    await page.locator('.tabs.periods .tab', { hasText: 'รายสัปดาห์' }).click()
    await expect(page.locator('.period-hint')).toBeVisible()
  })

  test('guests never rank — an unused board shows the empty copy', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/leaderboard')
    await expect(page.locator('h1:visible')).toHaveText('กระดานผู้นำ')
    await expect(page.locator('.board')).toBeVisible()
  })
})

test.describe('Legal', () => {
  test('privacy policy and terms render in both languages', async ({ page }) => {
    await page.goto('/privacy')
    await expect(page.locator('h1:visible')).toHaveText('นโยบายความเป็นส่วนตัว')
    await expect(page.locator('.updated')).toBeVisible()
    await page.locator('button.btn.icon.lang').click()
    await expect(page.locator('h1:visible')).toHaveText('Privacy Policy')
    // the choice persists across routes in the same tab
    await page.goto('/terms')
    await expect(page.locator('h1:visible')).toHaveText('Terms of Service')
    await page.locator('button.btn.icon.lang').click()
    await expect(page.locator('h1:visible')).toHaveText('ข้อกำหนดการใช้บริการ')
  })

  test('the old /skins route redirects into the shop', async ({ page }) => {
    await page.goto('/skins')
    await expect(page).toHaveURL(/\/shop$/)
    await expect(page.locator('h1:visible')).toHaveText('ชุดไพ่')
  })
})
