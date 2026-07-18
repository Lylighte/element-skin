<template>
  <UiDialog v-model="open" title="上传纹理" class="texture-upload-panel">
    <el-form label-width="100px" :model="form" class="upload-form">
      <el-form-item label="选择文件" class="upload-form-item">
        <el-upload
          ref="uploadRef"
          :auto-upload="false"
          :limit="1"
          accept=".png"
          :on-change="handleFileChange"
          drag
          class="upload-wrapper"
        >
          <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
          <div class="el-upload__text">将 PNG 文件拖到此处，或<em>点击上传</em></div>
          <template #tip>
            <div class="el-upload__tip">仅支持 PNG 格式的皮肤文件</div>
          </template>
        </el-upload>
      </el-form-item>
      <el-form-item label="纹理类型">
        <el-select v-model="form.texture_type" placeholder="选择类型" class="w-full">
          <el-option label="皮肤 (Skin)" value="skin" />
          <el-option label="披风 (Cape)" value="cape" />
        </el-select>
      </el-form-item>
      <el-form-item label="皮肤模型" v-if="form.texture_type === 'skin'">
        <el-select v-model="form.model" placeholder="选择模型" class="w-full">
          <el-option label="普通 (4px 手臂)" value="default" />
          <el-option label="纤细 (3px 手臂)" value="slim" />
        </el-select>
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.note" placeholder="给这个纹理添加备注（可选）" />
      </el-form-item>
      <el-form-item label="是否公开">
        <el-switch v-model="form.is_public" />
        <el-text size="small" type="info" class="ml-3">
          公开后其他用户可以在皮肤库中看到并使用
        </el-text>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="open = false">取消</el-button>
      <el-button type="primary" @click="emit('submit')">
        <el-icon><Upload /></el-icon>
        确认上传
      </el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Upload, UploadFilled } from '@element-plus/icons-vue'
import type { UploadFile, UploadInstance } from 'element-plus'

import UiDialog from '@/components/ui/UiDialog.vue'
import type { TextureUploadForm } from '@/components/dashboard/wardrobe/uploadForm'

const open = defineModel<boolean>({ required: true })
const form = defineModel<TextureUploadForm>('form', { required: true })

const emit = defineEmits<{
  fileChange: [file: UploadFile]
  submit: []
}>()

const uploadRef = ref<UploadInstance | null>(null)

function handleFileChange(file: UploadFile) {
  emit('fileChange', file)
}

function clearFiles() {
  uploadRef.value?.clearFiles()
}

defineExpose({ clearFiles })
</script>

<style scoped>
.texture-upload-panel :deep(.el-upload-dragger) {
  width: 100%;
}

.upload-wrapper {
  width: 100%;
}
</style>
