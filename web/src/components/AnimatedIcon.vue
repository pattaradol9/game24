<script setup>
// Animated siblings of Icon.vue, for modals and dialogs: a small looping
// motion that *explains* the action — the pencil writes, the dice rolls,
// the heart beats, the X draws itself. Same 24px grid and stroke style,
// so the two components stay interchangeable.
defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 20 },
})

const PATHS = {
  'sound-on': 'M4 9v6h4l5 4V5L8 9H4zM16.5 8.5a5 5 0 0 1 0 7M19 6a8.5 8.5 0 0 1 0 12',
  'sound-off': 'M4 9v6h4l5 4V5L8 9H4zM17 9.5l5 5M22 9.5l-5 5',
  back: 'M15 5l-7 7 7 7',
  'chevron-down': 'M6 9l6 6 6-6',
  'chevron-right': 'M9 6l6 6-6 6',
  'arrow-right': 'M5 12h14M12 5l7 7-7 7',
  bulb: 'M9 18h6M10 21h4M12 3a6 6 0 0 0-3.5 10.9c.5.4.8 1 .8 1.6v.5h5.4v-.5c0-.6.3-1.2.8-1.6A6 6 0 0 0 12 3z',
  undo: 'M4 9h11a5 5 0 0 1 0 10h-6M4 9l4-4M4 9l4 4',
  skip: 'M5 5l8 7-8 7V5zM17 5v14',
  clock: 'M12 7v5l3.5 2M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z',
  link: 'M10 13.5a4 4 0 0 0 5.7 0l2.8-2.8a4 4 0 0 0-5.7-5.7l-1.4 1.4M14 10.5a4 4 0 0 0-5.7 0l-2.8 2.8a4 4 0 0 0 5.7 5.7l1.4-1.4',
  play: 'M7 4.5l12 7.5-12 7.5v-15z',
  check: 'M4.5 12.5l5 5 10-11',
  close: 'M6 6l12 12M18 6L6 18',
  user: 'M4.5 20a7.5 7.5 0 0 1 15 0M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z',
  crown: 'M4 18h16M4 18l-1-10 5.5 4L12 5l3.5 7L21 8l-1 10',
  flame: 'M12 3s5 4.2 5 8.6A5 5 0 0 1 7 12c0-2 1-3.4 1-3.4S9 11 10.5 11 12 8 12 3z',
  refresh: 'M20 11a8 8 0 1 0-.6 4M20 5v6h-6',
  logout: 'M15 16l4-4-4-4M19 12H9M12 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h6',
  edit: 'M4 20h4L19 9a2.1 2.1 0 0 0-3-3L5 17v3z',
  dice: 'M5 5h14v14H5zM9 9h.01M15 9h.01M9 15h.01M15 15h.01M12 12h.01',
  heart: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.6l-1-1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21.2l7.8-7.8 1-1a5.5 5.5 0 0 0 0-7.8z',
}
</script>

<template>
  <svg
    class="ai"
    :class="`a-${name}`"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="PATHS[name] ?? ''" />
    <!-- crown gets literal sparkles popping at its tips -->
    <template v-if="name === 'crown'">
      <circle class="spark p1" cx="12" cy="2.6" r="0.9" />
      <circle class="spark p2" cx="21.4" cy="5.4" r="0.7" />
      <circle class="spark p3" cx="2.6" cy="5.4" r="0.7" />
    </template>
  </svg>
</template>

<style scoped>
.ai {
  display: block;
  flex: none;
  transform-origin: 50% 50%;
}

/* pencil scribbles on an invisible pad */
.a-edit { animation: ai-write 2.2s ease-in-out infinite; }
@keyframes ai-write {
  0%, 100% { transform: translate(0, 0) rotate(0deg); }
  25% { transform: translate(0.9px, -0.9px) rotate(-11deg); }
  55% { transform: translate(1.7px, 0.7px) rotate(8deg); }
  80% { transform: translate(0.6px, 1.5px) rotate(-4deg); }
}

/* dice shakes like a throw, then settles */
.a-dice { animation: ai-roll 2.6s ease-in-out infinite; }
@keyframes ai-roll {
  0%, 58%, 100% { transform: rotate(0deg) scale(1); }
  66% { transform: rotate(-15deg) scale(1.07); }
  74% { transform: rotate(12deg); }
  82% { transform: rotate(-8deg) scale(1.04); }
  90% { transform: rotate(4deg); }
}

