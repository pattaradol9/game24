<script setup>
import { computed } from 'vue'

const props = defineProps({
  tier: { type: String, default: 'bronze' },
  size: { type: String, default: 'md' }, // sm | md
})

const TIERS = {
  // guests have no rank yet — the badge reads flat grey to match their
  // plain frame everywhere else
  guest: '#8a94a8',
  bronze: '#c07a3e',
  silver: '#a9b1c1',
  gold: '#f6b73c',
  platinum: '#6fd6c8',
  diamond: '#78a8f5',
  master: '#b283f0',
}
const color = computed(() => TIERS[props.tier] ?? TIERS.guest)
</script>

<template>
  <span class="tier" :class="size" :style="{ '--tier': color }">
    <i />{{ tier }}
  </span>
</template>

<style scoped>
.tier {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border-radius: var(--r-xs);
  border: 1px solid var(--line);
  background: var(--surface-2);
  padding: 5px 10px;
  font-size: 0.72rem;
  font-weight: 500;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--tier);
  line-height: 1;
  white-space: nowrap;
}
.tier i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--tier);
  flex: none;
}
.tier.sm { font-size: 0.63rem; padding: 4px 8px; gap: 5px; }
.tier.sm i { width: 6px; height: 6px; }
</style>
