<script setup>
// Header profile menu: shows the current session with rename / google
// sign-in / logout actions.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { currentPlayer, clearSession } from '../auth.js'
import { levelProgress, tierName } from '../core/progress.js'
import { sfx } from '../audio.js'
import GoogleSignIn from './GoogleSignIn.vue'
import AnimatedIcon from './AnimatedIcon.vue'
import PlayerChip from './PlayerChip.vue'
import RenameModal from './RenameModal.vue'
import TierBadge from './TierBadge.vue'
import TierAvatar from './TierAvatar.vue'

const { t } = useI18n()
const emit = defineEmits(['require-name'])

const open = ref(false)
const showRename = ref(false)
const wrap = ref(null)
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
const fmt = (n) => n.toLocaleString('en-US')

function toggle() {
  sfx.click()
  open.value = !open.value
}

function close() {
  open.value = false
}

function onDocClick(e) {
  if (open.value && wrap.value && !wrap.value.contains(e.target)) close()
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
})

function logout() {
  sfx.click()
  close()
  clearSession()
  // re-show the entry modal through the parent
  emit('require-name')
}
</script>

<template>
  <div ref="wrap" class="profile">
    <button v-if="!player" class="btn" @click="$emit('require-name')">
      <AnimatedIcon name="user" :size="17" />{{ t('profileStart') }}
    </button>

    <button v-else class="profile-btn" :class="{ open }" @click="toggle">
      <PlayerChip />
      <Icon class="caret" name="chevron-down" :size="14" />
    </button>

    <Transition name="drop">
      <div v-if="open && player" class="menu">
        <div class="hero">
          <div class="avatar-wrap">
            <TierAvatar
              :tier="tier"
              :src="player.picture"
              :name="player.nickname"
              :size="62"
            />
            <span class="lv-chip">Lv.{{ player.level }}</span>
          </div>
          <b class="nick">{{ player.nickname }}</b>
          <div class="meta-row">
            <TierBadge :tier="tier" size="sm" />
            <span class="sub">{{ player.isGuest ? t('profileGuest') : t('profileGoogle') }}</span>
          </div>
        </div>

        <div class="stats">
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
        </div>

        <div class="actions">
          <button class="item" @click="showRename = true; close()">
            <span class="ic"><AnimatedIcon name="edit" :size="15" /></span>
            <span class="lbl">{{ t('profileRename') }}</span>
            <AnimatedIcon class="go" name="chevron-right" :size="14" />
          </button>
          <button v-if="player.isAdmin" class="item admin" @click="close(); $router.push('/admin')">
            <span class="ic"><AnimatedIcon name="crown" :size="15" /></span>
            <span class="lbl">{{ t('profileAdmin') }}</span>
            <AnimatedIcon class="go" name="chevron-right" :size="14" />
          </button>
          <div v-if="player.isGuest" class="gsi-wrap">
            <GoogleSignIn fit-width @signed-in="close()" />
          </div>
          <button v-else class="item danger" @click="logout">
            <span class="ic"><AnimatedIcon name="logout" :size="15" /></span>
            <span class="lbl">{{ t('profileLogout') }}</span>
            <AnimatedIcon class="go" name="chevron-right" :size="14" />
          </button>
          <p v-if="error" class="err">{{ error }}</p>
        </div>
      </div>
    </Transition>

    <RenameModal v-if="showRename" @close="showRename = false" />
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
  width: 300px;
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
  display: grid;
  place-items: center;
  /* 40px button + 8px vertical padding = 48px, matching .item rows */
  padding: 4px 0;
}
.err { color: var(--bad); font-size: 0.8rem; padding: 0 4px 2px; }

/* ---------- transition ---------- */
.drop-enter-active { animation: menu-pop 0.2s var(--ease-out-back); }
@keyframes menu-pop {
  from { transform: translateY(-6px) scale(0.97); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}
.drop-leave-active { transition: opacity 0.12s var(--ease), transform 0.12s var(--ease); }
.drop-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }
</style>
