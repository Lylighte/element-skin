<template>
  <el-radio-group v-model="model" v-bind="forwardedAttrs" :class="rootClass">
    <slot />
  </el-radio-group>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'

defineOptions({ inheritAttrs: false })

const model = defineModel<string | number | boolean | null | undefined>({ required: true })

const props = withDefaults(
  defineProps<{
    variant?: 'capsule' | 'modern'
  }>(),
  {
    variant: 'capsule',
  },
)

const attrs = useAttrs()

const forwardedAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})

const rootClass = computed(() => ['ui-segmented', `ui-segmented--${props.variant}`, attrs.class])
</script>

<style>
.ui-segmented--capsule .el-radio-button__inner {
  background: var(--color-card-background);
  color: var(--color-text);
  border: 1px solid var(--color-border) !important;
  font-weight: 500;
  transition: var(--transition-base);
}

.ui-segmented--capsule .el-radio-button.is-active .el-radio-button__inner {
  background-color: var(--el-color-primary) !important;
  border-color: var(--el-color-primary) !important;
  color: #fff !important;
}

.ui-segmented--modern .el-radio-button__inner {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 30px;
  border-radius: 8px !important;
  margin-right: 12px;
  border: 1px solid var(--color-border) !important;
  background: var(--color-card-background);
  color: var(--color-text);
  box-shadow: none !important;
  transition: var(--transition-base);
}

.ui-segmented--modern .el-radio-button.is-active .el-radio-button__inner {
  background-color: rgba(64, 158, 255, 0.1) !important;
  color: var(--el-color-primary) !important;
  border-color: var(--el-color-primary) !important;
}

html.dark .ui-segmented .el-radio-button__inner {
  background: #1d1d1d !important;
  color: var(--color-text);
  border-color: var(--color-border) !important;
}

html.dark .ui-segmented--modern .el-radio-button.is-active .el-radio-button__inner {
  background-color: rgba(64, 158, 255, 0.2) !important;
  color: #409eff !important;
}
</style>
