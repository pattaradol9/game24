<script setup>
// Header profile menu: shows the current session with rename / google
// sign-in / logout actions.
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from '../i18n/index.js'
import { currentPlayer, clearSession } from '../auth.js'
import { levelProgress, tierName } from '../core/progress.js'
import { MODES } from '../modes.js'
import { sfx } from '../audio.js'
import GoogleSignIn from './GoogleSignIn.vue'
import AnimatedIcon from './AnimatedIcon.vue'
import DeleteAccountModal from './DeleteAccountModal.vue'
import Icon from './Icon.vue'
import PlayerChip from './PlayerChip.vue'
import ProfileHalo from './ProfileHalo.vue'
import RenameModal from './RenameModal.vue'
import Suit from './Suit.vue'
import TierBadge from './TierBadge.vue'
import TierAvatar from './TierAvatar.vue'

const { t } = useI18n()
const emit = defineEmits(['require-name'])
// direction "up" marks the mobile bottom-bar instance: the menu renders as
// an app-style bottom sheet, teleported to <body> so it can anchor to the
// viewport edge and cover the nav with a scrim. A trigger slot replaces the
// default chip button.
const props = defineProps({
  direction: { type: String, default: 'down' }, // "down" | "up"
})
const sheet = computed(() => props.direction === 'up')

const open = ref(false)
const showRename = ref(false)
const showDelete = ref(false)
const wrap = ref(null)
const sheetEl = ref(null)
const error = ref('')

const player = computed(() => currentPlayer.value)
const prog = computed(() =>
  player.value ? levelProgress(player.value.totalExp) : null
)
// the tier name badge lives here and nowhere else; guests sit outside the
// ladder entirely, so they get the flat grey identity
const tier = computed(() =>
  player.value && !player.value.isGuest ? tierName(player.value.level) : 'guest'
)
const pct = computed(() => {
  const p = prog.value
  if (!p || !p.forNext) return 100
  return Math.min(100, Math.round((p.into / p.forNext) * 100))
})
// the per-mode high scores the leaderboard ranks; the block stays hidden
// until a hand actually scored, guests included — they rank on the board too
const scoreModes = computed(() =>
  MODES.filter((m) => (player.value?.highScores?.[m.id] ?? 0) > 0)
)
const fmt = (n) => n.toLocaleString('en-US')

function toggle() {
  sfx.click()
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocClick(e) {
  if (!open.value) return
  // the teleported sheet lives outside the trigger's subtree — count it in
  if (wrap.value?.contains(e.target) || sheetEl.value?.contains(e.target)) return
  close()
}
function onKey(e) {
  if (e.key === 'Escape') close()
}
onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})

// an open sheet locks the page behind it, like a native app
watch(open, (o) => {
  if (!sheet.value) return
  document.body.style.overflow = o ? 'hidden' : ''
})

function logout() {
  sfx.click()
  close()
  clearSession()
  // re-show the entry modal through the parent
  emit('require-name')
}

// the account is already gone server-side when the modal reports success;
// behave like a logout so the parent re-opens the entry flow
function onDeleted() {
  close()
  emit('require-name')
}
</script>

