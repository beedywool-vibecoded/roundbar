<script setup lang="ts">
// Custom disclosure, in place of <details>/<summary> and their browser triangle.
import { ref, watch } from 'vue'
import Icon from './Icon.vue'

const props = defineProps<{ initiallyOpen?: boolean }>()
const open = ref(props.initiallyOpen ?? false)
const id = `exp-${Math.random().toString(36).slice(2, 8)}`
// Once open, the content may overflow (menus, pickers); it is clipped only
// during the animation
const settled = ref(open.value)
watch(open, () => (settled.value = false))
function onEnd(e: TransitionEvent) {
  if (e.target === e.currentTarget) settled.value = open.value
}
</script>

<template>
  <div class="expand" :class="{ open, settled }">
    <button type="button" class="head st" :aria-expanded="open" :aria-controls="id" @click="open = !open">
      <slot name="summary" />
      <Icon name="expand_more" :size="18" class="chev" />
    </button>
    <div :id="id" class="body" :inert="!open" @transitionend="onEnd">
      <div class="inner"><slot /></div>
    </div>
  </div>
</template>

<style scoped>
.head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  border: none;
  background: transparent;
  color: inherit;
  text-align: left;
  font: inherit;
  padding: 6px 10px;
  border-radius: 10px;
}
.chev {
  flex-shrink: 0;
  margin-left: auto;
  transition: rotate 0.25s var(--ease);
}
/* Animates to the content height without measuring it. Use the direct parent, or a nested
   disclosure inherits the outer one's open state. */
.body {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.3s var(--ease-decel);
}
.expand.open > .body {
  grid-template-rows: 1fr;
}
.expand.open > .head .chev {
  rotate: 180deg;
}
.inner {
  overflow: hidden;
  min-height: 0;
  /* A grid item: without this, wide content (button groups) widens the whole section */
  min-width: 0;
}
.expand.open.settled > .body > .inner {
  overflow: visible;
}
</style>
