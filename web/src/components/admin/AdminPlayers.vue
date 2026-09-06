<script setup>
// Player administration: searchable table + a management dialog per player
// (rename, ban/unban, EXP grant/revoke, stat resets, delete).
import { inject, onMounted, onUnmounted, ref, watch } from 'vue'
import { adminApi } from '../../admin.js'
import { MODES } from '../../modes.js'
import AdminPager from './AdminPager.vue'
import AdminModal from './AdminModal.vue'
import { fmt, fmtDate, fmtDuration } from './format.js'

const onAuthFail = inject('onAuthFail', () => {})

const LIMIT = 20
const players = ref([])
const total = ref(0)
const offset = ref(0)
const loading = ref(false)
const error = ref('')
const search = ref('')
const filter = ref('all')

let searchTimer = null
watch(search, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    offset.value = 0
    load()
  }, 300)
})
watch(filter, () => {
  offset.value = 0
  load()
})
onUnmounted(() => clearTimeout(searchTimer))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await adminApi.players({
      search: search.value.trim(),
      filter: filter.value,
      limit: LIMIT,
      offset: offset.value,
    })
    players.value = data.players ?? []
    total.value = data.total ?? 0
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    loading.value = false
  }
}
onMounted(load)
function page(newOffset) {
  offset.value = newOffset
  load()
}

// --- management dialog ---
const detail = ref(null)
const detailError = ref('')
const rounds = ref([])
const roundsTotal = ref(0)
const roundsOffset = ref(0)
const roundsLoading = ref(false)

const renameValue = ref('')
const banReason = ref('')
const expMode = ref('queen')
const expDelta = ref('')
const resetMode = ref('')
const deleteConfirm = ref('')
const busy = ref(false)
const notice = ref('')

async function open(p) {
  detail.value = p
  detailError.value = ''
  notice.value = ''
  renameValue.value = p.nickname
  banReason.value = p.banReason ?? ''
  expMode.value = 'queen'
  expDelta.value = ''
  resetMode.value = ''
  deleteConfirm.value = ''
  await Promise.all([reloadDetail(), loadRounds(0)])
}

async function reloadDetail() {
  try {
    const data = await adminApi.playerDetail(detail.value.id)
    detail.value = data.player
  } catch (err) {
    if (err.status === 404) {
      detail.value = null
      load()
    } else {
      detailError.value = err.message
    }
  }
}

async function loadRounds(newOffset) {
  roundsLoading.value = true
  try {
    const data = await adminApi.playerRounds(detail.value.id, { limit: 10, offset: newOffset })
    rounds.value = data.rounds ?? []
    roundsTotal.value = data.total ?? 0
    roundsOffset.value = newOffset
  } finally {
    roundsLoading.value = false
  }
}

async function act(fn, okMsg) {
  busy.value = true
  notice.value = ''
  detailError.value = ''
  try {
    await fn()
    notice.value = okMsg
    if (detail.value) await reloadDetail()
    await load()
  } catch (err) {
    detailError.value = err.message
    onAuthFail(err)
  } finally {
    busy.value = false
  }
}

const doRename = () =>
  act(() => adminApi.patchPlayer(detail.value.id, { nickname: renameValue.value }), 'Nickname updated.')

const doBan = () =>
  act(() => adminApi.patchPlayer(detail.value.id, { banned: true, banReason: banReason.value }), 'Player banned.')

const doUnban = () =>
  act(() => adminApi.patchPlayer(detail.value.id, { banned: false }), 'Player unbanned.')

const doAdjustExp = () =>
  act(
    () => adminApi.adjustExp(detail.value.id, expMode.value, Number(expDelta.value)),
    `EXP adjusted for ${expMode.value}.`
  )

const doReset = () =>
  act(() => adminApi.resetStats(detail.value.id, resetMode.value || null), 'Stats reset.')

const doDelete = () =>
  act(async () => {
    await adminApi.deletePlayer(detail.value.id)
    detail.value = null
    await load()
  }, 'Player deleted.')

const perModeEntries = () => Object.entries(detail.value?.perMode ?? {})
</script>

