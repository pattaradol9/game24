<script setup>
import { ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { getPlayer, signInAsGuest } from '../auth.js'
import { randomName } from '../core/names.js'
import { sfx } from '../audio.js'
import GoogleSignIn from './GoogleSignIn.vue'
import Brand from './Brand.vue'
import AnimatedIcon from './AnimatedIcon.vue'

const { t } = useI18n()
const emit = defineEmits(['done'])

const name = ref(randomName())
const busy = ref(false)
const error = ref('')

function reroll() {
  sfx.click()
  name.value = randomName()
}

async function playAsGuest() {
  if (busy.value) return
  error.value = ''
  busy.value = true
  try {
    await signInAsGuest(name.value.trim() || randomName())
    emit('done', getPlayer())
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="overlay">
    <div class="panel modal">
      <Brand size="md" :label="t('appName')" />
      <h2>{{ t('nicknameLabel') }}</h2>
      <div class="name-row">
        <input
          v-model="name"
          class="field"
          :placeholder="t('nicknamePlaceholder')"
          maxlength="24"
          @keyup.enter="playAsGuest"
        />
        <button class="btn icon dice" :aria-label="t('randomName')" :title="t('randomName')" @click="reroll">
          <AnimatedIcon name="dice" :size="19" />
        </button>
      </div>
      <button class="btn primary block" :disabled="busy" @click="playAsGuest">
        {{ t('playAsGuest') }}
      </button>
      <div class="rule"><span>or</span></div>
      <GoogleSignIn @signed-in="emit('done', $event)" />
      <p class="hint">{{ t('googleHint') }}</p>
      <p class="legal">
        {{ t('legalAgree') }}
        <RouterLink to="/terms">{{ t('terms') }}</RouterLink>
        {{ t('legalAnd') }}
        <RouterLink to="/privacy">{{ t('privacy') }}</RouterLink>
      </p>
      <p v-if="error" class="err">{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.72);
}
.modal {
  width: min(400px, 100%);
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 14px;
  padding: 28px;
  animation: rise-in 0.24s var(--ease);
}
h2 { font-size: 1.25rem; font-weight: 500; margin-top: 4px; }
.name-row { display: flex; gap: 8px; }
.name-row .field { flex: 1; min-width: 0; }
.dice { flex: none; }
.hint { font-size: 0.8rem; color: var(--text-mute); line-height: 1.5; }
.legal { font-size: 0.75rem; color: var(--text-mute); line-height: 1.6; }
.legal a { color: var(--text-dim); }
@media (hover: hover) { .legal a:hover { color: var(--accent); text-decoration: underline; } }
.err { color: var(--bad); font-size: 0.84rem; }
.rule {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--text-mute);
  font-size: 0.75rem;
}
.rule::before, .rule::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--line);
}
</style>
