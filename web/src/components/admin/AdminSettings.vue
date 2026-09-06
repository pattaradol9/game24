<script setup>
// Settings: database overview, snapshot download, and the full-database
// reset. A reset always snapshots to a backup file on the server first and
// signs out every session — including this admin's.
import { inject, onMounted, ref } from 'vue'
import { adminApi, downloadDbBackup } from '../../admin.js'
import { fmt, fmtDate } from './format.js'

const onAuthFail = inject('onAdminAuthFail', () => {})
const settings = ref(null)
const loading = ref(true)
const error = ref('')
const notice = ref('')
const busy = ref(false)
const confirmText = ref('')
const resetDone = ref(null)

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
  } finally {
    loading.value = false
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

      <section class="panel block danger">
        <h2>Reset database</h2>
        <p class="hint">
          Drops <b>everything</b>: players, stats, rounds and the event log. The server keeps one
          automatic snapshot (<code>game24-backup-&lt;timestamp&gt;.db</code> next to the database) before
          wiping, and every session — including yours — is signed out. There is no undo beyond that
          backup file.
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
.done { display: flex; flex-direction: column; gap: 8px; }
.done code { font-family: var(--font-mono); font-size: 0.78rem; color: var(--text-dim); }
</style>
