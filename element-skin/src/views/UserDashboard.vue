<template>
  <div class="dashboard-container">
    <el-container style="height:100%">
      <el-aside width="220px" class="dashboard-sidebar">
        <div class="user-info">
          <el-avatar :size="60" class="user-avatar">{{ emailInitial }}</el-avatar>
          <div class="user-name">{{ user?.display_name || user?.email || '用户' }}</div>
          <div class="user-status">
            <el-tag v-if="user?.is_admin" type="danger" size="small">管理员</el-tag>
            <el-tag v-else-if="getUserBanStatus()" type="warning" size="small">封禁</el-tag>
            <el-tag v-else type="info" size="small">用户</el-tag>
          </div>
        </div>
        <el-menu :default-active="activeRoute" mode="vertical" router class="sidebar-menu">
          <el-menu-item index="/dashboard/wardrobe">
            <el-icon><Box /></el-icon>
            <span>我的衣柜</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/roles">
            <el-icon><User /></el-icon>
            <span>角色管理</span>
          </el-menu-item>
          <el-menu-item index="/dashboard/profile">
            <el-icon><Setting /></el-icon>
            <span>个人资料</span>
          </el-menu-item>
          <div v-if="user?.is_admin" class="menu-divider"></div>
          <el-menu-item v-if="user?.is_admin" index="/admin" class="admin-menu-item">
            <el-icon><Tools /></el-icon>
            <span>管理面板</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-main class="dashboard-main">
        <div v-if="active === 'wardrobe'" class="wardrobe-section">
          <div class="section-header">
            <h2>我的衣柜</h2>
            <el-button type="primary" @click="showUploadDialog = true" size="large">
              <el-icon><Upload /></el-icon>
              <span style="margin-left:8px">上传纹理</span>
            </el-button>
          </div>

          <div class="textures-grid" v-if="textures.length > 0">
            <div class="texture-card" v-for="tex in textures" :key="tex.hash + tex.type">
              <div class="texture-preview">
                <SkinViewer
                  v-if="tex.type === 'skin'"
                  :skinUrl="texturesUrl(tex.hash)"
                  :width="200"
                  :height="280"
                  @load="handleTextureLoad(tex.hash)"
                />
                <CapeViewer
                  v-else
                  :capeUrl="texturesUrl(tex.hash)"
                  :width="200"
                  :height="280"
                />
                <!-- 皮肤分辨率标签 -->
                <div
                  v-if="tex.type === 'skin' && textureResolutions.get(tex.hash)"
                  class="resolution-badge"
                  :style="getResolutionBadgeStyle(textureResolutions.get(tex.hash))"
                >
                  {{ textureResolutions.get(tex.hash) }}x
                </div>
              </div>
              <div class="texture-info">
                <div class="texture-type-badge" :class="tex.type">
                  {{ tex.type === 'skin' ? '皮肤' : '披风' }}
                </div>
                <div class="texture-note" @click="startEditNote(tex)" v-if="editingNoteHash !== tex.hash">
                  {{ tex.note || '无备注' }}
                </div>
                <el-input
                  v-else
                  v-model="editingNoteValue"
                  placeholder="输入备注，最多200字"
                  size="default"
                  class="texture-note-input"
                  autofocus
                  @blur="finishEditNote(tex)"
                  @keyup.enter="finishEditNote(tex)"
                />
              </div>
              <div class="texture-actions">
                <el-button class="action-btn action-btn-primary" @click="openApplyDialog(tex)">
                  <el-icon><Check /></el-icon>
                  <span>使用</span>
                </el-button>
                <el-button class="action-btn action-btn-danger" @click="deleteMyTexture(tex.hash, tex.type)">
                  <el-icon><Delete /></el-icon>
                  <span>删除</span>
                </el-button>
              </div>
            </div>
          </div>

          <el-empty v-else description="还没有纹理，快去上传吧！" />
        </div>

        <div v-if="active === 'roles' && user" class="roles-section">
          <div class="section-header">
            <h2>角色管理</h2>
            <div class="header-actions">
              <el-button type="success" size="large" @click="startMicrosoftAuth">
                <el-icon><Connection /></el-icon>
                <span style="margin-left:8px">绑定正版角色</span>
              </el-button>
              <el-button type="primary" size="large" @click="showCreateRoleDialog = true">
                <el-icon><Plus /></el-icon>
                <span style="margin-left:8px">新建角色</span>
              </el-button>
            </div>
          </div>

          <div class="roles-grid">
            <div v-for="profile in user.profiles || []" :key="profile.id" class="role-card">
              <div class="role-preview">
                <SkinViewer
                  v-if="profile.skin_hash"
                  :skinUrl="texturesUrl(profile.skin_hash)"
                  :capeUrl="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
                  :width="200"
                  :height="280"
                />
                <el-empty v-else description="未设置皮肤" :image-size="120" />
              </div>
              <div class="role-info">
                <div class="role-name">{{ profile.name }}</div>
                <div class="role-model">模型: {{ profile.model || 'default' }}</div>
              </div>
              <div class="role-actions">
                <el-button
                  class="action-btn action-btn-danger"
                  @click="deleteRole(profile.id)"
                  size="default"
                >
                  <span class="btn-content">
                    <el-icon class="btn-icon"><Delete /></el-icon>
                    <span class="btn-label">删除</span>
                  </span>
                </el-button>

                <el-button
                  v-if="profile.skin_hash"
                  class="action-btn action-btn-warning"
                  @click="clearRoleSkin(profile.id)"
                  size="default"
                >
                  <span class="btn-content">
                    <el-icon class="btn-icon"><Close /></el-icon>
                    <span class="btn-label">皮肤</span>
                  </span>
                </el-button>

                <el-button
                  v-if="profile.cape_hash"
                  class="action-btn action-btn-warning"
                  @click="clearRoleCape(profile.id)"
                  size="default"
                >
                  <span class="btn-content">
                    <el-icon class="btn-icon"><Close /></el-icon>
                    <span class="btn-label">披风</span>
                  </span>
                </el-button>
              </div>
            </div>
          </div>
        </div>

        <div v-if="active === 'profile' && user" class="profile-section">
          <div class="section-header">
            <h2>个人资料</h2>
          </div>

          <el-card class="profile-form-card">
            <div class="profile-header">
              <el-avatar :size="72" class="profile-avatar">{{ emailInitial }}</el-avatar>
              <div class="profile-meta">
                <h3>{{ user.display_name || '未设置显示名' }}</h3>
                <p>{{ user.email }}</p>
              </div>
            </div>

            <el-divider />

            <!-- 封禁状态显示 -->
            <el-alert
              v-if="getUserBanStatus()"
              type="warning"
              :closable="false"
              style="margin-bottom: 20px;"
            >
              <template #title>
                <div style="font-weight: 600; font-size: 16px;">账号已被封禁</div>
              </template>
              <div style="margin-top: 8px; font-size: 14px;">
                <p style="margin: 4px 0;">您的账号已被管理员封禁，暂时无法通过 Minecraft 客户端登录游戏。</p>
                <p style="margin: 4px 0;">但您仍可以正常访问皮肤站进行皮肤管理等操作。</p>
                <p style="margin: 8px 0 0 0; font-size: 15px; color: #e6a23c;">
                  <el-icon><Clock /></el-icon>
                  <strong>封禁剩余时间：{{ formatBanRemaining() }}</strong>
                </p>
                <p style="margin: 4px 0 0 0; color: #909399; font-size: 13px;">
                  解封时间：{{ formatBanUntilTime() }}
                </p>
              </div>
            </el-alert>

            <el-form label-width="120px" :model="form" label-position="left">
              <el-form-item label="邮箱">
                <el-input v-model="form.email" placeholder="请输入邮箱" />
              </el-form-item>
              <el-form-item label="显示名">
                <el-input v-model="form.display_name" placeholder="显示名称（可选）" />
              </el-form-item>

              <el-divider content-position="left">修改密码</el-divider>

              <el-form-item label="旧密码">
                <el-input type="password" v-model="form.old_password" placeholder="请输入旧密码" show-password />
              </el-form-item>
              <el-form-item label="新密码">
                <el-input type="password" v-model="form.new_password" placeholder="请输入新密码（留空则不修改）" show-password />
              </el-form-item>
              <el-form-item label="确认新密码">
                <el-input type="password" v-model="form.confirm_password" placeholder="请再次输入新密码" show-password />
              </el-form-item>

              <div class="profile-actions">
                <el-button type="primary" @click="updateProfile" size="large">
                  <el-icon><Check /></el-icon>
                  保存修改
                </el-button>
                <el-button type="danger" @click="showDeleteDialog = true" size="large" v-if="!user.is_admin">
                  <el-icon><Delete /></el-icon>
                  注销账号
                </el-button>
              </div>
            </el-form>
          </el-card>
        </div>
      </el-main>
    </el-container>

    <!-- 上传对话框 -->
    <el-dialog v-model="showUploadDialog" title="上传纹理" width="500px" class="upload-dialog">
      <el-form label-width="100px" :model="uploadForm" class="upload-form">
        <el-form-item label="选择文件" class="upload-form-item">
          <el-upload
            ref="uploadRef"
            :auto-upload="false"
            :limit="1"
            accept=".png"
            :on-change="handleFileChange"
            drag
            class="upload-wrapper"
          >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
              将 PNG 文件拖到此处，或<em>点击上传</em>
            </div>
            <template #tip>
              <div class="el-upload__tip">
                仅支持 PNG 格式的皮肤文件
              </div>
            </template>
          </el-upload>
        </el-form-item>
        <el-form-item label="纹理类型">
          <el-select v-model="uploadForm.texture_type" placeholder="选择类型" style="width:100%">
            <el-option label="皮肤 (Skin)" value="skin" />
            <el-option label="披风 (Cape)" value="cape" />
          </el-select>
        </el-form-item>
        <el-form-item label="皮肤模型" v-if="uploadForm.texture_type === 'skin'">
          <el-select v-model="uploadForm.model" placeholder="选择模型" style="width:100%">
            <el-option label="普通 (4px 手臂)" value="default" />
            <el-option label="纤细 (3px 手臂)" value="slim" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="uploadForm.note" placeholder="给这个纹理添加备注（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUploadDialog = false">取消</el-button>
        <el-button type="primary" @click="doUpload">
          <el-icon><Upload /></el-icon>
          确认上传
        </el-button>
      </template>
    </el-dialog>

    <!-- 应用纹理对话框 -->
    <el-dialog v-model="showApplyDialog" title="应用纹理到角色" width="450px">
      <el-form label-width="100px" :model="applyForm">
        <el-form-item label="选择角色">
          <el-select v-model="applyForm.profile_id" placeholder="选择要应用的角色" style="width:100%">
            <el-option
              v-for="p in user?.profiles || []"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            >
              <span>{{ p.name }}</span>
              <span style="float:right; color: #8492a6; font-size: 13px">{{ p.model || 'default' }}</span>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showApplyDialog = false">取消</el-button>
        <el-button type="primary" @click="doApply">
          <el-icon><Check /></el-icon>
          确认应用
        </el-button>
      </template>
    </el-dialog>

    <!-- 新建角色对话框 -->
    <el-dialog v-model="showCreateRoleDialog" title="新建角色" width="420px">
      <el-form label-width="100px">
        <el-form-item label="角色名称">
          <el-input v-model="newRoleName" placeholder="请输入角色名称" maxlength="32" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateRoleDialog = false">取消</el-button>
        <el-button type="primary" @click="createRole">
          <el-icon><Check /></el-icon>
          创建
        </el-button>
      </template>
    </el-dialog>

    <!-- 注销账号确认对话框 -->
    <el-dialog
      v-model="showDeleteDialog"
      title="确认注销账号"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="警告：该操作不可逆！"
        type="error"
        description="注销账号后，您的所有数据（包括角色、皮肤、披风等）将被永久删除，无法恢复。"
        :closable="false"
        style="margin-bottom: 20px;"
      />
      <p style="font-size: 14px; color: #606266;">
        请输入 <strong style="color: #f56c6c;">注销账号</strong> 来确认操作：
      </p>
      <el-input
        v-model="deleteConfirmText"
        placeholder="请输入：注销账号"
        style="margin-top: 10px;"
      />
      <template #footer>
        <el-button @click="showDeleteDialog = false">取消</el-button>
        <el-button
          type="danger"
          @click="confirmDeleteAccount"
          :disabled="deleteConfirmText !== '注销账号'"
        >
          <el-icon><Delete /></el-icon>
          确认注销
        </el-button>
      </template>
    </el-dialog>

    <!-- 微软正版登录对话框 -->
    <el-dialog
      v-model="showMicrosoftLoginDialog"
      title="绑定正版角色"
      width="600px"
      :close-on-click-modal="false"
      :destroy-on-close="true"
    >
      <div class="microsoft-login-content">
        <!-- 选择角色 -->
        <div v-if="microsoftStep === 'select-profile'" class="step-container">
          <el-result icon="success" title="登录成功！">
            <template #sub-title>
              <div class="profile-selection">
                <p style="margin-bottom: 16px;">检测到正版角色：</p>
                <el-card class="profile-card">
                  <div class="profile-info-display">
                    <el-avatar :size="80" class="profile-avatar">
                      {{ microsoftProfile.name.charAt(0).toUpperCase() }}
                    </el-avatar>
                    <div class="profile-details">
                      <h3>{{ microsoftProfile.name }}</h3>
                      <p>UUID: {{ formatUUID(microsoftProfile.id) }}</p>
                      <el-tag v-if="microsoftProfile.has_game" type="success" size="large">
                        <el-icon><Select /></el-icon>
                        拥有游戏
                      </el-tag>
                      <el-tag v-else type="info" size="large">
                        <el-icon><Warning /></el-icon>
                        Demo 版本
                      </el-tag>
                    </div>
                  </div>
                  <el-divider />
                  <div class="skin-preview">
                    <div v-if="microsoftProfile.skins && microsoftProfile.skins.length > 0">
                      <p><strong>皮肤：</strong>{{ microsoftProfile.skins[0].variant }}</p>
                    </div>
                    <div v-if="microsoftProfile.capes && microsoftProfile.capes.length > 0">
                      <p><strong>披风：</strong>已拥有</p>
                    </div>
                  </div>
                </el-card>
              </div>
            </template>
            <template #extra>
              <el-button type="primary" @click="importMicrosoftProfile" size="large" :loading="importing">
                <el-icon v-if="!importing"><Download /></el-icon>
                导入角色
              </el-button>
            </template>
          </el-result>
        </div>

        <!-- 步骤3: 导入中 -->
        <div v-else-if="microsoftStep === 'importing'" class="step-container">
          <el-result icon="info" title="正在导入角色...">
            <template #sub-title>
              <p>正在下载皮肤和披风，请稍候...</p>
            </template>
          </el-result>
        </div>
      </div>

      <template #footer>
        <el-button @click="cancelMicrosoftLogin">取消</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Box, User, Setting, Upload, UploadFilled, Check, Delete, Plus, Tools, Close, Clock,
  Connection, Link, Loading, Select, Warning, Download
} from '@element-plus/icons-vue'
import SkinViewer from '@/components/SkinViewer.vue'
import CapeViewer from '@/components/CapeViewer.vue'

