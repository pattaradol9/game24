# 24 Game 🎴

The classic math game **"24 Game"** reimagined as a cartoon-style web game — draw 4 cards, combine them with `+ − × ÷` into a single expression that equals **24**. Play solo to grind EXP and climb tiers, or create a room and race your friends in real time.

## Features

- **Single Player** — timed hands, streaks, hints, skip (reveals the solution), EXP per difficulty mode
- **Multiplayer Room** — 6-digit room codes, shareable invite links, everyone races the same hand, first correct answer wins the round; configurable match length (12/24/36/48 rounds) and hint quotas (equal for all); end-of-match podium summary (round winners still earn real EXP)
- **4 difficulty modes** — the solver guarantees the quality of every dealt hand:
  | Mode | Numbers | Solution constraint | Time | EXP |
  |---|---|---|---|---|
  | JACK | 1–10 | integer-only solutions | 120s | ×1 |
  | QUEEN | 1–13 | classic | 90s | ×2 |
  | KING | 1–13 | requires intermediate fractions | 75s | ×3 |
  | ACE | 1–19 | fractions + unique solution only | 60s | ×5 |
- **Tier + Level system** (Sign in with Google only)
  - Level: cumulative EXP = `30×(n−1)×n` → Lv.100 = 297,000 EXP
  - Tier every 20 levels: Bronze / Silver (Lv.20) / Gold (Lv.40) / Platinum (Lv.60) / Diamond (Lv.80) / Master (Lv.100)
  - Tiers grant **bonus hints in Single**: Gold/Platinum +1, Diamond/Master +2 (Bronze/Silver = 1 per hand)
  - Anonymous players (enter a nickname) can play every mode but earn no statistics
