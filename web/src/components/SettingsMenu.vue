<script setup>
// Settings menu: the sound and language toggles collapsed into one popover,
// used on desktop and mobile alike. On phones the rows drop their icons and
// go text-only. Toggles mirror Home's own behaviour (audio.js / i18n).
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { soundEnabled, toggleSound } from '../audio.js'
import Icon from './Icon.vue'

const { t, lang, toggle: toggleLang } = useI18n()

const open = ref(false)
const sound = ref(soundEnabled())
const wrap = ref(null)

function flipSound() {
  sound.value = toggleSound()
}

function flipLang() {
  toggleLang()
}

function onDocClick(e) {
  if (open.value && wrap.value && !wrap.value.contains(e.target)) open.value = false
}
function onKey(e) {
  if (e.key === 'Escape') open.value = false
}
onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div ref="wrap" class="smenu">
    <button
      class="btn icon gear"
      :class="{ on: open }"
      :aria-label="t('settings')"
      @click="open = !open"
    >
      <Icon name="sliders" :size="19" />
    </button>

    <Transition name="drop">
      <div v-if="open" class="sheet">
        <button class="row" @click="flipSound">
          <span class="ric"><Icon :name="sound ? 'sound-on' : 'sound-off'" :size="17" /></span>
          <span class="lbl">{{ t('sound') }}</span>
          <span class="val" :class="{ off: !sound }">{{ sound ? t('stateOn') : t('stateOff') }}</span>
        </button>
        <button class="row" @click="flipLang">
          <span class="ric"><Icon name="globe" :size="17" /></span>
          <span class="lbl">{{ t('language') }}</span>
          <span class="val">{{ lang === 'th' ? 'ไทย' : 'English' }}</span>
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.smenu { position: relative; }

.gear.on {
  border-color: rgba(246, 183, 60, 0.55);
  color: var(--accent);
}

.sheet {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  width: 212px;
  border-radius: var(--r-md);
  border: 1px solid var(--line);
  background: var(--surface-2);
  box-shadow: var(--sh-3), 0 10px 44px rgba(0, 0, 0, 0.4);
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  transform-origin: top right;
  z-index: 90;
}

.row {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 0.86rem;
  border-radius: var(--r-sm);
  padding: 9px 8px;
  transition: background 0.15s var(--ease);
}
@media (hover: hover) { .row:hover { background: var(--surface-3); } }
.ric {
  width: 17px;
  display: inline-flex;
  justify-content: center;
  color: var(--text-dim);
}
.lbl { flex: 1; text-align: left; }
.val { color: var(--accent); font-size: 0.78rem; font-weight: 600; }
.val.off { color: var(--text-mute); }

.drop-enter-active { animation: menu-pop 0.2s var(--ease-out-back); }
@keyframes menu-pop {
  from { transform: translateY(-6px) scale(0.97); opacity: 0; }
  to { transform: translateY(0) scale(1); opacity: 1; }
}
.drop-leave-active { transition: opacity 0.12s var(--ease), transform 0.12s var(--ease); }
.drop-leave-to { opacity: 0; transform: translateY(-4px) scale(0.98); }
</style>
