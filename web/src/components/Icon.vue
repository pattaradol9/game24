<script setup>
// Inline stroke icons, replacing the emoji the old UI leaned on. 24px grid,
// currentColor, so they inherit weight and colour from the button they sit in.
defineProps({
  name: { type: String, required: true },
  size: { type: [Number, String], default: 20 },
})

const PATHS = {
  'sound-on': 'M4 9v6h4l5 4V5L8 9H4zM16.5 8.5a5 5 0 0 1 0 7M19 6a8.5 8.5 0 0 1 0 12',
  'sound-off': 'M4 9v6h4l5 4V5L8 9H4zM17 9.5l5 5M22 9.5l-5 5',
  back: 'M15 5l-7 7 7 7',
  bulb: 'M9 18h6M10 21h4M12 3a6 6 0 0 0-3.5 10.9c.5.4.8 1 .8 1.6v.5h5.4v-.5c0-.6.3-1.2.8-1.6A6 6 0 0 0 12 3z',
  undo: 'M4 9h11a5 5 0 0 1 0 10h-6M4 9l4-4M4 9l4 4',
  skip: 'M5 5l8 7-8 7V5zM17 5v14',
  clock: 'M12 7v5l3.5 2M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z',
  link: 'M10 13.5a4 4 0 0 0 5.7 0l2.8-2.8a4 4 0 0 0-5.7-5.7l-1.4 1.4M14 10.5a4 4 0 0 0-5.7 0l-2.8 2.8a4 4 0 0 0 5.7 5.7l1.4-1.4',
  play: 'M7 4.5l12 7.5-12 7.5v-15z',
  check: 'M4.5 12.5l5 5 10-11',
  close: 'M6 6l12 12M18 6L6 18',
  user: 'M4.5 20a7.5 7.5 0 0 1 15 0M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z',
  crown: 'M4 18h16M4 18l-1-10 5.5 4L12 5l3.5 7L21 8l-1 10',
  flame: 'M12 3s5 4.2 5 8.6A5 5 0 0 1 7 12c0-2 1-3.4 1-3.4S9 11 10.5 11 12 8 12 3z',
  refresh: 'M20 11a8 8 0 1 0-.6 4M20 5v6h-6',
  logout: 'M15 16l4-4-4-4M19 12H9M12 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h6',
  edit: 'M4 20h4L19 9a2.1 2.1 0 0 0-3-3L5 17v3z',
  heart: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.6l-1-1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21.2l7.8-7.8 1-1a5.5 5.5 0 0 0 0-7.8z',
}
const FILLED = new Set(['play'])
</script>

<template>
  <svg
    class="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    :fill="FILLED.has(name) ? 'currentColor' : 'none'"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="PATHS[name] ?? ''" />
  </svg>
</template>

<style scoped>
.icon { display: block; flex: none; }
</style>
