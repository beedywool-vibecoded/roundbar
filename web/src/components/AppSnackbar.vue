<script setup lang="ts">
import { ref, watch } from 'vue'
import { snackbar } from '@/lib/ui'

const visible = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
watch(
  () => snackbar.id,
  () => {
    visible.value = true
    clearTimeout(timer)
    timer = setTimeout(() => (visible.value = false), snackbar.error || snackbar.action ? 7000 : 4000)
  },
)
function act() {
  const a = snackbar.action
  snackbar.action = null
  visible.value = false
  a?.run()
}
</script>

<template>
  <div class="snackbar" :class="{ visible, error: snackbar.error }" role="status" aria-live="polite">
    <span class="grow">{{ snackbar.text }}</span>
    <button v-if="snackbar.action" class="btn btn-text st act" type="button" @click="act">{{ snackbar.action.label }}</button>
    <button class="btn btn-text st" type="button" @click="visible = false">Dismiss</button>
  </div>
</template>

<style scoped>
.snackbar {
  position: fixed;
  left: 50%;
  bottom: 24px;
  z-index: 50;
  display: flex;
  align-items: center;
  gap: 12px;
  width: min(560px, calc(100vw - 32px));
  min-height: 48px;
  padding: 6px 8px 6px 18px;
  border-radius: 12px;
  background: var(--on-surface);
  color: var(--surface);
  font-size: 14px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  translate: -50% 140%;
  opacity: 0;
  transition: translate 0.35s var(--ease-decel), opacity 0.2s;
  pointer-events: none;
}
.snackbar.visible {
  translate: -50% 0;
  opacity: 1;
  pointer-events: auto;
}
.snackbar.error {
  background: var(--ec);
  color: var(--on-ec);
}
.snackbar .btn-text {
  color: inherit;
}
.snackbar .act {
  /* inverse primary: readable on the inverted surface in both modes */
  color: var(--pc);
  font-weight: 600;
}
.snackbar.error .act {
  color: inherit;
}
@media (max-width: 760px) {
  .snackbar {
    bottom: 96px;
  }
}
</style>
