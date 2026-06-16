<template>
  <div class="dashboard-home animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>我的主页</h1>
        <p>这里汇总了您的资源数量、启动器接入入口与备用服务的健康状态</p>
      </div>
    </div>

    <section class="dashboard-section stats-section">
      <div class="stats-grid">
        <el-card shadow="hover" class="surface-card">
          <div class="stats-card-content">
            <div class="stats-card-icon bg-gradient-blue">
              <el-icon><Box /></el-icon>
            </div>
            <div class="stats-card-info">
              <div class="stats-card-label">材质数量</div>
              <div class="stats-card-value">{{ textureCount }}</div>
            </div>
          </div>
        </el-card>
        <el-card shadow="hover" class="surface-card">
          <div class="stats-card-content">
            <div class="stats-card-icon bg-gradient-purple">
              <el-icon><User /></el-icon>
            </div>
            <div class="stats-card-info">
              <div class="stats-card-label">角色数量</div>
              <div class="stats-card-value">{{ profileCount }}</div>
            </div>
          </div>
        </el-card>
      </div>
    </section>

    <section class="dashboard-section">
      <div class="section-header">
        <h2>快速接入启动器</h2>
      </div>
      <el-card shadow="hover" class="surface-card">
        <div class="config-content">
          <p class="config-desc">
            点击下方按钮复制 API 地址，或直接将其拖到支持 authlib-injector 的启动器窗口中。
          </p>
          <div class="config-actions">
            <el-input v-model="apiUrl" readonly class="api-url-input" />
            <a
              class="el-button el-button--primary drag-btn"
              :href="`authlib-injector:yggdrasil-server:${encodeURIComponent(apiUrl)}`"
              title="点击复制，或拖到启动器"
              @click.prevent="copyApiUrl"
            >
              <el-icon><CopyDocument /></el-icon>
              <span>复制或拖到启动器</span>
            </a>
          </div>
        </div>
      </el-card>
    </section>

    <section v-if="fallbackEntries.length" class="dashboard-section">
      <div class="section-header">
        <h2>备用服务状态</h2>
        <el-button @click="loadFallbackStatus" :loading="isChecking" size="small" text>
          <el-icon><Refresh /></el-icon>
          <span>刷新</span>
        </el-button>
      </div>

      <div class="fallback-list">
        <FallbackStatusCard
          v-for="entry in fallbackEntries"
          :key="entry.id"
          :entry="entry"
        />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getPublicSettings, getPublicFallbackStatus } from '@/api/public'
import { getMe } from '@/api/me'
import { Box, User, CopyDocument, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FallbackStatusEntry } from '@/api/types'
import FallbackStatusCard from './FallbackStatusCard.vue'

const textureCount = ref(0)
const profileCount = ref(0)
const apiUrl = ref('')

function getApiUrl() {
  const base = import.meta.env.VITE_API_BASE || ''
  if (base.startsWith('http')) {
    return base
  }
  const origin = window.location.origin
  const path = base.startsWith('/') ? base : '/' + base
  let full = origin + path
  if (full.endsWith('/') && full.length > 1) {
    full = full.slice(0, -1)
  }
  return full
}

function copyApiUrl() {
  if (!apiUrl.value) return
  navigator.clipboard.writeText(apiUrl.value).then(() => {
    ElMessage.success('API 地址已复制')
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

const fallbackEntries = ref<FallbackStatusEntry[]>([])
const isChecking = ref(false)

async function loadFallbackStatus() {
  isChecking.value = true
  try {
    const res = await getPublicFallbackStatus()
    const list = (res.data.endpoints || []).slice()
    list.sort((a, b) => (a.priority || 0) - (b.priority || 0))
    fallbackEntries.value = list
  } catch (e) {
    ElMessage.error('加载 Fallback 状态失败')
  } finally {
    isChecking.value = false
  }
}

onMounted(async () => {
  try {
    const res = await getPublicSettings()
    if (res.data.api_url) {
      apiUrl.value = res.data.api_url.endsWith('/') ? res.data.api_url.slice(0, -1) : res.data.api_url
    } else {
      apiUrl.value = getApiUrl()
    }
  } catch (e) {
    apiUrl.value = getApiUrl()
  }

  await loadFallbackStatus()

  try {
    const res = await getMe()
    if (res.data) {
      profileCount.value = res.data.profile_count || 0
      textureCount.value = res.data.texture_count || 0
    }
  } catch (e) {
    console.error('Failed to load user stats', e)
  }
})
</script>

<style scoped>
.dashboard-home {
  display: flex;
  flex-direction: column;
}

.dashboard-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 32px;
}
.dashboard-section:last-child {
  margin-bottom: 0;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
}
.section-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--color-heading);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}
@media (max-width: 640px) {
  .stats-grid { grid-template-columns: 1fr; }
}

.config-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 4px 0;
}
.config-desc {
  font-size: 14px;
  color: var(--color-text-light);
  margin: 0;
  line-height: 1.6;
}
.config-actions {
  display: flex;
  gap: 12px;
  align-items: stretch;
  flex-wrap: wrap;
}
.api-url-input {
  flex: 99 1 320px;
  min-width: 0;
}
.drag-btn {
  flex: 1 1 240px;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: var(--el-component-size, 32px);
  padding: 0 16px;
  font-weight: 500;
  white-space: nowrap;
  transition: transform 0.2s;
}
.drag-btn:hover {
  transform: translateY(-1px);
  color: white;
}

.fallback-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
</style>
