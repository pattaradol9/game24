<script setup>
// App shell: routes render their own layout; global styles live in style.css.
// GameBackdrop mounts the ambient Three.js card scene once for all routes;
// BannedModal is the global ban verdict dialog (fired by the live socket);
// MobileNav gathers the primary destinations (profile included) into a bottom
// bar on phones. The admin portal skips the bottom bar — it is an internal
// tool with its own chrome.
import { useRoute } from 'vue-router'
import GameBackdrop from './components/GameBackdrop.vue'
import BannedModal from './components/BannedModal.vue'
import MobileNav from './components/MobileNav.vue'

const route = useRoute()
const showBottomNav = () => !route.path.startsWith('/admin')
</script>

<template>
  <GameBackdrop />
  <router-view />
  <MobileNav v-if="showBottomNav()" />
  <BannedModal />
</template>
