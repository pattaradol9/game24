# game24 — End-to-End Test Suite

Playwright E2E coverage for the whole game24 stack (web + API + database),
excluding the admin portal by scope.

## What it covers

| Suite | Coverage |
|---|---|
| `home.spec.mjs` | entry gate (nickname modal), guest sessions, mode cards, play CTA, room-code join, guest vs. Google profile menus, rename, sign in/out, language + sound settings |
| `solo.spec.mjs` | winning hands (auto-submit, stars, tally), wrong merges + undo, Solution helper, item-gated helper buttons with why-bubbles, exit confirmation + game summary, refresh-resume, the 3-2-1-GO intro (motion-enabled), free timeout fold |
| `solo-player.spec.mjs` | Google sign-in through the real button path, Add-time item (clamped top-up, per-session rule), the 0:00 Time-Extension offer, Skip Pass flow, live EXP banking into the level bar, account deletion |
| `shop.spec.mjs` | tab switching + deep links, catalog sizes, guest Google-gates, skin buy → equip → board skin class, batch item buying via the count slider, personal boost use → BuffBar tray, cross-tab inventory sync through the player socket |
| `meta.spec.mjs` | achievements catalog (50 entries, unlock + progress), leaderboards (mode/period tabs, ranked solve), legal pages in both languages, `/skins` redirect |
| `rooms.spec.mjs` | a full 12-round match to the podium (both seats), round summary rows/finish-order/auto-advance copy, per-seat helpers (hint, private add-time, private regen) with once-per-round gates, rename broadcast, leave confirmation, the 10-seat cap (11th refused), host-loss overlay |
| `realtime.spec.mjs` | live `deleted` push across tabs, ban dialog on session restore and on sign-in (ban state seeded straight into SQLite — no admin API) |

## How it runs

`npx playwright test` (from this directory) starts an isolated stack:

1. `scripts/start-deps.mjs` builds the Go server, then runs it on **:18080**
   against a throwaway SQLite database in `.tmp/`, with:
   - `GOOGLE_JWKS_URL` pointed at a local fake JWKS (`:14801`) — the suite
     mints real RS256 ID tokens, so `/auth/google` verifies through its
     genuine code path; the browser gets a GSI stub injected instead of the
     real Google iframe,
   - `RATE_LIMIT_PER_MIN` raised (the browser bursts far past 200 req/min),
   - the admin API disabled.
2. The Vite dev server runs on **:15173** with `VITE_API_TARGET` pointing at
   :18080, so a developer's own `make dev` on :5173/:8080 is never touched.

Both servers are torn down when the run ends. Every run starts from an empty
database.

Seeding (coins, items, bans) writes straight to the SQLite file through
`node:sqlite` — deliberately never through the admin API, which is out of
scope for this suite.

## Commands

```sh
npm install                # once
npx playwright test        # full suite (chromium)
npx playwright test tests/rooms.spec.mjs        # one suite
npx playwright test -g "podium" --trace on      # one test, with trace
npx playwright show-trace .tmp/artifacts/<...>/trace.zip
```

Config knobs live in `playwright.config.mjs` (workers, retries, timeouts);
`helpers/` holds the fixtures, the fake Google, a 24-game solver that plays
real hands through clicks, and the seeding harness. `scripts/probe-room.mjs`
is a standalone protocol debugger for the room websocket.
