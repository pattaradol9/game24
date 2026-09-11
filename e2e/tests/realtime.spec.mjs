// Live profile socket: a deletion in one tab drops every other tab, and a
// banned account surfaces the ban dialog whether it meets the ban on
// restore or on sign-in. (Ban state is seeded straight into the DB — the
// admin API is out of scope for this suite.)
import { test, expect } from '../helpers/fixtures.mjs'
import {
  apiGoogleSignIn,
  establishSession,
  banPlayer,
  unique,
  API,
} from '../helpers/harness.mjs'
import { stageCredential } from '../helpers/fake-google.mjs'

test.describe('Live profile socket', () => {
  test('deleting the account in one tab drops the session in the others', async ({ page }) => {
    const name = unique('Google-doomed')
    const { token } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name })
    await establishSession(page, token)
    await page.goto('/')
    await expect(page.locator('button.profile-btn')).toContainText(name)
    // a second tab on the same account, watching the shop
    const tabB = await page.context().newPage()
    await tabB.goto('/shop')
    await expect(tabB.locator('button.profile-btn')).toContainText(name)
    // tab A deletes through the profile menu
    await page.locator('button.profile-btn').click()
    await page.locator('.profile .menu').getByText('ลบบัญชีถาวร', { exact: true }).click()
    const modal = page.locator('.overlay .panel').filter({ hasText: 'ลบบัญชีของคุณถาวร?' })
    await modal.locator('#del-confirm').fill('DELETE')
    await modal.getByRole('button', { name: 'ลบบัญชีถาวร' }).click()
    await expect(page.locator('.overlay .panel.modal h2')).toHaveText('ตั้งชื่อเล่นของคุณ')
    // the `deleted` push lands on tab B without a reload: the shop tab drops
    // its session (the entry modal itself only mounts on Home)
    await expect(tabB.locator('.profile button.btn', { hasText: 'เริ่มเล่น' })).toBeVisible({ timeout: 15_000 })
    // and the token is truly gone server-side
    const res = await page.request.get(`${API}/me`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(res.status()).toBe(401)
  })

  test('a ban met on session restore raises the ban dialog', async ({ page }) => {
    const { token, player } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name: unique('Banned') })
    banPlayer(player.id, 'e2e ban reason')
    await establishSession(page, token)
    await page.goto('/')
    const dialog = page.locator('.overlay .panel').filter({ hasText: 'บัญชีของคุณถูกระงับ' })
    await expect(dialog).toBeVisible()
    // the restore path cannot carry the reason line (the /me probe only
    // sees the "account banned" message), so the standard body shows
    await expect(dialog).toContainText('ผู้ดูแลระบบได้ระงับบัญชีนี้')
    await dialog.getByRole('button', { name: 'รับทราบ' }).click()
    await expect(dialog).toHaveCount(0)
    // the dead session was cleared: the entry gate is back
    await expect(page.locator('.overlay .panel.modal h2')).toHaveText('ตั้งชื่อเล่นของคุณ')
  })

  test('signing in with a banned Google account surfaces the same dialog', async ({ page }) => {
    const sub = unique('banned-sub')
    const name = unique('Banned-login')
    const { player } = await apiGoogleSignIn(page.request, { sub, name })
    banPlayer(player.id, 'banned at login')
    await page.goto('/')
    await stageCredential(page, { sub, name })
    await page.locator('.gsi-box .g-hit button').click()
    const dialog = page.locator('.overlay .panel').filter({ hasText: 'บัญชีของคุณถูกระงับ' })
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText('ผู้ดูแลระบบได้ระงับบัญชีนี้')
  })
})
