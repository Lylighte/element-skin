<template>
  <div class="union-section animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div>
          <h1>角色绑定</h1>
          <p>管理您在 Union 中的角色绑定关系</p>
        </div>
      </div>
      <div class="page-header-actions">
        <el-tag v-if="securityLevel !== null" :type="securityLevelTagType" size="large" effect="dark">
          SL{{ securityLevel }}
        </el-tag>
      </div>
    </div>

    <!-- Security Level Info -->
    <el-card class="surface-card mb-6" shadow="never" v-if="securityLevel !== null">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><ShieldCheck /></el-icon>
            <span>安全等级</span>
          </div>
        </div>
      </template>
      <div class="security-level-info">
        <p v-if="securityLevel >= 3">SL3: 已加入 Union 黑名单并受信任</p>
        <p v-else-if="securityLevel >= 1">SL1: 已加入 Union 黑名单</p>
        <p v-else>SL0: 未加入 Union 黑名单</p>
      </div>
    </el-card>

    <!-- Profile Binding -->
    <el-card class="surface-card" shadow="never">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Connection /></el-icon>
            <span>角色绑定状态</span>
          </div>
        </div>
      </template>

      <div v-loading="loading">
        <div v-if="profiles.length === 0 && !loading">
          <el-empty description="没有可绑定的角色" />
        </div>

        <div v-for="(prof, index) in profiles" :key="prof.id" class="profile-binding-card">
          <div class="profile-info-row">
            <div class="profile-name-box">
              <div class="profile-name">{{ prof.name }}</div>
              <div class="profile-uuid">{{ prof.id }}</div>
            </div>
            <div class="profile-status-box">
              <!-- Reference uses bind_status > 0 to determine binding, not self truthiness -->
              <el-tag v-if="prof.self && prof.self.bind_status > 0" type="success" size="small">已绑定</el-tag>
              <el-tag v-else type="info" size="small">未绑定</el-tag>
            </div>
          </div>

          <!-- Bound details -->
          <div v-if="prof.self && prof.self.bind && prof.self.bind.length > 0" class="profile-detail-box">
            <div class="binding-table">
              <div class="binding-header">子绑定</div>
              <div v-for="b in prof.self.bind" :key="b.internal_id" class="binding-row">
                <span>{{ b.bind_mapped_name || b.name || '未知' }}</span>
                <span class="binding-server">{{ b.backend || b.backend_id || '未知服务器' }}</span>
              </div>
            </div>
          </div>

          <!-- Dup name warning -->
          <div v-if="prof.dup_name && prof.dup_name.length > 0" class="dup-warning-box">
            <el-alert :title="'检测到 ' + prof.dup_name.length + ' 个同名角色'" type="warning" :closable="false" show-icon>
              <template #default>
                <div v-for="d in prof.dup_name" :key="d.internal_id" class="dup-item">
                  <span>{{ d.bind_mapped_name || d.name || '未知' }}</span>
                  <span class="dup-server">{{ d.backend || d.backend_id || '未知服务器' }}</span>
                </div>
              </template>
            </el-alert>
          </div>

          <!-- Actions (matching reference: bind_status > 0 → unbind; else → token/bindTo UI) -->
          <div class="profile-actions">
            <template v-if="prof.self && prof.self.bind_status > 0">
              <el-button size="small" type="danger" plain @click="confirmUnbind(prof.id)" :loading="loadingUnbind === prof.id">
                解绑
              </el-button>
              <el-button size="small" @click="confirmRemapUUID(prof)" :loading="loadingRemap === prof.id">
                同步 UUID
              </el-button>
            </template>
            <template v-else>
              <el-button size="small" type="primary" plain @click="getToken(prof.id)" :loading="loadingToken === prof.id">
                获取 Token
              </el-button>
              <el-input
                v-model="prof._token_input"
                placeholder="输入其他服务器的 Token"
                size="small"
                class="token-input"
                @keyup.enter="bindTo(prof.id, prof._token_input)"
              >
                <template #append>
                  <el-button @click="bindTo(prof.id, prof._token_input)" :loading="loadingBindTo === prof.id">
                    绑定
                  </el-button>
                </template>
              </el-input>
            </template>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Ticket as ShieldCheck } from '@element-plus/icons-vue'

const jwt = localStorage.getItem('jwt')
const headers = { Authorization: 'Bearer ' + jwt }

const loading = ref(false)
const profiles = ref([])
const securityLevel = ref(null)

const loadingToken = ref('')
const loadingUnbind = ref('')
const loadingBindTo = ref('')
const loadingRemap = ref('')

const serverNames = ref({})

const securityLevelTagType = ref('')

