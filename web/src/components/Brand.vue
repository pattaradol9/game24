<script setup>
// The mark: a hand of cards fanned out, the top one a "24". Paths only, so it
// never waits on a webfont. Deals itself in on mount and catches the light
// every few seconds — enough movement to feel alive, not enough to nag.
import { ref } from 'vue'

defineProps({
  size: { type: String, default: 'md' }, // sm | md | lg
  label: { type: String, default: '' },
})

let seq = 0
const uid = `bm${(seq = (seq + 1) % 1e6)}-${Math.random().toString(36).slice(2, 7)}`
const clipId = ref(`${uid}-clip`)
const gradId = ref(`${uid}-grad`)
</script>

<template>
  <span class="brand" :class="size">
    <svg class="mark" viewBox="0 0 52 50" aria-hidden="true">
      <defs>
        <linearGradient :id="gradId" x1="0" y1="0" x2="0.4" y2="1">
          <stop offset="0%" stop-color="var(--accent-hi)" />
          <stop offset="58%" stop-color="var(--accent)" />
          <stop offset="100%" stop-color="#dc9a1f" />
        </linearGradient>
        <clipPath :id="clipId">
          <rect x="15" y="6" width="28" height="38" rx="5" />
        </clipPath>
      </defs>

      <rect class="card back" x="15" y="6" width="28" height="38" rx="5" />
      <rect class="card mid" x="15" y="6" width="28" height="38" rx="5" />

      <g class="front">
        <rect x="15" y="6" width="28" height="38" rx="5" :fill="`url(#${gradId})`" />
        <rect class="frame" x="18.2" y="9.2" width="21.6" height="31.6" rx="2.8" />
        <path class="pip" d="M12 2.4S3.4 9 3.4 13.8c0 2.4 1.8 4.1 4 4.1 1.2 0 2.3-.5 3-1.3 0 1.8-.8 3.4-2 4.6h7.2c-1.2-1.2-2-2.8-2-4.6.7.8 1.8 1.3 3 1.3 2.2 0 4-1.7 4-4.1C20.6 9 12 2.4 12 2.4Z" transform="translate(19.4 10.6) scale(0.2)" />
        <g class="ink">
          <path d="M19.4 22.6a3 3 0 1 1 5.8 1.3c-.6 2.1-5.8 3.7-5.8 7.3h6.4" />
          <path d="M35.5 19.2v12M35.5 19.2l-4.8 8.1h7.6" />
        </g>
        <g :clip-path="`url(#${clipId})`">
          <rect class="shine" x="-16" y="2" width="9" height="46" />
        </g>
      </g>
    </svg>
    <span v-if="label" class="label">{{ label }}</span>
  </span>
</template>

<style scoped>
.brand { display: inline-flex; align-items: center; gap: 12px; line-height: 1; }
.mark { display: block; flex: none; overflow: visible; }

/* everything fans around the bottom edge of the top card */
.card, .front { transform-box: view-box; transform-origin: 29px 44px; }
.card { fill: var(--accent); }
.back { opacity: 0.3; animation: fan-back 0.7s var(--ease-out-back) backwards; }
.mid { opacity: 0.55; animation: fan-mid 0.7s var(--ease-out-back) 0.06s backwards; }
.front { animation: fan-front 0.6s var(--ease-out-back) backwards; }

.frame { fill: none; stroke: var(--accent-ink); stroke-width: 1; opacity: 0.26; }
.pip { fill: var(--accent-ink); opacity: 0.5; }
.ink {
  fill: none;
  stroke: var(--accent-ink);
  stroke-width: 2.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.shine {
  fill: #fff;
  opacity: 0.4;
  transform: skewX(-18deg);
  animation: mark-shine 6s var(--ease) 1.4s infinite;
}
.label { font-weight: 500; letter-spacing: 0.01em; color: var(--text); }

/* the fan opens a little further when the mark is hovered */
.brand:hover .back { transform: rotate(-32deg); }
.brand:hover .mid { transform: rotate(-16deg); }
.card { transition: transform 0.35s var(--ease); }

@keyframes fan-back {
  from { transform: rotate(0deg); opacity: 0; }
  to { transform: rotate(-26deg); opacity: 0.3; }
}
@keyframes fan-mid {
  from { transform: rotate(0deg); opacity: 0; }
  to { transform: rotate(-13deg); opacity: 0.55; }
}
@keyframes fan-front {
  from { transform: translateY(9px) rotate(6deg); opacity: 0; }
  to { transform: translateY(0) rotate(0deg); opacity: 1; }
}
@keyframes mark-shine {
  0%, 6% { transform: translateX(0) skewX(-18deg); }
  16%, 100% { transform: translateX(74px) skewX(-18deg); }
}

.back { transform: rotate(-26deg); }
.mid { transform: rotate(-13deg); }

.sm .mark { width: 34px; height: 33px; }
.sm .label { font-size: 0.92rem; }
.md .mark { width: 46px; height: 44px; }
.md .label { font-size: 1.08rem; }
.lg .mark { width: 74px; height: 71px; }
.lg .label { font-size: 1.6rem; font-weight: 600; }

@media (prefers-reduced-motion: reduce) {
  .shine { animation: none; opacity: 0; }
}
</style>
