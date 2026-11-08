<script setup lang="ts">
// Material Symbols Rounded from the self-hosted icon font: the same glyphs the device draws.
// `name` is a ligature, e.g. "play_arrow". `fill` switches the FILL axis.
withDefaults(defineProps<{ name: string; size?: number; fill?: boolean }>(), { size: 22, fill: false })
</script>

<template>
  <span
    class="msr"
    aria-hidden="true"
    :style="{ fontSize: `${size}px`, fontVariationSettings: `'FILL' ${fill ? 1 : 0}, 'opsz' ${Math.min(48, Math.max(20, size))}` }"
    >{{ name }}</span
  >
</template>

<style scoped>
.msr {
  display: inline-block;
  flex-shrink: 0;
  width: 1em;
  height: 1em;
  overflow: hidden;
  font-family: 'Material Symbols Rounded';
  font-weight: normal;
  font-style: normal;
  line-height: 1;
  letter-spacing: normal;
  text-transform: none;
  white-space: nowrap;
  word-wrap: normal;
  direction: ltr;
  -webkit-font-smoothing: antialiased;
  font-feature-settings: 'liga';
  user-select: none;
}
</style>
