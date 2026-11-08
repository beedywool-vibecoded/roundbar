<script setup lang="ts">
const on = defineModel<boolean>({ required: true })
defineProps<{ label: string; disabled?: boolean }>()
</script>

<template>
  <button
    type="button"
    role="switch"
    class="switch"
    :class="{ on }"
    :aria-checked="on"
    :aria-label="label"
    :disabled="disabled"
    @click="on = !on"
  >
    <span class="knob" />
  </button>
</template>

<style scoped>
.switch {
  position: relative;
  width: 52px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 16px;
  border: 2px solid var(--outline);
  background: var(--schh);
  padding: 0;
  transition: background 0.2s var(--ease), border-color 0.2s var(--ease);
}
.knob {
  position: absolute;
  top: 50%;
  left: 6px;
  width: 16px;
  height: 16px;
  border-radius: 12px;
  background: var(--outline);
  translate: 0 -50%;
  transition: all 0.25s var(--ease-decel);
}
.switch.on {
  background: var(--primary);
  border-color: var(--primary);
}
.switch.on .knob {
  left: 22px;
  width: 24px;
  height: 24px;
  background: var(--on-primary);
}
</style>
