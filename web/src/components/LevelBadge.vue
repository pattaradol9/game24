<script setup>
import { computed } from 'vue'
import { levelProgress } from '../core/progress.js'

const props = defineProps({
  exp: { type: Number, default: 0 },
})
const prog = computed(() => levelProgress(props.exp))
const pct = computed(() =>
  prog.value.forNext > 0 ? Math.round((prog.value.into / prog.value.forNext) * 100) : 100
)
</script>

<template>
  <span class="level" :title="`EXP ${exp}`">
    <b class="num">Lv.{{ prog.lv }}</b>
    <span class="bar"><span class="fill" :style="{ width: pct + '%' }" /></span>
  </span>
</template>

<style scoped>
.level { display: inline-flex; align-items: center; gap: 8px; }
.level b { font-size: 0.8rem; font-weight: 500; color: var(--text-dim); white-space: nowrap; }
.bar { width: 46px; height: 4px; border-radius: var(--r-full); background: var(--surface-3); overflow: hidden; }
.fill { display: block; height: 100%; background: var(--accent); border-radius: var(--r-full); }
</style>