<template>
  <div ref="wrap" class="profile" :class="{ up: props.direction === 'up' }">
    <slot name="trigger" :open="open" :toggle="toggle">
      <!-- default header trigger -->
      <button v-if="!player" class="btn" @click="$emit('require-name')">
        <AnimatedIcon name="user" :size="17" />{{ t('profileStart') }}
      </button>
      <button v-else class="profile-btn" :class="{ open }" @click="toggle">
        <PlayerChip />
        <Icon class="caret" name="chevron-down" :size="14" />
      </button>
    </slot>

    <!-- the mobile sheet teleports to <body>: a scrim dims the page and the
         menu rises from the bottom edge, above the nav bar -->
    <Teleport to="body" :disabled="!sheet">
      <Transition name="fade">
        <div v-if="sheet && open && player" class="scrim" @click="close()" />
      </Transition>
      <Transition :name="sheet ? 'sheet' : 'drop'">
        <div v-if="open && player" class="menu" :class="{ 'sheet-menu': sheet }" ref="sheetEl">
          <span v-if="sheet" class="grab" aria-hidden="true" />
          <div class="hero">
            <ProfileHalo v-if="sheet" class="hero-halo" />
            <div class="avatar-wrap">
            <TierAvatar
              :tier="tier"
              :src="player.picture"
              :name="player.nickname"
              :size="62"
            />
            <span v-if="!player.isGuest" class="lv-chip">Lv.{{ player.level }}</span>
          </div>
          <b class="nick">{{ player.nickname }}</b>
          <div class="meta-row">
            <TierBadge :tier="tier" size="sm" />
            <span class="sub">{{ player.isGuest ? t('profileGuest') : t('profileGoogle') }}</span>
          </div>
        </div>

        <!-- the level ladder is signed-in territory: guests sit outside it,
             so the whole stats block (level row, bar, EXP, coins) stays hidden -->
        <div v-if="!player.isGuest" class="stats">
          <div class="lv-row">
            <span class="lv-now">Lv.{{ player.level }}</span>
            <span class="pct">{{ pct }}%</span>
            <span class="lv-next">Lv.{{ (prog && prog.lv + 1) || player.level + 1 }}</span>
          </div>
          <div class="bar">
            <span class="fill" :style="{ width: (prog && prog.forNext ? (prog.into / prog.forNext) * 100 : 100) + '%' }" />
          </div>
          <div class="statrow">
            <span>{{ prog ? `${fmt(prog.into)} / ${fmt(prog.forNext)}` : '—' }}</span>
            <span class="total">EXP {{ fmt(player.totalExp) }}</span>
          </div>
          <div class="statrow">
            <span class="coin-label"><Icon name="coin" :size="17" />{{ t('coins') }}</span>
            <span class="total num">{{ fmt(player.totalCoins ?? 0) }}</span>
          </div>
        </div>

        <!-- the leaderboard figure, per mode: shown for every player with a
             scored hand, guests included — it sits outside the level block
             because guests have no ladder to stand on -->
        <div v-if="scoreModes.length" class="scores">
          <div class="scores-head">
            <Icon name="crown" :size="13" />
            <span>{{ t('highScore') }}</span>
          </div>
          <div class="score-grid">
            <div v-for="m in scoreModes" :key="m.id" class="score-cell">
              <Suit :name="m.suit" :size="12" />
              <span class="num">{{ fmt(player.highScores[m.id] ?? 0) }}</span>
            </div>
          </div>
        </div>

        <div class="actions">
          <button class="item" @click="showRename = true; close()">
            <span class="ic"><AnimatedIcon name="edit" :size="15" /></span>
            <span class="lbl">{{ t('profileRename') }}</span>
            <AnimatedIcon class="go" name="chevron-right" :size="14" />
          </button>
          <RouterLink v-if="!player.isGuest" class="item" to="/achievements" @click="close()">
            <span class="ic"><Icon name="trophy" :size="15" /></span>
            <span class="lbl">{{ t('achievements') }}</span>
            <Icon class="go" name="chevron-right" :size="14" />
          </RouterLink>
          <RouterLink v-if="!player.isGuest" class="item" to="/shop" @click="close()">
            <span class="ic"><Icon name="bag" :size="15" /></span>
            <span class="lbl">{{ t('shop') }}</span>
            <Icon class="go" name="chevron-right" :size="14" />
          </RouterLink>
          <button v-if="player.isAdmin" class="item admin" @click="close(); $router.push('/admin')">
            <span class="ic"><AnimatedIcon name="crown" :size="15" /></span>
            <span class="lbl">{{ t('profileAdmin') }}</span>
            <AnimatedIcon class="go" name="chevron-right" :size="14" />
          </button>
          <div v-if="player.isGuest" class="gsi-wrap">
            <p class="guest-hint">{{ t('guestMenuHint') }}</p>
            <GoogleSignIn @signed-in="close()" />
          </div>
          <button v-else class="item danger" @click="logout">
            <span class="ic"><AnimatedIcon name="logout" :size="15" /></span>
            <span class="lbl">{{ t('profileLogout') }}</span>
            <AnimatedIcon class="go" name="chevron-right" :size="14" />
          </button>
          <div v-if="!player.isGuest" class="menu-sep" />
          <button v-if="!player.isGuest" class="item danger" @click="showDelete = true; close()">
            <span class="ic"><Icon name="trash" :size="15" /></span>
            <span class="lbl">{{ t('profileDelete') }}</span>
            <Icon class="go" name="chevron-right" :size="14" />
          </button>
          <p v-if="error" class="err">{{ error }}</p>
        </div>
      </div>
      </Transition>
    </Teleport>

    <RenameModal v-if="showRename" @close="showRename = false" />
    <DeleteAccountModal v-if="showDelete" @deleted="onDeleted" @close="showDelete = false" />
  </div>
