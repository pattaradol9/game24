<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { useRoom } from '../composables/useRoom.js'
import { sfx } from '../audio.js'
import { shake, popText, danger, haptic, centerOf } from '../fx.js'
import GameBoard from '../components/GameBoard.vue'
import StepHistory from '../components/StepHistory.vue'
import PlayersRail from '../components/PlayersRail.vue'
import RoundSummary from '../components/RoundSummary.vue'
import FinalPodium from '../components/FinalPodium.vue'
import Countdown from '../components/Countdown.vue'
import HostWaitIcon from '../components/HostWaitIcon.vue'
import Icon from '../components/Icon.vue'
import ModeBadge from '../components/ModeBadge.vue'
import PlayerChip from '../components/PlayerChip.vue'
import BuffBar from '../components/BuffBar.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import { modeMeta } from '../modes.js'
import { burst } from '../confetti.js'
import { currentPlayer, getPlayer } from '../auth.js'
import NicknameModal from '../components/NicknameModal.vue'
import RenameModal from '../components/RenameModal.vue'

const { t, lang } = useI18n()
const route = useRoute()
const router = useRouter()
const room = useRoom()
const {
  state, you, players, config, roundNo, totalRounds,
  numbers, remaining, timeLimit, hand, roundResult, matchResult, hint, intro,
  wrongFlash, error, isHost, combo, hostDeadline, achievementPops,
  extendExtra, hintUsedRound, extendUsedRound, regenUsedRound, autoNextAt,
  hostLeftEnd,
} = room

const myWins = computed(() => players.value.find((p) => p.id === you.value)?.wins ?? 0)
const myHints = computed(() => players.value.find((p) => p.id === you.value)?.hintsLeft ?? 0)
const myExtends = computed(() => players.value.find((p) => p.id === you.value)?.extendsLeft ?? 0)
const myRegens = computed(() => players.value.find((p) => p.id === you.value)?.regensLeft ?? 0)
// this seat already solved the round: the board parks until the summary
const mySolved = computed(() => !!players.value.find((p) => p.id === you.value)?.solved)
// done seats (solved, or their own clock ran out) just wait out the round —
// no urgency, no countdown beats on a clock that no longer matters
const myDone = computed(() => mySolved.value || remaining.value <= 0)
const urgent = computed(() => remaining.value <= 10 && state.value === 'round' && !myDone.value)

// achievement toast styling; the server pushes these after a round win
const ACH_TIERS = {
  bronze: '#cd7f32',
  silver: '#c0c0c0',
  gold: '#ffd700',
  platinum: '#7de3e1',
  legend: '#b283f0',
}
const achTierColor = (tier) => ACH_TIERS[String(tier || '').toLowerCase()] ?? ACH_TIERS.bronze
const achTitle = (a) => a.title?.[lang.value] ?? a.title?.en ?? a.id
// no session yet (direct room link in a fresh browser): ask for a name first
const needName = ref(!getPlayer())
const showRename = ref(false)
const hasPlayer = computed(() => !!currentPlayer.value)
const mood = computed(() => {
  if (state.value === 'summary' && roundResult.value?.winner === you.value) return 'happy'
  if (state.value === 'summary') return 'dizzy'
  if (hand.value && hand.value.steps.length > 0) return 'thinking'
  return 'idle'
})
const timePct = computed(() =>
  timeLimit.value > 0 ? Math.max(0, Math.min(100, (remaining.value / timeLimit.value) * 100)) : 0
)
// the round counter clamps at the match length: the server bumps roundNo
// one past the last round when the match ends
const matchRound = computed(() => {
  const total = totalRounds.value || config.value?.rounds || 0
  return Math.min(Math.max(roundNo.value, 0), total)
})
const roundPct = computed(() => {
  const total = totalRounds.value || config.value?.rounds || 0
  return total > 0 ? Math.min(100, (matchRound.value / total) * 100) : 0
})
// the summary's host button closes the match once the last round is done
const summaryLastRound = computed(() =>
  roundNo.value >= (totalRounds.value || config.value?.rounds || 0)
)
watch(urgent, (u) => danger(u && state.value === 'round' ? 0.85 : 0))

