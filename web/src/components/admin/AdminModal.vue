<script setup>
// Lightweight modal for admin dialogs. Deliberately sticky: only the ✕
// button closes it — backdrop clicks and Escape are ignored so a stray
// click mid-edit never throws away the form.
const emit = defineEmits(['close'])
defineProps({ title: { type: String, default: '' } })
</script>

<template>
  <teleport to="body">
    <div class="overlay">
      <div class="dialog panel">
        <header class="head">
          <h3>{{ title }}</h3>
          <button class="btn quiet close" title="Close" aria-label="Close" @click="emit('close')">✕</button>
        </header>
        <div class="body">
          <slot />
        </div>
      </div>
    </div>
  </teleport>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  background: rgba(5, 7, 12, 0.62);
  display: grid;
  place-items: center;
  padding: 20px;
}
.dialog {
  width: min(640px, 100%);
  max-height: min(82vh, 720px);
  display: flex;
  flex-direction: column;
  box-shadow: var(--sh-3);
}
.head { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 16px 20px 12px; }
.head h3 { font-size: 1.02rem; }
.close { min-height: 34px; width: 34px; padding: 0; }
.body { padding: 4px 20px 20px; overflow-y: auto; }
</style>
