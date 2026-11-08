<script setup lang="ts">
import { computed } from 'vue'
import { shapePath, type ShapeKind } from '@/lib/shapes'

const props = withDefaults(defineProps<{ kind: ShapeKind; size?: number | string; fill?: string }>(), {
  size: 48,
  fill: 'currentColor',
})
const d = computed(() => shapePath(props.kind))
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 100 100" aria-hidden="true">
    <path :d="d" :fill="fill" />
    <slot />
  </svg>
</template>
