<script setup>
import { useI18n } from '../i18n/index.js'
import Brand from '../components/Brand.vue'
import Icon from '../components/Icon.vue'
import LeaderboardView from '../components/LeaderboardView.vue'
import ProfileMenu from '../components/ProfileMenu.vue'
import SiteFooter from '../components/SiteFooter.vue'

const { t } = useI18n()
</script>

<template>
  <main class="wrap page">
    <header class="top">
      <button class="btn back" @click="$router.push('/')">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <Brand size="sm" />
      <div class="spacer" />
      <ProfileMenu />
    </header>

    <section class="panel body">
      <h1>{{ t('leaderboard') }}</h1>
      <LeaderboardView />
    </section>

    <SiteFooter />
  </main>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 20px; padding: 20px 0 56px; }
.top { display: flex; align-items: center; gap: 12px; }
.spacer { flex: 1; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
@media (hover: hover) { .back:hover { color: var(--text); } }
.body { display: flex; flex-direction: column; gap: 18px; padding: 26px 28px 30px; }
h1 { font-size: 1.4rem; font-weight: 600; letter-spacing: -0.01em; }
@media (max-width: 560px) {
  .back-label { display: none; }
  .back { padding: 0; width: 44px; justify-content: center; }
  .body { padding: 20px 16px 24px; }
}
</style>
