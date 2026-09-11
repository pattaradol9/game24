<script setup>
import { useI18n } from '../i18n/index.js'
import Icon from './Icon.vue'

const { t } = useI18n()

defineProps({
  history: { type: Array, default: () => [] },
  // null hides the undo button entirely (views that keep undo elsewhere,
  // e.g. the room screen); true/false shows it enabled/disabled
  canUndo: { type: Boolean, default: null },
  // horizontal renders the slim strip variant used in the room's board
  // column: steps flow left-to-right and the undo button pins the far
  // right end; the default stacks lines vertically in a side panel
  horizontal: { type: Boolean, default: false },
})

defineEmits(['undo'])
</script>

<template>
  <div class="panel history" :class="{ strip: horizontal }">
    <h3 class="section-title">{{ t('steps') }}</h3>
    <div class="body">
      <TransitionGroup name="line" tag="ol">
        <li v-for="(line, i) in history" :key="i"><span class="n">{{ i + 1 }}</span>{{ line }}</li>
      </TransitionGroup>
      <p v-if="history.length === 0" class="empty">{{ t('noSteps') }}</p>
    </div>
    <button
      v-if="canUndo !== null"
      class="undo-btn"
      :disabled="!canUndo"
      :title="canUndo ? '' : t('noSteps')"
      data-test="undo-step"
      @click="$emit('undo')"
    >
      <Icon name="undo" :size="16" />{{ t('undo') }}
    </button>
  </div>
</template>

<style scoped>
.history {
  min-height: 110px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
/* the step-rollback sits at the panel's bottom edge, full width and plainly
   visible — the panel it acts on is its label */
.undo-btn {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 100%;
  border: 1px solid var(--line);
  background: var(--surface-2);
  color: var(--text-dim);
  border-radius: var(--r-sm);
  padding: 9px 12px;
  font-size: 0.85rem;
  font-weight: 600;
  font-family: var(--font);
  cursor: pointer;
  transition: color 0.15s var(--ease), border-color 0.15s var(--ease);
}
@media (hover: hover) {
  .undo-btn:hover:not(:disabled) { color: var(--text); border-color: var(--text-mute); }
}
.undo-btn:disabled { opacity: 0.45; cursor: not-allowed; }
.body { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; overflow-y: auto; overscroll-behavior: contain; }
ol { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; }
li {
  display: flex;
  align-items: baseline;
  gap: 9px;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  color: var(--text-dim);
  padding: 6px 0;
  border-bottom: 1px solid var(--line-soft);
}
li:last-child { border-bottom: none; color: var(--text); }
.n {
  flex: none;
  width: 17px;
  height: 17px;
  display: grid;
  place-items: center;
  border-radius: var(--r-xs);
  background: var(--surface-3);
  color: var(--text-mute);
  font-family: var(--font);
  font-size: 0.62rem;
}
.empty { margin: auto; color: var(--text-mute); font-size: 0.84rem; }
.line-enter-active { animation: rise-in 0.22s var(--ease); }

/* horizontal strip variant: one slim row under the board — steps flow as
   chips, latest one bright, undo pinned to the far right end */
.strip { flex-direction: row; align-items: center; gap: 12px; min-height: 0; padding: 10px 14px; }
.strip h3 { flex: none; }
.strip .body { flex-direction: row; align-items: center; overflow-x: auto; overflow-y: hidden; }
.strip ol { flex-direction: row; align-items: center; gap: 8px; }
.strip li {
  flex: none;
  align-items: center;
  gap: 7px;
  padding: 5px 10px;
  border: 1px solid var(--line-soft);
  border-radius: var(--r-sm);
  background: var(--surface-2);
  white-space: nowrap;
}
.strip .empty { margin: 0; }
.strip .undo-btn { width: auto; margin-left: auto; }
</style>
