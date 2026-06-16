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
        <el-card
          v-for="entry in fallbackEntries"
          :key="entry.id"
          shadow="hover"
          class="surface-card fallback-status-card"
        >
          <div class="fallback-card-header">
            <div class="fallback-title-block">
              <span class="fallback-priority">#{{ entry.priority }}</span>
              <span class="fallback-note">{{ entry.note || '未命名端点' }}</span>
            </div>
            <div class="fallback-overall" :class="overallClass(entry)">
              <el-icon v-if="overallStatus(entry) === 'online'"><Check /></el-icon>
              <el-icon v-else-if="overallStatus(entry) === 'partial'"><Warning /></el-icon>
              <el-icon v-else-if="overallStatus(entry) === 'unknown'"><Loading /></el-icon>
              <el-icon v-else><CircleClose /></el-icon>
              <span>{{ overallText(entry) }}</span>
            </div>
          </div>

          <div class="fallback-history">
            <div class="fallback-history-header">
              <span>近 24 小时</span>
              <span class="fallback-history-meta">{{ historyMeta(entry) }}</span>
            </div>
            <div class="fallback-history-grid" ref="historyGridRef">
              <div
                v-for="api in API_ROWS"
                :key="api.key"
                class="fallback-history-row"
              >
                <div class="fallback-history-label" :class="`state-${currentStatus(entry, api.key)}`">
                  <span class="fallback-history-state-dot" />
                  <span class="fallback-history-row-label">{{ api.label }}</span>
                </div>
                <div class="fallback-history-track">
                  <span
                    v-for="(bucket, idx) in buckets(entry, api.key)"
                    :key="idx"
                    class="fallback-history-cell"
                    :class="bucketClass(bucket)"
                    :title="bucketTitle(bucket)"
                  />
                </div>
              </div>
            </div>
            <div class="fallback-history-axis">
              <span>24h 前</span>
              <span>现在</span>
            </div>
          </div>
        </el-card>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { getPublicSettings, getPublicFallbackStatus } from '@/api/public'
import { getMe } from '@/api/me'
import {
  Box, User, CopyDocument,
  Check, Loading, Warning, Refresh, CircleClose
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FallbackStatusEntry } from '@/api/types'

// --- Stats & Config ---
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

// --- Fallback Status ---
type ApiKey = 'session' | 'account' | 'services'
const API_ROWS: { key: ApiKey; label: string }[] = [
  { key: 'session', label: 'Session' },
  { key: 'account', label: 'Account' },
  { key: 'services', label: 'Services' }
]

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

function currentStatus(entry: FallbackStatusEntry, key: ApiKey): 'up' | 'down' | 'unknown' {
  if (!entry.latest) return 'unknown'
  const value = entry.latest[key]
  return value === 'up' ? 'up' : value === 'down' ? 'down' : 'unknown'
}

function overallStatus(entry: FallbackStatusEntry) {
  if (!entry.latest) return 'unknown'
  const values: ApiKey[] = ['session', 'account', 'services']
  const ups = values.filter(k => entry.latest![k] === 'up').length
  if (ups === values.length) return 'online'
  if (ups === 0) return 'offline'
  return 'partial'
}

function overallClass(entry: FallbackStatusEntry) {
  return `overall-${overallStatus(entry)}`
}

function overallText(entry: FallbackStatusEntry) {
  switch (overallStatus(entry)) {
    case 'online': return '全部在线'
    case 'partial': return '部分在线'
    case 'offline': return '全部离线'
    default: return '尚未探测'
  }
}

interface HourBucket {
  startMs: number
  endMs: number
  total: number
  up: number
  down: number
}

const HISTORY_MS = 24 * 3600_000
const CELL_PITCH_PX = 12 // ~10px cell + 2px gap
const MIN_BUCKETS = 12
const MAX_BUCKETS = 144

const historyGridRef = ref<HTMLElement[]>([])
const bucketCount = ref(24)
let resizeObserver: ResizeObserver | null = null

function recomputeBucketCount() {
  const grids = historyGridRef.value
  if (!grids?.length) return
  const track = grids[0]?.querySelector('.fallback-history-track') as HTMLElement | null
  if (!track) return
  const width = track.clientWidth
  if (!width) return
  const count = Math.min(MAX_BUCKETS, Math.max(MIN_BUCKETS, Math.floor(width / CELL_PITCH_PX)))
  if (count !== bucketCount.value) bucketCount.value = count
}

