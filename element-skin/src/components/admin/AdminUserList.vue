<template>
  <div class="max-w-[1000px] mx-auto py-5 animate-fade-in">
    <PageHeader title="用户管理" subtitle="管理站内所有用户及其角色的状态与权限">
      <template #icon><UserFilled /></template>
      <template #actions>
        <el-button
          type="primary"
          :icon="Refresh"
          @click="refreshUsersFromFirst"
          plain
          class="hover-lift"
        >
          刷新列表
        </el-button>
      </template>
    </PageHeader>

    <SearchBar
      v-model="searchQuery"
      placeholder="搜索用户名 / 邮箱 / 角色名"
      class="mb-6"
      @clear="handleClearSearch"
      @search="handleSearch"
    />

    <UiCard shadow="never">
      <el-table :data="users" class="modern-table w-full" v-loading="loading">
        <el-table-column prop="display_name" label="用户名" min-width="150">
          <template #default="{ row }">
            <div class="flex items-center">
              <el-avatar
                :size="32"
                :shape="row.avatar_hash ? 'square' : 'circle'"
                :class="[row.avatar_hash ? 'has-custom' : 'avatar-fallback', 'mr-2']"
                :src="userAvatars[row.avatar_hash || ''] || ''"
              >
                {{
                  !row.avatar_hash
                    ? row.display_name?.charAt(0).toUpperCase() || row.email.charAt(0).toUpperCase()
                    : ''
                }}
              </el-avatar>
              <span>{{ row.display_name || '未设置' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" min-width="220" />
        <el-table-column label="身份状态" width="120">
          <template #default="{ row }">
            <el-tag v-if="row.protected" type="danger" effect="dark" size="small">
              超级管理员
            </el-tag>
            <el-tag v-else-if="hasUserRole(row, 'admin')" type="danger" effect="light" size="small">
              管理员
            </el-tag>
            <el-tag
              v-else-if="hasUserRole(row, 'moderator')"
              type="warning"
              effect="light"
              size="small"
            >
              审核员
            </el-tag>
            <el-tag v-else-if="getUserBanStatus(row)" type="warning" effect="light" size="small"
              >已封禁</el-tag
            >
            <el-tag v-else type="success" effect="light" size="small">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="管理操作" width="120" align="center">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="showUserDetailDialog(row)" class="">
              管理
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <CursorPager
          v-if="users.length > 0"
          :count="users.length"
          :loading="usersPagination.isLoading.value"
          :disabled-prev="!usersPagination.canGoPrev.value"
          :disabled-next="!usersPagination.canGoNext.value"
          @prev="handleUsersPrevPage"
          @next="handleUsersNextPage"
        />
      </div>
    </UiCard>

    <UserDetailDialog
      v-model:visible="userDetailDialogVisible"
      :user="currentUser"
      :profiles="userProfiles"
      :user-avatars="userAvatars"
      :profiles-loading="profilesPagination.isLoading.value"
      :profiles-prev-disabled="!profilesPagination.canGoPrev.value"
      :profiles-next-disabled="!profilesPagination.canGoNext.value"
      :is-banned="currentUser ? getUserBanStatus(currentUser) : false"
      :ban-remaining="formatBanRemaining(currentUser?.banned_until)"
      :is-self="currentUser ? isCurrentUserSelf(currentUser) : false"
      :permission-state="currentPermissionState"
      :permissions-loading="permissionsLoading"
      :current-permissions="loggedInUser?.permissions || []"
      :current-user-protected="Boolean(loggedInUser?.protected)"
      @profiles-prev="handleProfilesPrevPage"
      @profiles-next="handleProfilesNextPage"
      @grant-role="grantRole"
      @revoke-role="revokeRole"
      @transfer-protected-subject="transferProtected"
      @set-permission="setPermission"
      @clear-permission="clearPermission"
      @show-ban="showBanDialog"
      @unban="unbanUser"
      @show-reset-password="showResetPasswordDialog"
      @delete-user="deleteUser"
    />

    <ResetPasswordDialog
      v-model:visible="resetPasswordDialogVisible"
      v-model:new-password="resetPasswordForm.new_password"
      v-model:confirm-password="resetPasswordForm.confirm_password"
      :resetting="resetting"
      @confirm="confirmResetPassword"
    />

    <BanUserDialog
      v-model:visible="banDialogVisible"
      v-model:duration-type="banDurationType"
      v-model:preset-duration="banPresetDuration"
      v-model:custom-hours="banCustomHours"
      v-model:reason="banReason"
      :presets="presetDurations"
      :until-label="formatBanUntilTime()"
      :banning="banning"
      @confirm="confirmBanUser"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, watch, inject, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, UserFilled } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import UiCard from '@/components/ui/UiCard.vue'
import SearchBar from '@/components/common/SearchBar.vue'
import UserDetailDialog from '@/components/admin/users/UserDetailDialog.vue'
import ResetPasswordDialog from '@/components/admin/users/ResetPasswordDialog.vue'
import BanUserDialog from '@/components/admin/users/BanUserDialog.vue'
import { getAvatarForHash } from '@/composables/useAvatar'
import { useCursorPagination } from '@/composables/useCursorPagination'
import {
  getUsers,
  getUser,
  getUserProfiles,
  getUserPermissions,
  grantUserRole,
  revokeUserRole,
  transferProtectedSubject,
  setUserPermissionOverride,
  clearUserPermissionOverride,
  deleteUser as apiDeleteUser,
  banUser as apiBanUser,
  unbanUser as apiUnbanUser,
  resetUserPassword,
} from '@/api/admin/users'
import { getMe } from '@/api/me'
import type { PermissionOverrideEffect, User, Profile, UserPermissionsResponse } from '@/api/types'
import PageHeader from '@/components/common/PageHeader.vue'

type UserQueryParams = { cursor?: string | null; limit?: number; q?: string }

const users = ref<User[]>([])
const limit = 15
const usersPagination = useCursorPagination<User>(limit)
const loading = ref(false)
const searchQuery = ref('')
const activeSearchQuery = ref('') // 当前生效的搜索词（点击搜索按钮后才同步）
const userAvatars = reactive<Record<string, string>>({}) // hash -> base64 avatar image cache
const currentUser = ref<User | null>(null)
const currentPermissionState = ref<UserPermissionsResponse | null>(null)
const permissionsLoading = ref(false)
const userProfiles = ref<Profile[]>([])
const profileLimit = 10
const profilesPagination = useCursorPagination<Profile>(profileLimit)
const userDetailDialogVisible = ref(false)
const resetPasswordDialogVisible = ref(false)
const resetPasswordForm = ref({ new_password: '', confirm_password: '' })
const resetting = ref(false)
const banDialogVisible = ref(false)
const banDurationType = ref('preset')
const banPresetDuration = ref(24)
const banCustomHours = ref(24)
const banReason = ref('')
const banning = ref(false)

const presetDurations = [
  { label: '1小时', value: 1 },
  { label: '1天', value: 24 },
  { label: '3天', value: 72 },
  { label: '7天', value: 168 },
  { label: '30天', value: 720 },
]

function buildSearchParams(extraParams: UserQueryParams = {}): UserQueryParams {
  const params: UserQueryParams = { limit, ...extraParams }
  if (activeSearchQuery.value) params.q = activeSearchQuery.value
  return params
}

async function refreshUsers() {
  loading.value = true
  usersPagination.isLoading.value = true
  try {
    const res = await getUsers(buildSearchParams({ cursor: usersPagination.currentCursor.value }))
    users.value = res.data.items
    usersPagination.setPageData(res.data)
  } catch {
    ElMessage.error('加载用户列表失败')
  } finally {
    loading.value = false
    usersPagination.isLoading.value = false
  }
}

async function refreshUsersFromFirst() {
  usersPagination.reset()
  await refreshUsers()
}

/** Load avatars for all users on the current page (sequentially, one WebGL at a time) */
async function loadAvatarsForUsers(userList: User[]) {
  for (const u of userList) {
    if (u.avatar_hash && !userAvatars[u.avatar_hash]) {
      const img = await getAvatarForHash(u.avatar_hash)
      if (img) userAvatars[u.avatar_hash] = img
    }
  }
}

async function handleUsersNextPage() {
  await usersPagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getUsers(buildSearchParams({ cursor, limit: pageLimit }))
    users.value = res.data.items
    return res.data
  })
}

