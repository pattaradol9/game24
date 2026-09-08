<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { useGame } from '../composables/useGame.js'
import { sfx } from '../audio.js'
import { shake, popText, danger, haptic, centerOf } from '../fx.js'
import GameBoard from '../components/GameBoard.vue'
import StepHistory from '../components/StepHistory.vue'
import ResultModal from '../components/ResultModal.vue'
import CelebrationPopup from '../components/CelebrationPopup.vue'
import Countdown from '../components/Countdown.vue'
import StreakFlame from '../components/StreakFlame.vue'
import XpBar from '../components/XpBar.vue'
import Icon from '../components/Icon.vue'
import ModeBadge from '../components/ModeBadge.vue'
import PlayerChip from '../components/PlayerChip.vue'
import { currentPlayer } from '../auth.js'
import BuffBar from '../components/BuffBar.vue'

const { t, lang } = useI18n()
const route = useRoute()
const game = useGame()
const { phase, mode, hand, remaining, timeLimit, hintLeft, hintCard, result, busy, paused, combo } = game

const player = currentPlayer
const toast = ref('')
const timers = []

/* newly unlocked achievements queue → sequential CelebrationPopup flashes */
const achPopup = ref({ show: false, tier: '', title: '' })
const achQueue = []
const achLangTitle = (a) => a.title?.[lang.value] ?? a.title?.en ?? a.id

function popNextAchievement() {
  const a = achQueue.shift()
  if (!a) return
  achPopup.value = { show: true, tier: a.tier, title: achLangTitle(a) }
  later(() => {
    achPopup.value = { ...achPopup.value, show: false }
    later(popNextAchievement, 380)
  }, 2600)
}

watch(result, (r) => {
  const list = r?.newAchievements ?? []
  if (!list.length) return
  achQueue.push(...list)
  if (achPopup.value.show) return
  // let the level/tier banner finish first so the two never overlap
  if (r?.levelUp || r?.tierUp) later(popNextAchievement, 2800)
  else popNextAchievement()
})

function later(fn, ms) {
  const id = setTimeout(fn, ms)
  timers.push(id)
  return id
}

let toastId = 0
function say(lines, ms = 2600) {
  const pool = Array.isArray(lines) ? lines : [lines]
  toast.value = pool[Math.floor(Math.random() * pool.length)]
  clearTimeout(toastId)
  toastId = later(() => (toast.value = ''), ms)
}

const urgent = computed(() => remaining.value <= 10 && phase.value === 'playing' && !paused.value)
const wrongFlash = ref(false)
const showCountdown = ref(false)
const dealing = ref(false)
const streak = computed(() => player.value?.perMode?.[mode.value]?.currentStreak ?? 0)

const timePct = computed(() =>
  timeLimit.value > 0 ? Math.max(0, Math.min(100, (remaining.value / timeLimit.value) * 100)) : 0
)
const mmss = computed(() =>
  `${Math.floor(remaining.value / 60)}:${String(remaining.value % 60).padStart(2, '0')}`
)

watch(urgent, (u) => danger(u ? 0.8 : 0))

onMounted(() => {
  game.start(route.query.mode || 'queen')
})

// leaving mid-round must take the vignette and every pending timer with it
onUnmounted(() => {
  danger(0)
  clearTimeout(toastId)
  timers.forEach(clearTimeout)
})

watch(phase, (p) => {
  if (p === 'playing') beginRound()
  if (p === 'result') {
    danger(0)
    say(result.value?.win ? t('bubbleWin') : t('bubbleTime'), 3200)
  }
})

function beginRound() {
  showCountdown.value = true
  dealing.value = true
  for (let i = 0; i < 4; i++) sfx.deal(i)
  later(() => (dealing.value = false), 700)
  say(t('bubbleHello'), 2200)
}
function onCountdownDone() {
  showCountdown.value = false
  game.resume()
}

watch(combo, (c) => {
  if (c >= 2 && phase.value === 'playing') {
    const center = centerOf(document.querySelector('.solo .board'))
    if (center) popText(center.x, center.y - 20, `${t('combo')} ×${c}`, 'fx-combo')
    say(t('bubbleCombo'), 1600)
  }
})

// final merge that isn't 24 → failure feedback
watch(
  () => hand.value?.cards.length,
  (nv, ov) => {
    if (phase.value !== 'playing') return
    if (nv === 1 && ov === 2 && hand.value && !hand.value.won) {
      wrongFlash.value = true
      later(() => (wrongFlash.value = false), 500)
      shake(1)
      sfx.wrong()
      haptic([0, 60, 40, 60])
      say(t('bubbleWrong'), 2400)
    }
  }
)

watch(() => game.hintCard.value, (h) => { if (h) say(t('bubbleHint'), 3400) })
</script>

