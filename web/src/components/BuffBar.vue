<script setup>
// Ragnarok-style buff tray, one cell per payout kind with something running.
// Cells are deliberately terse — icon, stacked multiplier, compact countdown
// — and hover (or keyboard focus) opens a small popup with the full story:
// every source with its name, rarity tint and own countdown. Colour is the
// pedigree: pure server-wide campaigns wear the reserved server teal, cells
// fed by an item wear the highest rarity among those items. A source that
// expires quietly stops contributing and the cell recolours/drops to match.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { boostColorKey, itemBoosts, mergeBoosts, serverBoosts } from '../auth.js'
import { fmtDuration } from '../duration.js'
import { useI18n } from '../i18n/index.js'
import Icon from './Icon.vue'

const { t, lang } = useI18n()

const nowMs = ref(Date.now())
let ticker = 0
onMounted(() => {
  ticker = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})
onUnmounted(() => clearInterval(ticker))

const ICONS = { exp: 'bolt', coins: 'coin' }
const NAME_KEYS = { exp: 'expBoostAny', coins: 'coinsBoostAny' }
const KIND_KEYS = { exp: 'itemKindExp', coins: 'itemKindCoins' }

const fmtMult = (m) => (Number.isInteger(m) ? String(m) : String(Math.round(m * 100) / 100))
const bi = (obj) => obj?.[lang.value] ?? obj?.en ?? ''
const rarityKey = (r) => 'rarity' + String(r || 'common').charAt(0).toUpperCase() + String(r || 'common').slice(1)

// prune expired sources first so the stack reflects right now, then add the
// survivors' bonuses together (×2 + ×2 shows ×3, never the compounded ×4)
const alive = (map, now) =>
  Object.fromEntries(Object.entries(map).filter(([, b]) => Date.parse(b.endsAt) > now))

const buffs = computed(() =>
  Object.values(mergeBoosts(alive(serverBoosts.value, nowMs.value), alive(itemBoosts.value, nowMs.value)))
    .map((b) => {
      const left = Math.floor((Date.parse(b.endsAt) - nowMs.value) / 1000)
      const sources = b.sources.map((s) => ({
        ...s,
        leftLabel: fmtDuration((Date.parse(s.endsAt) - nowMs.value) / 1000),
      }))
      return {
        kind: b.kind,
        left,
        leftLabel: fmtDuration(left),
        mult: fmtMult(b.multiplier),
        colorKey: boostColorKey(b.sources),
        name: t(NAME_KEYS[b.kind] ?? 'expBoostAny', { n: fmtMult(b.multiplier) }),
        kindLabel: t(KIND_KEYS[b.kind] ?? 'itemKindExp'),
        // hover-popup rows: one per source, each with its own countdown
        rows: sources.map((s) => ({
          colorKey: boostColorKey([s]),
          label:
            s.origin === 'server'
              ? t('boostFromServer', { n: fmtMult(s.multiplier) })
              : s.name
                ? `${bi(s.name)} ×${fmtMult(s.multiplier)} · ${t(rarityKey(s.rarity))}`
                : t('boostFromItem', { n: fmtMult(s.multiplier) }),
          leftLabel: s.leftLabel,
        })),
      }
    })
    .filter((b) => Number.isFinite(b.left) && b.left > 0)
)
</script>

<template>
  <TransitionGroup v-if="buffs.length" name="buffpop" tag="div" class="buffbar" aria-live="polite">
    <span
      v-for="b in buffs"
      :key="b.kind"
      class="buff"
      :class="'c-' + b.colorKey"
      tabindex="0"
    >
      <Icon :name="ICONS[b.kind] ?? 'bolt'" :size="16" />
      <span class="txt">
        <b class="num mult">×{{ b.mult }}</b>
        <span class="num left">{{ b.leftLabel }}</span>
      </span>

      <!-- hover / focus popup: the full breakdown the terse cell omits -->
      <span class="tip" role="tooltip">
        <b class="tip-title">{{ b.name }}</b>
        <span class="tip-sub">{{ b.kindLabel }}</span>
        <span v-for="row in b.rows" :key="row.label" class="tip-row" :class="'c-' + row.colorKey">
          <span class="dot" aria-hidden="true" />
          <span class="tip-label">{{ row.label }}</span>
          <b class="num tip-left">{{ row.leftLabel }}</b>
        </span>
      </span>
    </span>
  </TransitionGroup>