</template>

<style scoped>
.profile { position: relative; }

/* ---------- trigger ---------- */
.profile-btn {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--line);
  background: linear-gradient(180deg, var(--surface-2), var(--surface));
  color: var(--text);
  border-radius: var(--r-md);
  padding: 6px 12px 6px 6px;
  min-height: 48px;
  font: inherit;
  font-size: 0.9rem;
  font-weight: 500;
  transition: border-color 0.15s var(--ease), background 0.15s var(--ease),
    box-shadow 0.2s var(--ease);
}
@media (hover: hover) {
  .profile-btn:hover {
    border-color: rgba(246, 183, 60, 0.4);
    background: linear-gradient(180deg, var(--surface-3), var(--surface-2));
  }
}
.profile-btn.open {
  border-color: rgba(246, 183, 60, 0.55);
  background: linear-gradient(180deg, var(--surface-3), var(--surface-2));
  box-shadow: 0 0 0 3px rgba(246, 183, 60, 0.09), 0 6px 20px rgba(0, 0, 0, 0.35);
}
.caret { color: var(--text-mute); transition: transform 0.2s var(--ease), color 0.15s var(--ease); }
.profile-btn.open .caret { transform: rotate(180deg); color: var(--accent); }

/* ---------- menu panel ---------- */
.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  /* wide enough for the 312px Google sign-in button inside .actions/.gsi-wrap */
  width: 340px;
  max-width: calc(100vw - 24px);
  border-radius: var(--r-lg);
  border: 1px solid var(--line);
  background: var(--surface-2);
  box-shadow: var(--sh-3), 0 10px 44px rgba(246, 183, 60, 0.07);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transform-origin: top right;
  z-index: 80;
}
/* mobile app-style bottom sheet (the up variant): teleported to <body> and
   anchored to the viewport's bottom edge, rising over the nav bar behind a
   scrim — no transform centring, the slide-up animation owns the transform */
.menu.sheet-menu {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  top: auto;
  width: auto;
  max-width: none;
  margin: 0;
  max-height: 86vh;
  overflow-y: auto;
  border-radius: var(--r-lg) var(--r-lg) 0 0;
  border-bottom: 0;
  padding-bottom: calc(12px + env(safe-area-inset-bottom));
  transform-origin: bottom center;
  z-index: 201;
}
.scrim {
  position: fixed;
  inset: 0;
  background: rgba(4, 6, 10, 0.62);
  z-index: 200;
}
.fade-enter-active, .fade-leave-active { transition: opacity 0.22s var(--ease); }
.fade-enter-from, .fade-leave-to { opacity: 0; }
.sheet-enter-active { animation: sheet-up 0.34s var(--ease-out-back); }
.sheet-leave-active { transition: transform 0.2s var(--ease), opacity 0.2s var(--ease); }
.sheet-leave-to { transform: translateY(100%); opacity: 0.4; }

/* the sheet's pull handle */
.grab {
  display: block;
  width: 38px;
  height: 4px;
  margin: 10px auto 0;
  border-radius: var(--r-full);
  background: var(--surface-3);
}

/* golden particle stage behind the avatar */
.hero-halo { z-index: 0; }

/* warm light spilling from the top edge */
.menu::before {
  content: '';
  position: absolute;
  top: 0;
  left: 12%;
  right: 12%;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(246, 183, 60, 0.6), transparent);
  pointer-events: none;
}

