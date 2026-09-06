<script setup>
import { useI18n } from '../i18n/index.js'
import { MODES } from '../modes.js'
import Suit from './Suit.vue'

const { t } = useI18n()

defineProps({
  modelValue: { type: String, required: true },
  disabled: { type: Object, default: () => ({}) },
})

defineEmits(['update:modelValue'])
</script>

<template>
  <div class="modes">
    <button
      v-for="m in MODES"
      :key="m.id"
      class="mode"
      :class="{ active: modelValue === m.id, locked: disabled[m.id] }"
      :disabled="disabled[m.id]"
      @click="$emit('update:modelValue', m.id)"
    >
      <span class="medal"><Suit :name="m.suit" :size="28" /></span>
      <span class="name">{{ t(m.id) }}</span>
      <span class="stars" :aria-label="`${m.stars}/5`">
        <i v-for="n in 5" :key="n" :class="{ on: n <= m.stars }" />
      </span>
      <span class="desc">{{ t(`${m.id}Desc`) }}</span>
      <span class="mult num">×{{ m.mult }} EXP</span>
    </button>
  </div>
</template>

<style scoped>
.modes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 12px;
}
.mode {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 22px 16px 18px;
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface);
  text-align: center;
  font: inherit;
  transition: border-color 0.15s var(--ease), background 0.15s var(--ease), transform 0.12s var(--ease);
}
@media (hover: hover) {
  .mode:hover:not(:disabled) { border-color: #3a4560; background: var(--surface-2); transform: translateY(-2px); }
  .mode:hover:not(:disabled) .medal { background: var(--surface-3); }
}
.mode:active:not(:disabled) { transform: translateY(0); }
.mode.active { border-color: var(--accent); background: var(--accent-soft); }
.mode.locked { opacity: 0.4; cursor: not-allowed; }

.medal {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: var(--r-md);
  background: var(--surface-2);
  border: 1px solid var(--line-soft);
  transition: background 0.15s var(--ease);
}
.mode.active .medal { background: var(--surface); border-color: rgba(246, 183, 60, 0.3); }
.name { font-size: 1.05rem; font-weight: 500; }
.stars { display: flex; gap: 4px; }
.stars i { width: 14px; height: 3px; border-radius: 2px; background: var(--line); }
.stars i.on { background: var(--accent); }
.desc { font-size: 0.82rem; color: var(--text-mute); line-height: 1.45; min-height: 2.4em; }
.mult {
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: var(--r-xs);
  padding: 5px 9px;
  line-height: 1;
}
</style>
