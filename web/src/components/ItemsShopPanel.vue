<script setup>
// Boost-item tab of the unified shop: the consumable catalog (personal
// ×2/×3 EXP or coin boosts), coin prices, and buy against the player's
// balance. Items are consumable — the buy CTA never turns into an "owned"
// state; the stack size rides on the card instead. Guests get the Google
// sign-in gate, mirroring the skins tab.
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from '../i18n/index.js'
import { api } from '../api.js'
import { currentPlayer, getToken, updatePlayer } from '../auth.js'
import { fmtDuration } from '../duration.js'
import Icon from './Icon.vue'
import GuestVeil from './GuestVeil.vue'
import FxBackdrop from './FxBackdrop.vue'

const { t, lang } = useI18n()

const emit = defineEmits(['spent'])

const loading = ref(true)
const error = ref('')
const catalog = ref([])
const owned = ref({})
const active = ref({}) // item id → endsAt RFC3339 while its window runs
const busyId = ref('')
const actionError = ref('')
const confirming = ref(null)
const count = ref(1) // units the slider commits to buying
const maxBuy = ref(1) // most units the balance (and the server bound) allow

const MAX_BUY = 200 // mirrors the server's maxItemCount bound

const player = computed(() => currentPlayer.value)
const signedIn = computed(() => !!player.value && !player.value.isGuest)
const balance = computed(() => player.value?.totalCoins ?? 0)
const fmt = (n) => (n ?? 0).toLocaleString('en-US')
const bi = (obj) => obj?.[lang.value] ?? obj?.en ?? ''
const qtyOf = (id) => owned.value[id] ?? 0
const canAfford = (it) => balance.value >= it.price
const ICONS = { exp: 'bolt', coins: 'coin', time: 'hourglass', skip: 'skip' }
const KIND_KEYS = { exp: 'itemKindExp', coins: 'itemKindCoins', time: 'itemKindTime', skip: 'itemKindSkip' }
const SHORT_KEYS = { exp: 'kindShortExp', coins: 'kindShortCoins' }
const rarityKey = (r) => 'rarity' + String(r || 'common').charAt(0).toUpperCase() + String(r || 'common').slice(1)

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.items(getToken())
    catalog.value = data.items ?? []
    owned.value = data.owned ?? {}
    active.value = data.active ?? {}
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
onMounted(load)

// live profile pushes carry the same inventory — reconcile the counts so a
// purchase made in another tab shows up without leaving the panel
watch(
  () => player.value?.items,
  (items) => {
    if (!items) return
    const next = {}
    for (const it of items) next[it.id] = it.qty
    owned.value = next
  }
)

async function buy(it) {
  if (busyId.value) return
  const n = Math.min(count.value, maxBuy.value)
  busyId.value = it.id
  actionError.value = ''
  try {
    const from = player.value?.totalCoins ?? 0
    const data = await api.buyItem(it.id, getToken(), n)
    updatePlayer(data.player)
    owned.value = { ...owned.value, [it.id]: qtyOf(it.id) + n }
    confirming.value = null
    const to = data.player?.totalCoins ?? from
    if (from !== to) emit('spent', from, to)
  } catch (e) {
    actionError.value = e.status === 400 ? t('insufficientCoins') : e.message
  } finally {
    busyId.value = ''
  }
}

function askBuy(it) {
  actionError.value = ''
  // the slider maxes at what the balance affords — every unit costs its price
  maxBuy.value = Math.max(0, Math.min(MAX_BUY, Math.floor(balance.value / it.price)))
  count.value = 1
  confirming.value = it
}
function closeConfirm() {
  if (busyId.value) return
  confirming.value = null
  actionError.value = ''
}

function onSignedIn() {
  load()
}
defineExpose({ reload: load })
</script>

