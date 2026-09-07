<script setup>
// Small modal to change the player's nickname.
import { ref } from 'vue'
import AnimatedIcon from './AnimatedIcon.vue'
import { useI18n } from '../i18n/index.js'
import { getPlayer, renamePlayer } from '../auth.js'
import { randomName } from '../core/names.js'
import { sfx } from '../audio.js'

const { t } = useI18n()
const emit = defineEmits(['saved', 'close'])

const name = ref(getPlayer()?.nickname ?? '')
const busy = ref(false)
const error = ref('')

function reroll() {
  sfx.click()
  name.value = randomName()
}

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
    emit('saved', trimmed)
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
      <span class="hero-ic"><AnimatedIcon name="edit" :size="26" /></span>
      <h3>{{ t('profileRename') }}</h3>
      <div class="name-row">
        <input
          v-model="name"
          class="name-input"
          :placeholder="t('nicknamePlaceholder')"
          maxlength="24"
          @keyup.enter="save"
        />
        <button class="btn icon dice" :aria-label="t('randomName')" :title="t('randomName')" @click="reroll">
          <AnimatedIcon name="dice" :size="19" />
        </button>
      </div>
      <p v-if="error" class="err">{{ error }}</p>
      <div class="actions">
        <button class="btn icon" aria-label="close" @click="$emit('close')"><AnimatedIcon name="close" :size="18" /></button>
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
/* animated pencil in a warm tile — the modal explains itself at a glance */
.hero-ic {
  align-self: center;
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 16px;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid rgba(246, 183, 60, 0.3);
  box-shadow: 0 0 24px rgba(246, 183, 60, 0.12);
}
h2, h3 { font-size: 1.15rem; font-weight: 500; }
.name-row { display: flex; gap: 8px; }
.name-row .name-input { flex: 1; min-width: 0; }
.dice { flex: none; }
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
