// Solo play as a guest: solving, wrong merges, hints, gated helpers,
// exit confirmation, refresh-resume and the free timeout fold.
import { test, expect } from '../helpers/fixtures.mjs'
import {
  apiCreateGuest,
  establishSession,
  winHand,
  playHand,
  waitBoardReady,
  readNumbers,
  readClock,
} from '../helpers/harness.mjs'
import { solveNon24, OP_ARIA } from '../helpers/solver.mjs'

test.describe('Solo (guest)', () => {
  async function newSoloPage(page, mode = 'jack') {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto(`/solo?mode=${mode}`)
    await waitBoardReady(page)
  }

  test('winning a hand shows the win dialog and chains the tally', async ({ page }) => {
    await newSoloPage(page, 'queen')
    await winHand(page)
    const modal = page.locator('.overlay .panel.modal')
    await expect(modal.locator('.verdict.win')).toHaveText('ทำได้แล้ว!')
    await expect(modal.locator('.expr')).not.toBeEmpty()
    await expect(modal.locator('.star.on').first()).toBeVisible()
    // the running tally banks the hand's points
    const score = await page.locator('[data-test="session-score"] .score').textContent()
    expect(parseInt(score, 10)).toBeGreaterThan(0)
    // next round deals a fresh hand
    await modal.getByRole('button', { name: 'รอบต่อไป' }).click()
    await waitBoardReady(page)
    await expect(page.locator('.overlay .panel.modal')).toHaveCount(0)
  })

  test('a wrong final merge keeps the hand open and undo restores it', async ({ page }) => {
    await newSoloPage(page, 'queen')
    const nums = await readNumbers(page)
    const wrong = solveNon24(nums)
    expect(wrong, 'hand has no non-24 line — unexpected for queen').not.toBeNull()
    await playHand(page, wrong.target, nums)
    // no win dialog: the hand stays open with the wrong single card.
    // assert via the step history — slot counting is unreliable here,
    // because under reduced motion the board's TransitionGroup leaves its
    // enter/leave elements stuck in the DOM.
    const historyLines = page.locator('.history ol li')
    await expect(page.locator('.overlay .panel.modal')).toHaveCount(0)
    await expect(historyLines).toHaveCount(wrong.steps.length)
    await expect(historyLines.last()).toContainText(`= ${wrong.target}`)
    // undo walks one merge back
    await expect(page.locator('[data-test="undo-step"]')).toBeEnabled()
    await page.locator('[data-test="undo-step"]').click()
    await expect(historyLines).toHaveCount(wrong.steps.length - 1)
  })

  test('the Solution helper reveals the equation chip', async ({ page }) => {
    await newSoloPage(page, 'queen')
    await page.locator('.actions .helper').filter({ hasText: 'เฉลย' }).locator('button.btn').click()
    const chip = page.locator('.hint-pop.chip')
    await expect(chip).toBeVisible()
    await expect(chip.locator('.hint-row code')).not.toBeEmpty()
    await expect(chip.locator('.hint-eq')).toHaveText('= 24')
  })

  test('add-time and skip are dark for guests and explain why', async ({ page }) => {
    await newSoloPage(page, 'queen')
    // guests never own items: the gate explains the item requirement
    await page.locator('.helper', { has: page.locator('[data-test="add-time"]') }).click()
    await expect(page.locator('.why').filter({ hasText: 'การเพิ่มเวลาต้องใช้' })).toBeVisible()
    await page.locator('.helper', { has: page.locator('[data-test="skip-hand"]') }).click()
    await expect(page.locator('.why').filter({ hasText: 'การข้ามต้องใช้' })).toBeVisible()
  })

  test('exiting mid-hand confirms first, then shows the game summary', async ({ page }) => {
    await newSoloPage(page, 'queen')
    await page.locator('.btn.back').click()
    const confirm = page.locator('.panel.box[role="dialog"]')
    await expect(confirm).toHaveAttribute('aria-label', 'จบเกมแล้วหรือไม่?')
    // cancel keeps the hand alive
    await page.locator('[data-test="confirm-cancel"]').click()
    await expect(confirm).toHaveCount(0)
    await expect(page.locator('.clock .mmss')).toBeVisible()
    // confirm ends the game and shows the summary before routing home
    await page.locator('.btn.back').click()
    await page.locator('[data-test="confirm-ok"]').click()
    const modal = page.locator('.overlay .panel.modal')
    await expect(modal.locator('.verdict.lose')).toHaveText('จบเกมแล้ว!')
    await expect(modal.locator('[data-test="session-summary"]')).toBeVisible()
    await modal.getByRole('button', { name: 'ออกจากเกม' }).click()
    await expect(page).toHaveURL(/\/$/)
  })

  test('a refresh mid-hand resumes the same board with no countdown', async ({ page }) => {
    await newSoloPage(page, 'jack')
    const nums = await readNumbers(page)
    // make one harmless merge (never wins by construction: target ≠ 24)
    const wrong = solveNon24(nums)
    await page.locator(`[data-card-id="${wrong.steps[0].a}"]`).click()
    await page.locator(`.pad button.op[aria-label="${OP_ARIA[wrong.steps[0].op]}"]`).click()
    await page.locator(`[data-card-id="${wrong.steps[0].b}"]`).click()
    await expect(page.locator('[data-card-id="s0"]')).toBeVisible()

    const clockBefore = await readClock(page)
    await page.reload()
    // restored hand: no 3-2-1-GO, the merged card is back, the clock continues
    await expect(page.locator('.countdown')).toHaveCount(0)
    await expect(page.locator('[data-card-id="s0"]')).toBeVisible()
    expect(await readClock(page)).toBeLessThanOrEqual(clockBefore)
    // the tallies survived too
    await expect(page.locator('[data-test="session-score"]')).toBeVisible()
  })

  test('a hand whose clock runs out folds free and shows the solution', async ({ page }) => {
    test.setTimeout(200_000)
    await newSoloPage(page, 'ace') // 60s window, the shortest
    // the clock goes urgent in the last ten seconds
    await expect(page.locator('.clock.urgent')).toBeVisible({ timeout: 80_000 })
    const modal = page.locator('.overlay .panel.modal')
    await expect(modal.locator('.verdict.lose')).toHaveText('หมดเวลา!', { timeout: 60_000 })
    await expect(modal.locator('.solution code')).not.toBeEmpty()
    await expect(modal.locator('[data-test="session-summary"]')).toBeVisible()
    await modal.getByRole('button', { name: 'เริ่มเล่นใหม่' }).click()
    await waitBoardReady(page)
  })
})

test.describe('Solo (guest, motion enabled)', () => {
  test.use({ reducedMotion: 'no-preference' })

  test('the countdown intro plays 3-2-1-GO on a fresh hand', async ({ page }) => {
    const { token } = await apiCreateGuest(page.request)
    await establishSession(page, token)
    await page.goto('/solo?mode=jack')
    const cd = page.locator('.countdown')
    await expect(cd).toBeVisible()
    await expect(cd.locator('.n.num').first()).toHaveText('3')
    await expect(cd.locator('.n.num').first()).toHaveText('2')
    await expect(cd.locator('.n.num').first()).toHaveText('1')
    await expect(cd.locator('.go')).toHaveText('GO!')
    await expect(cd).toHaveCount(0, { timeout: 5_000 })
    await expect(page.locator('.clock .mmss')).toBeVisible()
  })
})
