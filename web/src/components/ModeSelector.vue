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
      :class="[m.suit, { active: modelValue === m.id, locked: disabled[m.id] }]"
      :disabled="disabled[m.id]"
      @click="$emit('update:modelValue', m.id)"
    >
      <span class="medal"><Suit :name="m.suit" :size="32" /></span>
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
  .mode:hover:not(:disabled) .medal {
    border-color: rgba(var(--suit-rgb), 0.4);
    background:
      radial-gradient(115% 115% at 50% 0%, rgba(var(--suit-rgb), 0.26), rgba(var(--suit-rgb), 0.07) 58%, transparent 78%),
      var(--surface-3);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.06),
      0 0 26px rgba(var(--suit-rgb), 0.16),
      0 10px 22px rgba(0, 0, 0, 0.28);
  }
}
.mode:active:not(:disabled) { transform: translateY(0); }
.mode.active { border-color: var(--accent); background: var(--accent-soft); }
.mode.locked { opacity: 0.4; cursor: not-allowed; }

/* suit-tinted medallion: the color melts into the card instead of a hard tile */
.mode.diamond, .mode.heart { --suit-ink: #e0566c; --suit-rgb: 224, 86, 108; }
.mode.club, .mode.spade { --suit-ink: #c7d2e8; --suit-rgb: 150, 168, 206; }

.medal {
  display: grid;
  place-items: center;
  width: 62px;
  height: 62px;
  border-radius: var(--r-full);
  color: var(--suit-ink);
  background:
    radial-gradient(115% 115% at 50% 0%, rgba(var(--suit-rgb), 0.18), rgba(var(--suit-rgb), 0.05) 58%, transparent 78%),
    var(--surface-2);
  border: 1px solid rgba(var(--suit-rgb), 0.22);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 0 18px rgba(var(--suit-rgb), 0.08),
    0 8px 18px rgba(0, 0, 0, 0.22);
  transition: background 0.2s var(--ease), border-color 0.2s var(--ease), box-shadow 0.2s var(--ease);
}
.medal :deep(.suit) {
  color: var(--suit-ink);
  filter: drop-shadow(0 1px 7px rgba(var(--suit-rgb), 0.5));
}
.mode.active .medal {
  border-color: rgba(var(--suit-rgb), 0.45);
  background:
    radial-gradient(115% 115% at 50% 0%, rgba(var(--suit-rgb), 0.3), rgba(var(--suit-rgb), 0.09) 60%, transparent 80%),
    var(--surface);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.07),
    0 0 30px rgba(var(--suit-rgb), 0.2),
    0 8px 20px rgba(0, 0, 0, 0.25);
}
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
