<script setup>
// Settings: database overview, snapshot download, restore from a backup
// (server-side safety snapshot or an uploaded one), and the full-database
// reset. Resets and restores always keep a safety snapshot on the server
// first; both sign out every session the backup does not contain — usually
// including this admin's.
import { inject, onMounted, ref } from 'vue'
import { adminApi, downloadDbBackup, restoreDbUpload } from '../../admin.js'
import { fmt, fmtDate } from './format.js'

const onAuthFail = inject('onAdminAuthFail', () => {})
const settings = ref(null)
const backups = ref([])
const loading = ref(true)
const error = ref('')
const notice = ref('')
const busy = ref(false)
const confirmText = ref('')
const restoreConfirmText = ref('')
const resetDone = ref(null)
const restoreDone = ref(null)
const pendingRestore = ref('') // server-side backup awaiting confirmation
const pendingUpload = ref(null) // uploaded file awaiting confirmation

const fmtSize = (bytes) => {
  if (!bytes) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    settings.value = await adminApi.settings()
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
    loading.value = false
    return
  } finally {
    loading.value = false
  }
  loadBackups()
}

async function loadBackups() {
  try {
    backups.value = (await adminApi.listDbBackups()).backups ?? []
  } catch {
    // informational only — the restore section degrades to upload-only
  }
}
onMounted(load)

async function backup() {
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    const name = await downloadDbBackup()
    notice.value = `Backup downloaded: ${name}`
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}

function askRestore(name) {
  pendingRestore.value = name
  pendingUpload.value = null
  restoreConfirmText.value = ''
}

function onUploadChosen(ev) {
  const file = ev.target.files?.[0]
  ev.target.value = ''
  if (!file) return
  pendingUpload.value = file
  pendingRestore.value = ''
  restoreConfirmText.value = ''
}

function cancelRestore() {
  pendingRestore.value = ''
  pendingUpload.value = null
  restoreConfirmText.value = ''
}

