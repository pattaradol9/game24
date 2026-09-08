import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'
import './skins.css'
import { restoreSession } from './auth.js'
import { initRealtime } from './realtime.js'
import { applySeo } from './seo.js'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('./views/Home.vue'),
      meta: {
        seo: {
          title: { th: '24 Game — รวมเลขให้ได้ 24!', en: '24 Game — Combine 4 cards into 24!' },
          description: {
            th: 'เกมไพ่คณิตศาสตร์คลาสสิก จับไพ่ 4 ใบ ผสมด้วย + − × ÷ ให้ได้เลข 24 เล่นเดี่ยวเก็บ EXP ไต่ tier หรือสร้างห้องแข่งกับเพื่อนแบบเรียลไทม์ ฟรี ไม่ต้องสมัคร',
            en: 'The classic math card game — draw 4 cards and combine them with + − × ÷ to make 24. Play solo for EXP and tiers, or race friends in real-time rooms. Free, no signup.',
          },
        },
      },
    },
    {
      path: '/solo',
      component: () => import('./views/SoloGame.vue'),
      meta: {
        seo: {
          title: { th: 'เล่นเดี่ยว — เก็บ EXP ไต่ Tier · 24 Game', en: 'Solo Play — Grind EXP & Tiers · 24 Game' },
          description: {
            th: 'เล่นเดี่ยว 4 โหมดความยาก (Jack, Queen, King, Ace) มีจับเวลา สตรีค และคำใบ้ เก็บ EXP สะสมเลเวลและ tier',
            en: 'Solo mode with 4 difficulties (Jack, Queen, King, Ace), timer, streaks and hints. Earn EXP, level up and climb tiers.',
          },
        },
      },
    },
    {
      path: '/room/:code',
      component: () => import('./views/RoomGame.vue'),
      meta: {
        seo: {
          title: { th: 'เข้าร่วมห้องแข่ง · 24 Game', en: 'Join the Race Room · 24 Game' },
          description: {
            th: 'แข่งสดกับเพื่อนในห้องเดียวกัน ใบไพ่เดียวกัน ผู้ที่ตอบถูกก่อนชนะรอบนั้น',
            en: 'Race friends in real time on the same hand — first correct answer wins the round.',
          },
          noindex: true,
        },
      },
    },
    {
      path: '/leaderboard',
      component: () => import('./views/Leaderboard.vue'),
      meta: {
        seo: {
          title: { th: 'จัดอันดับผู้เล่น · 24 Game', en: 'Player Leaderboards · 24 Game' },
          description: {
            th: 'กระดานจัดอันดับรายโหมด ดูอันดับ EXP เลเวล และ tier ของผู้เล่นทั้งหมด',
            en: 'Per-mode leaderboards — EXP, levels and tiers of every signed-in player.',
          },
        },
      },
    },
    {
      path: '/achievements',
      component: () => import('./views/AchievementsView.vue'),
      meta: {
        seo: {
          title: { th: 'ความสำเร็จ · 24 Game', en: 'Achievements · 24 Game' },
          description: {
            th: 'รวมความสำเร็จทั้งหมดของ 24 Game ไล่ระดับบรอนซ์ถึงเลเจนด์ ปลดล็อกเพื่อรับ EXP และเหรียญ',
            en: 'Every 24 Game achievement from bronze to legend — unlock them to earn EXP and coins.',
          },
        },
      },
    },
    {
      path: '/skins',
      component: () => import('./views/SkinShopView.vue'),
      meta: {
        seo: {
          title: { th: 'ชุดไพ่ · 24 Game', en: 'Card Skins · 24 Game' },
          description: {
            th: 'สะสมเหรียญจากการเล่นแล้วแลกชุดไพ่สุดพิเศษ 12 แบบ ตั้งแต่คลาสสิกถึงกาแล็กซี',
            en: 'Earn coins by playing and trade them for 12 special card skins, from classic to galaxy.',
          },
        },
      },
    },
    {
      path: '/privacy',
      component: () => import('./views/LegalView.vue'),
      props: { doc: 'privacy' },
      meta: {
        seo: {
          title: { th: 'นโยบายความเป็นส่วนตัว · 24 Game', en: 'Privacy Policy · 24 Game' },
          description: {
            th: 'นโยบายความเป็นส่วนตัวของ 24 Game — ข้อมูลที่เก็บ การเข้ารหัส และสิทธิของผู้เล่น',
            en: 'The 24 Game privacy policy — what data is stored, how it is encrypted, and your rights.',
          },
        },
      },
    },
    {
      path: '/terms',
      component: () => import('./views/LegalView.vue'),
      props: { doc: 'terms' },
      meta: {
        seo: {
          title: { th: 'ข้อกำหนดการใช้งาน · 24 Game', en: 'Terms of Service · 24 Game' },
          description: {
            th: 'ข้อกำหนดการใช้งาน 24 Game',
            en: 'The 24 Game terms of service.',
          },
        },
      },
    },
    {
      path: '/admin',
      component: () => import('./views/Admin.vue'),
      meta: {
        seo: {
          title: 'Admin portal · 24 Game',
          description: 'Back-office portal for 24 Game administrators.',
          noindex: true,
        },
      },
    },
  ],
})

router.afterEach((to) => {
  applySeo(to.meta.seo, to.path)
})

async function boot() {
  await restoreSession().catch(() => null)
  // live profile push: admin adjustments land without a refresh
  initRealtime()
  createApp(App).use(router).mount('#app')
}

boot()