const route = useRoute()
const router = useRouter()
const user = ref(null)
const newRoleName = ref('')
const showCreateRoleDialog = ref(false)
const form = ref({ email: '', password: '', display_name: '' })
const textures = ref([])
const textureResolutions = ref(new Map()) // 存储每个纹理的分辨率
const editingNoteHash = ref('')
const editingNoteValue = ref('')
const showUploadDialog = ref(false)
const uploadForm = ref({ texture_type: 'skin', model: 'default', note: '', file: null })
const uploadRef = ref(null)
const showApplyDialog = ref(false)
const applyForm = ref({ profile_id: '', texture_type: '', hash: '' })
const showDeleteDialog = ref(false)
const deleteConfirmText = ref('')

// 微软正版登录相关
const showMicrosoftLoginDialog = ref(false)
const microsoftStep = ref('select-profile') // 'select-profile', 'importing'
const microsoftProfile = ref(null) // {id, name, skins, capes, has_game}
const importing = ref(false)

const emailInitial = computed(() => {
  const email = user.value?.email || user.value?.display_name || 'U'
  return email.charAt(0).toUpperCase()
})

// 检查用户是否被封禁
function getUserBanStatus() {
  if (!user.value?.banned_until) return false
  return Date.now() < user.value.banned_until
}

