<script setup>
// Card-skin tab of the unified shop: live card previews themed by the same
// --skin-* scopes the game board uses, coin prices, and buy/equip against
// the player's balance. Guests browse the catalog behind a blurred veil
// (GuestVeil) until a Google account signs in. Page chrome (header, balance,
// tabs) lives in ShopView; this panel only owns the skin grid.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { api } from '../api.js'
import { currentPlayer, getToken, updatePlayer } from '../auth.js'
import Icon from './Icon.vue'
import GuestVeil from './GuestVeil.vue'
import FxBackdrop from './FxBackdrop.vue'

const { t, lang } = useI18n()

const emit = defineEmits(['spent'])

const loading = ref(true)
const error = ref('')
const skins = ref([])
const owned = ref([])
const equipped = ref('')
const busyId = ref('')
const actionError = ref('')

// purchase confirmation + deduction feedback state
const confirming = ref(null) // skin awaiting purchase confirmation
const justBought = ref('')
let justBoughtTimer = 0

const player = computed(() => currentPlayer.value)
// guests (and no session at all) cannot buy or equip — show the gate instead
const signedIn = computed(() => !!player.value && !player.value.isGuest)
const balance = computed(() => player.value?.totalCoins ?? 0)
const fmt = (n) => (n ?? 0).toLocaleString('en-US')

const rarityKey = (r) => 'rarity' + String(r || '').charAt(0).toUpperCase() + String(r || '').slice(1)
const bi = (obj) => obj?.[lang.value] ?? obj?.en ?? ''
const isOwned = (s) => s.price === 0 || owned.value.includes(s.id)
const isEquipped = (s) => equipped.value === s.id
const canAfford = (s) => balance.value >= s.price

// Server payloads encode the default skin as "" (players.skin and the skins
// catalog's equipped field); the catalog id is "classic", so normalize before
// the shop compares ids or classic would never show as equipped.
const equippedId = (p, fallback) => (p ? p.skin || 'classic' : fallback)

// the preview pip — same heart shape Suit.vue prints, but inlining it here
// lets it inherit the skin's red via currentColor
const HEART = 'M12 21.2C5.6 16.5 2.8 13.1 2.8 9.6 2.8 6.7 5 4.6 7.8 4.6c1.8 0 3.3.9 4.2 2.3.9-1.4 2.4-2.3 4.2-2.3 2.8 0 5 2.1 5 5 0 3.5-2.8 6.9-9.2 11.6Z'

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.skins(getToken())
    skins.value = data.skins ?? []
    owned.value = data.owned ?? []
    equipped.value = data.equipped || 'classic'
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function buy(s) {
  if (busyId.value) return
  busyId.value = s.id
  actionError.value = ''
  try {
    const from = player.value?.totalCoins ?? 0
    const data = await api.buySkin(s.id, getToken())
    updatePlayer(data.player)
    if (!owned.value.includes(s.id)) owned.value = [...owned.value, s.id]
    // the fresh player JSON is the source of truth for what's equipped
    equipped.value = equippedId(data.player, equipped.value)
    confirming.value = null
    celebratePurchase(s, from, data.player?.totalCoins ?? from)
  } catch (e) {
    actionError.value = e.status === 400 ? t('insufficientCoins') : e.message
  } finally {
    busyId.value = ''
  }
}

function askBuy(s) {
  actionError.value = ''
  confirming.value = s
}
function closeConfirm() {
  if (busyId.value) return
  confirming.value = null
  actionError.value = ''
}

/* Purchase feedback: the card glows and the page header (which owns the
   balance chip) pops the "-N" deduction off it. */
function celebratePurchase(s, from, to) {
  justBought.value = s.id
  clearTimeout(justBoughtTimer)
  justBoughtTimer = setTimeout(() => { if (justBought.value === s.id) justBought.value = '' }, 2200)
  if (from !== to) emit('spent', from, to)
}

onUnmounted(() => clearTimeout(justBoughtTimer))

/* Foil previews track the pointer through --mx/--my (same vars CardTile uses);
   one delegated listener covers the whole grid. */
