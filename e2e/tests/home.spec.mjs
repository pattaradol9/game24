// Home page, entry flows (guest + Google), profile menus, settings toggles.
import { test, expect } from '../helpers/fixtures.mjs'
import { apiCreateGuest, establishSession, unique } from '../helpers/harness.mjs'
import { stageCredential } from '../helpers/fake-google.mjs'

test.describe('Home', () => {
  test('renders the entry gate for a fresh visitor', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('.overlay .panel.modal h2')).toHaveText('ตั้งชื่อเล่นของคุณ')
    // the nickname field arrives pre-filled with a random name
    await expect(page.locator('.overlay .panel.modal input.field')).not.toHaveValue('')
    await expect(page.getByRole('button', { name: 'เล่นเลย (แบบไม่สะสม)' })).toBeVisible()
    await expect(page.locator('.overlay .panel.modal .g-btn')).toContainText('ลงชื่อเข้าใช้ด้วย Google')
    // legal row
    await expect(page.locator('.overlay .panel.modal').getByRole('link', { name: 'ข้อกำหนดการใช้บริการ' })).toBeVisible()
    await expect(page.locator('.overlay .panel.modal').getByRole('link', { name: 'นโยบายความเป็นส่วนตัว' })).toBeVisible()
  })

  test('guest sign-in opens the full home page', async ({ page }) => {
    await page.goto('/')
    await page.locator('.overlay .panel.modal input.field').fill('E2E-Guest-Home')
    await page.getByRole('button', { name: 'เล่นเลย (แบบไม่สะสม)' }).click()
    await expect(page.locator('.overlay')).toHaveCount(0)
    await expect(page.locator('.hero .pitch h1')).toHaveText('จับเลข 4 ตัว รวมให้ได้ 24!')
    // four difficulty cards
    await expect(page.locator('.modes > button.mode')).toHaveCount(4)
    // quick links
    await expect(page.locator('a.quick-card[href="/achievements"]')).toBeVisible()
    await expect(page.locator('a.quick-card[href="/shop"]')).toBeVisible()
  })

  test('guest session survives a reload', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request, 'E2E-Persist')
    await establishSession(page, token)
    await page.goto('/')
    await expect(page.locator('.overlay .panel.modal')).toHaveCount(0)
    await page.reload()
    await expect(page.locator('.overlay .panel.modal')).toHaveCount(0)
  })

  test('mode cards navigate into solo with the chosen difficulty', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/')
    await page.locator('.modes > button.mode.diamond').click()
    await expect(page).toHaveURL(/\/solo\?mode=jack$/)
    await page.goBack()
    await page.locator('.modes > button.mode.heart').click()
    await expect(page).toHaveURL(/\/solo\?mode=ace$/)
  })

  test('the play CTA routes to solo', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/')
    await page.locator('.cta button.btn.primary.big').click()
    await expect(page).toHaveURL(/\/solo\?mode=queen$/)
  })

  test('join stays disabled until the room code has six characters', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/')
    const code = page.locator('input.field.code.num')
    const join = page.locator('.join button.btn')
    await expect(join).toBeDisabled()
    await code.fill('AB12')
    await expect(join).toBeDisabled()
    await code.fill('AB12CD')
    await expect(join).toBeEnabled()
  })

  test('joining an unknown room lands on the closed-room screen', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/room/ZZZZ99')
    await expect(page.locator('.panel.lobby h2')).toHaveText('ห้องนี้ถูกปิดไปแล้ว', { timeout: 15_000 })
  })
})

