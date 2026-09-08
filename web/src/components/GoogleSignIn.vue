<script setup>
// Custom-styled Google sign-in button: the visible layer is drawn with the
// site's own button language, while the official GSI button is rendered on
// top fully transparent — every click still opens Google's real flow.
import { onMounted, ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { initGoogle, renderGoogleButton, signInWithGoogle } from '../auth.js'

const emit = defineEmits(['signed-in'])
const { t } = useI18n()

// null = probing availability, false = Google sign-in unavailable,
// true = button rendered
const ready = ref(null)
const hot = ref(false)
const wrapEl = ref(null)
const btnEl = ref(null)
const error = ref('')

onMounted(async () => {
  try {
    await initGoogle()
    // the container stays laid out while probing, so its width is real
    const width = wrapEl.value ? Math.round(wrapEl.value.clientWidth) : 0
    ready.value = renderGoogleButton(
      btnEl.value,
      async (credential) => {
        try {
          const player = await signInWithGoogle(credential)
          emit('signed-in', player)
        } catch (e) {
          // a banned account raises the global BannedModal from auth.js —
          // no inline line needed; everything else stays a small error here
          error.value = e.message === 'account banned' ? '' : e.message
        }
      },
      width ? { width } : {}
    )
  } catch {
    ready.value = false
  }
})
</script>

<template>
  <div
    ref="wrapEl"
    class="gsi-box"
    :class="{ pending: ready === null, gone: ready === false }"
    @mouseenter="hot = true"
    @mouseleave="hot = false"
  >
    <span class="g-hitwrap">
      <span class="g-btn" :class="{ hot }" aria-hidden="true">
        <svg class="g-logo" viewBox="0 0 18 18" width="17" height="17" aria-hidden="true">
          <path fill="#4285F4" d="M17.64 9.2045c0-.6381-.0573-1.2518-.1636-1.8409H9v3.4814h4.8436a4.14 4.14 0 0 1-1.796 2.716v2.2591h2.9087c1.7019-1.5668 2.6837-4.8749 2.6837-8.6156z" />
          <path fill="#34A853" d="M9 18c2.43 0 4.4673-.8059 5.9564-2.1805l-2.9087-2.2591c-.8059.54-1.8368.859-3.0477.859-2.344 0-4.3282-1.5831-5.036-3.7104H.9573v2.3318A8.9965 8.9965 0 0 0 9 18z" />
          <path fill="#FBBC05" d="M3.964 10.71a5.41 5.41 0 0 1 0-3.42V4.9592H.9573a8.9965 8.9965 0 0 0 0 8.0816l3.0067-2.3318z" />
          <path fill="#EA4335" d="M9 3.5795c1.3214 0 2.5077.4541 3.4405 1.346l2.5813-2.5814C13.4632.8918 11.4259 0 9 0A8.9965 8.9965 0 0 0 .9573 4.9592l3.0067 2.3318C4.6718 5.1627 6.6559 3.5795 9 3.5795z" />
        </svg>
        {{ t('googleSignIn') }}
      </span>
      <div ref="btnEl" class="g-hit" />
    </span>
    <p v-if="error" class="err">{{ error }}</p>
  </div>
</template>

<style scoped>
.gsi-box { width: 100%; }
.gsi-box.pending { visibility: hidden; }
.gsi-box.gone { display: none; }

.g-hitwrap { position: relative; display: block; }

/* mirrors style.css .btn exactly, so the row reads as one of our buttons */
.g-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  width: 100%;
  min-height: 44px;
  padding: 12px 18px;
  border: 1px solid var(--line);
  border-radius: var(--r-md);
  background: var(--surface-2);
  color: var(--text);
  font: inherit;
  font-size: 0.95rem;
  font-weight: 500;
  line-height: 1;
  user-select: none;
  cursor: pointer;
  transition: background 0.15s var(--ease), border-color 0.15s var(--ease);
}
.g-btn.hot {
  background: var(--surface-3);
  border-color: #35405a;
}
.g-logo { flex: none; }

/* the real GSI button: transparent, stretched over the drawn button so
   every click opens Google's official flow (hover styles come from the
   wrapper's mouseenter/leave, since the iframe owns the pointer) */
.g-hit {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  opacity: 0;
  cursor: pointer;
}
.err { color: var(--bad); font-size: 0.85rem; }
</style>
