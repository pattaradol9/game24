import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'

export default defineConfig({
  plugins: [
    vue(),
    VitePWA({
      // The service worker updates silently in the background; the next
      // reload picks up the new version (no "update available" prompt UI).
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg', 'apple-touch-icon.png', 'og-card.png'],
      manifest: {
        name: '24 Game — เกมรวมเลขให้ได้ 24',
        short_name: '24 Game',
        description:
          'จับไพ่ 4 ใบ ผสมด้วย + − × ÷ ให้ได้ 24 เล่นเดี่ยวเก็บ EXP หรือสร้างห้องปะทะเพื่อนแบบเรียลไทม์',
        lang: 'th',
        dir: 'ltr',
        id: '/',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        orientation: 'any',
        background_color: '#0c0f16',
        theme_color: '#0c0f16',
        categories: ['games', 'education'],
        icons: [
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          {
            src: 'pwa-maskable-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        // Precache the whole app shell (hashed assets + index.html) so the
        // SPA boots offline; runtime data still needs the network.
        globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
        navigateFallback: 'index.html',
        // Never answer API navigations with the SPA shell.
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            // Google Fonts: stylesheet refreshed lazily, font files forever.
            urlPattern: ({ url }) => url.origin === 'https://fonts.googleapis.com',
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-stylesheets',
              expiration: { maxEntries: 10, maxAgeSeconds: 60 * 60 * 24 * 365 },
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            urlPattern: ({ url }) => url.origin === 'https://fonts.gstatic.com',
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-files',
              expiration: { maxEntries: 60, maxAgeSeconds: 60 * 60 * 24 * 365 },
              cacheableResponse: { statuses: [200] },
            },
          },
        ],
      },
      devOptions: {
        enabled: true,
        type: 'module',
      },
    }),
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        // `make dev` hits the local API on :8080; the E2E suite points the
        // same dev server at its isolated instance through VITE_API_TARGET
        target: process.env.VITE_API_TARGET || 'http://localhost:8080',
        ws: true,
      },
    },
  },
})
