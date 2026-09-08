# Agent Guidelines

## Language policy

- Communicate with the user in **Thai**: conversation, explanations, summaries, and questions.
- Everything that gets implemented must be in **English**: code identifiers, code comments, commit messages, documentation (README, ADRs, API docs), `.env.example` descriptions, test names, and log/error messages.
- Exception: user-facing product UI copy in `web/` stays bilingual (Thai/English) by design — do not translate it as part of this policy.

## Terminal policy

- Never leave background processes running when the work is done. Stop every dev server, watcher, or long-running command you started (verify the ports are actually free — `go run` children can outlive their wrapper).
- The user runs and verifies the app themselves (e.g. `make dev`). Do not keep servers up "for convenience"; closing the terminal after finishing is mandatory, every time.

# Game Systems

Documentation for the meta-game systems layered on top of the core 24-gameplay: coins, achievements and card skins. All persistence is server-side in SQLite (`store` package); the web client never owns balances.

## Coins

- Every **solved hand pays EXP and coins**, in solo play and in multiplayer rooms alike, mirroring how EXP is already awarded.
- Payout formula (`store.CoinsForHand`): `coins = points / 10`, minimum 1 — deterministic, so the client can show the payout from the submit response without extra round trips. Skipped/expired hands pay nothing.
- Balances live in `players.total_coins`. Multiplayer round wins award coins through the same path (`store.AwardSolve` with `roomWin=true`, which also increments `total_wins` for win-count achievements).
- Guests never accumulate anything: award paths are gated on `!player.IsGuest`, exactly like EXP.

## Achievements

- Catalog of exactly **50 achievements** in `internal/achv` (`achv.Catalog`), single source of truth, evaluated server-side only.
- Each entry has an **escalating reward by difficulty tier**: bronze → silver → gold → platinum → legend (e.g. bronze ≈ 20–60 EXP / 15–40 coins, legend ≈ 1200–2000 EXP / 800–1300 coins). Rewards are paid automatically on unlock, in the same transaction that records the unlock.
- Tracked metrics: lifetime solves (total + per mode), best streak, level, coins held, multiplayer round wins, fastest solve (best time in ms, lower-is-better), solves without hints, hints used, distinct play days, skips. Counters derive from `player_mode_stats`, the `rounds` table and `players` columns — the snapshot builder is `store.AchievementSnapshot`.
- **Evaluation points**: after every hand (solved or skipped) in solo play (`store.AwardSolve` / `store.AwardSkip`) and after every multiplayer round win (hub award callback in `cmd/server/main.go`). Newly unlocked entries are returned inline in the REST responses (`newAchievements`) or pushed to the winner over the room websocket (`{type: "achievements", data: {unlocked: [...]}}`).
- Unlocks persist in `player_achievements` (id + timestamp); `store.EvaluateAchievements` is idempotent — already-unlocked entries are never re-granted.

### API surface

- `GET /api/v1/achievements` — public catalog + (with token) `unlocked` map and `progress` map (`{current, target}` per id). Guests get an empty unlocked/progress map.
- Player JSON now includes `totalCoins` and `skin` (equipped skin id, `""` = classic).

## Server boosts

- **Server-wide payout multipliers** the admin authors in advance and arms when the event should start — one campaign per kind: `exp` multiplies the EXP every solved hand banks, `coins` multiplies its coin payout. One row per kind in the `boosts` table (`kind` unique, seeded with disarmed ×2/60min drafts; `store.AllBoosts`). Databases from before the coins kind carried their config over through a migration that renames `exp_boost` → `boosts` (kind `exp`), so an armed window survives the upgrade.
- **Arming**: enable starts the window at now → now + duration (`store.EnableBoost`); enabling an already-active boost restarts the window. Disable stops payouts immediately while keeping the config for the next run (`store.DisableBoost`). An armed window simply expires when `ends_at` passes — nothing to clean up. Config saves (`store.SetBoostConfig`) never touch the run state, so a live boost can be retuned mid-event; the new multiplier applies to subsequent hands, and the running window is untouched.
- **Payout**: while a kind's window is open (`store.ActiveBoost(kind)`, compared against SQLite's clock) every solved hand's payout is multiplied by `BoostAmount(base, multiplier)` = `round(base × multiplier)` — solo play and multiplayer round wins alike (both lookups happen inside `store.awardEXP`, before the transaction opens: the pool holds a single connection, so querying inside the tx would deadlock). The coin boost multiplies `CoinsForHand`'s output. **Achievement rewards are never multiplied**, and guests are untouched by the existing gate.
- **Visibility**: running boosts ride in the player JSON as `boosts: {"exp": {kind, multiplier, endsAt}, "coins": …}` (omitted when nothing runs; each kind only while its window is open), so every profile fetch, ws snapshot and `player` push carries them. The solo submit response additionally returns `exp` — the EXP the hand actually banked — and its `coins` figure is the boosted amount; `ResultModal` badges the payout lines with `×N` chips. Players see Ragnarok-style buff icons (`BuffBar.vue`, an inline tray: in the Home header next to the profile/sound/language cluster on desktop, in the solo header, and inside the mobile quick-settings sheet; the mobile header itself collapses sound + language + boosts into one `QuickSettings.vue` gear popover), each square carrying its multiplier, a live countdown and a tooltip, driven by the `activeBoosts` ref in `web/src/auth.js` (kept fresh by `player` snapshots and `boost` pushes).
- **Live updates**: every boost mutation broadcasts a `boost` event (`{"boosts": {…}}` — the full active view, empty when none) to **all** connected player sockets via `presence.Broker.Broadcast`; open tabs swap their buff icons without a refresh.

