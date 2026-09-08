<script setup>
// Shared play surface for solo + rooms: the four cards, the operator pad,
// the hint toast — plus the merge juice (sparkle burst + floating result
// text right where the cards fused).
import { computed, nextTick, onBeforeUpdate, ref, watch } from 'vue'
import { useI18n } from '../i18n/index.js'
import { sfx } from '../audio.js'
import { sparkle, popText, centerOf, cardEl, haptic } from '../fx.js'
import CardTile from './CardTile.vue'
import OperatorPad from './OperatorPad.vue'
import ArrowText from './ArrowText.vue'
import Icon from './Icon.vue'
import SkinFx from './SkinFx.vue'

const props = defineProps({
  hand: { type: Object, default: null },
  disabled: { type: Boolean, default: false },
  hintData: { type: Object, default: null },
  dealing: { type: Boolean, default: false },
  // equipped card skin id ('' = classic); sets the --skin-* scope below
  skin: { type: String, default: '' },
})
const emit = defineEmits(['pick', 'op'])
const { t } = useI18n()

const root = ref(null)
// SkinFx handle: fires the skin's WebGL celebration at the merge point
const fx = ref(null)

// premium skins get the WebGL ambience layer behind the cards; classic/mono
// stay quiet (the :key remounts the canvas cleanly if the skin ever swaps)
const fxSkin = computed(() => !['', 'classic', 'mono'].includes(props.skin))

// The hint toast shows the full equation (and up to MAX_ALTS alternate ways),
// so a single glance answers "how do I solve this?".
const MAX_ALTS = 2
const shownAlts = computed(() => (props.hintData?.alternatives ?? []).slice(0, MAX_ALTS))
const extraCount = computed(() =>
  Math.max(0, (props.hintData?.solutionCount ?? 0) - 1 - shownAlts.value.length)
)
// solver expressions use ASCII ops; display the typographic ones
const pretty = (expr) => expr.replaceAll('*', '×').replaceAll('/', '÷')

function onPick(card) {
  if (props.disabled) return
  sfx.select()
  haptic(8)
  emit('pick', card)
}

/* ---------------------------------------------------------------------------
   Merge choreography.

   A merge removes two cards and appends one, so every surviving card changes
   position. TransitionGroup can only FLIP those into place if the two leaving
   cards are out of flow — otherwise the row keeps their gap for the whole leave
   and then snaps when they are finally removed.

   So: remember where each slot was *before* the update, then pin a leaving slot
   there absolutely. Positions have to be captured in onBeforeUpdate, since by
   the time the leave hook runs the row has already been re-laid out.
--------------------------------------------------------------------------- */
const slotRects = new Map()

onBeforeUpdate(() => {
  const host = root.value?.querySelector('.cards')
  if (!host) return
  const box = host.getBoundingClientRect()
  slotRects.clear()
  for (const el of host.children) {
    const id = el.dataset.slot
    if (!id) continue
    const r = el.getBoundingClientRect()
    slotRects.set(id, { x: r.left - box.left, y: r.top - box.top, w: r.width, h: r.height })
  }
})

function pinLeaving(el) {
  const r = slotRects.get(el.dataset.slot)
  if (!r) return
  Object.assign(el.style, {
    position: 'absolute',
    margin: '0',
    left: `${r.x}px`,
    top: `${r.y}px`,
    width: `${r.w}px`,
    height: `${r.h}px`,
  })
}

// when two cards fuse, celebrate at the new card's on-screen position
watch(
  () => props.hand?.cards.length,
  (nv, ov) => {
    if (ov == null || nv == null || nv >= ov) return
    nextTick(() => {
      const hand = props.hand
      if (!hand) return
      // the merge just recorded is the last step; its result card sits at the
      // pair's midpoint, not at the end of the row
      const merged = hand.cards.find((c) => c.id === `s${hand.steps.length - 1}`)
      if (!merged || !merged.id.startsWith('s')) return
      const c = centerOf(cardEl(merged.id))
      if (!c) return
      // the skin's WebGL celebration rides on top: a pulse for every merge,
      // the full ring + flash + spark shower when the hand resolves to 24
      fx.value?.burst?.(c.x, c.y, merged.display === '24' ? 1 : 0.45)
      sparkle(c.x, c.y, { count: merged.display === '24' ? 26 : 15, power: 1.15 })
      popText(c.x, c.y - 26, merged.display, merged.display === '24' ? 'fx-merge gold' : 'fx-merge')
    })
  }
)
</script>