<template>
  <section class="panel body">
    <FxBackdrop variant="vault" />
    <div class="content">
      <div class="head">
        <h1>{{ t('shopTabBoosts') }}</h1>
        <span class="hint">{{ t('stackNote') }}</span>
      </div>

      <p v-if="error" class="err">{{ error }}</p>

      <p v-if="signedIn && actionError" class="err">{{ actionError }}</p>

      <!-- guests browse the catalog behind a blurred veil until sign-in -->
      <GuestVeil :message="t('shopSignInRequired')" @signed-in="onSignedIn">
        <div v-if="loading" class="loading">…</div>
        <div v-else class="grid">
        <article v-for="it in catalog" :key="it.id" class="item-card" :class="'r-' + it.rarity">
          <span class="tile">
            <Icon class="glyph" :name="ICONS[it.kind] ?? 'bolt'" :size="60" />
            <Icon class="ic" :name="ICONS[it.kind] ?? 'bolt'" :size="32" />
          </span>

          <div class="info">
            <div class="top">
              <b class="name">{{ bi(it.name) }}</b>
              <span class="rarity" :class="'r-' + it.rarity">{{ t(rarityKey(it.rarity)) }}</span>
            </div>
            <p class="desc">{{ bi(it.desc) }}</p>

            <div class="meta">
              <!-- play helpers sell their in-game effect, not a multiplier:
                   time items read +30s, skip items read one hand per unit -->
              <span v-if="it.kind === 'time'" class="mult num">{{ t('itemTimeBonus', { n: it.extraSeconds ?? 30 }) }}</span>
              <span v-else-if="it.kind === 'skip'" class="mult num">{{ t('itemSkipOnce') }}</span>
              <template v-else>
                <span class="mult num">{{ t(SHORT_KEYS[it.kind] ?? 'kindShortExp') }} ×{{ it.multiplier }}</span>
                <span class="dur"><Icon name="clock" :size="14" /><b class="num">{{ fmtDuration(it.durationMinutes * 60) }}</b></span>
              </template>
              <span v-if="qtyOf(it.id)" class="bag num">{{ t('itemInBag', { n: fmt(qtyOf(it.id)) }) }}</span>
            </div>

            <div class="foot">
              <span class="chip" :class="{ poor: signedIn && !canAfford(it) }">
                <Icon name="coin" :size="18" />
                <b class="num">{{ fmt(it.price) }}</b>
              </span>
              <button
                class="btn buy-btn"
                :disabled="!signedIn || busyId === it.id || !canAfford(it)"
                @click="askBuy(it)"
              >
                <Icon name="coin" :size="20" />{{ t('buy') }}
              </button>
            </div>
          </div>
        </article>
        </div>
      </GuestVeil>
    </div>

    <!-- purchase confirmation: item, price, and the balance that remains -->
    <div v-if="confirming" class="overlay">
      <div class="panel confirm" :class="'r-' + confirming.rarity" role="dialog" :aria-label="t('confirmBuy')">
        <span class="confirm-title">{{ t('confirmBuy') }}</span>
        <div class="confirm-target">
          <span class="tile big">
            <Icon class="glyph" :name="ICONS[confirming.kind] ?? 'bolt'" :size="56" />
            <Icon class="ic" :name="ICONS[confirming.kind] ?? 'bolt'" :size="34" />
          </span>
          <div class="confirm-meta">
            <b class="name">{{ bi(confirming.name) }}</b>
            <span v-if="confirming.kind === 'time'" class="mult num">{{ t('itemTimeBonus', { n: confirming.extraSeconds ?? 30 }) }}</span>
            <span v-else-if="confirming.kind === 'skip'" class="mult num">{{ t('itemSkipOnce') }}</span>
            <span v-else class="mult num">×{{ confirming.multiplier }} · {{ fmtDuration(confirming.durationMinutes * 60) }}</span>
            <span class="price-row">
              <Icon name="coin" :size="22" />
              <b class="num">{{ fmt(confirming.price * count) }}</b>
              <span v-if="count > 1" class="unit num">×{{ count }}</span>
            </span>
          </div>
        </div>
        <p class="note">{{ confirming.kind === 'time' ? t('timeItemShopNote') : confirming.kind === 'skip' ? t('skipItemShopNote') : t('useItemNote') }}</p>
        <!-- how many units to buy: maxed by what the balance affords -->
        <div v-if="maxBuy > 1" class="countrow">
          <div class="counthead">
            <label for="buy-count">{{ t('buyCount') }}</label>
            <b class="countval num">×{{ count }}</b>
          </div>
          <input id="buy-count" v-model.number="count" type="range" min="1" :max="maxBuy" />
          <span class="counthint">{{ t('itemMaxBuyable', { n: fmt(maxBuy) }) }}</span>
        </div>
        <div class="after">
          <span>{{ t('balanceAfter') }}</span>
          <span class="after-val num">
            {{ fmt(balance) }} <Icon class="arrow" name="arrow-right" :size="14" />
            <b :class="{ zero: balance - confirming.price * count === 0 }">{{ fmt(balance - confirming.price * count) }}</b>
          </span>
        </div>
        <p v-if="actionError" class="err">{{ actionError }}</p>
        <div class="confirm-actions">
          <button class="btn" :disabled="busyId === confirming.id" @click="closeConfirm">{{ t('cancel') }}</button>
          <button class="btn buy-btn confirm-buy" :disabled="busyId === confirming.id" @click="buy(confirming)">
            <Icon name="coin" :size="20" />{{ t('itemBuyN', { n: count }) }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.body { position: relative; overflow: clip; padding: 26px 28px 30px; }
/* the WebGL ambience lives on .body (z 0); the content rides above it */
.content { position: relative; z-index: 1; display: flex; flex-direction: column; gap: 18px; }
.head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
h1 { font-size: 1.4rem; font-weight: 600; letter-spacing: -0.01em; }
.hint { font-size: 0.8rem; color: var(--text-mute); }
.err { color: var(--bad); font-size: 0.85rem; }
.loading { text-align: center; color: var(--text-mute); font-size: 1.4rem; padding: 60px 0; }

/* guests browse behind GuestVeil's blur; nothing gate-specific left here */

/* --rc defaults to the common tier here; each card's r-* class retints it
   for everything inside, tile included */
.grid {
  --rc: var(--rarity-common);
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(215px, 1fr));
  gap: 14px;
}
/* rarity palette — one accent (--rc) drives the tile, the wash, the kind pill
   and the hover glow of each card, mirroring the skin-card rarity chips.
   The tier dials scale the aura with prestige: --r-line tints the card
   border, --r-wash the corner wash, --r-haze the resting outer glow. */