</template>

<style scoped>
/* an inline tray: pages slot it into their header cluster — each cell is a
   compact square tile (icon / ×N / countdown) that shows the minimum */
.buffbar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.buff {
  /* --cc comes from the c-* pedigree class below */
  --buff-c: var(--cc, var(--accent));
  position: relative;
  /* a compact square tile: icon on top, stacked multiplier + countdown under */
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1px;
  width: 50px;
  height: 50px;
  padding: 0;
  border-radius: 13px;
  border: 1px solid color-mix(in srgb, var(--buff-c) 45%, transparent);
  background:
    linear-gradient(color-mix(in srgb, var(--buff-c) 12%, transparent), color-mix(in srgb, var(--buff-c) 12%, transparent)),
    var(--surface);
  color: var(--buff-c);
  cursor: help;
  animation: buff-glow 2.4s ease-in-out infinite;
}
/* pedigree colours: reserved server teal, then the rarity ladder */
.c-server { --cc: #29d3be; }
.c-common { --cc: var(--rarity-common); }
.c-rare { --cc: var(--info); }
.c-epic { --cc: #b283f0; }
.c-legend { --cc: var(--accent); }

.txt {
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1.12;
}
.mult { font-size: 0.72rem; font-weight: 700; letter-spacing: -0.02em; }
.left { font-size: 0.54rem; color: var(--text-mute); }

@keyframes buff-glow {
  0%, 100% { box-shadow: 0 0 0 0 transparent; }
  50% { box-shadow: 0 0 12px 0 color-mix(in srgb, var(--buff-c) 30%, transparent); }
}

/* ---------- hover popup ---------- */
/* The tray lives in page headers at the very top of the viewport, so the
   popup must drop BELOW the cell — above it it would clip off-screen and
   hide the content. */
.tip {
  position: absolute;
  left: 50%;
  top: calc(100% + 9px);
  transform: translateX(-50%) translateY(-3px);
  display: none;
  flex-direction: column;
  gap: 5px;
  min-width: 190px;
  max-width: min(264px, calc(100vw - 24px));
  padding: 11px 13px;
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface-2);
  box-shadow: var(--sh-3), 0 12px 40px rgba(0, 0, 0, 0.45);
  z-index: 80;
  pointer-events: none;
  white-space: nowrap;
}
.tip::after {
  /* the little arrow anchoring the popup to its cell */
  content: '';
  position: absolute;
  left: 50%;
  bottom: 100%;
  transform: translateX(-50%);
  border: 6px solid transparent;
  border-bottom-color: var(--line);
}
.buff:hover .tip,
.buff:focus .tip,
.buff:focus-visible .tip {
  display: flex;
  animation: tip-in 0.16s var(--ease);
}
@keyframes tip-in {
  from { opacity: 0; transform: translateX(-50%) translateY(-7px); }
  to { opacity: 1; transform: translateX(-50%) translateY(-3px); }
}
.tip-title { font-size: 0.86rem; font-weight: 700; color: var(--text); }
.tip-sub {
  font-size: 0.62rem;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-mute);
  margin: -2px 0 2px;
}
.tip-row {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 0.76rem;
  color: var(--text-dim);
}
.tip-row .dot {
  flex: none;
  width: 7px;
  height: 7px;
  border-radius: var(--r-full);
  background: var(--cc, var(--accent));
  box-shadow: 0 0 6px color-mix(in srgb, var(--cc, var(--accent)) 60%, transparent);
}
.tip-row .tip-label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
.tip-row .tip-left { color: var(--text-mute); font-weight: 600; font-size: 0.72rem; }

.buffpop-enter-active { animation: rise-in 0.3s var(--ease-out-back); }
.buffpop-enter-from,
.buffpop-leave-to { opacity: 0; transform: translateY(-8px) scale(0.9); }
.buffpop-leave-active { transition: all 0.2s var(--ease); }
</style>
