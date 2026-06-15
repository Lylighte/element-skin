<template>
  <div class="admin-homepage-media animate-fade-in">
    <PageHeader title="首页图片" subtitle="管理静态图与 Panorama 的播放顺序、时长和镜头轨迹">
      <template #icon><PictureFilled /></template>
      <template #actions>
        <div class="upload-actions">
          <el-upload action="#" :http-request="uploadImage" :show-file-list="false" accept=".png,.jpg,.jpeg,.webp">
            <el-button type="primary" :icon="Upload" size="large">上传图片</el-button>
          </el-upload>
          <el-upload action="#" :http-request="uploadPanorama" :show-file-list="false" accept=".zip">
            <el-button :icon="Box" size="large">上传 Panorama</el-button>
          </el-upload>
        </div>
      </template>
    </PageHeader>

    <div class="media-list" v-loading="loading">
      <div v-for="(item, index) in items" :key="item.id" class="surface-card media-row">
        <div class="preview">
          <el-image
            :src="previewUrl(item)"
            fit="cover"
            class="preview-image"
            :preview-src-list="[previewUrl(item)]"
            preview-teleported
          />
        </div>

        <div class="media-main">
          <div class="media-head">
            <el-tag :type="item.type === 'panorama' ? 'warning' : 'success'" size="small">
              {{ item.type === 'panorama' ? 'Panorama' : '图片' }}
            </el-tag>
            <el-input v-model="item.title" size="small" @change="saveItem(item)" />
          </div>

          <div class="controls">
            <label>
              <span>时长</span>
              <el-input-number v-model="item.duration_ms" :min="1000" :max="60000" :step="500" size="small" @change="saveItem(item)" />
            </label>
            <el-switch v-model="item.enabled" active-text="启用" inactive-text="停用" @change="saveItem(item)" />
          </div>

          <div v-if="item.type === 'panorama'" class="panorama-controls">
            <label v-for="field in panoramaFields" :key="field.key">
              <span>{{ field.label }}</span>
              <el-input-number
                v-model="item.config[field.key]"
                :min="field.min"
                :max="field.max"
                :step="1"
                size="small"
                @change="saveItem(item)"
              />
            </label>
          </div>
        </div>

        <div class="row-actions">
          <el-button :icon="ArrowUp" circle :disabled="index === 0" @click="move(index, -1)" />
          <el-button :icon="ArrowDown" circle :disabled="index === items.length - 1" @click="move(index, 1)" />
          <el-button type="danger" :icon="Delete" circle plain @click="remove(item)" />
        </div>
      </div>

      <div v-if="items.length === 0 && !loading" class="empty-placeholder">
        <el-empty description="暂无首页媒体" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadRequestOptions } from 'element-plus'
import { ArrowDown, ArrowUp, Box, Delete, PictureFilled, Upload } from '@element-plus/icons-vue'
import type { HomepageMedia } from '@/api/types'
import {
  deleteHomepageMedia,
  listHomepageMedia,
  patchHomepageMedia,
  reorderHomepageMedia,
  uploadHomepageImage,
  uploadHomepagePanorama,
} from '@/api/admin/homepage-media'
import PageHeader from '@/components/common/PageHeader.vue'

const items = ref<HomepageMedia[]>([])
const loading = ref(false)

const panoramaFields = [
  { key: 'start_yaw', label: '起始 yaw', min: -360, max: 360 },
  { key: 'start_pitch', label: '起始 pitch', min: -89, max: 89 },
  { key: 'yaw_speed_dps', label: 'yaw 速度', min: -90, max: 90 },
  { key: 'pitch_speed_dps', label: 'pitch 速度', min: -90, max: 90 },
] as const

function mediaUrl(item: HomepageMedia, face?: string) {
  const base = import.meta.env.BASE_URL
  const suffix = face ? `${item.storage_path}/${face}` : item.storage_path
  return `${base}static/carousel/${suffix}`.replace(/\/+/g, '/')
}

