// Fake Google Identity Services for E2E.
//
// Two halves:
//  - mintToken(): signs a real RS256 ID token with the keypair the deps
//    orchestrator published; the server verifies it against the local JWKS
//    (GOOGLE_JWKS_URL), so POST /auth/google runs its genuine code path.
//  - GSI_STUB: a browser-side stand-in for the accounts.google.com/gsi/client
//    script. It implements the two calls the app makes (initialize /
//    renderButton) and answers a click with the credential a test deposited
//    in window.__g24NextCredential — no Google iframe, no network.
import { createSign } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const helpersDir = path.dirname(fileURLToPath(import.meta.url))
const e2eDir = path.resolve(helpersDir, '..')
const keyPath = path.join(e2eDir, '.tmp', 'jwks-key.json')

export const GOOGLE_CLIENT_ID = 'e2e-game24-client-id'

let cached = null

/** The JWKS keypair the orchestrator wrote. Retries until deps are up. */
async function loadKey() {
  if (cached) return cached
  const deadline = Date.now() + 30_000
  for (;;) {
    try {
      cached = JSON.parse(await readFile(keyPath, 'utf8'))
      return cached
    } catch {
      if (Date.now() > deadline) throw new Error('fake google: jwks key file never appeared')
      await new Promise((r) => setTimeout(r, 250))
    }
  }
}

const b64url = (buf) => Buffer.from(buf).toString('base64url')

/** Mint a Google-shaped ID token for a synthetic account. */
export async function mintToken({ sub, email, name, picture = '' }) {
  const { kid, privatePkcs8 } = await loadKey()
  const header = b64url(JSON.stringify({ alg: 'RS256', kid, typ: 'JWT' }))
  const now = Math.floor(Date.now() / 1000)
  const claims = b64url(JSON.stringify({
    sub,
    email,
    name,
    picture,
    aud: GOOGLE_CLIENT_ID,
    iss: 'https://accounts.google.com',
    iat: now,
    exp: now + 3600,
  }))
  const signer = createSign('RSA-SHA256')
  signer.update(`${header}.${claims}`)
  const sig = signer.sign(privatePkcs8)
  return `${header}.${claims}.${b64url(sig)}`
}

/**
 * Injected before every page load: provides the `window.google.accounts.id`
 * surface auth.js renders its button through. renderButton plants a real
 * <button> that fills its container, so Playwright clicks land naturally.
 */
export const GSI_STUB = `
  window.__g24NextCredential = '';
  window.google = {
    accounts: {
      id: {
        initialize({ callback }) { window.__g24GoogleCallback = callback },
        renderButton(el) {
          if (!el) return false
          const b = document.createElement('button')
          b.type = 'button'
          b.textContent = 'g-stub'
          b.style.cssText = 'width:100%;height:100%;min-height:44px;cursor:pointer'
          b.addEventListener('click', () => {
            if (window.__g24GoogleCallback) window.__g24GoogleCallback({ credential: window.__g24NextCredential })
          })
          el.replaceChildren(b)
          return true
        },
      },
    },
  }
`

/**
 * Block the real GSI script tag (index.html loads it async) so it can never
 * clobber the stub above when the sandbox happens to have network access.
 */
export async function blockGoogleScript(context) {
  await context.route('**://accounts.google.com/gsi/client', (route) =>
    route.fulfill({ status: 200, contentType: 'text/javascript', body: '/* stubbed for e2e */' })
  )
}

/** Deposit the credential the stub button will hand to the app on click. */
export async function stageCredential(page, account) {
  const token = await mintToken(account)
  await page.evaluate((t) => { window.__g24NextCredential = t }, token)
  return token
}