// 格式化封禁剩余时间
function formatBanRemaining() {
  if (!user.value?.banned_until) return ''
  const remaining = user.value.banned_until - Date.now()
  if (remaining <= 0) return '已到期'

  const days = Math.floor(remaining / (1000 * 60 * 60 * 24))
  const hours = Math.floor((remaining % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60))
  const minutes = Math.floor((remaining % (1000 * 60 * 60)) / (1000 * 60))

  if (days > 0) {
    return `${days}天 ${hours}小时 ${minutes}分钟`
  } else if (hours > 0) {
    return `${hours}小时 ${minutes}分钟`
  } else {
    return `${minutes}分钟`
  }
}

// 格式化解封时间
function formatBanUntilTime() {
  if (!user.value?.banned_until) return ''
  const until = new Date(user.value.banned_until)
  return until.toLocaleString('zh-CN')
}

// 根据路由计算当前激活的标签
const activeRoute = computed(() => route.path)
const active = computed(() => {
  if (route.path.includes('/roles')) return 'roles'
  if (route.path.includes('/profile')) return 'profile'
  return 'wardrobe'
})

// 监听路由变化，加载对应数据
watch(() => route.path, (newPath) => {
  if (newPath.includes('/wardrobe')) {
    fetchTextures()
  } else if (newPath.includes('/roles') || newPath.includes('/profile')) {
    if (!user.value) fetchMe()
  }
}, { immediate: true })

