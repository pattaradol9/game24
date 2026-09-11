// Shop: skins + boost items + inventory. Guest gates, buying with seeded
// coins, equipping onto the board, arming a personal boost and the live
// cross-tab inventory sync through the player socket.
import { test, expect } from '../helpers/fixtures.mjs'
import {
  apiGoogleSignIn,
  apiCreateGuest,
  establishSession,
  seedCoins,
  seedItem,
  fetchMe,
  unique,
} from '../helpers/harness.mjs'

const parseCoins = (text) => parseInt(String(text).replace(/[^\d]/g, ''), 10)

async function newShopper(page, coins = 0) {
  const { token, player } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name: unique('Shop') })
  if (coins > 0) seedCoins(player.id, coins)
  await establishSession(page, token)
  return { token, player }
}

test.describe('Shop', () => {
  test('three tabs switch and deep-link', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/shop')
    await expect(page.locator('h1:visible')).toHaveText('ชุดไพ่')
    await expect(page.locator('article.skin-card')).toHaveCount(12)
    await page.locator('nav.tabs [role="tab"]', { hasText: 'ไอเทมบูสต์' }).click()
    await expect(page).toHaveURL(/tab=items/)
    await expect(page.locator('h1:visible')).toHaveText('ไอเทมบูสต์')
    await expect(page.locator('article.item-card')).toHaveCount(10)
    await page.goto('/shop?tab=inventory')
    await expect(page.locator('h1:visible')).toHaveText('คลังไอเทม')
  })

  test('guests can browse but every buy is gated on Google', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/shop')
    await expect(page.locator('.gate:visible')).toContainText('ต้องล็อกอินด้วย Google เพื่อซื้อและเปลี่ยนชุดไพ่')
    const firstBuy = page.locator('article.skin-card .buy-btn').first()
    await expect(firstBuy).toBeDisabled()
    await page.goto('/shop?tab=items')
    await expect(page.locator('.gate:visible')).toContainText('ต้องล็อกอินด้วย Google เพื่อซื้อและใช้ไอเทม')
    await page.goto('/shop?tab=inventory')
    await expect(page.locator('.empty')).toContainText('ยังไม่มีไอเทมในคลัง')
  })

  test('buying and equipping a skin dresses the board', async ({ page }) => {
    const { token } = await newShopper(page, 5_000)
    await page.goto('/shop')
    const card = page.locator('article.skin-card').nth(1) // first paid skin
    const price = parseCoins(await card.locator('.chip').first().textContent())
    const skinId = await card.getAttribute('data-skin')
    await card.locator('.buy-btn').click()
    const confirm = page.locator('.panel.confirm')
    await expect(confirm).toBeVisible()
    await confirm.locator('.confirm-buy').click()
    await expect(confirm).toHaveCount(0)
    // owned: the CTA flips to equip and the balance dropped by the price
    await expect(card.locator('.equip-btn')).toBeVisible()
    await expect(page.locator('.chip.accent b.num')).toBeVisible()
    await expect
      .poll(async () => (await fetchMe(page.request, token)).totalCoins, { timeout: 5_000 })
      .toBe(5_000 - price)
    // equipping marks the card and scopes the class onto the solo board
    await card.locator('.equip-btn').click()
    await expect
      .poll(async () => (await fetchMe(page.request, token)).skin, { timeout: 10_000 })
      .toBe(skinId)
    await expect(card).toHaveClass(/equipped/)
    await page.goto('/solo?mode=queen')
    await expect(page.locator('.board')).toHaveClass(new RegExp(`skin-${skinId}`))
  })

  test('boost items buy in batches through the count slider', async ({ page }) => {
    const { token } = await newShopper(page, 4_000)
    await page.goto('/shop?tab=items')
    const card = page.locator('article.item-card').filter({ hasText: '×2' }).first()
    await card.locator('.buy-btn').click()
    const confirm = page.locator('.panel.confirm')
    await expect(confirm).toBeVisible()
    const slider = confirm.locator('#buy-count')
    await slider.fill('2')
    await expect(confirm.locator('.confirm-buy')).toHaveText(/ซื้อ 2 ชิ้น/)
    await confirm.locator('.confirm-buy').click()
    await expect(confirm).toHaveCount(0)
    // 2 × price debited at once
    await expect(page.locator('.chip.accent b.num')).toBeVisible()
    await expect
      .poll(async () => (await fetchMe(page.request, token)).totalCoins, { timeout: 5_000 })
      .toBeLessThan(4_000)
    // the inventory stacks the units
    await page.goto('/shop?tab=inventory')
    await expect(page.locator('article.row').first().locator('.qty')).toHaveText('×2')
  })

  test('using an EXP item arms the personal boost tray', async ({ page }) => {
    const { token, player } = await newShopper(page, 2_000)
    seedItem(player.id, 'exp2', 2)
    await page.goto('/shop?tab=inventory')
    const row = page.locator('article.row').first()
    await expect(row.locator('.qty')).toHaveText('×2')
    await expect(page.locator('.buffbar')).toHaveCount(0)
    await row.locator('.use-btn').click()
    const confirm = page.locator('.panel.confirm')
    await confirm.getByRole('button', { name: /ใช้ 1 ชิ้น/ }).click()
    await expect(confirm).toHaveCount(0)
    // the window is live server-side…
    await expect
      .poll(async () => (await fetchMe(page.request, token)).boosts?.exp?.multiplier, { timeout: 8_000 })
      .toBe(2)
    // …the row chip starts counting down, and the HOME header tray (the
    // shop page carries no buff tray) shows the ×2 stack in item colour
    await expect(row.locator('.chip.active-chip')).toBeVisible({ timeout: 10_000 })
    await page.goto('/')
    const buff = page.locator('.buffbar .buff').first()
    await expect(buff).toContainText('×2')
    await expect(buff).toHaveClass(/c-common/)
  })

  test('a purchase in one tab lands in the inventory of another', async ({ page }) => {
    const { token, player } = await apiGoogleSignIn(page.request, { sub: unique('sub'), name: unique('Sync') })
    seedCoins(player.id, 2_000) // the buy needs an affordable balance
    // tab A: the shop — tab B: the inventory, same account, both live
    await establishSession(page, token)
    await page.goto('/shop?tab=items')
    const tabB = await page.context().newPage()
    await tabB.goto('/shop?tab=inventory')
    await expect(tabB.locator('.empty')).toBeVisible()
    // buy one helper item in tab A
    const timeCard = page.locator('article.item-card').filter({ hasText: 'นาฬิกาทราย' }).first()
    await timeCard.locator('.buy-btn').click()
    const confirm = page.locator('.panel.confirm')
    await confirm.locator('.confirm-buy').click()
    await expect(confirm).toHaveCount(0)
    // tab B picks the new stack up through the live player push
    await expect(tabB.locator('article.row')).toHaveCount(1, { timeout: 15_000 })
    await expect(tabB.locator('article.row').first().locator('.qty')).toHaveText('×1')
  })
})
