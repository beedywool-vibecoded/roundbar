<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from './components/AppShell.vue'
import AppSnackbar from './components/AppSnackbar.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ShapeLoader from './components/ShapeLoader.vue'
import M3Shape from './components/M3Shape.vue'
import Icon from './components/Icon.vue'
import { authProblem } from './lib/session'
import { config, loadAll, loadError, uiState } from './lib/store'

const route = useRoute()
const router = useRouter()
const ready = computed(() => route.matched.length > 0)
// First start: open the setup once the config is in, unless a link asked for a given page.
const stopFirstRun = watch(
  () => [config.value, uiState.value, ready.value] as const,
  ([c, ui, r]) => {
    if (!c || !ui || !r) return
    stopFirstRun()
    if (!ui.setupDone && route.path === '/') void router.replace('/setup')
  },
)
// The page editor keeps its state while switching buttons; it changes key per page.
const pageKey = computed(() => route.path)
</script>

<template>
  <div v-if="authProblem" class="gate">
    <M3Shape kind="cookie" :size="96" fill="var(--ec)" />
    <h1>{{ authProblem === 'missing' ? 'Open the configurator from the terminal' : 'This session has ended' }}</h1>
    <p class="muted">
      Run <code>roundbar config</code>: it starts the configurator and opens it with a one-time key in the address.
      {{ authProblem === 'expired' ? 'The key in this tab is no longer valid, probably because the configurator was restarted.' : '' }}
    </p>
  </div>
  <AppShell v-else-if="ready">
    <div v-if="loadError && !config" class="card empty">
      <Icon name="error" :size="32" />
      <h2>Could not load the config</h2>
      <p class="muted" style="margin: 0">{{ loadError }}</p>
      <button class="btn btn-filled st" type="button" @click="loadAll">Try again</button>
    </div>
    <div v-else-if="!config" class="loading-block"><ShapeLoader /></div>
    <RouterView v-else v-slot="{ Component }">
      <Transition name="page" mode="out-in">
        <component :is="Component" :key="pageKey" />
      </Transition>
    </RouterView>
  </AppShell>
  <AppSnackbar />
  <ConfirmDialog />
</template>

<style scoped>
.gate {
  min-height: 100vh;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 16px;
  padding: 24px;
  text-align: center;
}
.gate h1 {
  font-size: 28px;
}
.gate p {
  max-width: 520px;
  margin: 0;
}
</style>
