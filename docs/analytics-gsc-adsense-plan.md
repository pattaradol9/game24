# Analytics, Search Console & AdSense Rollout Plan

Status as of 2026-09-07:

| Integration | Decision | When |
|---|---|---|
| Google Analytics 4 (GA4) | Approved — implement | Next release after a production domain exists |
| Google Search Console (GSC) | Approved — implement | Together with GA4 |
| Google AdSense | Deferred | Only after the site has sustained meaningful traffic; requires a separate privacy-policy addendum before going live |

The privacy policy (`web/src/i18n/legal.js`) has already been rewritten to
disclose GA4, so analytics can be switched on without another legal pass.
The footer tagline (`web/src/i18n/messages.js`, `footerTag` th/en) no longer
claims "no tracking" for the same reason; it still truthfully says "no ads"
because AdSense is deferred.

## Architecture facts that shape this plan

- The web app is a Vue 3 SPA (vue-router, history mode) built with Vite into
  `web/dist`, embedded into the Go binary via `go:embed`
  (`server/internal/webui/embed.go`) and served with an SPA fallback to
  `index.html` (`server/internal/httpserver/server.go`).
- Any file placed in `web/public/` lands in `dist/` root and is served at the
  site root — so `robots.txt`, `sitemap.xml`, `ads.txt` and GSC verification
  files need no Go changes.
- No web build-time env vars are used yet; analytics IDs should be passed as
  `VITE_*` env vars so dev/test builds stay script-free (no-op).
- Deployment is a single container (Cloud Run style — Secret Manager
  conventions in `.env.example`). A custom domain with HTTPS is a hard
  prerequisite for AdSense and strongly recommended for GSC.

## Phase 1 — GA4

1. Create a GA4 property + web data stream → Measurement ID `G-XXXXXXXXXX`.
2. Add `web/src/analytics.js` that dynamically injects gtag.js only when
   `import.meta.env.VITE_GA_MEASUREMENT_ID` is set (no-op otherwise).
3. Set Consent Mode v2 defaults in `analytics.js` (`denied` for
   ad_storage/analytics_storage until consent) — required groundwork for any
   future AdSense/EEA traffic, harmless for Thai traffic.
4. SPA page views: `router.afterEach()` in `web/src/main.js` →
   `gtag('event', 'page_view', { page_path, page_title })` on every route
   change (an SPA never reloads, so the default page_view won't fire again).
5. Business events worth measuring: `game_start`, `puzzle_solved` (with time
   and hints used), `room_join`, `sign_in`, `donate_click`.
6. In the GA dashboard: enable 14-month data retention, define internal
   traffic filter.

Files touched: `web/src/analytics.js` (new), `web/src/main.js`,
`.env.example` (document `VITE_GA_MEASUREMENT_ID`), Docker/CI build env.

## Phase 2 — GSC

1. Verify a **Domain property** via a DNS TXT record (preferred — no code
   change, covers all subdomains). Fallback: drop the `googleXXXX.html`
   verification file into `web/public/`.
2. Add `web/public/robots.txt`: allow all, `Disallow: /admin`,
   `Disallow: /room/`, `Sitemap: <origin>/sitemap.xml`.
3. Add `web/public/sitemap.xml` with the fixed routes `/`, `/solo`,
   `/leaderboard`, `/privacy`, `/terms` — exclude `/room/:code` (private) and
   `/admin`. Needs the production origin (absolute URLs), so it lands once the
   domain is confirmed.
4. Submit the sitemap in GSC, run URL Inspection on the key pages.
5. Follow-up (separate task, not a blocker): per-route `<title>` and meta
   description via a `useSeo()` composable — `index.html` currently has one
   hardcoded Thai title.

## Phase 3 — AdSense (DEFERRED until traffic justifies it)

Prerequisites before applying: stable traffic on the custom domain, all
Phase 1–2 work live.

Checklist for when it is picked up:

1. Update the privacy policy again (th + en in `web/src/i18n/legal.js`):
   remove the remaining "no ads" claims, add a Cookies & Advertising section
   disclosing AdSense/DoubleClick cookies and linking to
   `https://policies.google.com/technologies/ads` and
   `https://adssettings.google.com` (mandatory for AdSense approval), and
   drop "no ads" from `footerTag` in `messages.js`.
2. Apply for AdSense with the custom domain; add the `ca-pub-XXXX` script to
   `index.html` after approval.
3. Add `web/public/ads.txt`: `google.com, pub-XXXX, DIRECT, f08c47fec0942fa0`.
4. Place curated ad units only — no Auto Ads. Banner below content on Home and
   Leaderboard, plus a mobile anchor. **Never inside gameplay**
   (`/solo`, `/room/:code`): a number-grid game generates dense accidental
   clicks, which risks invalid traffic enforcement.
5. Enable Google's free consent experience ("Privacy & messaging") scoped to
   EEA/UK so Thai users never see a banner; it fires Consent Mode v2 for
   AdSense + GA together.
6. Operator discipline: never click own ads, never instruct players to click.

## Phase 4 — Wiring & QA

- Link GA4 ↔ GSC (GA Admin → Search Console links) and GA4 ↔ AdSense (when
  Phase 3 happens).
- Verify with GA DebugView, GSC URL Inspection, AdSense ad preview.
- Update README with the new build env vars.

## Open questions (blockers only for the go-live, not for coding)

1. Production domain — needed for `sitemap.xml`, GSC property and AdSense.
2. GA4 property / Measurement ID — code can be merged with a placeholder env
   var left empty (script stays a no-op).
3. AdSense account — revisit at Phase 3 kickoff.
