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
      <el-table :data="apps" class="modern-table w-full" v-loading="loading">
        <el-table-column label="应用" min-width="260">
          <template #default="{ row }">
            <div class="font-semibold text-[var(--color-heading)]">{{ row.name }}</div>
            <div class="mt-1 break-all font-mono text-xs text-[var(--color-text-light)]">
              {{ row.client_id }}
            </div>
            <div class="mt-1 text-xs text-[var(--color-text-light)]">
              所有者：{{ row.owner_user_id }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.client_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="statusType(row.status)">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="申请权限" min-width="320">
          <template #default="{ row }">
            <div class="flex flex-wrap gap-2">
              <PermissionToneTag
                v-for="code in row.permissions"
                :key="code"
                :label="permissionLabel(code)"
                :title="code"
                tone="violet"
              />
              <el-text v-if="row.permissions.length === 0" size="small" type="info">
                未申请权限
              </el-text>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="回调地址" min-width="260">
          <template #default="{ row }">
            <div class="truncate text-sm">{{ row.redirect_uri }}</div>
            <div class="mt-1 truncate text-xs text-[var(--color-text-light)]">
              {{ row.website_url || '-' }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">
              {{ formatDate(row.updated_at) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <div class="flex flex-wrap justify-end gap-1">
              <el-button
                v-if="row.status !== 'active'"
                link
                type="success"
                :loading="reviewingId === row.client_id"
                @click="review(row.client_id, 'active')"
              >
                通过
              </el-button>
              <el-button
                v-if="row.status !== 'rejected'"
                link
                type="danger"
                :loading="reviewingId === row.client_id"
                @click="review(row.client_id, 'rejected')"
              >
                驳回
              </el-button>
              <el-button
                v-if="row.status !== 'disabled'"
                link
                type="warning"
                :loading="reviewingId === row.client_id"
                @click="review(row.client_id, 'disabled')"
              >
                停用
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import { getErrorMessage } from '@/utils/error'

const status = ref<OAuthClientStatus | 'all'>('pending')
const apps = ref<OAuthClient[]>([])
const catalog = ref<PermissionDefinition[]>([])
const loading = ref(false)
const reviewingId = ref('')

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

function permissionLabel(code: string) {
  return catalog.value.find((item) => item.code === code)?.description || code
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

function formatDate(value?: number) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
</script>
