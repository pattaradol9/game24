<script setup>
// Every round ever dealt, newest first, filterable by mode and status.
import { inject, onMounted, ref, watch } from 'vue'
import { adminApi } from '../../admin.js'
import { MODES } from '../../modes.js'
import AdminPager from './AdminPager.vue'
import { fmt, fmtDate, fmtDuration } from './format.js'

const onAuthFail = inject('onAdminAuthFail', () => {})
const LIMIT = 25
const rounds = ref([])
const total = ref(0)
const offset = ref(0)
const loading = ref(false)
const error = ref('')
const mode = ref('')
const status = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await adminApi.rounds({ mode: mode.value, status: status.value, limit: LIMIT, offset: offset.value })
    rounds.value = data.rounds ?? []
    total.value = data.total ?? 0
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch([mode, status], () => {
  offset.value = 0
  load()
})
function page(newOffset) {
  offset.value = newOffset
  load()
}
</script>

<template>
  <div class="rd">
    <div class="controls">
      <select v-model="mode" class="field sel">
        <option value="">All modes</option>
        <option v-for="m in MODES" :key="m.id" :value="m.id">{{ m.id }}</option>
      </select>
      <select v-model="status" class="field sel">
        <option value="">All statuses</option>
        <option value="open">open</option>
        <option value="solved">solved</option>
        <option value="skipped">skipped</option>
        <option value="expired">expired</option>
      </select>
    </div>

    <p v-if="error" class="err">{{ error }}</p>

    <div class="panel block">
      <div class="twrap">
        <table class="t">
          <thead>
            <tr>
              <th>Dealt</th><th>Player</th><th>Mode</th><th>Status</th>
              <th class="num">Points</th><th class="num">Time</th><th class="num">Hints</th><th>Finished</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !rounds.length"><td colspan="8" class="dim center">Loading…</td></tr>
            <tr v-else-if="!rounds.length"><td colspan="8" class="dim center">No rounds match.</td></tr>
            <tr v-for="r in rounds" :key="r.id">
              <td class="dim">{{ fmtDate(r.dealtAt) }}</td>
              <td class="who">
                <b>{{ r.nickname || '—' }}</b>
                <span class="sub">{{ r.playerId }}</span>
              </td>
              <td class="cap">{{ r.mode }}</td>
              <td><span class="tag" :class="r.status">{{ r.status }}</span></td>
              <td class="num">{{ fmt(r.points) }}</td>
              <td class="num">{{ fmtDuration(r.elapsedMs) }}</td>
              <td class="num">{{ r.hintsUsed }}</td>
              <td class="dim">{{ fmtDate(r.finishedAt) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <AdminPager :offset="offset" :limit="LIMIT" :total="total" :loading="loading" @page="page" />
    </div>
  </div>
</template>

<style scoped>
.rd { display: flex; flex-direction: column; gap: 14px; }
.controls { display: flex; gap: 10px; }
.sel { width: 160px; }
.err { color: var(--bad); font-size: 0.88rem; }

.block { padding: 6px 18px 16px; }
.twrap { overflow-x: auto; }
.t { width: 100%; border-collapse: collapse; font-size: 0.86rem; }
.t th {
  text-align: left; font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text-mute); font-weight: 600; padding: 10px 10px 8px; border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.t td { padding: 9px 10px; border-bottom: 1px solid var(--line-soft); }
.t tr:last-child td { border-bottom: none; }
.t .num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
.dim { color: var(--text-mute); font-size: 0.78rem; font-weight: 400; white-space: nowrap; }
.center { text-align: center; }
.cap { text-transform: capitalize; }

.who { display: flex; flex-direction: column; gap: 2px; }
.who b { font-weight: 500; }
.who .sub { font-family: var(--font-mono); font-size: 0.68rem; color: var(--text-mute); }

.tag {
  display: inline-block; font-size: 0.68rem; font-weight: 600; letter-spacing: 0.04em;
  text-transform: uppercase; border-radius: var(--r-full); padding: 2px 9px;
  border: 1px solid transparent;
}
.tag.solved { color: var(--good); background: rgba(63, 191, 131, 0.1); }
.tag.skipped, .tag.expired { color: var(--bad); background: rgba(239, 95, 95, 0.1); }
.tag.open { color: var(--info); background: rgba(106, 165, 240, 0.1); }
</style>
