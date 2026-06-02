<template>
  <div class="admin-union animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <div class="page-header-icon"><Connection /></div>
        <div class="page-header-text">
          <h2>Union 配置</h2>
          <p class="subtitle">配置联合认证系统，实现跨站角色绑定与数据同步</p>
        </div>
      </div>
    </div>

    <!-- Union Basic Config -->
    <el-card class="surface-card mb-6" shadow="never">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Setting /></el-icon>
            <span>基础配置</span>
          </div>
          <el-button type="primary" :icon="Check" @click="saveSettings" :loading="saving" class="hover-lift">
            保存更改
          </el-button>
        </div>
      </template>
      <el-form label-position="top" class="config-form">
        <div class="form-grid">
          <el-form-item label="Union API Root">
            <el-input v-model="settings.union_api_root" placeholder="https://skin.mualliance.ltd/api/union" />
          </el-form-item>
          <el-form-item label="Union Member Key">
            <el-input v-model="settings.union_member_key" type="password" placeholder="成员服务器认证令牌" show-password />
          </el-form-item>
          <el-form-item label="允许 Union 推送更新">
            <el-switch v-model="settings.union_enable_update" />
          </el-form-item>
          <el-form-item label="启用 Union OAuth2">
            <el-switch v-model="settings.union_enable_oauth2" />
          </el-form-item>
        </div>
      </el-form>
    </el-card>

    <!-- OAuth2 Keys -->
    <el-card class="surface-card mb-6" shadow="never">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Lock /></el-icon>
            <span>OAuth2 签名密钥</span>
          </div>
          <el-button size="small" plain :icon="Refresh" @click="generateKeypair" :loading="generating">
            生成新密钥
          </el-button>
        </div>
      </template>
      <div class="key-status-box" v-if="keyValid !== null">
        <el-tag :type="keyValid ? 'success' : 'danger'">
          {{ keyValid ? '密钥有效' : '密钥无效，此站点无法授权 Union OAuth2' }}
        </el-tag>
      </div>
      <el-form label-position="top" class="config-form mt-4">
        <el-form-item label="签名私钥">
          <el-input v-model="settings.union_oauth2_sig_private_key" type="textarea" :rows="5" placeholder="-----BEGIN PRIVATE KEY-----" />
        </el-form-item>
        <el-form-item label="验签公钥">
          <el-input v-model="settings.union_oauth2_sig_public_key" type="textarea" :rows="5" placeholder="-----BEGIN PUBLIC KEY-----" />
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Data Management -->
    <el-card class="surface-card mb-6" shadow="never">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Collection /></el-icon>
            <span>数据管理</span>
          </div>
          <el-tag :type="dataStatus === 'uptodate' ? 'success' : 'warning'" size="small">
            {{ dataStatus === 'uptodate' ? '数据已同步' : '数据可能已过时' }}
          </el-tag>
        </div>
      </template>
      <div class="data-action-grid">
        <el-button :icon="Refresh" @click="updateList" :loading="loadingList" class="data-action-btn">
          <span>更新服务器列表</span>
          <small>版本: {{ localListVersion }}</small>
        </el-button>
        <el-button :icon="Key" @click="updateKey" :loading="loadingKey" class="data-action-btn">
          <span>更新密钥对</span>
          <small>版本: {{ localKeyVersion }}</small>
        </el-button>
        <el-button :icon="Upload" @click="syncProfiles" :loading="loadingSync" class="data-action-btn">
          <span>同步角色数据</span>
        </el-button>
        <el-button :icon="Search" @click="openDiagnose" class="data-action-btn">
          <span>自助诊断</span>
        </el-button>
      </div>

      <!-- Diagnose Modal -->
      <el-dialog v-model="showDiagnose" title="自助诊断" width="720px" top="15vh" :close-on-click-modal="false" append-to-body>
        <div v-loading="loadingDiagnose">
          <div class="text-center" v-if="!diagnoseResult">
            <p style="padding:32px 0;color:var(--el-text-color-secondary)">点击下方按钮开始诊断</p>
            <el-button type="primary" @click="runDiagnose" :loading="loadingDiagnose">
              开始诊断
            </el-button>
          </div>
          <table v-if="diagnoseResult" class="diag-table">
            <tbody>
              <tr>
                <th>皮肤站 → Union</th>
                <td>
                  <el-tag :type="diagnoseResult.status === 'ok' ? 'success' : 'danger'" effect="dark">
                    {{ diagnoseResult.status === 'ok' ? '成功' : '失败' }}
                  </el-tag>
                </td>
              </tr>
              <template v-if="diagnoseResult.status === 'ok'">
                <tr>
                  <th>Union → 皮肤站</th>
                  <td>
                    <el-tag :type="diagnoseResult.data?.healthy ? 'success' : 'danger'" effect="dark">
                      {{ diagnoseResult.data?.healthy ? '成功' : '失败' }}
                    </el-tag>
                  </td>
                </tr>
                <tr v-if="diagnoseResult.data?.healthy">
                  <th>单程时延</th>
                  <td>{{ (diagnoseResult.data.delay * 1000).toFixed(2) }} 毫秒</td>
                </tr>
                <tr v-if="diagnoseResult.data?.timestamp">
                  <th>Union 时间戳</th>
                  <td>{{ diagnoseResult.data.timestamp }}</td>
                </tr>
                <tr v-if="diagnoseResult.data?.tls_handshake">
                  <th>TLS 握手</th>
                  <td>
                    <el-input type="textarea" :rows="6" :model-value="diagnoseResult.data.tls_handshake" readonly />
                  </td>
                </tr>
              </template>
              <template v-if="diagnoseResult.data?.exception">
                <tr>
                  <th>错误信息</th>
                  <td>
                    <el-input type="textarea" :rows="6" :model-value="diagnoseResult.data.exception" readonly />
                  </td>
                </tr>
              </template>
              <tr v-if="diagnoseResult.data?.status_code">
                <th>状态码</th>
                <td>{{ diagnoseResult.data.status_code }}</td>
              </tr>
              <tr v-if="diagnoseResult.data?.headers">
                <th>响应头</th>
                <td>
                  <el-input type="textarea" :rows="6" :model-value="formatHeaders(diagnoseResult.data.headers)" readonly />
                </td>
              </tr>
              <tr v-if="diagnoseResult.data?.body">
                <th>响应体</th>
                <td>
                  <el-input type="textarea" :rows="8" :model-value="diagnoseResult.data.body" readonly />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </el-dialog>
    </el-card>

    <!-- Server List -->
    <el-card class="surface-card mb-6" shadow="never" v-if="serverList.length > 0">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><List /></el-icon>
            <span>皮肤站列表</span>
          </div>
        </div>
      </template>
      <el-table :data="serverList" class="modern-table">
        <el-table-column prop="code" label="缩写" width="100" />
        <el-table-column label="皮肤站名称">
          <template #default="{ row }">
            <span>{{ row.displayName || '' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="bs_root" label="URL">
          <template #default="{ row }">
            <el-link :href="row.bs_root" target="_blank" type="primary">{{ row.bs_root }}</el-link>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Private Key Display -->
    <el-card class="surface-card mb-6" shadow="never" v-if="settings.ygg_private_key">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Key /></el-icon>
            <span>当前 Yggdrasil 私钥</span>
          </div>
        </div>
      </template>
      <el-input :model-value="settings.ygg_private_key" type="textarea" :rows="6" readonly />
    </el-card>

    <!-- Blacklist Management -->
    <el-card class="surface-card mb-6" shadow="never">
      <template #header>
        <div class="card-header-flex">
          <div class="title-group">
            <el-icon><Warning /></el-icon>
            <span>黑名单管理</span>
          </div>
        </div>
      </template>

      <!-- Add Form -->
      <div class="blacklist-add-box mb-4">
        <div class="section-title-label">添加黑名单</div>
        <div class="add-form-row">
          <el-input v-model="blacklistForm.email" placeholder="邮箱地址" size="default" class="add-input" />
          <el-input v-model="blacklistForm.reason" placeholder="原因" size="default" class="add-input" />
          <el-button type="danger" :icon="Plus" @click="createBlacklist" :loading="addingBlacklist">添加</el-button>
        </div>
      </div>

      <!-- Search -->
      <div class="blacklist-search-box mb-4">
        <el-input v-model="blacklistQuery" placeholder="搜索关键字" size="default" class="search-input" @keyup.enter="searchBlacklist">
          <template #append>
            <el-button @click="searchBlacklist">搜索</el-button>
          </template>
        </el-input>
      </div>

      <!-- Table -->
      <el-table :data="blacklistItems" v-loading="loadingBlacklist" class="modern-table">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="email" label="邮箱" min-width="200" />
        <el-table-column prop="source" label="来源" width="100" />
        <el-table-column prop="reason" label="原因" min-width="150" />
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="valid_until" label="有效至" width="160">
          <template #default="{ row }">
            {{ formatDate(row.valid_until) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" align="center">
          <template #default="{ row }">
            <el-button size="small" type="warning" plain @click="invalidateBlacklist(row.id)" :loading="operatingId === row.id">
              解封
            </el-button>
            <el-button size="small" type="danger" plain @click="deleteBlacklist(row.id)" :loading="operatingId === row.id">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="empty-state" v-if="!loadingBlacklist && blacklistItems.length === 0">
        <el-empty description="暂无黑名单记录" />
      </div>

      <!-- Pagination -->
      <div class="pagination-box" v-if="totalPages > 0">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="totalItems"
          layout="prev, pager, next"
          @current-change="fetchBlacklist"
          background
          small
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Connection, Check, Setting, Lock, Refresh, Collection,
  Key, Upload, Search, List, Plus, Warning, Delete, Loading
} from '@element-plus/icons-vue'
import {
  getAdminUnionSettings, saveAdminUnionSettings, generateUnionKeypair,
  updateUnionServerList, updateUnionPrivateKey, syncUnionProfiles,
  diagnoseUnion, getUnionBlacklist, createUnionBlacklist,
  invalidateUnionBlacklist, deleteUnionBlacklist
} from '@/api/union'

const settings = reactive({
  union_api_root: '',
  union_member_key: '',
  union_server_list_version: 0,
  union_private_key_version: 0,
  union_enable_update: true,
  union_enable_oauth2: true,
  union_oauth2_sig_private_key: '',
  union_oauth2_sig_public_key: '',
  ygg_private_key: '',
})
const saving = ref(false)
const generating = ref(false)
const loadingList = ref(false)
const loadingKey = ref(false)
const loadingSync = ref(false)
const loadingDiagnose = ref(false)
const showDiagnose = ref(false)
const diagnoseResult = ref(null)
const serverList = ref([])
const keyValid = ref(null)

// Blacklist
const blacklistItems = ref([])
const blacklistQuery = ref('')
const blacklistForm = reactive({ email: '', reason: '' })
const addingBlacklist = ref(false)
const loadingBlacklist = ref(false)
const operatingId = ref('')
const currentPage = ref(1)
const pageSize = ref(15)
const totalItems = ref(0)
const totalPages = computed(() => Math.ceil(totalItems.value / pageSize.value) || 0)

const localListVersion = computed(() => String(settings.union_server_list_version))
const localKeyVersion = computed(() => String(settings.union_private_key_version))

const dataStatus = computed(() => {
  return 'uptodate'
})

async function fetchSettings() {
  try {
    const res = await getAdminUnionSettings()
    Object.assign(settings, {
      union_api_root: res.data.union_api_root || '',
      union_member_key: res.data.union_member_key || '',
      union_server_list_version: res.data.union_server_list_version || 0,
      union_private_key_version: res.data.union_private_key_version || 0,
      union_enable_update: res.data.union_enable_update === 'true',
      union_enable_oauth2: res.data.union_enable_oauth2 === 'true',
      union_oauth2_sig_private_key: res.data.union_oauth2_sig_private_key || '',
      union_oauth2_sig_public_key: res.data.union_oauth2_sig_public_key || '',
      ygg_private_key: res.data.ygg_private_key || '',
    })
    serverList.value = (res.data.union_server_list || []).map(s => ({ ...s, displayName: '' }))

    // Check key validity
    checkKeyValidity()
    fetchServerNames()
  } catch (e) {
    ElMessage.error('加载 Union 配置失败')
  }
}

function checkKeyValidity() {
  const pk = settings.union_oauth2_sig_private_key
  const pub = settings.union_oauth2_sig_public_key
  if (pk && pub && pk.includes('BEGIN') && pub.includes('BEGIN')) {
    keyValid.value = true
  } else if (pk || pub) {
    keyValid.value = false
  } else {
    keyValid.value = null
  }
}

async function fetchServerNames() {
  await Promise.all(serverList.value.map(async (server) => {
    const base = server.bs_root.replace(/\/+$/, '')
    // element-skin: Yggdrasil at root /
    // Blessing Skin: Yggdrasil at /api/yggdrasil
    const candidates = [base + '/', base + '/api/yggdrasil']
    for (const apiUrl of candidates) {
      try {
        const res = await fetch(apiUrl, { mode: 'cors' })
        if (res.ok) {
          const data = await res.json()
          const name = data.meta?.serverName || data.serverName
          if (name) {
            server.displayName = name
            return
          }
        }
      } catch { /* try next candidate */ }
    }
    server.displayName = server.code
  }))
}

async function saveSettings() {
  saving.value = true
  try {
    await saveAdminUnionSettings(settings)
    ElMessage.success('配置已保存')
    checkKeyValidity()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    saving.value = false
  }
}

async function generateKeypair() {
  generating.value = true
  try {
    const res = await generateUnionKeypair()
    settings.union_oauth2_sig_private_key = res.data.privateKey
    settings.union_oauth2_sig_public_key = res.data.publicKey
    await saveSettings()
    ElMessage.success('密钥对已生成并保存')
  } catch (e) {
    ElMessage.error('生成密钥失败')
  } finally {
    generating.value = false
  }
}

async function updateList() {
  loadingList.value = true
  try {
    await updateUnionServerList()
    ElMessage.success('服务器列表已更新')
    await fetchSettings()
  } catch (e) {
    ElMessage.error('更新失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingList.value = false
  }
}

async function updateKey() {
  loadingKey.value = true
  try {
    await updateUnionPrivateKey()
    ElMessage.success('密钥已更新')
    await fetchSettings()
  } catch (e) {
    ElMessage.error('更新失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingKey.value = false
  }
}

async function syncProfiles() {
  loadingSync.value = true
  try {
    await syncUnionProfiles()
    ElMessage.success('角色数据已同步')
  } catch (e) {
    ElMessage.error('同步失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingSync.value = false
  }
}

function openDiagnose() {
  showDiagnose.value = true
  diagnoseResult.value = null
}

async function runDiagnose() {
  loadingDiagnose.value = true
  try {
    const res = await diagnoseUnion()
    diagnoseResult.value = res.data
  } catch (e) {
    diagnoseResult.value = { status: 'error', data: { exception: e.response?.data?.detail || e.message } }
  } finally {
    loadingDiagnose.value = false
  }
}

function formatDate(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  return d.toLocaleString()
}

function formatHeaders(headers) {
  if (!headers || typeof headers !== 'object') return ''
  return Object.entries(headers).map(([k, v]) => `${k}: ${Array.isArray(v) ? v.join(', ') : v}`).join('\n')
}

async function fetchBlacklist(page) {
  currentPage.value = page || 1
  loadingBlacklist.value = true
  try {
    const params = { page: currentPage.value }
    if (blacklistQuery.value) params.q = blacklistQuery.value
    const res = await getUnionBlacklist(params)
    blacklistItems.value = res.data.data || []
    totalItems.value = res.data.total || 0
  } catch (e) {
    ElMessage.error('加载黑名单失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    loadingBlacklist.value = false
  }
}

function searchBlacklist() {
  fetchBlacklist(1)
}

async function createBlacklist() {
  if (!blacklistForm.email) {
    ElMessage.warning('请输入邮箱地址')
    return
  }
  addingBlacklist.value = true
  try {
    await createUnionBlacklist(blacklistForm.email, blacklistForm.reason)
    ElMessage.success('黑名单已添加')
    blacklistForm.email = ''
    blacklistForm.reason = ''
    await fetchBlacklist(1)
  } catch (e) {
    ElMessage.error('添加失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    addingBlacklist.value = false
  }
}

async function invalidateBlacklist(id) {
  operatingId.value = id
  try {
    await invalidateUnionBlacklist(id)
    ElMessage.success('已解封')
    await fetchBlacklist(currentPage.value)
  } catch (e) {
    ElMessage.error('解封失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    operatingId.value = ''
  }
}

async function deleteBlacklist(id) {
  operatingId.value = id
  try {
    await deleteUnionBlacklist(id)
    ElMessage.success('已删除')
    await fetchBlacklist(currentPage.value)
  } catch (e) {
    ElMessage.error('删除失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    operatingId.value = ''
  }
}

onMounted(() => {
  fetchSettings()
  fetchBlacklist(1)
})
</script>

<style scoped>
@import "@/assets/styles/animations.css";
@import "@/assets/styles/layout.css";
@import "@/assets/styles/cards.css";
@import "@/assets/styles/headers.css";
@import "@/assets/styles/buttons.css";

.admin-union {
  max-width: 1100px;
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

.config-form {
  padding: 10px 0;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.data-action-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}
.data-action-btn {
  height: 80px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  border: 1px dashed var(--color-border);
  background: var(--color-background-soft);
  transition: var(--transition-base);
}
.data-action-btn:hover {
  border-color: var(--el-color-primary);
  background: rgba(64, 158, 255, 0.05);
}
.data-action-btn small {
  font-size: 11px;
  opacity: 0.6;
}

.key-status-box {
  margin-bottom: 8px;
}

.mb-6 {
  margin-bottom: 24px;
}
.mb-4 {
  margin-bottom: 16px;
}
.mt-4 {
  margin-top: 16px;
}

.section-title-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-light);
  margin-bottom: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.blacklist-add-box {
  padding: 16px;
  background: var(--color-background-soft);
  border-radius: 8px;
}
.add-form-row {
  display: flex;
  gap: 8px;
}
.add-input {
  flex: 1;
}

.blacklist-search-box {
  max-width: 400px;
}
.search-input {
  width: 100%;
}

.empty-state {
  padding: 40px 0;
}

.pagination-box {
  display: flex;
  justify-content: center;
  padding-top: 20px;
}

@media (max-width: 768px) {
  .form-grid,
  .data-action-grid {
    grid-template-columns: 1fr;
  }
}
</style>

<style>
.diag-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 8px;
}
.diag-table th {
  width: 180px;
  text-align: right;
  vertical-align: middle;
  padding: 10px 14px;
  font-weight: 600;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.diag-table td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.diag-table .el-textarea__inner {
  font-family: monospace;
  font-size: 12px;
}
</style>
