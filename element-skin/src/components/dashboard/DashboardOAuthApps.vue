<template>
  <div class="animate-fade-in">
    <div class="page-header">
      <div class="page-header-content">
        <h1>第三方应用</h1>
        <p>管理你申请的应用，以及你授权给外部应用的访问能力</p>
      </div>
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
      <div class="space-y-6">
        <UiCard class="p-6">
          <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">作为应用开发者</h2>
              <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
                提交应用审核，通过后才能开始 OAuth 授权流程。
              </p>
            </div>
            <el-button :loading="loading" @click="loadApps">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-empty v-if="!loading && apps.length === 0" description="还没有申请应用" />
          <div v-else class="grid gap-3">
            <button
              v-for="app in apps"
              :key="app.client_id"
              type="button"
              class="rounded-lg border border-[var(--color-border)] bg-[var(--color-card-background)] p-4 text-left transition hover:border-[var(--el-color-primary)]"
              :class="{
                'border-[var(--el-color-primary)] ring-2 ring-[var(--el-color-primary-light-8)]':
                  selectedClientId === app.client_id,
              }"
              @click="selectApp(app.client_id)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-semibold text-[var(--color-heading)]">{{ app.name }}</div>
                  <div class="mt-1 truncate text-xs text-[var(--color-text-light)]">
                    {{ app.client_id }}
                  </div>
                </div>
                <el-tag size="small" :type="statusType(app.status)">
                  {{ statusLabel(app.status) }}
                </el-tag>
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <PermissionToneTag
                  v-for="code in app.permissions.slice(0, 5)"
                  :key="code"
                  :label="permissionLabel(code)"
                  :title="code"
                  tone="violet"
                />
                <el-text v-if="app.permissions.length > 5" size="small" type="info">
                  +{{ app.permissions.length - 5 }}
                </el-text>
              </div>
            </button>
          </div>
        </UiCard>

        <UiCard class="p-6">
          <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">申请新应用</h2>
          <el-form class="mt-5" label-position="top">
            <div class="grid gap-4 md:grid-cols-2">
              <el-form-item label="名称">
                <el-input v-model="form.name" maxlength="80" show-word-limit />
              </el-form-item>
              <el-form-item label="类型">
                <el-select v-model="form.client_type">
                  <el-option label="Confidential" value="confidential" />
                  <el-option label="Public" value="public" />
                </el-select>
              </el-form-item>
            </div>
            <el-form-item label="回调地址">
              <el-input v-model="form.redirect_uri" placeholder="https://app.example/callback" />
            </el-form-item>
            <el-form-item label="网站地址">
              <el-input v-model="form.website_url" placeholder="https://app.example" />
            </el-form-item>
            <el-form-item label="说明">
              <el-input v-model="form.description" type="textarea" :rows="2" maxlength="160" />
            </el-form-item>
            <el-form-item label="申请权限">
              <PermissionTagPicker
                v-model="form.permissions"
                :permissions="delegablePermissions"
              />
            </el-form-item>
            <div class="flex justify-end">
              <el-button type="primary" :loading="creating" @click="createApp">
                <el-icon><Plus /></el-icon>
                提交审核
              </el-button>
            </div>
          </el-form>
        </UiCard>

        <UiCard class="p-6">
          <div class="mb-5 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">作为用户已授权</h2>
              <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">
                管理外部应用已经获得的用户委托权限。
              </p>
            </div>
            <el-button :loading="grantsLoading" @click="loadGrants">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
          </div>

          <el-empty v-if="!grantsLoading && grants.length === 0" description="暂无已授权应用" />
          <div v-else class="grid gap-3">
            <div
              v-for="grant in grants"
              :key="grant.id"
              class="rounded-lg bg-[var(--color-background-soft)] p-4"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-semibold text-[var(--color-heading)]">
                    {{ clientName(grant.client_id) }}
                  </div>
                  <div class="mt-1 truncate text-xs text-[var(--color-text-light)]">
                    {{ grant.client_id }}
                  </div>
                </div>
                <el-tag size="small" :type="grant.status === 'active' ? 'success' : 'info'">
                  {{ grant.status === 'active' ? '已授权' : '已撤销' }}
                </el-tag>
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <PermissionToneTag
                  v-for="code in grant.permissions"
                  :key="code"
                  :label="permissionLabel(code)"
                  :title="code"
                  tone="sky"
                />
              </div>
              <p
                v-if="grant.status === 'revoked'"
                class="mt-3 mb-0 text-xs text-[var(--color-text-light)]"
              >
                {{ revokedGrantCleanupText(grant) }}
              </p>
              <div class="mt-3 flex justify-end">
                <el-button
                  v-if="grant.status === 'active'"
                  type="danger"
                  link
                  :loading="revokingGrantId === grant.id"
                  @click="revokeGrant(grant.id)"
                >
                  撤销授权
                </el-button>
              </div>
            </div>
          </div>
        </UiCard>
      </div>

      <UiCard class="p-6">
        <el-empty v-if="!selectedApp" description="选择一个应用查看详情" />
        <div v-else class="space-y-5">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="m-0 truncate text-lg font-semibold text-[var(--color-heading)]">
                {{ selectedApp.name }}
              </h2>
              <p class="mt-2 mb-0 break-all text-xs text-[var(--color-text-light)]">
                {{ selectedApp.client_id }}
              </p>
            </div>
            <el-tag size="small" :type="statusType(selectedApp.status)">
              {{ statusLabel(selectedApp.status) }}
            </el-tag>
          </div>

          <el-alert
            v-if="newSecret"
            type="success"
            :closable="false"
            title="Client Secret 只显示一次"
          >
            <div class="mt-2 flex gap-2">
              <el-input :model-value="newSecret" readonly />
              <el-button @click="copyText(newSecret)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </el-alert>

          <el-alert
            v-if="selectedApp.status !== 'active'"
            type="info"
            :closable="false"
            :title="statusHint(selectedApp.status)"
          />

          <div class="grid gap-2 text-sm">
            <div class="flex items-center justify-between gap-3">
              <span class="text-[var(--color-text-light)]">类型</span>
              <el-tag>{{ selectedApp.client_type }}</el-tag>
            </div>
            <div class="flex items-center justify-between gap-3">
              <span class="text-[var(--color-text-light)]">回调</span>
              <span class="min-w-0 truncate">{{ selectedApp.redirect_uri }}</span>
            </div>
            <div class="flex items-center justify-between gap-3">
              <span class="text-[var(--color-text-light)]">网站</span>
              <span class="min-w-0 truncate">{{ selectedApp.website_url || '-' }}</span>
            </div>
          </div>

          <div>
            <h3 class="m-0 mb-3 text-base font-semibold text-[var(--color-heading)]">申请权限</h3>
            <div class="flex flex-wrap gap-2">
              <PermissionToneTag
                v-for="code in selectedApp.permissions"
                :key="code"
                :label="permissionLabel(code)"
                :title="code"
                tone="violet"
              />
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            <el-button
              v-if="selectedApp.client_type === 'confidential'"
              :loading="rotating"
              @click="rotateSecret"
            >
              <el-icon><Key /></el-icon>
              轮换密钥
            </el-button>
            <el-button
              v-if="selectedApp.status !== 'pending'"
              :loading="submittingAppId === selectedApp.client_id"
              @click="submitReview"
            >
              <el-icon><Upload /></el-icon>
              重新提交审核
            </el-button>
            <el-button type="danger" :loading="deleting" @click="deleteSelected">
              <el-icon><Delete /></el-icon>
              删除
            </el-button>
          </div>
        </div>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, reactive, ref, watch, type Ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Delete, Key, Plus, Refresh, Upload } from '@element-plus/icons-vue'
