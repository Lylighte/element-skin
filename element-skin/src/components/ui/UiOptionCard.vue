<template>
  <component :is="as" v-bind="$attrs" class="ui-option-card">
    <slot />
  </component>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    as?: string
  }>(),
  {
    as: 'div',
  },
)
</script>

<style scoped>
.ui-option-card {
  width: 100%;
  margin-right: 0 !important;
  height: auto !important;
  padding: 12px 16px !important;
  display: flex !important;
  align-items: center;
  border-radius: 8px !important;
  transition: var(--transition-base) !important;
  border: 1px solid var(--color-border) !important;
  background: var(--color-card-background) !important;
}

.ui-option-card:hover {
  border-color: var(--el-color-primary) !important;
  background: var(--color-background-soft) !important;
}

.ui-option-card.is-checked {
  background: rgba(64, 158, 255, 0.05) !important;
  border-color: var(--el-color-primary) !important;
}

:deep(.ui-option-card__info) {
  display: flex;
  flex-direction: column;
  text-align: left;
  gap: 2px;
}

:deep(.ui-option-card__title) {
  font-weight: 600;
  font-size: 15px;
  color: var(--color-heading);
}

:deep(.ui-option-card__subtitle) {
  font-size: 12px;
  color: var(--color-text-light);
  font-family: monospace;
}
</style>
