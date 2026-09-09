<script setup>
// Boosts menu (phones): the live boost status collapsed into its own
// popover, separate from the settings menu — server-wide campaigns and the
// player's own item boosts, stacked additively per payout kind. The trigger
// only renders while something is actually running — an idle server and an
// empty inventory show no button at all. Desktop shows the BuffBar tray in
// the header instead.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { boostColorKey, itemBoosts, mergeBoosts, serverBoosts } from '../auth.js'
import { fmtDuration } from '../duration.js'
import Icon from './Icon.vue'

const { t, lang } = useI18n()

const ICONS = { exp: 'bolt', coins: 'coin' }
const NAME_KEYS = { exp: 'expBoostAny', coins: 'coinsBoostAny' }

const nowMs = ref(Date.now())
let ticker = 0
onMounted(() => {
  ticker = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})
onUnmounted(() => clearInterval(ticker))

const alive = (map, now) =>
  Object.fromEntries(Object.entries(map).filter(([, b]) => Date.parse(b.endsAt) > now))

const bi = (obj) => obj?.[lang.value] ?? obj?.en ?? ''
const rarityKey = (r) => 'rarity' + String(r || 'common').charAt(0).toUpperCase() + String(r || 'common').slice(1)
const fmtMult = (m) => (Number.isInteger(m) ? String(m) : String(Math.round(m * 100) / 100))

const buffs = computed(() =>
  Object.values(mergeBoosts(alive(serverBoosts.value, nowMs.value), alive(itemBoosts.value, nowMs.value)))
    .map((b) => {
      const left = Math.floor((Date.parse(b.endsAt) - nowMs.value) / 1000)
      return {
        ...b,
        left,
        mult: fmtMult(b.multiplier),
        colorKey: boostColorKey(b.sources),
        leftLabel: fmtDuration(left),
        name: t(NAME_KEYS[b.kind] ?? 'expBoostAny', { n: fmtMult(b.multiplier) }),
        rows: b.sources.map((s) => ({
          colorKey: boostColorKey([s]),
          label:
            s.origin === 'server'
              ? t('boostFromServer', { n: fmtMult(s.multiplier) })
              : s.name
                ? `${bi(s.name)} ×${fmtMult(s.multiplier)} · ${t(rarityKey(s.rarity))}`
                : t('boostFromItem', { n: fmtMult(s.multiplier) }),
          leftLabel: fmtDuration((Date.parse(s.endsAt) - nowMs.value) / 1000),
        })),
      }
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
</script>

<template>
  <div v-if="buffs.length" ref="wrap" class="bmenu">
    <button
      class="btn icon gear"
      :class="{ on: open }"
      :aria-label="t('boostsTitle')"
      @click="open = !open"
    >
      <Icon name="bolt" :size="19" />
    </button>

    <Transition name="drop">
      <div v-if="open" class="sheet">
        <span class="sec-title">{{ t('boostsTitle') }}</span>
        <div v-for="b in buffs" :key="b.kind" class="row" :class="'c-' + b.colorKey">
          <div class="rowmain">
            <Icon :name="ICONS[b.kind] ?? 'bolt'" :size="18" />
            <span class="lbl">{{ b.name }}</span>
            <b class="num val">{{ b.leftLabel }}</b>
          </div>
          <div v-for="row in b.rows" :key="row.label" class="sub" :class="'c-' + row.colorKey">
            <span class="dot" aria-hidden="true" />
            <span class="sublbl">{{ row.label }}</span>
            <b class="num subleft">{{ row.leftLabel }}</b>
          </div>
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
  --cc: var(--accent);
  display: flex;
  flex-direction: column;
  gap: 3px;
  border-radius: var(--r-sm);
  padding: 9px 8px;
  font-size: 0.84rem;
  color: var(--cc);
}
/* pedigree colours: reserved server teal, then the rarity ladder */
.c-server { --cc: #29d3be; }
.c-common { --cc: var(--text-dim); }
.c-rare { --cc: var(--info); }
.c-epic { --cc: #b283f0; }
.c-legend { --cc: var(--accent); }
.rowmain { display: flex; align-items: center; gap: 10px; }
.lbl { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.val { color: var(--text-dim); font-size: 0.78rem; }
.sub {
  display: flex;
  align-items: center;
  gap: 7px;
  padding-left: 28px;
  font-size: 0.7rem;
  color: var(--cc);
}
.sub .dot {
  flex: none;
  width: 6px;
  height: 6px;
  border-radius: var(--r-full);
  background: var(--cc);
  box-shadow: 0 0 6px color-mix(in srgb, var(--cc) 60%, transparent);
}
.sublbl { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text-mute); }
.subleft { color: var(--text-mute); font-weight: 600; font-size: 0.66rem; }

.drop-enter-active { animation: menu-pop 0.2s var(--ease-out-back); }
@keyframes menu-pop {
  from { transform: translateY(-6px) scale(0.97); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}
.drop-leave-active { transition: opacity 0.12s var(--ease), transform 0.12s var(--ease); }
.drop-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }
</style>
