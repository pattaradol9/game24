<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { levelProgress, scoreMultiplier } from '../core/progress.js'
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
  // running tallies of the whole game (every solved hand, level multiplier
  // included) — surfaced in the game-over dialogs
  sessionScore: { type: Number, default: 0 },
  sessionExp: { type: Number, default: 0 },
  sessionCoins: { type: Number, default: 0 },
  // exiting from the game: verdict + action read "game over / exit game"
  sessionEnd: { type: Boolean, default: false },
})
defineEmits(['next'])

const starsShown = ref(0)

// the game-over dialogs (timeout, exit) carry the run's totals
const showSummary = computed(() => props.sessionEnd || !props.win)

// speed rating: fraction of the clock left when solved
const starCount = computed(() => {
  if (!props.win || !props.timeLimit) return 0
  const ratio = props.remaining / props.timeLimit
  if (ratio >= 0.5) return 3
  if (ratio >= 0.22) return 2
  return 1
})

/* ---------------------------------------------------------------------------
   Game-over context (timeout / exit): where the player stands after the run
   and the level handicap chip that badges the score total. The profile and
   level bar live ONLY here — the win dialog stays clean.
--------------------------------------------------------------------------- */
const prog = computed(() => {
  const p = props.player
  if (!showSummary.value || !p || p.isGuest) return null
  return levelProgress(p.totalExp ?? 0)
})
const fmtMult = (m) => (Number.isInteger(m) ? String(m) : String(Math.round(m * 100) / 100))
const multLabel = computed(() => {
  const lv = props.player?.level
  if (!lv || lv <= 1) return ''
  return `×${fmtMult(scoreMultiplier(lv))}`
})

let revealTimers = []
onMounted(() => { if (props.show) reveal() })
watch(() => props.show, (s) => {
  if (s) reveal()
  else revealTimers.forEach(clearTimeout)
})
onUnmounted(() => revealTimers.forEach(clearTimeout))

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
  <div v-if="show" class="overlay">
    <div class="panel modal">
      <span class="verdict" :class="win ? 'win' : 'lose'">{{ sessionEnd ? t('gameOver') : win ? t('youWin') : t('timeUp') }}</span>

      <div v-if="win" class="stars" :aria-label="`${starCount}/3`">
        <span v-for="i in 3" :key="i" class="star" :class="{ on: i <= starsShown }">★</span>
      </div>

      <p v-if="win" class="expr">{{ expr }}</p>
      <div v-if="!win && solution" class="solution">
        <span class="section-title">{{ t('solution') }}</span>
        <code>{{ solution }}</code>
      </div>

      <!-- game over: everything this run actually banked -->
      <div v-if="showSummary" class="session-summary" data-test="session-summary">
        <span class="section-title">{{ t('sessionSummary') }}</span>
        <div class="sum-row">
          <span>{{ t('score') }}</span>
          <span class="sum-val">
            <span v-if="multLabel" class="mult-chip" :title="t('levelMultHint')">{{ multLabel }}</span>
            <b class="num">{{ sessionScore }}</b>
          </span>
        </div>
        <div v-if="sessionExp > 0" class="sum-row">
          <span>EXP</span>
          <b class="num">+{{ sessionExp }}</b>
        </div>
        <div v-if="sessionCoins > 0" class="sum-row">
          <span class="coin-lab"><Icon name="coin" :size="14" />{{ t('coins') }}</span>
          <b class="num">+{{ sessionCoins }}</b>
        </div>
      </div>

      <TransitionGroup v-if="achRevealed && sortedAchievements.length" name="achpop" tag="div" class="ach-list">
        <div v-for="a in sortedAchievements" :key="a.id" class="ach-row" :style="{ '--tc': achTierColor(a.tier) }">
          <Icon class="ach-ic" name="trophy" :size="16" />
          <span class="ach-name">{{ achTitle(a) }}</span>
          <span class="ach-rewards num">+{{ a.expReward }} EXP · +{{ a.coinReward }}</span>
        </div>
      </TransitionGroup>

      <!-- the run ends here: profile + level bar live in the game-over dialog
           only — the win dialog hands straight to the next round -->
      <div v-if="prog" class="progress">
        <TierAvatar :tier="player.tier" :src="player.picture" :name="player.nickname" :size="46" />
        <XpBar
          class="bar"
          instant
          :exp="player.totalExp"
          :into="prog.into"
          :for-next="prog.forNext"
          :level="prog.lv"
        />
      </div>

      <!-- hand cleared → the session continues; game over → leave or restart -->
      <button class="btn primary big block" @click="$emit('next')">{{ sessionEnd ? t('exitGame') : win ? t('nextRound') : t('startOver') }}</button>
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

/* game-over summary: everything this run banked */
.session-summary {
  display: flex;
  flex-direction: column;
  gap: 7px;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  background: var(--bg);
  padding: 12px 16px;
}
.session-summary .section-title { text-align: center; }
.sum-row { display: flex; align-items: center; justify-content: space-between; font-size: 0.9rem; color: var(--text-dim); }
.sum-row b { color: var(--text); font-weight: 700; }
.sum-val { display: inline-flex; align-items: center; gap: 7px; }
/* the level handicap the server folded into the score, +5% per level */
.mult-chip {
  padding: 2px 8px;
  border-radius: var(--r-full);
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid rgba(246, 183, 60, 0.35);
}
.coin-lab { display: inline-flex; align-items: center; gap: 6px; }

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
/* framed avatar states the tier; the bar tells you how far to the next one —
   rendered only in the game-over dialogs */
.progress { display: flex; align-items: center; gap: 14px; }
.progress .bar { flex: 1; width: auto; min-width: 0; }
</style>
