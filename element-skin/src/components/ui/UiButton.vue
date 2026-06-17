<template>
  <el-button
    v-bind="forwardedAttrs"
    :class="rootClass"
  >
    <template v-if="iconSwap">
      <span class="ui-button-label"><slot /></span>
      <span class="ui-button-icon"><slot name="icon" /></span>
    </template>
    <template v-else>
      <slot />
    </template>
  </el-button>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?:
      | 'default'
      | 'gradient-primary'
      | 'gradient-success'
      | 'gradient-warning'
      | 'gradient-danger'
      | 'soft-warning'
      | 'outline'
      | 'outline-danger'
    iconSwap?: boolean
  }>(),
  {
    variant: 'default',
    iconSwap: false,
  },
)

const variantClass = computed(() => `ui-button--${props.variant}`)

defineOptions({ inheritAttrs: false })

const attrs = useAttrs()

const forwardedAttrs = computed(() => {
  const rest = { ...attrs }
  delete rest.class
  return rest
})

const rootClass = computed(() => [
  'ui-button',
  variantClass.value,
  attrs.class,
  { 'is-icon-swap': props.iconSwap },
])
</script>

<style scoped>
.ui-button {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.ui-button--gradient-primary,
.ui-button--gradient-success,
.ui-button--gradient-warning,
.ui-button--gradient-danger {
  border: none !important;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #fff !important;
  cursor: pointer;
}

.ui-button--gradient-primary:hover:not(:disabled),
.ui-button--gradient-success:hover:not(:disabled),
.ui-button--gradient-warning:hover:not(:disabled),
.ui-button--gradient-danger:hover:not(:disabled),
.ui-button--soft-warning:hover,
.ui-button--outline:hover {
  transform: translateY(-2px);
}

.ui-button--gradient-primary {
  background: linear-gradient(135deg, #409eff 0%, #5cadff 100%) !important;
}

.ui-button--gradient-primary:hover:not(:disabled) {
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.4);
}

.ui-button--gradient-success {
  background: linear-gradient(135deg, #67c23a 0%, #85ce61 100%) !important;
}

.ui-button--gradient-success:hover:not(:disabled) {
  box-shadow: 0 6px 20px rgba(103, 194, 58, 0.4);
}

.ui-button--gradient-warning {
  background: linear-gradient(135deg, #e6a23c 0%, #eebe77 100%) !important;
}

.ui-button--gradient-warning:hover:not(:disabled) {
  box-shadow: 0 6px 20px rgba(230, 162, 60, 0.4);
}

.ui-button--gradient-danger {
  background: linear-gradient(135deg, #f56c6c 0%, #f78989 100%) !important;
}

.ui-button--gradient-danger:hover:not(:disabled) {
  box-shadow: 0 6px 16px rgba(245, 108, 108, 0.25);
}

.ui-button--soft-warning {
  color: var(--color-text);
  border: 1px solid rgba(230, 162, 60, 0.3) !important;
  background: rgba(230, 162, 60, 0.1) !important;
  transition: all 0.25s ease;
}

.ui-button--soft-warning:hover {
  color: #fff !important;
  background: linear-gradient(135deg, #ffa726 0%, #fb8c00 100%) !important;
  border-color: transparent !important;
  box-shadow: 0 6px 16px rgba(251, 140, 0, 0.18);
}

.ui-button--outline,
.ui-button--outline-danger {
  background: var(--color-card-background);
  border: 1px solid var(--color-border);
  color: var(--color-text);
  border-radius: 8px;
  padding: 8px 16px;
}

.ui-button--outline:hover {
  background: var(--color-background-soft);
  border-color: var(--el-color-primary);
  color: var(--el-color-primary);
}

.ui-button--outline-danger {
  border-color: rgba(245, 108, 108, 0.4);
}

.ui-button--outline-danger:hover {
  background: #fef0f0;
  border-color: #f56c6c;
  color: #f56c6c !important;
}

html.dark .ui-button--outline-danger:hover {
  background: rgba(245, 108, 108, 0.15) !important;
  color: #f56c6c !important;
}

.is-icon-swap {
  position: relative;
  overflow: hidden;
}

.ui-button-label {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  transition:
    transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1),
    opacity 0.3s;
}

.ui-button-icon {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  transform: translateY(100%) rotate(-90deg);
  opacity: 0;
  transition:
    transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1),
    opacity 0.3s;
  font-size: 16px;
  pointer-events: none;
}

.is-icon-swap:hover .ui-button-label {
  transform: translateY(-100%) scale(0.8);
  opacity: 0;
}

.is-icon-swap:hover .ui-button-icon {
  transform: translateY(0) rotate(0deg);
  opacity: 1;
}
</style>
