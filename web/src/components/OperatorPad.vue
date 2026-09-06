<script setup>
import { sfx } from '../audio.js'
import { haptic } from '../fx.js'

const props = defineProps({
  modelValue: { type: String, default: null },
  disabled: { type: Boolean, default: false }, // true until a card is picked
})
const emit = defineEmits(['update:modelValue'])

const ops = [
  { sym: '+', real: '+', label: 'plus' },
  { sym: '−', real: '-', label: 'minus' },
  { sym: '×', real: '*', label: 'times' },
  { sym: '÷', real: '/', label: 'divide' },
]

function pick(op) {
  if (props.disabled) return
  sfx.click()
  haptic(8)
  emit('update:modelValue', props.modelValue === op.real ? null : op.real)
}
</script>

<template>
  <!-- the pad lifts out of standby the moment a card is selected, so the
       "pick a number, then an operator" order is visible rather than written -->
  <div class="pad" :class="{ ready: !disabled }" role="group">
    <button
      v-for="op in ops"
      :key="op.real"
      class="op"
      :class="{ on: modelValue === op.real }"
      :disabled="disabled"
      :aria-label="op.label"
      :aria-pressed="modelValue === op.real"
      @click="pick(op)"
    >
      {{ op.sym }}
    </button>
  </div>
</template>

<style scoped>
.pad {
  display: flex;
  gap: 8px;
  padding: 8px;
  border-radius: var(--r-lg);
  border: 1px solid var(--line-soft);
  background: var(--bg);
  transition: border-color 0.2s var(--ease), background 0.2s var(--ease);
}
.pad.ready { border-color: var(--line); background: var(--surface-2); }

.op {
  cursor: pointer;
  width: clamp(52px, 13vw, 66px);
  height: clamp(52px, 13vw, 66px);
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface-2);
  color: var(--text-mute);
  font: inherit;
  font-size: 2.5rem;
  font-weight: 400;
  line-height: 1;
  transition: background 0.14s var(--ease), border-color 0.14s var(--ease),
    color 0.14s var(--ease), transform 0.1s var(--ease);
}
.pad.ready .op { color: var(--text); background: var(--surface-3); border-color: #3a4560; }
@media (hover: hover) {
  .pad.ready .op:hover { background: var(--accent-soft); border-color: rgba(246, 183, 60, 0.55); color: var(--accent-hi); }
}
.op:active:not(:disabled) { transform: translateY(1px); }
.op:disabled { cursor: not-allowed; }

.op.on, .pad.ready .op.on {
  background: var(--accent);
  border-color: var(--accent);
  color: var(--accent-ink);
  font-weight: 500;
}
</style>
