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
        :skin-url="texturesUrl(profile.skin_hash)"
        :cape-url="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
        :model="profile.texture_model || profile.model || 'default'"
        :width="200"
        :height="280"
        is-static
      />
      <el-empty v-else description="未设置皮肤" :image-size="120" />
    </div>

    <div class="role-info">
      <div class="role-name">{{ profile.name }}</div>
      <div class="role-owner">所属: {{ profile.owner_display_name || profile.owner_email || '-' }}</div>
      <div class="role-model">模型: {{ profile.texture_model || profile.model || 'default' }}</div>
    </div>

    <CardActions>
      <el-button class="btn-gradient btn-gradient-primary" @click="$emit('preview', profile)">
        <el-icon><Edit /></el-icon>
        <span>编辑</span>
      </el-button>
    </CardActions>
  </div>
</template>

<script setup lang="ts">
import { Edit } from '@element-plus/icons-vue'
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
  margin-bottom: 4px;
}

.role-owner {
  font-size: 13px;
  color: var(--color-text-light);
  margin-bottom: 4px;
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
