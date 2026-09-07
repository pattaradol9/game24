<script setup>
import { onMounted, ref } from 'vue'
import { initGoogle, renderGoogleButton, signInWithGoogle } from '../auth.js'

// fitWidth renders the button at the container's width so it lines up
// with adjacent rows (the Google iframe size is otherwise fixed).
const props = defineProps({
  fitWidth: { type: Boolean, default: false },
})

const emit = defineEmits(['signed-in'])
const wrapEl = ref(null)
const btnEl = ref(null)
const error = ref('')
const available = ref(false)

onMounted(async () => {
  try {
    await initGoogle()
    const width =
      props.fitWidth && wrapEl.value ? Math.round(wrapEl.value.clientWidth) : 0
    available.value = renderGoogleButton(
      btnEl.value,
      async (credential) => {
        try {
          const player = await signInWithGoogle(credential)
          emit('signed-in', player)
        } catch (e) {
          error.value = e.message
        }
      },
      width ? { width } : {}
    )
  } catch {
    available.value = false
  }
})
</script>

<template>
  <div ref="wrapEl">
    <div v-show="available" ref="btnEl" class="gsi" />
    <p v-if="error" class="err">{{ error }}</p>
  </div>
</template>

<style scoped>
.gsi { min-height: 40px; display: flex; justify-content: center; }
.err { color: var(--bad); font-size: 0.85rem; }
</style>
