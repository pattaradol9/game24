<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  exp: { type: Number, default: 0 },
  into: { type: Number, default: 0 },
  forNext: { type: Number, default: 1 },
  level: { type: Number, default: 1 },
  compact: { type: Boolean, default: false },
  instant: { type: Boolean, default: false }, // caller animates the value itself
})

const pct = computed(() =>
  props.forNext > 0 ? Math.min(100, Math.round((props.into / props.forNext) * 100)) : 100
)

const leveled = ref(false)
watch(() => props.level, (nv, ov) => {
  if (nv > (ov ?? 0)) {
    leveled.value = true
    setTimeout(() => (leveled.value = false), 1200)
  }
})
</script>

<template>
  <div class="xp" :class="{ compact, instant }">
    <div class="row">
      <span class="lv">Lv.{{ level }}</span>
      <span v-if="!compact" class="count num">{{ forNext > 0 ? `${into} / ${forNext} EXP` : 'MAX' }}</span>
    </div>
    <span class="track">
      <span class="fill" :class="{ blink: leveled }" :style="{ width: pct + '%' }" />
    </span>
  </div>
</template>

<style scoped>
.xp { display: flex; flex-direction: column; gap: 8px; width: 100%; min-width: 0; }
.row { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }
.lv { font-size: 0.85rem; font-weight: 500; }
.count { font-size: 0.72rem; color: var(--text-mute); }
.track { height: 5px; border-radius: var(--r-full); background: var(--surface-3); overflow: hidden; }
.fill {
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: var(--accent);
  transition: width 0.7s var(--ease);
}
.fill.blink { animation: xp-blink 0.3s ease-in-out 3; }
.xp.compact .lv { font-size: 0.78rem; }
.xp.instant .fill { transition: none; }
@keyframes xp-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.45; }
}
</style>