/* ---------- toast bubbles, cloned from the solo play room ---------- */
const toast = ref('')
const timers = []
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

watch(state, (s) => {
  if (s !== 'round') danger(0)
  if (s === 'summary') {
    const mine = roundResult.value?.standings?.find((p) => p.id === you.value)
    say(mine?.gained > 0 ? t('bubbleWin') : t('bubbleTime'), 3200)
  }
})

// the seconds the last Add-time press granted, toasted once
watch(extendExtra, (n) => {
  if (n > 0) say(t('timeExtendToast', { n }), 2400)
})

// host seat empty: warn after a short debounce so a quick refresh never
// flashes the dialog, and count down to the room closing
const hostWaitVisible = ref(false)
const hostCountdown = ref(0)
let hostWaitIv = null
watch(hostDeadline, (v) => {
  clearInterval(hostWaitIv)
  hostWaitVisible.value = false
  if (!v) {
    hostCountdown.value = 0
    return
  }
  const showAt = Date.now() + 2500
  hostWaitIv = setInterval(() => {
    hostCountdown.value = Math.max(0, Math.ceil((v - Date.now()) / 1000))
    hostWaitVisible.value = Date.now() >= showAt && hostCountdown.value > 0
  }, 250)
})

const celebrated = new Set()
let celebrateIv = null

// celebrate once per new round where this seat solved (banked points)
function watchish() {
  let lastRound = 0
  celebrateIv = setInterval(() => {
    const mine = roundResult.value?.standings?.find((p) => p.id === you.value)
    if (mine?.gained > 0 && roundNo.value !== lastRound) {
      lastRound = roundNo.value
      burst()
      haptic([0, 40, 60, 40])
    }
    if (state.value === 'finished') clearInterval(celebrateIv)
  }, 300)
}
watchish()

/* ---------- helper row: solution · add-time · new-hand — every helper is
   backed by the room's own quotas (set at creation, identical for all
   seats); no one's personal inventory is ever touched here ---------- */
const playing = computed(() => state.value === 'round' && !intro.value)

// Add-time spends one of the seat's room-granted uses for a private +30s,
// only while the clock still has room under the mode's limit; every helper
// also works once per round — a spent helper is dark with its own reason
const canExtend = computed(() =>
  playing.value && !mySolved.value && myExtends.value > 0 && !extendUsedRound.value && remaining.value < timeLimit.value
)
const extendHint = computed(() => {
  if (mySolved.value) return t('helperAfterSolve')
  if (extendUsedRound.value) return t('extendOnceRound')
  if (myExtends.value <= 0) return t('extendsSpent')
  if (remaining.value >= timeLimit.value) return t('timeFullLimit')
  return ''
})
const extendSpent = computed(() => playing.value && !!extendHint.value)

// quota-backed helpers go dark when the match's allowance is gone, the
// helper already ran once this round, or the seat has solved
const hintSpent = computed(() => playing.value && (mySolved.value || myHints.value <= 0 || hintUsedRound.value))
const hintWhy = computed(() => {
  if (mySolved.value) return t('helperAfterSolve')
  if (hintUsedRound.value) return t('hintOnceRound')
  return t('hintsSpent')
})
const regenSpent = computed(() => playing.value && (mySolved.value || myRegens.value <= 0 || regenUsedRound.value))
const regenWhy = computed(() => {
  if (mySolved.value) return t('helperAfterSolve')
  if (regenUsedRound.value) return t('regenOnceRound')
  return t('regensSpent')
})

// a dead own clock (someone else still extends) parks the board
const boardLocked = computed(() =>
  intro.value || mySolved.value || (state.value === 'round' && remaining.value <= 0)
)

