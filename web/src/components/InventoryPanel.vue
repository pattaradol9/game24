<script setup>
// Inventory tab of the unified shop: the item stacks the player holds with a
// Use button per kind, the live countdown of an already-running window, and
// the additive-stacking note so it's clear what using another item will pay.
// The player JSON carries the same inventory — this panel just makes it
// actionable. Skins equip from their own tab.
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from '../i18n/index.js'
import { api } from '../api.js'
import { currentPlayer, getToken, updatePlayer } from '../auth.js'
import { fmtDuration } from '../duration.js'
import Icon from './Icon.vue'
import GuestVeil from './GuestVeil.vue'
import FxBackdrop from './FxBackdrop.vue'

const { t, lang } = useI18n()

const loading = ref(true)
const error = ref('')
const catalog = ref([])
const owned = ref({})
const active = ref({}) // item id → endsAt RFC3339 while its window runs
const busyId = ref('')
const actionError = ref('')
const confirming = ref(null)
const replacing = ref([]) // same-kind live windows this use will overwrite
const extending = ref(null) // same-kind + same-multiplier window this use adds time to
const count = ref(1) // units the slider commits to spending
const maxCount = ref(1) // most units the 24h cap (and the bag) still allow
const justUsed = ref('')
let justUsedTimer = 0

const player = computed(() => currentPlayer.value)
const signedIn = computed(() => !!player.value && !player.value.isGuest)
const bi = (obj) => obj?.[lang.value] ?? obj?.en ?? ''
const fmt = (n) => (n ?? 0).toLocaleString('en-US')
const ICONS = { exp: 'bolt', coins: 'coin', time: 'hourglass', skip: 'skip' }
const KIND_KEYS = { exp: 'itemKindExp', coins: 'itemKindCoins', time: 'itemKindTime', skip: 'itemKindSkip' }

// play helpers (time + skip) never arm a window — the game spends them
const inGame = (it) => it.kind === 'time' || it.kind === 'skip'
const inGameIcon = (it) => (it.kind === 'time' ? 'hourglass' : 'skip')
const inGameKey = (it) => (it.kind === 'time' ? 'itemInGameOnly' : 'itemSkipInGameOnly')

// stacks the player actually holds, in catalog order
const stacks = computed(() => catalog.value.filter((it) => (owned.value[it.id] ?? 0) > 0))

const nowMs = ref(Date.now())
let ticker = 0
onMounted(() => {
  ticker = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})
onUnmounted(() => {
  clearInterval(ticker)
  clearTimeout(justUsedTimer)
})

// seconds left on an item's window, or 0 when it is not the active one
const leftOf = (id) => {
  const end = Date.parse(active.value[id] ?? '')
  if (!Number.isFinite(end)) return 0
  return Math.max(0, Math.floor((end - nowMs.value) / 1000))
}
const leftLabel = (id) => fmtDuration(leftOf(id))

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

// same-kind boosts follow three rules: same multiplier stacks TIME on top of
// the live window, different multiplier REPLACES the live window, other kinds
// are untouched and always run alongside. The window may never run past 24h
// from now, and that cap is what bounds the slider: every unit of the item
// buys one more duration, so the slider maxes at how many durations still fit
// under the cap — rounded UP (7.5 fits → 8), the overhang trimmed server-side.
const CAP_MS = 24 * 3600 * 1000

function askUse(it) {
  if (inGame(it)) return // play helpers spend themselves mid-game
  actionError.value = ''
  replacing.value = []
  extending.value = null
  const live = catalog.value
    .filter((x) => x.kind === it.kind && Date.parse(active.value[x.id] ?? '') > nowMs.value)
    .map((x) => ({ ...x, ends: Date.parse(active.value[x.id]) }))
  for (const x of live) {
    if (x.multiplier === it.multiplier) {
      if (!extending.value || x.ends > extending.value.ends) {
        extending.value = { id: x.id, name: bi(x.name), left: leftLabel(x.id), ends: x.ends }
      }
    } else {
      replacing.value.push({ id: x.id, name: bi(x.name), left: leftLabel(x.id), self: x.id === it.id })
    }
  }
  const durMs = it.durationMinutes * 60000
  let byCap = Math.floor(CAP_MS / durMs) // fresh/replace windows start from now
  if (extending.value) {
    const room = CAP_MS - (extending.value.ends - nowMs.value)
    byCap = room > 0 ? Math.ceil(room / durMs) : 0 // already at the cap → nothing fits
  }
  maxCount.value = Math.max(0, Math.min(owned.value[it.id] ?? 0, byCap))
  count.value = 1
  confirming.value = it
}

