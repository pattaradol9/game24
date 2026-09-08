<script setup>
// One Server Boost section (a kind: "exp" or "coins"): the status line with
// a live countdown, the draft form (multiplier, duration, label) and the
// arm/stop actions. Saving never starts or stops the boost — the draft can
// sit ready days before the event. Internal tool — English-only by design.
import { computed, inject, onMounted, onUnmounted, ref, watch } from 'vue'
import { adminApi } from '../../admin.js'
import { fmt, fmtDate } from './format.js'

const props = defineProps({
  kind: { type: String, required: true }, // "exp" | "coins"
  title: { type: String, required: true },
  blurb: { type: String, default: '' },
})
const emit = defineEmits(['changed'])

const onAuthFail = inject('onAdminAuthFail', () => {})

const boost = ref(null)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const notice = ref('')

// form state: duration is edited as a number plus a unit for convenience
const multiplier = ref(2)
const durValue = ref(60)
const durUnit = ref('m') // m | h | d
const label = ref('')

const UNIT_MINUTES = { m: 1, h: 60, d: 1440 }
const PRESETS = [
  { minutes: 30, text: '30m' },
  { minutes: 60, text: '1h' },
  { minutes: 180, text: '3h' },
  { minutes: 720, text: '12h' },
  { minutes: 1440, text: '24h' },
  { minutes: 4320, text: '3d' },
]

const durationMinutes = computed(() => {
  const n = Math.floor(Number(durValue.value) || 0)
  return n * UNIT_MINUTES[durUnit.value]
})

// live countdown while the boost runs: server-reported seconds left, minus
// the seconds since the state was fetched, ticking once a second
const nowSec = ref(Math.floor(Date.now() / 1000))
let fetchedAt = 0
let ticker = 0
const remainingSec = computed(() => {
  if (!boost.value?.active) return 0
  const fromServer = boost.value.remainingSec || 0
  return Math.max(0, fromServer - (nowSec.value - fetchedAt))
})

const remainingLabel = computed(() => {
  let s = remainingSec.value
  if (s <= 0) return '0:00'
  const d = Math.floor(s / 86400); s %= 86400
  const h = Math.floor(s / 3600); s %= 3600
  const m = Math.floor(s / 60)
  const pad = (n) => String(n).padStart(2, '0')
  if (d) return `${d}d ${h}:${pad(m)}:${pad(s % 60)}`
  if (h) return `${h}:${pad(m)}:${pad(s % 60)}`
  return `${m}:${pad(s % 60)}`
})

const stateLabel = computed(() => {
  if (!boost.value) return ''
  if (boost.value.active) return 'Active'
  if (boost.value.enabled && !boost.value.active) return 'Expired'
  return boost.value.startsAt ? 'Stopped' : 'Draft'
})

const multiplierLabel = (m) => (Number.isInteger(m) ? String(m) : String(Math.round(m * 100) / 100))

function loadFormFrom(b) {
  multiplier.value = b.multiplier
  const mins = b.durationMinutes
  if (mins % 1440 === 0) { durValue.value = mins / 1440; durUnit.value = 'd' }
  else if (mins % 60 === 0) { durValue.value = mins / 60; durUnit.value = 'h' }
  else { durValue.value = mins; durUnit.value = 'm' }
  label.value = b.label || ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await adminApi.boost(props.kind)
    boost.value = data.boost
    fetchedAt = Math.floor(Date.now() / 1000)
    loadFormFrom(data.boost)
    emit('changed')
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    loading.value = false
  }
}
onMounted(() => {
  load()
  ticker = setInterval(() => { nowSec.value = Math.floor(Date.now() / 1000) }, 1000)
})
onUnmounted(() => clearInterval(ticker))

// the window ran out under our eyes: pull the state so the badge flips to
// Expired instead of a stuck 0:00 countdown
watch(remainingSec, (s) => {
  if (s <= 0 && boost.value?.active && !busy.value) load()
})

function done(msg) {
  notice.value = msg
  error.value = ''
}

async function save() {
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    const data = await adminApi.saveBoost(props.kind, {
      multiplier: Number(multiplier.value),
      durationMinutes: durationMinutes.value,
      label: label.value.trim(),
    })
    boost.value = data.boost
    fetchedAt = Math.floor(Date.now() / 1000)
    loadFormFrom(data.boost)
    emit('changed')
    done(`Settings saved — the boost is ${data.boost.active ? 'running' : 'still a draft'}.`)
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}

async function arm() {
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    const data = await adminApi.enableBoost(props.kind)
    boost.value = data.boost
    fetchedAt = Math.floor(Date.now() / 1000)
    emit('changed')
    done(`Boost armed — every solved hand pays ×${multiplierLabel(data.boost.multiplier)} for the next ${fmt(data.boost.durationMinutes)} minutes.`)
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}