/* ---------- hero ---------- */
.hero {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 22px 20px 16px;
}
.hero::before {
  content: '';
  position: absolute;
  top: -34px;
  left: 50%;
  transform: translateX(-50%);
  width: 170px;
  height: 120px;
  background: radial-gradient(closest-side, rgba(246, 183, 60, 0.16), transparent);
  pointer-events: none;
}
.avatar-wrap { position: relative; margin-bottom: 10px; }
.lv-chip {
  position: absolute;
  bottom: -8px;
  left: 50%;
  transform: translateX(-50%);
  white-space: nowrap;
  background: linear-gradient(135deg, var(--accent-hi), var(--accent));
  color: var(--accent-ink);
  font-size: 0.68rem;
  font-weight: 700;
  line-height: 1;
  padding: 4px 9px;
  border-radius: var(--r-full);
  box-shadow: 0 2px 10px rgba(246, 183, 60, 0.35), 0 1px 3px rgba(0, 0, 0, 0.4);
}
.nick {
  position: relative;
  max-width: 100%;
  font-weight: 600;
  font-size: 1.05rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta-row { position: relative; display: flex; align-items: center; gap: 8px; }
.sub { font-size: 0.75rem; color: var(--text-mute); }

/* ---------- exp card ---------- */
.stats {
  margin: 0 16px;
  background: var(--bg);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-sm);
  padding: 11px 12px 10px;
}
.lv-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  font-size: 0.72rem;
  font-weight: 600;
  margin-bottom: 7px;
}
.lv-now { color: var(--accent); }
.pct { color: var(--text-dim); font-size: 0.68rem; }
.lv-next { color: var(--text-mute); }
.bar {
  position: relative;
  height: 6px;
  border-radius: var(--r-full);
  background: var(--surface-3);
  overflow: hidden;
}
.fill {
  position: relative;
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: linear-gradient(90deg, var(--accent), var(--accent-hi));
  box-shadow: 0 0 10px rgba(246, 183, 60, 0.5);
  overflow: hidden;
  transition: width 0.7s var(--ease);
}
.fill::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(100deg, transparent 30%, rgba(255, 255, 255, 0.5) 50%, transparent 70%);
  animation: chip-shine 2.6s var(--ease) infinite;
}
@keyframes chip-shine {
  to { transform: translateX(100%); }
}
.statrow {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-top: 7px;
  font-size: 0.72rem;
  color: var(--text-mute);
  font-variant-numeric: tabular-nums;
}
.total { color: var(--text-dim); }
.coin-label { display: inline-flex; align-items: center; gap: 5px; color: var(--accent); }

/* ---------- high scores ---------- */
.scores {
  margin: 8px 16px 0;
  background: var(--bg);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-sm);
  padding: 9px 12px 10px;
}
.scores-head {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-mute);
  margin-bottom: 7px;
}
.scores-head svg { color: var(--accent); }
.score-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
}
.score-cell {
  display: flex;
  align-items: center;
  gap: 7px;
  background: var(--surface-3);
  border-radius: var(--r-sm);
  padding: 6px 9px;
  font-size: 0.78rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.score-cell .num { margin-left: auto; color: var(--text-dim); }

/* ---------- actions ---------- */
.actions {
  display: flex;
  flex-direction: column;
  margin-top: 12px;
  padding: 8px;
  border-top: 1px solid var(--line-soft);
}
.item {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 11px;
  text-align: left;
  text-decoration: none;
  border: 0;
  background: transparent;
  color: var(--text);
  border-radius: var(--r-sm);
  padding: 9px 10px;
  font: inherit;
  font-size: 0.9rem;
  transition: background 0.15s var(--ease);
}
@media (hover: hover) { .item:hover { background: var(--surface-3); } }
.ic {
  flex: none;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  display: grid;
  place-items: center;
  background: var(--surface-3);
  color: var(--text-dim);
  transition: background 0.15s var(--ease), color 0.15s var(--ease);
}
.lbl { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.go { color: var(--text-mute); opacity: 0; transform: translateX(-4px); transition: opacity 0.15s var(--ease), transform 0.15s var(--ease); }
@media (hover: hover) {
  .item:hover .ic { background: var(--accent-soft); color: var(--accent); }
  .item:hover .go { opacity: 1; transform: none; }
}
.item.admin .ic, .item.admin .lbl { color: var(--accent); }
.item.admin .ic { background: var(--accent-soft); }
.item.danger .ic { background: rgba(239, 95, 95, 0.12); color: var(--bad); }
.item.danger .lbl { color: var(--bad); }
@media (hover: hover) { .item.danger:hover .ic { background: rgba(239, 95, 95, 0.22); color: var(--bad); } }
.gsi-wrap {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 6px 4px 4px;
}
.guest-hint {
  font-size: 0.72rem;
  line-height: 1.5;
  color: var(--text-mute);
  text-align: center;
}
.menu-sep { height: 1px; background: var(--line-soft); margin: 6px 8px; }
.err { color: var(--bad); font-size: 0.8rem; padding: 0 4px 2px; }

/* ---------- transition ---------- */
.drop-enter-active { animation: menu-pop 0.2s var(--ease-out-back); }
@keyframes menu-pop {
  from { transform: translateY(-6px) scale(0.97); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}
.drop-leave-active { transition: opacity 0.12s var(--ease), transform 0.12s var(--ease); }
.drop-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }

/* the mobile bottom sheet rises from the viewport's bottom edge */
@keyframes sheet-up {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}
</style>
