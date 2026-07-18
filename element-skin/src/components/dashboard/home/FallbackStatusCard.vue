<template>
  <UiCard shadow="hover" class="fallback-status-card">
    <div class="fallback-card-header">
      <div class="fallback-title-block">
        <span class="fallback-priority">#{{ entry.priority }}</span>
        <span class="fallback-note">{{ entry.note || '未命名端点' }}</span>
      </div>
      <div class="fallback-overall" :class="`overall-${overallStatus}`">
        <el-icon v-if="overallStatus === 'online'"><Check /></el-icon>
        <el-icon v-else-if="overallStatus === 'partial'"><Warning /></el-icon>
        <el-icon v-else-if="overallStatus === 'unknown'"><Loading /></el-icon>
        <el-icon v-else><CircleClose /></el-icon>
        <span>{{ overallText }}</span>
      </div>
    </div>

    <div class="fallback-history">
      <div class="fallback-history-header">
        <span>近 24 小时</span>
        <span class="fallback-history-meta">{{ historyMeta }}</span>
      </div>

      <div class="fallback-history-grid">
        <div
          v-for="api in API_ROWS"
          :key="api.key"
          class="fallback-history-row"
        >
          <div class="fallback-history-label" :class="`state-${currentStatus(api.key)}`">
            <span class="fallback-history-state-dot" />
            <span class="fallback-history-row-label">{{ api.label }}</span>
          </div>
          <div class="fallback-history-track" ref="trackRefs">
            <span
              v-for="(bucket, idx) in buckets(api.key)"
              :key="idx"
              class="fallback-history-cell"
              :class="bucketClass(bucket)"
              :title="bucketTitle(bucket)"
            />
          </div>
        </div>
      </div>

      <div class="fallback-history-axis">
        <span
          v-for="(label, idx) in axisLabels"
          :key="idx"
          class="fallback-history-axis-tick"
          :style="{ left: `${label.percent}%`, transform: axisTransform(idx) }"
        >{{ label.text }}</span>
      </div>
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { Check, Loading, Warning, CircleClose } from '@element-plus/icons-vue'
import type { FallbackStatusEntry } from '@/api/types'
import UiCard from '@/components/ui/UiCard.vue'

type ApiKey = 'session' | 'account' | 'services'
const API_ROWS: { key: ApiKey; label: string }[] = [
  { key: 'session', label: 'Session' },
  { key: 'account', label: 'Account' },
  { key: 'services', label: 'Services' },
]

interface HourBucket {
  startMs: number
  endMs: number
  total: number
  up: number
  down: number
}

const props = defineProps<{ entry: FallbackStatusEntry }>()

const HISTORY_MS = 24 * 3600_000
const CELL_PITCH_PX = 12
const MIN_BUCKETS = 12
const MAX_BUCKETS = 144
const LABEL_PITCH_PX = 80
const MIN_LABELS = 2
const MAX_LABELS = 7

const trackRefs = ref<HTMLElement[]>([])
const trackWidth = ref(0)
const now = ref(Date.now())
let resizeObserver: ResizeObserver | null = null
let nowTimer: ReturnType<typeof setInterval> | null = null

const bucketCount = computed(() => {
  if (!trackWidth.value) return 24
  return Math.min(MAX_BUCKETS, Math.max(MIN_BUCKETS, Math.floor(trackWidth.value / CELL_PITCH_PX)))
})

const labelCount = computed(() => {
  if (!trackWidth.value) return 4
  return Math.min(MAX_LABELS, Math.max(MIN_LABELS, Math.floor(trackWidth.value / LABEL_PITCH_PX) + 1))
})

const baseMs = computed(() => now.value - HISTORY_MS)

function currentStatus(key: ApiKey): 'up' | 'down' | 'unknown' {
  if (!props.entry.latest) return 'unknown'
  const value = props.entry.latest[key]
  return value === 'up' ? 'up' : value === 'down' ? 'down' : 'unknown'
}

