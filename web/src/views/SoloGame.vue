<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { useGame } from '../composables/useGame.js'
import { sfx } from '../audio.js'
import { shake, popText, danger, haptic, centerOf } from '../fx.js'
import GameBoard from '../components/GameBoard.vue'
import StepHistory from '../components/StepHistory.vue'
import ResultModal from '../components/ResultModal.vue'
import TimeExtendModal from '../components/TimeExtendModal.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
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
const router = useRouter()
const game = useGame()
const { phase, mode, hand, remaining, timeLimit, hintLeft, hintCard, result, busy, paused, combo, extendOffer, extendBusy, extendUsed, skipUsed, addTime, sessionScore, sessionExp, sessionCoins, restored } = game

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

// Time Extension stock for the "time over?" dialog — the player JSON's
// inventory rides along every profile fetch
const bagTimeItems = computed(() =>
  (player.value?.items ?? []).find((i) => i.id === 'time30')?.qty ?? 0
)

// leaving mid-game ends the run: confirm first, then show the summary dialog
// before actually exiting — the open hand is forfeit either way (it can never
// pay: the server only banks verified submits inside the hand's own window)
const exitConfirm = ref(false)
const sessionEnd = ref(false)
function onExit() {
  if (phase.value === 'playing' && !sessionEnd.value) {
    exitConfirm.value = true
    return
  }
  router.push('/')
}
function confirmExit() {
  exitConfirm.value = false
  game.stop() // freeze the clock, drop the stored session; in-flight responses are ignored
  sessionEnd.value = true
}
function onNext() {
  if (sessionEnd.value) {
    router.push('/')
    return
  }
  // a game-over reset happens inside the composable's next()
  game.next()
}

// Skip Pass stock: the Skip button folds one hand per play session and costs
// one unit, so guests and empty bags see the button go dark
const bagSkipItems = computed(() =>
  (player.value?.items ?? []).find((i) => i.id === 'skip1')?.qty ?? 0
)
const canSkip = computed(() =>
  phase.value === 'playing' && !paused.value && !extendOffer.value && !busy.value &&
  !skipUsed.value && bagSkipItems.value > 0 && !player.value?.isGuest
)

// the dialog's verdict: true spends one unit and resumes with +30s (a toast
// confirms it), false lets the hand go
async function onExtendResolve(use) {
  const added = await game.resolveExtend(use)
  if (added) say(t('timeExtendToast', { n: added }), 2400)
}

// the Add-time button (undo's old slot): spends one Time Extension any time
// mid-play, but only while the clock still has room under the mode's limit —
// one press per session, like the zero-time bailout dialog
const canExtend = computed(() =>
  phase.value === 'playing' && !paused.value && !extendOffer.value && !extendBusy.value &&
  !extendUsed.value && bagTimeItems.value > 0 && !player.value?.isGuest &&
  remaining.value < timeLimit.value
)
const extendHint = computed(() => {
  if (player.value?.isGuest || bagTimeItems.value <= 0) return t('timeNeedsItem')
  if (extendUsed.value) return t('timeExtendOnce')
  if (remaining.value >= timeLimit.value) return t('timeFullLimit')
  return ''
})
async function onAddTime() {
  const added = await game.addTime()
  if (added) say(t('timeExtendToast', { n: added }), 2400)
}

// a dark helper explains itself where it stands: pressing it pops a bubble
// over the button naming the reason — no item in the bag, this visit's quota
// already spent, or the clock already full. Disabled buttons swallow clicks,
// so a transparent gate lies over the dark button to catch the press.
const refused = ref('') // which helper is talking: hint | extend | skip
const refuseMsg = ref('')
let refuseTimer = 0
function refuse(which, msg, ev) {
  refused.value = which
  refuseMsg.value = msg
  clearTimeout(refuseTimer)
  refuseTimer = later(() => (refused.value = ''), 2600)
  flourish(ev?.currentTarget)
}

// the popup's three.js flourish: a short golden spark burst from the bubble.
// The module is code-split and boots its one shared canvas on the first press
// only (GameBackdrop discipline); reduced motion never boots it at all.
function flourish(el) {
  const r = el?.getBoundingClientRect?.()
  if (!r) return
  import('../three/popupFx.js')
    .then((m) => m.glint(r.left + r.width / 2, r.top))
    .catch(() => {})
}

// the Solution button is dark with a reason only when the hint quota is gone
const hintSpent = computed(() =>
  phase.value === 'playing' && !paused.value && !extendOffer.value && (hintLeft.value ?? 0) <= 0
)
// extendHint already names the extend reasons (no item / session used / clock full)
const extendSpent = computed(() =>
  phase.value === 'playing' && !paused.value && !extendOffer.value && !extendBusy.value && !!extendHint.value
)
// the same two reasons Skip can be dark for
const skipHint = computed(() => {
  if (player.value?.isGuest || bagSkipItems.value <= 0) return t('skipNeedsItem')
  if (skipUsed.value) return t('skipSpent')
  return ''
})
const skipSpent = computed(() =>
  phase.value === 'playing' && !paused.value && !extendOffer.value && !busy.value && !!skipHint.value
)

// Undo lives in the Steps panel now — active only while there is a step back
const canUndo = computed(() =>
  phase.value === 'playing' && !paused.value && !extendOffer.value &&
  (hand.value?.history?.length ?? 0) > 0
)

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
  import('../three/popupFx.js').then((m) => m.disposeGlint()).catch(() => {})
})

watch(phase, (p) => {
  if (p === 'playing') beginRound()
  if (p === 'result') {
    danger(0)
    say(result.value?.win ? t('bubbleWin') : t('bubbleTime'), 3200)
  }
})

