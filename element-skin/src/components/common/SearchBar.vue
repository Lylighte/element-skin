<template>
  <div class="search-bar-container">
    <el-input
      v-model="query"
      :placeholder="placeholder"
      clearable
      :size="size"
      @clear="$emit('clear')"
      @keyup.enter="$emit('search')"
    >
      <template #prefix>
        <el-icon><Search /></el-icon>
      </template>
      <template #append>
        <el-button :icon="Search" @click="$emit('search')">搜索</el-button>
      </template>
    </el-input>
  </div>
</template>

<script setup lang="ts">
import { Search } from '@element-plus/icons-vue'

const query = defineModel<string>({ required: true })

withDefaults(defineProps<{
  placeholder?: string
  size?: 'small' | 'default' | 'large'
}>(), {
  placeholder: '搜索',
  size: 'large',
})

defineEmits<{
  search: []
  clear: []
}>()
</script>

<style scoped>
.search-bar-container {
  display: flex;
  min-width: 0;
}

.search-bar-container :deep(.el-input-group) {
  display: flex;
  align-items: stretch;
}

.search-bar-container :deep(.el-input-group__append) {
  background: var(--el-color-primary);
  color: #fff;
  border-color: var(--el-color-primary);
  cursor: pointer;
  padding: 0 20px;
  display: flex;
  align-items: center;
  transition: all 0.3s ease;
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
}

.search-bar-container :deep(.el-input-group__append:hover) {
  background: var(--el-color-primary-light-3);
  border-color: var(--el-color-primary-light-3);
  opacity: 0.9;
}

.search-bar-container :deep(.el-input-group__append .el-button) {
  border: none;
  background: transparent;
  color: inherit;
  padding: 0;
  margin: 0;
  height: 100%;
}
</style>