const overallStatus = computed(() => {
  if (!props.entry.latest) return 'unknown'
  const ups = (['session', 'account', 'services'] as ApiKey[]).filter(
    (k) => props.entry.latest![k] === 'up',
  ).length
  if (ups === 3) return 'online'
  if (ups === 0) return 'offline'
  return 'partial'
})

const overallText = computed(() => {
  switch (overallStatus.value) {
    case 'online': return '全部在线'
    case 'partial': return '部分在线'
    case 'offline': return '全部离线'
    default: return '尚未探测'
  }
})

function buckets(key: ApiKey): HourBucket[] {
  const count = bucketCount.value
  const bucketMs = HISTORY_MS / count
  const out: HourBucket[] = []
  for (let i = 0; i < count; i++) {
    out.push({
      startMs: baseMs.value + i * bucketMs,
      endMs: baseMs.value + (i + 1) * bucketMs,
      total: 0,
      up: 0,
      down: 0,
    })
  }
  for (const tick of props.entry.history) {
    const t = new Date(tick.checked_at).getTime()
    const idx = Math.floor((t - baseMs.value) / bucketMs)
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

function pad2(n: number) {
  return n.toString().padStart(2, '0')
}

function formatHM(ms: number) {
  const d = new Date(ms)
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}`
}

function bucketTitle(bucket: HourBucket) {
  const range = `${formatHM(bucket.startMs)}–${formatHM(bucket.endMs)}`
  if (bucket.total === 0) return `${range} · 暂无探测`
  return `${range} · ${bucket.total} 次探测 · 在线 ${bucket.up} / 离线 ${bucket.down}`
}

const axisLabels = computed(() => {
  const count = labelCount.value
  const labels: { percent: number; text: string }[] = []
  for (let i = 0; i < count; i++) {
    const percent = (i / (count - 1)) * 100
    labels.push({
      percent,
      text: formatHM(baseMs.value + (i / (count - 1)) * HISTORY_MS),
    })
  }
  return labels
})

function axisTransform(idx: number) {
  if (idx === 0) return 'translateX(0)'
  if (idx === axisLabels.value.length - 1) return 'translateX(-100%)'
  return 'translateX(-50%)'
}

const historyMeta = computed(() => {
  const total = props.entry.history.length
  if (!total) return ''
  const ups = { session: 0, account: 0, services: 0 } as Record<ApiKey, number>
  for (const tick of props.entry.history) {
    if (tick.session === 'up') ups.session++
    if (tick.account === 'up') ups.account++
    if (tick.services === 'up') ups.services++
  }
  const pct = Math.round(((ups.session + ups.account + ups.services) / (total * 3)) * 100)
  return `${total} 次探测 · 可用率 ${pct}%`
})

function refreshTrackWidth() {
  const track = trackRefs.value[0]
  if (!track) return
  const width = track.clientWidth
  if (width && width !== trackWidth.value) trackWidth.value = width
}

onMounted(async () => {
  await nextTick()
  refreshTrackWidth()
  const track = trackRefs.value[0]
  if (track) {
    resizeObserver = new ResizeObserver(() => refreshTrackWidth())
    resizeObserver.observe(track)
  }
  nowTimer = setInterval(() => {
    now.value = Date.now()
  }, 60_000)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (nowTimer) clearInterval(nowTimer)
  nowTimer = null
})

watch(() => props.entry, async () => {
  await nextTick()
  refreshTrackWidth()
})
</script>

<style scoped>
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
  position: relative;
  height: 16px;
  margin-top: 4px;
  margin-left: 102px;
}
.fallback-history-axis-tick {
  position: absolute;
  top: 0;
  font-size: 11px;
  color: var(--color-text-light);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .fallback-history-row { grid-template-columns: 80px 1fr; }
  .fallback-history-axis { margin-left: 90px; }
}
</style>