// Undo lives in the Steps panel now — active only while there is a step back
// and the hand is still being played
const canUndo = computed(() =>
  playing.value && !mySolved.value && (hand.value?.history?.length ?? 0) > 0
)

// The deal fanfare is a one-shot, exactly like the solo game's 700ms window:
// the cards fly in when a fresh hand lands (round start, regen), then the
// flag stays off — so an undo (3 cards back to 4) replays the solo board's
// plain pop-in + glide instead of staging a second deal. Deriving this from
// the board shape (4 untouched cards) would flip it back on every undo.
const dealing = ref(false)
let dealTimer = 0
function dealOnce() {
  dealing.value = true
  clearTimeout(dealTimer)
  dealTimer = later(() => (dealing.value = false), 700)
}
watch(roundNo, dealOnce) // round start: every seat deals a fresh hand
watch(numbers, dealOnce) // regen: a brand-new hand flies in

// a dark helper explains itself where it stands: pressing it pops a bubble
// over the button naming the reason. Disabled buttons swallow clicks, so a
// transparent gate lies over the dark button to catch the press.
const refused = ref('') // which helper is talking: hint | extend | skip | regen
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
// The module is code-split and boots its one shared canvas on the first
// press only; reduced motion never boots it at all.
function flourish(el) {
  const r = el?.getBoundingClientRect?.()
  if (!r) return
  import('../three/popupFx.js')
    .then((m) => m.glint(r.left + r.width / 2, r.top))
    .catch(() => {})
}

// final local merge that isn't 24 → juicy failure
watch(
  () => hand.value?.cards.length,
  (nv, ov) => {
    if (state.value !== 'round') return
    if (nv === 1 && ov === 2 && hand.value && !hand.value.won) {
      shake(1.2)
      sfx.wrong()
      haptic([0, 60, 40, 60])
      say(t('bubbleWrong'), 2400)
    }
  }
)

// merge streak, mirrored from the solo board: a pop over the board center
watch(combo, (c) => {
  if (c >= 2 && state.value === 'round') {
    const center = centerOf(document.querySelector('.room .board'))
    if (center) popText(center.x, center.y - 20, `${t('combo')} ×${c}`, 'fx-combo')
    say(t('bubbleCombo'), 1600)
  }
})

onMounted(() => {
  if (!needName.value) room.open(route.params.code, route.query.hostKey || '')
})
onUnmounted(() => {
  clearInterval(celebrateIv)
  clearInterval(hostWaitIv)
  clearTimeout(toastId)
  timers.forEach(clearTimeout)
  danger(0)
  import('../three/popupFx.js').then((m) => m.disposeGlint()).catch(() => {})
  room.leave()
})

function onNamed() {
  needName.value = false
  room.open(route.params.code, route.query.hostKey || '')
}

// leaving mid-round asks first — the open hand scores nothing either way,
// but the seat only survives the trip through the reconnect grace. The host
// leaving a summary asks too: their goodbye ends the match for everyone.
const leaveConfirm = ref(false)
function onExit() {
  if (state.value === 'round' || (isHost.value && state.value === 'summary')) {
    leaveConfirm.value = true
    return
  }
  backHome()
}
function confirmLeave() {
  leaveConfirm.value = false
  if (!isHost.value || state.value === 'lobby') {
    backHome()
    return
  }
  // the host's goodbye ends the match for every seat: the server answers
  // with the final podium — stay for it, home is one press from there. A
  // dead socket falls back to plain routing.
  room.leaveMatch()
  later(() => {
    if (state.value !== 'finished' && state.value !== 'gone') backHome()
  }, 2500)
}
function backHome() {
  router.push('/')
}
function reload() {
  location.reload()
}
function copyLink() {
  // the hostKey is the host's secret — shared links must not carry it,
  // otherwise whoever opens the link could claim the host seat
  const url = new URL(location.href)
  url.searchParams.delete('hostKey')
  navigator.clipboard
    ?.writeText(url.toString())
    .then(() => say(t('copied')))
    .catch(() => {})
}

