<script setup>
// Small modal to change the player's nickname.
import { ref } from 'vue'
import Icon from './Icon.vue'
import { useI18n } from '../i18n/index.js'
import { getPlayer, renamePlayer } from '../auth.js'

const { t } = useI18n()
const emit = defineEmits(['saved', 'close'])

const name = ref(getPlayer()?.nickname ?? '')
const busy = ref(false)
const error = ref('')

async function save() {
  if (busy.value) return
  const trimmed = name.value.trim()
  if (!trimmed) {
    error.value = t('nicknameLabel')
    return
  }
  busy.value = true
  error.value = ''
  try {
    await renamePlayer(trimmed)
    emit('saved')
    emit('close')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="panel">
      <h3>{{ t('profileRename') }}</h3>
      <input
        v-model="name"
        class="name-input"
        :placeholder="t('nicknamePlaceholder')"
        maxlength="24"
        @keyup.enter="save"
      />
      <p v-if="error" class="err">{{ error }}</p>
      <div class="actions">
        <button class="btn icon" aria-label="close" @click="$emit('close')"><Icon name="close" :size="18" /></button>
        <button class="btn primary" :disabled="busy" @click="save">{{ t('save') }}</button>
      </div>
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
  width: min(380px, 100%);
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 24px;
  animation: rise-in 0.22s var(--ease);
}
h2, h3 { font-size: 1.15rem; font-weight: 500; }
label { font-size: 0.75rem; letter-spacing: 0.1em; text-transform: uppercase; color: var(--text-mute); margin-top: 6px; }
select, .name-input {
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
select:focus, .name-input:focus { border-color: var(--accent); }
.row-btns { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.row-btns .btn { padding-inline: 0; font-variant-numeric: tabular-nums; }
input[type='range'] { width: 100%; accent-color: var(--accent); }
.note { font-size: 0.78rem; color: var(--text-mute); line-height: 1.5; }
.err { color: var(--bad); font-size: 0.84rem; }
.actions { display: flex; gap: 10px; margin-top: 8px; }
.actions .btn:last-child { flex: 1; }
</style>