.r-common { --rc: var(--rarity-common); --r-line: 22%; --r-wash: 12%; --r-haze: 8%; }
.r-rare { --rc: var(--info); --r-line: 48%; --r-wash: 22%; --r-haze: 16%; }
.r-epic { --rc: #b283f0; --r-line: 58%; --r-wash: 28%; --r-haze: 20%; }
.r-legend { --rc: var(--accent); --r-line: 66%; --r-wash: 32%; --r-haze: 24%; }

.item-card {
  position: relative;
  /* vertical shop card: tile on top, copy below — every card in a row shares
     one height and Thai copy gets a full-width line to live on */
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 12px;
  border: 1px solid var(--line);
  /* the rarity reads before anything else: the border drinks --rc and the
     card sits in a soft halo of its tier colour */
  border-color: color-mix(in srgb, var(--rc) var(--r-line), var(--line));
  box-shadow: 0 6px 24px color-mix(in srgb, var(--rc) var(--r-haze), transparent);
  /* translucent so the vault motes behind shimmer through the card */
  background: linear-gradient(180deg,
    color-mix(in srgb, var(--surface-2) 74%, transparent),
    color-mix(in srgb, var(--surface-2) 92%, transparent));
  border-radius: var(--r-md);
  padding: 16px 16px 14px;
  overflow: hidden;
  transition: transform 0.18s var(--ease), border-color 0.18s var(--ease), box-shadow 0.18s var(--ease);
}
/* rarity wash bleeding in from the tile edge, plus a haze pooling behind it */
.item-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(60% 46% at 50% 0%, color-mix(in srgb, var(--rc) calc(var(--r-wash) * 1.5), transparent), transparent 74%),
    linear-gradient(102deg, color-mix(in srgb, var(--rc) var(--r-wash), transparent), transparent 54%);
  pointer-events: none;
}
/* legend is the only tier that breathes — the halo slowly pulses */
@media (prefers-reduced-motion: no-preference) {
  .item-card.r-legend { animation: rarity-breathe 3.6s ease-in-out infinite; }
}
@keyframes rarity-breathe {
  0%, 100% { box-shadow: 0 6px 22px color-mix(in srgb, var(--rc) calc(var(--r-haze) * 0.6), transparent); }
  50% { box-shadow: 0 8px 32px color-mix(in srgb, var(--rc) calc(var(--r-haze) * 2), transparent); }
}
@media (hover: hover) and (prefers-reduced-motion: no-preference) {
  .item-card:hover {
    transform: translateY(-2px);
    border-color: color-mix(in srgb, var(--rc) 70%, var(--line));
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.35), 0 0 28px color-mix(in srgb, var(--rc) 26%, transparent);
  }
}