function beginRound() {
  // a refresh-restored hand resumes silently — no countdown, no deal fanfare
  if (restored.value) {
    restored.value = false
    return
  }
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
      <button class="btn back" @click="onExit">
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
          :disabled="paused || !!extendOffer"
          :hint-data="hintCard"
          :dealing="dealing"
          :skin="player?.skin"
          @pick="game.pickCard"
          @op="game.setOperator"
        />
        <div v-else class="loading">{{ phase === 'loading' ? '…' : '' }}</div>

        <div class="actions">
          <div class="helper">
            <button class="btn" :disabled="phase !== 'playing' || paused || !!extendOffer || (hintLeft ?? 0) <= 0" @click="game.hint">
              <Icon name="bulb" :size="17" />{{ t('solution') }}
            </button>
            <span v-if="hintSpent" class="gate" :title="t('hintsSpent')" @click="refuse('hint', t('hintsSpent'), $event)" />
            <Transition name="whypop">
              <span v-if="refused === 'hint'" class="why" role="status">{{ refuseMsg }}</span>
            </Transition>
          </div>
          <div class="helper">
            <button
              class="btn"
              :disabled="!canExtend"
              :title="extendHint"
              data-test="add-time"
              @click="onAddTime"
            >
              <Icon name="hourglass" :size="17" />{{ t('addTime') }}
            </button>
            <span v-if="extendSpent" class="gate" :title="extendHint" @click="refuse('extend', extendHint, $event)" />
            <Transition name="whypop">
              <span v-if="refused === 'extend'" class="why" role="status">{{ refuseMsg }}</span>
            </Transition>
          </div>
          <div class="helper">
            <button
              class="btn danger"
              :disabled="!canSkip"
              :title="canSkip ? '' : skipHint"
              data-test="skip-hand"
              @click="game.skip"
            >
              <Icon name="skip" :size="17" />{{ t('skip') }}
            </button>
            <span v-if="skipSpent" class="gate" :title="skipHint" @click="refuse('skip', skipHint, $event)" />
            <Transition name="whypop">
              <span v-if="refused === 'skip'" class="why" role="status">{{ refuseMsg }}</span>
            </Transition>
          </div>
        </div>
      </div>

      <aside class="side">
        <div class="panel score-panel" data-test="session-score">
          <h3 class="section-title">{{ t('score') }}</h3>
          <p class="score num">{{ sessionScore }}</p>
          <!-- what the run has banked so far, mirroring the game-over summary -->
          <div v-if="player && !player.isGuest && (sessionExp > 0 || sessionCoins > 0)" class="payouts">
            <span v-if="sessionExp > 0" class="payout num">+{{ sessionExp }} EXP</span>
            <span v-if="sessionCoins > 0" class="payout num"><Icon name="coin" :size="13" />{{ sessionCoins }}</span>
          </div>
        </div>
        <StepHistory :history="hand?.history ?? []" :can-undo="canUndo" @undo="game.undo" />
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
    <TimeExtendModal
      :show="!!extendOffer"
      :qty="bagTimeItems"
      :busy="extendBusy"
      @resolve="onExtendResolve"
    />
    <ResultModal
      :show="phase === 'result' || sessionEnd"
      v-bind="result ?? {}"
      :player="result?.player ?? player"
      :session-score="sessionScore"
      :session-exp="sessionExp"
      :session-coins="sessionCoins"
      :session-end="sessionEnd"
      @next="onNext"
    />
    <ConfirmModal
      :show="exitConfirm"
      :title="t('exitConfirmTitle')"
      :body="t('exitConfirmBody')"
      :ok-label="t('endGame')"
      :cancel-label="t('cancel')"
      danger
      @confirm="confirmExit"
      @cancel="exitConfirm = false"
    />
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
/* each helper owns its reason bubble: a transparent gate lies over the dark
   button so a press is catchable (a disabled button swallows clicks) */
.helper { position: relative; display: flex; }
.helper .btn { flex: 1 1 auto; }
.gate { position: absolute; inset: 0; z-index: 2; cursor: not-allowed; -webkit-tap-highlight-color: transparent; }
.why {
  position: absolute;
  bottom: calc(100% + 9px);
  left: 50%;
  transform: translateX(-50%);
  z-index: 60;
  width: max-content;
  max-width: 240px;
  padding: 9px 13px;
  border-radius: var(--r-sm);
  background: var(--surface-3);
  border: 1px solid var(--line);
  box-shadow: var(--sh-3);
  color: var(--text);
  font-size: 0.8rem;
  line-height: 1.45;
  text-align: center;
  pointer-events: none;
}
.why::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 6px solid transparent;
  border-top-color: var(--line);
}
/* the bubble's centering transform must survive the whole run: a bare
   rise-in animates transform too and would strip translateX(-50%), so the
   keyframes restate it — otherwise the bubble jumps sideways mid-pop */
@keyframes why-pop {
  from { opacity: 0; transform: translateX(-50%) translateY(9px) scale(0.88); }
  to { opacity: 1; transform: translateX(-50%) translateY(0) scale(1); }
}
.whypop-enter-active { animation: why-pop 0.26s var(--ease-out-back); transform-origin: bottom center; }
.whypop-leave-active { transition: opacity 0.16s var(--ease), transform 0.16s var(--ease); }
.whypop-leave-to { opacity: 0; transform: translateX(-50%) translateY(4px) scale(0.96); }
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
/* session score tally, fed by every solved hand */
.score-panel { flex: none; padding: 13px 16px; display: flex; flex-direction: column; gap: 6px; }
.score { font-size: 1.55rem; font-weight: 700; line-height: 1; color: var(--text); }
.payouts { display: flex; flex-wrap: wrap; gap: 3px 14px; }
.payout { display: inline-flex; align-items: center; gap: 5px; font-size: 0.82rem; font-weight: 600; color: var(--text-dim); }
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
