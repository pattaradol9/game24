import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import { restoreSession } from './auth.js'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: () => import('./views/Home.vue') },
    { path: '/solo', component: () => import('./views/SoloGame.vue') },
    { path: '/room/:code', component: () => import('./views/RoomGame.vue') },
    { path: '/leaderboard', component: () => import('./views/Leaderboard.vue') },
    { path: '/admin', component: () => import('./views/Admin.vue') },
  ],
})

async function boot() {
  await restoreSession().catch(() => null)
  createApp(App).use(router).mount('#app')
}

boot()
