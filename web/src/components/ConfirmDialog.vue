<script setup lang="ts">
import { computed } from 'vue'
import M3Dialog from './M3Dialog.vue'
import { confirmState, settleConfirm } from '@/lib/ui'

const open = computed({
  get: () => confirmState.open,
  set: (v) => !v && settleConfirm(false),
})
</script>

<template>
  <M3Dialog v-model="open" :title="confirmState.req?.title ?? ''">
    <p class="muted" style="margin: 0">{{ confirmState.req?.body }}</p>
    <template #actions>
      <button class="btn btn-text st" type="button" @click="settleConfirm(false)">Cancel</button>
      <button
        class="btn st"
        :class="confirmState.req?.danger ? 'btn-danger' : 'btn-filled'"
        type="button"
        @click="settleConfirm(true)"
      >
        {{ confirmState.req?.action }}
      </button>
    </template>
  </M3Dialog>
</template>