function onPreviewMove(e) {
  if (!matchMedia('(hover: hover) and (pointer: fine)').matches || matchMedia('(prefers-reduced-motion: reduce)').matches) return
  const card = e.target.closest?.('.pcard')
  if (!card) return
  const r = card.getBoundingClientRect()
  card.style.setProperty('--mx', `${(((e.clientX - r.left) / r.width) * 100).toFixed(1)}%`)
  card.style.setProperty('--my', `${(((e.clientY - r.top) / r.height) * 100).toFixed(1)}%`)
}

async function equip(s) {
  if (busyId.value) return
  busyId.value = s.id
  actionError.value = ''
  try {
    const data = await api.equipSkin(s.id, getToken())
    updatePlayer(data.player)
    equipped.value = equippedId(data.player, s.id)
  } catch (e) {
    actionError.value = e.message
  } finally {
    busyId.value = ''
  }
}

function onSignedIn() {
  load()
}
defineExpose({ reload: load })
</script>

<template>
  <section class="panel body">
    <FxBackdrop variant="atelier" />
    <div class="content">
    <div class="head">
      <h1>{{ t('shopTabSkins') }}</h1>
      <span class="hint">{{ t('skinsHint') }}</span>
    </div>
    <p v-if="error" class="err">{{ error }}</p>

    <p v-if="signedIn && actionError" class="err">{{ actionError }}</p>

    <GuestVeil :message="t('skinSignInRequired')" @signed-in="onSignedIn">
      <div v-if="loading" class="loading">…</div>
      <div v-else class="grid" @pointermove="onPreviewMove">
      <article
        v-for="s in skins"
        :key="s.id"
        class="skin-card"
        :class="['r-' + s.rarity, { equipped: isEquipped(s), bought: justBought === s.id }]"
        :data-skin="s.id"
      >
        <div class="preview" :class="'skin-' + s.id">
          <!-- a faithful miniature of the real CardTile: same anatomy
               (holo, frame, corner indices, watermark, rank) so the shop
               shows the card you will actually play -->
          <span class="pcard">
            <i class="holo" aria-hidden="true" />
            <i class="pframe" aria-hidden="true" />
            <span class="idx tl"><b class="num">7</b><svg class="pip" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="HEART" /></svg></span>
            <svg class="watermark" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="HEART" /></svg>
            <b class="rank num">7</b>
            <span class="idx br"><b class="num">7</b><svg class="pip" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="HEART" /></svg></span>
          </span>
        </div>

        <div class="info">
          <div class="skin-top">
            <b class="skin-name">{{ s.name }}</b>
            <span class="rarity" :class="'r-' + s.rarity">{{ t(rarityKey(s.rarity)) }}</span>
          </div>
          <p class="skin-desc">{{ bi(s.desc) }}</p>

          <div class="foot">
            <span v-if="s.price === 0" class="chip">{{ t('skinDefault') }}</span>
            <span v-else-if="!isOwned(s)" class="chip" :class="{ poor: signedIn && !canAfford(s) }">
              <Icon name="coin" :size="18" />
              <b class="num">{{ fmt(s.price) }}</b>
            </span>
            <span v-else class="chip">{{ t('owned') }}</span>

            <button
              v-if="isEquipped(s)"
              class="btn equipped-btn"
              disabled
            >
              <Icon name="check" :size="15" />{{ t('equipped') }}
            </button>
            <button
              v-else-if="isOwned(s)"
              class="btn equip-btn"
              :disabled="!signedIn || busyId === s.id"
              @click="equip(s)"
            >
              {{ t('equip') }}
            </button>
            <button
              v-else
              class="btn buy-btn"
              :disabled="!signedIn || busyId === s.id || !canAfford(s)"
              @click="askBuy(s)"
            >
              <Icon name="coin" :size="20" />{{ t('buy') }}
            </button>
          </div>
          </div>
        </article>
    </div>
    </GuestVeil>
    </div>

    <!-- purchase confirmation: skin, price, and the balance that remains -->
    <div v-if="confirming" class="overlay">
      <div class="panel confirm" :class="'r-' + confirming.rarity" role="dialog" :aria-label="t('confirmBuy')">
        <span class="confirm-title">{{ t('confirmBuy') }}</span>
        <div class="confirm-target">
          <div class="preview" :class="'skin-' + confirming.id">
            <span class="pcard">
              <i class="holo" aria-hidden="true" />
              <i class="pframe" aria-hidden="true" />
              <span class="idx tl"><b class="num">7</b><svg class="pip" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="HEART" /></svg></span>
              <svg class="watermark" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="HEART" /></svg>
              <b class="rank num">7</b>
              <span class="idx br"><b class="num">7</b><svg class="pip" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path :d="HEART" /></svg></span>
            </span>
          </div>
          <div class="confirm-meta">
            <b class="name">{{ confirming.name }}</b>
            <span class="rarity" :class="'r-' + confirming.rarity">{{ t(rarityKey(confirming.rarity)) }}</span>
            <span class="price-row">
              <Icon name="coin" :size="22" />
              <b class="num">{{ fmt(confirming.price) }}</b>
            </span>
          </div>
        </div>
        <div class="after">
          <span>{{ t('balanceAfter') }}</span>
          <span class="after-val num">
            {{ fmt(balance) }} <Icon class="arrow" name="arrow-right" :size="14" />
            <b :class="{ zero: balance - confirming.price === 0 }">{{ fmt(balance - confirming.price) }}</b>
          </span>
        </div>
        <p v-if="actionError" class="err">{{ actionError }}</p>
        <div class="confirm-actions">
          <button class="btn" :disabled="busyId === confirming.id" @click="closeConfirm">{{ t('cancel') }}</button>
          <button class="btn buy-btn confirm-buy" :disabled="busyId === confirming.id" @click="buy(confirming)">
            <Icon name="coin" :size="20" />{{ t('buy') }}
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