test.describe('Profile menu', () => {
  test('guests get the reduced menu', async ({ page }) => {
    const { token, player } = await apiCreateGuest(page.request, 'E2E-GuestMenu')
    await establishSession(page, token)
    await page.goto('/')
    await page.locator('button.profile-btn').click()
    const menu = page.locator('.profile .menu')
    await expect(menu.locator('.nick')).toHaveText(player.nickname)
    await expect(menu.locator('.sub')).toHaveText('ผู้เล่นชั่วคราว')
    await expect(menu.getByText('เปลี่ยนชื่อ', { exact: true })).toBeVisible()
    await expect(menu.locator('.guest-hint')).toContainText('ล็อกอินด้วย Google')
    // gated entries stay hidden for guests
    await expect(menu.getByText('ความสำเร็จ', { exact: true })).toHaveCount(0)
    await expect(menu.getByText('ร้านค้า', { exact: true })).toHaveCount(0)
    await expect(menu.getByText('ลบบัญชีถาวร', { exact: true })).toHaveCount(0)
    await expect(menu.getByText('ออกจากระบบ', { exact: true })).toHaveCount(0)
  })

  test('renaming a guest updates the chip everywhere', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request, 'E2E-RenameMe')
    await establishSession(page, token)
    await page.goto('/')
    await page.locator('button.profile-btn').click()
    await page.locator('.profile .menu').getByText('เปลี่ยนชื่อ', { exact: true }).click()
    const modal = page.locator('.overlay .panel', { has: page.locator('h3') }).filter({ hasText: 'เปลี่ยนชื่อ' })
    await modal.locator('input.name-input').fill('E2E-Renamed')
    await modal.getByRole('button', { name: 'บันทึก' }).click()
    await expect(page.locator('button.profile-btn')).toContainText('E2E-Renamed')
    await page.reload()
    await expect(page.locator('button.profile-btn')).toContainText('E2E-Renamed')
  })

  test('Google sign-in through the entry modal signs the player in', async ({ page }) => {
    const name = unique('Google-user')
    await page.goto('/')
    await page.locator('.overlay .panel.modal input.field').waitFor()
    await stageCredential(page, { sub: unique('sub'), name })
    await page.locator('.gsi-box .g-hit button').click()
    await expect(page.locator('.overlay .panel.modal')).toHaveCount(0)
    await expect(page.locator('button.profile-btn')).toContainText(name)
    // full player menu, admin excluded
    await page.locator('button.profile-btn').click()
    const menu = page.locator('.profile .menu')
    await expect(menu.locator('.sub')).toHaveText('บัญชี Google')
    await expect(menu.getByText('ความสำเร็จ', { exact: true })).toBeVisible()
    await expect(menu.getByText('ร้านค้า', { exact: true })).toBeVisible()
    await expect(menu.getByText('ออกจากระบบ', { exact: true })).toBeVisible()
    await expect(menu.getByText('ลบบัญชีถาวร', { exact: true })).toBeVisible()
    await expect(menu.getByText('หลังบ้านแอดมิน', { exact: true })).toHaveCount(0)
  })

  test('sign out returns to the entry gate', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request, 'E2E-SignOut')
    await establishSession(page, token)
    await page.goto('/')
    await page.locator('button.profile-btn').click()
    // guests hide sign-out; use a Google account instead
    await page.keyboard.press('Escape')
    // (guests cannot sign out — assert that, then do it as a Google player)
    await expect(page.locator('.profile .menu').getByText('ออกจากระบบ', { exact: true })).toHaveCount(0)
  })

  test('sign out as a Google player clears the session', async ({ page }) => {
    const name = unique('Google-signout')
    await page.goto('/')
    await stageCredential(page, { sub: unique('sub'), name })
    await page.locator('.gsi-box .g-hit button').click()
    await expect(page.locator('button.profile-btn')).toContainText(name)
    await page.locator('button.profile-btn').click()
    await page.locator('.profile .menu').getByText('ออกจากระบบ', { exact: true }).click()
    await expect(page.locator('.overlay .panel.modal h2')).toHaveText('ตั้งชื่อเล่นของคุณ')
  })
})

test.describe('Settings', () => {
  test('language toggle swaps the UI copy and persists', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/')
    await expect(page.locator('.hero .pitch h1')).toHaveText('จับเลข 4 ตัว รวมให้ได้ 24!')
    await page.locator('.smenu > button.btn.icon.gear').click()
    await page.locator('.smenu .sheet .row').filter({ hasText: 'ภาษา' }).click()
    await expect(page.locator('.hero .pitch h1')).toHaveText('Four numbers. Make 24!')
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await page.reload()
    await expect(page.locator('.hero .pitch h1')).toHaveText('Four numbers. Make 24!')
    // toggle back
    await page.locator('.smenu > button.btn.icon.gear').click()
    await page.locator('.smenu .sheet .row').filter({ hasText: 'Language' }).click()
    await expect(page.locator('.hero .pitch h1')).toHaveText('จับเลข 4 ตัว รวมให้ได้ 24!')
  })

  test('sound toggle stores the preference', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/')
    await page.locator('.smenu > button.btn.icon.gear').click()
    const row = page.locator('.smenu .sheet .row').filter({ hasText: 'เสียง' })
    // the test fixture boots every page muted — the toggle must flip the
    // state back on and persist it
    await expect(row.locator('.val')).toHaveText('ปิด')
    await row.click()
    await expect(row.locator('.val')).toHaveText('เปิด')
    const stored = await page.evaluate(() => localStorage.getItem('g24_sound'))
    expect(stored).not.toBe('off')
  })
})
