<script setup>
// Avatar wrapped in a tier frame — the public identity mark across the
// leaderboard, player rails and podiums. The badge with the tier name
// lives only in the profile menu; everywhere else the frame *is* the tier.
//
// Effect ladder (index = rank): guests get a plain grey ring on purpose —
// nothing glamorous to lose, everything to gain. Each rank above adds
// metal depth, then glow, a passing sheen, sparkles, and finally a
// rotating aura, so climbing tiers is visibly worth it.
import { computed } from 'vue'

const props = defineProps({
  tier: { type: String, default: 'guest' },
  src: { type: String, default: '' }, // picture url; falls back to initial
  name: { type: String, default: '?' },
  size: { type: Number, default: 36 },
})

const LADDER = ['guest', 'bronze', 'silver', 'gold', 'platinum', 'diamond', 'master']
const COLORS = {
  guest: '#8a94a8',
  bronze: '#c07a3e',
  silver: '#a9b1c1',
  gold: '#f6b73c',
  platinum: '#6fd6c8',
  diamond: '#78a8f5',
  master: '#b283f0',
}

const tier = computed(() => (LADDER.includes(props.tier) ? props.tier : 'guest'))
const rank = computed(() => LADDER.indexOf(tier.value))
const color = computed(() => COLORS[tier.value])
const initial = computed(() => (props.name || '?').slice(0, 1).toUpperCase())
</script>

<template>
  <span class="tav" :class="`r${rank}`" :style="{ '--tc': color, '--size': size + 'px' }">
    <span class="ring">
      <span v-if="rank >= 5" class="aura" />
      <span class="disc">
        <img v-if="src" :src="src" referrerpolicy="no-referrer" alt="" />
        <span v-else class="initial">{{ initial }}</span>
      </span>
      <span v-if="rank >= 2" class="sheen" />
    </span>
    <i v-if="rank >= 3" class="sp s1" />
    <i v-if="rank >= 4" class="sp s2" />
    <i v-if="rank >= 6" class="sp s3" />
  </span>
</template>

<style scoped>
.tav {
  position: relative;
  display: inline-block;
  flex: none;
  width: var(--size);
  height: var(--size);
  /* ring thickness grows with the avatar so small chips stay readable */
  --ring: max(2px, calc(var(--size) * 0.07));
}

/* ---------- the frame ---------- */
.ring {
  position: absolute;
  inset: 0;
  border-radius: calc(var(--size) * 0.3);
  padding: var(--ring);
  overflow: hidden; /* clips the rotating aura to the ring */
  background: var(--surface-3);
  box-shadow: inset 0 0 0 1px var(--line), 0 2px 8px rgba(0, 0, 0, 0.45);
}
.r1 .ring {
  background: linear-gradient(135deg, #e09a5a 0%, #a05f2c 48%, #7c4a22 100%);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.45);
}
.r2 .ring {
  background: linear-gradient(135deg, #e8edf5 0%, #939db0 55%, #6f7a90 100%);
  box-shadow: 0 0 10px rgba(169, 177, 193, 0.35), 0 2px 8px rgba(0, 0, 0, 0.45);
}
.r3 .ring {
  background: linear-gradient(135deg, #ffe08a 0%, #f6b73c 45%, #b07d1a 100%);
  box-shadow: 0 0 12px rgba(246, 183, 60, 0.5), 0 2px 8px rgba(0, 0, 0, 0.45);
}
.r4 .ring {
  background: linear-gradient(135deg, #d9fbf5 0%, #6fd6c8 50%, #2e9a8d 100%);
  box-shadow: 0 0 14px rgba(111, 214, 200, 0.55), 0 2px 8px rgba(0, 0, 0, 0.45);
}
.r5 .ring {
  background: linear-gradient(135deg, #cfe4ff 0%, #78a8f5 50%, #3f6fc0 100%);
  box-shadow: 0 0 16px rgba(120, 168, 245, 0.6), 0 2px 8px rgba(0, 0, 0, 0.45);
}
.r6 .ring {
  background: linear-gradient(135deg, #e3d0ff 0%, #b283f0 50%, #6d3fb8 100%);
  box-shadow: 0 0 14px rgba(178, 131, 240, 0.5), 0 2px 8px rgba(0, 0, 0, 0.45);
  animation: tav-pulse 2.4s ease-in-out infinite;
}
@keyframes tav-pulse {
  0%, 100% { box-shadow: 0 0 14px rgba(178, 131, 240, 0.5), 0 2px 8px rgba(0, 0, 0, 0.45); }
  50% { box-shadow: 0 0 22px rgba(178, 131, 240, 0.85), 0 2px 8px rgba(0, 0, 0, 0.45); }
}

/* ---------- rotating aura: diamond keeps a slow streak, master a fast one ---------- */
.aura {
  position: absolute;
  inset: -55%;
  background: conic-gradient(
    from 0deg,
    transparent 0deg 250deg,
    var(--tc) 295deg,
    rgba(255, 255, 255, 0.95) 310deg,
    var(--tc) 325deg,
    transparent 370deg
  );
  animation: tav-spin 3.6s linear infinite;
}
.r6 .aura { animation-duration: 2.2s; }
@keyframes tav-spin { to { transform: rotate(360deg); } }

/* ---------- avatar face ---------- */
.disc {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 100%;
  border-radius: calc(var(--size) * 0.3 - var(--ring));
  overflow: hidden;
  display: grid;
  place-items: center;
  background: linear-gradient(180deg, var(--surface-3), var(--surface-2));
}
.r0 .disc { background: var(--surface-3); }
.disc img { width: 100%; height: 100%; object-fit: cover; display: block; }
.initial {
  font-size: calc(var(--size) * 0.42);
  font-weight: 700;
  line-height: 1;
  color: var(--tc);
}
.r0 .initial { color: var(--text-mute); }

/* ---------- sheen sweeping across the frame ---------- */
.sheen {
  position: absolute;
  inset: 0;
  z-index: 2;
  border-radius: inherit;
  overflow: hidden;
  pointer-events: none;
}
.sheen::before {
  content: '';
  position: absolute;
  top: -25%;
  bottom: -25%;
  width: 30%;
  left: -60%;
  transform: skewX(-20deg);
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.38), transparent);
  animation: tav-glide 4.2s var(--ease) infinite;
}
.r6 .sheen::before { animation-duration: 2.8s; }
@keyframes tav-glide {
  0%, 55% { left: -60%; }
  80%, 100% { left: 135%; }
}

/* ---------- sparkles around the frame ---------- */
.sp {
  position: absolute;
  z-index: 3;
  width: 24%;
  height: 24%;
  min-width: 7px;
  min-height: 7px;
  background: #fff;
  clip-path: polygon(50% 0%, 62% 38%, 100% 50%, 62% 62%, 50% 100%, 38% 62%, 0% 50%, 38% 38%);
  filter: drop-shadow(0 0 3px var(--tc));
  opacity: 0;
  pointer-events: none;
  animation: tav-twinkle 2.6s ease-in-out infinite;
}
.s1 { top: -7%; right: -6%; }
.s2 { bottom: -4%; left: -8%; animation-delay: 1.3s; }
.s3 { top: 24%; right: -11%; animation-delay: 0.7s; }
@keyframes tav-twinkle {
  0%, 100% { opacity: 0; transform: scale(0.3) rotate(0deg); }
  45% { opacity: 1; transform: scale(1) rotate(90deg); }
  60% { opacity: 0.85; transform: scale(0.85) rotate(90deg); }
}
</style>