### API surface

- `GET /api/v1/admin/boosts` — every kind's config + run state (`kind`, `multiplier`, `durationMinutes`, `label`, `enabled`, `active`, `startsAt`/`endsAt` RFC3339, `remainingSec`, `updatedBy`/`updatedAt`); `GET /api/v1/admin/boosts/{kind}` — one kind.
- `PUT /api/v1/admin/boosts/{kind}` with `{multiplier, durationMinutes, label}` — saves the draft (×1–×100, 1 min–30 days; out of range → 400 via `store.ErrBoostRange`; unknown kind → 404; label clamped to 80 chars). Events `admin.boost.set` (kind + before/after), `admin.boost.enable` (kind, endsAt), `admin.boost.disable` (kind).
- `POST /api/v1/admin/boosts/{kind}/enable` / `POST /api/v1/admin/boosts/{kind}/disable` — arm/stop one kind; each success responds with the fresh state and fans it out to every open socket.
- Admin portal: the **Server Boosts** tab (`AdminBoosts.vue`) renders one `AdminBoostCard` section per kind — status badge with a live countdown, the draft form (multiplier, duration with unit presets, label), Save settings / Enable now / Disable. The portal header shows a pulsing badge per running boost (polled every 30s and refreshed on every boost mutation), so what's active is visible from any tab.

## Admin economy management

The admin portal (ADMIN_EMAILS allowlist, `internal/handler/admin.go`) can correct a player's meta-game economy directly; every mutation is event-logged with the acting admin as actor. Every player-facing mutation also pushes a fresh profile snapshot through the player's live socket (see Live profile socket), so open tabs update without a refresh.

- **Coins**: `POST /api/v1/admin/players/{id}/coins` with `{value}` — overwrites the balance outright (the admin UI pre-fills the current value); negatives → 400, unknown player → 404 (`store.SetPlayerCoins`, event `admin.coins.set` with the before/after pair). The admin player JSON (list + detail) carries `totalCoins`.
- **Achievements**: `POST /api/v1/admin/players/{id}/achievements/grant` and `.../revoke` with `{id}` (`store.GrantAchievement` / `store.RevokeAchievement`, events `admin.achievement.grant` / `admin.achievement.revoke`). Granting pays the entry's standard EXP/coin rewards onto the lifetime totals only (never per-mode stats), exactly like a natural unlock; a double grant → 409, unknown achievement/player → 404. Revoking removes only the unlock row — banked rewards stay, and a revoked entry re-unlocks on the player's next evaluation if its condition still holds. Admin adjustments deliberately do **not** trigger an evaluation cascade (evaluation points stay as listed under Achievements).
- **EXP**: `POST /api/v1/admin/players/{id}/exp` with `{mode, value}` — overwrites that mode's EXP with an absolute value (negatives → 400) and resyncs `total_exp` to the per-mode sum (`store.SetPlayerEXP`, event `admin.exp.set`).
- **Tier**: `POST /api/v1/admin/players/{id}/tier` with `{tier}` — stores a tier override on `players.tier` (`store.SetPlayerTier`, event `admin.tier.set`); `{tier: ""}` clears it. Tiers are bronze → silver → gold → platinum → diamond → master (`progress.TierFromName` validates; unknown names → 400). The override replaces the level-derived tier **everywhere**: profile JSON, single-player and admin leaderboards, room seats (websocket join) and the tier hint bonus (`progress.TierBonusQuota`). `players.tier` empty string = derive from level, the normal path.
- **Ban/unban**: `PATCH /api/v1/admin/players/{id}` with `{banned, banReason}` (`store.SetPlayerBanned`, events `admin.ban` / `admin.unban`). An admin can never ban their own account — the API refuses with 400 (`cannot ban your own admin account`) and the portal hides the button on self rows. Banning pushes a terminal `banned` event through the player's live socket (reason included) instead of the plain profile snapshot; unbanning resumes normal pushes and the player's old token works again immediately.
- `GET /api/v1/admin/players/{id}` detail now also returns `achievements` — the player's unlocked entries (id, tier, title, rewards, unlock timestamp RFC3339), newest first.

