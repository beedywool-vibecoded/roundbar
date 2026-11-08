<script setup lang="ts">
// M3 filled text field with an optional character count and error line.
const model = defineModel<string>({ required: true })
defineProps<{
  label: string
  placeholder?: string
  max?: number
  error?: string | null
  help?: string
  mono?: boolean
  multiline?: boolean
  /** Input type, e.g. "password"; default text */
  type?: string
}>()
</script>

<template>
  <div class="tf-wrap">
    <label class="tf" :class="{ invalid: !!error }">
      <span class="lbl">
        <span>{{ label }}</span>
        <span v-if="max" class="count">{{ model.length }} / {{ max }}</span>
      </span>
      <span class="in">
        <slot name="leading" />
        <textarea v-if="multiline" v-model="model" rows="2" :placeholder="placeholder" :style="mono ? { fontFamily: 'var(--mono)', fontSize: '14px' } : undefined" spellcheck="false" />
        <input v-else v-model="model" :type="type ?? 'text'" :placeholder="placeholder" :style="mono ? { fontFamily: 'var(--mono)', fontSize: '14px' } : undefined" spellcheck="false" autocomplete="off" />
        <slot name="trailing" />
      </span>
    </label>
    <p v-if="error" class="tf-help err">{{ error }}</p>
    <p v-else-if="help" class="tf-help">{{ help }}</p>
  </div>
</template>

<style scoped>
.tf-wrap {
  min-width: 0;
}
</style>
