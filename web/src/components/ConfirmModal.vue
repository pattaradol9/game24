<script setup>
// generic confirmation dialog: backdrop is inert by house rule — the dialog
// only closes through its own Cancel/OK actions
defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  body: { type: String, default: '' },
  okLabel: { type: String, default: 'OK' },
  cancelLabel: { type: String, default: '' },
  danger: { type: Boolean, default: false }, // tint the OK action
  busy: { type: Boolean, default: false },
})
defineEmits(['confirm', 'cancel'])
</script>

<template>
  <div v-if="show" class="overlay">
    <div class="panel box" role="dialog" :aria-label="title">
      <h2 class="title">{{ title }}</h2>
      <p v-if="body" class="body">{{ body }}</p>
      <div class="actions">
        <button class="btn" :disabled="busy" data-test="confirm-cancel" @click="$emit('cancel')">
          {{ cancelLabel }}
        </button>
        <button
          class="btn"
          :class="danger ? 'danger' : 'primary'"
          :disabled="busy"
          data-test="confirm-ok"
          @click="$emit('confirm')"
        >
          {{ okLabel }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 90;
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
.title { font-size: 1.2rem; font-weight: 600; letter-spacing: -0.01em; }
.body { font-size: 0.9rem; color: var(--text-dim); line-height: 1.5; max-width: 34ch; }
.actions { display: flex; gap: 9px; width: 100%; margin-top: 4px; }
.actions .btn { flex: 1; min-height: 42px; justify-content: center; font-weight: 700; }
</style>