function authHeaders() {
  const token = localStorage.getItem('jwt')
  return token ? { Authorization: 'Bearer ' + token } : {}
}

function texturesUrl(hash) {
  if (!hash) return ''
  return (import.meta.env.VITE_API_BASE || '') + '/static/textures/' + hash + '.png'
}

function startEditNote(tex){
  editingNoteHash.value = tex.hash
  editingNoteValue.value = tex.note || ''
}

async function finishEditNote(tex){
  const original = tex.note || ''
  const updated = editingNoteValue.value || ''
  editingNoteHash.value = ''
  editingNoteValue.value = ''
  if (updated === original) return
  try {
    await axios.patch(`/me/textures/${tex.hash}/${tex.type}`, { note: updated }, { headers: authHeaders() })
    tex.note = updated
    ElMessage.success('备注已更新')
  } catch (e) {
    console.error('update note error:', e)
    ElMessage.error('更新备注失败')
  }
}

async function fetchMe() {
  try {
    const res = await axios.get('/me', { headers: authHeaders() })
    user.value = res.data
    form.value.email = user.value.email
    form.value.display_name = user.value.display_name || ''
  } catch (e) {
    console.error('fetchMe error:', e)
    if (e.response?.status === 401 || e.response?.status === 403) {
      ElMessage.error('登录已过期，请重新登录')
      localStorage.removeItem('jwt')
      localStorage.removeItem('accessToken')
      setTimeout(() => {
        router.push('/login')
      }, 1000)
    } else {
      ElMessage.error('获取用户信息失败')
    }
  }
}

onMounted(async () => {
  // 先刷新 token 获取最新的管理员状态
  try {
    const res = await axios.post('/me/refresh-token', {}, { headers: authHeaders() })
    if (res.data.token) {
      localStorage.setItem('token', res.data.token)
    }
  } catch (e) {
    console.warn('Failed to refresh token:', e)
  }

  await fetchMe()

  // 检查是否是Microsoft OAuth回调
  const urlParams = new URLSearchParams(window.location.search)
  const msToken = urlParams.get('ms_token')
  const error = urlParams.get('error')

  if (error) {
    ElMessage.error('微软登录失败: ' + error)
    // 清除URL参数
    router.replace({ query: {} })
  } else if (msToken) {
    // 获取profile数据并显示导入对话框
    try {
      const response = await axios.post('/microsoft/get-profile',
        { ms_token: msToken },
        { headers: authHeaders() }
      )

      microsoftProfile.value = response.data.profile
      microsoftProfile.value.has_game = response.data.has_game
      microsoftStep.value = 'select-profile'
      showMicrosoftLoginDialog.value = true

      ElMessage.success('授权成功！')
    } catch (e) {
      ElMessage.error('获取角色信息失败: ' + (e.response?.data?.detail || e.message))
    }

    // 清除URL参数
    router.replace({ query: {} })
  }

  if (route.path.includes('/wardrobe') || route.path === '/dashboard' || route.path === '/dashboard/') {
    fetchTextures()
  }
})

