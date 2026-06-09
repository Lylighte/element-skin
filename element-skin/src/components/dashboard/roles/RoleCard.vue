<template>
  <div
    class="surface-card hoverable animate-card-slide clickable-card"
    :style="{ '--delay-index': delayIndex }"
    @click="$emit('preview', profile)"
  >
    <div
      class="role-preview"
      :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }"
    >
      <SkinViewer
        v-if="profile.skin_hash"
        :skinUrl="texturesUrl(profile.skin_hash)"
        :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
        :model="profile.model || 'default'"
        :width="200"
        :height="280"
        is-static
      />
      <el-empty v-else description="未设置皮肤" :image-size="120" />
    </div>

    <div class="role-info">
      <div class="role-name">{{ profile.name }}</div>
      <div class="role-model">模型: {{ profile.model || 'default' }}</div>
    </div>

    <CardActions>
      <el-button
        class="btn-gradient btn-gradient-danger btn-icon-swap"
        size="default"
        @click="$emit('delete', profile.id)"
      >
        <span class="btn-label">删除</span>
        <el-icon class="btn-icon"><Delete /></el-icon>
      </el-button>

      <el-button
        v-if="profile.skin_hash"
        class="btn-soft-warning btn-icon-swap"
        size="default"
        @click="$emit('clear-skin', profile.id)"
      >
        <span class="btn-label">皮肤</span>
        <el-icon class="btn-icon"><Close /></el-icon>
      </el-button>

      <el-button
        v-if="profile.cape_hash"
        class="btn-soft-warning btn-icon-swap"
        size="default"
        @click="$emit('clear-cape', profile.id)"
      >
        <span class="btn-label">披风</span>
        <el-icon class="btn-icon"><Close /></el-icon>
      </el-button>
    </CardActions>
  </div>
</template>

<script setup lang="ts">
import { Close, Delete } from '@element-plus/icons-vue'
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import CardActions from '@/components/common/CardActions.vue'

defineProps<{
  profile: Profile
  delayIndex: number
  isDark: boolean
  texturesUrl: (hash: string | null | undefined) => string
}>()

defineEmits<{
  preview: [profile: Profile]
  delete: [profileId: string]
  'clear-skin': [profileId: string]
  'clear-cape': [profileId: string]
}>()
</script>

<style scoped>
.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
}

.role-info {
  padding: 16px;
  text-align: center;
  background: var(--color-card-background);
}

.role-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
  margin-bottom: 8px;
}

.role-model {
  font-size: 13px;
  color: var(--color-text-light);
  font-weight: 500;
}

.clickable-card {
  cursor: pointer;
}
</style>
