<script setup>
// Who you are and how far to the next level, in one block. Reads straight from
// the live session, so it updates the moment a round banks its EXP.
import { computed } from 'vue'
import { useI18n } from '../i18n/index.js'
import { currentPlayer } from '../auth.js'
import TierAvatar from './TierAvatar.vue'

const props = defineProps({
  compact: { type: Boolean, default: false }, // header variant: hides the name on phones
})
const { t } = useI18n()

const player = computed(() => currentPlayer.value)
const tier = computed(() =>
  !player.value || player.value.isGuest ? 'guest' : player.value.tier
)
const pct = computed(() => {
  const p = player.value
  if (!p || !p.levelExpForNext) return 100
  return Math.min(100, Math.max(0, (p.levelExpInto / p.levelExpForNext) * 100))
})
</script>

<template>
  <span v-if="player" class="pchip" :class="{ compact }">
    <TierAvatar :tier="tier" :src="player.picture" :name="player.nickname" :size="32" />
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
  font-weight: 700;
  color: var(--accent-ink);
  background: linear-gradient(135deg, var(--accent-hi), var(--accent));
  border-radius: var(--r-full);
  padding: 3px 7px;
  line-height: 1;
  box-shadow: 0 1px 6px rgba(246, 183, 60, 0.35);
}
.tag { font-size: 0.68rem; color: var(--text-mute); }
.bar { width: 92px; height: 5px; border-radius: var(--r-full); background: var(--surface-3); overflow: hidden; }
.fill {
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: linear-gradient(90deg, var(--accent), var(--accent-hi));
  box-shadow: 0 0 8px rgba(246, 183, 60, 0.45);
  transition: width 0.7s var(--ease);
}

@media (max-width: 640px) {
  .compact .name { display: none; }
  .compact .bar { width: 62px; }
}
</style>
