<script setup>
// Header profile menu: shows the current session with rename / google
// sign-in / logout actions.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { currentPlayer, clearSession } from '../auth.js'
import { levelProgress } from '../core/progress.js'
import { sfx } from '../audio.js'
import GoogleSignIn from './GoogleSignIn.vue'
import Icon from './Icon.vue'
import PlayerChip from './PlayerChip.vue'
import RenameModal from './RenameModal.vue'

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

function toggle() {
  sfx.click()
  open.value = !open.value
}

function onDocClick(e) {
  if (open.value && wrap.value && !wrap.value.contains(e.target)) open.value = false
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))

function logout() {
  sfx.click()
  open.value = false
  clearSession()
  // re-show the entry modal through the parent
  emit('require-name')
}
</script>

<template>
  <div ref="wrap" class="profile">
    <button v-if="!player" class="btn" @click="$emit('require-name')">
      <Icon name="user" :size="17" />{{ t('profileStart') }}
    </button>

    <button v-else class="profile-btn" @click="toggle">
      <PlayerChip />
      <span class="caret" aria-hidden="true">▾</span>
    </button>

    <Transition name="drop">
      <div v-if="open && player" class="menu">
        <div class="head">
          <img v-if="player.picture" :src="player.picture" referrerpolicy="no-referrer" alt="" />
          <span v-else class="avatar big guest">{{ player.nickname.slice(0, 1).toUpperCase() }}</span>
          <div class="who">
            <b>{{ player.nickname }}</b>
            <span class="sub">
              {{ player.isGuest ? t('profileGuest') : t('profileGoogle') }}
              · Lv.{{ player.level }}
            </span>
          </div>
        </div>

        <div class="stats">
          <div class="statrow">
            <span>EXP {{ player.totalExp }}</span>
            <span v-if="prog">→ Lv.{{ prog.lv + 1 }}: {{ prog.into }}/{{ prog.forNext }}</span>
          </div>
          <div class="bar"><span class="fill" :style="{ width: (prog && prog.forNext ? (prog.into / prog.forNext) * 100 : 100) + '%' }" /></div>
        </div>

        <button class="item" @click="showRename = true; open = false">
          <Icon name="edit" :size="16" />{{ t('profileRename') }}
        </button>
        <div v-if="player.isGuest" class="gsi-wrap">
          <GoogleSignIn @signed-in="open = false" />
        </div>
        <button v-else class="item danger" @click="logout">
          <Icon name="logout" :size="16" />{{ t('profileLogout') }}
        </button>
        <p v-if="error" class="err">{{ error }}</p>
      </div>
    </Transition>

    <RenameModal v-if="showRename" @close="showRename = false" />
  </div>
</template>

<style scoped>
.profile { position: relative; }
.profile-btn {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--line);
  background: var(--surface-2);
  color: var(--text);
  border-radius: var(--r-md);
  padding: 6px 12px 6px 6px;
  min-height: 48px;
  font: inherit;
  font-size: 0.9rem;
  font-weight: 500;
  transition: border-color 0.15s var(--ease), background 0.15s var(--ease);
}
@media (hover: hover) { .profile-btn:hover { border-color: #3a4560; background: var(--surface-3); } }
.name { max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.profile-btn img, .avatar { width: 30px; height: 30px; border-radius: var(--r-xs); object-fit: cover; }
.avatar {
  display: grid;
  place-items: center;
  background: var(--accent);
  color: var(--accent-ink);
  font-weight: 700;
  font-size: 0.95rem;
}
.avatar.big { width: 44px; height: 44px; border-radius: var(--r-sm); font-size: 1.25rem; }
.caret { color: var(--text-mute); font-size: 0.7rem; }

.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  width: 280px;
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface-2);
  box-shadow: var(--sh-3);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  z-index: 80;
}
.head { display: flex; align-items: center; gap: 12px; }
.who { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.who b { font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sub { font-size: 0.75rem; color: var(--text-mute); }
.stats { background: var(--bg); border: 1px solid var(--line-soft); border-radius: var(--r-sm); padding: 11px 12px; }
.statrow { display: flex; justify-content: space-between; gap: 8px; font-size: 0.76rem; color: var(--text-mute); margin-bottom: 8px; }
.bar { height: 5px; border-radius: var(--r-full); background: var(--surface-3); overflow: hidden; }
.fill { display: block; height: 100%; background: var(--accent); border-radius: var(--r-full); }
.item {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 9px;
  text-align: left;
  border: 1px solid var(--line);
  background: transparent;
  color: var(--text);
  border-radius: var(--r-sm);
  padding: 10px 12px;
  font: inherit;
  font-size: 0.88rem;
  transition: background 0.15s var(--ease);
}
@media (hover: hover) { .item:hover { background: var(--surface-3); } }
.item.danger { color: var(--bad); border-color: rgba(239, 95, 95, 0.3); }
@media (hover: hover) { .item.danger:hover { background: rgba(239, 95, 95, 0.12); } }
.gsi-wrap { display: grid; place-items: center; }
.err { color: var(--bad); font-size: 0.8rem; }
.drop-enter-active { animation: rise-in 0.18s var(--ease); }
.drop-leave-active { transition: opacity 0.12s var(--ease); }
.drop-leave-to { opacity: 0; }
</style>
