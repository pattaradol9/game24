<script setup>
// App shell: routes render their own layout; global styles live in style.css.
// GameBackdrop mounts the ambient Three.js card scene once for all routes.
import GameBackdrop from './components/GameBackdrop.vue'
</script>

<template>
  <GameBackdrop />
  <router-view />
</template>
