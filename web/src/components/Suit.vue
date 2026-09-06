<script setup>
// The four card suits as real vector shapes, so they stay sharp at any size
// and are not at the mercy of the platform's emoji font.
defineProps({
  name: { type: String, required: true }, // spade | heart | diamond | club
  size: { type: [Number, String], default: 20 },
})

const PATHS = {
  diamond: 'M12 2.2 20.4 12 12 21.8 3.6 12Z',
  heart: 'M12 21.2C5.6 16.5 2.8 13.1 2.8 9.6 2.8 6.7 5 4.6 7.8 4.6c1.8 0 3.3.9 4.2 2.3.9-1.4 2.4-2.3 4.2-2.3 2.8 0 5 2.1 5 5 0 3.5-2.8 6.9-9.2 11.6Z',
  spade: 'M12 2.4S3.4 9 3.4 13.8c0 2.4 1.8 4.1 4 4.1 1.2 0 2.3-.5 3-1.3 0 1.8-.8 3.4-2 4.6h7.2c-1.2-1.2-2-2.8-2-4.6.7.8 1.8 1.3 3 1.3 2.2 0 4-1.7 4-4.1C20.6 9 12 2.4 12 2.4Z',
  club: 'M12 2.6a3.9 3.9 0 0 1 3.3 6 3.9 3.9 0 1 1-1.6 6.5c.2 2.2 1 3.9 2.1 5.1H8.2c1.1-1.2 1.9-2.9 2.1-5.1a3.9 3.9 0 1 1-1.6-6.5A3.9 3.9 0 0 1 12 2.6Z',
}
const RED = new Set(['heart', 'diamond'])
</script>

<template>
  <svg
    class="suit"
    :class="{ red: RED.has(name) }"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="currentColor"
    aria-hidden="true"
  >
    <path :d="PATHS[name] ?? ''" />
  </svg>
</template>

<style scoped>
.suit { display: block; flex: none; color: inherit; }
.suit.red { color: var(--card-red); }
</style>
