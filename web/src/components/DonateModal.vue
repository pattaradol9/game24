<script setup>
// Donation prompt: the operator's PromptPay QR plus the "gift, not a sale"
// disclaimer that keeps the donation legally one-way (no consideration).
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from '../i18n/index.js'
import AnimatedIcon from './AnimatedIcon.vue'

const { t } = useI18n()
const emit = defineEmits(['close'])

function onKey(e) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="overlay">
    <div class="panel modal" role="dialog" aria-modal="true" :aria-label="t('donateTitle')">
      <button class="btn icon close" :aria-label="t('close')" @click="emit('close')">
        <AnimatedIcon name="close" :size="18" />
      </button>
      <span class="hero-ic"><AnimatedIcon name="heart" :size="26" /></span>
      <h2>{{ t('donateTitle') }}</h2>
      <p class="lead">{{ t('donateLead') }}</p>
      <div class="qr-wrap">
        <img class="qr" src="/promptpay-qr.png" alt="PromptPay QR" width="220" height="220" />
      </div>
      <p class="scan">{{ t('donateScanHint') }}</p>
      <div class="notice">
        <strong>{{ t('donateNoticeTitle') }}</strong>
        <p>{{ t('donateNotice') }}</p>
      </div>
      <p class="legal">
        {{ t('donateTermsNote') }}
        <RouterLink to="/terms">{{ t('terms') }}</RouterLink>
      </p>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.72);
}
.modal {
  position: relative;
  width: min(360px, 100%);
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 13px;
  padding: 26px;
  animation: rise-in 0.24s var(--ease);
}
.close {
  position: absolute;
  top: 12px;
  right: 12px;
  color: var(--text-mute);
}
@media (hover: hover) { .close:hover { color: var(--text); } }
/* beating heart — a gift, not a sale */
.hero-ic {
  align-self: center;
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 16px;
  color: var(--bad);
  background: rgba(239, 95, 95, 0.12);
  border: 1px solid rgba(239, 95, 95, 0.3);
  box-shadow: 0 0 24px rgba(239, 95, 95, 0.12);
}
h2 { font-size: 1.2rem; font-weight: 500; padding-right: 34px; }
.lead { font-size: 0.86rem; color: var(--text-dim); line-height: 1.6; }
.qr-wrap {
  align-self: center;
  padding: 12px;
  background: #fff;
  border-radius: var(--r-md);
  box-shadow: var(--sh-1);
}
.qr { display: block; width: 220px; height: 220px; }
.scan {
  text-align: center;
  font-size: 0.8rem;
  color: var(--text-mute);
  margin-top: -4px;
}
.notice {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 13px 15px;
  background: var(--accent-soft);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
}
.notice strong { font-size: 0.82rem; color: var(--accent-hi); font-weight: 600; }
.notice p { font-size: 0.78rem; color: var(--text-dim); line-height: 1.65; }
.legal { font-size: 0.75rem; color: var(--text-mute); line-height: 1.6; }
.legal a { color: var(--text-dim); }
@media (hover: hover) { .legal a:hover { color: var(--accent); text-decoration: underline; } }
@media (max-width: 400px) {
  .modal { padding: 20px; }
  .qr { width: 200px; height: 200px; }
}
</style>