import {
  createOAuthApp,
  deleteOAuthApp,
  getPermissionCatalog,
  listOAuthApps,
  listOAuthGrants,
  revokeOAuthGrant,
  rotateOAuthSecret,
  submitOAuthAppReview,
  type OAuthClient,
  type OAuthClientStatus,
  type OAuthGrant,
} from '@/api/oauth'
import type { PermissionDefinition, User } from '@/api/types'
import UiCard from '@/components/ui/UiCard.vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import PermissionTagPicker from '@/components/permissions/PermissionTagPicker.vue'
import { getErrorMessage } from '@/utils/error'

const user = inject<Ref<User | null>>('user', ref(null))

const apps = ref<OAuthClient[]>([])
const grants = ref<OAuthGrant[]>([])
const catalog = ref<PermissionDefinition[]>([])
const selectedClientId = ref('')
const loading = ref(false)
const grantsLoading = ref(false)
const creating = ref(false)
const rotating = ref(false)
const deleting = ref(false)
const revokingGrantId = ref('')
const submittingAppId = ref('')
const newSecret = ref('')

const form = reactive({
  name: '',
  description: '',
  redirect_uri: '',
  website_url: '',
  client_type: 'confidential' as 'public' | 'confidential',
  permissions: [] as string[],
})

const revokedGrantRetentionMs = 30 * 24 * 60 * 60 * 1000

const selectedApp = computed(() => apps.value.find((app) => app.client_id === selectedClientId.value) ?? null)
const userPermissionSet = computed(() => new Set(user.value?.permissions ?? []))
const delegablePermissions = computed(() =>
  catalog.value.filter(
    (item) =>
      item.scope !== 'system' &&
      ((form.client_type === 'confidential' && item.scope === 'server') ||
        userPermissionSet.value.has(item.code)),
  ),
)
const permissionByCode = computed(() => {
  const out = new Map<string, PermissionDefinition>()
  for (const item of catalog.value) out.set(item.code, item)
  return out
})

onMounted(async () => {
  await Promise.all([loadCatalog(), loadApps(), loadGrants()])
})

