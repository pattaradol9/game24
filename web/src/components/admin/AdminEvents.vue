<script setup>
// Audit trail: player lifecycle events and every admin mutation, newest first.
import { inject, onMounted, ref, watch } from 'vue'
import { adminApi } from '../../admin.js'
import AdminPager from './AdminPager.vue'
import { fmtDate } from './format.js'

const onAuthFail = inject('onAdminAuthFail', () => {})
const LIMIT = 30
const events = ref([])
const total = ref(0)
const offset = ref(0)
const loading = ref(false)
const error = ref('')
const action = ref('')

const ACTIONS = [
  'player.create',
  'player.login.google',
  'player.rename',
  'admin.ban',
  'admin.unban',
  'admin.delete',
  'admin.exp.adjust',
  'admin.stats.reset',
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await adminApi.events({ action: action.value, limit: LIMIT, offset: offset.value })
    events.value = data.events ?? []
    total.value = data.total ?? 0
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(action, () => {
  offset.value = 0
  load()
})
function page(newOffset) {
  offset.value = newOffset
  load()
}

const actionClass = (a) => (a.startsWith('admin.') ? 'admin' : 'player')
const prettyDetail = (d) => {
  if (!d) return ''
  try {
    return JSON.stringify(JSON.parse(d))
  } catch {
    return d
  }
}
</script>

<template>
  <div class="ev">
    <div class="controls">
      <select v-model="action" class="field sel">
        <option value="">All actions</option>
        <option v-for="a in ACTIONS" :key="a" :value="a">{{ a }}</option>
      </select>
    </div>

    <p v-if="error" class="err">{{ error }}</p>

    <div class="panel block">
      <div class="twrap">
        <table class="t">
          <thead>
            <tr><th>Time</th><th>Actor</th><th>Action</th><th>Target player</th><th>Detail</th></tr>
          </thead>
          <tbody>
            <tr v-if="loading && !events.length"><td colspan="5" class="dim center">Loading…</td></tr>
            <tr v-else-if="!events.length"><td colspan="5" class="dim center">No events recorded.</td></tr>
            <tr v-for="e in events" :key="e.id">
              <td class="dim">{{ fmtDate(e.ts) }}</td>
              <td><span class="actor" :class="e.actor">{{ e.actor }}</span></td>
              <td><span class="tag" :class="actionClass(e.action)">{{ e.action }}</span></td>
              <td class="mono">{{ e.target || '—' }}</td>
              <td class="dim detail">{{ prettyDetail(e.detail) || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <AdminPager :offset="offset" :limit="LIMIT" :total="total" :loading="loading" @page="page" />
    </div>
  </div>
</template>

<style scoped>
.ev { display: flex; flex-direction: column; gap: 14px; }
.controls { display: flex; gap: 10px; }
.sel { width: 200px; }
.err { color: var(--bad); font-size: 0.88rem; }

.block { padding: 6px 18px 16px; }
.twrap { overflow-x: auto; }
.t { width: 100%; border-collapse: collapse; font-size: 0.84rem; }
.t th {
  text-align: left; font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text-mute); font-weight: 600; padding: 10px 10px 8px; border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.t td { padding: 9px 10px; border-bottom: 1px solid var(--line-soft); }
.t tr:last-child td { border-bottom: none; }
.dim { color: var(--text-mute); font-size: 0.78rem; font-weight: 400; white-space: nowrap; }
.center { text-align: center; }
.mono { font-family: var(--font-mono); font-size: 0.74rem; }
.detail { font-family: var(--font-mono); white-space: normal; overflow-wrap: anywhere; max-width: 320px; }

.tag {
  display: inline-block; font-size: 0.68rem; font-weight: 600; letter-spacing: 0.03em;
  font-family: var(--font-mono); border-radius: var(--r-full); padding: 2px 9px;
  border: 1px solid transparent;
}
.tag.admin { color: var(--accent); background: var(--accent-soft); }
.tag.player { color: var(--info); background: rgba(106, 165, 240, 0.1); }
.actor { font-size: 0.78rem; color: var(--text-dim); }
</style>
