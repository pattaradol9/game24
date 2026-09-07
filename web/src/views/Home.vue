<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from '../i18n/index.js'
import { getPlayer, initGoogle } from '../auth.js'
import { sfx, toggleSound, soundEnabled } from '../audio.js'
import Brand from '../components/Brand.vue'
import Suit from '../components/Suit.vue'
import Icon from '../components/Icon.vue'
import ModeSelector from '../components/ModeSelector.vue'
import LeaderboardView from '../components/LeaderboardView.vue'
import NicknameModal from '../components/NicknameModal.vue'
import RoomCreateModal from '../components/RoomCreateModal.vue'
import ProfileMenu from '../components/ProfileMenu.vue'
import SiteFooter from '../components/SiteFooter.vue'

const { t, lang, toggle } = useI18n()
const router = useRouter()

const needName = ref(false)
const showCreate = ref(false)
const joinCode = ref('')
const sound = ref(soundEnabled())

// the four numbers in the hero are a worked example, not a live hand
const SAMPLE = [
  { n: '6', suit: 'diamond', red: true },
  { n: '3', suit: 'club', red: false },
  { n: '4', suit: 'heart', red: true },
  { n: '2', suit: 'spade', red: false },
]

onMounted(async () => {
  if (!getPlayer()) needName.value = true
  try {
    await initGoogle()
  } catch { /* offline ok */ }
})

function playSolo(mode) {
  sfx.whoosh()
  router.push({ path: '/solo', query: { mode } })
}

function createRoom() {
  sfx.click()
  showCreate.value = true
}

function joinRoom() {
  const code = (joinCode.value || '').trim().toUpperCase()
  if (code.length !== 6) return
  sfx.whoosh()
  router.push(`/room/${code}`)
}

function onRoomCreated(code, hostKey) {
  showCreate.value = false
  router.push({ path: `/room/${code}`, query: { hostKey } })
}

function flipSound() {
  sound.value = toggleSound()
}
</script>

<template>
  <main class="wrap home">
    <header class="top">
      <Brand size="md" :label="t('appName')" />
      <div class="spacer" />
      <ProfileMenu @require-name="needName = true" />
      <button class="btn icon" :aria-label="sound ? 'mute' : 'unmute'" @click="flipSound">
        <Icon :name="sound ? 'sound-on' : 'sound-off'" />
      </button>
      <button class="btn icon lang" @click="toggle">{{ lang === 'th' ? 'EN' : 'TH' }}</button>
    </header>

    <section class="hero panel">
      <div class="pitch">
        <h1>{{ t('tagline') }}</h1>
        <p class="howto">{{ t('howTo') }}</p>
        <div class="cta">
          <button class="btn primary big" @click="playSolo('queen')">{{ t('playNow') }}</button>
        </div>
      </div>

      <div class="example" aria-hidden="true">
        <div class="hand">
          <span v-for="c in SAMPLE" :key="c.n" class="mini" :class="{ red: c.red }">
            <i class="frame" />
            <Suit :name="c.suit" :size="10" class="pip tl" />
            <b class="num">{{ c.n }}</b>
            <Suit :name="c.suit" :size="10" class="pip br" />
          </span>
        </div>
        <div class="equals">
          <span class="expr">(6 − 4) × 3 × 4</span>
          <span class="result num">24</span>
        </div>
      </div>
    </section>

    <section class="block">
      <h2 class="section-title">{{ t('modes') }}</h2>
      <ModeSelector model-value="" @update:model-value="playSolo" />
    </section>

    <section class="split">
      <div class="block panel">
        <h2 class="section-title">{{ t('playMulti') }}</h2>
        <p class="hint">{{ t('multiplayerBoardHint') }}</p>
        <div class="join">
          <input
            v-model="joinCode"
            class="field code num"
            :placeholder="t('roomCode')"
            maxlength="6"
            @keyup.enter="joinRoom"
          />
          <button class="btn" :disabled="joinCode.trim().length !== 6" @click="joinRoom">
            {{ t('joinRoom') }}
          </button>
        </div>
        <button class="btn block" @click="createRoom">{{ t('createRoom') }}</button>
      </div>

      <div class="block panel">
        <div class="head">
          <h2 class="section-title">{{ t('leaderboard') }}</h2>
          <RouterLink class="more" to="/leaderboard">{{ t('viewAll') }} →</RouterLink>
        </div>
        <LeaderboardView compact />
      </div>
    </section>

    <NicknameModal v-if="needName" @done="needName = false" />
    <RoomCreateModal v-if="showCreate" @created="onRoomCreated" @close="showCreate = false" />

    <SiteFooter />
  </main>
