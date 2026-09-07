<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { useRoom } from '../composables/useRoom.js'
import { sfx } from '../audio.js'
import { shake, danger, haptic } from '../fx.js'
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
import { burst } from '../confetti.js'
import { currentPlayer, getPlayer } from '../auth.js'
import NicknameModal from '../components/NicknameModal.vue'
import RenameModal from '../components/RenameModal.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const room = useRoom()
const {
  state, you, players, config, roundNo, totalRounds,
  numbers, remaining, timeLimit, hand, roundResult, matchResult, hint, intro,
  wrongFlash, error, isHost, combo, hostDeadline,
} = room

const myWins = computed(() => players.value.find((p) => p.id === you.value)?.wins ?? 0)
const myHints = computed(() => players.value.find((p) => p.id === you.value)?.hintsLeft ?? 0)
const myRegens = computed(() => players.value.find((p) => p.id === you.value)?.regensLeft ?? 0)
// no session yet (direct room link in a fresh browser): ask for a name first
const needName = ref(!getPlayer())
const showRename = ref(false)
const hasPlayer = computed(() => !!currentPlayer.value)
const urgent = computed(() => remaining.value <= 10 && state.value === 'round')
const mood = computed(() => {
  if (state.value === 'summary' && roundResult.value?.winner === you.value) return 'happy'
  if (state.value === 'summary') return 'dizzy'
  if (hand.value && hand.value.steps.length > 0) return 'thinking'
  return 'idle'
})
const timePct = computed(() =>
  timeLimit.value > 0 ? Math.max(0, Math.min(100, (remaining.value / timeLimit.value) * 100)) : 0
)
watch(urgent, (u) => danger(u && state.value === 'round' ? 0.85 : 0))
watch(state, (s) => { if (s !== 'round') danger(0) })

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

// celebrate once per new round result won by me
function watchish() {
  let lastRound = 0
  celebrateIv = setInterval(() => {
    if (roundResult.value?.winner === you.value && roundNo.value !== lastRound) {
      lastRound = roundNo.value
      burst()
      haptic([0, 40, 60, 40])
    }
    if (state.value === 'finished') clearInterval(celebrateIv)
  }, 300)
}
watchish()

// final local merge that isn't 24 → juicy failure
watch(
  () => hand.value?.cards.length,
  (nv, ov) => {
    if (state.value !== 'round') return
    if (nv === 1 && ov === 2 && hand.value && !hand.value.won) {
      shake(1.2)
      sfx.wrong()
      haptic([0, 60, 40, 60])
    }
  }
)

onMounted(() => {
  if (!needName.value) room.open(route.params.code, route.query.hostKey || '')
})
onUnmounted(() => {
  clearInterval(celebrateIv)
  clearInterval(hostWaitIv)
  danger(0)
  room.leave()
})

function onNamed() {
  needName.value = false
  room.open(route.params.code, route.query.hostKey || '')
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
  navigator.clipboard?.writeText(url.toString())
}

const mmss = computed(() =>
  `${Math.floor(remaining.value / 60)}:${String(remaining.value % 60).padStart(2, '0')}`
)
</script>

<template>
  <main class="wrap room">
    <header class="top">
      <button class="btn back" @click="backHome">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <ModeBadge
        v-if="config"
        :mode="config.mode"
        :round="`${t('round')} ${roundNo}/${totalRounds || config.rounds}`"
      />
      <span class="chip code">#{{ room.code.value }}</span>
      <div class="spacer" />
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
          <PlayersRail :players="players" :you="you" />
          <button v-if="isHost" class="btn primary big" @click="room.start">{{ t('startMatch') }}</button>
        </div>

        <GameBoard
          v-else-if="state === 'round' && hand"
          :hand="hand"
          :disabled="intro"
          :hint-data="hint"
          :dealing="hand.cards.length === 4 && hand.cards.every((c) => c.id.startsWith('c')) && combo === 0 && hand.steps.length === 0"
          @pick="room.pickCard"
          @op="room.setOperator"
        />
        <div v-else-if="state === 'connecting'" class="loading">…</div>
        <div v-else-if="state === 'gone' || state === 'lost'" class="panel lobby">
          <h2>{{ state === 'gone' ? t('roomGone') : t('connLost') }}</h2>
          <div class="cta">
            <button class="btn primary big" @click="backHome">{{ t('backHome') }}</button>
            <button v-if="state === 'lost'" class="btn big" @click="reload">{{ t('retry') }}</button>
          </div>
        </div>

        <div class="actions">
          <button class="btn" :disabled="state !== 'round' || intro || myHints <= 0" @click="room.askHint">
            <Icon name="bulb" :size="17" />{{ t('hint') }}
            <b v-if="myHints > 0" class="count num">{{ myHints }}</b>
          </button>
          <button class="btn" :disabled="state !== 'round' || intro" @click="room.undo">
            <Icon name="undo" :size="17" />{{ t('undo') }}
          </button>
          <button class="btn" :disabled="state !== 'round' || intro || myRegens <= 0" @click="room.askRegen">
            <Icon name="refresh" :size="17" />{{ t('regen') }}
            <b v-if="myRegens > 0" class="count num">{{ myRegens }}</b>
          </button>
        </div>
      </div>

      <aside class="side">
        <PlayersRail :players="players" :you="you" />
        <StepHistory :history="hand?.history ?? []" />
      </aside>
    </div>

    <Countdown v-if="intro && state === 'round'" @done="room.introDone" />
    <RoundSummary :show="state === 'summary'" :result="roundResult" :round-no="roundNo" />
    <FinalPodium :show="state === 'finished'" :standings="matchResult" @home="backHome" />

    <!-- no <Transition>: leave animations never finish in background tabs
         (rendering is frozen, so transitionend never fires) and the dialog
         would linger on screen after the host reconnects -->
    <div v-if="hostWaitVisible" class="overlay host-wait">
      <div class="wait-panel panel">
        <HostWaitIcon />
        <h2>{{ t('hostWaitTitle') }}</h2>
        <p class="wait-body">{{ t('hostWaitBody') }}</p>
        <p class="wait-count num">{{ t('hostWaitClose', { n: hostCountdown }) }}</p>
      </div>
    </div>

    <NicknameModal v-if="needName" @done="onNamed" />
    <RenameModal v-if="showRename" @saved="room.rename" @close="showRename = false" />
  </main>
</template>

<style scoped>
.room { display: flex; flex-direction: column; flex: 1 1 auto; gap: 14px; padding: 18px 0 26px; }
.top { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
@media (hover: hover) { .back:hover { color: var(--text); } }
.code { font-variant-numeric: tabular-nums; letter-spacing: 0.06em; }
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
.lobby { display: flex; flex-direction: column; align-items: center; gap: 18px; padding: 40px 26px; }
.lobby h2 { font-size: 1.2rem; font-weight: 500; }
.cta { display: flex; gap: 12px; flex-wrap: wrap; justify-content: center; }
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
.loading { text-align: center; color: var(--text-mute); font-size: 1.4rem; padding: 60px 0; }

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

@media (max-width: 900px) {
  .room { padding-bottom: calc(88px + env(safe-area-inset-bottom)); }
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
}
@media (max-width: 560px) {
  .back-label { display: none; }
  .back { padding: 0; width: 44px; justify-content: center; }
}
@media (max-width: 430px) {
  .actions .btn { font-size: 0.85rem; padding-inline: 6px; gap: 5px; }
}
</style>