async function handleUsersPrevPage() {
  await usersPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getUsers(buildSearchParams({ cursor, limit: pageLimit }))
    users.value = res.data.items
    return res.data
  })
}

function handleSearch() {
  activeSearchQuery.value = searchQuery.value.trim()
  usersPagination.reset()
  refreshUsers()
}

function handleClearSearch() {
  searchQuery.value = ''
  activeSearchQuery.value = ''
  usersPagination.reset()
  refreshUsers()
}

async function showUserDetailDialog(user: User) {
  try {
    currentPermissionState.value = null
    profilesPagination.reset()
    permissionsLoading.value = true
    const [userRes, permissionsRes] = await Promise.all([
      getUser(user.id),
      getUserPermissions(user.id),
    ])
    currentPermissionState.value = permissionsRes.data
    currentUser.value = {
      ...userRes.data,
      roles: permissionsRes.data.roles,
      protected: permissionsRes.data.protected,
    }
    await fetchUserProfilesAdmin()
    userDetailDialogVisible.value = true
  } catch {
    ElMessage.error('无法加载用户详情')
  } finally {
    permissionsLoading.value = false
  }
}

async function fetchUserPermissions(userId = currentUser.value?.id) {
  if (!userId) return
  permissionsLoading.value = true
  try {
    const res = await getUserPermissions(userId)
    currentPermissionState.value = res.data
    if (currentUser.value?.id === userId) {
      currentUser.value.roles = res.data.roles
      currentUser.value.protected = res.data.protected
    }
  } finally {
    permissionsLoading.value = false
  }
}

