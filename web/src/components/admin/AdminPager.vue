<script setup>
// Minimal list pager: prev/next + "from–to of total".
defineProps({
  offset: { type: Number, required: true },
  limit: { type: Number, required: true },
  total: { type: Number, required: true },
  loading: { type: Boolean, default: false },
})
const emit = defineEmits(['page'])
</script>

<template>
  <div class="pager">
    <span class="count">
      {{ total === 0 ? 'no results' : `${offset + 1}–${Math.min(offset + limit, total)} of ${total}` }}
    </span>
    <div class="nav">
      <button class="btn quiet" :disabled="loading || offset === 0" @click="emit('page', offset - limit)">‹ Prev</button>
      <button class="btn quiet" :disabled="loading || offset + limit >= total" @click="emit('page', offset + limit)">Next ›</button>
    </div>
  </div>
</template>

<style scoped>
.pager { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding-top: 12px; border-top: 1px solid var(--line-soft); }
.count { font-size: 0.78rem; color: var(--text-mute); }
.nav { display: flex; gap: 8px; }
.nav .btn { min-height: 34px; padding: 6px 14px; font-size: 0.82rem; }
</style>