function buckets(entry: FallbackStatusEntry, key: ApiKey): HourBucket[] {
  const count = bucketCount.value
  const bucketMs = HISTORY_MS / count
  const now = Date.now()
  const baseMs = now - HISTORY_MS
  const out: HourBucket[] = []
  for (let i = 0; i < count; i++) {
    out.push({
      startMs: baseMs + i * bucketMs,
      endMs: baseMs + (i + 1) * bucketMs,
      total: 0,
      up: 0,
      down: 0,
    })
  }
  for (const tick of entry.history) {
    const t = new Date(tick.checked_at).getTime()
    const idx = Math.floor((t - baseMs) / bucketMs)
    if (idx < 0 || idx >= count) continue
    const bucket = out[idx]
    if (!bucket) continue
    const value = tick[key]
    bucket.total++
    if (value === 'up') bucket.up++
    else if (value === 'down') bucket.down++
  }
  return out
}

function bucketClass(bucket: HourBucket) {
  if (bucket.total === 0) return 'cell-empty'
  if (bucket.down === 0) return 'cell-up'
  if (bucket.up === 0) return 'cell-down'
  return 'cell-mixed'
}

function formatTimeOfDay(ms: number) {
  const d = new Date(ms)
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

function bucketTitle(bucket: HourBucket) {
  const range = `${formatTimeOfDay(bucket.startMs)}–${formatTimeOfDay(bucket.endMs)}`
  if (bucket.total === 0) return `${range} · 暂无探测`
  return `${range} · ${bucket.total} 次探测 · 在线 ${bucket.up} / 离线 ${bucket.down}`
}

function historyMeta(entry: FallbackStatusEntry) {
  const total = entry.history.length
  if (!total) return ''
  const ups: Record<ApiKey, number> = { session: 0, account: 0, services: 0 }
  for (const tick of entry.history) {
    if (tick.session === 'up') ups.session++
    if (tick.account === 'up') ups.account++
    if (tick.services === 'up') ups.services++
  }
  const sumUp = ups.session + ups.account + ups.services
  const total3 = total * 3
  const pct = total3 ? Math.round((sumUp / total3) * 100) : 0
  return `${total} 次探测 · 可用率 ${pct}%`
}

// --- Lifecycle ---
function attachResizeObserver() {
  resizeObserver?.disconnect()
  resizeObserver = null
  const grids = historyGridRef.value
  if (!grids?.length) return
  resizeObserver = new ResizeObserver(() => recomputeBucketCount())
  resizeObserver.observe(grids[0]!)
}

watch(fallbackEntries, async () => {
  await nextTick()
  attachResizeObserver()
  recomputeBucketCount()
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
})

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

/* Quick Config */
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

/* Fallback list */
.fallback-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.fallback-status-card :deep(.el-card__body) {
  padding: 20px;
}
.fallback-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.fallback-title-block {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.fallback-priority {
  background: var(--el-color-primary);
  color: #fff;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.fallback-note {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-heading);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.fallback-overall {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.overall-online { background: rgba(103, 194, 58, 0.15); color: var(--el-color-success); }
.overall-partial { background: rgba(230, 162, 60, 0.15); color: var(--el-color-warning); }
.overall-offline { background: rgba(245, 108, 108, 0.15); color: var(--el-color-danger); }
.overall-unknown { background: rgba(144, 147, 153, 0.15); color: var(--el-color-info); }

.fallback-history {
  padding-top: 4px;
}
.fallback-history-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  font-size: 13px;
  color: var(--color-text-light);
  margin-bottom: 10px;
  font-weight: 600;
}
.fallback-history-meta {
  font-size: 12px;
  font-weight: 500;
}
.fallback-history-grid {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.fallback-history-row {
  display: grid;
  grid-template-columns: 92px 1fr;
  align-items: center;
  gap: 10px;
}
.fallback-history-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-light);
}
.fallback-history-row-label {
  letter-spacing: 0.2px;
}
.fallback-history-state-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-info);
  flex-shrink: 0;
}
.state-up { color: var(--el-color-success); }
.state-up .fallback-history-state-dot { background: var(--el-color-success); }
.state-down { color: var(--el-color-danger); }
.state-down .fallback-history-state-dot { background: var(--el-color-danger); }
.state-unknown { color: var(--el-color-info); }
.state-unknown .fallback-history-state-dot { background: var(--el-color-info); }

.fallback-history-track {
  display: grid;
  grid-auto-columns: 1fr;
  grid-auto-flow: column;
  gap: 2px;
}
.fallback-history-cell {
  height: 14px;
  border-radius: 3px;
  background: var(--color-background-soft);
  border: 1px solid var(--color-border);
}
.cell-up { background: var(--el-color-success); border-color: var(--el-color-success); }
.cell-down { background: var(--el-color-danger); border-color: var(--el-color-danger); }
.cell-mixed { background: var(--el-color-warning); border-color: var(--el-color-warning); }
.cell-empty { background: transparent; }

.fallback-history-axis {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--color-text-light);
  margin-top: 6px;
  padding-left: 102px;
}

@media (max-width: 768px) {
  .fallback-history-row { grid-template-columns: 80px 1fr; }
  .fallback-history-axis { padding-left: 90px; }
}
</style>
