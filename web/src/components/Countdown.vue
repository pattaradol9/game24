<script setup>
// Classic 3-2-1-GO round intro. Fires tick sounds, ends with a whoosh,
// and completes instantly under prefers-reduced-motion.
import { onMounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { sfx } from '../audio.js'
import { haptic } from '../fx.js'

const emit = defineEmits(['done'])
const { t } = useI18n()
const step = ref(3) // 3, 2, 1, then GO
const leaving = ref(false)

onMounted(() => {
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
    emit('done')
    return
  }
  const seq = [
    [0, () => { step.value = 3; sfx.count(); haptic(18) }],
    [700, () => { step.value = 2; sfx.count(); haptic(18) }],
    [1400, () => { step.value = 1; sfx.count(); haptic(18) }],
    [2100, () => { step.value = 0; sfx.count(true); sfx.whoosh(); haptic([0, 30, 40, 30]) }],
    [2500, () => { leaving.value = true }],
    [2900, () => emit('done')],
  ]
  const timers = seq.map(([ms, fn]) => setTimeout(fn, ms))
  return () => timers.forEach(clearTimeout)
})
</script>

<template>
  <div class="countdown" :class="{ out: leaving }">
    <div :key="step" class="cd">
      <span v-if="step > 0" class="n num">{{ step }}</span>
      <span v-else class="go">{{ t('go') }}</span>
    </div>
  </div>
</template>

<style scoped>
.countdown {
  position: fixed;
  inset: 0;
  display: grid;
  place-items: center;
  z-index: 150;
  pointer-events: none;
  background: rgba(6, 8, 13, 0.55);
  transition: opacity 0.3s var(--ease);
}
.countdown.out { opacity: 0; }
.cd { animation: cd-in 0.55s var(--ease-out-back); }
.n {
  font-size: clamp(5rem, 20vw, 9rem);
  font-weight: 600;
  line-height: 1;
  letter-spacing: -0.04em;
  color: var(--text);
}
.go {
  font-size: clamp(3rem, 13vw, 5.5rem);
  font-weight: 700;
  line-height: 1;
  letter-spacing: -0.02em;
  color: var(--accent);
}
@keyframes cd-in {
  0% { transform: scale(1.5); opacity: 0; }
  28% { opacity: 1; }
  55% { transform: scale(0.97); }
  100% { transform: scale(1); opacity: 1; }
}
</style>
