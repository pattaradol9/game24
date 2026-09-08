<script setup>
// Boosts menu (phones): the server-boost status collapsed into its own
// popover, separate from the settings menu. The trigger only renders while
// a boost is actually running — an idle server shows no button at all.
// Desktop shows the BuffBar tray in the header instead.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { activeBoosts } from '../auth.js'
import Icon from './Icon.vue'

const { t } = useI18n()

const ICONS = { exp: 'bolt', coins: 'coin' }
const NAME_KEYS = { exp: 'expBoost', coins: 'coinsBoost' }

const nowMs = ref(Date.now())
let ticker = 0
onMounted(() => {
  ticker = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})
onUnmounted(() => clearInterval(ticker))

const buffs = computed(() =>
  Object.values(activeBoosts.value)
    .map((b) => {
      const left = Math.floor((Date.parse(b.endsAt) - nowMs.value) / 1000)
      const mult = Number.isInteger(b.multiplier) ? String(b.multiplier) : String(Math.round(b.multiplier * 100) / 100)
      return { ...b, left, mult, name: t(NAME_KEYS[b.kind], { n: mult }) }
    })
    .filter((b) => Number.isFinite(b.left) && b.left > 0)
)

const open = ref(false)
const wrap = ref(null)

function onDocClick(e) {
  if (open.value && wrap.value && !wrap.value.contains(e.target)) open.value = false
}
function onKey(e) {
  if (e.key === 'Escape') open.value = false
}
onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
})

const pad = (n) => String(n).padStart(2, '0')
const clock = (s) => {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h) return `${h}:${pad(m)}:${pad(s % 60)}`
  return `${m}:${pad(s % 60)}`
}
</script>

<template>
  <div v-if="buffs.length" ref="wrap" class="bmenu">
    <button
      class="btn icon gear"
      :class="{ on: open }"
      :aria-label="t('serverBoosts')"
      @click="open = !open"
    >
      <Icon name="bolt" :size="19" />
    </button>

    <Transition name="drop">
      <div v-if="open" class="sheet">
        <span class="sec-title">{{ t('serverBoosts') }}</span>
        <div v-for="b in buffs" :key="b.kind" class="row" :class="`row-${b.kind}`">
          <Icon :name="ICONS[b.kind] ?? 'bolt'" :size="18" />
          <span class="lbl">{{ b.name }}</span>
          <b class="num val">{{ clock(b.left) }}</b>
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.bmenu {
  /* phones only — the desktop header shows the inline buff tray instead */
  display: none;
  position: relative;
}
@media (max-width: 640px) {
  .bmenu { display: block; }
}

.gear.on {
  border-color: rgba(246, 183, 60, 0.55);
  color: var(--accent);
}

.sheet {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  width: min(272px, calc(100vw - 24px));
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface-2);
  box-shadow: var(--sh-3), 0 10px 44px rgba(0, 0, 0, 0.4);
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  transform-origin: top right;
  z-index: 90;
}

.sec-title {
  font-size: 0.66rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-mute);
  padding: 4px 8px 6px;
}

.row {
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: var(--r-sm);
  padding: 9px 8px;
  font-size: 0.84rem;
  color: var(--text);
}
.row.coins { color: #eda32c; }
.row.exp { color: var(--accent); }
.lbl { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.val { color: var(--text-dim); font-size: 0.78rem; }

.drop-enter-active { animation: menu-pop 0.2s var(--ease-out-back); }
@keyframes menu-pop {
  from { transform: translateY(-6px) scale(0.97); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}
.drop-leave-active { transition: opacity 0.12s var(--ease), transform 0.12s var(--ease); }
.drop-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }
</style>