<template>
  <div ref="root" class="board" :class="'skin-' + (skin || 'classic')">
    <!-- skin ambience (WebGL particles) sits behind every board element,
         and the celebration canvas rides above the cards -->
    <SkinFx ref="fx" v-if="fxSkin" :key="skin" :skin="skin" />
    <!-- dealing runs its own fly-in on the card, so the slot stays out of it -->
    <TransitionGroup :name="dealing ? 'carddeal' : 'cardf'" tag="div" class="cards" @leave="pinLeaving">
      <div v-for="(c, i) in hand.cards" :key="c.id" class="slot" :data-slot="c.id">
        <CardTile
          :card="c"
          :selected="hand.selection === c"
          :deal-delay="dealing && c.id.startsWith('c') ? i * 90 : null"
          @pick="onPick"
        />
      </div>
    </TransitionGroup>
    <OperatorPad
      :model-value="hand.operator"
      :disabled="!hand.selection || disabled"
      @update:model-value="(op) => emit('op', op)"
    />
    <p class="tip"><ArrowText :text="t('selectCards')" /></p>
    <Transition name="hint">
      <div v-if="hintData" :key="hintData.at ?? hintData.result" class="hint-pop chip">
        <div class="hint-row">
          <Icon name="bulb" :size="15" />
          <template v-if="hintData.expr">
            <code>{{ pretty(hintData.expr) }}</code>
            <span class="hint-eq">= 24</span>
          </template>
          <template v-else>
            <span>{{ hintData.leftCard + 1 }} {{ hintData.op }} {{ hintData.rightCard + 1 }} = {{ hintData.result }}</span>
          </template>
        </div>
        <div v-if="hintData.expr && shownAlts.length" class="hint-alts">
          <code v-for="a in shownAlts" :key="a" class="hint-alt">{{ pretty(a) }}</code>
        </div>
        <span v-if="extraCount > 0" class="hint-extra">{{ t('hintMoreWays').replace('{n}', extraCount) }}</span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.board {
  position: relative; /* containing block for the SkinFx layer */
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: var(--r-lg);
  padding: clamp(20px, 4vw, 34px) 18px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: clamp(18px, 3.2vw, 28px);
  box-shadow: var(--sh-2);
}
.cards {
  position: relative; /* containing block for the pinned leaving slots */
  z-index: 1; /* above the SkinFx canvas */
  display: flex;
  gap: clamp(10px, 2.8vw, 20px);
  flex-wrap: wrap;
  justify-content: center;
  min-height: calc(var(--card-w) * 1.45);
}
.slot { display: flex; }
/* keep the pad, tip and hint toast above the particle layer too */
.tip, .hint-pop { position: relative; z-index: 1; }
.board :deep(.pad) { position: relative; z-index: 1; }

/* survivors glide to their new spot instead of jumping */
.cardf-move { transition: transform 0.42s var(--ease); }
.cardf-enter-active { animation: pop-in 0.3s var(--ease-out-back); }
.cardf-leave-active {
  transition: opacity 0.26s var(--ease), transform 0.3s var(--ease);
  pointer-events: none;
  z-index: 0;
}
.cardf-leave-to { opacity: 0; transform: scale(0.6); }

.tip { font-size: 0.82rem; color: var(--text-mute); text-align: center; }
.hint-pop {
  flex-direction: column;
  align-items: center;
  gap: 3px;
  color: var(--accent);
  border-color: rgba(246, 183, 60, 0.35);
  background: var(--accent-soft);
}
.hint-row { display: flex; align-items: center; gap: 6px; }
.hint-row code, .hint-eq { font-weight: 700; letter-spacing: 0.02em; }
.hint-alts { display: flex; flex-wrap: wrap; justify-content: center; gap: 2px 14px; }
.hint-alt { font-size: 0.85rem; color: var(--text-mute); }
.hint-extra { font-size: 0.78rem; color: var(--text-mute); }
.hint-enter-active { animation: pop-in 0.3s var(--ease-out-back); }
.hint-leave-active { transition: opacity 0.3s var(--ease), transform 0.3s var(--ease); }
.hint-leave-to { opacity: 0; transform: translateY(-12px); }
@media (prefers-reduced-motion: reduce) {
  .cardf-move, .cardf-leave-active { transition: none; }
}
</style>
