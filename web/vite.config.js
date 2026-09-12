import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { VitePWA } from 'vite-plugin-pwa'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.dirname(fileURLToPath(import.meta.url))

// Inlines the built app stylesheet into index.html. The CSS then travels
// with the document instead of chaining a second render-blocking request
// after it, which shaves a full round trip off first paint.
function inlineAppCss() {
  return {
    name: 'inline-app-css',
    apply: 'build',
    closeBundle() {
      const htmlPath = path.join(root, 'dist', 'index.html')
      const html = fs.readFileSync(htmlPath, 'utf8')
      const inlined = html.replace(
        /<link rel="stylesheet"[^>]*href="\/(assets\/[^"]+\.css)"[^>]*\/?>(?:<\/link>)?/,
        (match, href) => {
          const cssPath = path.join(root, 'dist', href)
          return `<style>${fs.readFileSync(cssPath, 'utf8')}</style>`
        },
      )
      fs.writeFileSync(htmlPath, inlined)
    },
  }
}

export default defineConfig({
  // Workers bundling as ES modules keeps the backdrop worker's three.js
  // import off the main thread.
  worker: { format: 'es' },
  build: {
    // One stylesheet for the whole app: it gets inlined into index.html by
    // the plugin below, so first paint never waits on a chained CSS request.
    cssCodeSplit: false,
  },
  plugins: [
    vue(),
    inlineAppCss(),
    VitePWA({
      // The service worker updates silently in the background; the next
      // reload picks up the new version (no "update available" prompt UI).
      registerType: 'autoUpdate',
      // main.js registers the worker itself after the page has loaded and
      // the browser is idle: registration during load shows up as a long
      // task on the main thread and steals bandwidth from first paint.
      injectRegister: null,
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
