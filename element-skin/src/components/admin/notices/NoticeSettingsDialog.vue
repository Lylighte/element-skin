<template>
  <UiDialog
    :model-value="modelValue"
    title="发布设置"
    variant="wide-form"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="max-h-[72vh] overflow-auto p-6">
      <el-form label-position="top">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <el-form-item label="级别">
            <el-select v-model="form.level">
              <el-option label="普通" value="info" />
              <el-option label="成功" value="success" />
              <el-option label="重要" value="warning" />
              <el-option label="紧急" value="danger" />
            </el-select>
          </el-form-item>
          <el-form-item label="可见人群">
            <el-select v-model="form.audience">
              <el-option label="所有用户" value="users" />
              <el-option label="管理员" value="admins" />
            </el-select>
          </el-form-item>
          <el-form-item label="控制项">
            <div class="flex h-8 flex-wrap items-center gap-4">
              <el-checkbox v-model="form.enabled">启用</el-checkbox>
              <el-checkbox v-model="form.pinned">置顶</el-checkbox>
              <el-checkbox v-model="form.dismissible">可忽略</el-checkbox>
            </div>
          </el-form-item>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <el-form-item label="生效时间">
            <el-date-picker
              v-model="form.starts_at"
              type="datetime"
              value-format="x"
              clearable
              placeholder="立即生效"
              class="w-full"
            />
          </el-form-item>
          <el-form-item label="过期时间">
            <el-date-picker
              v-model="form.ends_at"
              type="datetime"
              value-format="x"
              clearable
              placeholder="无期限"
              class="w-full"
            />
          </el-form-item>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <el-form-item label="链接文字">
            <el-input v-model="form.link_text" maxlength="40" />
          </el-form-item>
          <el-form-item label="链接地址">
            <el-input
              v-model="form.link_url"
              maxlength="512"
              placeholder="/oauth/apps 或 https://..."
            />
          </el-form-item>
        </div>
      </el-form>
    </div>

    <template #footer>
      <div class="px-6 pb-6">
        <el-button @click="emit('back')">返回编辑</el-button>
        <el-button @click="emit('update:modelValue', false)">取消</el-button>
        <el-button type="primary" :loading="saving" @click="emit('save')">
          {{ createMode ? '创建公告' : '保存' }}
        </el-button>
      </div>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import UiDialog from '@/components/ui/UiDialog.vue'
import type { NoticeDraft } from '@/api/admin/notices'

defineProps<{
  modelValue: boolean
  createMode: boolean
  saving: boolean
  form: NoticeDraft
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  back: []
  save: []
}>()
</script>