/* the item tile: a glowing reliquary tinted by rarity — radial core,
   watermark glyph, a slow conic light sweep circling the icon, and the icon
   bobbing above it all */
.tile {
  position: relative;
  flex: none;
  align-self: center;
  display: grid;
  place-items: center;
  width: 92px;
  height: 122px;
  margin-top: 2px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--rc) 52%, transparent);
  background:
    radial-gradient(64% 52% at 50% 36%, color-mix(in srgb, var(--rc) 38%, transparent), transparent 74%),
    linear-gradient(168deg, color-mix(in srgb, var(--rc) 18%, transparent), color-mix(in srgb, var(--rc) 5%, transparent) 70%),
    var(--surface-3);
  overflow: hidden;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 0 14px color-mix(in srgb, var(--rc) 10%, transparent),
    0 6px 16px color-mix(in srgb, var(--rc) 8%, transparent);
}
.tile .glyph {
  position: absolute;
  color: var(--rc);
  opacity: 0.15;
}
.tile .ic {
  position: relative;
  z-index: 1;
  color: var(--rc);
  filter: drop-shadow(0 0 8px color-mix(in srgb, var(--rc) 32%, transparent));
}
@media (prefers-reduced-motion: no-preference) {
  .tile .ic { animation: item-bob 3.6s ease-in-out infinite; }
  .tile::after {
    content: '';
    position: absolute;
    inset: -45%;
    background: conic-gradient(from 0turn,
      transparent 0turn 0.6turn,
      color-mix(in srgb, var(--rc) 70%, white 12%) 0.84turn,
      transparent 1turn);
    animation: item-turn 7s linear infinite;
    opacity: 0.55;
  }
}
@keyframes item-bob {
  0%, 100% { transform: translateY(2px); }
  50% { transform: translateY(-3px); }
}
@keyframes item-turn {
  to { transform: rotate(1turn); }
}
.tile.big { width: 88px; height: 88px; border-radius: 16px; }

