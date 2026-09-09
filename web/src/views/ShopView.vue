<script setup>
// Unified shop: card skins, consumable boost items and the player's
// inventory under one roof, switched by tabs. The page owns the header
// chrome — back, brand, the live coin balance and profile menu — while each
// tab is a self-contained panel that loads its own data. Deep links keep
// working through ?tab= (the old /skins route redirects here).
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { currentPlayer } from '../auth.js'
import { popText } from '../fx.js'
import Brand from '../components/Brand.vue'
import Icon from '../components/Icon.vue'
import ProfileMenu from '../components/ProfileMenu.vue'
import SkinShopPanel from '../components/SkinShopPanel.vue'
import ItemsShopPanel from '../components/ItemsShopPanel.vue'
import InventoryPanel from '../components/InventoryPanel.vue'
import SiteFooter from '../components/SiteFooter.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const TABS = [
  { id: 'skins', icon: 'palette', key: 'shopTabSkins' },
  { id: 'items', icon: 'sparkles', key: 'shopTabBoosts' },
  { id: 'inventory', icon: 'bag', key: 'shopTabInventory' },
]

const tab = ref(TABS.some((x) => x.id === route.query.tab) ? route.query.tab : 'skins')
watch(
  () => route.query.tab,
  (v) => {
    if (TABS.some((x) => x.id === v)) tab.value = v
  }
)
function pick(id) {
  tab.value = id
  router.replace({ query: { ...route.query, tab: id } })
}

const player = computed(() => currentPlayer.value)
const balance = computed(() => player.value?.totalCoins ?? 0)
const balanceEl = ref(null)
const fmt = (n) => (n ?? 0).toLocaleString('en-US')

/* a purchase anywhere in the shop pops its "-N" off the shared balance chip
   — the deduction amount is the whole message */
function onSpent(from, to) {
  if (from === to) return
  const chip = balanceEl.value?.getBoundingClientRect()
  const x = chip ? Math.min(Math.max(chip.left + chip.width / 2, 44), innerWidth - 44) : innerWidth / 2
  const y = chip ? Math.min(Math.max(chip.top + chip.height / 2, 26), innerHeight - 26) : 44
  popText(x, y, `-${fmt(from - to)}`, 'fx-merge gold')
}
</script>

<template>
  <main class="wrap page">
    <header class="top">
      <button class="btn back" @click="$router.push('/')">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <Brand size="sm" />
      <div class="spacer" />
      <span v-if="player" ref="balanceEl" class="chip accent" :title="t('balance')">
        <Icon name="coin" :size="18" />
        <b class="num">{{ fmt(balance) }}</b>
      </span>
      <ProfileMenu />
    </header>

    <nav class="tabs" role="tablist">
      <button
        v-for="x in TABS"
        :key="x.id"
        class="tab"
        :class="{ on: tab === x.id }"
        role="tab"
        :aria-selected="tab === x.id"
        @click="pick(x.id)"
      >
        <Icon :name="x.icon" :size="16" />
        {{ t(x.key) }}
      </button>
    </nav>

    <!-- v-show keeps every tab's state warm: switching tabs never refetches
         or loses a half-scrolled grid -->
    <SkinShopPanel v-show="tab === 'skins'" @spent="onSpent" />
    <ItemsShopPanel v-show="tab === 'items'" @spent="onSpent" />
    <InventoryPanel v-show="tab === 'inventory'" />

    <SiteFooter />
  </main>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 20px; padding: 20px 0 56px; }
/* sticky so the balance chip (and the "-N" deduction pop) stays visible no
   matter how far the grid is scrolled */
.top {
  display: flex; align-items: center; gap: 12px;
  position: sticky; top: 0; z-index: 40;
  margin: -20px 0 0;
  padding: 18px 0 12px;
}
.spacer { flex: 1; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
@media (hover: hover) { .back:hover { color: var(--text); } }

.tabs {
  display: flex;
  gap: 8px;
}
.tab {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 44px;
  padding: 10px 14px;
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface-2);
  color: var(--text-dim);
  font-size: 0.9rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  transition: color 0.15s var(--ease), border-color 0.15s var(--ease), background 0.15s var(--ease);
}
@media (hover: hover) {
  .tab:hover { color: var(--text); }
}
.tab.on {
  color: var(--accent);
  border-color: rgba(246, 183, 60, 0.5);
  background: linear-gradient(rgba(246, 183, 60, 0.1), rgba(246, 183, 60, 0.1)), var(--surface-2);
}

@media (max-width: 560px) {
  .back-label { display: none; }
  .back { padding: 0; width: 44px; justify-content: center; }
  .tab { font-size: 0.8rem; padding: 10px 8px; gap: 6px; }
}
</style>