### Database settings: backup & restore

- **Backup**: `GET /api/v1/admin/db/backup` streams a consistent `VACUUM INTO` snapshot as a download. Resets and restores always write a safety snapshot `game24-backup-<timestamp>.db` next to the database first (same-second collisions grow a `-N` suffix); `GET /api/v1/admin/db/backups` lists those server-side files (name, size, modified) for the Settings page.
- **Restore**: `POST /api/v1/admin/db/restore` (`store.RestoreFrom`) replaces the live data with a snapshot — either `{"name": "game24-backup-…db", "confirm": "RESTORE"}` for a server-side file (name must match `game24-backup-*.db`, so no path traversal) or a multipart upload (`file` + `confirm=RESTORE`, staged to a temp file that is removed with the request; 1 GiB cap). Validation runs before anything is touched: readable SQLite image, the four core tables present, and a decrypt probe on one nickname — a snapshot written under a different `ENCRYPTION_KEY` → 400 (`store.ErrInvalidBackup`). All rows then move in one transaction (only columns shared by both schemas are copied, so older snapshots tolerate newer columns); on failure the live data is untouched.
- **Consequences**: sessions recorded inside the snapshot come back to life; every token minted after it dies — usually including the acting admin's. The restored event log gains one `db.restore` entry recording the actor, the restored file and the safety file. The Settings UI (English-only, like the whole portal) lists server-side snapshots with per-row Restore, accepts uploads, and requires typing RESTORE — mirroring the reset flow.

## Live profile socket

- `/api/v1/ws/player` (`internal/handler/wsplayer.go`) is a per-player **websocket** that keeps every open tab of a player in sync without a page refresh. One tab = one socket, multiple tabs each hold one; the Vite dev proxy already forwards it (`ws: true` on `/api`).
- **Protocol** mirrors the room socket's `{type, data}` envelope: the first client message must be `{"type":"auth","data":{"token":...}}` (token in the payload, never in the URL) within 10s or the socket is closed; an unknown/dead/banned token gets one `error` message then a close. After auth the server sends a full `player` snapshot (`{"type":"player","data":{"player":<player JSON>}}`), then relays presence pushes live: `player` events carry a complete fresh profile, `banned` (`{"reason"}`) announces a fresh ban, `deleted` announces the account is gone — the client drops the dead session in both cases — and `boost` (`{"boosts": {…}}`, the full active view) announces a server-wide boost change.
- **Publish points** (`api.notifyPlayer` / `api.notifyPlayerBanned` / `api.notifyPlayerDeleted` in `internal/handler/notify.go`, nil-broker tolerant): every admin player mutation (coins, EXP, tier, grant/revoke, rename, stat reset) pushes `player`; a fresh ban pushes `banned`; deletes push `deleted`; plus self-service rename and delete from another tab. Solo hand payouts don't publish — their REST responses already carry the fresh profile inline.
- **Transport discipline**: fan-out goes through `internal/presence`; delivery is best-effort and never blocks the mutating handler — a socket whose buffer (16) overflows is kicked and left to resync through reconnect. Timings and origin policy mirror the room socket (10s write deadline, 60s pong wait, 45s ping period, shared `ws.CheckOrigin`).
- **Client** (`web/src/realtime.js`, wired once from `main.js`): applies `player` events to the reactive `currentPlayer` ref (bumping `playerIdentityVersion` on renames), acts on `banned`/`deleted` by raising the global ban dialog (`BannedModal.vue`, mounted from `App.vue`; state lives in `auth.js` as `banNotice`/`markBanned`) and signing out, and reconnects with full-jitter exponential backoff hard-capped at 30s. The backoff resets **only after a socket has stayed up ≥10s** — a socket that dies instantly must never loop at full speed (that once hammered the endpoint and tripped the shared 200 req/min rate limit). Reconnects eagerly on `online`/visibility regain. Every ban encounter surfaces through that same dialog: a live `banned` push; a reconnect auth rejected with `error` `"account banned"`; a `restoreSession` against a banned token; and a sign-in attempt — `POST /auth/google` 403s with the same message and `signInWithGoogle` raises the dialog instead of an inline error line.

## Account deletion

