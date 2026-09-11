// Multiplayer rooms: two-seat matches played end to end — lobby rules,
// round play, the animated summary, the podium, seat quotas, the 10-seat
// cap, host loss and the leave confirmation.
import { test, expect } from '../helpers/fixtures.mjs'
import { guestPage, waitBoardReady, winHand, readNumbers, readClock } from '../helpers/harness.mjs'

const SUMMARY = '.overlay .panel.modal'

async function createRoomAsHost(host, { mode = 'jack', rounds = 12 } = {}) {
  const page = host.page
  await page.goto('/')
  await page.locator('button.btn.block', { hasText: 'สร้างห้อง' }).click()
  const modal = page.locator('.overlay .panel.modal')
  await expect(modal.locator('h2')).toHaveText('สร้างห้อง')
  await modal.locator('select').selectOption(mode)
  if (rounds !== 12) await modal.locator('.row-btns button.btn', { hasText: String(rounds) }).click()
  await modal.getByRole('button', { name: 'สร้างห้อง' }).click()
  await page.waitForURL(/\/room\/[A-Z0-9]{6}\?hostKey=/)
  return page.url().match(/\/room\/([A-Z0-9]{6})/)[1]
}

async function setupMatch(browser, opts = {}) {
  const host = await guestPage(browser)
  const member = await guestPage(browser)
  const code = await createRoomAsHost(host, opts)
  await member.page.goto(`/room/${code}`)
  // lobby: the member waits, the host sees the start button
  await expect(member.page.locator('.panel.lobby h2')).toHaveText('รอเจ้าของห้องกดเริ่ม...')
  await expect(member.page.locator('.lobby button.btn.primary.big')).toHaveCount(0)
  await expect(host.page.locator('.lobby button.btn.primary.big', { hasText: 'เริ่มแข่ง' })).toBeVisible()
  return { host, member, code }
}

function startMatch(host) {
  return host.page.locator('.lobby button.btn.primary.big').click()
}

/** Both seats solve the live round, host first; both end on the summary. */
async function playRoundTogether(host, member) {
  await winHand(host.page)
  // the solved seat keeps the full board under a status veil, ranked #1
  await expect(host.page.locator('.board-veil h2.good')).toHaveText('ตอบเสร็จแล้ว!')
  await expect(host.page.locator('.rail li.you .solved-tag')).toContainText('#1')
  await expect(member.page.locator('.rail li', { hasText: 'HOST' })).toContainText('#1')
  await winHand(member.page)
  const hostSummary = host.page.locator(SUMMARY).filter({ hasText: 'สรุปคะแนน!' })
  const memberSummary = member.page.locator(SUMMARY).filter({ hasText: 'สรุปคะแนน!' })
  await expect(hostSummary).toBeVisible()
  await expect(memberSummary).toBeVisible()
  return { hostSummary, memberSummary }
}

