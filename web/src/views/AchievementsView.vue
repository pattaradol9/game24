<script setup>
// Achievement catalog: every tier from bronze to legend, with live progress
// and unlock dates for the signed-in player (all of it reads as locked and
// untracked for guests — the server simply returns empty maps).
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { api } from '../api.js'
import { getToken } from '../auth.js'
import Brand from '../components/Brand.vue'
import GuestVeil from '../components/GuestVeil.vue'
import Icon from '../components/Icon.vue'
import ProfileMenu from '../components/ProfileMenu.vue'
import SiteFooter from '../components/SiteFooter.vue'

const { t, lang } = useI18n()

const loading = ref(true)
const error = ref('')
const achievements = ref([])
const unlocked = ref({})
const progress = ref({})

const TIER_COLORS = {
  bronze: '#cd7f32',
  silver: '#c0c0c0',
  gold: '#ffd700',
  platinum: '#7de3e1',
  legend: '#b283f0',
}
// display order: common → rarest; the server catalog is metric-grouped, so
// tiers arrive interleaved without this
const TIER_ORDER = ['bronze', 'silver', 'gold', 'platinum', 'legend']
const tierRank = (tier) => {
  const i = TIER_ORDER.indexOf(String(tier || '').toLowerCase())
  return i === -1 ? TIER_ORDER.length : i
}
const fmt = (n) => (n ?? 0).toLocaleString('en-US')
const tierColor = (tier) => TIER_COLORS[String(tier || '').toLowerCase()] ?? TIER_COLORS.bronze
const tierKey = (tier) => 'tier' + String(tier || '').charAt(0).toUpperCase() + String(tier || '').slice(1)
const bi = (obj) => obj?.[lang.value] ?? obj?.en ?? ''
const isUnlocked = (a) => !!unlocked.value[a.id]
const currentOf = (a) => progress.value[a.id]?.current ?? 0
const pctOf = (a) => (a.target > 0 ? Math.min(100, Math.round((currentOf(a) / a.target) * 100)) : 0)

const doneCount = computed(() => Object.keys(unlocked.value).length)

function fmtDate(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleDateString(lang.value === 'th' ? 'th-TH' : 'en-US', {
    year: 'numeric', month: 'short', day: 'numeric',
  })
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.achievements(getToken())
    achievements.value = (data.achievements ?? []).sort((a, b) => tierRank(a.tier) - tierRank(b.tier))
    unlocked.value = data.unlocked ?? {}
    progress.value = data.progress ?? {}
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
onMounted(load)

// signing in from the veil lifts it and pulls the fresh unlocked/progress maps
function onSignedIn() {
  load()
}
</script>

<template>
  <main class="wrap page">
    <header class="top">
      <button class="btn back" @click="$router.push('/')">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <Brand size="sm" />
      <div class="spacer" />
      <ProfileMenu />
    </header>

    <section class="panel body">
      <div class="head">
        <h1>{{ t('achievements') }}</h1>
        <span class="chip accent num">{{ t('unlockedCount', { done: doneCount, total: achievements.length }) }}</span>
      </div>

      <p v-if="error" class="err">{{ error }}</p>

      <!-- guests see the catalog through GuestVeil's blur; signing in
           lifts the veil and reloads the unlocked/progress maps -->
      <GuestVeil :message="t('achSignInRequired')" @signed-in="onSignedIn">
        <div v-if="loading" class="loading">…</div>
        <div v-else class="grid">
          <article
            v-for="a in achievements"
            :key="a.id"
            class="ach"
            :class="{ unlocked: isUnlocked(a) }"
            :style="{ '--tc': tierColor(a.tier) }"
          >
            <div class="ach-head">
              <span class="tier-badge">{{ t(tierKey(a.tier)) }}</span>
              <span v-if="isUnlocked(a)" class="state on">
                <Icon name="check" :size="12" />{{ t('unlocked') }}
              </span>
              <span v-else class="state">
                <Icon name="lock" :size="12" />{{ t('locked') }}
              </span>
            </div>

            <b class="ach-title">{{ bi(a.title) }}</b>
            <p class="ach-desc">{{ bi(a.desc) }}</p>

            <div v-if="isUnlocked(a)" class="when">
              <Icon name="check" :size="12" />
              <span>{{ fmtDate(unlocked[a.id]) }}</span>
            </div>
            <div v-else-if="a.target > 0" class="prog">
              <span class="prog-track"><span class="prog-fill" :style="{ width: pctOf(a) + '%' }" /></span>
              <span class="prog-num num">{{ fmt(currentOf(a)) }} / {{ fmt(a.target) }}</span>
            </div>

          <div class="rewards">
            <span class="chip reward num">+{{ fmt(a.expReward) }} EXP</span>
            <span class="chip reward coin num"><Icon name="coin" :size="16" />+{{ fmt(a.coinReward) }}</span>
          </div>
        </article>
        </div>
      </GuestVeil>
    </section>

    <SiteFooter />
  </main>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 20px; padding: 20px 0 56px; }
