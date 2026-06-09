<template>
  <el-dialog v-model="visible" title="新建角色" class="dialog-form dialog-create-role" append-to-body>
    <el-form label-width="100px">
      <el-form-item label="角色名称">
        <el-input v-model="name" placeholder="请输入角色名称" maxlength="32" show-word-limit />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="submit">
        <el-icon><Check /></el-icon>
        创建
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { Check } from '@element-plus/icons-vue'

const visible = defineModel<boolean>('visible', { required: true })
const name = defineModel<string>('name', { required: true })

const emit = defineEmits<{
  create: []
}>()

function submit() {
  emit('create')
}
</script>