test.describe('Rooms', () => {
  test('a full match plays through the animated summary to the podium', async ({ browser }) => {
    test.setTimeout(600_000)
    const { host, member } = await setupMatch(browser, { mode: 'jack', rounds: 12 })
    try {
      await startMatch(host)
      // round one on both seats, with the round progress panel
      await expect(host.page.locator('.round-count')).toHaveText(/1\s*\/\s*12/)
      for (let round = 1; round <= 12; round++) {
        const { hostSummary, memberSummary } = await playRoundTogether(host, member)
        if (round === 1) {
          // both rows carry a finish order and a positive gain
          await expect(hostSummary.locator('ol.tally li')).toHaveCount(2)
          await expect(hostSummary.locator('ol.tally li.me .state.ok')).toContainText('ตอบเสร็จ #1')
          await expect(memberSummary.locator('ol.tally li.me .state.ok')).toContainText('ตอบเสร็จ #2')
          await expect(hostSummary.locator('ol.tally li.me .gain')).not.toHaveText('+0')
          // the host drives; the member waits (with the auto-advance line)
          await expect(hostSummary.locator('button.btn.primary.big')).toContainText('รอบต่อไป')
          await expect(memberSummary.locator('p.host-wait')).toContainText('รอ host')
          await expect(memberSummary.locator('p.auto')).toContainText('วิ')
        }
        const label = round === 12 ? 'ดูผลการแข่งขัน' : 'รอบต่อไป'
        await hostSummary.getByRole('button', { name: new RegExp(label) }).click()
        if (round < 12) {
          await expect(host.page.locator('.round-count')).toHaveText(new RegExp(`${round + 1}\\s*/\\s*12`))
        }
      }
      // podium on both seats, host on top (won every round)
      const hostName = host.player.nickname
      for (const page of [host.page, member.page]) {
        const podium = page.locator(`${SUMMARY} .podium`)
        await expect(podium.locator('.block.first')).toBeVisible({ timeout: 15_000 })
        await expect(podium).toContainText(hostName)
        await page.locator(SUMMARY).getByRole('button', { name: 'กลับหน้าแรก' }).click()
        await expect(page).toHaveURL(/\/$/)
      }
    } finally {
      await host.ctx.close()
      await member.ctx.close()
    }
  })

  test('seat quotas: private add-time, once-per-round gates, hint and private regen', async ({ browser }) => {
    test.setTimeout(150_000)
    const { host, member } = await setupMatch(browser, { mode: 'jack' })
    try {
      await startMatch(host)
      await waitBoardReady(host.page)
      await waitBoardReady(member.page)

      // host: the Solution helper fires once per round
      await host.page.locator('.actions .helper').filter({ hasText: 'เฉลย' }).locator('button.btn').click()
      await expect(host.page.locator('.hint-pop.chip')).toBeVisible()
      await host.page.locator('.actions .helper').filter({ hasText: 'เฉลย' }).click()
      await expect(host.page.locator('.why').filter({ hasText: 'เฉลยกดได้ 1 ครั้งต่อรอบ' })).toBeVisible()

      // member: the room add-time extends THEIR clock privately. The display
      // re-arms on the next one-second tick, so poll rather than read once.
      const before = await readClock(member.page)
      await member.page.locator('[data-test="room-add-time"]').click()
      await expect(member.page.locator('.toast')).toContainText('ต่อเวลาอีก 30 วินาที!')
      await expect
        .poll(() => readClock(member.page), { timeout: 5_000 })
        .toBeGreaterThanOrEqual(before + 25)
      // one extend per round: the button is dark now and explains itself
      await member.page.locator('.helper', { has: member.page.locator('[data-test="room-add-time"]') }).click()
      await expect(member.page.locator('.why').filter({ hasText: 'เพิ่มเวลากดได้ 1 ครั้งต่อรอบ' })).toBeVisible()

      // member: a regen swaps in a private fresh hand, host board untouched
      const hostNums = (await readNumbers(host.page)).join(',')
      const memberNums = (await readNumbers(member.page)).join(',')
      await member.page.locator('.actions .helper').filter({ hasText: 'เปลี่ยนโจทย์' }).locator('button.btn').click()
      await expect
        .poll(async () => (await readNumbers(member.page)).join(','), { timeout: 8_000 })
        .not.toBe(memberNums)
      expect((await readNumbers(host.page)).join(',')).toBe(hostNums)
    } finally {
      await host.ctx.close()
      await member.ctx.close()
    }
  })

  test('renaming in the room broadcasts to every seat', async ({ browser }) => {
    const { host, member } = await setupMatch(browser)
    try {
      await host.page.locator('button.who.panel').click()
      const modal = host.page.locator('.overlay .panel').filter({ hasText: 'เปลี่ยนชื่อ' })
      await modal.locator('input.name-input').fill('Host-Renamed')
      await modal.getByRole('button', { name: 'บันทึก' }).click()
      await expect(member.page.locator('.rail')).toContainText('Host-Renamed', { timeout: 10_000 })
    } finally {
      await host.ctx.close()
      await member.ctx.close()
    }
  })

  test('leaving mid-round confirms first', async ({ browser }) => {
    const { host, member } = await setupMatch(browser)
    try {
      await startMatch(host)
      await waitBoardReady(member.page)
      await member.page.locator('.btn.back').click()
      const confirm = member.page.locator('.panel.box[role="dialog"]')
      await expect(confirm).toHaveAttribute('aria-label', 'ออกจากแมตช์หรือไม่?')
      await member.page.locator('[data-test="confirm-cancel"]').click()
      await expect(confirm).toHaveCount(0)
      await expect(member.page.locator('.clock')).toBeVisible()
      // real leave routes home
      await member.page.locator('.btn.back').click()
      await member.page.locator('[data-test="confirm-ok"]').click()
      await expect(member.page).toHaveURL(/\/$/)
    } finally {
      await host.ctx.close()
      await member.ctx.close()
    }
  })

  test("the host's confirmed exit ends the match with the final podium on every seat", async ({ browser }) => {
    const { host, member } = await setupMatch(browser)
    try {
      await startMatch(host)
      await waitBoardReady(host.page)
      await waitBoardReady(member.page)
      // the host confirms the exit: the copy says the match ends for everyone
      await host.page.locator('.btn.back').click()
      const confirm = host.page.locator('.panel.box[role="dialog"]')
      await expect(confirm).toContainText('จบการแข่งขันของทุกคนทันที')
      await host.page.locator('[data-test="confirm-ok"]').click()
      // both seats land on the final podium with the host-left note
      const hostPodium = host.page.locator('.overlay .panel.modal').filter({ hasText: 'Host ออกจากห้อง' })
      await expect(hostPodium).toBeVisible()
      const memberPodium = member.page.locator('.overlay .panel.modal').filter({ hasText: 'Host ออกจากห้อง' })
      await expect(memberPodium).toBeVisible()
      // the podium's back-home is the way out, for the member…
      await memberPodium.getByRole('button', { name: 'กลับหน้าแรก' }).click()
      await expect(member.page).toHaveURL(/\/$/)
      // …and for the host too
      await hostPodium.getByRole('button', { name: 'กลับหน้าแรก' }).click()
      await expect(host.page).toHaveURL(/\/$/)
    } finally {
      await host.ctx.close()
      await member.ctx.close()
    }
  })

  test('the room caps at ten seats and refuses the eleventh', async ({ browser }) => {
    // host seat only — no second member, the fillers must fill seats 2..10
    const host = await guestPage(browser)
    const code = await createRoomAsHost(host)
    let eleventh
    try {
      // fill the remaining nine seats through the room socket directly
      await host.page.evaluate(async (roomCode) => {
        const join = (name, token) =>
          new Promise((resolve, reject) => {
            const url = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/v1/ws/room/${roomCode}`
            const ws = new WebSocket(url)
            const timer = setTimeout(() => { ws.close(); reject(new Error(`join timeout: ${name}`)) }, 10_000)
            ws.onopen = () => ws.send(JSON.stringify({ type: 'join', data: { name, token } }))
            ws.onmessage = (ev) => {
              const msg = JSON.parse(ev.data)
              if (msg.type === 'welcome') { clearTimeout(timer); resolve(msg) }
              if (msg.type === 'error') { clearTimeout(timer); reject(new Error(`server: ${JSON.stringify(msg.data)}`)) }
            }
            ws.onerror = () => { clearTimeout(timer); reject(new Error('ws error')) }
          })
        for (let i = 0; i < 9; i++) {
          const res = await fetch('/api/v1/players', {
            method: 'POST',
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify({ nickname: `filler-${i}` }),
          })
          const { data } = await res.json()
          try {
            await join(`filler-${i}`, data.token)
          } catch (err) {
            err.message = `filler ${i}: ${err.message}`
            throw err
          }
        }
      }, code)
      // the roster is full: a fresh join drops to the closed-room screen
      eleventh = await guestPage(browser)
      await eleventh.page.goto(`/room/${code}`)
      await expect(eleventh.page.locator('.panel.lobby h2')).toHaveText('ห้องนี้ถูกปิดไปแล้ว', { timeout: 15_000 })
    } finally {
      await host.ctx.close()
      if (eleventh) await eleventh.ctx.close()
    }
  })

  test('a vanished host raises the waiting overlay on the members', async ({ browser }) => {
    test.setTimeout(90_000)
    const { host, member } = await setupMatch(browser)
    try {
      await startMatch(host)
      await waitBoardReady(member.page)
      await host.ctx.close()
      // after a short debounce the member sees the host-wait overlay
      const overlay = member.page.locator('.overlay.host-wait')
      await expect(overlay).toBeVisible({ timeout: 15_000 })
      await expect(overlay.locator('h2')).toHaveText('Host หลุดจากห้องชั่วคราว')
      await expect(overlay.locator('.wait-count')).toContainText('ห้องจะปิดในอีก')
      await expect(overlay.locator('[data-test="host-wait-leave"]')).toBeVisible()
    } finally {
      await member.ctx.close()
    }
  })
})
