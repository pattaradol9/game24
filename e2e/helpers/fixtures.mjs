// Shared Playwright fixtures: every page gets the Google GSI stub (before the
// app boots), a blocked real GSI script, and muted sound so headless audio
// never stalls. Tests import `test`/`expect` from here, never from
// @playwright/test directly.
import { test as base, expect } from '@playwright/test'
import { GSI_STUB, blockGoogleScript } from './fake-google.mjs'

export const test = base.extend({
  context: async ({ context }, use) => {
    await blockGoogleScript(context)
    await use(context)
  },
  page: async ({ page }, use) => {
    await page.addInitScript(GSI_STUB)
    await page.addInitScript(() => {
      try { localStorage.setItem('g24_sound', 'off') } catch { /* private mode */ }
    })
    await use(page)
  },
})

export { expect }
