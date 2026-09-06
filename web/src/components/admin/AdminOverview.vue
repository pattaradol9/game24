<script setup>
// Dashboard: counters, per-mode aggregate stats and a 14-day signup chart.
import { inject, onMounted, ref } from 'vue'
import { adminApi } from '../../admin.js'
import { MODES } from '../../modes.js'
import { fmt } from './format.js'

const onAuthFail = inject('onAdminAuthFail', () => {})
const loading = ref(true)
const error = ref('')
const o = ref(null)

const modeLabel = (id) => id.charAt(0).toUpperCase() + id.slice(1)

async function load() {
  loading.value = true
  error.value = ''
  try {
    o.value = await adminApi.overview()
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    loading.value = false
  }
}

function statOf(mode) {
  return o.value?.modes?.find((m) => m.mode === mode) ?? null
}

const maxSignup = () => Math.max(1, ...(o.value?.signups ?? []).map((d) => d.count))

onMounted(load)
</script>

<template>
  <div class="ov">
    <p v-if="loading" class="loading">Loading…</p>
    <p v-else-if="error" class="err">{{ error }}</p>

    <template v-else-if="o">
      <div class="cards">
        <div class="card panel">
          <span class="label">Total players</span>
          <b class="num">{{ fmt(o.players.total) }}</b>
          <span class="meta">{{ fmt(o.players.google) }} Google · {{ fmt(o.players.guests) }} guests</span>
        </div>
        <div class="card panel">
          <span class="label">Banned</span>
          <b class="num bad">{{ fmt(o.players.banned) }}</b>
          <span class="meta">excluded from boards</span>
        </div>
        <div class="card panel">
          <span class="label">Rounds played</span>
          <b class="num">{{ fmt(o.roundsTotal) }}</b>
          <span class="meta">{{ fmt(o.rounds.solved ?? 0) }} solved · {{ fmt(o.rounds.skipped ?? 0) }} skipped</span>
        </div>
        <div class="card panel">
          <span class="label">EXP in system</span>
          <b class="num">{{ fmt(o.totalExp) }}</b>
          <span class="meta">sum of all players</span>
        </div>
        <div class="card panel">
          <span class="label">Event log entries</span>
          <b class="num">{{ fmt(o.eventsTotal) }}</b>
          <span class="meta">audit trail</span>
        </div>
      </div>

      <section class="panel block">
        <h2>Per-mode statistics</h2>
        <div class="twrap">
          <table class="t">
            <thead>
              <tr>
                <th>Mode</th><th class="num">EXP</th><th class="num">Solved</th>
                <th class="num">Skipped</th><th class="num">Best streak</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in MODES" :key="m.id">
                <td>{{ modeLabel(m.id) }} <span class="dim">×{{ m.mult }}</span></td>
                <td class="num">{{ fmt(statOf(m.id)?.exp) }}</td>
                <td class="num">{{ fmt(statOf(m.id)?.handsSolved) }}</td>
                <td class="num">{{ fmt(statOf(m.id)?.handsSkipped) }}</td>
                <td class="num">{{ fmt(statOf(m.id)?.bestStreak) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="panel block">
        <h2>New players — last 14 days</h2>
        <div v-if="!o.signups?.length" class="dim pad">No signups recorded yet.</div>
        <div v-else class="chart">
          <div v-for="d in o.signups" :key="d.day" class="col" :title="`${d.day}: ${d.count}`">
            <span class="bar" :style="{ height: `${(d.count / maxSignup()) * 100}%` }" />
            <span class="day">{{ d.day.slice(5) }}</span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.ov { display: flex; flex-direction: column; gap: 18px; }
.loading, .err { color: var(--text-mute); padding: 20px 4px; }
.err { color: var(--bad); }

.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; }
.card { display: flex; flex-direction: column; gap: 6px; padding: 18px 20px; }
.label { font-size: 0.74rem; text-transform: uppercase; letter-spacing: 0.07em; color: var(--text-mute); }
.card b { font-size: 1.7rem; font-weight: 600; line-height: 1.1; }
.card b.bad { color: var(--bad); }
.meta { font-size: 0.78rem; color: var(--text-dim); }

.block { padding: 20px 22px; display: flex; flex-direction: column; gap: 14px; }
.block h2 { font-size: 0.95rem; color: var(--text-dim); font-weight: 600; }

.twrap { overflow-x: auto; }
.t { width: 100%; border-collapse: collapse; font-size: 0.88rem; }
.t th {
  text-align: left; font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text-mute); font-weight: 600; padding: 8px 10px; border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.t td { padding: 9px 10px; border-bottom: 1px solid var(--line-soft); }
.t tr:last-child td { border-bottom: none; }
.t .num { text-align: right; font-variant-numeric: tabular-nums; }
.dim { color: var(--text-mute); font-size: 0.8rem; }
.pad { padding: 6px 0 10px; }

.chart { display: flex; align-items: flex-end; gap: 6px; height: 130px; padding-top: 8px; }
.col { flex: 1; display: flex; flex-direction: column; align-items: center; gap: 6px; height: 100%; justify-content: flex-end; min-width: 0; }
.bar { width: min(100%, 26px); background: var(--accent); opacity: 0.85; border-radius: 4px 4px 2px 2px; min-height: 2px; }
.day { font-size: 0.62rem; color: var(--text-mute); font-family: var(--font-mono); white-space: nowrap; }
</style>
