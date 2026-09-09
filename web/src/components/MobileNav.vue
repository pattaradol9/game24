<script setup>
// Mobile bottom navigation: the main destinations gather into a fixed
// thumb-reach bar, with the player's avatar front and centre — raised above
// the bar, its popup opening upward. Hidden on desktop (the pages keep
// their own headers) and on gameplay screens where a stray tap could quit
// the round. Rendered once from App.vue.
import { computed, onUnmounted, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { currentPlayer, getPlayer, requestPlayerName } from '../auth.js'
import ProfileMenu from './ProfileMenu.vue'
import Icon from './Icon.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const player = currentPlayer

const LINKS = [
  { to: '/', icon: 'home', key: 'navHome' },
  { to: '/leaderboard', icon: 'crown', key: 'leaderboard' },
  { to: '/achievements', icon: 'trophy', key: 'achievements' },
  { to: '/shop', icon: 'bag', key: 'shop' },
]

const isActive = (to) =>
  to === '/' ? route.path === '/' : route.path === to || route.path.startsWith(to + '/')

// one level of prefix matching is enough — the game routes carry params
const visible = computed(() => {
  const p = route.path
  return !p.startsWith('/admin') && !p.startsWith('/solo') && !p.startsWith('/room')
})

// the centre avatar is not a route: signed-in players get their menu (opening
// upward), anyone else gets the nickname entry modal on the home page
function onProfile() {
  if (!getPlayer()) {
    requestPlayerName()
    router.push('/')
  }
}

// flag the body so pages add bottom clearance only while the bar shows
// (gameplay screens skip it — the nav is hidden there)
const mobileMQ = window.matchMedia('(max-width: 640px)')
function syncBodyFlag() {
  document.body.classList.toggle('mn-pad', visible.value && mobileMQ.matches)
}
watchEffect(syncBodyFlag)
mobileMQ.addEventListener('change', syncBodyFlag)
onUnmounted(() => {
  mobileMQ.removeEventListener('change', syncBodyFlag)
  document.body.classList.remove('mn-pad')
})
</script>

<template>
  <nav v-if="visible" class="mobile-nav" aria-label="Primary">
    <RouterLink
      v-for="item in LINKS.slice(0, 2)"
      :key="item.to"
      :to="item.to"
      class="mn-item"
      :class="{ on: isActive(item.to) }"
    >
      <Icon :name="item.icon" :size="21" />
      <span class="mn-label">{{ t(item.key) }}</span>
    </RouterLink>

    <!-- centre stage: the player's avatar, raised and glowing -->
    <ProfileMenu class="mn-profile" direction="up" @require-name="onProfile()">
      <template #trigger="{ toggle }">
        <button class="mn-center" :aria-label="t('navProfile')" @click="getPlayer() ? toggle() : onProfile()">
          <img v-if="player?.picture" :src="player.picture" alt="" referrerpolicy="no-referrer" />
          <span v-else-if="player" class="initial">{{ player.nickname.slice(0, 1).toUpperCase() }}</span>
          <Icon v-else name="user" :size="24" />
        </button>
      </template>
    </ProfileMenu>

    <RouterLink
      v-for="item in LINKS.slice(2)"
      :key="item.to"
      :to="item.to"
      class="mn-item"
      :class="{ on: isActive(item.to) }"
    >
      <Icon :name="item.icon" :size="21" />
      <span class="mn-label">{{ t(item.key) }}</span>
    </RouterLink>
  </nav>
</template>

<style>
/* keep the page content clear of the fixed bar (unscoped on purpose; the
   mn-pad class is toggled by the component only while the bar shows) */
body.mn-pad main.wrap { padding-bottom: 88px; }

/* the page headers' profile menu moves into this bottom bar on phones */
@media (max-width: 640px) {
  header .profile:not(.mn-profile) { display: none; }
}
</style>

<style scoped>
.mobile-nav {
  display: none;
}

@media (max-width: 640px) {
  .mobile-nav {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 140;
    display: flex;
    align-items: flex-end;
    justify-content: space-around;
    padding: 6px 8px calc(6px + env(safe-area-inset-bottom));
    background: color-mix(in srgb, var(--surface) 88%, transparent);
    backdrop-filter: blur(14px);
    -webkit-backdrop-filter: blur(14px);
    border-top: 1px solid var(--line);
  }
}

.mn-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 6px 4px 5px;
  border: 0;
  background: transparent;
  font: inherit;
  border-radius: var(--r-sm);
  color: var(--text-mute);
  text-decoration: none;
  transition: color 0.15s var(--ease), background 0.15s var(--ease);
}
.mn-item:active { background: var(--surface-2); }
.mn-item.on { color: var(--accent); }
.mn-label { font-size: 0.62rem; letter-spacing: 0.02em; line-height: 1; }

/* the raised centre avatar — deliberately louder than everything else */
.mn-center {
  width: 52px;
  height: 52px;
  margin-bottom: 10px;
  border-radius: 50%;
  border: 2px solid rgba(246, 183, 60, 0.6);
  background: var(--surface-2);
  color: var(--text-dim);
  display: grid;
  place-items: center;
  overflow: hidden;
  cursor: pointer;
  transform: translateY(-14px);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.5), 0 0 16px rgba(246, 183, 60, 0.28);
  transition: box-shadow 0.2s var(--ease), border-color 0.2s var(--ease);
}
.mn-center:active {
  border-color: var(--accent);
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.5), 0 0 22px rgba(246, 183, 60, 0.45);
}
.mn-center img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.mn-center .initial {
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--accent);
}

/* the profile menu anchors to the bar itself (display: contents removes the
   wrapper box), popping above the nav's right-hand edge */
.mn-profile { display: contents; }
</style>