- **Per-mode leaderboards** — signed-in players only (guests don't appear)
- **Anti-cheat** — the server deals every hand, records the deal time, and always verifies the submitted calculation trace server-side using exact fraction arithmetic
- **Player data encryption** — AES-256-GCM under a 32-byte secret key (injected via `ENCRYPTION_KEY`, with the real key kept in **Google Secret Manager**) — a fresh random nonce per message, column purpose bound as associated data (prevents swapping ciphertexts between columns), fail-closed on a missing or malformed key · Encrypted columns: token, email, google_sub, nickname, picture · Lookups go through hashes (`token_hash`, `sub_hash` + a salt derived from the same key, so no secret of any kind lives in the DB)
- **Admin portal** (`/admin`, back-office) — **email-allowlist access**: admins sign in with Google like any player; while their email is on `ADMIN_EMAILS` the game shows an "Admin portal" shortcut in the profile menu and their session unlocks the portal. Dashboard counters (players, rounds, EXP, per-mode stats, 14-day signup chart), player administration (search by nickname/email/id, ban with reason, unban, rename, delete, EXP grant/revoke that keeps `total_exp = Σ mode exp`, per-mode/full stat resets), full leaderboards including banned/guest rows, a global rounds log and an event/audit log of every lifecycle + admin action (with the acting admin recorded)

## Structure

```
web/     Vue 3 + Vite + vue-router (cartoon SPA, Thai/English UI, light/dark themes)
server/  Go (chi + gorilla/websocket + modernc.org/sqlite + stdlib AES-GCM)
         cmd/server + internal/{game,progress,crypto,secretmanager,store,room,ws,auth,handler,httpserver,webui,config}
```

The build produces a **single binary** with the SPA embedded (`go:embed`) serving on one port.

## Getting started (development)

```bash
# terminal 1: API server (needs ENCRYPTION_KEY in .env — create with openssl rand -base64 32)
cd server && go run ./cmd/server

# terminal 2: web dev server (proxies /api -> :8080)
cd web && npm install && npm run dev
```

Open http://localhost:5173

## Build & Deploy

```bash
make build          # web build -> embed -> single binary server/game24-server
./server/game24-server
```

Or the whole stack with Docker:

```bash
docker build -t game24 .
docker run --rm -p 8080:8080 -v game24-data:/data game24
```

### Environment variables (see `.env.example` for all)

| Variable | Default | Meaning |
|---|---|---|
| `APP_PORT` | 8080 | Service port |
| `DB_PATH` | data/game24.db | SQLite file |
| `ENCRYPTION_KEY` | — | 32-byte secret key (base64 or hex) for AES-256-GCM — use directly for local dev (generate with `openssl rand -base64 32`) |
| `SECRETMANAGER_ENCRYPTION_KEY` | — | When set, the service pulls the key from **Google Secret Manager** and it overrides `ENCRYPTION_KEY` — accepts `projects/PROJECT/secrets/NAME[/versions/V]` or a bare secret name (project from `GOOGLE_CLOUD_PROJECT`) |
| `GOOGLE_OAUTH_CLIENT_ID` | — | OAuth Web client id; empty = Google sign-in disabled (guest play only) |
| `ADMIN_EMAILS` | — | CSV allowlist of Google emails that may use the admin portal at `/admin`; empty = admin API disabled (all `/api/v1/admin/*` routes 404) |
| `CORS_ORIGINS` | http://localhost:5173 | Allowed origins |
| `PUBLIC_URL` | — | Canonical site origin (e.g. `https://game24.example.com`) used for `robots.txt`, `sitemap.xml` and absolute SEO tags; empty = canonical/OG URLs are derived per-request from the Host header, `robots.txt` blocks indexing entirely, and `sitemap.xml` 404s |

### Set up Sign in with Google

1. Create an OAuth Client (type: **Web application**) in Google Cloud Console
2. Authorized JavaScript origins: `http://localhost:5173`, `http://localhost:8080` (+ your production origin)
3. Put the client id in `GOOGLE_OAUTH_CLIENT_ID`

### Set up the secret key + Google Secret Manager (production)

The service uses AES-256-GCM with a 32-byte secret key. There are two ways to wire the key in production (pick one):

**Option A — let the service pull from Secret Manager (recommended):** set `SECRETMANAGER_ENCRYPTION_KEY`; the fetched value automatically overrides `ENCRYPTION_KEY`

1. Create the key and store it in Secret Manager (save the output to a file first):
   ```bash
   openssl rand -base64 32 | tee ./key.txt
   gcloud secrets create game24-encryption-key --data-file=./key.txt --replication-policy=automatic
   rm ./key.txt   # don't leave it lying around
   ```
   Keep an offline backup of the key somewhere safe — **losing the key = losing all encrypted player data**
2. Enable the Secret Manager API and grant the runtime's service account access (Cloud Run/GKE/GCE = the service's attached service account; outside GCP = set `GOOGLE_APPLICATION_CREDENTIALS` to a service account file — a non-empty value in `.env` wins over any shell export, see `.env.example`):
   ```bash
   gcloud services enable secretmanager.googleapis.com
   gcloud projects add-iam-policy-binding PROJECT \
     --member=serviceAccount:RUNTIME_SA --role=roles/secretmanager.secretAccessor
   ```
3. Set the env for the service:
   ```bash
   SECRETMANAGER_ENCRYPTION_KEY=projects/PROJECT/secrets/game24-encryption-key
   ```
   With a bare secret name (`game24-encryption-key`) you must also set `GOOGLE_CLOUD_PROJECT=PROJECT`

**Option B — inject it directly as an env var:** fetch the value at deploy time into `ENCRYPTION_KEY`, e.g. Cloud Run:
```bash
gcloud run deploy game24 --image=... \
  --set-secrets=ENCRYPTION_KEY=game24-encryption-key:latest
```
Or Docker/VM: `gcloud secrets versions access latest --secret=game24-encryption-key` and pass it as an env var to the container

**Local dev needs no GCP at all** — just put `ENCRYPTION_KEY` in `.env`

