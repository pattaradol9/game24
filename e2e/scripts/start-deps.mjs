// E2E dependency orchestrator — the Playwright "web server" for the API half.
//
// Responsibilities, in order:
//   1. wipe the throwaway SQLite database (fresh state every run),
//   2. generate the RSA keypair for the fake Google JWKS and publish it on
//      http://127.0.0.1:4801/certs (the private half is written to
//      .tmp/jwks-key.json so test processes can mint ID tokens),
//   3. build the Go server binary (always fresh — no stale-binary surprises),
//   4. run it with an isolated env: throwaway DB, test Google client id,
//      GOOGLE_JWKS_URL pointed at the local JWKS, admin API disabled,
//   5. wait for /healthz, then stay alive until Playwright SIGTERMs us.
import { spawn, execSync } from 'node:child_process'
import { createServer } from 'node:http'
import { generateKeyPairSync } from 'node:crypto'
import { rmSync, mkdirSync, writeFileSync, createWriteStream } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const e2eDir = path.resolve(scriptDir, '..')
const repoDir = path.resolve(e2eDir, '..')
const tmpDir = path.join(e2eDir, '.tmp')
const dbPath = path.join(tmpDir, 'game24-e2e.db')
const keyPath = path.join(tmpDir, 'jwks-key.json')
const serverBin = path.join(tmpDir, 'game24-server')
const logPath = path.join(tmpDir, 'server.log')

const PORT_API = Number(process.env.G24_E2E_API_PORT || 18080)
const PORT_JWKS = Number(process.env.G24_E2E_JWKS_PORT || 14801)
// any client id works: the fake tokens are minted against it and the server
// only checks that the token's aud matches this value
const CLIENT_ID = 'e2e-game24-client-id'
// 32 bytes, hex — a valid AES-256 key for the store's Crypter
const ENCRYPTION_KEY = '0e1e2e3e4e5e6e7e8e9eaebecedee0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1'

mkdirSync(tmpDir, { recursive: true })
for (const suffix of ['', '-wal', '-shm']) {
  try { rmSync(dbPath + suffix, { force: true }) } catch { /* fresh start is best-effort */ }
}

// ---------------------------------------------------------------------------
// fake Google JWKS
const { publicKey, privateKey } = generateKeyPairSync('rsa', { modulusLength: 2048 })
const kid = 'e2e-test-key-1'
const publicJwk = publicKey.export({ format: 'jwk' }) // { kty, n, e, ... }
writeFileSync(keyPath, JSON.stringify({ kid, privatePkcs8: privateKey.export({ format: 'pem', type: 'pkcs8' }) }))

const jwks = createServer((req, res) => {
  res.writeHead(200, { 'content-type': 'application/json' })
  res.end(JSON.stringify({ keys: [{ kid, kty: 'RSA', alg: 'RS256', use: 'sig', n: publicJwk.n, e: publicJwk.e }] }))
})
await new Promise((resolve) => jwks.listen(PORT_JWKS, '127.0.0.1', resolve))
console.log(`[start-deps] fake google jwks on http://127.0.0.1:${PORT_JWKS}/certs`)

// ---------------------------------------------------------------------------
// build + run the Go server
execSync(`go build -o ${JSON.stringify(serverBin)} ./cmd/server`, { cwd: path.join(repoDir, 'server'), stdio: 'inherit' })
console.log('[start-deps] server binary built')

const serverEnv = {
  ...process.env,
  APP_PORT: String(PORT_API),
  DB_PATH: dbPath,
  ENCRYPTION_KEY,
  SECRETMANAGER_ENCRYPTION_KEY: '', // never reach for Secret Manager in tests
  GOOGLE_OAUTH_CLIENT_ID: CLIENT_ID,
  GOOGLE_JWKS_URL: `http://127.0.0.1:${PORT_JWKS}/certs`,
  ADMIN_EMAILS: '', // admin API disabled — out of scope for this suite
  // the browser bursts well past 200 req/min across parallel workers
  RATE_LIMIT_PER_MIN: '1000000',
  CORS_ORIGINS: 'http://localhost:15173,http://127.0.0.1:15173',
}
const server = spawn(serverBin, [], { env: serverEnv, cwd: tmpDir })
const logStream = createWriteStream(logPath, { flags: 'a' })
server.stdout.pipe(logStream)
server.stderr.pipe(logStream)
server.on('exit', (code) => console.log(`[start-deps] server exited (${code})`))

async function healthy() {
  for (let i = 0; i < 120; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${PORT_API}/api/v1/healthz`)
      if (res.ok) return true
    } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 500))
  }
  return false
}
if (!(await healthy())) {
  console.error(`[start-deps] server did not become healthy — see ${logPath}`)
  server.kill('SIGKILL')
  process.exit(1)
}
console.log(`[start-deps] game24 api healthy on :${PORT_API} (db: ${dbPath})`)

// ---------------------------------------------------------------------------
// stay up until Playwright tears us down, then take the server with us
function shutdown() {
  server.kill('SIGTERM')
  jwks.close()
  // give the server a moment to flush, then leave; go run-style children are
  // not a concern here — we spawned the binary directly
  setTimeout(() => process.exit(0), 500)
}
process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
setInterval(() => {
  if (server.exitCode !== null) process.exit(1) // server died underneath us
}, 1000)
