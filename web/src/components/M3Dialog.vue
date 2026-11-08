<script setup lang="ts">
// Native <dialog>: focus trap, Escape and the top layer for free.
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { lockScroll, unlockScroll } from '@/lib/scrollLock'

const open = defineModel<boolean>({ required: true })
defineProps<{ title: string; wide?: boolean }>()
const el = ref<HTMLDialogElement>()

let locked = false
function sync() {
  if (!el.value) return
  if (open.value && !el.value.open) el.value.showModal()
  if (!open.value && el.value.open) el.value.close()
  if (open.value && !locked) {
    lockScroll()
    locked = true
  } else if (!open.value && locked) {
    unlockScroll()
    locked = false
  }
}
watch(open, sync)
onMounted(sync)
onBeforeUnmount(() => locked && unlockScroll())
</script>

<template>
  <dialog ref="el" :class="{ wide }" :aria-label="title" @close="open = false" @click.self="open = false">
    <div class="body">
      <header>
        <h2>{{ title }}</h2>
        <slot name="header" />
        <button class="icon-btn st" type="button" aria-label="Close" @click="open = false">
          <Icon name="close" :size="22" />
        </button>
      </header>
      <slot />
      <footer v-if="$slots.actions"><slot name="actions" /></footer>
    </div>
  </dialog>
</template>

<style scoped>
dialog {
  width: min(480px, calc(100vw - 32px));
  max-height: calc(100vh - 48px);
  padding: 0;
  border: none;
  border-radius: 28px;
  background: var(--sch);
  color: var(--on-surface);
}
dialog.wide {
  width: min(1100px, calc(100vw - 32px));
}
dialog::backdrop {
  background: rgba(0, 0, 0, 0.5);
}
dialog[open] {
  animation: pop 0.3s var(--ease-decel);
}
@keyframes pop {
  from {
    opacity: 0;
    scale: 0.94;
  }
}
.body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 20px 24px 24px;
  max-height: calc(100vh - 48px);
  overscroll-behavior: contain;
}
header {
  display: flex;
  align-items: center;
  gap: 12px;
}
header h2 {
  flex: 1;
  font-size: 22px;
}
/* No outline: the close button reads as a button from its icon and hover. */
header .icon-btn {
  border: none;
}
footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
