<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { levelProgress } from '../core/progress.js'
import { useI18n } from '../i18n/index.js'
import XpBar from './XpBar.vue'
import TierAvatar from './TierAvatar.vue'
import Icon from './Icon.vue'
import { sfx } from '../audio.js'
import { haptic } from '../fx.js'

const { t, lang } = useI18n()

const props = defineProps({
  show: { type: Boolean, default: false },
  win: { type: Boolean, default: false },
  expr: { type: String, default: '' },
  points: { type: Number, default: 0 },
  // EXP the hand actually banked (server boost applied); falls back to points
  exp: { type: Number, default: -1 },
  solution: { type: String, default: '' },
  player: { type: Object, default: null },
  levelUp: { type: Boolean, default: false },
  tierUp: { type: Boolean, default: false },
  coins: { type: Number, default: 0 },
  newAchievements: { type: Array, default: () => [] },
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

// boost chips only make sense when the multiplier actually paid more (EXP:
// banked > base points; coins: any payout while a coin boost runs)
const fmtMult = (m) => (Number.isInteger(m) ? String(m) : String(Math.round(m * 100) / 100))
const expBoostLabel = computed(() => {
  const b = props.player?.boosts?.exp
  if (!b || props.exp < 0 || props.exp <= props.points) return ''
  return `×${fmtMult(b.multiplier)}`
})
const coinBoostLabel = computed(() => {
  const b = props.player?.boosts?.coins
  if (!b || props.coins <= 0) return ''
  return `×${fmtMult(b.multiplier)}`
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
  const from = Math.max(0, to - (props.exp >= 0 ? props.exp : props.points ?? 0))
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

/* ---------------------------------------------------------------------------
   Newly unlocked achievements slide in once the payout has (mostly) settled.
--------------------------------------------------------------------------- */
const ACH_TIERS = {
  bronze: '#cd7f32',
  silver: '#c0c0c0',
  gold: '#ffd700',
  platinum: '#7de3e1',
  legend: '#b283f0',
}
const achTierColor = (tier) => ACH_TIERS[String(tier || '').toLowerCase()] ?? ACH_TIERS.bronze
const achTitle = (a) => a.title?.[lang.value] ?? a.title?.en ?? a.id

// several unlocks in one hand read bronze → legend
const ACH_TIER_ORDER = ['bronze', 'silver', 'gold', 'platinum', 'legend']
const sortedAchievements = computed(() =>
  [...props.newAchievements].sort(
    (a, b) => ACH_TIER_ORDER.indexOf(String(a.tier || '').toLowerCase()) - ACH_TIER_ORDER.indexOf(String(b.tier || '').toLowerCase()),
  )
)

const achRevealed = ref(false)
let achTimer = 0
watch(() => props.show, (s) => {
  clearTimeout(achTimer)
  achRevealed.value = false
  if (s && props.newAchievements.length) {
    achTimer = setTimeout(() => (achRevealed.value = true), 1250)
  }
}, { immediate: true })
onUnmounted(() => clearTimeout(achTimer))
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
        <span v-if="expBoostLabel" class="boost-chip">{{ expBoostLabel }}</span>
      </p>

      <p v-if="win && coins > 0" class="coin-gain chip accent">
        <Icon name="coin" :size="18" />
        <b class="num">+{{ coins }}</b>
        <span v-if="coinBoostLabel" class="boost-chip">{{ coinBoostLabel }}</span>
      </p>

      <TransitionGroup v-if="achRevealed && sortedAchievements.length" name="achpop" tag="div" class="ach-list">
        <div v-for="a in sortedAchievements" :key="a.id" class="ach-row" :style="{ '--tc': achTierColor(a.tier) }">
          <Icon class="ach-ic" name="trophy" :size="16" />
          <span class="ach-name">{{ achTitle(a) }}</span>
          <span class="ach-rewards num">+{{ a.expReward }} EXP · +{{ a.coinReward }}</span>
        </div>
      </TransitionGroup>

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

/* server-wide EXP boost multiplier that bumped this payout */
.boost-chip {
  display: inline-block;
  margin-left: 4px;
  padding: 2px 8px;
  border-radius: var(--r-full);
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid rgba(246, 183, 60, 0.35);
  animation: rise-in 0.3s var(--ease-out-back);
}

/* coins earned this hand */
.coin-gain { align-self: center; font-weight: 600; }
.coin-gain b { font-weight: 700; }

/* achievements unlocked by this hand, revealed after the payout settles */
.ach-list { display: flex; flex-direction: column; gap: 8px; }
.ach-row {
  display: flex;
  align-items: center;
  gap: 9px;
  border: 1px solid color-mix(in srgb, var(--tc) 45%, transparent);
  background: color-mix(in srgb, var(--tc) 10%, transparent);
  border-radius: var(--r-sm);
  padding: 9px 12px;
  text-align: left;
}
.ach-ic { color: var(--tc); flex: none; }
.ach-name { flex: 1; min-width: 0; font-weight: 500; font-size: 0.9rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ach-rewards { flex: none; font-size: 0.72rem; color: var(--text-dim); }
.achpop-enter-active { animation: rise-in 0.3s var(--ease-out-back); }
.achpop-enter-from { opacity: 0; transform: translateY(8px); }
/* framed avatar states the tier; the bar tells you how far to the next one */
.progress { display: flex; align-items: center; gap: 14px; }
.progress .bar { flex: 1; width: auto; min-width: 0; }
.guest-note { font-size: 0.8rem; color: var(--text-mute); }
</style>
