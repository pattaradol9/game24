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
  'chevron-down': 'M6 9l6 6 6-6',
  'chevron-right': 'M9 6l6 6-6 6',
  'arrow-right': 'M5 12h14M12 5l7 7-7 7',
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
  dice: 'M5 5h14v14H5zM9 9h.01M15 9h.01M9 15h.01M15 15h.01M12 12h.01',
  heart: 'M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.6l-1-1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21.2l7.8-7.8 1-1a5.5 5.5 0 0 0 0-7.8z',
  trophy: 'M8 21h8M12 17v4M7 4h10v6a5 5 0 0 1-10 0V4zM7 5H4v1a3 3 0 0 0 3 3M17 5h3v1a3 3 0 0 1-3 3',
  lock: 'M6 11h12v9H6zM9 11V8a3 3 0 0 1 6 0v3',
  palette: 'M12 21a9 9 0 1 1 0-18c5 0 9 3.6 9 8 0 2.6-2.1 4.2-4.6 4.2H15a2 2 0 0 0-1.4 3.4c.5.6 0 1.5-.8 1.6-.3.1-.5.1-.8.1zM7.5 10.5h.01M12 7.5h.01M16.5 10.5h.01M7.5 14.5h.01',
  trash: 'M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6M10 11v6M14 11v6',
  bolt: 'M13 2L4.5 13.5H11L9.5 22 19 9.5h-6.5L13 2z',
  home: 'M3 11l9-8 9 8M5.5 9.5V20h13V9.5M10 20v-6h4v6',
  sliders: 'M4 7h9M17.5 7H20M15 4.5v5M4 17h3.5M12 17h8M9.5 14.5v5',
  globe: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3.6 9h16.8M3.6 15h16.8M12 3c2.5 2.3 3.9 5.2 3.9 9s-1.4 6.7-3.9 9M12 3c-2.5 2.3-3.9 5.2-3.9 9s1.4 6.7 3.9 9',
}
const FILLED = new Set(['play'])
</script>

<template>
  <!-- Currency chip: a casino poker chip, always in the gold/cream palette
       regardless of currentColor (currency shouldn't recolour inside
       chips/buttons). Flat fills, no defs, so duplicated instances are safe. -->
  <svg v-if="name === 'coin'" class="icon" :width="size" :height="size" viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="12" cy="12" r="9.75" fill="none" stroke="#cf8f1e" stroke-width="0.9" />
    <circle cx="12" cy="12" r="9.75" fill="#fbf3de" />
    <g fill="none" stroke="#eda32c" stroke-width="3.4">
      <path d="M18.6 12h2.95" />
      <path d="M15.3 17.72l1.48 2.55" />
      <path d="M8.7 17.72l-1.48 2.55" />
      <path d="M5.4 12H2.45" />
      <path d="M8.7 6.28L7.22 3.73" />
      <path d="M15.3 6.28l1.48-2.55" />
    </g>
    <circle cx="12" cy="12" r="5.3" fill="none" stroke="#eda32c" stroke-width="1.5" />
  </svg>
  <svg
    v-else
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