async function stop() {
  busy.value = true
  notice.value = ''
  error.value = ''
  try {
    const data = await adminApi.disableBoost(props.kind)
    boost.value = data.boost
    fetchedAt = Math.floor(Date.now() / 1000)
    emit('changed')
    done('Boost stopped — payouts are back to base. The saved settings stay for the next run.')
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="panel block">
    <h2>{{ title }}</h2>
    <p class="hint">{{ blurb }}</p>

    <p v-if="loading" class="loading">Loading…</p>
    <p v-if="error" class="err">{{ error }}</p>

    <template v-if="boost">
      <div class="status" :class="{ live: boost.active }">
        <span class="badge" :class="{ live: boost.active }">{{ stateLabel }}</span>
        <span v-if="boost.active" class="count num">
          ×{{ multiplierLabel(boost.multiplier) }} · ends in <b>{{ remainingLabel }}</b>
          <span v-if="boost.endsAt" class="dim">({{ fmtDate(boost.endsAt) }})</span>
        </span>
        <span v-else-if="boost.startsAt" class="count dim">
          last window {{ fmtDate(boost.startsAt) }} → {{ fmtDate(boost.endsAt) }}
        </span>
        <span v-else class="count dim">never run yet</span>
      </div>

      <div class="form">
        <label class="fld">
          <span class="lbl">Multiplier</span>
          <input
            v-model.number="multiplier"
            class="field"
            type="number"
            min="1"
            max="100"
            step="0.5"
          />
          <span class="unit">× {{ kind === 'exp' ? 'EXP' : 'coins' }}</span>
        </label>
        <label class="fld">
          <span class="lbl">Duration</span>
          <input v-model.number="durValue" class="field" type="number" min="1" step="1" />
          <select v-model="durUnit" class="field unit-sel">
            <option value="m">minutes</option>
            <option value="h">hours</option>
            <option value="d">days</option>
          </select>
        </label>
        <label class="fld grow">
          <span class="lbl">Label (optional)</span>
          <input
            v-model="label"
            class="field"
            type="text"
            maxlength="80"
            :placeholder="kind === 'exp' ? 'e.g. Weekend double EXP' : 'e.g. Coin rain'"
          />
        </label>
      </div>

      <div class="row presets">
        <button
          v-for="p in PRESETS"
          :key="p.minutes"
          class="btn quiet chip"
          :disabled="busy"
          @click="durValue = p.minutes / UNIT_MINUTES[durUnit]"
        >
          {{ p.text }}
        </button>
        <span class="hint">Accepted: ×1–×100, 1 minute – 30 days.</span>
      </div>

      <div class="row">
        <button class="btn primary" :disabled="busy" @click="save">Save settings</button>
        <button v-if="!boost.active" class="btn" :disabled="busy" @click="arm">Enable now</button>
        <button v-else class="btn danger" :disabled="busy" @click="stop">Disable</button>
        <span class="hint">
          Saving never starts or stops the boost — it only updates what the next run uses. Enabling
          starts the window from this moment.
        </span>
      </div>

      <p class="meta dim">
        Last updated {{ boost.updatedAt ? fmtDate(boost.updatedAt) : '—'
        }}<template v-if="boost.updatedBy"> by <code>{{ boost.updatedBy }}</code></template>
      </p>
    </template>
  </section>
</template>

<style scoped>
.block { padding: 20px 22px; display: flex; flex-direction: column; gap: 14px; }
.block h2 { font-size: 0.95rem; color: var(--text-dim); font-weight: 600; }
.hint { font-size: 0.78rem; color: var(--text-mute); line-height: 1.5; flex: 1; min-width: 220px; }
.loading { color: var(--text-mute); padding: 8px 4px; }
.err { color: var(--bad); font-size: 0.85rem; }

.status { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.badge {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  padding: 3px 10px;
  border-radius: var(--r-full);
  border: 1px solid var(--line);
  color: var(--text-mute);
}
.badge.live {
  color: var(--accent);
  background: var(--accent-soft);
  border-color: rgba(246, 183, 60, 0.35);
}
.count { font-size: 0.9rem; }
.count b { font-weight: 700; color: var(--accent); }
.dim { color: var(--text-mute); font-size: 0.78rem; }

.form { display: flex; gap: 14px; flex-wrap: wrap; }
.fld { display: flex; align-items: center; gap: 8px; }
.fld .lbl {
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-mute);
}
.fld .field { width: 110px; }
.fld.grow { flex: 1; min-width: 220px; }
.fld.grow .field { width: 100%; }
.unit { color: var(--text-mute); font-size: 0.82rem; }
.unit-sel { width: auto; min-width: 104px; }

.row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.row .btn { min-height: 42px; white-space: nowrap; }
.chip { min-height: 34px; padding: 4px 12px; font-size: 0.78rem; }

.meta { font-size: 0.78rem; }
.meta code { font-family: var(--font-mono); font-size: 0.74rem; }
</style>