.top { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
.name { font-size: 0.98rem; font-weight: 600; letter-spacing: -0.01em; line-height: 1.25; }
.rarity {
  flex: none;
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  border-radius: var(--r-full);
  padding: 3px 8px;
  line-height: 1;
  color: var(--rc);
  border: 1px solid color-mix(in srgb, var(--rc) 40%, transparent);
  background: color-mix(in srgb, var(--rc) 10%, transparent);
}
/* the top tiers badge their own light */
.rarity.r-epic, .rarity.r-legend {
  box-shadow: 0 0 12px color-mix(in srgb, var(--rc) 35%, transparent);
}
.desc {
  font-size: 0.78rem;
  color: var(--text-mute);
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
/* the copy block: fixed-height desc keeps meta/foot rows aligned across a row */
.info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.info .desc { min-height: calc(1.45em * 3); }

.meta { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 8px; font-size: 0.78rem; color: var(--text-dim); }
.meta .mult {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
  padding: 2px 10px;
  border-radius: var(--r-full);
  font-size: 0.84rem;
  font-weight: 700;
  color: var(--rc);
  border: 1px solid color-mix(in srgb, var(--rc) 40%, transparent);
  background: color-mix(in srgb, var(--rc) 12%, transparent);
}
.meta .dur { display: inline-flex; align-items: center; gap: 4px; }
.meta .dur b { color: var(--text); font-weight: 600; }
.meta .bag { margin-left: auto; color: var(--good); font-weight: 600; }

.foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: auto; padding-top: 10px; }
.foot .chip { padding: 4px 10px; font-size: 0.75rem; }
.foot .chip b { color: var(--text); }
.foot .chip.poor { color: var(--bad); border-color: rgba(239, 95, 95, 0.35); }
.foot .chip.poor b { color: var(--bad); }
.foot .btn { width: auto; min-width: 96px; justify-content: center; min-height: 34px; padding: 7px 14px; font-size: 0.85rem; }
.foot .btn.buy-btn {
  --btn-bg: linear-gradient(180deg, var(--accent-hi), var(--accent));
  --btn-line: var(--accent);
  --btn-fg: var(--accent-ink);
  font-weight: 700;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4), 0 3px 14px rgba(246, 183, 60, 0.35);
}
@media (hover: hover) {
  .foot .btn.buy-btn:hover:not(:disabled) {
    --btn-bg: linear-gradient(180deg, #ffe08f, var(--accent-hi));
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4), 0 5px 20px rgba(246, 183, 60, 0.55);
  }
}

.overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.74);
}
.confirm {
  display: flex;
  flex-direction: column;
  gap: 14px;
  width: min(360px, 100%);
  padding: 24px;
  animation: rise-in 0.26s var(--ease);
}
.confirm-title { font-size: 1.2rem; font-weight: 600; letter-spacing: -0.01em; }
.confirm-target { display: flex; align-items: center; gap: 16px; }
.confirm-meta { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; min-width: 0; }
.confirm-meta .name { font-size: 1.05rem; font-weight: 600; letter-spacing: -0.01em; }
.confirm-meta .mult { font-size: 0.85rem; color: var(--text-dim); }
.price-row {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--accent);
  margin-top: 2px;
}
.price-row .unit { font-size: 0.8rem; font-weight: 600; color: var(--text-dim); }
/* units-to-buy slider, maxed by the balance */
.countrow { display: flex; flex-direction: column; gap: 7px; }
.counthead { display: flex; align-items: baseline; justify-content: space-between; }
.counthead label {
  font-size: 0.7rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--text-mute);
}
.countval { font-size: 1.1rem; color: var(--accent); }
input[type='range'] { width: 100%; accent-color: var(--accent); }
.counthint { font-size: 0.7rem; color: var(--text-mute); }
.note {
  font-size: 0.78rem;
  color: var(--text-mute);
  line-height: 1.5;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 10px 13px;
  background: var(--bg);
}
.after {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 0.85rem;
  color: var(--text-mute);
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 10px 13px;
  background: var(--bg);
}
.after-val { display: inline-flex; align-items: center; gap: 6px; color: var(--text); font-weight: 600; }
.after-val .arrow { color: var(--text-mute); font-style: normal; }
.after-val b { color: var(--accent); }
.after-val b.zero { color: var(--text-mute); }
.confirm-actions { display: flex; justify-content: flex-end; gap: 10px; }
.confirm-actions .btn { min-height: 40px; padding: 9px 16px; }
.confirm-actions .buy-btn { min-width: 110px; }

@media (max-width: 560px) {
  .body { padding: 20px 16px 24px; }
  .grid { grid-template-columns: 1fr; }
}
</style>