// window the chosen count lands: the live end (if extending) plus one
// duration per unit, trimmed at the cap
function projectedMs() {
  if (!confirming.value) return 0
  const durMs = confirming.value.durationMinutes * 60000
  const base = extending.value ? Math.max(0, extending.value.ends - nowMs.value) : 0
  return base + count.value * durMs
}
const resultMs = computed(() => Math.min(projectedMs(), CAP_MS))
const capHit = computed(() => confirming.value && projectedMs() > CAP_MS)
// a stack already at the cap has room for zero units — the use is refused
const overCap = computed(() => !!confirming.value && maxCount.value === 0)

function closeConfirm() {
  if (busyId.value) return
  confirming.value = null
  replacing.value = []
  extending.value = null
  actionError.value = ''
}

async function use(it) {
  if (busyId.value || overCap.value) return
  const n = Math.min(count.value, maxCount.value)
  busyId.value = it.id
  actionError.value = ''
  try {
    const data = await api.useItem(it.id, getToken(), n)
    updatePlayer(data.player)
    owned.value = { ...owned.value, [it.id]: Math.max(0, (owned.value[it.id] ?? 0) - n) }
    // the kind holds one window: drop same-kind tiers that were replaced and
    // point every same-multiplier item at the fresh end
    const nextActive = {}
    for (const [id, ends] of Object.entries(active.value)) {
      const c = catalog.value.find((x) => x.id === id)
      if (c && c.kind === it.kind && c.multiplier !== it.multiplier) continue
      nextActive[id] = ends
    }
    if (data.boost?.endsAt) {
      for (const x of catalog.value) {
        if (x.kind === it.kind && x.multiplier === it.multiplier) nextActive[x.id] = data.boost.endsAt
      }
    }
    active.value = nextActive
    confirming.value = null
    replacing.value = []
    extending.value = null
    justUsed.value = it.id
    clearTimeout(justUsedTimer)
    justUsedTimer = setTimeout(() => { if (justUsed.value === it.id) justUsed.value = '' }, 2200)
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
    <FxBackdrop variant="vault" />
    <div class="content">
      <div class="head">
        <h1>{{ t('shopTabInventory') }}</h1>
        <span class="hint">{{ t('stackNote') }}</span>
      </div>

      <p v-if="error" class="err">{{ error }}</p>

      <p v-if="signedIn && actionError" class="err">{{ actionError }}</p>

      <!-- guests see the (always empty) bag behind a blurred veil until sign-in -->
      <GuestVeil :message="t('shopSignInRequired')" @signed-in="onSignedIn">
        <div v-if="loading" class="loading">…</div>
        <div v-else-if="!stacks.length" class="empty">
          <Icon name="bag" :size="26" />
          <p>{{ t('inventoryEmpty') }}</p>
        </div>
        <div v-else class="list">
        <article
          v-for="it in stacks"
          :key="it.id"
          class="row"
          :class="[{ used: justUsed === it.id }, 'r-' + it.rarity]"
        >
          <span class="tile">
            <Icon class="glyph" :name="ICONS[it.kind] ?? 'bolt'" :size="44" />
            <Icon class="ic" :name="ICONS[it.kind] ?? 'bolt'" :size="24" />
          </span>
          <div class="info">
            <div class="top">
              <b class="name">{{ bi(it.name) }}</b>
              <span class="qty num">×{{ fmt(owned[it.id]) }}</span>
            </div>
            <p class="desc">{{ bi(it.desc) }}</p>
          </div>
          <div class="side">
            <span v-if="leftOf(it.id) > 0" class="chip active-chip">
              <Icon name="clock" :size="14" />
              <b class="num">{{ leftLabel(it.id) }}</b>
            </span>
            <!-- play helpers have no window to arm: time items spend themselves
                 when a solo hand's clock hits zero, skip items when the Skip
                 button is pressed — the row explains instead of offering Use -->
            <span v-if="inGame(it)" class="ingame-note">
              <Icon :name="inGameIcon(it)" :size="14" />{{ t(inGameKey(it)) }}
            </span>
            <button v-else class="btn use-btn" :disabled="busyId === it.id" @click="askUse(it)">
              <Icon name="play" :size="15" />{{ t('itemUse') }}
            </button>
          </div>
        </article>
        </div>
      </GuestVeil>
    </div>

    <!-- use confirmation: what it arms and the window-extension rule -->
    <div v-if="confirming" class="overlay">
      <div class="panel confirm" :class="'r-' + confirming.rarity" role="dialog" :aria-label="t('confirmUse')">
        <span class="confirm-title">{{ t('confirmUse') }}</span>
        <div class="confirm-target">
          <span class="tile big">
            <Icon class="glyph" :name="ICONS[confirming.kind] ?? 'bolt'" :size="52" />
            <Icon class="ic" :name="ICONS[confirming.kind] ?? 'bolt'" :size="30" />
          </span>
          <div class="confirm-meta">
            <b class="name">{{ bi(confirming.name) }}</b>
            <span class="mult num">×{{ confirming.multiplier }} · {{ fmtDuration(confirming.durationMinutes * 60) }}</span>
            <span class="inbag num">{{ t('itemInBag', { n: fmt(owned[confirming.id] ?? 0) }) }}</span>
          </div>
        </div>
        <p class="note">{{ t('useItemNote') }}</p>
        <!-- how many units to spend: maxed by what still fits under the 24h cap -->
        <div v-if="maxCount > 1" class="countrow">
          <div class="counthead">
            <label for="use-count">{{ t('useCount') }}</label>
            <b class="countval num">×{{ count }}</b>
          </div>
          <input id="use-count" v-model.number="count" type="range" min="1" :max="maxCount" />
          <span class="counthint">{{ t('itemMaxUsable', { n: fmt(maxCount) }) }}</span>
        </div>
        <div v-if="overCap" class="overcap">
          <div class="warnhead">
            <Icon name="flame" :size="15" />
            <b>{{ t('itemOverCap') }}</b>
          </div>
        </div>
        <div v-else-if="extending" class="extend">
          <div class="extendhead">
            <Icon name="clock" :size="15" />
            <b>{{ t('itemExtendInfo', { n: confirming.multiplier, left: extending.left, dur: fmtDuration(confirming.durationMinutes * 60), count, total: fmtDuration(resultMs / 1000) }) }}</b>
          </div>
          <span v-if="capHit" class="caphit">{{ t('itemCapHit') }}</span>
        </div>
        <div v-else class="durbox">
          <div class="extendhead">
            <Icon name="clock" :size="15" />
            <b>{{ t('itemDurationInfo', { n: confirming.multiplier, total: fmtDuration(resultMs / 1000) }) }}</b>
          </div>
          <span v-if="capHit" class="caphit">{{ t('itemCapHit') }}</span>
        </div>
        <div v-if="replacing.length" class="warn">
          <div class="warnhead">
            <Icon name="flame" :size="15" />
            <b>{{ t('itemReplaceWarn', { kind: t(KIND_KEYS[confirming.kind] ?? 'itemKindExp') }) }}</b>
          </div>
          <span v-for="r in replacing" :key="r.id" class="warnrow">
            {{ r.self ? t('itemReplaceSelf') : r.name }} · {{ r.left }}
          </span>
        </div>
        <p v-if="actionError" class="err">{{ actionError }}</p>
        <div class="confirm-actions">
          <button class="btn" :disabled="busyId === confirming.id" @click="closeConfirm">{{ t('cancel') }}</button>
          <button class="btn use-btn confirm-use" :disabled="busyId === confirming.id || overCap" @click="use(confirming)">
            <Icon name="play" :size="16" />{{ t('itemUseN', { n: count }) }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.body { position: relative; overflow: clip; padding: 26px 28px 30px; }
/* the WebGL ambience lives on .body (z 0); the content rides above it.
   The floor keeps the tab from collapsing when the bag is empty (guests,
   fresh accounts) so switching tabs never makes the page jump shorter */
.content { position: relative; z-index: 1; display: flex; flex-direction: column; gap: 18px; min-height: 480px; }
.head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
h1 { font-size: 1.4rem; font-weight: 600; letter-spacing: -0.01em; }
.hint { font-size: 0.8rem; color: var(--text-mute); }
.err { color: var(--bad); font-size: 0.85rem; }
.loading { text-align: center; color: var(--text-mute); font-size: 1.4rem; padding: 60px 0; }

/* guests browse behind GuestVeil's blur; nothing gate-specific left here */

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  color: var(--text-mute);
  border: 1px dashed var(--line);
  border-radius: var(--r-md);
  padding: 42px 18px;
  text-align: center;
}
.empty p { font-size: 0.88rem; max-width: 36ch; line-height: 1.5; }

/* --rc defaults to the common tier here; each row's r-* class retints it
   for everything inside, tile included */
.list {
  --rc: var(--rarity-common);
  display: flex;
  flex-direction: column;
  gap: 12px;
}
/* rarity palette shared with the shop cards' tile treatment. The tier dials
   scale the aura with prestige: --r-line tints the row border, --r-wash the
   corner wash, --r-haze the resting outer glow. */
.r-common { --rc: var(--rarity-common); --r-line: 22%; --r-wash: 12%; --r-haze: 8%; }
.r-rare { --rc: var(--info); --r-line: 48%; --r-wash: 22%; --r-haze: 16%; }
.r-epic { --rc: #b283f0; --r-line: 58%; --r-wash: 28%; --r-haze: 20%; }
.r-legend { --rc: var(--accent); --r-line: 66%; --r-wash: 32%; --r-haze: 24%; }

.row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  border: 1px solid var(--line);
  /* the rarity reads before anything else: the border drinks --rc and the
     row sits in a soft halo of its tier colour */
  border-color: color-mix(in srgb, var(--rc) var(--r-line), var(--line));
  box-shadow: 0 4px 20px color-mix(in srgb, var(--rc) var(--r-haze), transparent);
  background: linear-gradient(180deg,
    color-mix(in srgb, var(--surface-2) 74%, transparent),
    color-mix(in srgb, var(--surface-2) 92%, transparent));
  border-radius: var(--r-md);
  padding: 13px 16px;
  overflow: hidden;
  transition: border-color 0.18s var(--ease), box-shadow 0.18s var(--ease);
}
.row::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(46% 90% at 0% 50%, color-mix(in srgb, var(--rc) calc(var(--r-wash) * 1.5), transparent), transparent 74%),
    linear-gradient(102deg, color-mix(in srgb, var(--rc) var(--r-wash), transparent), transparent 52%);
  pointer-events: none;
}
@media (hover: hover) {
  .row:hover { border-color: color-mix(in srgb, var(--rc) 70%, var(--line)); }
}
/* legend is the only tier that breathes — the halo slowly pulses */
@media (prefers-reduced-motion: no-preference) {
  .row.r-legend { animation: rarity-breathe 3.6s ease-in-out infinite; }
}
@keyframes rarity-breathe {
  0%, 100% { box-shadow: 0 4px 18px color-mix(in srgb, var(--rc) calc(var(--r-haze) * 0.6), transparent); }
  50% { box-shadow: 0 6px 26px color-mix(in srgb, var(--rc) calc(var(--r-haze) * 2), transparent); }
}
.row.used {
  border-color: rgba(126, 217, 87, 0.55);
  animation: used-glow 1.8s var(--ease);
}
@keyframes used-glow {
  0% { box-shadow: 0 0 0 rgba(126, 217, 87, 0); }
  25% { box-shadow: 0 0 26px rgba(126, 217, 87, 0.35); }
  100% { box-shadow: 0 0 0 rgba(126, 217, 87, 0); }
}

.tile {
  position: relative;
  flex: none;
  display: grid;
  place-items: center;
  width: 64px;
  height: 86px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--rc) 52%, transparent);
  background:
    radial-gradient(64% 52% at 50% 36%, color-mix(in srgb, var(--rc) 38%, transparent), transparent 74%),
    linear-gradient(168deg, color-mix(in srgb, var(--rc) 18%, transparent), color-mix(in srgb, var(--rc) 5%, transparent) 70%),
    var(--surface-3);
  overflow: hidden;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 0 12px color-mix(in srgb, var(--rc) 10%, transparent),
    0 5px 14px color-mix(in srgb, var(--rc) 8%, transparent);
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
  filter: drop-shadow(0 0 6px color-mix(in srgb, var(--rc) 32%, transparent));
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
    opacity: 0.5;
  }
}
@keyframes item-bob {
  0%, 100% { transform: translateY(2px); }
  50% { transform: translateY(-3px); }
}
@keyframes item-turn {
  to { transform: rotate(1turn); }
}
.tile.big { width: 84px; height: 84px; border-radius: 14px; }

