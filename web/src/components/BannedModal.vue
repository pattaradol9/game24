<script setup>
// Global ban verdict dialog. `banNotice` flips when the live profile socket
// announces a ban (or a session restore / reconnect auth bumps into one);
// App.vue keeps this mounted on every route so the kick is impossible to
// miss. The session is already cleared — acknowledging is the only action,
// and only the button dismisses it (no click-away).
import { banNotice } from '../auth.js'
import { useI18n } from '../i18n/index.js'
import Icon from './Icon.vue'

const { t } = useI18n()

function dismiss() {
  banNotice.value = null
}
</script>

<template>
  <div v-if="banNotice" class="overlay" role="alertdialog" :aria-label="t('bannedTitle')">
    <div class="panel">
      <span class="hero-ic"><Icon name="lock" :size="24" /></span>
      <h3>{{ t('bannedTitle') }}</h3>
      <p class="lead">{{ t('bannedBody') }}</p>
      <p v-if="banNotice.reason" class="reason">
        <span class="reason-label">{{ t('bannedReason') }}</span>
        {{ banNotice.reason }}
      </p>
      <button class="btn" @click="dismiss">{{ t('bannedOkBtn') }}</button>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  /* the ban verdict must top every other dialog (login/nickname, donate,
     celebration… all sit at 200 or below) — a kick is never hidden behind
     the very dialog it interrupts */
  z-index: 250;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.78);
}
.panel {
  width: min(400px, 100%);
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 24px;
  border-color: rgba(239, 95, 95, 0.35);
  animation: rise-in 0.22s var(--ease);
}
.hero-ic {
  align-self: center;
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 16px;
  color: var(--bad);
  background: rgba(239, 95, 95, 0.12);
  border: 1px solid rgba(239, 95, 95, 0.35);
  box-shadow: 0 0 24px rgba(239, 95, 95, 0.12);
}
h3 { font-size: 1.15rem; font-weight: 500; }
.lead { font-size: 0.88rem; line-height: 1.65; color: var(--text-dim); }
.reason {
  font-size: 0.85rem;
  color: var(--text);
  background: rgba(239, 95, 95, 0.08);
  border: 1px solid rgba(239, 95, 95, 0.25);
  border-radius: var(--r-md);
  padding: 10px 12px;
  overflow-wrap: anywhere;
}
.reason-label {
  display: block;
  font-size: 0.7rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--bad);
  margin-bottom: 4px;
}
</style>