If the key is missing / malformed / cannot be fetched from Secret Manager at startup, the service **refuses to start** (fail-closed) — running instances are unaffected

## PWA & SEO

The SPA ships as a full **PWA** (via `vite-plugin-pwa`, workbox `generateSW`):

- **Web app manifest** generated from `web/vite.config.js` (Thai name, standalone display, dark navy theme, 192/512 + maskable PNG icons in `web/public/` — regenerate with ImageMagick from the `favicon.svg` artwork if the brand changes)
- **Service worker** (`/sw.js`) auto-updates and precaches the whole app shell (49 entries) so the game boots offline; Google Fonts are cached at runtime; `/api/` requests are never served from cache (game state must stay fresh)
- **Installable** on Android/desktop (Chrome) and addable to the home screen on iOS (`apple-touch-icon` + iOS meta tags in `web/index.html`)
- In dev (`npm run dev`) the SW is enabled too (`devOptions`); unregister it in DevTools → Application if it interferes

**SEO** is handled on three layers:

1. **Static defaults** in `web/index.html` — Thai title/description, canonical, Open Graph + Twitter cards (with a generated 1200×630 `og-card.png`), `WebApplication` JSON-LD, theme-color
2. **Server-side rewriting** (`server/internal/httpserver/seo.go`) — the embedded-binary SPA handler rewrites title/description/canonical/OG/Twitter/robots per route on the first HTML response, so crawlers that don't run JavaScript (Facebook/Line/Twitter link previews) still get correct cards. Shared room links render a "เข้าร่วมห้อง CODE" card with `noindex`
3. **Client-side sync** (`web/src/seo.js`, wired in `web/src/main.js` route meta) — every SPA navigation updates title, description, canonical, OG/Twitter tags and `<html lang>` from the per-route bilingual meta table

`robots.txt` and `sitemap.xml` are **generated dynamically by the Go server** (not static files) because they need absolute URLs:

- With `PUBLIC_URL` set: robots allows all public routes but disallows `/admin`, `/room/`, `/api/`, and points at `PUBLIC_URL/sitemap.xml` (fixed public routes: `/`, `/solo`, `/leaderboard`, `/privacy`, `/terms`)
- Without `PUBLIC_URL`: `robots.txt` returns `Disallow: /` (staging/preview origins stay unindexed) and `sitemap.xml` returns 404

After deploying to the production domain: set `PUBLIC_URL`, then verify the property in Google Search Console and submit `/sitemap.xml` (see `docs/analytics-gsc-adsense-plan.md` Phase 2).

## Testing

```bash
make test   # go test ./... + web core (node --test)
make lint   # go vet + gofmt
```

## Reset the database

Resetting wipes **everything**: players, statistics, rounds and the event log. The schema is recreated empty on the next start.

**From the admin portal (any environment):** Admin Portal → **Settings** → *Download backup* first, then type `RESET` to confirm. The server always writes one automatic snapshot (`game24-backup-<timestamp>.db` next to the DB file) before wiping, and every session — including the admin's — is signed out. To restore, stop the server, replace the DB file with the backup, start again.

**By hand (development):**

```bash
# stop the server first, then:
rm -f server/data/game24.db server/data/game24.db-wal server/data/game24.db-shm
make dev    # fresh database created automatically by the migrations
```

**Production checklist before resetting:**

1. **Back up first** — either download a snapshot from Admin → Settings, or on the host run
   `sqlite3 data/game24.db "VACUUM INTO 'backup.db'"` (a plain file copy while the server runs can miss data sitting in the WAL)
2. **Keep the ENCRYPTION_KEY safe** — a backup contains encrypted PII; it is only restorable together with the same key (losing the key makes every backup permanently unreadable; keeping the backup *next to* the key reintroduces the single-leak risk the encryption exists to avoid)
3. Expect **all players to be signed out** and anonymous progress/exp to be gone — announce it if anyone depends on it
4. **Docker:** the DB lives in the mounted volume (`-v game24-data:/data`) — copy the backup out of the volume (`docker cp`) before removing the volume, otherwise it disappears with the container data
5. After the reset, sign in with Google again — accounts (including allowlisted admins) are recreated on first sign-in

