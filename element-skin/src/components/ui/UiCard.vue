<template>
  <el-card
    v-bind="forwardedAttrs"
    :shadow="shadow"
    :class="rootClass"
  >
    <template v-if="$slots.header" #header>
      <slot name="header" />
    </template>
    <slot />
  </el-card>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    hoverable?: boolean
    shadow?: 'always' | 'hover' | 'never'
  }>(),
  {
    hoverable: false,
    shadow: 'never',
  },
)

const attrs = useAttrs()

const forwardedAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})

const rootClass = computed(() => ['ui-card', attrs.class, { 'is-hoverable': props.hoverable }])
</script>

<style scoped>
.ui-card {
  border-radius: 16px;
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  background: var(--color-card-background);
  transition:
    background-color 0.3s ease,
    border-color 0.3s ease,
    transform 0.3s ease,
    box-shadow 0.3s ease;
  overflow: hidden;
}

.ui-card.is-hoverable:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}
</style>
