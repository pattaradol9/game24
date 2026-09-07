<script setup>
// The banner that slaps onto the screen when you level up or reach a new
// tier. Tier-ups wear the colour of the tier just reached and come with a
// bobbing medal; level-ups get a popping star.
import { computed } from 'vue'
import { useI18n } from '../i18n/index.js'

const { t } = useI18n()

const props = defineProps({
  show: { type: Boolean, default: false },
  kind: { type: String, default: 'level' }, // level | tier
  value: { type: String, default: '' },
})

const TIER_COLORS = {
  bronze: '#c07a3e',
  silver: '#a9b1c1',
  gold: '#f6b73c',
  platinum: '#6fd6c8',
  diamond: '#78a8f5',
  master: '#b283f0',
}
const tierColor = computed(() =>
  props.kind === 'tier' ? (TIER_COLORS[props.value?.toLowerCase()] ?? TIER_COLORS.gold) : TIER_COLORS.gold
)
</script>

<template>
  <Transition name="cele">
    <div v-if="show" class="cele" role="status">
      <div class="banner" :class="kind" :style="{ '--tc': tierColor }">
        <svg v-if="kind === 'tier'" class="medal" viewBox="0 0 24 24" width="46" height="46" fill="none" aria-hidden="true">
          <path d="M8.2 1.5l2.6 5.4M15.8 1.5l-2.6 5.4" stroke="var(--tc)" stroke-width="2" stroke-linecap="round" />
          <circle class="coin" cx="12" cy="14.6" r="6.1" fill="var(--tc)" stroke="rgba(255, 255, 255, 0.75)" stroke-width="1.2" />
          <path class="gleam" d="M12 11.2l1.1 2.3 2.5.3-1.85 1.75.5 2.5L12 16.8l-2.25 1.25.5-2.5L8.4 13.8l2.5-.3z" fill="rgba(255, 255, 255, 0.92)" />
        </svg>
        <svg v-else class="medal" viewBox="0 0 24 24" width="46" height="46" fill="none" aria-hidden="true">
          <path class="star-big" d="M12 2l2.9 6.2 6.6.8-4.9 4.6 1.3 6.6L12 17l-5.9 3.2 1.3-6.6L2.5 9l6.6-.8z" fill="var(--good)" stroke="rgba(255, 255, 255, 0.75)" stroke-width="1" />
        </svg>
        <span class="section-title">{{ kind === 'level' ? t('levelUp') : t('tierUp') }}</span>
        <span class="value">{{ value }}</span>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.cele {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: start center;
  padding-top: 14vh;
  pointer-events: none;
  z-index: 200;
}
.banner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 18px 34px;
  border-radius: var(--r-md);
  background: var(--surface-2);
  border: 1px solid var(--line);
  box-shadow: var(--sh-3);
  animation: rise-in 0.28s var(--ease);
}
.banner.tier { border-color: color-mix(in srgb, var(--tc) 55%, transparent); }
.value { font-size: 1.8rem; font-weight: 600; letter-spacing: -0.02em; }
.banner.level .value { color: var(--good); }
.banner.tier .value { color: var(--tc); text-transform: capitalize; }

.medal {
  filter: drop-shadow(0 4px 14px rgba(0, 0, 0, 0.5));
  animation: cele-bob 1.6s ease-in-out infinite;
}
@keyframes cele-bob {
  0%, 100% { transform: translateY(0) rotate(-3deg); }
  50% { transform: translateY(-3px) rotate(3deg); }
}
.gleam, .star-big {
  transform-box: fill-box;
  transform-origin: center;
}
.gleam { animation: cele-gleam 1.6s ease-in-out infinite; }
@keyframes cele-gleam {
  0%, 100% { opacity: 0.85; }
  50% { opacity: 1; }
}
.star-big { animation: cele-pop 1.4s var(--ease-out-back) infinite; }
@keyframes cele-pop {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.14); }
}

.cele-leave-active { transition: opacity 0.3s var(--ease), transform 0.3s var(--ease); }
.cele-leave-to { opacity: 0; transform: translateY(-14px); }
</style>
