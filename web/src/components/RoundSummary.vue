<script setup>
// Round summary: every seat's cumulative score tallies up live — each row
// counts from what the seat stood at before the round (score − gained,
// which is 0 in round one) to its new total, with the round's gain as a
// chip beside it. Non-solvers sit still at their old total. The host's
// continue button is the fast path; the auto-advance countdown below it
// shows when the summary moves on by itself.
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from '../i18n/index.js'
import TierAvatar from './TierAvatar.vue'

const { t } = useI18n()
const props = defineProps({
  show: { type: Boolean, default: false },
  result: { type: Object, default: null }, // { roundNo, solution, standings, autoNextAt }
  roundNo: { type: Number, default: 0 },
  you: { type: String, default: '' },
  // only the host can move the match on: their button starts the next
  // round — or closes the match after the last one — everyone else waits
  isHost: { type: Boolean, default: false },
  lastRound: { type: Boolean, default: false },
  // epoch ms when the summary advances itself (the stall guard)
  autoNextAt: { type: Number, default: 0 },
})
const emit = defineEmits(['next'])

const rows = computed(() => props.result?.standings ?? [])
// per-round ranking: this round's gain puts the seats in order (the fastest
// solve out-earns the slow one), cumulative score breaks ties
const ranked = computed(() => {
  const list = rows.value.map((r) => ({ ...r }))
  list.sort((a, b) => (b.gained ?? 0) - (a.gained ?? 0) || b.score - a.score)
  return list
})
const rankClass = (i) => (i < 3 ? `r${i + 1}` : '')
const display = ref({}) // seat id → the score number being shown right now
let raf = 0
const easeOut = (p) => 1 - Math.pow(1 - p, 3)

function animate() {
  cancelAnimationFrame(raf)
  const list = ranked.value
  if (!list.length) return
  const t0 = performance.now()
  const dur = 900 // one row's count-up
  const stagger = 130 // rows take turns, top of the table first
  const tick = (now) => {
    let pending = false
    const next = {}
    list.forEach((row, i) => {
      const start = row.score - (row.gained ?? 0)
      const p = Math.min(1, Math.max(0, (now - t0 - i * stagger) / dur))
      if (p < 1) pending = true
      next[row.id] = Math.round(start + (row.score - start) * easeOut(p))
    })
    display.value = next
    if (pending) raf = requestAnimationFrame(tick)
  }
  raf = requestAnimationFrame(tick)
}
watch(() => props.show, (s) => { if (s) animate() })
onUnmounted(() => cancelAnimationFrame(raf))

// live seconds until the summary advances itself
const left = ref(0)
let iv = 0
function tick() {
  left.value = props.autoNextAt > 0 ? Math.max(0, Math.ceil((props.autoNextAt - Date.now()) / 1000)) : 0
}
watch(() => [props.show, props.autoNextAt], ([s]) => {
  clearInterval(iv)
  if (s) {
    tick()
    iv = setInterval(tick, 500)
  }
})
onUnmounted(() => clearInterval(iv))
</script>

<template>
  <Transition name="fade">
    <div v-if="show && result" class="overlay">
      <div class="panel modal">
        <p class="round">{{ t('round') }} {{ roundNo }}</p>
        <h2>{{ t('finalResult') }}</h2>
        <ol class="tally">
          <li v-for="(row, i) in ranked" :key="row.id" :class="{ me: row.id === you }">
            <span class="rank" :class="rankClass(i)">{{ i + 1 }}</span>
            <TierAvatar :tier="row.guest ? 'guest' : row.tier" :name="row.name" :size="28" />
            <span class="who">
              <span class="name">{{ row.name }}</span>
              <span class="state" :class="row.solved ? 'ok' : 'missed'">
                {{ row.solved ? `${t('solvedStatus')} #${row.solveOrder}` : row.timedOut ? t('timeUpStatus') : t('notSolved') }}
              </span>
            </span>
            <span class="gain" :class="{ none: !(row.gained > 0) }">
              {{ row.gained > 0 ? `+${row.gained}` : '+0' }}
            </span>
            <span class="score num">{{ display[row.id] ?? row.score - (row.gained ?? 0) }}</span>
          </li>
        </ol>
        <p v-if="result.solution" class="expr">{{ result.solution }}</p>
        <button v-if="isHost" class="btn primary big" @click="emit('next')">
          <span>{{ lastRound ? t('finishMatch') : t('nextRound') }}</span>
          <!-- the auto-advance countdown rides on the button itself -->
          <span v-if="left > 0" class="cd num">{{ left }}</span>
        </button>
        <p v-else class="host-wait">{{ t('waitingHostNext') }}</p>
        <p v-if="!isHost && left > 0" class="auto num">
          {{ lastRound ? t('autoFinishIn', { n: left }) : t('autoNextIn', { n: left }) }}
        </p>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 90;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.66);
}
.modal {
  text-align: center;
  width: min(420px, 100%);
  padding: 26px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  animation: rise-in 0.24s var(--ease);
}
.round { font-size: 0.75rem; letter-spacing: 0.14em; text-transform: uppercase; color: var(--text-mute); }
h2 { font-size: 1.3rem; font-weight: 600; color: var(--good); }
.tally { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.tally li {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border: 1px solid var(--line-soft);
  border-radius: var(--r-sm);
  background: var(--surface-2);
}
.tally li.me { border-color: rgba(246, 183, 60, 0.35); background: var(--accent-soft); }
/* per-round place: gold / silver / bronze for the podium, muted below */
.rank {
  flex: none;
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  border-radius: var(--r-full);
  background: var(--surface-3);
  color: var(--text-mute);
  font-size: 0.78rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.rank.r1 { color: #241a02; background: #ffd700; }
.rank.r2 { color: #12151c; background: #c9ced8; }
.rank.r3 { color: #2b1707; background: #cd7f32; }
.who { flex: 1; min-width: 0; display: flex; flex-direction: column; align-items: flex-start; gap: 2px; }
.name { font-size: 0.9rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; }
.state {
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}
.state.ok { color: var(--good); }
.state.missed { color: var(--text-mute); }
.gain {
  flex: none;
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--good);
  background: color-mix(in srgb, var(--good) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--good) 40%, transparent);
  border-radius: var(--r-full);
  padding: 3px 9px;
  font-variant-numeric: tabular-nums;
}
.gain.none { color: var(--text-mute); background: transparent; border-color: var(--line-soft); }
.score {
  flex: none;
  min-width: 3.2ch;
  text-align: right;
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}
.expr {
  font-family: var(--font-mono);
  font-size: 0.95rem;
  color: var(--accent);
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 10px 14px;
}
.host-wait { font-size: 0.84rem; color: var(--text-mute); }
.auto { font-size: 0.78rem; color: var(--text-mute); font-variant-numeric: tabular-nums; }
/* the seconds-to-auto-advance pill riding on the host's continue button */
.cd {
  min-width: 2.4ch;
  padding: 2px 9px;
  border-radius: var(--r-full);
  background: rgba(0, 0, 0, 0.16);
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.fade-enter-active, .fade-leave-active { transition: opacity 0.22s var(--ease); }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