<template>
  <main class="wrap solo">
    <header class="top">
      <button class="btn back" @click="$router.push('/')">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <ModeBadge :mode="mode" />
      <div class="spacer" />
      <BuffBar class="solo-buffs" />
      <StreakFlame :streak="streak" />
      <span class="who panel"><PlayerChip compact /></span>
    </header>

    <div class="clock" :class="{ urgent }">
      <Icon name="clock" :size="17" />
      <span class="mmss num">{{ mmss }}</span>
      <span class="track"><span class="fill" :style="{ width: timePct + '%' }" /></span>
    </div>

    <div class="stage">
      <div class="board-wrap" :class="{ flash: wrongFlash }">
        <GameBoard
          v-if="phase === 'playing' && hand"
          :hand="hand"
          :disabled="paused"
          :hint-data="hintCard"
          :dealing="dealing"
          :skin="player?.skin"
          @pick="game.pickCard"
          @op="game.setOperator"
        />
        <div v-else class="loading">{{ phase === 'loading' ? '…' : '' }}</div>

        <div class="actions">
          <button class="btn" :disabled="phase !== 'playing' || paused || (hintLeft ?? 0) <= 0" @click="game.hint">
            <Icon name="bulb" :size="17" />{{ t('hint') }}
            <b v-if="(hintLeft ?? 0) > 0" class="count num">{{ hintLeft }}</b>
          </button>
          <button class="btn" :disabled="phase !== 'playing' || paused" @click="game.undo">
            <Icon name="undo" :size="17" />{{ t('undo') }}
          </button>
          <button class="btn danger" :disabled="phase !== 'playing' || paused || busy" @click="game.skip">
            <Icon name="skip" :size="17" />{{ t('skip') }}
          </button>
        </div>
      </div>

      <aside class="side">
        <StepHistory :history="hand?.history ?? []" />
        <div v-if="player && !player.isGuest" class="panel xp-panel">
          <XpBar
            :exp="player.totalExp"
            :into="player.levelExpInto"
            :for-next="player.levelExpForNext"
            :level="player.level"
          />
        </div>
      </aside>
    </div>

    <Transition name="toast">
      <p v-if="toast" class="toast">{{ toast }}</p>
    </Transition>

    <Countdown v-if="showCountdown" @done="onCountdownDone" />
    <ResultModal :show="phase === 'result'" v-bind="result ?? {}" @next="game.next" />
    <CelebrationPopup
      :show="!!(result && (result.levelUp || result.tierUp))"
      :kind="result?.tierUp ? 'tier' : 'level'"
      :value="result?.tierUp ? result?.player?.tier : `Lv.${result?.player?.level}`"
    />
    <CelebrationPopup
      :show="achPopup.show"
      kind="achievement"
      :tier="achPopup.tier"
      :value="achPopup.title"
    />
  </main>
</template>

<style scoped>
.solo { display: flex; flex-direction: column; flex: 1 1 auto; gap: 14px; padding: 18px 0 26px; }
.top { display: flex; align-items: center; gap: 10px; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
.who { padding: 7px 14px 7px 7px; border-radius: var(--r-md); box-shadow: none; }
/* boost tray rides in the header on desktop; phones keep the HUD minimal */
.solo-buffs { display: none; }
@media (min-width: 641px) {
  .solo-buffs { display: flex; }
}
@media (hover: hover) { .back:hover { color: var(--text); } }
.spacer { flex: 1; }

/* ---------- clock ---------- */
.clock {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--text-dim);
  transition: color 0.3s var(--ease);
}
.mmss { font-size: 1.05rem; font-weight: 600; color: var(--text); min-width: 3.4ch; }
.track {
  flex: 1;
  height: 4px;
  border-radius: var(--r-full);
  background: var(--surface-2);
  overflow: hidden;
}
.fill {
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: var(--accent);
  transition: width 0.3s linear, background 0.3s var(--ease);
}
.clock.urgent { color: var(--bad); }
.clock.urgent .mmss { color: var(--bad); }
.clock.urgent .fill { background: var(--bad); }

/* ---------- stage ---------- */
.stage {
  display: grid;
  grid-template-columns: minmax(0, 1fr) clamp(230px, 24vw, 300px);
  grid-template-areas:
    'board side'
    'quick side';
  gap: 16px 20px;
  flex: 1 1 auto;
  align-content: center;
  min-height: 0;
}
.board-wrap { grid-area: board; min-width: 0; display: flex; flex-direction: column; }
.board-wrap.flash { animation: shake 0.3s var(--ease); }
.actions {
  grid-area: quick;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-top: 16px;
}
.count {
  background: var(--accent);
  color: var(--accent-ink);
  border-radius: var(--r-full);
  min-width: 19px;
  height: 19px;
  display: inline-grid;
  place-items: center;
  font-size: 0.68rem;
  font-weight: 700;
  padding: 0 5px;
}
.side {
  grid-area: side;
  align-self: stretch;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
}
.side :deep(.history) { flex: 1 1 auto; }
.xp-panel { flex: none; padding: 14px 16px; }
.loading { text-align: center; color: var(--text-mute); font-size: 1.4rem; padding: 60px 0; }

/* ---------- toast ---------- */
.toast {
  position: fixed;
  left: 16px;
  right: 16px;
  bottom: calc(20px + env(safe-area-inset-bottom));
  margin-inline: auto;
  width: fit-content;
  max-width: min(360px, calc(100vw - 32px));
  z-index: 70;
  background: var(--surface-3);
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  box-shadow: var(--sh-3);
  padding: 11px 18px;
  text-align: center;
  font-size: 0.9rem;
}
.toast-enter-active { animation: rise-in 0.22s var(--ease); }
.toast-leave-active { transition: opacity 0.2s var(--ease); }
.toast-leave-to { opacity: 0; }

@media (max-width: 900px) {
  .solo { padding-bottom: calc(88px + env(safe-area-inset-bottom)); }
  .stage {
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas: 'board' 'side' 'quick';
    gap: 12px;
    flex: 0 0 auto;
    align-content: start;
  }
  .side { align-self: auto; }
  .actions {
    position: fixed;
    left: 12px;
    right: 12px;
    bottom: calc(12px + env(safe-area-inset-bottom));
    z-index: 50;
    margin-top: 0;
  }
  .toast { bottom: calc(76px + env(safe-area-inset-bottom)); }
}
@media (max-width: 560px) {
  .back-label { display: none; }
  .back { padding: 0; width: 44px; justify-content: center; }
}
@media (max-width: 430px) {
  .actions .btn { font-size: 0.85rem; padding-inline: 6px; gap: 5px; }
}
</style>
