<template>
  <div class="mx-auto max-w-[1200px] py-5 animate-fade-in">
    <PageHeader title="第三方应用" subtitle="审核应用申请，管理全站 OAuth 应用状态">
      <template #icon><Link /></template>
      <template #actions>
        <el-select v-model="status" class="w-[150px]" @change="loadApps">
          <el-option label="全部状态" value="all" />
          <el-option label="待审核" value="pending" />
          <el-option label="已通过" value="active" />
          <el-option label="已驳回" value="rejected" />
          <el-option label="已停用" value="disabled" />
        </el-select>
        <el-button :icon="Refresh" plain class="hover-lift" :loading="loading" @click="loadApps">
          刷新
        </el-button>
      </template>
    </PageHeader>

    <UiCard shadow="never">
      <el-table
        :data="apps"
        class="modern-table w-full"
        v-loading="loading"
        @row-click="openDetails"
      >
        <el-table-column label="应用" min-width="280">
          <template #default="{ row }">
            <div class="flex min-w-0 flex-col">
              <span class="font-semibold text-[var(--color-heading)]">{{ row.name }}</span>
              <span class="mt-1 truncate text-sm text-[var(--color-text-light)]">
                {{ row.description || '开发者未填写说明' }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="所有者" min-width="180">
          <template #default="{ row }">
            <span class="font-mono text-xs text-[var(--color-text-light)]">
              {{ row.owner_user_id }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ clientTypeLabel(row.client_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status)">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="申请权限" width="110" align="center">
          <template #default="{ row }">{{ row.permissions.length }}</template>
        </el-table-column>
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">
              {{ formatDate(row.updated_at) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetails(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!loading && apps.length === 0" description="暂无第三方应用申请" />
    </UiCard>

    <AdminOAuthAppDetailDialog
      v-model:visible="detailVisible"
      :app="selectedApp"
      :catalog="catalog"
      :reviewing="reviewingId === selectedApp?.client_id"
      @review="review"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Link, Refresh } from '@element-plus/icons-vue'
import {
  getPermissionCatalog,
  listAdminOAuthApps,
  reviewAdminOAuthApp,
  type OAuthClient,
  type OAuthClientStatus,
} from '@/api/oauth'
import type { PermissionDefinition } from '@/api/types'
import UiCard from '@/components/ui/UiCard.vue'
import AdminOAuthAppDetailDialog from '@/components/admin/oauth/AdminOAuthAppDetailDialog.vue'
import { getErrorMessage } from '@/utils/error'

const status = ref<OAuthClientStatus | 'all'>('pending')
const apps = ref<OAuthClient[]>([])
const catalog = ref<PermissionDefinition[]>([])
const loading = ref(false)
const reviewingId = ref('')
const detailVisible = ref(false)
const selectedClientId = ref('')

const selectedApp = computed(
  () => apps.value.find((app) => app.client_id === selectedClientId.value) ?? null,
)

onMounted(async () => {
  await Promise.all([loadCatalog(), loadApps()])
})

async function loadCatalog() {
  const res = await getPermissionCatalog()
  catalog.value = res.data.permissions
}

async function loadApps() {
  loading.value = true
  try {
    const res = await listAdminOAuthApps(status.value)
    apps.value = res.data.items
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '加载第三方应用失败'))
  } finally {
    loading.value = false
  }
}

async function review(clientId: string, nextStatus: Exclude<OAuthClientStatus, 'pending'>) {
  reviewingId.value = clientId
  try {
    const res = await reviewAdminOAuthApp(clientId, nextStatus)
    apps.value = apps.value.map((app) => (app.client_id === clientId ? res.data : app))
    ElMessage.success('应用状态已更新')
  } catch (error) {
    ElMessage.error(getErrorMessage(error, '更新应用状态失败'))
  } finally {
    reviewingId.value = ''
  }
}

function openDetails(app: OAuthClient) {
  selectedClientId.value = app.client_id
  detailVisible.value = true
}

function statusLabel(appStatus: OAuthClientStatus) {
  const labels: Record<OAuthClientStatus, string> = {
    pending: '待审核',
    active: '已通过',
    rejected: '已驳回',
    disabled: '已停用',
  }
  return labels[appStatus]
}

function statusType(appStatus: OAuthClientStatus) {
  if (appStatus === 'active') return 'success'
  if (appStatus === 'rejected') return 'danger'
  if (appStatus === 'pending') return 'warning'
  return 'info'
}

function clientTypeLabel(clientType: OAuthClient['client_type']) {
  return clientType === 'confidential' ? '机密应用' : '公开应用'
}

function formatDate(value?: number) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
</script>