## API summary (prefix `/api/v1`)

| Method | Path | Description |
|---|---|---|
| POST | `/players` | Register guest {nickname} → player + token |
| POST | `/auth/google` | Sign in {credential} → player + token |
| GET | `/me` | Own stats + level/tier + progress |
| POST | `/rounds` | Deal a hand {mode} (time limit + tier-based hint quota) |
| POST | `/rounds/{id}/submit` | Submit trace {steps} — server verifies + awards EXP |
| POST | `/rounds/{id}/skip` | Skip (resets streak) + reveal solution |
| POST | `/rounds/{id}/hint` | Use hint (reveals first step) |
| GET | `/leaderboard?mode=` | Per-mode board (signed-in players only) |
| POST | `/rooms` | Create room {mode, rounds, hintQuota, regenQuota} → code + hostKey |
| WS | `/ws/room/{code}` | join/start/submit/hint/regen/leave |

Admin API (prefix `/api/v1/admin`, authenticated with a normal player Bearer token whose Google email is on `ADMIN_EMAILS`; 404 for everything while the allowlist is empty):

| Method | Path | Description |
|---|---|---|
| GET | `/admin/session` | Validate the admin token |
| GET | `/admin/overview` | Dashboard counters (players/rounds/EXP/modes/signups) |
| GET | `/admin/players?search=&filter=&limit=&offset=` | Player list + search (nickname/email/id) |
| GET | `/admin/players/{id}` | Player detail incl. per-mode stats |
| GET | `/admin/players/{id}/rounds` | That player's rounds |
| PATCH | `/admin/players/{id}` | Rename {nickname} and/or ban/unban {banned, banReason} |
| DELETE | `/admin/players/{id}` | Delete player (cascades stats + rounds) |
| POST | `/admin/players/{id}/exp` | Adjust mode EXP {mode, delta} — total resyncs |
| POST | `/admin/players/{id}/stats/reset` | Reset stats {mode?} — omitted mode = all |
| GET | `/admin/leaderboard?mode=` | Full board incl. banned/guest flags |
| GET | `/admin/rounds?mode=&status=` | Global rounds log |
| GET | `/admin/events?action=` | Event/audit log |
| GET | `/admin/settings` | DB overview (row counts + size on disk) |
| GET | `/admin/db/backup` | Download a consistent DB snapshot (SQLite image) |
| POST | `/admin/db/reset` | Wipe the database — body `{"confirm":"RESET"}`; auto-snapshots to `game24-backup-<ts>.db` beside the DB first, then signs out every session |

Every response is an envelope: `{success, message, data}`

## Security notes

- The encryption key lives entirely outside the service (Google Secret Manager or env) — no key/salt/keystore in the DB or repo; a leaked DB alone cannot decrypt PII
- Admin portal: access via Google sign-in + `ADMIN_EMAILS` allowlist (case-insensitive; removing an email revokes admin on their next request; banned admins are locked out too), admin API entirely disabled while the allowlist is empty; banning revokes access on the player's very next request (token **and** Google sign-in) and hides the player from public leaderboards; the event log stores actor player IDs and admin reasons only — no decrypted PII lands in it
- `SECRETMANAGER_ENCRYPTION_KEY` takes priority over `ENCRYPTION_KEY` — if the fetch fails at startup the service **refuses to start** (fail-closed, no silent fallback to the old key); running instances are unaffected
- Data encrypted under the previous scheme (Tink/KMS) cannot be decrypted with the new key — in dev, delete `data/game24.db` and start fresh
- TLS should be terminated at a reverse proxy (nginx) in front of the service
- Key rotation: create a new secret version and re-encrypt old data — future work