</template>

<style scoped>
.home { display: flex; flex-direction: column; gap: 28px; padding: 20px 0 64px; }

.top { display: flex; align-items: center; gap: 10px; }
.spacer { flex: 1; }
.lang { width: auto; padding-inline: 12px; font-size: 0.85rem; font-weight: 500; }

/* ---------- hero ---------- */
.hero {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr);
  gap: 40px;
  align-items: center;
  padding: 40px 40px 42px;
}
.pitch h1 {
  font-size: clamp(1.9rem, 3.6vw, 2.9rem);
  font-weight: 600;
  line-height: 1.18;
  letter-spacing: -0.02em;
  margin-bottom: 12px;
}
.howto { color: var(--text-dim); font-size: 0.98rem; line-height: 1.6; max-width: 46ch; }
.cta { display: flex; gap: 12px; margin-top: 26px; flex-wrap: wrap; }

/* worked example, so the rules are obvious before you press play */
.example { display: flex; flex-direction: column; align-items: center; gap: 18px; }
.hand { display: flex; gap: 10px; }
.mini {
  width: 62px;
  height: 90px;
  border-radius: 7px;
  border: 1px solid #cdd3e3;
  background: linear-gradient(163deg, #fff 0%, #fafbfe 46%, #eef0f7 100%);
  color: var(--card-black);
  display: grid;
  place-items: center;
  position: relative;
  box-shadow: 0 1px 1px rgba(0, 0, 0, 0.3), 0 10px 22px rgba(0, 0, 0, 0.3);
}
.mini.red { color: var(--card-red); }
.mini b { font-size: 1.7rem; font-weight: 600; line-height: 1; letter-spacing: -0.03em; }
.mini .frame {
  position: absolute;
  inset: 4px;
  border: 1px solid currentColor;
  border-radius: 4px;
  opacity: 0.13;
}
.mini .pip { position: absolute; }
.mini .pip.tl { top: 7px; left: 7px; }
.mini .pip.br { bottom: 7px; right: 7px; transform: rotate(180deg); }
.equals { display: flex; align-items: center; gap: 14px; }
.expr { font-family: var(--font-mono); font-size: 0.9rem; color: var(--text-mute); }
.result {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--accent);
  letter-spacing: -0.02em;
}
.equals::before {
  content: '=';
  order: 1;
  color: var(--text-mute);
}
.result { order: 2; }

/* ---------- sections ---------- */
.block { display: flex; flex-direction: column; gap: 14px; }
.split { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; align-items: start; }
.hint { color: var(--text-mute); font-size: 0.88rem; line-height: 1.5; }
.head { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.more {
  font-size: 0.82rem;
  color: var(--accent);
  text-decoration: none;
  white-space: nowrap;
}
@media (hover: hover) { .more:hover { text-decoration: underline; } }
.join { display: flex; gap: 10px; }
.code {
  flex: 1;
  min-width: 0;
  letter-spacing: 0.34em;
  text-transform: uppercase;
  text-align: center;
  font-weight: 600;
}
.code::placeholder { letter-spacing: 0.06em; font-weight: 400; }

@media (max-width: 900px) {
  .hero { grid-template-columns: 1fr; gap: 30px; padding: 30px 24px 32px; }
  .split { grid-template-columns: 1fr; }
}
@media (max-width: 560px) {
  .home { gap: 22px; padding-bottom: 44px; }
  .hero { padding: 24px 20px 26px; }
  .cta { flex-direction: column; }
  .cta .btn { width: 100%; }
  .mini { width: 52px; height: 74px; }
  .mini b { font-size: 1.45rem; }
}
</style>
