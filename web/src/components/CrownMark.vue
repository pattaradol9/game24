<script setup>
// Gold crown used where the board itself is the subject — the empty
// leaderboard, before anyone has claimed the top spot. Inline SVG so it stays
// crisp at any size, animated with CSS only.
defineProps({
  size: { type: Number, default: 96 },
})

</script>

<template>
  <div class="crown-mark" :style="{ width: `${size}px` }">
    <svg viewBox="0 0 120 104" aria-hidden="true">
      <defs>
        <linearGradient id="cmGold" x1="0" y1="0" x2="0.3" y2="1">
          <stop offset="0%" stop-color="#ffd88f" />
          <stop offset="45%" stop-color="#f6b73c" />
          <stop offset="100%" stop-color="#b9791a" />
        </linearGradient>
        <linearGradient id="cmBand" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#f0ad33" />
          <stop offset="100%" stop-color="#8f5c12" />
        </linearGradient>
        <radialGradient id="cmHalo">
          <stop offset="0%" stop-color="#f6b73c" stop-opacity="0.2" />
          <stop offset="60%" stop-color="#f6b73c" stop-opacity="0.07" />
          <stop offset="100%" stop-color="#f59e0b" stop-opacity="0" />
        </radialGradient>
        <clipPath id="cmClip">
          <path d="M14 70 L14 28 L37 50 L60 18 L83 50 L106 28 L106 70 Z" />
          <rect x="10" y="66" width="100" height="18" rx="7" />
        </clipPath>
      </defs>

      <circle class="halo" cx="60" cy="56" r="52" fill="url(#cmHalo)" />

      <g class="crown">
        <g stroke="#6b4408" stroke-width="3.4" stroke-linejoin="round">
          <path d="M14 70 L14 28 L37 50 L60 18 L83 50 L106 28 L106 70 Z" fill="url(#cmGold)" />
          <rect x="10" y="66" width="100" height="18" rx="7" fill="url(#cmBand)" />
        </g>

        <!-- tip jewels -->
        <circle cx="14" cy="25" r="5.5" fill="#f0c266" stroke="#6b4408" stroke-width="2.6" />
        <circle cx="106" cy="25" r="5.5" fill="#f0c266" stroke="#6b4408" stroke-width="2.6" />
        <circle cx="60" cy="15" r="6.5" fill="#ffe0a3" stroke="#6b4408" stroke-width="2.6" />
        <!-- band jewels -->
        <circle cx="35" cy="75" r="4" fill="#ffe0a3" opacity="0.9" />
        <circle cx="60" cy="75" r="5" fill="#f0c266" opacity="0.9" />
        <circle cx="85" cy="75" r="4" fill="#f0c266" opacity="0.9" />

        <!-- light sweeping across the metal -->
        <g clip-path="url(#cmClip)">
          <rect class="shine" x="-46" y="4" width="26" height="96" fill="#fff" opacity="0.5" transform="skewX(-22)" />
        </g>
      </g>

    </svg>
  </div>
</template>

<style scoped>
.crown-mark { display: inline-block; line-height: 0; }
svg { width: 100%; height: auto; overflow: visible; }

.crown {
  transform-origin: 60px 76px;
  animation: cm-float 3.6s ease-in-out infinite;
  filter: drop-shadow(0 10px 18px rgba(0, 0, 0, 0.45));
}
.halo { animation: cm-breathe 3.6s ease-in-out infinite; transform-origin: 60px 56px; }
.shine { animation: cm-shine 4.2s ease-in-out infinite; }

@keyframes cm-float {
  0%, 100% { transform: translateY(2px) rotate(-1.6deg); }
  50% { transform: translateY(-5px) rotate(1.6deg); }
}
@keyframes cm-breathe {
  0%, 100% { transform: scale(0.92); opacity: 0.8; }
  50% { transform: scale(1.06); opacity: 1; }
}
@keyframes cm-shine {
  0%, 55% { transform: translateX(0) skewX(-22deg); }
  85%, 100% { transform: translateX(190px) skewX(-22deg); }
}
</style>