async function fetchTextures() {
  try {
    const res = await axios.get('/me/textures', { headers: authHeaders() })
    textures.value = res.data
    // 为每个皮肤纹理计算分辨率
    textures.value.forEach(tex => {
      if (tex.type === 'skin') {
        loadTextureResolution(tex.hash)
      }
    })
  } catch (e) {
    console.error(e)
  }
}

function loadTextureResolution(hash) {
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    const resolution = img.width // 假设是正方形或使用宽度
    textureResolutions.value.set(hash, resolution)
  }
  img.src = texturesUrl(hash)
}

function handleTextureLoad(hash) {
  // SkinViewer 加载完成后的回调（如果需要）
}

function getResolutionBadgeStyle(resolution) {
  // 根据分辨率计算颜色，使用渐变色带
  // 64x -> 绿色, 128x -> 黄色, 256x -> 紫色, 512x+ -> 红色
  let hue = 0
  if (resolution <= 64) {
    hue = 120 // 绿色
  } else if (resolution <= 128) {
    // 64-128: 绿色到黄色 (120-60)
    hue = 120 - ((resolution - 64) / 64) * 60
  } else if (resolution <= 256) {
    // 128-256: 黄色到橙色 (60-30)
    hue = 60 - ((resolution - 128) / 128) * 30
  } else if (resolution <= 512) {
    // 256-512: 橙色到红色 (30-0)
    hue = 30 - ((resolution - 256) / 256) * 30
  } else {
    // 512+: 红色到紫红 (0-330)
    hue = 330
  }

  const saturation = 58 // 适中的饱和度，柔和但不暗淡
  const lightness = 65 // 适中的亮度，明亮但不刺眼

  return {
    background: `linear-gradient(135deg, hsl(${hue}, ${saturation}%, ${lightness}%), hsl(${hue + 15}, ${saturation - 5}%, ${lightness - 3}%))`,
    boxShadow: `0 2px 6px hsla(${hue}, ${saturation}%, ${lightness - 15}%, 0.25)` // 稍微增强阴影
  }
}

function handleFileChange(file) {
  uploadForm.value.file = file.raw
}

