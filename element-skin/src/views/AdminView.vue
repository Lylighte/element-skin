<template>
  <div class="admin-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="admin-sidebar">
        <div class="admin-title">
          <el-icon size="24"><Tools /></el-icon>
          <span>管理面板</span>
        </div>
        <div class="title-divider"></div>
        <el-menu :default-active="activeRoute" mode="vertical" router class="sidebar-menu">
          <el-menu-item index="/admin/settings">
            <el-icon><Setting /></el-icon>
            <span>站点设置</span>
          </el-menu-item>
          <el-menu-item index="/admin/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
          <el-menu-item index="/admin/invites">
            <el-icon><Ticket /></el-icon>
            <span>邀请码管理</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/wardrobe">
            <el-icon><Back /></el-icon>
            <span>返回用户面板</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-main class="admin-main">
        <!-- 站点设置 -->
        <div v-if="active === 'settings'" class="settings-section">
          <div class="section-header">
            <h2>站点设置</h2>
            <el-button type="primary" @click="loadSettings">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-card class="settings-card">
            <el-form label-width="140px" :model="siteSettings">
              <el-form-item label="站点名称">
                <el-input v-model="siteSettings.site_name" placeholder="皮肤站" />
              </el-form-item>
              <el-form-item label="后端 API 地址">
                <el-input v-model="siteSettings.site_url" placeholder="https://skin.example.com" />
              </el-form-item>
              <el-form-item label="需要邀请码注册">
                <el-switch v-model="siteSettings.require_invite" />
              </el-form-item>
              <el-form-item label="允许用户注册">
                <el-switch v-model="siteSettings.allow_register" />
              </el-form-item>
              <el-form-item label="最大纹理大小">
                <el-input v-model="siteSettings.max_texture_size" type="number">
                  <template #suffix>KB</template>
                </el-input>
              </el-form-item>
              <el-divider content-position="left">安全设置</el-divider>
              <el-form-item label="启用速率限制">
                <el-switch v-model="siteSettings.rate_limit_enabled" />
              </el-form-item>
              <el-form-item label="登录失败限制" v-if="siteSettings.rate_limit_enabled">
                <el-input v-model="siteSettings.rate_limit_auth_attempts" type="number">
                  <template #suffix>次</template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">每个时间窗口内允许的最大尝试次数</el-text>
              </el-form-item>
              <el-form-item label="时间窗口" v-if="siteSettings.rate_limit_enabled">
                <el-input v-model="siteSettings.rate_limit_auth_window" type="number">
                  <template #suffix>分钟</template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">超限后需等待的时间</el-text>
              </el-form-item>
              <el-divider content-position="left">JWT 认证设置</el-divider>
              <el-form-item label="JWT 过期时间">
                <el-input v-model="siteSettings.jwt_expire_days" type="number">
                  <template #suffix>天</template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">用户登录后 Token 的有效期</el-text>
              </el-form-item>
              <el-divider content-position="left">微软正版登录设置</el-divider>
              <el-form-item label="Client ID">
                <el-input v-model="siteSettings.microsoft_client_id" placeholder="Azure 应用的 Client ID">
                  <template #prepend>
                    <el-icon><Key /></el-icon>
                  </template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">
                  在 <el-link href="https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade" target="_blank" type="primary">Azure Portal</el-link> 创建应用获取
                </el-text>
              </el-form-item>
              <el-form-item label="Client Secret">
                <el-input
                  v-model="siteSettings.microsoft_client_secret"
                  placeholder="Azure 应用的 Client Secret（必填）"
                  type="password"
                  show-password
                >
                  <template #prepend>
                    <el-icon><Lock /></el-icon>
                  </template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">授权码模式需要 Client Secret</el-text>
              </el-form-item>
              <el-form-item label="Redirect URI">
                <el-input v-model="siteSettings.microsoft_redirect_uri" placeholder="OAuth 回调地址（如：http://localhost:8000/microsoft/callback）">
                  <template #prepend>
                    <el-icon><Link /></el-icon>
                  </template>
                </el-input>
                <el-text size="small" type="info" style="margin-top:4px">必须与 Azure 应用配置中的重定向 URI 完全一致</el-text>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="saveSettings" size="large">
                  <el-icon><Check /></el-icon>
                  保存设置
                </el-button>
              </el-form-item>
            </el-form>
          </el-card>
        </div>

        <!-- 用户管理 -->
        <div v-if="active === 'users'" class="users-section">
          <div class="section-header">
            <h2>用户管理</h2>
            <el-button type="primary" @click="refreshUsers">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-card>
            <el-table :data="users" style="width: 100%">
              <el-table-column prop="email" label="邮箱" min-width="200" />
              <el-table-column prop="display_name" label="显示名" min-width="150" />
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-tag v-if="row.is_admin" type="danger">管理员</el-tag>
                  <el-tag v-else-if="getUserBanStatus(row)" type="warning">封禁</el-tag>
                  <el-tag v-else type="info">用户</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="角色数" width="100">
                <template #default="{ row }">
                  {{ row.profile_count || 0 }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="120" fixed="right">
                <template #default="{ row }">
                  <el-button
                    size="small"
                    type="primary"
                    @click="showUserDetailDialog(row)"
                  >
                    查看详情
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>

        <!-- 邀请码管理 -->
        <div v-if="active === 'invites'" class="invites-section">
          <div class="section-header">
            <h2>邀请码管理</h2>
            <div style="display: flex; gap: 12px;">
              <el-button type="primary" @click="loadInvites">
                <el-icon><Refresh /></el-icon>
                刷新
              </el-button>
              <el-button type="success" @click="showInviteDialog">
                <el-icon><Plus /></el-icon>
                创建邀请码
              </el-button>
            </div>
          </div>

          <el-card>
            <el-table :data="invites" style="width: 100%">
              <el-table-column prop="code" label="邀请码" min-width="300">
                <template #default="{ row }">
                  <el-text copyable>{{ row.code }}</el-text>
                </template>
              </el-table-column>
              <el-table-column label="使用次数" width="150">
                <template #default="{ row }">
                  <span :style="{ color: getRemainingColor(row) }">
                    {{ row.used_count || 0 }} / {{ row.total_uses || '∞' }}
                  </span>
                  <el-tag
                    v-if="row.total_uses && row.used_count >= row.total_uses"
                    type="danger"
                    size="small"
                    style="margin-left: 8px;"
                  >
                    已用完
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="used_by" label="最后使用者" min-width="200" />
              <el-table-column label="创建时间" width="180">
                <template #default="{ row }">
                  {{ formatDate(row.created_at) }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="100" fixed="right">
                <template #default="{ row }">
                  <el-button
                    size="small"
                    type="danger"
                    @click="deleteInvite(row)"
                  >
                    删除
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>
      </el-main>
    </el-container>

    <!-- 邀请码创建弹窗 -->
    <el-dialog
      v-model="inviteDialogVisible"
      title="创建邀请码"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-width="100px">
        <el-form-item label="生成方式">
          <el-radio-group v-model="inviteMode">
            <el-radio value="auto">自动生成</el-radio>
            <el-radio value="manual">手动输入</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="inviteMode === 'manual'" label="邀请码">
          <el-input
            v-model="customInviteCode"
            placeholder="请输入自定义邀请码（6-32个字符）"
            maxlength="32"
            show-word-limit
          />
          <el-text size="small" type="info" style="margin-top: 8px;">
            支持字母、数字和常见符号，建议使用易记的格式
          </el-text>
        </el-form-item>

        <el-form-item v-if="inviteMode === 'auto'" label="预览">
          <el-text type="success" size="large" style="font-family: monospace;">
            {{ previewInviteCode }}
          </el-text>
          <el-button
            link
            type="primary"
            @click="refreshPreview"
            style="margin-left: 12px;"
          >
            <el-icon><Refresh /></el-icon>
            换一个
          </el-button>
        </el-form-item>

        <el-form-item label="使用次数">
          <el-radio-group v-model="inviteUsesMode" style="margin-bottom: 12px;">
            <el-radio value="limited">限制次数</el-radio>
            <el-radio value="unlimited">无限使用</el-radio>
          </el-radio-group>
          <el-input-number
            v-if="inviteUsesMode === 'limited'"
            v-model="inviteUses"
            :min="1"
            :max="1000"
            controls-position="right"
            style="width: 100%;"
          />
          <el-text v-if="inviteUsesMode === 'limited'" size="small" type="info" style="margin-top: 8px; display: block;">
            设置该邀请码可以被使用的次数
          </el-text>
          <el-text v-else size="small" type="info" style="margin-top: 8px; display: block;">
            该邀请码可以被无限次使用
          </el-text>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="inviteDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmCreateInvite" :loading="creating">
          <el-icon><Check /></el-icon>
          创建
        </el-button>
      </template>
    </el-dialog>

    <!-- 重置用户密码对话框 -->
    <el-dialog
      v-model="resetPasswordDialogVisible"
      title="重置用户密码"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-width="100px">
        <el-form-item label="用户邮箱">
          <el-input :value="currentUser?.email" disabled />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input
            v-model="resetPasswordForm.new_password"
            type="password"
            placeholder="请输入新密码（至少6位）"
            show-password
          />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input
            v-model="resetPasswordForm.confirm_password"
            type="password"
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPasswordDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmResetPassword" :loading="resetting">
          <el-icon><Check /></el-icon>
          确认重置
        </el-button>
      </template>
    </el-dialog>

    <!-- 用户详情对话框 -->
    <el-dialog
      v-model="userDetailDialogVisible"
      :title="currentUser?.email || '用户详情'"
      width="650px"
      :close-on-click-modal="false"
      :destroy-on-close="true"
      class="user-detail-dialog"
    >
      <div v-if="currentUser" class="user-detail-content">
        <!-- 用户状态卡片 -->
        <div class="user-status-card">
          <el-avatar :size="80" class="user-detail-avatar">
            {{ currentUser.email.charAt(0).toUpperCase() }}
          </el-avatar>
          <div class="user-detail-info">
            <h3>{{ currentUser.display_name || '未设置显示名' }}</h3>
            <p class="user-email">{{ currentUser.email }}</p>
            <div class="user-status-tag">
              <el-tag v-if="currentUser.is_admin" type="danger" size="large" effect="dark" class="status-tag">
                <el-icon style="vertical-align: middle;"><User /></el-icon>
                <span style="margin-left: 6px; vertical-align: middle;">管理员</span>
              </el-tag>
              <el-tag v-else-if="getUserBanStatus(currentUser)" type="warning" size="large" effect="dark" class="status-tag">
                <el-icon style="vertical-align: middle;"><Warning /></el-icon>
                <span style="margin-left: 6px; vertical-align: middle;">封禁中</span>
              </el-tag>
              <el-tag v-else type="success" size="large" effect="dark" class="status-tag">
                <el-icon style="vertical-align: middle;"><CircleCheck /></el-icon>
                <span style="margin-left: 6px; vertical-align: middle;">正常用户</span>
              </el-tag>
            </div>
          </div>
        </div>

        <!-- 详细信息 -->
        <div class="user-info-grid">
          <div class="info-item">
            <span class="info-label">用户ID</span>
            <el-text class="info-value" copyable>{{ currentUser.id }}</el-text>
          </div>
          <div class="info-item">
            <span class="info-label">角色数量</span>
            <span class="info-value">{{ currentUser.profile_count || 0 }}</span>
          </div>
          <div v-if="getUserBanStatus(currentUser)" class="info-item info-full">
            <span class="info-label">封禁剩余</span>
            <span class="info-value ban-time">{{ formatBanRemaining(currentUser.banned_until) }}</span>
          </div>
        </div>

        <!-- 操作按钮组 -->
        <el-divider />

        <div class="action-section">
          <div class="action-row">
            <el-button
              class="action-btn"
              :type="currentUser.is_admin ? 'warning' : 'primary'"
              @click="toggleAdmin(currentUser)"
              :disabled="isCurrentUserSelf(currentUser)"
              size="large"
            >
              <el-icon><User /></el-icon>
              <span>{{ currentUser.is_admin ? '取消管理员' : '设为管理员' }}</span>
            </el-button>

            <el-button
              v-if="!getUserBanStatus(currentUser)"
              class="action-btn"
              type="warning"
              @click="showBanDialog"
              :disabled="currentUser.is_admin"
              size="large"
            >
              <el-icon><Warning /></el-icon>
              <span>封禁用户</span>
            </el-button>
            <el-button
              v-else
              class="action-btn"
              type="success"
              @click="unbanUser(currentUser)"
              size="large"
            >
              <el-icon><CircleCheck /></el-icon>
              <span>解除封禁</span>
            </el-button>
          </div>

          <div class="action-row">
            <el-button
              class="action-btn"
              @click="showResetPasswordDialog(currentUser)"
              size="large"
            >
              <el-icon><Key /></el-icon>
              <span>重置密码</span>
            </el-button>

            <el-button
              class="action-btn"
              type="danger"
              @click="deleteUser(currentUser)"
              :disabled="currentUser.is_admin"
              size="large"
            >
              <el-icon><Delete /></el-icon>
              <span>删除用户</span>
            </el-button>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 封禁用户对话框 -->
    <el-dialog
      v-model="banDialogVisible"
      title="封禁用户"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-alert
        type="warning"
        :closable="false"
        style="margin-bottom: 20px;"
      >
        <template #title>
          <div style="font-weight: 600;">封禁说明</div>
        </template>
        封禁后，用户将无法通过 Minecraft 客户端登录游戏，但仍可以正常访问皮肤站进行皮肤管理等操作。
      </el-alert>

      <el-form label-width="100px">
        <el-form-item label="用户">
          <el-text>{{ currentUser?.email }}</el-text>
        </el-form-item>

        <el-form-item label="封禁时长">
          <div class="ban-duration-wrapper">
            <el-radio-group v-model="banDurationType" class="ban-type-selector">
              <el-radio value="preset">预设时长</el-radio>
              <el-radio value="custom">自定义时长</el-radio>
            </el-radio-group>

            <div v-if="banDurationType === 'preset'" class="duration-content">
              <div class="preset-grid">
                <el-button
                  v-for="preset in presetDurations"
                  :key="preset.value"
                  :type="banPresetDuration === preset.value ? 'primary' : ''"
                  @click="banPresetDuration = preset.value"
                  size="default"
                >
                  {{ preset.label }}
                </el-button>
              </div>
            </div>

            <div v-if="banDurationType === 'custom'" class="duration-content">
              <el-input-number
                v-model="banCustomHours"
                :min="1"
                :max="8760"
                :step="1"
                controls-position="right"
                size="large"
                style="width: 100%;"
              />
              <el-text size="small" type="info" class="duration-hint">
                输入小时数（最多365天 = 8760小时）
              </el-text>
            </div>
          </div>
        </el-form-item>

        <el-form-item label="解封时间">
          <el-text type="primary" size="large" style="font-weight: 600;">{{ formatBanUntilTime() }}</el-text>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="banDialogVisible = false">取消</el-button>
        <el-button type="danger" @click="confirmBanUser" :loading="banning">
          <el-icon><Check /></el-icon>
          确认封禁
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Tools, Setting, User, Ticket, Back, Check, Refresh, Plus, Warning, CircleCheck, Key, Delete, InfoFilled, Clock, Lock, Link
} from '@element-plus/icons-vue'

const route = useRoute()
const users = ref([])
const invites = ref([])
const inviteDialogVisible = ref(false)
const inviteMode = ref('auto')
const customInviteCode = ref('')
const previewInviteCode = ref('')
const creating = ref(false)
const resetPasswordDialogVisible = ref(false)
const currentUser = ref(null)
const resetPasswordForm = ref({ new_password: '', confirm_password: '' })
const resetting = ref(false)
const inviteUsesMode = ref('limited')
const inviteUses = ref(1)
const userDetailDialogVisible = ref(false)
const banDialogVisible = ref(false)
const banDurationType = ref('preset')
const banPresetDuration = ref(24)
const banCustomHours = ref(24)
const banning = ref(false)

const presetDurations = [
  { label: '1小时', value: 1 },
  { label: '6小时', value: 6 },
  { label: '1天', value: 24 },
  { label: '3天', value: 72 },
  { label: '7天', value: 168 },
  { label: '30天', value: 720 }
]

const siteSettings = ref({
  site_name: '皮肤站',
  site_url: '',
  require_invite: false,
  allow_register: true,
  max_texture_size: 1024,
  rate_limit_enabled: true,
  rate_limit_auth_attempts: 5,
  rate_limit_auth_window: 15,
  jwt_expire_days: 7,
  microsoft_client_id: '',
  microsoft_client_secret: '',
  microsoft_redirect_uri: ''
})

const activeRoute = computed(() => route.path)
const active = computed(() => {
  if (route.path.includes('/users')) return 'users'
  if (route.path.includes('/invites')) return 'invites'
  return 'settings'
})

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function formatDate(timestamp) {
  if (!timestamp) return '-'
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN')
}

async function loadSettings() {
  try {
    const res = await axios.get('/admin/settings', { headers: authHeaders() })
    if (res.data) {
      Object.assign(siteSettings.value, res.data)
    }
  } catch (e) {
    console.error('Load settings error:', e)
  }
}

async function saveSettings() {
  try {
    await axios.post('/admin/settings', siteSettings.value, { headers: authHeaders() })
    ElMessage.success('保存成功')
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function refreshUsers() {
  try {
    const res = await axios.get('/admin/users', { headers: authHeaders() })
    users.value = res.data
  } catch (e) {
    ElMessage.error('获取用户列表失败')
  }
}

async function toggleAdmin(user) {
  try {
    await ElMessageBox.confirm(
      `确定要${user.is_admin ? '取消' : '设置'} ${user.email} 的管理员权限吗？`,
      '确认操作',
      { type: 'warning' }
    )
    // 阻止管理员取消自己的管理员权限
    const token = localStorage.getItem('jwt')
    if (token) {
      const payload = JSON.parse(atob(token.split('.')[1]))
      if (payload.sub === user.id && user.is_admin) {
        ElMessage.warning('不能取消自身的管理员权限')
        return
      }
    }
    await axios.post(`/admin/users/${user.id}/toggle-admin`, {}, { headers: authHeaders() })
    ElMessage.success('操作成功')
    refreshUsers()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('操作失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

async function deleteUser(user) {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 ${user.email} 吗？此操作将删除该用户的所有数据！`,
      '危险操作',
      { type: 'error', confirmButtonText: '确定删除' }
    )
    await axios.delete(`/admin/users/${user.id}`, { headers: authHeaders() })
    ElMessage.success('删除成功')
    refreshUsers()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

function showResetPasswordDialog(user) {
  currentUser.value = user
  resetPasswordForm.value = { new_password: '', confirm_password: '' }
  resetPasswordDialogVisible.value = true
}

async function confirmResetPassword() {
  if (!resetPasswordForm.value.new_password) {
    ElMessage.error('请输入新密码')
    return
  }
  if (resetPasswordForm.value.new_password.length < 6) {
    ElMessage.error('密码长度不能少于6个字符')
    return
  }
  if (resetPasswordForm.value.new_password !== resetPasswordForm.value.confirm_password) {
    ElMessage.error('两次输入的密码不一致')
    return
  }

  try {
    resetting.value = true
    await axios.post('/admin/users/reset-password', {
      user_id: currentUser.value.id,
      new_password: resetPasswordForm.value.new_password
    }, { headers: authHeaders() })
    ElMessage.success('密码重置成功')
    resetPasswordDialogVisible.value = false
  } catch (error) {
    console.error(error)
    ElMessage.error(error.response?.data?.detail || '重置失败')
  } finally {
    resetting.value = false
  }
}

async function loadInvites() {
  try {
    const res = await axios.get('/admin/invites', { headers: authHeaders() })
    invites.value = res.data
  } catch (e) {
    ElMessage.error('获取邀请码列表失败')
  }
}

function generateRandomCode() {
  // 生成一个随机的邀请码（16个字符，URL安全）
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'
  let code = ''
  for (let i = 0; i < 16; i++) {
    code += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return code
}

function showInviteDialog() {
  inviteMode.value = 'auto'
  customInviteCode.value = ''
  previewInviteCode.value = generateRandomCode()
  inviteUsesMode.value = 'limited'
  inviteUses.value = 1
  inviteDialogVisible.value = true
}

function refreshPreview() {
  previewInviteCode.value = generateRandomCode()
}

function getRemainingColor(row) {
  if (!row.total_uses) return '#67c23a' // 无限制，绿色
  const remaining = row.total_uses - (row.used_count || 0)
  const percentage = remaining / row.total_uses
  if (percentage <= 0) return '#f56c6c' // 红色
  if (percentage <= 0.3) return '#e6a23c' // 黄色
  return '#67c23a' // 绿色
}

async function confirmCreateInvite() {
  const code = inviteMode.value === 'auto' ? previewInviteCode.value : customInviteCode.value.trim()

  // 验证邀请码
  if (!code) {
    ElMessage.warning('请输入邀请码')
    return
  }

  if (code.length < 6) {
    ElMessage.warning('邀请码至少需要6个字符')
    return
  }

  if (!/^[a-zA-Z0-9_-]+$/.test(code)) {
    ElMessage.warning('邀请码只能包含字母、数字、下划线和横线')
    return
  }

  creating.value = true
  try {
    const payload = { code }

    // 添加使用次数
    if (inviteUsesMode.value === 'unlimited') {
      payload.total_uses = null
    } else {
      payload.total_uses = inviteUses.value
    }

    const res = await axios.post('/admin/invites', payload, { headers: authHeaders() })
    ElMessage.success('创建成功！邀请码：' + res.data.code)
    inviteDialogVisible.value = false
    loadInvites()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.detail || e.message))
  } finally {
    creating.value = false
  }
}

function getUserBanStatus(user) {
  if (!user.banned_until) return false
  return Date.now() < user.banned_until
}

function formatBanRemaining(bannedUntil) {
  if (!bannedUntil) return ''
  const remaining = bannedUntil - Date.now()
  if (remaining <= 0) return '已到期'

  const days = Math.floor(remaining / (1000 * 60 * 60 * 24))
  const hours = Math.floor((remaining % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((remaining % (1000 * 60 * 60)) / (1000 * 60))

  if (days > 0) {
    return `${days}天 ${hours}小时`
  } else if (hours > 0) {
    return `${hours}小时 ${minutes}分钟`
  } else {
    return `${minutes}分钟`
  }
}

function formatBanUntilTime() {
  let hours = 0
  if (banDurationType.value === 'preset') {
    hours = banPresetDuration.value
  } else {
    hours = banCustomHours.value
  }

  const until = new Date(Date.now() + hours * 60 * 60 * 1000)
  return until.toLocaleString('zh-CN')
}

function isCurrentUserSelf(user) {
  const token = localStorage.getItem('jwt')
  if (!token) return false
  const payload = JSON.parse(atob(token.split('.')[1]))
  return payload.sub === user.id
}

function showUserDetailDialog(user) {
  currentUser.value = user
  userDetailDialogVisible.value = true
}

function showBanDialog() {
  banDurationType.value = 'preset'
  banPresetDuration.value = 24
  banCustomHours.value = 24
  banDialogVisible.value = true
}

async function confirmBanUser() {
  if (!currentUser.value) return

  let hours = 0
  if (banDurationType.value === 'preset') {
    hours = banPresetDuration.value
  } else {
    hours = banCustomHours.value
  }

  const bannedUntil = Date.now() + hours * 60 * 60 * 1000

  try {
    banning.value = true
    await axios.post(`/admin/users/${currentUser.value.id}/ban`, {
      banned_until: bannedUntil
    }, { headers: authHeaders() })
    ElMessage.success('封禁成功')
    banDialogVisible.value = false
    refreshUsers()
    // 更新当前用户信息
    const updatedUser = users.value.find(u => u.id === currentUser.value.id)
    if (updatedUser) {
      currentUser.value = updatedUser
    }
  } catch (error) {
    console.error(error)
    ElMessage.error(error.response?.data?.detail || '封禁失败')
  } finally {
    banning.value = false
  }
}

async function unbanUser(user) {
  try {
    await ElMessageBox.confirm('确定要解除该用户的封禁吗？', '确认操作', { type: 'info' })
    await axios.post(`/admin/users/${user.id}/unban`, {}, { headers: authHeaders() })
    ElMessage.success('解封成功')
    refreshUsers()
    // 更新当前用户信息
    const updatedUser = users.value.find(u => u.id === user.id)
    if (updatedUser) {
      currentUser.value = updatedUser
    }
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('解封失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

async function deleteInvite(invite) {
  try {
    await ElMessageBox.confirm('确定要删除此邀请码吗？', '确认', { type: 'warning' })
    await axios.delete(`/admin/invites/${invite.code}`, { headers: authHeaders() })
    ElMessage.success('删除成功')
    loadInvites()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

onMounted(() => {
  loadSettings()
  if (route.path.includes('/users')) {
    refreshUsers()
  } else if (route.path.includes('/invites')) {
    loadInvites()
  }
})

// 监听路由变化，自动刷新对应页面数据
watch(() => route.path, (newPath) => {
  if (newPath.includes('/admin/settings')) {
    loadSettings()
  } else if (newPath.includes('/admin/users')) {
    refreshUsers()
  } else if (newPath.includes('/admin/invites')) {
    loadInvites()
  }
})
</script>

<style scoped>
.admin-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.admin-container :deep(.el-container) {
  min-height: 100vh;
}

.admin-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
  min-height: 100vh;
}

.admin-title {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 20px 0;
  margin: 0 16px 4px 16px;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
}

.title-divider {
  height: 1px;
  background: #ebeef5;
  margin: 0 16px 4px 16px;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 16px;
  padding: 0 16px !important;
  border-radius: 8px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.sidebar-menu .el-menu-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 3px;
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  transform: translateX(-100%);
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-menu .el-menu-item:hover::before {
  transform: translateX(0);
}

.sidebar-menu .el-menu-item:hover {
  background-color: #ecf5ff;
  transform: translateX(4px);
}

.sidebar-menu .el-menu-item.is-active {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
  transform: translateX(0);
}

.admin-main {
  padding: 30px;
  background: #f5f7fa;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  animation: fadeInUp 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  width: 100%;
  max-width: 800px;
}

@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.section-header h2 {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.settings-card {
  width: 100%;
  max-width: 800px;
  padding: 30px;
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1) 0.1s backwards;
}

@keyframes cardSlideIn {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.settings-card :deep(.el-form-item) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card :deep(.el-form-item:hover) {
  transform: translateX(4px);
}

.settings-card :deep(.el-button) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card :deep(.el-button:hover) {
  transform: scale(1.05);
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.3);
}

.settings-section,
.users-section,
.invites-section {
  width: 100%;
  max-width: 1200px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.user-detail-content {
  padding: 0;
}

.user-status-card {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 24px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 12px;
  margin-bottom: 24px;
  color: white;
}

.user-detail-avatar {
  background: rgba(255, 255, 255, 0.2);
  color: white;
  font-size: 32px;
  font-weight: 600;
  border: 3px solid rgba(255, 255, 255, 0.3);
  flex-shrink: 0;
}

.user-detail-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
}

.user-detail-info h3 {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  width: 100%;
}

.user-email {
  margin: 0 0 12px 0;
  opacity: 0.9;
  font-size: 14px;
  width: 100%;
}

.user-status-tag {
  margin-top: 8px;
  width: auto;
}

.user-status-tag .status-tag {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
}

.user-status-tag .status-tag .el-icon {
  display: inline-flex;
  align-items: center;
}

.user-info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-bottom: 16px;
}

.info-item {
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-item.info-full {
  grid-column: span 2;
}

.info-label {
  font-size: 13px;
  color: #909399;
  font-weight: 500;
}

.info-value {
  font-size: 15px;
  color: #303133;
  font-weight: 600;
}

.info-value.ban-time {
  color: #e6a23c;
  font-size: 16px;
}

.action-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.action-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.action-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.ban-duration-wrapper {
  width: 100%;
}

.ban-type-selector {
  width: 100%;
  margin-bottom: 16px;
}

.ban-type-selector .el-radio {
  margin-right: 24px;
}

.duration-content {
  width: 100%;
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  width: 100%;
}

.preset-grid .el-button {
  width: 100%;
  margin: 0;
  padding: 8px 15px;
  justify-content: center;
}

.duration-hint {
  display: block;
  margin-top: 12px;
  padding-left: 4px;
}

.ban-dialog-content {
  padding: 4px 0;
}

.ban-alert {
  margin-bottom: 24px;
  border-radius: 8px;
}

.alert-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
}

.alert-content {
  margin-top: 8px;
  line-height: 1.6;
  font-size: 14px;
}

.ban-user-info {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 24px;
}

.ban-user-info .label {
  font-weight: 500;
  color: #606266;
}

.ban-duration-section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 16px 0;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.duration-radio-group {
  width: 100%;
  display: flex;
  flex-direction: column;
}

.duration-radio {
  margin-bottom: 12px;
}

.radio-label {
  font-weight: 500;
}

.preset-options {
  margin-left: 28px;
  margin-top: 12px;
  margin-bottom: 8px;
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.preset-grid .el-radio-button {
  width: 100%;
}

.custom-options {
  margin-left: 28px;
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.custom-input {
  width: 100%;
}

.input-suffix {
  color: #909399;
  font-size: 13px;
}

.custom-hint {
  display: block;
  padding-left: 4px;
}

.ban-result {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px;
  background: linear-gradient(135deg, #ffeaa7 0%, #fdcb6e 100%);
  border-radius: 8px;
}

.result-icon {
  font-size: 28px;
  color: #e17055;
}

.result-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.result-label {
  font-size: 13px;
  color: #636e72;
  font-weight: 500;
}

.result-time {
  font-size: 16px;
  color: #2d3436;
  font-weight: 600;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
