<script setup>
// Server Boosts: the admin-authored payout events, one section per kind —
// an EXP multiplier and a coin multiplier, each independently configured and
// armed. Cards report every mutation up so the portal header's active strip
// refreshes in step. Internal tool — English-only by design.
import AdminBoostCard from './AdminBoostCard.vue'

defineEmits(['changed'])

const SECTIONS = [
  {
    kind: 'exp',
    title: 'EXP boost',
    blurb:
      'Multiplies the EXP every solved hand banks — solo play and multiplayer round wins alike. ' +
      'Achievement rewards and coin payouts are unaffected.',
  },
  {
    kind: 'coins',
    title: 'Coin boost',
    blurb:
      'Multiplies the coins every solved hand pays. Guests never bank anything, and the EXP ' +
      'payout is unaffected. Both boosts can run at the same time.',
  },
]
</script>

<template>
  <div class="sb">
    <AdminBoostCard v-for="s in SECTIONS" :key="s.kind" v-bind="s" @changed="$emit('changed')" />
  </div>
</template>

<style scoped>
.sb { display: flex; flex-direction: column; gap: 16px; }
</style>