/* ---------- guest gate ---------- */
/* guests browse behind GuestVeil's blur; nothing gate-specific left here */

/* ---------- shop grid ---------- */
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 14px;
}
/* rarity palette shared with the item tabs. The tier dials scale the aura
   with prestige: --r-line tints the card border, --r-wash the corner wash,
   --r-haze the resting outer glow. */
.r-common { --rc: var(--rarity-common); --r-line: 22%; --r-wash: 12%; --r-haze: 8%; }
.r-rare { --rc: var(--info); --r-line: 48%; --r-wash: 22%; --r-haze: 16%; }
.r-epic { --rc: #b283f0; --r-line: 58%; --r-wash: 28%; --r-haze: 20%; }
.r-legend { --rc: var(--accent); --r-line: 66%; --r-wash: 32%; --r-haze: 24%; }

.skin-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  border: 1px solid var(--line);
  /* the rarity reads before anything else: the border drinks --rc and the
     card sits in a soft halo of its tier colour */
  border-color: color-mix(in srgb, var(--rc) var(--r-line), var(--line));
  box-shadow: 0 4px 20px color-mix(in srgb, var(--rc) var(--r-haze), transparent);
  /* translucent so the atelier cards and glitter behind shimmer through */
  background: linear-gradient(180deg,
    color-mix(in srgb, var(--surface-2) 78%, transparent),
    color-mix(in srgb, var(--surface-2) 94%, transparent));
  border-radius: var(--r-md);
  padding: 14px 16px;
  overflow: hidden;
}
/* rarity wash bleeding in from the preview edge */
.skin-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(46% 90% at 0% 50%, color-mix(in srgb, var(--rc) calc(var(--r-wash) * 1.5), transparent), transparent 74%),
    linear-gradient(102deg, color-mix(in srgb, var(--rc) var(--r-wash), transparent), transparent 52%);
  pointer-events: none;
}
/* legend is the only tier that breathes — the halo slowly pulses */
@media (prefers-reduced-motion: no-preference) {
  .skin-card.r-legend { animation: rarity-breathe 3.6s ease-in-out infinite; }
}
@keyframes rarity-breathe {
  0%, 100% { box-shadow: 0 4px 18px color-mix(in srgb, var(--rc) calc(var(--r-haze) * 0.6), transparent); }
  50% { box-shadow: 0 6px 26px color-mix(in srgb, var(--rc) calc(var(--r-haze) * 2), transparent); }
}
.skin-card.equipped {
  border-color: rgba(246, 183, 60, 0.45);
  box-shadow: 0 0 18px rgba(246, 183, 60, 0.1);
}

