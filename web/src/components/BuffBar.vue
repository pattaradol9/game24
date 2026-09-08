<script setup>
// Ragnarok-style buff tray: one framed cell per running server-wide boost,
// each ticking down to the end of its window. An inline element — pages slot
// it into their header cluster (desktop); on phones the boost status lives
// in its own BoostsMenu popover instead. Hidden entirely when nothing is
// running; a boost that expires without a push (tab asleep) silently drops
// off when its countdown hits zero — the next profile fetch or boost push
// reconciles the view.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { activeBoosts } from '../auth.js'
import { useI18n } from '../i18n/index.js'
import Icon from './Icon.vue'

const { t } = useI18n()

const nowMs = ref(Date.now())
let ticker = 0
onMounted(() => {
  ticker = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})
onUnmounted(() => clearInterval(ticker))

const ICONS = { exp: 'bolt', coins: 'coin' }
const NAME_KEYS = { exp: 'expBoost', coins: 'coinsBoost' }

const fmtMult = (m) => (Number.isInteger(m) ? String(m) : String(Math.round(m * 100) / 100))

const buffs = computed(() =>
  Object.values(activeBoosts.value)
    .map((b) => {
      const left = Math.floor((Date.parse(b.endsAt) - nowMs.value) / 1000)
      return { ...b, left, mult: fmtMult(b.multiplier), name: t(NAME_KEYS[b.kind], { n: fmtMult(b.multiplier) }) }
    })
    .filter((b) => Number.isFinite(b.left) && b.left > 0)
)

const pad = (n) => String(n).padStart(2, '0')
const clock = (s) => {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h) return `${h}:${pad(m)}:${pad(s % 60)}`
  return `${m}:${pad(s % 60)}`
}
</script>

<template>
  <TransitionGroup v-if="buffs.length" name="buffpop" tag="div" class="buffbar" aria-live="polite">
    <span
      v-for="b in buffs"
      :key="b.kind"
      class="buff"
      :class="`buff-${b.kind}`"
      :title="`${b.name} — ${clock(b.left)}`"
    >
      <Icon :name="ICONS[b.kind] ?? 'bolt'" :size="19" />
      <span class="txt">
        <b class="num mult">×{{ b.mult }}</b>
        <span class="num left">{{ clock(b.left) }}</span>
      </span>
    </span>
  </TransitionGroup>
</template>

<style scoped>
/* an inline tray: pages slot it into their header cluster — each cell is
   sized to match the neighbouring header buttons (48px, like the profile
   chip), with the multiplier and countdown stacked beside the icon */
.buffbar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.buff {
  --buff-c: var(--accent);
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  gap: 7px;
  /* the countdown sheds digits as it ticks down — pin the box so the header
     never jitters sideways */
  min-width: 118px;
  min-height: 48px;
  padding: 0 11px;
  border-radius: var(--r-md);
  border: 1px solid color-mix(in srgb, var(--buff-c) 45%, transparent);
  background:
    linear-gradient(color-mix(in srgb, var(--buff-c) 12%, transparent), color-mix(in srgb, var(--buff-c) 12%, transparent)),
    var(--surface);
  color: var(--buff-c);
  cursor: help;
  animation: buff-glow 2.4s ease-in-out infinite;
}
.buff-coins { --buff-c: #eda32c; }
.buff.exp { --buff-c: var(--accent); }

.txt {
  display: flex;
  flex-direction: column;
  align-items: center;
  /* widest countdown is h:mm:ss (8 digits, tabular) */
  min-width: 9ch;
  line-height: 1.15;
}
.mult { font-size: 0.8rem; font-weight: 700; }
.left { font-size: 0.6rem; color: var(--text-mute); }

@keyframes buff-glow {
  0%, 100% { box-shadow: 0 0 0 0 transparent; }
  50% { box-shadow: 0 0 12px 0 color-mix(in srgb, var(--buff-c) 30%, transparent); }
}

.buffpop-enter-active { animation: rise-in 0.3s var(--ease-out-back); }
.buffpop-enter-from,
.buffpop-leave-to { opacity: 0; transform: translateY(-8px) scale(0.9); }
.buffpop-leave-active { transition: all 0.2s var(--ease); }
</style>
