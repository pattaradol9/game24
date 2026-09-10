<script setup>
// Irreversible self-service account deletion. The typed confirmation word
// feeds the API's {"confirm":"DELETE"} guard on DELETE /me.
import { computed, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { deleteAccount } from '../auth.js'
import { sfx } from '../audio.js'
import Icon from './Icon.vue'

const { t } = useI18n()
const emit = defineEmits(['deleted', 'close'])

const word = ref('')
const busy = ref(false)
const error = ref('')
const ready = computed(() => word.value.trim().toUpperCase() === 'DELETE')

async function confirmDelete() {
  if (busy.value || !ready.value) return
  busy.value = true
  error.value = ''
  try {
    await deleteAccount()
    sfx.click()
    emit('deleted')
    emit('close')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="overlay">
    <div class="panel">
      <span class="hero-ic"><Icon name="trash" :size="24" /></span>
      <h3>{{ t('deleteTitle') }}</h3>
      <p class="lead">{{ t('deleteBody') }}</p>
      <label for="del-confirm">{{ t('deleteTypeHint') }}</label>
      <input
        id="del-confirm"
        v-model="word"
        class="name-input"
        autocomplete="off"
        spellcheck="false"
        maxlength="12"
        @keyup.enter="confirmDelete"
      />
      <p v-if="error" class="err">{{ error }}</p>
      <div class="actions">
        <button class="btn" :disabled="busy" @click="$emit('close')">{{ t('deleteKeepBtn') }}</button>
        <button class="btn danger" :disabled="busy || !ready" @click="confirmDelete">
          <Icon name="trash" :size="15" />{{ t('deleteConfirmBtn') }}
        </button>
      </div>
      <p class="note">
        {{ t('deleteSeePrivacy') }} <RouterLink to="/privacy" @click="$emit('close')">{{ t('privacy') }}</RouterLink>
      </p>
    </div>
  </div>
</template>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 160;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(6, 8, 13, 0.74);
}
.panel {
  width: min(400px, 100%);
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 24px;
  border-color: rgba(239, 95, 95, 0.35);
  animation: rise-in 0.22s var(--ease);
}
.hero-ic {
  align-self: center;
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 16px;
  color: var(--bad);
  background: rgba(239, 95, 95, 0.12);
  border: 1px solid rgba(239, 95, 95, 0.35);
  box-shadow: 0 0 24px rgba(239, 95, 95, 0.12);
}
h3 { font-size: 1.15rem; font-weight: 500; }
.lead { font-size: 0.88rem; line-height: 1.65; color: var(--text-dim); }
label { font-size: 0.75rem; letter-spacing: 0.1em; text-transform: uppercase; color: var(--text-mute); margin-top: 2px; }
.name-input {
  width: 100%;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  padding: 12px 14px;
  font: inherit;
  font-size: 0.95rem;
  background: var(--bg);
  color: var(--text);
  outline: none;
}
.name-input:focus { border-color: var(--bad); }
.err { color: var(--bad); font-size: 0.84rem; }
.actions { display: flex; gap: 10px; margin-top: 6px; }
.actions .btn { flex: 1; }
.note { font-size: 0.78rem; color: var(--text-mute); line-height: 1.5; text-align: center; }
.note a { color: var(--text-dim); text-underline-offset: 3px; }
@media (hover: hover) { .note a:hover { color: var(--text); } }
</style>
