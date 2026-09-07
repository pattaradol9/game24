<script setup>
// Small legal footer reused on content pages (home, leaderboard, ...).
import { ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import Icon from './Icon.vue'
import DonateModal from './DonateModal.vue'

const { t } = useI18n()
const showDonate = ref(false)
</script>

<template>
  <footer class="site-footer">
    <p class="tag">{{ t('footerTag') }}</p>
    <button class="donate" @click="showDonate = true">
      <Icon name="heart" :size="15" />
      <span>{{ t('donate') }}</span>
    </button>
    <nav class="links">
      <RouterLink to="/terms">{{ t('terms') }}</RouterLink>
      <span class="dot" aria-hidden="true">·</span>
      <RouterLink to="/privacy">{{ t('privacy') }}</RouterLink>
    </nav>
    <DonateModal v-if="showDonate" @close="showDonate = false" />
  </footer>
</template>

<style scoped>
.site-footer {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 9px;
  padding: 6px 0 10px;
  text-align: center;
}
.tag { font-size: 0.78rem; color: var(--text-mute); }
.donate {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 16px;
  border-radius: var(--r-full);
  border: 1px solid var(--line);
  background: var(--surface-2);
  color: var(--text-dim);
  font-size: 0.8rem;
  font-weight: 500;
  cursor: pointer;
}
.donate :deep(.icon) { color: var(--accent); }
@media (hover: hover) {
  .donate:hover { color: var(--text); border-color: var(--accent); background: var(--accent-soft); }
}
.donate:active { transform: translateY(1px); }
.links {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 0.8rem;
}
.links a { color: var(--text-dim); text-decoration: none; }
@media (hover: hover) { .links a:hover { color: var(--accent); text-decoration: underline; } }
.dot { color: var(--text-mute); }
</style>