// the room code chip is a one-tap copy too
function copyCode() {
  navigator.clipboard
    ?.writeText(room.code.value)
    .then(() => say(t('copied')))
    .catch(() => {})
}

const mmss = computed(() =>
  `${Math.floor(remaining.value / 60)}:${String(remaining.value % 60).padStart(2, '0')}`
)
</script>

<template>
  <main class="wrap room">
    <header class="top">
      <button class="btn back" @click="onExit">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <ModeBadge
        v-if="config"
        :mode="config.mode"
        :round="`${t('round')} ${roundNo}/${totalRounds || config.rounds}`"
      />
      <button class="chip code" :title="t('tapToCopy')" @click="copyCode">#{{ room.code.value }}</button>
      <div class="spacer" />
      <BuffBar class="room-buffs" />
      <button
        v-if="hasPlayer"
        class="who panel"
        :title="t('profileRename')"
        @click="showRename = true"
      >
        <PlayerChip compact />
        <Icon class="edit-hint" name="edit" :size="14" />
      </button>
      <button class="btn" @click="copyLink"><Icon name="link" :size="17" />{{ t('share') }}</button>
    </header>

    <div v-if="state === 'round'" class="clock" :class="{ urgent }">
      <Icon name="clock" :size="17" />
      <span class="mmss num">{{ mmss }}</span>
      <span class="track"><span class="fill" :style="{ width: timePct + '%' }" /></span>
    </div>

    <p v-if="error" class="err chip">{{ error }}</p>

    <div class="stage">
      <div class="board-wrap" :class="{ flash: wrongFlash }">
        <div v-if="state === 'lobby'" class="panel lobby">
          <span class="section-title">#{{ room.code.value }}</span>
          <h2>{{ t('waitingHost') }}</h2>

          <!-- the lobby card briefs the match instead of repeating the
               roster: the rules, the identical per-seat helper allowances
               and how the score resolves — seats live in the side rail -->
          <div v-if="config" class="brief">
            <div class="brief-sec">
              <span class="brief-head"><Icon name="sliders" :size="14" />{{ t('matchRules') }}</span>
              <div class="brief-rows">
                <span class="brief-row">
                  <i>{{ t('ruleMode') }}</i>
                  <b>{{ t(config.mode) }} ×{{ modeMeta(config.mode).mult }}</b>
                </span>
                <span class="brief-row">
                  <i>{{ t('rounds') }}</i>
                  <b class="num">{{ config.rounds }}</b>
                </span>
                <span class="brief-row">
                  <i>{{ t('ruleScoring') }}</i>
                  <b class="soft">{{ t('ruleScoringFormula') }}</b>
                </span>
              </div>
            </div>

            <!-- helpers + score recap share one compact band: each takes a
                 single horizontal row — title in its own column, content
                 flowing beside it, a hairline between the two rows -->
            <div class="brief-band">
              <div class="band-row">
                <span class="band-head"><Icon name="sparkles" :size="14" />{{ t('matchHelpers') }}</span>
                <div class="band-chips">
                  <span class="brief-chip"><Icon name="bulb" :size="15" />{{ t('solution') }} <b>×{{ config.hintQuota }}</b></span>
                  <span class="brief-chip"><Icon name="hourglass" :size="15" />{{ t('addTime') }} <b>×{{ config.extendQuota }}</b></span>
                  <span class="brief-chip"><Icon name="refresh" :size="15" />{{ t('regen') }} <b>×{{ config.regenQuota }}</b></span>
                </div>
                <p class="band-note">{{ t('helpersNote') }}</p>
              </div>
              <div class="band-sep" role="presentation" />
              <div class="band-row">
                <span class="band-head"><Icon name="trophy" :size="14" />{{ t('scoreRecap') }}</span>
                <p class="band-note">{{ t('scoreRecapNote') }}</p>
              </div>
            </div>
          </div>

          <button v-if="isHost" class="btn primary big" @click="room.start">{{ t('startMatch') }}</button>
        </div>

        <!-- the board keeps its exact footprint in every state: a done seat
             (solved / own clock ran out) and the round summary blur it in
             place with a status veil — it never collapses -->
        <div v-else-if="(state === 'round' || state === 'summary') && hand" class="board-stage">
          <GameBoard
            :hand="hand"
            :disabled="boardLocked"
            :hint-data="hint"
            :dealing="dealing"
            :skin="currentPlayer?.skin"
            @pick="room.pickCard"
            @op="room.setOperator"
          />
          <div v-if="state === 'summary' || myDone" class="board-veil">
            <div v-if="state === 'round'" class="veil-copy">
              <h2 :class="mySolved ? 'good' : 'bad'">{{ mySolved ? t('solvedTitle') : t('timeUp') }}</h2>
              <p class="veil-body">{{ mySolved ? t('solvedWaitBody') : t('waitOthersBody') }}</p>
            </div>
          </div>
        </div>
        <div v-else-if="state === 'connecting'" class="loading">…</div>
        <div v-else-if="state === 'gone' || state === 'lost'" class="panel lobby">
          <h2>{{ state === 'gone' ? t('roomGone') : t('connLost') }}</h2>
          <div class="cta">
            <button class="btn primary big" @click="backHome">{{ t('backHome') }}</button>
            <button v-if="state === 'lost'" class="btn big" @click="reload">{{ t('retry') }}</button>
          </div>
        </div>

        <!-- the steps history rides under the board as a horizontal strip,
             undo pinned at its far right end; it stays through the summary
             so the column never jumps -->
        <StepHistory
          v-if="state === 'round' || state === 'summary'"
          horizontal
          :history="hand?.history ?? []"
          :can-undo="canUndo"
          @undo="room.undo"
        />

        <div class="actions">
          <div class="helper">
            <button class="btn" :disabled="!playing || mySolved || myHints <= 0 || hintUsedRound" @click="room.askHint">
              <Icon name="bulb" :size="17" />{{ t('solution') }}
            </button>
            <span v-if="hintSpent" class="gate" :title="hintWhy" @click="refuse('hint', hintWhy, $event)" />
            <Transition name="whypop">
              <span v-if="refused === 'hint'" class="why" role="status">{{ refuseMsg }}</span>
            </Transition>
          </div>
          <div class="helper">
            <button
              class="btn"
              :disabled="!canExtend"
              :title="extendHint"
              data-test="room-add-time"
              @click="room.extend"
            >
              <Icon name="hourglass" :size="17" />{{ t('addTime') }}
            </button>
            <span v-if="extendSpent" class="gate" :title="extendHint" @click="refuse('extend', extendHint, $event)" />
            <Transition name="whypop">
              <span v-if="refused === 'extend'" class="why" role="status">{{ refuseMsg }}</span>
            </Transition>
          </div>
          <div class="helper">
            <button class="btn" :disabled="!playing || mySolved || myRegens <= 0 || regenUsedRound" @click="room.askRegen">
              <Icon name="refresh" :size="17" />{{ t('regen') }}
            </button>
            <span v-if="regenSpent" class="gate" :title="regenWhy" @click="refuse('regen', regenWhy, $event)" />
            <Transition name="whypop">
              <span v-if="refused === 'regen'" class="why" role="status">{{ refuseMsg }}</span>
            </Transition>
          </div>
        </div>
      </div>

      <aside class="side">
        <PlayersRail :players="players" :you="you" />
        <!-- match progress: which round the seats are on, out of the total;
             one slim 44px row so it sits exactly level with the helper
             button group across the column gap (hidden before the host
             starts — there is no round to count yet) -->
        <div v-if="config && state !== 'lobby'" class="panel round-panel">
          <span class="round-label">{{ t('round') }}</span>
          <span class="track"><span class="fill" :style="{ width: roundPct + '%' }" /></span>
          <span class="round-count num">{{ matchRound }} / {{ totalRounds || config.rounds }}</span>
        </div>
      </aside>
    </div>

    <Transition name="toast">
      <p v-if="toast" class="toast">{{ toast }}</p>
    </Transition>

    <Countdown v-if="intro && state === 'round'" @done="room.introDone" />
    <RoundSummary
      :show="state === 'summary'"
      :result="roundResult"
      :round-no="roundNo"
      :you="you"
      :is-host="isHost"
      :last-round="summaryLastRound"
      :auto-next-at="autoNextAt"
      @next="room.nextRound"
    />
    <FinalPodium :show="state === 'finished'" :standings="matchResult" :host-left="hostLeftEnd" @home="backHome" />

    <!-- achievements unlocked by winning a round: stacked self-dismissing
         toasts, always non-blocking (they may land over a round summary) -->
    <div class="ach-toasts" aria-live="polite">
      <TransitionGroup name="achtoast">
        <div v-for="a in achievementPops" :key="a.key" class="ach-toast" :style="{ '--tc': achTierColor(a.tier) }">
          <Icon class="ach-ic" name="trophy" :size="18" />
          <span class="ach-txt">
            <span class="ach-label">{{ t('achievementUnlocked') }}</span>
            <b class="ach-name">{{ achTitle(a) }}</b>
          </span>
        </div>
      </TransitionGroup>
    </div>

    <!-- no <Transition>: leave animations never finish in background tabs
         (rendering is frozen, so transitionend never fires) and the dialog
         would linger on screen after the host reconnects -->
    <div v-if="hostWaitVisible" class="overlay host-wait">
      <div class="wait-panel panel">
        <HostWaitIcon />
        <h2>{{ t('hostWaitTitle') }}</h2>
        <p class="wait-body">{{ t('hostWaitBody') }}</p>
        <p class="wait-count num">{{ t('hostWaitClose', { n: hostCountdown }) }}</p>
        <!-- the notice stays click-through so the match can keep running;
             only this button takes pointer events — leaving is a decision -->
        <button class="btn danger wait-leave" data-test="host-wait-leave" @click="backHome">
          <Icon name="logout" :size="16" />{{ t('hostWaitLeave') }}
        </button>
      </div>
    </div>

    <ConfirmModal
      :show="leaveConfirm"
      :title="t('leaveTitle')"
      :body="isHost && state !== 'lobby' ? t('hostLeaveBody') : t('leaveBody')"
      :ok-label="t('leaveMatch')"
      :cancel-label="t('cancel')"
      danger
      @confirm="confirmLeave"
      @cancel="leaveConfirm = false"
    />
    <NicknameModal v-if="needName" @done="onNamed" />
    <RenameModal v-if="showRename" @saved="room.rename" @close="showRename = false" />
  </main>