/* crown bobs as if being lowered onto the head */
.a-crown { animation: ai-bob 2.4s ease-in-out infinite; }
@keyframes ai-bob {
  0%, 100% { transform: translateY(0) rotate(0deg); }
  50% { transform: translateY(-1.4px) rotate(-2.5deg); }
}
.spark {
  fill: currentColor;
  stroke: none;
  opacity: 0;
  transform-box: fill-box;
  transform-origin: center;
  animation: ai-sparkle 2.4s ease-in-out infinite;
}
.spark.p2 { animation-delay: 0.5s; }
.spark.p3 { animation-delay: 1.1s; }
@keyframes ai-sparkle {
  0%, 100% { opacity: 0; transform: scale(0.2); }
  40% { opacity: 1; transform: scale(1.25); }
  60% { opacity: 0.4; transform: scale(0.8); }
}

/* logout: the whole mark drifts out, comes back, drifts again */
.a-logout { animation: ai-out 2.4s var(--ease) infinite; }
@keyframes ai-out {
  0%, 55%, 100% { transform: translateX(0); }
  72% { transform: translateX(2.2px); }
  86% { transform: translateX(0.6px); }
}

/* close and check draw themselves over and over */
.a-close path, .a-check path {
  stroke-dasharray: 30;
  animation: ai-draw 2.8s var(--ease) infinite;
}
@keyframes ai-draw {
  0% { stroke-dashoffset: 30; opacity: 0.35; }
  45%, 78% { stroke-dashoffset: 0; opacity: 1; }
  100% { stroke-dashoffset: 30; opacity: 0.35; }
}

/* heart keeps a double-beat */
.a-heart { animation: ai-beat 1.6s ease-in-out infinite; }
@keyframes ai-beat {
  0%, 30%, 100% { transform: scale(1); }
  10% { transform: scale(1.18); }
  20% { transform: scale(1.04); }
}

/* user pops to say "this is you" */
.a-user { animation: ai-pop 2.2s var(--ease-out-back) infinite; }
@keyframes ai-pop {
  0%, 70%, 100% { transform: scale(1); }
  82% { transform: scale(1.12); }
}

/* bulb breathes light */
.a-bulb { animation: ai-glow 2.2s ease-in-out infinite; }
@keyframes ai-glow {
  0%, 100% { opacity: 0.8; filter: none; }
  50% { opacity: 1; filter: drop-shadow(0 0 4px currentColor); }
}

.a-refresh { animation: ai-spin 1.8s linear infinite; }
@keyframes ai-spin { to { transform: rotate(360deg); } }

/* flame flickers */
.a-flame { animation: ai-flicker 1.4s ease-in-out infinite; }
@keyframes ai-flicker {
  0%, 100% { transform: scaleY(1) rotate(0deg); }
  35% { transform: scaleY(1.08) rotate(-2deg); }
  70% { transform: scaleY(0.94) rotate(2deg); }
}

/* play pulses like a beat drop */
.a-play { animation: ai-pulse 1.8s ease-in-out infinite; }
@keyframes ai-pulse {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.12); opacity: 0.85; }
}

/* directional icons nudge where they point */
.a-chevron-down, .a-arrow-right, .a-skip { animation: ai-nudge-r 2s var(--ease) infinite; }
.a-back, .a-undo { animation: ai-nudge-l 2s var(--ease) infinite; }
.a-chevron-right { animation: ai-nudge-r 2s var(--ease) infinite; }
@keyframes ai-nudge-r {
  0%, 55%, 100% { transform: translateX(0); }
  70% { transform: translateX(1.8px); }
}
@keyframes ai-nudge-l {
  0%, 55%, 100% { transform: translateX(0); }
  70% { transform: translateX(-1.8px); }
}
.a-chevron-down { animation-name: ai-nudge-d; }
@keyframes ai-nudge-d {
  0%, 55%, 100% { transform: translateY(0); }
  70% { transform: translateY(1.8px); }
}

/* clock rocks, sound waves breathe */
.a-clock { animation: ai-rock 2.4s ease-in-out infinite; }
@keyframes ai-rock {
  0%, 100% { transform: rotate(0deg); }
  25% { transform: rotate(-7deg); }
  75% { transform: rotate(7deg); }
}
.a-sound-on, .a-sound-off { animation: ai-breathe 2s ease-in-out infinite; }
@keyframes ai-breathe {
  0%, 100% { opacity: 0.75; }
  50% { opacity: 1; }
}

/* link sways its two halves */
.a-link { animation: ai-sway 2.6s ease-in-out infinite; }
@keyframes ai-sway {
  0%, 100% { transform: rotate(0deg); }
  30% { transform: rotate(-8deg); }
  70% { transform: rotate(8deg); }
}
</style>
