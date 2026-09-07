<script setup>
// Live list of room players with scores (multiplayer).
import { useI18n } from '../i18n/index.js'
import TierAvatar from './TierAvatar.vue'

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
      <li v-for="p in players" :key="p.id" :class="{ you: p.id === you, away: p.absent }">
        <TierAvatar :tier="p.guest ? 'guest' : p.tier" :name="p.name" :size="26" />
        <span class="name">
          {{ p.name }}
          <span v-if="p.host" class="host-tag">{{ t('hostTag') }}</span>
        </span>
        <!-- a refresh holds this row's seat: only this chip shows the wait -->
        <span v-if="p.absent" class="wait" :title="t('playerReconnecting')" />
        <span v-else class="score">{{ p.score }}</span>
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
.rail li.away .name, .rail li.away :deep(.tav) { opacity: 0.45; }
.name { flex: 1; display: flex; align-items: center; gap: 7px; font-size: 0.88rem; min-width: 0; }
.name > span:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* reconnecting spinner, swapped in for the score on the waiting row */
.wait {
  flex: none;
  width: 15px;
  height: 15px;
  border-radius: var(--r-full);
  border: 2px solid rgba(246, 183, 60, 0.25);
  border-top-color: var(--accent);
  animation: rail-spin 0.9s linear infinite;
}
@keyframes rail-spin { to { transform: rotate(360deg); } }
.host-tag {
  flex: none;
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid rgba(246, 183, 60, 0.45);
  border-radius: var(--r-full);
  padding: 2px 7px;
  line-height: 1.3;
}
.score { font-size: 1rem; font-weight: 600; color: var(--accent); font-variant-numeric: tabular-nums; }
.jump-enter-active { animation: rise-in 0.24s var(--ease); }
</style>
