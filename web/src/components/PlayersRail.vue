<script setup>
// Live list of room players with scores (multiplayer).
import { useI18n } from '../i18n/index.js'
import TierBadge from './TierBadge.vue'

const { t } = useI18n()
defineProps({
  players: { type: Array, default: () => [] },
  you: { type: String, default: '' },
})
</script>

<template>
  <div class="panel rail">
    <h3>{{ t('vs') }}</h3>
    <TransitionGroup name="jump" tag="ul">
      <li v-for="p in players" :key="p.id" :class="{ you: p.id === you }">
        <span class="avatar">{{ (p.name || '?').slice(0, 1).toUpperCase() }}</span>
        <span class="name">
          {{ p.name }}
          <TierBadge v-if="!p.guest" :tier="p.tier" size="sm" />
        </span>
        <span class="score">{{ p.score }}</span>
      </li>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.rail { padding: 16px 18px; }
.rail h3 { font-size: 0.78rem; font-weight: 500; letter-spacing: 0.14em; text-transform: uppercase; color: var(--text-mute); }
.rail ul { list-style: none; margin: 10px 0 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.rail li {
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: var(--r-sm);
  padding: 8px 10px;
  border: 1px solid transparent;
  transition: border-color 0.2s var(--ease), background 0.2s var(--ease);
}
.rail li.you { border-color: rgba(246, 183, 60, 0.35); background: var(--accent-soft); }
.avatar {
  flex: none;
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: var(--r-xs);
  background: var(--surface-3);
  color: var(--text-dim);
  font-size: 0.75rem;
  font-weight: 600;
}
.name { flex: 1; display: flex; align-items: center; gap: 7px; font-size: 0.88rem; min-width: 0; }
.name > span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.score { font-size: 1rem; font-weight: 600; color: var(--accent); font-variant-numeric: tabular-nums; }
.jump-enter-active { animation: rise-in 0.24s var(--ease); }
</style>
