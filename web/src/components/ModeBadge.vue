<script setup>
// The "which difficulty am I playing" badge in the game header.
import { computed } from 'vue'
import { useI18n } from '../i18n/index.js'
import { modeMeta } from '../modes.js'
import Suit from './Suit.vue'

const props = defineProps({
  mode: { type: String, required: true },
  round: { type: String, default: '' }, // optional trailing detail, e.g. "3/12"
})
const { t } = useI18n()
const meta = computed(() => modeMeta(props.mode))
</script>

<template>
  <span class="mode-badge">
    <span class="pip"><Suit :name="meta.suit" :size="16" /></span>
    <span class="text">
      <b>{{ t(mode) }}</b>
      <i>{{ round || `×${meta.mult} EXP` }}</i>
    </span>
  </span>
</template>

<style scoped>
.mode-badge {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  background: var(--surface);
  padding: 6px 14px 6px 6px;
  min-height: 44px;
  line-height: 1;
}
.pip {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: var(--r-xs);
  background: var(--surface-3);
}
.text { display: flex; flex-direction: column; gap: 3px; }
.text b { font-size: 0.92rem; font-weight: 500; }
.text i {
  font-style: normal;
  font-size: 0.68rem;
  letter-spacing: 0.06em;
  color: var(--text-mute);
  font-variant-numeric: tabular-nums;
}
</style>
