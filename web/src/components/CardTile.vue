<script setup>
import { computed } from 'vue'
import Suit from './Suit.vue'

const props = defineProps({
  card: { type: Object, required: true },
  selected: { type: Boolean, default: false },
  selectable: { type: Boolean, default: true },
  dealDelay: { type: Number, default: null }, // ms stagger for the deal-in
})

// the suit is decorative but must be stable for a given card
const SUITS = ['spade', 'heart', 'diamond', 'club']
const suit = computed(() => {
  let h = 0
  for (const ch of props.card.id) h = (h * 31 + ch.charCodeAt(0)) % 997
  return SUITS[h % 4]
})
const red = computed(() => suit.value === 'heart' || suit.value === 'diamond')
const chars = computed(() => props.card.display.length)

const emit = defineEmits(['pick'])

function pick() {
  if (!props.selectable) return
  emit('pick', props.card)
}
</script>

<template>
  <button
    class="card"
    :class="{ selected, disabled: !selectable, red, dealt: dealDelay !== null }"
    :style="{ '--chars': chars, animationDelay: dealDelay !== null ? dealDelay + 'ms' : undefined }"
    :data-card-id="card.id"
    :aria-pressed="selected"
    @click="pick"
  >
    <span class="frame" aria-hidden="true" />
    <!-- the two indices, mirrored like a real card -->
    <span class="idx tl"><b class="num">{{ card.display }}</b><Suit :name="suit" class="pip" /></span>
    <span class="idx br"><b class="num">{{ card.display }}</b><Suit :name="suit" class="pip" /></span>
    <Suit :name="suit" class="watermark" aria-hidden="true" />
    <span class="rank num">{{ card.display }}</span>
  </button>
</template>

<style scoped>
.card {
  --pad: calc(var(--card-w) * 0.065);
  position: relative;
  width: var(--card-w);
  height: calc(var(--card-w) * 1.45);
  border: 1px solid #cdd3e3;
  border-radius: calc(var(--card-w) * 0.1);
  /* pressed card stock: warm-white face with a faint sheen off the top edge */
  background:
    linear-gradient(163deg, #ffffff 0%, #fafbfe 46%, #eef0f7 100%);
  box-shadow:
    0 1px 1px rgba(0, 0, 0, 0.3),
    0 10px 24px rgba(0, 0, 0, 0.32),
    inset 0 1px 0 #fff;
  cursor: pointer;
  color: var(--card-black);
  padding: 0;
  display: grid;
  place-items: center;
  overflow: hidden;
  transition: transform 0.14s var(--ease), box-shadow 0.14s var(--ease),
    outline-color 0.14s var(--ease);
  outline: 2px solid transparent;
  outline-offset: 2px;
  -webkit-tap-highlight-color: transparent;
}
.card.red { color: var(--card-red); }

/* the printed border every real deck has inside the trim */
.frame {
  position: absolute;
  inset: calc(var(--card-w) * 0.055);
  border: 1px solid currentColor;
  border-radius: calc(var(--card-w) * 0.055);
  opacity: 0.13;
  pointer-events: none;
}

@media (hover: hover) {
  .card:hover:not(.disabled) { transform: translateY(-5px); }
}
.card:active:not(.disabled) { transform: translateY(-2px); }
.card.selected {
  transform: translateY(calc(var(--card-w) * -0.11));
  outline-color: var(--accent);
  box-shadow:
    0 1px 1px rgba(0, 0, 0, 0.3),
    0 16px 32px rgba(0, 0, 0, 0.42),
    inset 0 1px 0 #fff;
}
.card.disabled { opacity: 0.4; cursor: default; }

/* ---------- indices ---------- */
.idx {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: calc(var(--card-w) * 0.012);
  line-height: 1;
}
.idx b {
  font-size: calc(var(--card-w) * min(0.2, 0.62 / var(--chars, 1)));
  font-weight: 600;
  letter-spacing: -0.02em;
}
.idx .pip { width: calc(var(--card-w) * 0.13); height: calc(var(--card-w) * 0.13); }
.tl { top: var(--pad); left: var(--pad); }
.br { bottom: var(--pad); right: var(--pad); transform: rotate(180deg); }

/* ---------- centre ---------- */
.watermark {
  position: absolute;
  width: calc(var(--card-w) * 0.62);
  height: calc(var(--card-w) * 0.62);
  opacity: 0.06;
  pointer-events: none;
}
.rank {
  position: relative;
  font-size: calc(var(--card-w) * min(0.46, 1.5 / var(--chars, 1)));
  font-weight: 600;
  line-height: 1;
  letter-spacing: -0.035em;
}

.card.dealt { animation: deal-in 0.42s var(--ease-out-back) backwards; }
@keyframes deal-in {
  from { transform: translateY(-38vh) rotate(-10deg); opacity: 0; }
  to { transform: translateY(0) rotate(0); opacity: 1; }
}
</style>