async function runRestore() {
  busy.value = true
  error.value = ''
  try {
    const data = pendingUpload.value
      ? await restoreDbUpload(pendingUpload.value)
      : await adminApi.restoreDb(pendingRestore.value)
    restoreDone.value = data
    cancelRestore()
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}

async function reset() {
  busy.value = true
  error.value = ''
  try {
    const data = await adminApi.resetDb()
    resetDone.value = data
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="st">
    <p v-if="loading" class="loading">Loading…</p>
    <p v-if="error" class="err">{{ error }}</p>

    <template v-if="settings">
      <section class="panel block">
        <h2>Database</h2>
        <div class="kv">
          <span class="k">File</span><span class="v mono">{{ settings.dbFile }}</span>
          <span class="k">Size on disk</span><span class="v">{{ fmtSize(settings.dbSize) }} <span class="dim">(WAL included)</span></span>
          <span class="k">Players</span><span class="v">{{ fmt(settings.players) }}</span>
          <span class="k">Rounds</span><span class="v">{{ fmt(settings.rounds) }}</span>
          <span class="k">Event log entries</span><span class="v">{{ fmt(settings.events) }}</span>
        </div>
        <div class="row">
          <button class="btn" :disabled="busy" @click="backup">Download backup</button>
          <span class="hint">Consistent snapshot (SQLite image). Player PII inside stays encrypted — store it as carefully as the database itself.</span>
        </div>
      </section>

      <section class="panel block">
        <h2>Restore backup</h2>
        <p class="hint">
          Replaces the live database with a snapshot. A safety snapshot of the current data is kept on
          the server first, and every session that is not inside the backup is signed out — usually
          including yours. Snapshots only decrypt with the current <code>ENCRYPTION_KEY</code>.
        </p>

        <div v-if="restoreDone" class="done">
          <p class="ok-msg">
            Restored from <code>{{ restoreDone.backupFile }}</code> —
            {{ fmt(restoreDone.restoredRows.players) }} players,
            {{ fmt(restoreDone.restoredRows.rounds) }} rounds. The previous state was kept as
            <code>{{ restoreDone.safetyBackup }}</code>.
          </p>
          <p class="hint">
            Sessions that were not inside that backup are signed out. If your admin session ended,
            sign in with Google again — this backup's sessions still work.
          </p>
          <div class="row">
            <button class="btn quiet" :disabled="busy" @click="load">Refresh lists</button>
          </div>
        </div>

        <template v-else>
          <div v-if="pendingRestore || pendingUpload" class="row confirm-row">
            <span class="hint">
              Restore <b class="mono">{{ pendingRestore || pendingUpload.name }}</b>?
              This overwrites every player, round and log entry with the snapshot's contents.
            </span>
            <input
              v-model="restoreConfirmText"
              class="field confirm"
              placeholder='Type "RESTORE" to confirm'
              autocomplete="off"
              spellcheck="false"
            />
            <button class="btn danger" :disabled="busy || restoreConfirmText !== 'RESTORE'" @click="runRestore">
              Restore
            </button>
            <button class="btn quiet" :disabled="busy" @click="cancelRestore">Cancel</button>
          </div>

          <template v-else>
            <table v-if="backups.length" class="bk">
              <thead>
                <tr><th>Backup file</th><th>Size</th><th>Written</th><th></th></tr>
              </thead>
              <tbody>
                <tr v-for="b in backups" :key="b.name">
                  <td class="mono">{{ b.name }}</td>
                  <td>{{ fmtSize(b.size) }}</td>
                  <td>{{ fmtDate(b.modified) }}</td>
                  <td class="ta-r">
                    <button class="btn quiet" :disabled="busy" @click="askRestore(b.name)">Restore</button>
                  </td>
                </tr>
              </tbody>
            </table>
            <p v-else class="hint">
              No server-side snapshots yet — one appears here after every reset or restore (each keeps
              a safety copy), or upload a snapshot you downloaded earlier.
            </p>

            <div class="row">
              <label class="btn quiet file-btn" :class="{ off: busy }">
                Upload snapshot…
                <input type="file" accept=".db,application/octet-stream" :disabled="busy" @change="onUploadChosen" />
              </label>
              <span class="hint">Pick a <code>.db</code> snapshot file to restore from.</span>
            </div>
          </template>
        </template>
      </section>

      <section class="panel block danger">
        <h2>Reset database</h2>
        <p class="hint">
          Drops <b>everything</b>: players, stats, rounds and the event log. The server keeps one
          automatic snapshot (<code>game24-backup-&lt;timestamp&gt;.db</code> next to the database) before
          wiping, and every session — including yours — is signed out. There is no undo beyond that
          backup file, which you can restore from this page once you are signed back in.
        </p>
        <div v-if="!resetDone" class="row">
          <input
            v-model="confirmText"
            class="field confirm"
            placeholder='Type "RESET" to confirm'
            autocomplete="off"
            spellcheck="false"
          />
          <button class="btn danger" :disabled="busy || confirmText !== 'RESET'" @click="reset">
            Reset database
          </button>
        </div>
        <div v-else class="done">
          <p class="ok-msg">
            Database wiped at {{ fmtDate(new Date().toISOString()) }} —
            backup kept on the server as <code>{{ resetDone.backupFile }}</code>.
          </p>
          <p class="hint">
            Your admin session ended with the reset. Sign in with Google again to continue —
            or reload the game page.
          </p>
        </div>
      </section>
    </template>

    <p v-if="notice" class="ok-msg pad">{{ notice }}</p>
  </div>
</template>

<style scoped>
.st { display: flex; flex-direction: column; gap: 16px; }
.loading { color: var(--text-mute); padding: 16px 4px; }
.err { color: var(--bad); font-size: 0.88rem; }
.ok-msg { color: var(--good); font-size: 0.88rem; }
.pad { padding: 0 4px; }

.block { padding: 20px 22px; display: flex; flex-direction: column; gap: 14px; }
.block h2 { font-size: 0.95rem; color: var(--text-dim); font-weight: 600; }
.block.danger { border-color: rgba(239, 95, 95, 0.35); }
.block.danger h2 { color: var(--bad); }

.kv { display: grid; grid-template-columns: 130px 1fr; gap: 8px 14px; font-size: 0.88rem; }
.kv .k { color: var(--text-mute); font-size: 0.76rem; text-transform: uppercase; letter-spacing: 0.05em; padding-top: 2px; }
.kv .v { min-width: 0; overflow-wrap: anywhere; }
.mono { font-family: var(--font-mono); font-size: 0.82rem; }
.dim { color: var(--text-mute); font-size: 0.78rem; font-weight: 400; }

.row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.row .btn { min-height: 42px; white-space: nowrap; }
.confirm { max-width: 240px; flex: none; }
.hint { font-size: 0.78rem; color: var(--text-mute); line-height: 1.5; flex: 1; min-width: 220px; }
.hint code { font-family: var(--font-mono); font-size: 0.72rem; color: var(--text-dim); }
.hint b { color: var(--text-dim); }
.done { display: flex; flex-direction: column; gap: 8px; }
.done code { font-family: var(--font-mono); font-size: 0.78rem; color: var(--text-dim); }

.bk { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
.bk th { text-align: left; font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-mute); font-weight: 600; padding: 4px 10px 8px 0; }
.bk td { padding: 8px 10px 8px 0; border-top: 1px solid var(--line); vertical-align: middle; }
.bk td.mono { font-family: var(--font-mono); font-size: 0.78rem; overflow-wrap: anywhere; }
.bk .ta-r { text-align: right; }

.file-btn { position: relative; cursor: pointer; display: inline-flex; align-items: center; min-height: 42px; }
.file-btn input { position: absolute; inset: 0; opacity: 0; cursor: pointer; }
.file-btn.off { opacity: 0.55; pointer-events: none; }
</style>
