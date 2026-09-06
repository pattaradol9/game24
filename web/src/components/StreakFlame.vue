<script setup>
import Icon from './Icon.vue'

defineProps({
  streak: { type: Number, default: 0 },
})
</script>

<template>
  <span v-if="streak > 0" class="streak" :class="{ hot: streak >= 4 }">
    <Icon name="flame" :size="15" />
    <b class="num">{{ streak }}</b>
  </span>
</template>

<style scoped>
.streak {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: var(--r-xs);
  border: 1px solid var(--line);
  background: var(--surface-2);
  padding: 6px 10px;
  color: var(--text-dim);
  line-height: 1;
}
.streak b { font-size: 0.82rem; font-weight: 600; }
.streak.hot { color: var(--accent); border-color: rgba(246, 183, 60, 0.35); background: var(--accent-soft); }
</style>