async function fetchUserProfilesAdmin() {
  if (!currentUser.value) return
  try {
    const res = await getUserProfiles(currentUser.value.id, {
      cursor: profilesPagination.currentCursor.value,
      limit: profileLimit,
    })
    userProfiles.value = res.data.items
    profilesPagination.setPageData(res.data)
  } catch {
    ElMessage.error('无法加载用户角色列表')
  }
}

async function handleProfilesNextPage() {
  const u = currentUser.value
  if (!u) return
  await profilesPagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getUserProfiles(u.id, { cursor, limit: pageLimit })
    userProfiles.value = res.data.items
    return res.data
  })
}

async function handleProfilesPrevPage() {
  const u = currentUser.value
  if (!u) return
  await profilesPagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getUserProfiles(u.id, { cursor, limit: pageLimit })
    userProfiles.value = res.data.items
    return res.data
  })
}

async function grantRole(roleId: string) {
  if (!currentUser.value) return
  try {
    await grantUserRole(currentUser.value.id, roleId)
    ElMessage.success('角色已授予')
    await fetchUserPermissions()
    await refreshUsers()
  } catch {}
}

async function revokeRole(roleId: string) {
  if (!currentUser.value) return
  try {
    await ElMessageBox.confirm('确定要撤销该角色吗？', '确认', { type: 'warning' })
    await revokeUserRole(currentUser.value.id, roleId)
    ElMessage.success('角色已撤销')
    await fetchUserPermissions()
    await refreshUsers()
  } catch {}
}

async function transferProtected() {
  if (!currentUser.value) return
  try {
    await ElMessageBox.confirm(
      `确定将超级管理员转让给 ${currentUser.value.display_name || currentUser.value.email} 吗？`,
      '确认转让',
      { type: 'warning' },
    )
    await transferProtectedSubject(currentUser.value.id)
    ElMessage.success('超级管理员已转让')
    await fetchUserPermissions()
    await refreshUsers()
    await refreshLoggedInUser()
  } catch {}
}

async function setPermission(permissionCode: string, effect: PermissionOverrideEffect) {
  if (!currentUser.value) return
  try {
    await setUserPermissionOverride(currentUser.value.id, permissionCode, effect)
    ElMessage.success(effect === 'allow' ? '权限已允许' : '权限已拒绝')
    await fetchUserPermissions()
  } catch {
    ElMessage.error('权限更新失败')
  }
}

