<script setup>
// Full per-mode leaderboards — the admin sees everyone, including banned
// and guest rows, and can ban/unban straight from the board.
import { inject, onMounted, ref, watch } from 'vue'
import { adminApi } from '../../admin.js'
import { MODES } from '../../modes.js'
import { fmt } from './format.js'

const onAuthFail = inject('onAdminAuthFail', () => {})
const mode = ref('queen')
const entries = ref([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await adminApi.leaderboard(mode.value, { limit: 100 })
    entries.value = data.entries ?? []
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  } finally {
    loading.value = false
  }
}
onMounted(load)
watch(mode, load)

async function toggleBan(entry) {
  error.value = ''
  try {
    if (entry.banned) {
      await adminApi.patchPlayer(entry.playerId, { banned: false })
    } else {
      await adminApi.patchPlayer(entry.playerId, { banned: true, banReason: 'removed from leaderboard' })
    }
    await load()
  } catch (err) {
    error.value = err.message
    onAuthFail(err)
  }
}
</script>

<template>
  <div class="lb">
    <div class="controls">
      <div class="tabs">
        <button
          v-for="m in MODES"
          :key="m.id"
          class="tab"
          :class="{ on: mode === m.id }"
          @click="mode = m.id"
        >
          {{ m.id }}
        </button>
      </div>
      <span class="hint">Full board including guests and banned players.</span>
    </div>

    <p v-if="error" class="err">{{ error }}</p>

    <div class="panel block">
      <div class="twrap">
        <table class="t">
          <thead>
            <tr>
              <th class="num">#</th><th>Player</th><th class="num">EXP</th><th class="num">Lv</th>
              <th>Tier</th><th class="num">Solved</th><th class="num">Streak</th><th>Flags</th><th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading && !entries.length"><td colspan="9" class="dim center">Loading…</td></tr>
            <tr v-else-if="!entries.length"><td colspan="9" class="dim center">No entries for this mode.</td></tr>
            <tr v-for="e in entries" :key="e.playerId" :class="{ banned: e.banned }">
              <td class="num rank">{{ e.rank }}</td>
              <td class="who"><b>{{ e.nickname }}</b></td>
              <td class="num">{{ fmt(e.exp) }}</td>
              <td class="num">{{ e.level }}</td>
              <td class="dim cap">{{ e.tier }}</td>
              <td class="num">{{ fmt(e.handsSolved) }}</td>
              <td class="num">{{ fmt(e.bestStreak) }}</td>
              <td>
                <span v-if="e.banned" class="tag bad">Banned</span>
                <span v-if="e.isGuest" class="tag guest">Guest</span>
              </td>
              <td class="num">
                <button class="btn quiet act" @click="toggleBan(e)">
                  {{ e.banned ? 'Unban' : 'Ban' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lb { display: flex; flex-direction: column; gap: 14px; }
.controls { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.tabs { display: flex; gap: 6px; }
.tab {
  cursor: pointer; border: 1px solid transparent; background: transparent; color: var(--text-mute);
  font: inherit; font-size: 0.86rem; border-radius: var(--r-sm); padding: 8px 16px;
  transition: color 0.15s var(--ease), background 0.15s var(--ease);
}
.tab:hover { color: var(--text); background: var(--surface-2); }
.tab.on { color: var(--accent); background: var(--accent-soft); border-color: rgba(246, 183, 60, 0.3); }
.hint { font-size: 0.76rem; color: var(--text-mute); }
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
.rank { font-weight: 600; }
.dim { color: var(--text-mute); font-size: 0.8rem; font-weight: 400; }
.center { text-align: center; }
.cap { text-transform: capitalize; }
.t tr.banned td { opacity: 0.55; }
.who b { font-weight: 500; }

.tag {
  display: inline-block; font-size: 0.66rem; font-weight: 600; letter-spacing: 0.04em;
  text-transform: uppercase; border-radius: var(--r-full); padding: 2px 8px; margin-right: 4px;
  border: 1px solid var(--line); color: var(--text-dim);
}
.tag.bad { color: var(--bad); border-color: rgba(239, 95, 95, 0.4); background: rgba(239, 95, 95, 0.1); }
.tag.guest { color: var(--text-mute); }
.act { min-height: 30px; padding: 3px 12px; font-size: 0.78rem; }
.act:hover { color: var(--bad); }
</style>
