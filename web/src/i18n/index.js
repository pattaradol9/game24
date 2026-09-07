import { reactive, computed } from 'vue'
import { messages } from './messages.js'

const saved = localStorage.getItem('locale')
export const locale = reactive({ lang: saved === 'en' ? 'en' : 'th' })

// Keep <html lang> in sync for accessibility and crawlers.
document.documentElement.lang = locale.lang

export function t(key) {
  return messages[locale.lang][key] ?? messages.th[key] ?? key
}

export function useI18n() {
  const lang = computed(() => locale.lang)
  function toggle() {
    locale.lang = locale.lang === 'th' ? 'en' : 'th'
    localStorage.setItem('locale', locale.lang)
    document.documentElement.lang = locale.lang
  }
  return { lang, t, toggle }
}
