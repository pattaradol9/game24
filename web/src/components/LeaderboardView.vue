<script setup>
import { onMounted, ref, watch, computed } from 'vue'
import { useI18n } from '../i18n/index.js'
import { api } from '../api.js'
import { playerIdentityVersion } from '../auth.js'
import { MODES } from '../modes.js'
import Suit from './Suit.vue'
import TierAvatar from './TierAvatar.vue'
import CrownMark from './CrownMark.vue'

// ranking windows; the server counts weekly hands since Monday 00:00 UTC
const PERIODS = [
  { id: 'weekly', label: 'weekly' },
  { id: 'alltime', label: 'allTime' },
]

const { t } = useI18n()

const props = defineProps({
  compact: { type: Boolean, default: false }, // Home panel: top 5, no podium
})
const mode = ref('queen')
const period = ref('weekly')
const entries = ref([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const data = await api.leaderboard(mode.value, period.value)
    entries.value = data.entries ?? []
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch([mode, period], load)
// a rename (or Google sign-in) from the header profile menu must show up
// here without a page reload
watch(playerIdentityVersion, load)
defineExpose({ mode, period })

const fmt = (n) => (n ?? 0).toLocaleString('en-US')

const top3 = computed(() => entries.value.slice(0, 3))
// second place on the left, winner centre, third on the right
const podium = computed(() => [top3.value[1], top3.value[0], top3.value[2]].filter(Boolean))
const rows = computed(() =>
  props.compact ? entries.value.slice(0, 5) : entries.value.slice(3)
)
</script>

<template>
  <div class="board">
    <div class="tabs">
      <button
        v-for="m in MODES"
        :key="m.id"
        class="tab"
        :class="{ on: mode === m.id }"
        @click="mode = m.id"
      >
        <Suit :name="m.suit" :size="13" />{{ t(m.id) }}
      </button>
    </div>

    <div class="tabs periods">
      <button
        v-for="p in PERIODS"
        :key="p.id"
        class="tab"
        :class="{ on: period === p.id }"
        @click="period = p.id"
      >
        {{ t(p.label) }}
      </button>
      <span v-if="period === 'weekly'" class="period-hint">{{ t('weeklyResets') }}</span>
    </div>

    <p v-if="loading" class="loading">…</p>

    <div v-else-if="entries.length === 0" class="empty">
      <CrownMark :size="76" />
      <p>{{ t('emptyBoard') }}</p>
    </div>

    <template v-else>
      <!-- the three that matter get their own stage -->
      <div v-if="!compact" class="podium">
        <div
          v-for="e in podium"
          :key="e.playerId"
          class="seat"
          :class="[`r${e.rank}`, { lead: e.rank === 1 }]"
        >
          <CrownMark v-if="e.rank === 1" :size="52" class="crown" />
          <span class="place">{{ e.rank }}</span>
          <TierAvatar :tier="e.tier" :src="e.picture" :name="e.nickname" :size="46" />
          <b class="pname">{{ e.nickname }}</b>
          <span class="pexp num">{{ fmt(e.score) }} {{ t('pts') }}</span>
        </div>
      </div>

      <ol class="rows">
        <li v-for="e in rows" :key="e.playerId">
          <span class="rank num" :class="`r${e.rank}`">{{ e.rank }}</span>
          <TierAvatar :tier="e.tier" :src="e.picture" :name="e.nickname" :size="32" />
          <span class="who">
            <b>{{ e.nickname }}</b>
            <span class="meta num">
              Lv.{{ e.level }}
              <i>·</i> {{ fmt(e.handsSolved) }} {{ t('handsSolved') }}
              <span v-if="e.bestStreak" class="streak"><i>·</i> {{ t('streak') }} {{ e.bestStreak }}</span>
            </span>
          </span>
          <span class="exp num" :title="t('score')">{{ fmt(e.score) }}</span>
        </li>
      </ol>
    </template>
  </div>
</template>

<style scoped>
.board { display: flex; flex-direction: column; gap: 16px; }

.tabs { display: flex; gap: 6px; flex-wrap: wrap; }
.tab {
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-mute);
  border-radius: var(--r-sm);
  padding: 8px 14px;
  font: inherit;
  font-size: 0.88rem;
  transition: color 0.15s var(--ease), background 0.15s var(--ease);
}
@media (hover: hover) { .tab:hover { color: var(--text); background: var(--surface-2); } }
.tab.on { color: var(--accent); background: var(--accent-soft); border-color: rgba(246, 183, 60, 0.3); }

/* weekly / all-time windows — a quieter row under the mode tabs */
.periods { margin-top: -8px; align-items: center; }
.periods .tab { padding: 6px 12px; font-size: 0.8rem; }
.period-hint { font-size: 0.74rem; color: var(--text-mute); margin-left: 6px; }

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  text-align: center;
  padding: 30px 20px;
  color: var(--text-mute);
  font-size: 0.9rem;
  border: 1px dashed var(--line);
  border-radius: var(--r-md);
}
.loading { text-align: center; color: var(--text-mute); padding: 24px; }

/* ---------- podium ---------- */
.podium { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; align-items: end; }
.seat {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 20px 12px 16px;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  background: var(--surface-2);
  min-width: 0;
}
.seat.lead {
  padding-top: 34px;
  border-color: rgba(246, 183, 60, 0.45);
  background: var(--accent-soft);
}
.crown { position: absolute; top: -26px; left: 50%; transform: translateX(-50%); }
.place {
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.1em;
  color: var(--text-mute);
}
.seat.lead .place { color: var(--accent); }
.pname {
  font-size: 0.92rem;
  font-weight: 500;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pexp { font-size: 0.8rem; color: var(--text-mute); }
.seat.lead .pexp { color: var(--accent); font-weight: 600; }

/* ---------- ranked rows ---------- */
.rows { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; }
.rows li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 4px;
  border-top: 1px solid var(--line-soft);
  min-width: 0;
}
.rows li:first-child { border-top: none; }
.rank {
  flex: none;
  width: 26px;
  text-align: center;
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-mute);
}
.rank.r1, .rank.r2, .rank.r3 { color: var(--accent); }
.who { flex: 1; display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.who b { font-size: 0.9rem; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.meta { font-size: 0.74rem; color: var(--text-mute); white-space: nowrap; }
.meta i { font-style: normal; opacity: 0.5; margin: 0 3px; }
.exp { font-size: 0.95rem; font-weight: 600; color: var(--text); min-width: 68px; text-align: right; }

@media (max-width: 560px) {
  .podium { gap: 8px; }
  .seat { padding: 16px 6px 12px; }
  .seat.lead { padding-top: 30px; }
  .seat img, .avatar { width: 38px; height: 38px; }
  .pname { font-size: 0.82rem; }
  .streak { display: none; } /* keep the meta line to one row on phones */
  .exp { min-width: 56px; font-size: 0.88rem; }
}
</style>