</template>

<style scoped>
.room { display: flex; flex-direction: column; flex: 1 1 auto; gap: 14px; padding: 18px 0 26px; }
.top { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
@media (hover: hover) { .back:hover { color: var(--text); } }
.code {
  cursor: pointer;
  font: inherit;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.06em;
  transition: border-color 0.15s var(--ease), background-color 0.15s var(--ease);
}
/* same hover language as every .btn: gray line + lighter surface */
@media (hover: hover) {
  .code:hover { border-color: #35405a; background-color: var(--surface-3); }
}
.who {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px 7px 7px;
  border-radius: var(--r-md);
  box-shadow: none;
  cursor: pointer;
  font: inherit;
  color: var(--text);
}
.edit-hint { color: var(--text-mute); transition: color 0.15s var(--ease); }
@media (hover: hover) {
  .who:hover { border-color: rgba(246, 183, 60, 0.4); }
  .who:hover .edit-hint { color: var(--accent); }
}
.spacer { flex: 1; }
/* boost tray rides in the header on desktop; phones keep the HUD minimal */
.room-buffs { display: none; }
@media (min-width: 641px) {
  .room-buffs { display: flex; }
}
.err { color: var(--bad); border-color: rgba(239, 95, 95, 0.4); align-self: flex-start; }

.clock { display: flex; align-items: center; gap: 12px; color: var(--text-dim); transition: color 0.3s var(--ease); }
.mmss { font-size: 1.05rem; font-weight: 600; color: var(--text); min-width: 3.4ch; }
.track { flex: 1; height: 4px; border-radius: var(--r-full); background: var(--surface-2); overflow: hidden; }
.fill {
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: var(--accent);
  transition: width 0.3s linear, background 0.3s var(--ease);
}
.clock.urgent, .clock.urgent .mmss { color: var(--bad); }
.clock.urgent .fill { background: var(--bad); }

.stage {
  display: grid;
  grid-template-columns: minmax(0, 1fr) clamp(230px, 24vw, 300px);
  grid-template-areas: 'board side';
  gap: 16px 20px;
  flex: 1 1 auto;
  align-content: center;
  min-height: 0;
}
.board-wrap { grid-area: board; min-width: 0; display: flex; flex-direction: column; }
.board-wrap.flash { animation: shake 0.3s var(--ease); }
/* the steps strip sits between the board and the helper row */
.board-wrap :deep(.history) { margin-top: 12px; }
.lobby { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px; padding: 26px; }
.lobby h2 { font-size: 1.2rem; font-weight: 500; }
.cta { display: flex; gap: 12px; flex-wrap: wrap; justify-content: center; }
/* the lobby reserves the play column's exact footprint — the board plus the
   steps strip under it — so nothing on the page jumps when the host starts
   the match. The calc mirrors GameBoard's own construction (padding clamps,
   card row --card-w*1.45, inner gaps, operator pad, tip line) + strip 62 + its
   12px margin; below 901px the stage stacks and the panel sizes naturally */
@media (min-width: 901px) {
  .lobby {
    height: calc(
      clamp(20px, 4vw, 34px) * 2 + 2px /* board padding + border */
      + var(--card-w) * 1.45 /* card row */
      + clamp(18px, 3.2vw, 28px) * 2 /* board inner gaps */
      + clamp(52px, 13vw, 66px) + 18px /* operator pad + its padding/border */
      + 19px /* tip line */
      + 12px + 62px /* steps strip + its margin */
    );
  }
}
/* lobby match brief: rules / helper allowances / score recap, replacing the
   duplicated roster — the seats themselves stay in the side rail */
.brief { display: flex; flex-direction: column; gap: 10px; width: min(600px, 100%); }
.brief-sec {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 12px 14px;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  background: var(--surface-2);
  text-align: left;
}
.brief-head {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 0.72rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-mute);
}
.brief-rows { display: flex; flex-direction: column; gap: 6px; }
.brief-row { display: flex; align-items: baseline; justify-content: space-between; gap: 14px; font-size: 0.9rem; }
.brief-row i { font-style: normal; color: var(--text-dim); white-space: nowrap; }
.brief-row b { font-weight: 600; color: var(--text); text-align: right; }
.brief-row b.soft { font-weight: 500; color: var(--text-mute); }
/* helpers and the score recap share one compact band: two horizontal rows,
   titles in a fixed column so chips and notes align across the rows; on
   narrow screens each row's content wraps gracefully under its title */
.brief-band {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  background: var(--surface-2);
  text-align: left;
}
.band-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.band-head {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  flex: 0 0 150px;
  font-size: 0.72rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-mute);
}
.band-chips { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; flex: 1 1 auto; }
.brief-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  background: var(--surface-3);
  padding: 7px 11px;
  font-size: 0.82rem;
  line-height: 1;
  color: var(--text-dim);
  white-space: nowrap;
}
.brief-chip b { color: var(--accent); font-weight: 600; font-variant-numeric: tabular-nums; }
.band-note {
  flex: 1 1 220px;
  margin: 0;
  font-size: 0.82rem;
  line-height: 1.45;
  color: var(--text-mute);
}
.band-sep { height: 1px; background: var(--line); }

