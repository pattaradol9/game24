// Client-side SEO: keeps <title>, meta description, canonical and Open
// Graph/Twitter tags in sync with the active route and UI language.
// The Go server rewrites the same tags server-side for crawlers that do
// not execute JS (link previews, some bots); this module covers every
// client-side navigation after that.
import { locale } from './i18n/index.js'

function pick(value, fallback) {
  if (value == null) return fallback
  return typeof value === 'string' ? value : value[locale.lang] ?? fallback
}

function setMeta(attr, key, content) {
  let el = document.head.querySelector(`meta[${attr}="${key}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, key)
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

function setLink(rel, href) {
  let el = document.head.querySelector(`link[rel="${rel}"]`)
  if (!el) {
    el = document.createElement('link')
    el.setAttribute('rel', rel)
    document.head.appendChild(el)
  }
  el.setAttribute('href', href)
}

// Apply route-level SEO data: { title, description, image?, noindex? }.
// title/description may be a plain string or a { th, en } pair.
export function applySeo(seo, path) {
  const origin = window.location.origin
  const title = pick(seo?.title, document.title)
  const description = pick(
    seo?.description,
    document.querySelector('meta[name="description"]')?.content ?? '',
  )

  document.title = title
  document.documentElement.lang = locale.lang
  setMeta('name', 'description', description)
  setMeta('name', 'robots', seo?.noindex ? 'noindex, noarchive' : 'index, follow')

  const url = `${origin}${path}`
  const image = `${origin}${seo?.image ?? '/og-card.png'}`
  setLink('canonical', url)
  setMeta('property', 'og:title', title)
  setMeta('property', 'og:description', description)
  setMeta('property', 'og:url', url)
  setMeta('property', 'og:image', image)
  setMeta('name', 'twitter:title', title)
  setMeta('name', 'twitter:description', description)
  setMeta('name', 'twitter:image', image)
}