watch(
  () => form.client_type,
  (clientType) => {
    if (clientType === 'confidential') return
    form.permissions = form.permissions.filter((code) => permissionByCode.value.get(code)?.scope !== 'server')
  },
)

async function loadCatalog() {
  const res = await getPermissionCatalog()
  catalog.value = res.data.permissions
}

async function loadApps() {
  loading.value = true
  try {
    const res = await listOAuthApps()
    apps.value = res.data.items
    if (!selectedClientId.value && apps.value[0]) selectApp(apps.value[0].client_id)
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载应用失败'))
  } finally {
    loading.value = false
  }
}

async function loadGrants() {
  grantsLoading.value = true
  try {
    const res = await listOAuthGrants()
    grants.value = res.data.items
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载授权失败'))
  } finally {
    grantsLoading.value = false
  }
}

function selectApp(clientId: string) {
  selectedClientId.value = clientId
  newSecret.value = ''
}

async function createApp() {
  if (!form.name || !form.redirect_uri || form.permissions.length === 0) {
    ElMessage.warning('请填写名称、回调地址并选择至少一个权限')
    return
  }
  creating.value = true
  try {
    const res = await createOAuthApp({ ...form })
    apps.value.unshift(res.data)
    selectedClientId.value = res.data.client_id
    newSecret.value = res.data.client_secret ?? ''
    form.name = ''
    form.description = ''
    form.redirect_uri = ''
    form.website_url = ''
    form.permissions = []
    ElMessage.success('应用已提交审核')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '提交应用失败'))
  } finally {
    creating.value = false
  }
}

async function submitReview() {
  if (!selectedApp.value) return
  submittingAppId.value = selectedApp.value.client_id
  try {
    const res = await submitOAuthAppReview(selectedApp.value.client_id)
    replaceApp(res.data)
    ElMessage.success('应用已重新提交审核')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '提交审核失败'))
  } finally {
    submittingAppId.value = ''
  }
}

async function rotateSecret() {
  if (!selectedApp.value) return
  rotating.value = true
  try {
    const res = await rotateOAuthSecret(selectedApp.value.client_id)
    newSecret.value = res.data.client_secret ?? ''
    ElMessage.success('密钥已轮换')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '轮换失败'))
  } finally {
    rotating.value = false
  }
}

async function deleteSelected() {
  if (!selectedApp.value) return
  await ElMessageBox.confirm('删除后应用将无法继续完成 OAuth 授权，确认删除？', '删除应用')
  deleting.value = true
  try {
    await deleteOAuthApp(selectedApp.value.client_id)
    apps.value = apps.value.filter((app) => app.client_id !== selectedClientId.value)
    selectedClientId.value = apps.value[0]?.client_id ?? ''
    ElMessage.success('应用已删除')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '删除失败'))
  } finally {
    deleting.value = false
  }
}

async function revokeGrant(grantId: string) {
  revokingGrantId.value = grantId
  try {
    await revokeOAuthGrant(grantId)
    await loadGrants()
    ElMessage.success('授权已撤销')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '撤销授权失败'))
  } finally {
    revokingGrantId.value = ''
  }
}

async function copyText(text: string) {
  await navigator.clipboard.writeText(text)
  ElMessage.success('已复制')
}

function replaceApp(next: OAuthClient) {
  apps.value = apps.value.map((app) => (app.client_id === next.client_id ? next : app))
}

function clientName(clientId: string) {
  return apps.value.find((app) => app.client_id === clientId)?.name || '第三方应用'
}

function permissionLabel(code: string) {
  return permissionByCode.value.get(code)?.description || code
}

function revokedGrantCleanupText(grant: OAuthGrant) {
  if (!grant.revoked_at) return '已撤销的授权将在 30 天后自动清除'
  const cleanupAt = grant.revoked_at + revokedGrantRetentionMs
  const remaining = cleanupAt - Date.now()
  if (remaining <= 0) return '已撤销的授权即将自动清除'
  const days = Math.ceil(remaining / (24 * 60 * 60 * 1000))
  return `已撤销的授权将在 ${days} 天后自动清除`
}

function statusLabel(status: OAuthClientStatus) {
  const labels: Record<OAuthClientStatus, string> = {
    pending: '待审核',
    active: '已通过',
    rejected: '已驳回',
    disabled: '已停用',
  }
  return labels[status]
}

function statusType(status: OAuthClientStatus) {
  if (status === 'active') return 'success'
  if (status === 'rejected') return 'danger'
  if (status === 'pending') return 'warning'
  return 'info'
}

function statusHint(status: OAuthClientStatus) {
  if (status === 'pending') return '应用正在等待管理员审核，审核通过前不能发起 OAuth 授权。'
  if (status === 'rejected') return '应用审核未通过，可以调整信息后重新提交。'
  return '应用已被管理员停用，当前不能发起 OAuth 授权。'
}
</script>