/* done seats and the summary keep the board's exact footprint: the cards
   blur in place under a veil carrying the status copy */
.board-stage { position: relative; }
.board-veil {
  position: absolute;
  inset: 0;
  z-index: 4;
  display: grid;
  place-items: center;
  background: rgba(6, 8, 13, 0.4);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border-radius: var(--r-lg);
}
.veil-copy { display: flex; flex-direction: column; gap: 8px; text-align: center; padding: 20px; }
.veil-copy h2 { font-size: 1.5rem; font-weight: 600; }
.veil-copy h2.good { color: var(--good); }
.veil-copy h2.bad { color: var(--bad); }
.veil-body { color: var(--text-dim); font-size: 0.92rem; line-height: 1.5; max-width: 34ch; }
.actions {
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
/* the roster fills the column; the round counter stays pinned at its bottom,
   one slim row matching the helper buttons' 44px height across the gap.
   On desktop the board column dictates the row height — the roster's own
   natural height must not push the column taller (it scrolls instead) —
   while phones stack the stage and let the roster size naturally */
.side :deep(.rail) { flex: 1 1 auto; min-height: 0; overflow-y: auto; }
@media (min-width: 901px) {
  .side :deep(.rail) { flex: 1 1 0; }
}
.round-panel {
  flex: none;
  min-height: 44px;
  padding: 6px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}
.round-label {
  font-size: 0.72rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-mute);
  white-space: nowrap;
}
.round-count { font-size: 0.95rem; font-weight: 600; color: var(--text); white-space: nowrap; }
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

