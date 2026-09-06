<script setup>
import { useI18n } from '../i18n/index.js'

const { t } = useI18n()

defineProps({
  history: { type: Array, default: () => [] },
})
</script>

<template>
  <div class="panel history">
    <h3 class="section-title">{{ t('steps') }}</h3>
    <div class="body">
      <TransitionGroup name="line" tag="ol">
        <li v-for="(line, i) in history" :key="i"><span class="n">{{ i + 1 }}</span>{{ line }}</li>
      </TransitionGroup>
      <p v-if="history.length === 0" class="empty">{{ t('noSteps') }}</p>
    </div>
  </div>
</template>

<style scoped>
.history {
  min-height: 110px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.body { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; overflow-y: auto; overscroll-behavior: contain; }
ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
li {
  display: flex;
  align-items: baseline;
  gap: 9px;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  color: var(--text-dim);
  padding: 6px 0;
  border-bottom: 1px solid var(--line-soft);
}
li:last-child { border-bottom: none; color: var(--text); }
.n {
  flex: none;
  width: 17px;
  height: 17px;
  display: grid;
  place-items: center;
  border-radius: var(--r-xs);
  background: var(--surface-3);
  color: var(--text-mute);
  font-family: var(--font);
  font-size: 0.62rem;
}
.empty { margin: auto; color: var(--text-mute); font-size: 0.84rem; }
.line-enter-active { animation: rise-in 0.22s var(--ease); }
</style>