.top { display: flex; align-items: center; gap: 12px; }
.spacer { flex: 1; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
@media (hover: hover) { .back:hover { color: var(--text); } }
.body { display: flex; flex-direction: column; gap: 18px; padding: 26px 28px 30px; }
h1 { font-size: 1.4rem; font-weight: 600; letter-spacing: -0.01em; }
.head { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.err { color: var(--bad); font-size: 0.85rem; }
.loading { text-align: center; color: var(--text-mute); font-size: 1.4rem; padding: 60px 0; }

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 14px;
}

/* ---------- one achievement card ---------- */
.ach {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px solid var(--line);
  background: var(--surface-2);
  border-radius: var(--r-md);
  padding: 14px 16px;
}
.ach.unlocked {
  border-color: color-mix(in srgb, var(--tc) 55%, transparent);
  background:
    radial-gradient(120% 90% at 50% -20%, color-mix(in srgb, var(--tc) 16%, transparent), transparent 65%),
    var(--surface-2);
  box-shadow: 0 0 22px color-mix(in srgb, var(--tc) 16%, transparent);
}

.ach-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.tier-badge {
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0.09em;
  text-transform: uppercase;
  color: var(--tc);
  border: 1px solid color-mix(in srgb, var(--tc) 45%, transparent);
  background: color-mix(in srgb, var(--tc) 10%, transparent);
  border-radius: var(--r-full);
  padding: 3px 9px;
  line-height: 1;
}
.state {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.7rem;
  color: var(--text-mute);
}
.state.on { color: var(--good); }

.ach-title { font-size: 1rem; font-weight: 600; letter-spacing: -0.01em; }
.ach-desc { font-size: 0.8rem; color: var(--text-dim); line-height: 1.5; }

/* progress toward the target (hidden when target = 0) */
.prog { display: flex; align-items: center; gap: 10px; }
.prog-track {
  flex: 1;
  height: 5px;
  border-radius: var(--r-full);
  background: var(--surface-3);
  overflow: hidden;
}
.prog-fill {
  display: block;
  height: 100%;
  border-radius: var(--r-full);
  background: linear-gradient(90deg, var(--tc), color-mix(in srgb, var(--tc) 60%, #fff));
  transition: width 0.7s var(--ease);
}
.prog-num { flex: none; font-size: 0.7rem; color: var(--text-mute); }

.when {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 0.72rem;
  color: var(--good);
}

.rewards { display: flex; gap: 8px; flex-wrap: wrap; margin-top: auto; }
.reward {
  padding: 5px 10px;
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-dim);
}
.reward.coin { color: var(--accent); border-color: rgba(246, 183, 60, 0.35); background: var(--accent-soft); }

@media (max-width: 560px) {
  .back-label { display: none; }
  .back { padding: 0; width: 44px; justify-content: center; }
  .body { padding: 20px 16px 24px; }
  .grid { grid-template-columns: 1fr; }
}
</style>
