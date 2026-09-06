<script setup>
// Shared play surface for solo + rooms: the four cards, the operator pad,
// the hint toast — plus the merge juice (sparkle burst + floating result
// text right where the cards fused).
import { nextTick, onBeforeUpdate, ref, watch } from 'vue'
import { useI18n } from '../i18n/index.js'
import { sfx } from '../audio.js'
import { sparkle, popText, centerOf, cardEl, haptic } from '../fx.js'
import CardTile from './CardTile.vue'
import OperatorPad from './OperatorPad.vue'
import Icon from './Icon.vue'

const props = defineProps({
  hand: { type: Object, default: null },
  disabled: { type: Boolean, default: false },
  hintData: { type: Object, default: null },
  dealing: { type: Boolean, default: false },
})
const emit = defineEmits(['pick', 'op'])
const { t } = useI18n()

const root = ref(null)

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
      const cards = props.hand?.cards ?? []
      const merged = cards[cards.length - 1]
      if (!merged || !merged.id.startsWith('s')) return
      const c = centerOf(cardEl(merged.id))
      if (!c) return
      sparkle(c.x, c.y, { count: merged.display === '24' ? 26 : 15, power: 1.15 })
      popText(c.x, c.y - 26, merged.display, merged.display === '24' ? 'fx-merge gold' : 'fx-merge')
    })
  }
)
</script>

<template>
  <div ref="root" class="board">
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
    <p class="tip">{{ t('selectCards') }}</p>
    <Transition name="hint">
      <div v-if="hintData" :key="hintData.result" class="hint-pop chip">
        <Icon name="bulb" :size="15" />
        {{ hintData.leftCard + 1 }} {{ hintData.op }} {{ hintData.rightCard + 1 }} = {{ hintData.result }}
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.board {
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
  display: flex;
  gap: clamp(10px, 2.8vw, 20px);
  flex-wrap: wrap;
  justify-content: center;
  min-height: calc(var(--card-w) * 1.45);
}
.slot { display: flex; }

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
  color: var(--accent);
  border-color: rgba(246, 183, 60, 0.35);
  background: var(--accent-soft);
  animation: float-up 4s var(--ease) forwards;
}
@media (prefers-reduced-motion: reduce) {
  .cardf-move, .cardf-leave-active { transition: none; }
}
</style>
