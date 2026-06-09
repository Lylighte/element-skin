<template>
  <div
    class="surface-card hoverable animate-card-slide clickable-card texture-card"
    :style="{ '--delay-index': delayIndex }"
    @click="$emit('preview', texture)"
  >
    <div class="card-clip">
      <div
        class="item-card-preview"
        :style="{ background: isDark ? 'var(--color-background-hero-dark)' : 'var(--color-background-hero-light)' }"
      >
        <SkinViewer
          v-if="texture.type === 'skin'"
          :skin-url="texturesUrl(texture.hash)"
          :model="texture.model || 'default'"
          :width="200"
          :height="280"
          is-static
        />
        <CapeViewer
          v-else
          :cape-url="texturesUrl(texture.hash)"
          :width="200"
          :height="280"
          is-static
        />
        <div
          v-if="texture.type === 'skin' && resolution"
          class="floating-badge"
          :style="resolutionBadgeStyle"
        >
          {{ resolution }}x
        </div>
      </div>

      <div class="item-card-info texture-card-info">
        <slot name="info" :texture="texture">
          <div v-if="showType" class="type-tag" :class="texture.type">
            {{ texture.type === 'skin' ? '皮肤' : '披风' }}
          </div>
          <div class="item-card-title">{{ title || '未命名纹理' }}</div>
          <div v-if="subtitle" class="item-card-subtitle">{{ subtitle }}</div>
        </slot>
      </div>

      <CardActions v-if="$slots.actions">
        <slot name="actions" :texture="texture" />
      </CardActions>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'
import CardActions from '@/components/common/CardActions.vue'
import type { Texture } from '@/api/types'

const props = withDefaults(defineProps<{
  texture: Texture
  delayIndex?: number
  isDark?: boolean
  texturesUrl: (hash: string | null | undefined) => string
  title?: string | null
  subtitle?: string | null
  resolution?: number
  showType?: boolean
}>(), {
  delayIndex: 0,
  isDark: false,
  title: '',
  subtitle: '',
  resolution: undefined,
  showType: false,
})

defineEmits<{
  preview: [texture: Texture]
}>()

const resolutionBadgeStyle = computed(() => {
  const resolution = props.resolution
  if (!resolution) return {}

  let hue = 0
  if (resolution <= 64) hue = 120
  else if (resolution <= 128) hue = 120 - ((resolution - 64) / 64) * 60
  else if (resolution <= 256) hue = 60 - ((resolution - 128) / 128) * 30
  else if (resolution <= 512) hue = 30 - ((resolution - 256) / 256) * 30
  else hue = 330

  return {
    background: `linear-gradient(135deg, hsl(${hue}, 58%, 65%), hsl(${hue + 15}, 53%, 62%))`,
    boxShadow: `0 2px 6px hsla(${hue}, 58%, 50%, 0.25)`,
  }
})
</script>

<style scoped>
.texture-card {
  cursor: pointer;
}

.card-clip {
  border-radius: inherit;
  overflow: hidden;
}

.texture-card-info {
  min-height: 82px;
}
</style>