async function fetchProfiles() {
  loading.value = true
  try {
    const res = await axios.get('/union/profiles', { headers })
    profiles.value = (res.data.items || []).map(p => ({
      ...p,
      _token_input: '',
      dup_name: Array.isArray(p.dup_name) ? p.dup_name : (p.dup_name?.data || []),
    }))
  } catch (e) {
    ElMessage.error('加载角色绑定信息失败')
  } finally {
    loading.value = false
  }
}

async function fetchSecurityLevel() {
  try {
    const res = await axios.get('/union/security/level', { headers })
    const level = res.data.security_level
    securityLevel.value = level
    if (level >= 3) securityLevelTagType.value = 'success'
    else if (level >= 1) securityLevelTagType.value = 'warning'
    else securityLevelTagType.value = 'info'
  } catch (e) {
    securityLevel.value = 0
    securityLevelTagType.value = 'info'
  }
}

function getServerName(backendId) {
  return serverNames.value[backendId] || '未知服务器'
}

async function getToken(uuid) {
  loadingToken.value = uuid
  try {
    const res = await axios.post('/union/bind', { uuid }, { headers })
    await navigator.clipboard.writeText(res.data.token)
    ElMessage.success('Token 已复制到剪贴板（有效期2分钟）')
  } catch (e) {
    ElMessage.error('获取 Token 失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingToken.value = ''
  }
}

async function bindTo(uuid, token) {
  if (!token) return
  loadingBindTo.value = uuid
  try {
    await axios.post('/union/bindto', { uuid, token }, { headers })
    ElMessage.success('绑定成功')
    const p = profiles.value.find(p => p.id === uuid)
    if (p) p._token_input = ''
    await fetchProfiles()
  } catch (e) {
    ElMessage.error('绑定失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingBindTo.value = ''
  }
}

async function unbind(uuid) {
  loadingUnbind.value = uuid
  try {
    await axios.post('/union/unbind', { uuid }, { headers })
    ElMessage.success('解绑成功')
    await fetchProfiles()
  } catch (e) {
    ElMessage.error('解绑失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingUnbind.value = ''
  }
}

function confirmUnbind(uuid) {
  ElMessageBox.confirm('确认解绑？', '解绑确认', {
    confirmButtonText: '确认',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(() => unbind(uuid)).catch(() => {})
}

async function remapUUID(me, target) {
  loadingRemap.value = me
  try {
    await axios.post('/union/remapuuid', { me, target }, { headers })
    ElMessage.success('UUID 同步请求已发送')
  } catch (e) {
    ElMessage.error('UUID 同步失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingRemap.value = ''
  }
}

function confirmRemapUUID(prof) {
  const me = prof.self?.uuid || prof.id
  const target = prof.self?.mapped_uuid || me
  ElMessageBox.confirm(
    '注意！绑定的 UUID 将同步到所有皮肤站。对于不使用 Union 的 Minecraft 服务器，修改 UUID 将导致基于 UUID 的玩家数据不再有效。',
    '同步 UUID 确认',
    { confirmButtonText: '确认', cancelButtonText: '取消', type: 'warning' },
  ).then(() => remapUUID(me, target)).catch(() => {})
}

onMounted(() => {
  fetchProfiles()
  fetchSecurityLevel()
})
</script>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/headers.css";
@import "@/assets/styles/buttons.css";

.union-section {
  max-width: 900px;
  margin: 0 auto;
  padding: 20px 0;
}

.card-header-flex {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-header-flex .title-group {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  color: var(--color-heading);
}

.mb-6 { margin-bottom: 24px; }

.profile-binding-card {
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
  background: var(--color-card-background);
  transition: var(--transition-base);
}
.profile-binding-card:hover {
  border-color: var(--el-color-primary-light-5);
}

.profile-info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.profile-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-heading);
}
.profile-uuid {
  font-size: 11px;
  color: var(--color-text-light);
  font-family: monospace;
}

.profile-detail-box {
  margin-bottom: 12px;
  padding: 12px;
  background: var(--color-background-soft);
  border-radius: 8px;
}

.binding-header {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-light);
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.binding-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  font-size: 13px;
}
.binding-server {
  color: var(--color-text-light);
  font-size: 12px;
}

.dup-warning-box {
  margin-bottom: 12px;
}
.dup-item {
  display: flex;
  justify-content: space-between;
  padding: 2px 0;
  font-size: 13px;
}
.dup-server {
  color: var(--color-text-light);
  font-size: 12px;
}

.profile-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.token-input {
  width: 240px;
}

.security-level-info p {
  margin: 0;
  font-size: 14px;
}

@media (max-width: 768px) {
  .profile-actions {
    flex-direction: column;
    align-items: stretch;
  }
  .token-input {
    width: 100%;
  }
}
</style>