- Players delete their own account through `DELETE /api/v1/me` (`store.DeleteOwnPlayer`), **Google accounts only** — guests are rejected with 403 (`google sign-in required`, same gate style as the skins endpoints). The body must confirm with `{"confirm":"DELETE"}` — the exact word, mirroring the admin DB reset guard; anything else → 400.
- Deletion is a **hard delete**: the `players` row goes and every dependent table cascades (`player_mode_stats`, `rounds`, `player_achievements`, `player_skins`), so profile, EXP/level, coins, achievements, owned skins and full round history all vanish irrecoverably. The session token dies with the row — the next authenticated call 401s.
- The audit trail distinguishes paths: self-service logs `player.delete` with actor `player:<id>`, the admin portal keeps `admin.delete` (shared helper `store.deletePlayer`).
- UI entry: the profile menu's danger item → `DeleteAccountModal.vue`, which warns that everything is permanently erased and unrecoverable and requires typing DELETE; the modal links to `/privacy`. On success the client clears the session and re-opens the entry flow.
- **Guest profile menu** (`ProfileMenu.vue`): guests get a reduced menu — only rename plus a Google sign-in row with the `guestMenuHint` nudge. The achievements and skins links and the delete action are hidden (`isGuest` gate), mirroring the server gates.
- The legal copy describes the self-service deletion: Privacy Policy ("Retention & deletion") and Terms ("Player accounts") in `web/src/i18n/legal.js` — bump `OPERATOR.updated` when touching legal copy.

## Card Skins

- Catalog of **12 card skins** in `internal/skins` (`skins.Catalog`): `classic` (free default) up to `inferno` (6500 coins). Rarity bands: common / rare / epic / legend.
- **Google-gated**: buying and equipping require a non-guest player (`store.ErrGoogleRequired` → HTTP 403). Guests see the shop but cannot purchase or equip; the UI shows the Google sign-in gate.
- A skin is a **visual theme plus a signature animation stack** — per-skin custom properties (`--skin-face`, `--skin-ink`, `--skin-red`, `--skin-border`, `--skin-glow` for the selection ring) scoped by a `.skin-<id>` class on the game board; `CardTile.vue` consumes them with classic-looking fallbacks. Premium skins layer three levels of effects:
  - **CSS ambience** (neon flicker, gold sheen sweep, sakura petals, …) in `web/src/skins.css`, mirrored on the shop's `.pcard` previews.
  - **A pointer-tracked holographic foil** — a real `.holo` element inside every card/preview whose gradients follow the cursor through the registered `--mx`/`--my` custom props (per-skin foil backgrounds in `skins.css`; classic/mono keep it at opacity 0). Follow-the-pointer only runs for fine pointers without reduced motion; everyone else sees the static sheen.
  - **A three.js scene pair** (`web/src/three/skinScenes.js`, mounted by `SkinFx.vue`) with **two canvases**: an ambience scene behind the cards (one GPU-shader scene per skin — galaxy spiral disc with differential rotation, inferno ember storm (drifting sparks over a breathing molten glow), midnight aurora + moon, ocean god rays, neon synthwave grid with scheduled glitch tears, …, all with damped pointer parallax) and a **celebration scene above the cards**: `GameBoard` calls `SkinFx.burst(x, y, strength)` at every card merge (`strength 1` when the merge makes 24), firing the skin's shockwave ring + core flash + spark shower over the board.
  - The mobile profile sheet's golden particle halo (`three/profileHalo.js` + `ProfileHalo.vue`) follows the same GameBackdrop discipline (dynamic import, qualityGuard, hidden-tab pause, full dispose); its layout math (`ringPositions` / `sparkField` / `advanceSparks`) is pure and pinned by `three/profileHalo.test.mjs`.
  - The shop intentionally stays CSS-only (a WebGL context per preview would blow the browser's context limit). No gameplay effect; all motion is gated behind `prefers-reduced-motion` (bursts included), and the three.js layers follow the `GameBackdrop` discipline (dynamic import, qualityGuard, hidden-tab pause, full dispose). Scene layout math (`spiralLayout`, `circleLayout`, `glintCurve`, `burstFade`) is pure and pinned by `web/src/three/skinScenes.test.mjs`.
- Buying debits `players.total_coins` in one transaction with the ownership insert; equipping writes `players.skin` and requires prior ownership. `classic` is always equippable and never purchasable.

### API surface

- `GET /api/v1/skins` — public catalog + (with token) `owned` ids (classic always first) and `equipped` id.
- `POST /api/v1/skins/{id}/buy` — 200 `{player}`; errors: 403 guest, 400 insufficient coins/unknown, 409 already owned, 404 unknown skin.
- `POST /api/v1/skins/{id}/equip` — 200 `{player}`; errors: 403 guest, 400 not owned, 404 unknown skin.