/* ---------- host reconnecting notice (non-blocking) ---------- */
.host-wait {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 20px;
  pointer-events: none; /* the match keeps running — let clicks through */
  background: rgba(6, 8, 13, 0.55);
  z-index: 120;
}
.wait-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
  width: min(380px, 100%);
  padding: 26px;
}
.wait-panel h2 { font-size: 1.15rem; font-weight: 500; }
.wait-body { color: var(--text-dim); font-size: 0.9rem; line-height: 1.5; }
.wait-count { color: var(--accent); font-weight: 600; }
.wait-leave { pointer-events: auto; margin-top: 8px; min-height: 42px; }

/* ---------- achievement toasts (non-blocking) ---------- */
.ach-toasts {
  position: fixed;
  top: calc(14px + env(safe-area-inset-top));
  left: 50%;
  transform: translateX(-50%);
  z-index: 150;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  pointer-events: none;
  width: min(340px, calc(100vw - 32px));
}
.ach-toast {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  border: 1px solid color-mix(in srgb, var(--tc) 50%, transparent);
  background: var(--surface-2);
  border-radius: var(--r-md);
  box-shadow: var(--sh-2);
  padding: 10px 14px;
}
.ach-ic { color: var(--tc); flex: none; }
.ach-txt { display: flex; flex-direction: column; gap: 1px; min-width: 0; }
.ach-label { font-size: 0.66rem; font-weight: 500; letter-spacing: 0.1em; text-transform: uppercase; color: var(--tc); }
.ach-name { font-size: 0.9rem; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.achtoast-enter-active { animation: rise-in 0.28s var(--ease-out-back); }
.achtoast-leave-active { transition: opacity 0.3s var(--ease), transform 0.3s var(--ease); }
.achtoast-leave-to { opacity: 0; transform: translateY(-10px); }
.achtoast-move { transition: transform 0.3s var(--ease); }

@media (max-width: 900px) {
  .room { padding-bottom: calc(88px + env(safe-area-inset-bottom)); }
  .stage {
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas: 'board' 'side';
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
  .actions .btn { font-size: 0.82rem; padding-inline: 6px; gap: 4px; }
}
</style>
