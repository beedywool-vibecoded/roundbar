<script setup lang="ts">
// One option as an M3 list row with a switch: the whole row toggles it (same as RightsEditor)
import M3Switch from './M3Switch.vue';
import Icon from './Icon.vue';

const on = defineModel<boolean>({ required: true });
defineProps<{ label: string; hint?: string; icon?: string; disabled?: boolean }>();
</script>

<template>
    <label class="sw st" :class="{ off: disabled }">
        <Icon v-if="icon" :name="icon" :size="20" />
        <span class="grow">
            <span class="t">{{ label }}</span>
            <span v-if="hint" class="muted small">{{ hint }}</span>
        </span>
        <M3Switch v-model="on" :label="label" :disabled="disabled" />
    </label>
</template>

<style scoped>
.sw {
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: 64px;
    padding: 8px 14px;
    /* Corners come from a .linked list around it; alone, a 16 px row */
    border-radius: var(--row-radius, 16px);
    background: var(--sch);
    color: var(--on-sv);
    cursor: pointer;
}
.sw.off {
    cursor: default;
    opacity: 0.6;
}
.grow {
    flex: 1;
    min-width: 0;
}
.grow > * {
    display: block;
}
.t {
    color: var(--on-surface);
    font-size: 15px;
}
</style>