<template>
  <div class="pl">
    <div class="controls">
      <input v-model="search" class="field search" type="search" placeholder="Search nickname, email or id…" />
      <select v-model="filter" class="field select">
        <option value="all">All players</option>
        <option value="google">Google accounts</option>
        <option value="guests">Guests</option>
        <option value="banned">Banned</option>
      </select>
    </div>

    <p v-if="error" class="err">{{ error }}</p>

    <div class="panel block">
      <div class="twrap">
        <table class="t">
          <thead>
            <tr>
              <th>Player</th><th>Type</th><th class="num">Level</th><th class="num">EXP</th>
              <th class="num">Solved</th><th class="num">Skipped</th><th>Status</th>
              <th class="num">Last seen</th><th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !players.length">
              <td colspan="9" class="dim center">Loading…</td>
            </tr>
            <tr v-else-if="!players.length">
              <td colspan="9" class="dim center">No players match.</td>
            </tr>
            <tr v-for="p in players" :key="p.id" :class="{ banned: p.banned }">
              <td class="who">
                <b>{{ p.nickname }}</b>
                <span class="sub">{{ p.email || p.id }}</span>
              </td>
              <td>
                <span class="tag" :class="p.isGuest ? 'guest' : 'google'">{{ p.isGuest ? 'Guest' : 'Google' }}</span>
              </td>
              <td class="num">Lv.{{ p.level }} <span class="dim">{{ p.tier }}</span></td>
              <td class="num">{{ fmt(p.totalExp) }}</td>
              <td class="num">{{ fmt(p.handsSolved) }}</td>
              <td class="num">{{ fmt(p.handsSkipped) }}</td>
              <td>
                <span v-if="p.banned" class="tag bad" :title="p.banReason">Banned</span>
                <span v-else class="tag ok">Active</span>
              </td>
              <td class="num dim">{{ fmtDate(p.lastSeenAt) }}</td>
              <td class="num"><button class="btn quiet manage" @click="open(p)">Manage</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <AdminPager :offset="offset" :limit="LIMIT" :total="total" :loading="loading" @page="page" />
    </div>

    <!-- management dialog -->
    <AdminModal v-if="detail" :title="`Player — ${detail.nickname}`" @close="detail = null">
      <p v-if="detailError" class="err">{{ detailError }}</p>
      <p v-if="notice" class="ok-msg">{{ notice }}</p>

      <section class="sect">
        <div class="kv">
          <span class="k">ID</span><span class="v mono">{{ detail.id }}</span>
          <span class="k">Email</span><span class="v">{{ detail.email || '—' }}</span>
          <span class="k">Type</span><span class="v">{{ detail.isGuest ? 'Guest' : 'Google account' }}</span>
          <span class="k">Status</span>
          <span class="v">
            <template v-if="detail.banned">
              <span class="tag bad">Banned</span>
              <span v-if="detail.banReason" class="dim"> — {{ detail.banReason }}</span>
            </template>
            <span v-else class="tag ok">Active</span>
          </span>
          <span class="k">Level</span><span class="v">Lv.{{ detail.level }} · {{ detail.tier }} · {{ fmt(detail.totalExp) }} EXP</span>
          <span class="k">Created</span><span class="v">{{ fmtDate(detail.createdAt) }}</span>
          <span class="k">Last seen</span><span class="v">{{ fmtDate(detail.lastSeenAt) }}</span>
        </div>
      </section>

      <section class="sect">
        <h4>Per-mode stats</h4>
        <div class="twrap">
          <table class="t tight">
            <thead>
              <tr><th>Mode</th><th class="num">EXP</th><th class="num">Solved</th><th class="num">Skipped</th><th class="num">Best streak</th><th class="num">Current</th></tr>
            </thead>
            <tbody>
              <tr v-if="!perModeEntries().length"><td colspan="6" class="dim center">No stats — guests never earn any.</td></tr>
              <tr v-for="[mode, st] in perModeEntries()" :key="mode">
                <td class="cap">{{ mode }}</td>
                <td class="num">{{ fmt(st.exp) }}</td>
                <td class="num">{{ fmt(st.handsSolved) }}</td>
                <td class="num">{{ fmt(st.handsSkipped) }}</td>
                <td class="num">{{ fmt(st.bestStreak) }}</td>
                <td class="num">{{ fmt(st.currentStreak) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="sect">
        <h4>Recent rounds <span class="dim">({{ fmt(roundsTotal) }} total)</span></h4>
        <p v-if="roundsLoading" class="dim">Loading…</p>
        <div v-else-if="!rounds.length" class="dim pad">No rounds played.</div>
        <div v-else class="twrap">
          <table class="t tight">
            <thead>
              <tr><th>Dealt</th><th>Mode</th><th>Status</th><th class="num">Points</th><th class="num">Time</th><th class="num">Hints</th></tr>
            </thead>
            <tbody>
              <tr v-for="r in rounds" :key="r.id">
                <td class="dim">{{ fmtDate(r.dealtAt) }}</td>
                <td class="cap">{{ r.mode }}</td>
                <td><span class="tag" :class="r.status">{{ r.status }}</span></td>
                <td class="num">{{ fmt(r.points) }}</td>
                <td class="num">{{ fmtDuration(r.elapsedMs) }}</td>
                <td class="num">{{ r.hintsUsed }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="roundsTotal > 10" class="rounds-nav">
          <button class="btn quiet" :disabled="roundsOffset === 0" @click="loadRounds(roundsOffset - 10)">‹</button>
          <span class="dim">{{ roundsOffset + 1 }}–{{ Math.min(roundsOffset + 10, roundsTotal) }}</span>
          <button class="btn quiet" :disabled="roundsOffset + 10 >= roundsTotal" @click="loadRounds(roundsOffset + 10)">›</button>
        </div>
      </section>

      <!-- actions -->
      <section class="sect actions">
        <h4>Actions</h4>

        <div class="action">
          <label>Rename</label>
          <div class="row">
            <input v-model="renameValue" class="field" maxlength="24" placeholder="New nickname" />
            <button class="btn" :disabled="busy || !renameValue.trim() || renameValue === detail.nickname" @click="doRename">Save</button>
          </div>
        </div>

        <div class="action">
          <label>{{ detail.banned ? 'Unban' : 'Ban' }}</label>
          <template v-if="!detail.banned">
            <div class="row">
              <input v-model="banReason" class="field" maxlength="200" placeholder="Reason (optional, shown in event log)" />
              <button class="btn danger" :disabled="busy" @click="doBan">Ban player</button>
            </div>
            <p class="hint">Banning locks the account instantly (token + Google sign-in) and hides them from leaderboards.</p>
          </template>
          <template v-else>
            <div class="row">
              <button class="btn" :disabled="busy" @click="doUnban">Unban player</button>
            </div>
          </template>
        </div>

        <div class="action">
          <label>Adjust EXP <span class="dim">(total resyncs to the per-mode sum)</span></label>
          <div class="row">
            <select v-model="expMode" class="field">
              <option v-for="m in MODES" :key="m.id" :value="m.id">{{ m.id }}</option>
            </select>
            <input v-model="expDelta" class="field num-input" type="number" placeholder="e.g. 500 or -200" />
            <button class="btn" :disabled="busy || !expDelta.trim() || Number(expDelta) === 0" @click="doAdjustExp">Apply</button>
          </div>
        </div>

        <div class="action">
          <label>Reset stats</label>
          <div class="row">
            <select v-model="resetMode" class="field">
              <option value="">All modes</option>
              <option v-for="m in MODES" :key="m.id" :value="m.id">{{ m.id }} only</option>
            </select>
            <button class="btn danger" :disabled="busy" @click="doReset">Reset</button>
          </div>
          <p class="hint">Clears the selected mode stats and the EXP that came with them. Levels follow the new total.</p>
        </div>

        <div class="action danger-zone">
          <label>Delete player</label>
          <div class="row">
            <input v-model="deleteConfirm" class="field" :placeholder="`Type “${detail.nickname}” to confirm`" />
            <button class="btn danger" :disabled="busy || deleteConfirm !== detail.nickname" @click="doDelete">Delete</button>
          </div>
          <p class="hint">Irreversible: removes the player, their stats and every round they played.</p>
        </div>
      </section>
    </AdminModal>
  </div>
</template>

<style scoped>
.pl { display: flex; flex-direction: column; gap: 14px; }
.controls { display: flex; gap: 10px; flex-wrap: wrap; }
.search { flex: 1; min-width: 220px; }
.select { width: 180px; }
.err { color: var(--bad); font-size: 0.88rem; }
.ok-msg { color: var(--good); font-size: 0.88rem; }

.block { padding: 6px 18px 16px; }
.twrap { overflow-x: auto; }
.t { width: 100%; border-collapse: collapse; font-size: 0.86rem; }
.t th {
  text-align: left; font-size: 0.7rem; text-transform: uppercase; letter-spacing: 0.06em;
  color: var(--text-mute); font-weight: 600; padding: 10px 10px 8px; border-bottom: 1px solid var(--line);
  white-space: nowrap;
}
.t td { padding: 9px 10px; border-bottom: 1px solid var(--line-soft); vertical-align: middle; }
.t tr:last-child td { border-bottom: none; }
.t .num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
.t.tight td, .t.tight th { padding: 7px 8px; font-size: 0.82rem; }
.t tr.banned td { opacity: 0.6; }
.center { text-align: center; }
.dim { color: var(--text-mute); font-size: 0.78rem; font-weight: 400; }
.pad { padding: 4px 0 8px; }
.cap { text-transform: capitalize; }
.mono { font-family: var(--font-mono); font-size: 0.78rem; }

.who { display: flex; flex-direction: column; gap: 2px; min-width: 140px; }
.who b { font-weight: 500; }
.who .sub { font-size: 0.72rem; color: var(--text-mute); max-width: 240px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.tag {
  display: inline-block; font-size: 0.68rem; font-weight: 600; letter-spacing: 0.04em;
  text-transform: uppercase; border-radius: var(--r-full); padding: 3px 9px;
  border: 1px solid var(--line); color: var(--text-dim);
}
.tag.google { color: var(--info); border-color: rgba(106, 165, 240, 0.4); background: rgba(106, 165, 240, 0.1); }
.tag.guest { color: var(--text-mute); }
.tag.ok { color: var(--good); border-color: rgba(63, 191, 131, 0.4); background: rgba(63, 191, 131, 0.08); }
.tag.bad { color: var(--bad); border-color: rgba(239, 95, 95, 0.4); background: rgba(239, 95, 95, 0.1); }
.tag.solved { color: var(--good); border-color: transparent; }
.tag.skipped, .tag.expired { color: var(--bad); border-color: transparent; }
.tag.open { color: var(--info); border-color: transparent; }

.manage { min-height: 32px; padding: 4px 12px; font-size: 0.8rem; }

.sect { padding: 14px 0; border-top: 1px solid var(--line-soft); display: flex; flex-direction: column; gap: 10px; }
.sect:first-child { border-top: none; padding-top: 4px; }
.sect h4 { font-size: 0.82rem; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-dim); }

.kv { display: grid; grid-template-columns: 90px 1fr; gap: 7px 12px; font-size: 0.85rem; }
.kv .k { color: var(--text-mute); font-size: 0.76rem; text-transform: uppercase; letter-spacing: 0.05em; padding-top: 2px; }
.kv .v { min-width: 0; overflow-wrap: anywhere; }

.action { display: flex; flex-direction: column; gap: 7px; }
.action label { font-size: 0.82rem; color: var(--text); font-weight: 500; }
.row { display: flex; gap: 8px; flex-wrap: wrap; }
.row .field { flex: 1; min-width: 160px; }
.row .num-input { max-width: 170px; flex: none; }
.row .btn { min-height: 40px; white-space: nowrap; }
.hint { font-size: 0.75rem; color: var(--text-mute); line-height: 1.45; }
.danger-zone { border-top: 1px solid rgba(239, 95, 95, 0.3); padding-top: 14px; }
.rounds-nav { display: flex; align-items: center; gap: 10px; }
.rounds-nav .btn { min-height: 30px; width: 30px; padding: 0; }
</style>
