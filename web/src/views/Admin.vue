<script setup>
// Admin portal shell: admins sign in with their regular Google account —
// access is granted by the server's ADMIN_EMAILS allowlist, not a separate
// password. Internal tool — English-only by design, unlike the game UI.
import { computed, onMounted, provide, ref } from 'vue'
import { adminApi } from '../admin.js'
import { currentPlayer, clearSession, getPlayer } from '../auth.js'
import GoogleSignIn from '../components/GoogleSignIn.vue'
import AdminOverview from '../components/admin/AdminOverview.vue'
import AdminPlayers from '../components/admin/AdminPlayers.vue'
import AdminLeaderboards from '../components/admin/AdminLeaderboards.vue'
import AdminRounds from '../components/admin/AdminRounds.vue'
import AdminEvents from '../components/admin/AdminEvents.vue'
import AdminSettings from '../components/admin/AdminSettings.vue'

const authed = ref(null) // null = validating, false = gate, true = portal
const checking = ref(false)
const gateMsg = ref('')
const adminName = ref('')
const tab = ref('overview')

const tabs = [
  { id: 'overview', label: 'Overview' },
  { id: 'players', label: 'Players' },
  { id: 'boards', label: 'Leaderboards' },
  { id: 'rounds', label: 'Rounds' },
  { id: 'events', label: 'Event Log' },
  { id: 'settings', label: 'Settings' },
]

const signedInAs = computed(() => currentPlayer.value?.nickname ?? '')

async function validate() {
  if (!getPlayer()) {
    // no player session at all — nothing to test against the allowlist
    authed.value = false
    gateMsg.value = ''
    return
  }
  checking.value = true
  try {
    const data = await adminApi.session()
    adminName.value = data.admin?.nickname ?? ''
    authed.value = true
  } catch (err) {
    authed.value = false
    gateMsg.value =
      err.status === 404
        ? 'Admin portal disabled on this server (set ADMIN_EMAILS).'
        : `“${signedInAs.value}” is not an admin account — sign in with an allowlisted Google account.`
  } finally {
    checking.value = false
  }
}

function onSignedIn() {
  validate()
}

function logout() {
  clearSession()
  adminName.value = ''
  authed.value = false
  gateMsg.value = ''
}

// children call this when the API answers 401/403 (token revoked mid-session)
function onAuthFail(err) {
  if (err?.status === 401 || err?.status === 403) logout()
}

provide('onAdminAuthFail', onAuthFail)

onMounted(validate)
</script>

<template>
  <main class="wrap admin">
    <!-- gate: sign in with an allowlisted Google account -->
    <section v-if="authed === false" class="panel login">
      <div class="login-head">
        <h1>Admin Portal</h1>
        <p class="sub">
          Admin access is granted by allowlist — sign in with the Google account your server
          lists in <code>ADMIN_EMAILS</code>.
        </p>
      </div>
      <p v-if="gateMsg" class="note">{{ gateMsg }}</p>
      <div class="gsi">
        <GoogleSignIn @signed-in="onSignedIn" />
      </div>
      <router-link class="backlink" to="/">← back to the game</router-link>
    </section>

    <p v-else-if="authed === null || checking" class="checking">Checking admin session…</p>

    <!-- portal -->
    <template v-else>
      <header class="top">
        <h1>Admin Portal</h1>
        <span class="badge">game24</span>
        <span class="who">{{ adminName }}</span>
        <div class="spacer" />
        <router-link class="btn quiet" to="/">Game</router-link>
        <button class="btn quiet" @click="logout">Sign out</button>
      </header>

      <nav class="tabs">
        <button
          v-for="t in tabs"
          :key="t.id"
          class="tab"
          :class="{ on: tab === t.id }"
          @click="tab = t.id"
        >
          {{ t.label }}
        </button>
      </nav>

      <AdminOverview v-if="tab === 'overview'" />
      <AdminPlayers v-else-if="tab === 'players'" />
      <AdminLeaderboards v-else-if="tab === 'boards'" />
      <AdminRounds v-else-if="tab === 'rounds'" />
      <AdminEvents v-else-if="tab === 'events'" />
      <AdminSettings v-else-if="tab === 'settings'" />
    </template>
  </main>
</template>

<style scoped>
.admin { display: flex; flex-direction: column; gap: 18px; padding: 26px 0 60px; }

.login { max-width: 440px; margin: 8vh auto 0; padding: 34px 32px; display: flex; flex-direction: column; gap: 18px; }
.login-head { display: flex; flex-direction: column; gap: 8px; }
.login-head h1 { font-size: 1.35rem; }
.sub { color: var(--text-dim); font-size: 0.88rem; line-height: 1.5; }
.sub code { font-family: var(--font-mono); font-size: 0.8rem; color: var(--accent); }
.note { color: var(--bad); font-size: 0.85rem; line-height: 1.5; }
.gsi { display: grid; place-items: center; min-height: 44px; }
.backlink { color: var(--text-mute); font-size: 0.82rem; text-decoration: none; }
.backlink:hover { color: var(--text-dim); }
.checking { text-align: center; color: var(--text-mute); padding: 40px; }

.top { display: flex; align-items: center; gap: 12px; }
.top h1 { font-size: 1.3rem; }
.spacer { flex: 1; }
.who { color: var(--text-dim); font-size: 0.85rem; }
.badge {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
  background: var(--accent-soft);
  border: 1px solid rgba(246, 183, 60, 0.3);
  border-radius: var(--r-full);
  padding: 3px 10px;
}

.tabs { display: flex; gap: 6px; flex-wrap: wrap; border-bottom: 1px solid var(--line); padding-bottom: 0; }
.tab {
  cursor: pointer;
  border: none;
  background: transparent;
  color: var(--text-mute);
  font: inherit;
  font-size: 0.9rem;
  padding: 10px 16px 12px;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  transition: color 0.15s var(--ease), border-color 0.15s var(--ease);
}
.tab:hover { color: var(--text); }
.tab.on { color: var(--accent); border-bottom-color: var(--accent); }
</style>