.info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.top { display: flex; align-items: center; gap: 10px; }
.name { font-size: 0.95rem; font-weight: 600; letter-spacing: -0.01em; }
.qty { font-size: 0.85rem; font-weight: 700; color: var(--text-dim); }
.desc {
  font-size: 0.76rem;
  color: var(--text-mute);
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.side { flex: none; display: flex; flex-direction: column; align-items: stretch; gap: 7px; }
.active-chip { align-self: center; display: inline-flex; align-items: center; gap: 5px; padding: 3px 9px; font-size: 0.72rem; color: var(--good); border-color: rgba(126, 217, 87, 0.4); }
.active-chip b { color: var(--good); }
/* time items: no Use button — the row explains where they spend themselves */
.ingame-note {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  padding: 4px 10px;
  font-size: 0.72rem;
  color: var(--text-mute);
  border: 1px dashed var(--line);
  border-radius: var(--r-sm);
  text-align: left;
  max-width: 150px;
  line-height: 1.4;
}
.use-btn {
  min-width: 96px;
  justify-content: center;
  min-height: 34px;
  padding: 7px 12px;
  font-size: 0.85rem;
  --btn-bg: var(--accent-soft);
  --btn-line: rgba(246, 183, 60, 0.5);
  --btn-fg: var(--accent);
  font-weight: 600;
}
@media (hover: hover) {
  .use-btn:hover:not(:disabled) {
    --btn-bg: rgba(246, 183, 60, 0.22);
    --btn-line: rgba(246, 183, 60, 0.75);
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
.confirm-meta .inbag { font-size: 0.78rem; color: var(--good); font-weight: 600; }
.note {
  font-size: 0.78rem;
  color: var(--text-mute);
  line-height: 1.5;
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 10px 13px;
  background: var(--bg);
}
/* the replace warning: same-kind boosts of a different tier swap, never stack */
.warn {
  display: flex;
  flex-direction: column;
  gap: 6px;
  border: 1px solid rgba(239, 95, 95, 0.4);
  border-radius: var(--r-sm);
  padding: 11px 13px;
  background: linear-gradient(color-mix(in srgb, var(--bad) 8%, transparent), color-mix(in srgb, var(--bad) 8%, transparent)), var(--bg);
}
.warnhead {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 0.78rem;
  color: var(--bad);
  line-height: 1.45;
}
.warnhead b { font-weight: 600; }
.warnrow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.72rem;
  color: var(--text-dim);
}
.warnrow + .warnrow { margin-top: 2px; }
/* the friendly flavour of stacking: same tier adds time */
.extend {
  border: 1px solid rgba(126, 217, 87, 0.4);
  border-radius: var(--r-sm);
  padding: 11px 13px;
  background: linear-gradient(color-mix(in srgb, var(--good) 9%, transparent), color-mix(in srgb, var(--good) 9%, transparent)), var(--bg);
}
.extendhead {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 0.78rem;
  color: var(--good);
  line-height: 1.45;
}
.extendhead b { font-weight: 600; }
/* neutral flavour of a fresh/replace window: ×N for the chosen span */
.durbox {
  border: 1px solid var(--line);
  border-radius: var(--r-sm);
  padding: 11px 13px;
  background: var(--bg);
}
.durbox .extendhead { color: var(--text-dim); }
/* the trimmed-at-the-cap footnote under a preview that ran past 24h */
.caphit { display: block; margin-top: 5px; font-size: 0.7rem; color: var(--accent); }
/* units-to-spend slider, maxed by the 24h cap */
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
/* past the 24h cap: the use is refused outright */
.overcap {
  border: 1px solid rgba(239, 95, 95, 0.55);
  border-radius: var(--r-sm);
  padding: 11px 13px;
  background: linear-gradient(color-mix(in srgb, var(--bad) 12%, transparent), color-mix(in srgb, var(--bad) 12%, transparent)), var(--bg);
}
.confirm-actions { display: flex; justify-content: flex-end; gap: 10px; }
.confirm-actions .btn { min-height: 40px; padding: 9px 16px; }
.confirm-use { min-width: 110px; }

@media (max-width: 560px) {
  .body { padding: 20px 16px 24px; }
}
</style>
