<script setup>
import { ref } from 'vue'
import Icon from './Icon.vue'
import { useI18n } from '../i18n/index.js'
import { api } from '../api.js'

const { t } = useI18n()
const emit = defineEmits(['created', 'close'])

const creating = ref(false)
const error = ref('')
const mode = ref('queen')
const rounds = ref(12)
const hintQuota = ref(3)
const regenQuota = ref(2)

async function create() {
  creating.value = true
  error.value = ''
  try {
    const data = await api.createRoom({
      mode: mode.value,
      rounds: rounds.value,
      hintQuota: hintQuota.value,
      regenQuota: regenQuota.value,
    })
    emit('created', data.code, data.hostKey)
  } catch (e) {
    error.value = e.message
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div class="overlay" @click.self="$emit('close')">
    <div class="panel modal">
      <h2>{{ t('createRoom') }}</h2>
      <label>{{ t('modes') }}</label>
      <select v-model="mode">
        <option value="jack">JACK ×1</option>
        <option value="queen">QUEEN ×2</option>
        <option value="king">KING ×3</option>
        <option value="ace">ACE ×5</option>
      </select>
      <label>{{ t('rounds') }}</label>
      <div class="row-btns">
        <button
          v-for="r in [12, 24, 36, 48]"
          :key="r"
          class="btn"
          :class="{ primary: rounds === r }"
          @click="rounds = r"
        >
          {{ r }}
        </button>
      </div>
      <label>{{ t('hintQuota') }}: {{ hintQuota }}</label>
      <input v-model.number="hintQuota" type="range" min="0" max="5" />
      <label>{{ t('regenQuota') }}: {{ regenQuota }}</label>
      <input v-model.number="regenQuota" type="range" min="0" max="5" />
      <p class="note">{{ t('multiplayerBoardHint') }}</p>
      <p v-if="error" class="err">{{ error }}</p>
      <div class="actions">
        <button class="btn icon" aria-label="close" @click="$emit('close')"><Icon name="close" :size="18" /></button>
        <button class="btn primary big" :disabled="creating" @click="create">{{ t('createRoom') }}</button>
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