async function clearPermission(permissionCode: string) {
  if (!currentUser.value) return
  try {
    await clearUserPermissionOverride(currentUser.value.id, permissionCode)
    ElMessage.success('权限已恢复继承')
    await fetchUserPermissions()
  } catch {}
}

async function deleteUser(user: User) {
  try {
    await ElMessageBox.confirm('永久删除该用户？此操作不可逆！', '极端警告', { type: 'error' })
    await apiDeleteUser(user.id)
    ElMessage.success('用户已删除')
    userDetailDialogVisible.value = false
    await refreshUsersFromFirst()
  } catch {}
}

function showResetPasswordDialog() {
  resetPasswordForm.value = { new_password: '', confirm_password: '' }
  resetPasswordDialogVisible.value = true
}

async function confirmResetPassword() {
  const f = resetPasswordForm.value
  if (!f.new_password || f.new_password.length < 6) return ElMessage.error('密码长度不足')
  if (f.new_password !== f.confirm_password) return ElMessage.error('两次密码不一致')
  if (!currentUser.value) return

  resetting.value = true
  try {
    await resetUserPassword({
      user_id: currentUser.value.id,
      new_password: f.new_password,
    })
    ElMessage.success('密码已重置')
    resetPasswordDialogVisible.value = false
  } catch {
    ElMessage.error('重置失败')
  } finally {
    resetting.value = false
  }
}

function showBanDialog() {
  banReason.value = ''
  banDialogVisible.value = true
}

async function confirmBanUser() {
  if (!currentUser.value) return
  const reason = banReason.value.trim()
  if (!reason) return ElMessage.error('请填写封禁原因')
  const hours = banDurationType.value === 'preset' ? banPresetDuration.value : banCustomHours.value
  const bannedUntil = Date.now() + hours * 60 * 60 * 1000

  banning.value = true
  try {
    await apiBanUser(currentUser.value.id, { banned_until: bannedUntil, reason })
    ElMessage.success('封禁已执行')
    banDialogVisible.value = false
    banReason.value = ''
    await refreshUsers()
    if (currentUser.value) currentUser.value.banned_until = bannedUntil
  } catch {
    ElMessage.error('封禁失败')
  } finally {
    banning.value = false
  }
}

async function unbanUser(user: User) {
  try {
    await apiUnbanUser(user.id)
    ElMessage.success('封禁已解除')
    await refreshUsers()
    if (currentUser.value) currentUser.value.banned_until = 0
  } catch {}
}

async function refreshLoggedInUser() {
  try {
    const res = await getMe()
    if (loggedInUser) loggedInUser.value = res.data
  } catch {}
}

// Helpers
const getUserBanStatus = (user: User) => user.banned_until != null && Date.now() < user.banned_until
const hasUserRole = (user: User, role: string) => (user.roles || []).includes(role)
const loggedInUser = inject<Ref<User | null>>('user', ref(null))
const isCurrentUserSelf = (user: User) => loggedInUser?.value?.id === user.id
const formatBanRemaining = (ts: number | null | undefined) => {
  if (ts == null) return ''
  const m = Math.ceil((ts - Date.now()) / 60000)
  if (m > 1440) return Math.floor(m / 1440) + ' 天'
  if (m > 60) return Math.floor(m / 60) + ' 小时'
  return m + ' 分钟'
}
const formatBanUntilTime = () => {
  const h = banDurationType.value === 'preset' ? banPresetDuration.value : banCustomHours.value
  return new Date(Date.now() + h * 3600000).toLocaleString()
}

onMounted(refreshUsersFromFirst)

// Watch users list changes to load avatars
watch(users, (newUsers) => {
  if (newUsers?.length) loadAvatarsForUsers(newUsers)
})

// When dialog opens and user has avatar_hash, ensure it's loaded
watch(currentUser, async (u) => {
  if (u?.avatar_hash && !userAvatars[u.avatar_hash]) {
    const img = await getAvatarForHash(u.avatar_hash)
    if (img) userAvatars[u.avatar_hash] = img
  }
})
</script>

<style scoped>
.count-text {
  font-weight: 600;
  color: var(--color-text);
  font-family: var(--el-font-family-mono);
  background: var(--color-background-soft);
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 13px;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(.el-table__row) {
  transition: background-color 0.3s ease;
}
</style>
