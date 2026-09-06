<script setup>
// Who you are and how far to the next level, in one block. Reads straight from
// the live session, so it updates the moment a round banks its EXP.
import { computed } from 'vue'
import { useI18n } from '../i18n/index.js'
import { currentPlayer } from '../auth.js'

const props = defineProps({
  compact: { type: Boolean, default: false }, // header variant: hides the name on phones
})
const { t } = useI18n()

const player = computed(() => currentPlayer.value)
const initial = computed(() => (player.value?.nickname || '?').slice(0, 1).toUpperCase())
const pct = computed(() => {
  const p = player.value
  if (!p || !p.levelExpForNext) return 100
  return Math.min(100, Math.max(0, (p.levelExpInto / p.levelExpForNext) * 100))
})
</script>

<template>
  <span v-if="player" class="pchip" :class="{ compact }">
    <img v-if="player.picture" :src="player.picture" referrerpolicy="no-referrer" alt="" />
    <span v-else class="avatar">{{ initial }}</span>
    <span class="meta">
      <span class="row">
        <b class="name">{{ player.nickname }}</b>
        <span class="lv num">Lv.{{ player.level }}</span>
      </span>
      <span v-if="player.isGuest" class="tag">{{ t('profileGuest') }}</span>
      <span v-else class="bar">
        <span class="fill" :style="{ width: pct + '%' }" />
      </span>
    </span>
  </span>
</template>

<style scoped>
.pchip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  line-height: 1;
}
img, .avatar {
  flex: none;
  width: 32px;
  height: 32px;
  border-radius: var(--r-xs);
  object-fit: cover;
}
.avatar {
  display: grid;
  place-items: center;
  background: var(--accent);
  color: var(--accent-ink);
  font-size: 0.95rem;
  font-weight: 700;
}
.meta { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.row { display: flex; align-items: center; gap: 8px; min-width: 0; }
.name {
  font-size: 0.9rem;
  font-weight: 500;
  max-width: 118px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.lv {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--accent);
  background: var(--accent-soft);
  border-radius: var(--r-xs);
  padding: 3px 6px;
  line-height: 1;
}
.tag { font-size: 0.68rem; color: var(--text-mute); }
.bar { width: 92px; height: 4px; border-radius: var(--r-full); background: var(--surface-3); overflow: hidden; }
.fill {
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: var(--accent);
  transition: width 0.7s var(--ease);
}

@media (max-width: 640px) {
  .compact .name { display: none; }
  .compact .bar { width: 62px; }
}
</style>
