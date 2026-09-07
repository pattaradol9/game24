<script setup>
// Shared renderer for the /privacy and /terms documents.
// The copy itself lives in i18n/legal.js (bilingual, structured sections).
import { computed } from 'vue'
import { useI18n } from '../i18n/index.js'
import { legalDoc, OPERATOR } from '../i18n/legal.js'
import Brand from '../components/Brand.vue'
import Icon from '../components/Icon.vue'

const props = defineProps({
  doc: { type: String, required: true }, // 'privacy' | 'terms'
})

const { t, lang, toggle } = useI18n()

const title = computed(() => t(props.doc))
const sections = computed(() => legalDoc(props.doc, lang.value))
</script>

<template>
  <main class="wrap page">
    <header class="top">
      <button class="btn back" @click="$router.push('/')">
        <Icon name="back" :size="18" /><span class="back-label">{{ t('exit') }}</span>
      </button>
      <Brand size="sm" />
      <div class="spacer" />
      <button class="btn icon lang" :aria-label="lang === 'th' ? 'English' : 'ไทย'" @click="toggle">
        {{ lang === 'th' ? 'EN' : 'TH' }}
      </button>
    </header>

    <article class="panel body">
      <h1>{{ title }}</h1>
      <p class="updated num">{{ t('legalUpdated') }}: {{ OPERATOR.updated }}</p>

      <section v-for="s in sections" :key="s.title">
        <h2>{{ s.title }}</h2>
        <template v-for="(item, i) in s.body" :key="i">
          <p v-if="item.p">{{ item.p }}</p>
          <ul v-else-if="item.ul">
            <li v-for="line in item.ul" :key="line">{{ line }}</li>
          </ul>
        </template>
      </section>
    </article>
  </main>
</template>

<style scoped>
.page { display: flex; flex-direction: column; gap: 20px; padding: 20px 0 56px; }
.top { display: flex; align-items: center; gap: 12px; }
.spacer { flex: 1; }
.back { padding: 12px 16px 12px 13px; gap: 7px; color: var(--text-dim); }
@media (hover: hover) { .back:hover { color: var(--text); } }
.lang { color: var(--text-dim); font-size: 0.85rem; font-weight: 500; }

.body { display: flex; flex-direction: column; gap: 22px; padding: 30px 34px 36px; }
h1 { font-size: 1.4rem; font-weight: 600; }
.updated { font-size: 0.8rem; color: var(--text-mute); margin-top: -14px; }

section { display: flex; flex-direction: column; gap: 10px; }
h2 {
  font-size: 1.02rem;
  font-weight: 600;
  color: var(--accent);
  letter-spacing: -0.01em;
}
p, li { font-size: 0.92rem; line-height: 1.75; color: var(--text-dim); }
li::marker { color: var(--text-mute); }
ul { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 6px; }
strong, b { color: var(--text); }

@media (max-width: 560px) {
  .back-label { display: none; }
  .back { padding: 0; width: 44px; justify-content: center; }
  .body { padding: 22px 18px 26px; }
  .updated { margin-top: -12px; }
}
</style>
