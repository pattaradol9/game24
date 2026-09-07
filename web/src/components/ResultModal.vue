<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { levelProgress } from '../core/progress.js'
import { useI18n } from '../i18n/index.js'
import XpBar from './XpBar.vue'
import TierAvatar from './TierAvatar.vue'
import { sfx } from '../audio.js'
import { haptic } from '../fx.js'

const { t } = useI18n()

const props = defineProps({
  show: { type: Boolean, default: false },
  win: { type: Boolean, default: false },
  expr: { type: String, default: '' },
  points: { type: Number, default: 0 },
  solution: { type: String, default: '' },
  player: { type: Object, default: null },
  levelUp: { type: Boolean, default: false },
  tierUp: { type: Boolean, default: false },
  remaining: { type: Number, default: 0 },
  timeLimit: { type: Number, default: 0 },
})
defineEmits(['next'])

const starsShown = ref(0)

// speed rating: fraction of the clock left when solved
const starCount = computed(() => {
  if (!props.win || !props.timeLimit) return 0
  const ratio = props.remaining / props.timeLimit
  if (ratio >= 0.5) return 3
  if (ratio >= 0.22) return 2
  return 1
})

/* ---------------------------------------------------------------------------
   EXP payout.

   The bar starts where the player was *before* the round and is driven frame by
   frame to where they are now, recomputing the level each step — so a level-up
   simply fills the bar, wraps to empty and keeps going, however many levels the
   round was worth.
--------------------------------------------------------------------------- */
const expFrom = ref(0)
const expNow = ref(0)
const expShown = computed(() => Math.round(expNow.value))
const expProg = computed(() => levelProgress(expShown.value))
const gained = computed(() => Math.max(0, expShown.value - expFrom.value))
let expRaf = 0

function runPayout() {
  cancelAnimationFrame(expRaf)
  const p = props.player
  if (!p || p.isGuest) return
  const to = p.totalExp ?? 0
  const from = Math.max(0, to - (props.points ?? 0))
  expFrom.value = from
  expNow.value = from
  if (to <= from) return
  const t0 = performance.now() + 420 // let the panel settle first
  const dur = 900
  const step = (now) => {
    const k = Math.min(1, Math.max(0, (now - t0) / dur))
    const eased = 1 - (1 - k) ** 3
    expNow.value = from + (to - from) * eased
    if (k < 1) expRaf = requestAnimationFrame(step)
  }
  expRaf = requestAnimationFrame(step)
}

let revealTimers = []
onMounted(() => { if (props.show) { reveal(); runPayout() } })
watch(() => props.show, (s) => {
  if (s) { reveal(); runPayout() }
  else { revealTimers.forEach(clearTimeout); cancelAnimationFrame(expRaf) }
})
onUnmounted(() => { revealTimers.forEach(clearTimeout); cancelAnimationFrame(expRaf) })

function reveal() {
  starsShown.value = 0
  revealTimers.forEach(clearTimeout)
  revealTimers = []
  if (!props.win) return
  // short drumroll, then stars pop one by one with rising dings
  sfx.drumroll()
  for (let i = 0; i < starCount.value; i++) {
    revealTimers.push(setTimeout(() => {
      starsShown.value = i + 1
      sfx.star(i)
      haptic(12)
    }, 620 + i * 260))
  }
}
</script>

<template>
  <div v-if="show" class="overlay" @click.self="$emit('next')">
    <div class="panel modal">
      <span class="verdict" :class="win ? 'win' : 'lose'">{{ win ? t('youWin') : t('timeUp') }}</span>

      <div v-if="win" class="stars" :aria-label="`${starCount}/3`">
        <span v-for="i in 3" :key="i" class="star" :class="{ on: i <= starsShown }">★</span>
      </div>

      <p v-if="win" class="expr">{{ expr }}</p>
      <div v-if="!win && solution" class="solution">
        <span class="section-title">{{ t('solution') }}</span>
        <code>{{ solution }}</code>
      </div>

      <p v-if="points > 0" class="points num" :class="{ ticking: gained < points }">
        +{{ player && !player.isGuest ? gained : points }} EXP
      </p>

      <div v-if="player && !player.isGuest" class="progress">
        <TierAvatar :tier="player.tier" :src="player.picture" :name="player.nickname" :size="46" />
        <XpBar
          class="bar"
          instant
          :exp="expShown"
          :into="expProg.into"
          :for-next="expProg.forNext"
          :level="expProg.lv"
        />
      </div>
      <p v-else-if="player" class="guest-note">{{ t('guestsNoExp') }}</p>

      <button class="btn primary big block" @click="$emit('next')">{{ t('nextHand') }}</button>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.74);
}
.modal {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 16px;
  width: min(400px, 100%);
  padding: 28px;
  text-align: center;
  animation: rise-in 0.26s var(--ease);
}
.verdict { font-size: 1.5rem; font-weight: 600; letter-spacing: -0.01em; }
.verdict.win { color: var(--good); }
.verdict.lose { color: var(--bad); }

.stars { display: flex; justify-content: center; gap: 8px; }
.star {
  font-size: 1.5rem;
  color: var(--surface-3);
  transition: color 0.25s var(--ease), transform 0.25s var(--ease-out-back);
}
.star.on { color: var(--accent); transform: scale(1.12); }

.expr {
  font-family: var(--font-mono);
  font-size: 1rem;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 12px;
  word-break: break-all;
}
.solution { display: flex; flex-direction: column; gap: 7px; }
.solution code {
  font-family: var(--font-mono);
  font-size: 0.95rem;
  color: var(--text-dim);
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 12px;
  word-break: break-all;
}
.points { color: var(--accent); font-size: 1.1rem; font-weight: 600; font-variant-numeric: tabular-nums; }
.points.ticking { animation: points-tick 0.5s var(--ease) infinite; }
@keyframes points-tick {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.65; }
}
/* framed avatar states the tier; the bar tells you how far to the next one */
.progress { display: flex; align-items: center; gap: 14px; }
.progress .bar { flex: 1; width: auto; min-width: 0; }
.guest-note { font-size: 0.8rem; color: var(--text-mute); }
</style>