async function doUpload() {
  const file = uploadForm.value.file
  if (!file) return ElMessage.error('请选择文件')
  if (!uploadForm.value.texture_type) return ElMessage.error('请选择纹理类型')

  const formData = new FormData()
  formData.append('file', file)
  formData.append('texture_type', uploadForm.value.texture_type)
  if (uploadForm.value.texture_type === 'skin') {
    formData.append('model', uploadForm.value.model || 'default')
  }
  formData.append('note', uploadForm.value.note || '')

  try {
    await axios.post('/me/textures', formData, { headers: { ...authHeaders(), 'Content-Type': 'multipart/form-data' } })
    ElMessage.success('上传成功')
    showUploadDialog.value = false
    uploadForm.value = { texture_type: 'skin', model: 'default', note: '', file: null }
    if (uploadRef.value) {
      uploadRef.value.clearFiles()
    }
    fetchTextures()
  } catch (e) {
    ElMessage.error('上传失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteMyTexture(hash, type) {
  try {
    await axios.delete(`/me/textures/${hash}/${type}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    fetchTextures()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

function openApplyDialog(tex) {
  applyForm.value.hash = tex.hash
  applyForm.value.texture_type = tex.type
  applyForm.value.profile_id = ''
  showApplyDialog.value = true
}

async function doApply() {
  if (!applyForm.value.profile_id) return ElMessage.error('请选择角色')
  try {
    await axios.post(`/me/textures/${applyForm.value.hash}/apply`, {
      profile_id: applyForm.value.profile_id,
      texture_type: applyForm.value.texture_type
    }, { headers: authHeaders() })
    ElMessage.success('已应用')
    showApplyDialog.value = false
    fetchMe()
    fetchTextures()
  } catch (e) {
    ElMessage.error('应用失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function createRole() {
  const name = (newRoleName.value || '').trim()
  if (!name) return ElMessage.error('请输入角色名称')
  try {
    await axios.post('/me/profiles', { name }, { headers: authHeaders() })
    newRoleName.value = ''
    showCreateRoleDialog.value = false
    ElMessage.success('创建成功')
    fetchMe()
  } catch (e) {
    ElMessage.error('创建失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function deleteRole(pid) {
  try {
    await axios.delete(`/me/profiles/${pid}`, { headers: authHeaders() })
    ElMessage.success('已删除')
    fetchMe()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

async function clearRoleSkin(pid) {
  try {
    await ElMessageBox.confirm(
      '确定要清除该角色的皮肤吗？',
      '确认清除',
      { type: 'warning', confirmButtonText: '确定清除', cancelButtonText: '取消' }
    )
    await axios.delete(`/me/profiles/${pid}/skin`, { headers: authHeaders() })
    ElMessage.success('皮肤已清除')
    fetchMe()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

async function clearRoleCape(pid) {
  try {
    await ElMessageBox.confirm(
      '确定要清除该角色的披风吗？',
      '确认清除',
      { type: 'warning', confirmButtonText: '确定清除', cancelButtonText: '取消' }
    )
    await axios.delete(`/me/profiles/${pid}/cape`, { headers: authHeaders() })
    ElMessage.success('披风已清除')
    fetchMe()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('清除失败: ' + (e.response?.data?.detail || e.message))
    }
  }
}

// 微软正版登录相关函数
function formatUUID(uuid) {
  // 将32位UUID格式化为标准格式：xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
  if (uuid.length === 32) {
    return `${uuid.slice(0, 8)}-${uuid.slice(8, 12)}-${uuid.slice(12, 16)}-${uuid.slice(16, 20)}-${uuid.slice(20)}`
  }
  return uuid
}

async function startMicrosoftAuth() {
  try {
    // 获取授权URL
    const response = await axios.get('/microsoft/auth-url', { headers: authHeaders() })
    const authUrl = response.data.auth_url

    // 将state保存到sessionStorage，用于回调后恢复
    sessionStorage.setItem('ms_auth_state', response.data.state)

    // 重定向到微软登录页面
    window.location.href = authUrl
  } catch (error) {
    ElMessage.error('启动微软登录失败: ' + (error.response?.data?.detail || error.message))
  }
}

async function importMicrosoftProfile() {
  if (!microsoftProfile.value) return

  try {
    importing.value = true
    microsoftStep.value = 'importing'

    // 提取皮肤和披风数据
    const skinData = microsoftProfile.value.skins?.[0]
    const capeData = microsoftProfile.value.capes?.[0]

    const importData = {
      profile_id: microsoftProfile.value.id,
      profile_name: microsoftProfile.value.name,
      skin_url: skinData?.url || null,
      skin_variant: skinData?.variant || 'classic',
      cape_url: capeData?.url || null
    }

    await axios.post('/microsoft/import-profile', importData, { headers: authHeaders() })

    ElMessage.success('正版角色导入成功！')

    // 刷新用户数据
    await fetchMe()

    // 关闭对话框
    showMicrosoftLoginDialog.value = false

    // 重置状态
    microsoftStep.value = 'select-profile'
    microsoftProfile.value = null
    importing.value = false
    importing.value = false
  } catch (error) {
    ElMessage.error('导入失败: ' + (error.response?.data?.detail || error.message))
    importing.value = false
    microsoftStep.value = 'select-profile'
  }
}

function cancelMicrosoftLogin() {
  // 重置状态
  showMicrosoftLoginDialog.value = false
  microsoftStep.value = 'select-profile'
  microsoftProfile.value = null
  importing.value = false
}

async function updateProfile() {
  try {
    // 如果要修改密码，需要验证
    if (form.value.new_password) {
      if (!form.value.old_password) {
        ElMessage.error('请输入旧密码')
        return
      }
      if (form.value.new_password.length < 6) {
        ElMessage.error('新密码长度不能少于6个字符')
        return
      }
      if (form.value.new_password !== form.value.confirm_password) {
        ElMessage.error('两次输入的新密码不一致')
        return
      }

      // 修改密码
      await axios.post('/me/password', {
        old_password: form.value.old_password,
        new_password: form.value.new_password
      }, { headers: authHeaders() })

      ElMessage.success('密码修改成功')
      // 清空密码字段
      form.value.old_password = ''
      form.value.new_password = ''
      form.value.confirm_password = ''
    }

    // 更新基本信息
    const payload = {
      email: form.value.email,
      display_name: form.value.display_name
    }
    await axios.patch('/me', payload, { headers: authHeaders() })
    ElMessage.success('信息修改成功')
    fetchMe()
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.response?.data?.detail || e.message))
  }
}

async function confirmDeleteAccount() {
  try {
    await axios.delete('/me', { headers: authHeaders() })
    ElMessage.success('账号已注销')
    localStorage.removeItem('jwt')
    localStorage.removeItem('accessToken')
    setTimeout(() => {
      router.push('/')
    }, 1000)
  } catch (e) {
    ElMessage.error('注销失败: ' + (e.response?.data?.detail || e.message))
  }
}
</script>

<style scoped>
.dashboard-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.dashboard-container :deep(.el-container) {
  min-height: 100vh;
}

.dashboard-sidebar {
  background: #fff;
  border-right: 1px solid #e4e7ed;
  padding: 20px 0;
  min-height: 100vh;
}

.user-info {
  text-align: center;
  padding: 20px;
  margin-bottom: 20px;
}

.user-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  font-weight: bold;
  font-size: 24px;
  margin-bottom: 12px;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
}

.user-avatar:hover {
  transform: scale(1.15) rotate(8deg);
  box-shadow: 0 8px 24px rgba(102, 126, 234, 0.3);
}

.user-name {
  font-size: 16px;
  font-weight: 500;
  color: #303133;
  margin-top: 12px;
}

.user-status {
  margin-top: 8px;
  display: flex;
  justify-content: center;
}

.sidebar-menu {
  border: none;
}

.sidebar-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  margin: 4px 12px;
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
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
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
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  transform: translateX(0);
}

.menu-divider {
  height: 1px;
  background: #ebeef5;
  margin: 8px 12px;
}

.sidebar-menu .admin-menu-item:hover {
  background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);
  color: #fff;
}

.dashboard-main {
  padding: 30px;
  background: #f5f7fa;
  min-height: 100vh;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  animation: fadeIn 0.4s ease-out;
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

/* 衣柜样式 */
.textures-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 24px;
}

.texture-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  animation: fadeInUp 0.5s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

/* 为每个纹理卡片添加错开延迟 */
.texture-card:nth-child(1) { animation-delay: 0.05s; }
.texture-card:nth-child(2) { animation-delay: 0.1s; }
.texture-card:nth-child(3) { animation-delay: 0.15s; }
.texture-card:nth-child(4) { animation-delay: 0.2s; }
.texture-card:nth-child(5) { animation-delay: 0.25s; }
.texture-card:nth-child(6) { animation-delay: 0.3s; }
.texture-card:nth-child(7) { animation-delay: 0.35s; }
.texture-card:nth-child(8) { animation-delay: 0.4s; }

.texture-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.12);
}

.texture-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;
}

/* 分辨率标签 */
.resolution-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 10px;
  border-radius: 6px;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  backdrop-filter: blur(4px);
  animation: badgeFadeIn 0.5s cubic-bezier(0.4, 0, 0.2, 1) 0.3s backwards;
  z-index: 10;
}

@keyframes badgeFadeIn {
  from {
    opacity: 0;
    transform: translateY(-10px) scale(0.8);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.cape-preview {
  max-width: 80%;
  max-height: 80%;
  object-fit: contain;
}

.texture-info {
  padding: 16px;
  text-align: center;
}

.texture-type-badge {
  display: inline-block;
  padding: 6px 14px;
  border-radius: 14px;
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 10px;
  letter-spacing: 0.5px;
}

.texture-type-badge.skin {
  background: #ecf5ff;
  color: #409eff;
}

.texture-type-badge.cape {
  background: #f0f9ff;
  color: #67c23a;
}

.texture-note {
  font-size: 14px;
  color: #606266;
  min-height: 22px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: all 0.2s ease;
}

.texture-note:hover {
  background: #f5f7fa;
  color: #409eff;
}

.texture-note-input {
  margin-top: 4px;
}

.texture-note-input :deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 0 0 1px #dcdfe6 inset;
  transition: all 0.3s ease;
}

.texture-note-input :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.texture-note-input :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #409eff inset, 0 0 0 3px rgba(64, 158, 255, 0.1);
}

.texture-note-input :deep(.el-input__inner) {
  font-size: 14px;
}

.texture-actions {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #ebeef5;
  background: #fafafa;
}

.texture-actions .el-button {
  flex: 1;
}

.action-btn {
  border: none;
  font-weight: 500;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.action-btn span {
  font-size: 14px;
}

.action-btn .el-icon {
  font-size: 16px;
}

.action-btn-primary {
  background: linear-gradient(135deg, #409eff 0%, #5cadff 100%);
  color: #fff;
}

.action-btn-primary:hover {
  background: linear-gradient(135deg, #66b1ff 0%, #79bbff 100%);
  transform: translateY(-2px) scale(1.02);
  box-shadow: 0 6px 20px rgba(64, 158, 255, 0.4);
}

.action-btn-primary:active {
  transform: translateY(0) scale(0.98);
}

.action-btn-danger {
  background: linear-gradient(135deg, #f56c6c 0%, #f78989 100%);
  color: #fff;
}

.action-btn-danger:hover {
  background: linear-gradient(135deg, #f78989 0%, #f9a7a7 100%);
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(245, 108, 108, 0.4);
}

.action-btn-danger:active {
  transform: translateY(0);
}

.action-btn-danger {
  position: relative;
  overflow: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.action-btn-danger .btn-content {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

.action-btn-danger .btn-label {
  padding: 0;
  margin: 0;
  grid-area: 1 / 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  opacity: 1;
  transform: translateY(0) scale(1);
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  font-weight: 500;
}

.action-btn-danger .btn-icon {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%) scale(0.6) rotate(-90deg);
  opacity: 0;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.action-btn-danger:hover .btn-label {
  opacity: 0;
  transform: translateY(8px) scale(0.8);
}

.action-btn-danger:hover .btn-icon {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1) rotate(0deg);
}

.action-btn-danger:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(245, 108, 108, 0.25);
}

.action-btn-danger:active {
  transform: translateY(0);
}

/* 角色卡片加载动画 */
.create-role-card {
  margin-bottom: 24px;
}

.roles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 24px;
}

.role-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  animation: fadeInUp 0.5s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

/* 为每个卡片添加错开延迟 */
.role-card:nth-child(1) { animation-delay: 0.05s; }
.role-card:nth-child(2) { animation-delay: 0.1s; }
.role-card:nth-child(3) { animation-delay: 0.15s; }
.role-card:nth-child(4) { animation-delay: 0.2s; }
.role-card:nth-child(5) { animation-delay: 0.25s; }
.role-card:nth-child(6) { animation-delay: 0.3s; }

.role-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.12);
}

.role-preview {
  width: 100%;
  height: 280px;
  display: flex;
  justify-content: center;
  align-items: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.role-info {
  padding: 16px;
  text-align: center;
}

.role-name {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.role-model {
  font-size: 13px;
  color: #909399;
  font-weight: 500;
}

.role-actions {
  display: flex;
  flex-direction: row;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #ebeef5;
  background: #fafafa;
  align-items: center;
}

.role-actions .el-button {
  /* 三个按钮平均分配空间 */
  flex: 1;
  min-width: 0;
}

.action-btn-warning {
  color: #e6a23c;
  border-color: #e6a23c;
  background: rgba(230,162,60,0.06);
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.action-btn-warning .btn-content {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

/* 文本默认显示，完全居中 */
.action-btn-warning .btn-label {
  padding: 0;
  margin: 0;
  grid-area: 1 / 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  opacity: 1;
  transform: translateY(0) scale(1);
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  font-weight: 500;
}

/* 图标默认隐藏，绝对定位完全脱离文档流 */
.action-btn-warning .btn-icon {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%) scale(0.6) rotate(-90deg);
  opacity: 0;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

/* hover 时文本淡出下滑，图标旋转放大淡入 */
.action-btn-warning:hover .btn-label {
  opacity: 0;
  transform: translateY(8px) scale(0.8);
}

.action-btn-warning:hover .btn-icon {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1) rotate(0deg);
}

.action-btn-warning:hover {
  color: #fff;
  background: linear-gradient(135deg, #ffa726 0%, #fb8c00 100%);
  border-color: transparent;
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(251,140,0,0.18);
}

.action-btn-warning:active {
  transform: translateY(0);
}

/* 个人资料样式 */
.profile-form-card {
  max-width: 600px;
  margin: 0 auto;
  padding: 30px;
  animation: cardSlideIn 0.5s cubic-bezier(0.4, 0, 0.2, 1);
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

.profile-header {
  display: flex;
  align-items: center;
  gap: 16px;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.1s backwards;
}

.profile-avatar {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.profile-avatar:hover {
  transform: scale(1.1) rotate(5deg);
  box-shadow: 0 8px 16px rgba(0, 0, 0, 0.15);
}

.profile-meta h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #303133;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.2s backwards;
}

.profile-meta p {
  margin: 6px 0 0;
  color: #909399;
  font-size: 13px;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.25s backwards;
}

.profile-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) 0.4s backwards;
}

.profile-form-card :deep(.el-form-item) {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  animation: fadeInUp 0.6s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

.profile-form-card :deep(.el-form-item):nth-child(1) { animation-delay: 0.3s; }
.profile-form-card :deep(.el-form-item):nth-child(2) { animation-delay: 0.35s; }
.profile-form-card :deep(.el-form-item):nth-child(3) { animation-delay: 0.4s; }

.profile-form-card :deep(.el-form-item:hover) {
  transform: translateX(4px);
}

/* 上传对话框样式 */
.upload-dialog :deep(.el-upload-dragger) {
  width: 100%;
}

.upload-form-item {
  overflow: hidden;
}

.upload-form-item :deep(.el-form-item__content) {
  overflow: hidden;
}

.upload-wrapper {
  width: 100%;
  overflow: hidden;
}

.upload-dialog :deep(.el-upload-list) {
  max-width: 100%;
  overflow: hidden;
}

.upload-dialog :deep(.el-upload-list__item) {
  max-width: 100%;
  overflow: hidden;
}

.upload-dialog :deep(.el-upload-list__item-name) {
  max-width: 280px !important;
  overflow: hidden !important;
  text-overflow: ellipsis !important;
  white-space: nowrap !important;
  display: inline-block !important;
}

.upload-dialog :deep(.el-icon--document) + span {
  max-width: 250px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: inline-block;
  vertical-align: middle;
}
/* 动画关键帧 */
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
  }
  to {
    opacity: 1;
  }
}

/* 页面切换动画 */
.wardrobe-section,
.roles-section,
.profile-section {
  animation: fadeIn 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.section-header {
  animation: fadeInUp 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 微软正版登录对话框样式 */
.microsoft-login-content {
  padding: 20px;
}

.step-container {
  min-height: 300px;
}

.device-code-info {
  text-align: center;
  margin: 20px 0;
}

.user-code-display {
  font-size: 32px;
  font-weight: bold;
  font-family: 'Courier New', monospace;
  letter-spacing: 4px;
  color: #409EFF;
  background: #f0f9ff;
  padding: 20px;
  border-radius: 8px;
  margin: 20px 0;
  user-select: all;
}

.countdown-timer {
  font-size: 16px;
  color: #909399;
  margin-top: 16px;
}

.polling-status {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 24px;
  color: #606266;
  font-size: 14px;
}

.polling-status .el-icon {
  font-size: 20px;
}

.profile-selection {
  width: 100%;
}

.profile-card {
  margin: 0 auto;
  max-width: 500px;
}

.profile-info-display {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 16px 0;
}

.profile-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  font-weight: bold;
  font-size: 32px;
  flex-shrink: 0;
}

.profile-details {
  flex: 1;
}

.profile-details h3 {
  margin: 0 0 8px 0;
  font-size: 20px;
  color: #303133;
}

.profile-details p {
  margin: 8px 0;
  color: #606266;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.profile-details .el-tag {
  margin-top: 12px;
}

.skin-preview {
  padding: 12px 0 0 0;
}

.skin-preview p {
  margin: 8px 0;
  color: #606266;
  font-size: 14px;
}

</style>