/* the live preview: a themed vignette tile (a CSS-only hint of the in-game
   WebGL scene) holding a miniature built from the same --skin-* vars, holo
   foil and signature effects as the real CardTile */
.preview {
  flex: none;
  position: relative;
  display: grid;
  place-items: center;
  width: 96px;
  height: 130px;
  border-radius: 10px;
  /* the rarity rings the preview: tier-tinted frame plus a soft halo */
  border: 1px solid color-mix(in srgb, var(--rc, var(--line)) 45%, var(--line));
  box-shadow: 0 0 18px color-mix(in srgb, var(--rc, transparent) 20%, transparent);
  overflow: hidden;
  background: linear-gradient(160deg, var(--surface-3), var(--surface-2));
}
.pcard {
  --pw: 62px;
  position: relative;
  width: var(--pw);
  height: calc(var(--pw) * 1.45);
  border-radius: calc(var(--pw) * 0.1);
  border: 1px solid var(--skin-border, #cdd3e3);
  background: var(--skin-face, linear-gradient(163deg, #fff 0%, #fafbfe 46%, #eef0f7 100%));
  color: var(--skin-red, var(--card-red));
  display: grid;
  place-items: center;
  position: relative;
  overflow: hidden;
  box-shadow: 0 1px 1px rgba(0, 0, 0, 0.3), 0 8px 18px rgba(0, 0, 0, 0.3);
  transition: transform 0.22s var(--ease);
}
@media (prefers-reduced-motion: no-preference) {
  .preview:hover .pcard { transform: translateY(-5px) rotate(-2.5deg); }
}
.pcard .rank {
  position: relative;
  font-size: calc(var(--pw) * 0.46);
  font-weight: 600;
  line-height: 1;
  letter-spacing: -0.035em;
}
.pcard .idx {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
  line-height: 1;
}
.pcard .idx b {
  font-size: calc(var(--pw) * 0.2);
  font-weight: 600;
  letter-spacing: -0.02em;
}
.pcard .idx .pip { width: calc(var(--pw) * 0.13); height: calc(var(--pw) * 0.13); }
.pcard .idx.tl { top: calc(var(--pw) * 0.065); left: calc(var(--pw) * 0.065); }
.pcard .idx.br { bottom: calc(var(--pw) * 0.065); right: calc(var(--pw) * 0.065); transform: rotate(180deg); }
.pcard .watermark {
  position: absolute;
  width: calc(var(--pw) * 0.62);
  height: calc(var(--pw) * 0.62);
  opacity: 0.06;
  pointer-events: none;
}
.pframe {
  position: absolute;
  inset: calc(var(--pw) * 0.055);
  border: 1px solid currentColor;
  border-radius: calc(var(--pw) * 0.055);
  opacity: 0.13;
  pointer-events: none;
}

/* ---------- per-skin vignette backdrops (CSS hints of the WebGL scenes) ---------- */
.skin-midnight.preview {
  background:
    radial-gradient(2px 2px at 24% 22%, rgba(255, 255, 255, 0.9) 50%, transparent 51%),
    radial-gradient(1.5px 1.5px at 70% 34%, rgba(255, 255, 255, 0.7) 50%, transparent 51%),
    radial-gradient(2px 2px at 82% 76%, rgba(205, 222, 255, 0.8) 50%, transparent 51%),
    radial-gradient(1.5px 1.5px at 40% 84%, rgba(255, 255, 255, 0.6) 50%, transparent 51%),
    radial-gradient(26px 26px at 78% 16%, rgba(143, 178, 245, 0.35), transparent 70%),
    linear-gradient(160deg, #1a2340, #101830);
}
.skin-sakura.preview {
  background:
    radial-gradient(14px 10px at 20% 82%, rgba(239, 143, 180, 0.5), transparent 70%),
    radial-gradient(10px 8px at 78% 70%, rgba(243, 169, 198, 0.45), transparent 70%),
    radial-gradient(12px 9px at 66% 14%, rgba(239, 143, 180, 0.35), transparent 70%),
    linear-gradient(160deg, #4a2c3c, #2e1c28);
}
.skin-sunset.preview {
  background:
    radial-gradient(22px 15px at 50% 102%, rgba(255, 209, 102, 0.8), transparent 70%),
    linear-gradient(160deg, #472a3f 0%, #6e3a44 55%, #b4583f 100%);
}
.skin-ocean.preview {
  background:
    linear-gradient(115deg, transparent 30%, rgba(127, 220, 255, 0.14) 46%, transparent 60%),
    linear-gradient(115deg, transparent 55%, rgba(127, 220, 255, 0.1) 70%, transparent 84%),
    linear-gradient(160deg, #12354f, #0b2438);
}
.skin-forest.preview {
  background:
    radial-gradient(2px 2px at 30% 70%, rgba(255, 233, 163, 0.9) 40%, transparent 60%),
    radial-gradient(1.5px 1.5px at 70% 30%, rgba(255, 243, 196, 0.8) 40%, transparent 60%),
    radial-gradient(30px 22px at 70% 82%, rgba(127, 155, 94, 0.3), transparent 70%),
    linear-gradient(160deg, #24371f, #182412);
}
.skin-neon.preview {
  background:
    repeating-linear-gradient(0deg, rgba(61, 255, 143, 0.08) 0 1px, transparent 1px 7px),
    radial-gradient(40px 26px at 50% 112%, rgba(255, 61, 143, 0.3), transparent 70%),
    linear-gradient(160deg, #10181a, #0a0f11);
}
.skin-royal.preview {
  background:
    radial-gradient(34px 24px at 30% 20%, rgba(201, 167, 255, 0.22), transparent 70%),
    radial-gradient(24px 18px at 75% 75%, rgba(243, 217, 139, 0.16), transparent 70%),
    linear-gradient(160deg, #2b1d52, #1c1236);
}
.skin-gold.preview {
  background:
    linear-gradient(125deg, transparent 40%, rgba(255, 224, 138, 0.22) 50%, transparent 60%),
    linear-gradient(160deg, #3a2f1a, #241d10);
}
.skin-galaxy.preview {
  background:
    radial-gradient(2px 2px at 22% 26%, rgba(255, 255, 255, 0.95) 50%, transparent 51%),
    radial-gradient(1.5px 1.5px at 68% 16%, rgba(214, 205, 255, 0.8) 50%, transparent 51%),
    radial-gradient(2px 2px at 80% 62%, rgba(255, 255, 255, 0.7) 50%, transparent 51%),
    radial-gradient(1.5px 1.5px at 36% 78%, rgba(226, 214, 255, 0.7) 50%, transparent 51%),
    radial-gradient(30px 22px at 72% 72%, rgba(239, 159, 216, 0.2), transparent 70%),
    radial-gradient(36px 26px at 24% 30%, rgba(106, 90, 205, 0.35), transparent 70%),
    linear-gradient(160deg, #1d1540, #120d2b);
}
.skin-inferno.preview {
  background:
    radial-gradient(30px 20px at 50% 104%, rgba(255, 122, 51, 0.5), transparent 72%),
    radial-gradient(1.5px 1.5px at 30% 60%, rgba(255, 179, 71, 0.8) 40%, transparent 60%),
    radial-gradient(1.5px 1.5px at 70% 40%, rgba(255, 122, 51, 0.7) 40%, transparent 60%),
    linear-gradient(160deg, #241410, #120a07);
}

/* ---------- text + actions ---------- */
.info { flex: 1; min-width: 0; align-self: stretch; display: flex; flex-direction: column; gap: 5px; }
.skin-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.skin-name { font-size: 0.98rem; font-weight: 600; letter-spacing: -0.01em; }
.rarity {
  flex: none;
  font-size: 0.64rem;
  font-weight: 700;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  border-radius: var(--r-full);
  padding: 3px 8px;
  line-height: 1;
  color: var(--rc, var(--text-dim));
  border: 1px solid color-mix(in srgb, var(--rc, var(--text-dim)) 40%, transparent);
  background: color-mix(in srgb, var(--rc, var(--text-dim)) 10%, transparent);
}
/* the top tiers badge their own light */
.rarity.r-epic, .rarity.r-legend {
  box-shadow: 0 0 12px color-mix(in srgb, var(--rc) 35%, transparent);
}

.skin-desc {
  font-size: 0.78rem;
  color: var(--text-mute);
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
/* Price above a full-width action button: the info column is narrower than
   chip + button side by side, which pushed the button past the card edge. */
.foot {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 7px;
  margin-top: auto;
  padding-top: 9px;
}
.foot .chip { align-self: flex-start; padding: 4px 10px; font-size: 0.75rem; }
.foot .chip b { color: var(--text); }
.foot .chip.poor { color: var(--bad); border-color: rgba(239, 95, 95, 0.35); }
.foot .chip.poor b { color: var(--bad); }
.foot .btn { width: 100%; justify-content: center; min-height: 34px; padding: 7px 12px; font-size: 0.85rem; }
.equipped-btn { --btn-fg: var(--good); }
/* owned cards get a quiet outlined action so the filled-gold buy CTA stays
   the one loud element on every card */
.foot .btn.equip-btn {
  --btn-bg: var(--accent-soft);
  --btn-line: rgba(246, 183, 60, 0.5);
  --btn-fg: var(--accent);
  font-weight: 600;
}
@media (hover: hover) {
  .foot .btn.equip-btn:hover:not(:disabled) {
    --btn-bg: rgba(246, 183, 60, 0.22);
    --btn-line: rgba(246, 183, 60, 0.75);
  }
}
/* the buy CTA: the only gradient-gold, glowing button on the card */
.foot .btn.buy-btn {
  --btn-bg: linear-gradient(180deg, var(--accent-hi), var(--accent));
  --btn-line: var(--accent);
  --btn-fg: var(--accent-ink);
  font-weight: 700;
  transition: background 0.15s var(--ease), border-color 0.15s var(--ease),
    transform 0.1s var(--ease), opacity 0.15s var(--ease), box-shadow 0.15s var(--ease);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4), 0 3px 14px rgba(246, 183, 60, 0.35);
}
@media (hover: hover) {
  .foot .btn.buy-btn:hover:not(:disabled) {
    --btn-bg: linear-gradient(180deg, #ffe08f, var(--accent-hi));
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.4), 0 5px 20px rgba(246, 183, 60, 0.55);
  }
}

/* ---------- purchase confirmation ---------- */
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
  gap: 16px;
  width: min(360px, 100%);
  padding: 24px;
  animation: rise-in 0.26s var(--ease);
}
.confirm-title { font-size: 1.2rem; font-weight: 600; letter-spacing: -0.01em; }
.confirm-target { display: flex; align-items: center; gap: 16px; }
.confirm-meta { display: flex; flex-direction: column; align-items: flex-start; gap: 7px; min-width: 0; }
.confirm-meta .name { font-size: 1.05rem; font-weight: 600; letter-spacing: -0.01em; }
.price-row {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--accent);
  margin-top: 2px;
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

/* the skin card glows once the payment lands */
.skin-card.bought {
  border-color: rgba(126, 217, 87, 0.55);
  animation: bought-glow 1.8s var(--ease);
}
@keyframes bought-glow {
  0% { box-shadow: 0 0 0 rgba(126, 217, 87, 0); }
  25% { box-shadow: 0 0 26px rgba(126, 217, 87, 0.35); }
  100% { box-shadow: 0 0 0 rgba(126, 217, 87, 0); }
}

@media (max-width: 560px) {
  .body { padding: 20px 16px 24px; }
  .grid { grid-template-columns: 1fr; }
}
</style>