function previewUrl(item: HomepageMedia) {
  return item.type === 'panorama' ? mediaUrl(item, 'panorama_0.png') : mediaUrl(item)
}

async function fetchItems() {
  loading.value = true
  try {
    const res = await listHomepageMedia()
    items.value = res.data.map(normalizeItem)
  } catch (e) {
    ElMessage.error('获取首页媒体失败')
  } finally {
    loading.value = false
  }
}

function normalizeItem(item: HomepageMedia): HomepageMedia {
  if (item.type === 'panorama') {
    item.config = {
      start_yaw: Number(item.config?.start_yaw ?? 0),
      start_pitch: Number(item.config?.start_pitch ?? 0),
      yaw_speed_dps: Number(item.config?.yaw_speed_dps ?? 4),
      pitch_speed_dps: Number(item.config?.pitch_speed_dps ?? 0),
    }
  }
  return item
}

async function uploadImage({ file }: UploadRequestOptions) {
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepageImage(formData)
    ElMessage.success('图片已上传')
    fetchItems()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '上传失败')
  }
}

async function uploadPanorama({ file }: UploadRequestOptions) {
  const formData = new FormData()
  formData.append('file', file)
  try {
    await uploadHomepagePanorama(formData)
    ElMessage.success('Panorama 已上传')
    fetchItems()
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '上传失败')
  }
}

async function saveItem(item: HomepageMedia) {
  try {
    const body: Partial<HomepageMedia> = {
      title: item.title,
      enabled: item.enabled,
      duration_ms: item.duration_ms,
    }
    if (item.type === 'panorama') {
      body.config = {
        start_yaw: Number(item.config.start_yaw),
        start_pitch: Number(item.config.start_pitch),
        yaw_speed_dps: Number(item.config.yaw_speed_dps),
        pitch_speed_dps: Number(item.config.pitch_speed_dps),
      }
    }
    const res = await patchHomepageMedia(item.id, body)
    Object.assign(item, normalizeItem(res.data))
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '保存失败')
    fetchItems()
  }
}

async function move(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= items.value.length) return
  const copy = items.value.slice()
  const [item] = copy.splice(index, 1)
  if (!item) return
  copy.splice(target, 0, item)
  items.value = copy
  try {
    await reorderHomepageMedia(copy.map((row) => row.id))
  } catch (e: any) {
    ElMessage.error(e.response?.data?.detail || '排序失败')
    fetchItems()
  }
}

async function remove(item: HomepageMedia) {
  try {
    await ElMessageBox.confirm('确定要删除这个首页媒体吗？', '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteHomepageMedia(item.id)
    ElMessage.success('已删除')
    fetchItems()
  } catch (e) {}
}

onMounted(fetchItems)
</script>

<style scoped>
.admin-homepage-media { max-width: 1040px; margin: 0 auto; padding: 20px 0; }
.upload-actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.media-list { display: flex; flex-direction: column; gap: 14px; }
.media-row { display: grid; grid-template-columns: 180px 1fr auto; gap: 16px; padding: 14px; align-items: center; }
.preview { width: 180px; height: 104px; overflow: hidden; border-radius: 8px; background: var(--color-background-soft); }
.preview-image { width: 100%; height: 100%; display: block; }
.preview-panorama { width: 100%; height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; color: var(--color-text-secondary); }
.media-main { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
.media-head { display: grid; grid-template-columns: auto 1fr; gap: 10px; align-items: center; }
.controls, .panorama-controls { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; }
label { display: inline-flex; gap: 8px; align-items: center; font-size: 13px; color: var(--color-text-secondary); }
.row-actions { display: flex; gap: 8px; }
.empty-placeholder { padding: 40px 0; }

@media (max-width: 768px) {
  .media-row { grid-template-columns: 1fr; }
  .preview { width: 100%; height: 160px; }
  .row-actions { justify-content: flex-end; }
}
</style>
