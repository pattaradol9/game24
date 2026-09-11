// Playwright E2E configuration for the whole game24 stack.
//
// Two web servers are started for the run:
//   1. scripts/start-deps.mjs — builds and runs the Go API on :8080 against a
//      throwaway SQLite database, plus a local JWKS stand-in for Google
//      (GOOGLE_JWKS_URL) so sign-in works end to end without the network.
//   2. the Vite dev server on :5173 (the `make dev` front end), proxying /api.
import { defineConfig } from '@playwright/test'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const e2eDir = path.dirname(fileURLToPath(import.meta.url))
const tmpDir = path.join(e2eDir, '.tmp')

export default defineConfig({
  testDir: path.join(e2eDir, 'tests'),
  timeout: 45_000,
  expect: { timeout: 10_000 },
  globalTimeout: 30 * 60_000,
  fullyParallel: true,
  workers: process.env.G24_E2E_WORKERS ? Number(process.env.G24_E2E_WORKERS) : 2,
  retries: process.env.G24_E2E_RETRIES ? Number(process.env.G24_E2E_RETRIES) : 1,
  reporter: [['list'], ['html', { open: 'never', outputFolder: path.join(tmpDir, 'report') }]],
  outputDir: path.join(tmpDir, 'artifacts'),
  use: {
    // the suite runs its own stack on :15173 (vite) + :18080 (API) so a
    // developer's `make dev` on :5173/:8080 is never touched
    baseURL: 'http://localhost:15173',
    viewport: { width: 1280, height: 800 },
    locale: 'th-TH',
    timezoneId: 'Asia/Bangkok',
    // default: animations off (the 3-2-1-GO intro, merge glides, WebGL
    // layers) for speed and stability; set G24_E2E_MOTION=1 for a headed,
    // full-motion demo run
    reducedMotion: process.env.G24_E2E_MOTION ? 'no-preference' : 'reduce',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    actionTimeout: 15_000,
    navigationTimeout: 20_000,
  },
  webServer: [
    {
      command: 'node scripts/start-deps.mjs',
      cwd: e2eDir,
      url: 'http://127.0.0.1:18080/api/v1/healthz',
      reuseExistingServer: false, // always a fresh database
      timeout: 120_000,
      stdout: 'pipe',
      stderr: 'pipe',
    },
    {
      command: 'npm run dev -- --strictPort --port 15173',
      cwd: path.join(e2eDir, '..', 'web'),
      url: 'http://localhost:15173',
      reuseExistingServer: false,
      timeout: 120_000,
      env: { ...process.env, VITE_API_TARGET: 'http://127.0.0.1:18080' },
      stdout: 'ignore',
      stderr: 'pipe',
    },
  ],
})
