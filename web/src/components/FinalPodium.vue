<script setup>
import { computed } from 'vue'
import { useI18n } from '../i18n/index.js'
import TierAvatar from './TierAvatar.vue'
import CrownMark from './CrownMark.vue'

const { t } = useI18n()
const props = defineProps({
  show: { type: Boolean, default: false },
  standings: { type: Array, default: () => [] },
  // the match ended because the host left: say so under the title
  hostLeft: { type: Boolean, default: false },
})
defineEmits(['home'])

const order = computed(() => {
  const s = props.standings
  return { second: s[1] ?? null, first: s[0] ?? null, third: s[2] ?? null }
})
</script>

<template>
  <div v-if="show" class="overlay">
    <div class="panel modal">
      <CrownMark :size="68" />
      <h2>{{ t('finalResult') }}</h2>
      <p v-if="hostLeft" class="host-note">{{ t('hostLeftNote') }}</p>
      <div class="podium">
        <div v-for="(col, name) in order" :key="name" class="col">
          <template v-if="col">
            <TierAvatar :tier="col.guest ? 'guest' : col.tier" :name="col.name" :size="44" />
            <span class="pname">{{ col.name }}</span>
            <span class="pts">{{ col.score }}</span>
            <div class="block" :class="name">
              <span>{{ col.rank ?? '' }}</span>
            </div>
          </template>
          <template v-else><div class="block ghost" /></template>
        </div>
      </div>
      <table v-if="standings.length > 3" class="extra">
        <tbody>
          <tr v-for="p in standings.slice(3)" :key="p.id">
            <td>{{ p.rank }}.</td>
            <td>{{ p.name }}</td>
            <td>{{ p.score }}</td>
          </tr>
        </tbody>
      </table>
      <button class="btn primary big" @click="$emit('home')">{{ t('backHome') }}</button>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 150;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.78);
}
.modal {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
  width: min(460px, 100%);
  padding: 28px;
  text-align: center;
  animation: rise-in 0.26s var(--ease);
}
h2 { font-size: 1.25rem; font-weight: 500; }
.host-note { margin-top: -10px; font-size: 0.85rem; line-height: 1.45; color: var(--text-mute); }
.podium { display: flex; align-items: flex-end; justify-content: center; gap: 10px; width: 100%; }
.col { display: flex; flex-direction: column; align-items: center; gap: 6px; flex: 1; min-width: 0; }
.pname { font-size: 0.88rem; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.pts { font-size: 0.78rem; color: var(--text-mute); font-variant-numeric: tabular-nums; }
.block {
  width: 100%;
  display: grid;
  place-items: center;
  border: 1px solid var(--line);
  border-bottom: none;
  border-radius: var(--r-sm) var(--r-sm) 0 0;
  background: var(--surface-2);
  color: var(--text-dim);
  font-size: 1.2rem;
  font-weight: 600;
}
.block.first { height: 92px; color: var(--accent); border-color: rgba(246, 183, 60, 0.45); background: var(--accent-soft); }
.block.second { height: 66px; }
.block.third { height: 48px; }
</style>
