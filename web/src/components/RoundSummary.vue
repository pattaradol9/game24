<script setup>
import { useI18n } from '../i18n/index.js'

const { t } = useI18n()
defineProps({
  show: { type: Boolean, default: false },
  result: { type: Object, default: null }, // { winner, winnerName, expr, points, standings }
  roundNo: { type: Number, default: 0 },
})
</script>

<template>
  <Transition name="fade">
    <div v-if="show && result" class="overlay">
      <div class="panel modal">
        <p class="round">{{ t('round') }} {{ roundNo }}</p>
        <h2 :class="result.winner ? 'win' : 'meh'">
          {{ result.winner ? `${t('winner')}: ${result.winnerName}` : t('noWinner') }}
        </h2>
        <p v-if="result.expr" class="expr">{{ result.expr }} {{ result.points ? `+${result.points}` : '' }}</p>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 90;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.66);
}
.modal {
  text-align: center;
  width: min(400px, 100%);
  padding: 26px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  animation: rise-in 0.24s var(--ease);
}
.round { font-size: 0.75rem; letter-spacing: 0.14em; text-transform: uppercase; color: var(--text-mute); }
h2 { font-size: 1.3rem; font-weight: 600; }
.win { color: var(--good); }
.meh { color: var(--bad); }
.expr {
  font-family: var(--font-mono);
  font-size: 0.98rem;
  color: var(--accent);
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 11px 14px;
}
.fade-enter-active, .fade-leave-active { transition: opacity 0.22s var(--ease); }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
