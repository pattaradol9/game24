import { reactive, computed } from 'vue'
import { messages } from './messages.js'

const saved = localStorage.getItem('locale')
const locale = reactive({ lang: saved === 'en' ? 'en' : 'th' })

export function t(key) {
  return messages[locale.lang][key] ?? messages.th[key] ?? key
}

export function useI18n() {
  const lang = computed(() => locale.lang)
  function toggle() {
    locale.lang = locale.lang === 'th' ? 'en' : 'th'
    localStorage.setItem('locale', locale.lang)
  }
  return { lang, t, toggle }
}
