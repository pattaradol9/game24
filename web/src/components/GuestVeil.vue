<script setup>
// Guest veil: the wrapped content (shop grids, achievement cards) shows
// through a blur while a sign-in gate rides above it. The gate pins near the
// viewport top while scrolling, so the CTA never leaves the screen on long
// lists. The veil lifts reactively the moment a Google account signs in;
// the parent listens for signed-in to reload its token-scoped data.
import { computed } from 'vue'
import { currentPlayer } from '../auth.js'
import GoogleSignIn from './GoogleSignIn.vue'
import Icon from './Icon.vue'

defineProps({
  // gate copy — bilingual through i18n at the call site
  message: { type: String, required: true },
})
const emit = defineEmits(['signed-in'])
const isGuest = computed(() => !!currentPlayer.value?.isGuest)
</script>

<template>
  <div class="veil-area">
    <div class="veil-content" :class="{ veiled: isGuest }" :aria-hidden="isGuest">
      <slot />
    </div>

    <div v-if="isGuest" class="veil">
      <div class="gate">
        <Icon name="lock" :size="20" />
        <p>{{ message }}</p>
        <GoogleSignIn @signed-in="emit('signed-in')" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.veil-area { position: relative; }
.veil-content.veiled {
  filter: blur(7px);
  pointer-events: none;
  user-select: none;
}
.veil {
  position: absolute;
  inset: 0;
  display: flex;
  justify-content: center;
  align-items: flex-start;
}
.gate {
  position: sticky;
  top: 90px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  border: 1px dashed var(--line);
  border-radius: var(--r-md);
  padding: 26px 18px;
  color: var(--text-dim);
  text-align: center;
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
  backdrop-filter: blur(4px);
  /* 352 = the 312px sign-in button + the gate's own 18px side paddings */
  max-width: 352px;
}
.gate p { font-size: 0.9rem; max-width: 30ch; line-height: 1.5; }
</style>
