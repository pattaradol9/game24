<script setup>
// "Time over — extend?" dialog. Shown when a solo hand's clock hits zero and
// the player still holds a Time Extension item and hasn't spent this
// session's single bailout yet: the round parks on 0:00 until they either
// spend one unit (+30s on THIS hand) or let the hand go.
import { useI18n } from '../i18n/index.js'
import Icon from './Icon.vue'

const { t } = useI18n()

defineProps({
  show: { type: Boolean, default: false },
  qty: { type: Number, default: 0 }, // units left in the bag
  busy: { type: Boolean, default: false }, // extend request in flight
})
defineEmits(['resolve'])
</script>

<template>
  <div v-if="show" class="overlay">
    <div class="panel box" role="dialog" :aria-label="t('timeExtendTitle')">
      <span class="tile">
        <Icon class="glyph" name="hourglass" :size="56" />
        <Icon class="ic" name="hourglass" :size="30" />
      </span>
      <h2 class="title">{{ t('timeExtendTitle') }}</h2>
      <p class="body">{{ t('timeExtendBody', { n: 30 }) }}</p>
      <p class="note">{{ t('timeExtendOnce') }}</p>
      <div class="actions">
        <button class="btn" :disabled="busy" data-test="decline-extend" @click="$emit('resolve', false)">
          {{ t('timeExtendDecline') }}
        </button>
        <button class="btn use-btn" :disabled="busy" data-test="confirm-extend" @click="$emit('resolve', true)">
          <Icon name="hourglass" :size="17" />{{ t('timeExtendUse', { n: 30 }) }}
          <b class="qty num">×{{ qty }}</b>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 95;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.74);
}
.box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: min(360px, 100%);
  padding: 26px 24px 22px;
  text-align: center;
  animation: rise-in 0.26s var(--ease);
}
/* the item tile, tinted like the shop's common tier */
.tile {
  --rc: var(--rarity-common);
  position: relative;
  display: grid;
  place-items: center;
  width: 78px;
  height: 78px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--rc) 52%, transparent);
  background:
    radial-gradient(64% 52% at 50% 36%, color-mix(in srgb, var(--rc) 38%, transparent), transparent 74%),
    linear-gradient(168deg, color-mix(in srgb, var(--rc) 18%, transparent), color-mix(in srgb, var(--rc) 5%, transparent) 70%),
    var(--surface-3);
  overflow: hidden;
  box-shadow: 0 0 20px color-mix(in srgb, var(--rc) 22%, transparent);
}
.tile .glyph { position: absolute; color: var(--rc); opacity: 0.15; }
.tile .ic { position: relative; z-index: 1; color: var(--rc); filter: drop-shadow(0 0 8px color-mix(in srgb, var(--rc) 60%, transparent)); }
@media (prefers-reduced-motion: no-preference) {
  .tile .ic { animation: tip-bob 3.6s ease-in-out infinite; }
}
@keyframes tip-bob {
  0%, 100% { transform: translateY(2px); }
  50% { transform: translateY(-3px); }
}
.title { font-size: 1.2rem; font-weight: 600; letter-spacing: -0.01em; }
.body { font-size: 0.9rem; color: var(--text-dim); line-height: 1.5; max-width: 30ch; }
.note {
  font-size: 0.75rem;
  color: var(--text-mute);
  line-height: 1.45;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 8px 12px;
  background: var(--bg);
}
.actions { display: flex; flex-direction: column; gap: 9px; width: 100%; margin-top: 4px; }
.actions .btn { min-height: 42px; justify-content: center; }
.use-btn {
  --btn-bg: linear-gradient(180deg, var(--accent-hi), var(--accent));
  --btn-line: var(--accent);
  --btn-fg: var(--accent-ink);
  font-weight: 700;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4), 0 3px 14px rgba(246, 183, 60, 0.35);
}
@media (hover: hover) {
  .use-btn:hover:not(:disabled) {
    --btn-bg: linear-gradient(180deg, #ffe08f, var(--accent-hi));
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4), 0 5px 20px rgba(246, 183, 60, 0.55);
  }
}
.qty {
  background: rgba(0, 0, 0, 0.18);
  border-radius: var(--r-full);
  min-width: 20px;
  padding: 1px 6px;
  font-size: 0.72rem;
}
</style>
