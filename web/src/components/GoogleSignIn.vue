<script setup>
import { onMounted, ref } from 'vue'
import { initGoogle, renderGoogleButton, signInWithGoogle } from '../auth.js'

const emit = defineEmits(['signed-in'])
const btnEl = ref(null)
const error = ref('')
const available = ref(false)

onMounted(async () => {
  try {
    await initGoogle()
    available.value = renderGoogleButton(btnEl.value, async (credential) => {
      try {
        const player = await signInWithGoogle(credential)
        emit('signed-in', player)
      } catch (e) {
        error.value = e.message
      }
    })
  } catch {
    available.value = false
  }
})
</script>

<template>
  <div>
    <div v-show="available" ref="btnEl" class="gsi" />
    <p v-if="error" class="err">{{ error }}</p>
  </div>
</template>

<style scoped>
.gsi { min-height: 40px; display: flex; justify-content: center; }
.err { color: var(--bad); font-size: 0.85rem; }
</style>
