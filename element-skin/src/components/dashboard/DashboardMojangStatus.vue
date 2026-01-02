<template>
  <div class="mojang-status-section">
    <div class="section-header">
      <h2>Mojang 服务状态</h2>
      <el-button @click="checkMojangStatus" :loading="isChecking">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <div class="status-container">
      <div v-if="mojangStatusUrls">
        <el-row :gutter="20">
          <el-col :xs="24" :sm="8" v-for="(url, key) in mojangStatusUrls" :key="key">
            <el-card shadow="hover" class="status-card">
              <div class="status-item">
                <div class="status-label">{{ key.toUpperCase() }} API</div>
                <div class="status-indicator" :class="getMojangStatus(key)">
                  <el-icon v-if="getMojangStatus(key) === 'online'"><Check /></el-icon>
                  <el-icon v-else-if="getMojangStatus(key) === 'checking'"><Loading /></el-icon>
                  <el-icon v-else><Warning /></el-icon>
                  <span>{{ formatStatusText(getMojangStatus(key)) }}</span>
                </div>
                <div class="status-url">{{ url }}</div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>
      <el-empty v-else description="无法获取状态配置" />
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { Check, Loading, Warning, Refresh } from '@element-plus/icons-vue'

const mojangStatusUrls = ref(null)
const mojangHealth = ref({})
const isChecking = ref(false)

async function checkMojangStatus() {
  if (!mojangStatusUrls.value) return
  isChecking.value = true

  for (const [key, url] of Object.entries(mojangStatusUrls.value)) {
    mojangHealth.value[key] = 'checking'
    try {
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 5000)

      await fetch(url, { mode: 'no-cors', signal: controller.signal })
      clearTimeout(timeoutId)
      mojangHealth.value[key] = 'online'
    } catch (e) {
      mojangHealth.value[key] = 'offline'
    }
  }
  isChecking.value = false
}

function getMojangStatus(key) {
  return mojangHealth.value[key] || 'checking'
}

function formatStatusText(status) {
  if (status === 'online') return '在线'
  if (status === 'checking') return '检查中...'
  return '连接超时'
}

onMounted(async () => {
  try {
    const res = await axios.get('/public/settings')
    if (res.data.mojang_status_urls) {
      mojangStatusUrls.value = res.data.mojang_status_urls
      checkMojangStatus()
    }
  } catch (e) {
    console.warn('Failed to load Mojang status URLs')
  }
})
</script>

<style scoped>
.mojang-status-section {
  width: 100%;
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.section-header h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #303133;
}

.status-container {
  padding: 20px;
}

.status-card {
  margin-bottom: 20px;
  border-radius: 12px;
  transition: all 0.3s ease;
}

.status-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 10px;
}

.status-label {
  font-size: 16px;
  color: #606266;
  font-weight: 600;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 600;
  padding: 8px 16px;
  border-radius: 20px;
  background: #f5f7fa;
}

.status-indicator.online {
  color: #67c23a;
  background: #f0f9eb;
}

.status-indicator.checking {
  color: #409eff;
  background: #ecf5ff;
}

.status-indicator.offline {
  color: #f56c6c;
  background: #fef0f0;
}

.status-url {
  font-size: 12px;
  color: #909399;
  word-break: break-all;
  text-align: center;
}
</style>
