<script setup>
// Renders a translated string, swapping every {arrow} token for an inline
// SVG arrow so copy like "number {arrow} operator {arrow} number" never
// falls back to the plain-text glyph.
import { computed } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  text: { type: String, required: true },
  size: { type: [Number, String], default: 13 },
})

const parts = computed(() => props.text.split('{arrow}'))
</script>

<template>
  <span class="arrow-text"
    ><template v-for="(part, i) in parts" :key="i"
      ><span>{{ part }}</span
      ><span v-if="i < parts.length - 1" class="sep" aria-hidden="true">
        <Icon name="arrow-right" :size="size" />
      </span></template
    ></span
  >
</template>

<style scoped>
.arrow-text { display: inline; }
.sep {
  display: inline-block;
  vertical-align: -0.14em;
  margin-inline: 0.18em;
  color: var(--accent);
}
</style>
