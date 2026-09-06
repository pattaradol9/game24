<script setup>
import { useI18n } from '../i18n/index.js'

const { t } = useI18n()

defineProps({
  show: { type: Boolean, default: false },
  kind: { type: String, default: 'level' }, // level | tier
  value: { type: String, default: '' },
})
</script>

<template>
  <Transition name="cele">
    <div v-if="show" class="cele" role="status">
      <div class="banner" :class="kind">
        <span class="section-title">{{ kind === 'level' ? t('levelUp') : t('tierUp') }}</span>
        <span class="value">{{ value }}</span>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.cele {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: start center;
  padding-top: 14vh;
  pointer-events: none;
  z-index: 200;
}
.banner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 18px 34px;
  border-radius: var(--r-md);
  background: var(--surface-2);
  border: 1px solid var(--line);
  box-shadow: var(--sh-3);
  animation: rise-in 0.28s var(--ease);
}
.banner.tier { border-color: rgba(246, 183, 60, 0.5); }
.value { font-size: 1.8rem; font-weight: 600; letter-spacing: -0.02em; }
.banner.level .value { color: var(--good); }
.banner.tier .value { color: var(--accent); }
.cele-leave-active { transition: opacity 0.3s var(--ease), transform 0.3s var(--ease); }
.cele-leave-to { opacity: 0; transform: translateY(-14px); }
</style>
