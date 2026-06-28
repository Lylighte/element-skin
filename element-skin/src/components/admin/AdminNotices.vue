<template>
  <div class="max-w-[1100px] mx-auto py-5 animate-fade-in">
    <PageHeader title="通知公告" subtitle="发布公告并控制可见人群、展示方式和生命周期">
      <template #icon><Bell /></template>
      <template #actions>
        <el-select v-model="status" class="w-[140px]" @change="refreshFirstPage">
          <el-option label="全部状态" value="all" />
          <el-option label="已启用" value="enabled" />
          <el-option label="已停用" value="disabled" />
          <el-option label="定时发布" value="scheduled" />
          <el-option label="已过期" value="expired" />
        </el-select>
        <el-button :icon="Refresh" plain class="hover-lift" @click="refreshFirstPage"
          >刷新</el-button
        >
        <el-button type="primary" :icon="Plus" class="hover-lift" @click="openCreateDialog">
          新建公告
        </el-button>
      </template>
    </PageHeader>

    <UiCard shadow="never">
      <el-table :data="notices" class="modern-table w-full">
        <el-table-column label="标题" min-width="260">
          <template #default="{ row }">
            <div class="font-semibold text-[var(--color-heading)]">{{ row.title }}</div>
            <div class="mt-1 text-xs text-[var(--color-text-light)] line-clamp-1">
              {{ row.summary || row.content_markdown }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="展示" width="110" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.display_mode === 'detail' ? 'primary' : 'info'">
              {{ row.display_mode === 'detail' ? '长公告' : '短公告' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="级别" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="levelTagType(row.level)">{{
              levelLabel(row.level)
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="人群" width="100" align="center">
          <template #default="{ row }">
            {{ row.audience === 'admins' ? '管理员' : '用户' }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="140" align="center">
          <template #default="{ row }">
            <div class="flex flex-col items-center gap-1">
              <el-switch v-model="row.enabled" @change="toggleEnabled(row)" />
              <span class="text-xs text-[var(--color-text-light)]">{{ lifecycleLabel(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="置顶" width="80" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.pinned" size="small" type="warning">置顶</el-tag>
            <span v-else class="text-xs text-[var(--color-text-light)]">否</span>
          </template>
        </el-table-column>
        <el-table-column label="有效期" min-width="180">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">
              {{ formatDate(row.starts_at) }} -
              {{ row.ends_at ? formatDate(row.ends_at) : '无期限' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="发布时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">{{
              formatDate(row.created_at)
            }}</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">
            <span class="text-xs text-[var(--color-text-light)]">{{
              formatDate(row.updated_at)
            }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right" align="center">
          <template #default="{ row }">
            <el-button size="small" :icon="Edit" link @click="openEditDialog(row)">编辑</el-button>
            <el-button size="small" :icon="Setting" link @click="openSettingsDialog(row)">
              设置
            </el-button>
            <el-button size="small" type="danger" :icon="Delete" link @click="deleteNotice(row)" />
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <CursorPager
          v-if="notices.length > 0"
          :count="notices.length"
          :loading="pagination.isLoading.value"
          :disabled-prev="!pagination.canGoPrev.value"
          :disabled-next="!pagination.canGoNext.value"
          @prev="handlePrevPage"
          @next="handleNextPage"
        />
      </div>
    </UiCard>

    <UiDialog
      v-model="contentDialogVisible"
      :title="createMode ? '新建公告' : '编辑公告'"
      variant="wide-form"
    >
      <div
        class="grid max-h-[72vh] grid-cols-1 gap-5 overflow-auto p-6 lg:grid-cols-[minmax(0,1fr)_420px]"
      >
        <el-form label-position="top">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_auto]">
            <el-form-item label="标题">
              <el-input
                v-model="form.title"
                maxlength="80"
                show-word-limit
                placeholder="例如：OAuth 应用注册开放说明"
              />
            </el-form-item>
            <el-form-item label="展示方式">
              <el-radio-group v-model="form.display_mode">
                <el-radio-button value="inline">短公告</el-radio-button>
                <el-radio-button value="detail">长公告</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </div>
          <el-form-item label="摘要">
            <el-input
              v-model="form.summary"
              type="textarea"
              :rows="4"
              maxlength="160"
              show-word-limit
              :placeholder="
                form.display_mode === 'detail'
                  ? '长公告必填；仪表盘和通知列表会展示摘要'
                  : '短公告内容，会直接展示在仪表盘和通知列表'
              "
            />
          </el-form-item>
          <el-form-item v-if="form.display_mode === 'detail'" label="正文 Markdown">
            <el-input
              v-model="form.content_markdown"
              type="textarea"
              :rows="18"
              maxlength="20000"
              show-word-limit
              placeholder="支持标题、段落、列表、引用、代码块和链接；原始 HTML 会被清洗"
            />
          </el-form-item>
        </el-form>

        <aside
          class="rounded-xl border border-[var(--color-border)] bg-[var(--color-background-soft)] p-4"
        >
          <div class="mb-4 flex items-center justify-between gap-3">
            <div class="font-semibold text-[var(--color-heading)]">预览</div>
            <div class="flex items-center gap-2">
              <el-tag size="small" :type="form.display_mode === 'detail' ? 'primary' : 'info'">
                {{ form.display_mode === 'detail' ? '长公告' : '短公告' }}
              </el-tag>
              <el-tag size="small" :type="levelTagType(form.level)">
                {{ levelLabel(form.level) }}
              </el-tag>
            </div>
          </div>
          <UiCard shadow="never">
            <article class="p-1">
              <h2 class="m-0 text-2xl font-semibold text-[var(--color-heading)]">
                {{ form.title || '未命名公告' }}
              </h2>
              <p class="mt-4 mb-0 text-sm leading-7 text-[var(--color-text-light)]">
                {{ form.summary || '暂无摘要' }}
              </p>
              <div
                v-if="form.display_mode === 'detail'"
                class="mt-6 border-t border-[var(--color-border)] pt-5 text-sm leading-7 text-[var(--color-text)] [&_a]:text-[var(--el-color-primary)] [&_blockquote]:border-l-4 [&_blockquote]:border-[var(--el-color-primary)] [&_blockquote]:pl-3 [&_blockquote]:text-[var(--color-text-light)] [&_code]:rounded [&_code]:bg-[var(--color-background-soft)] [&_code]:px-1.5 [&_code]:py-0.5 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:font-semibold [&_ol]:pl-5 [&_p]:my-3 [&_pre]:overflow-auto [&_pre]:rounded-xl [&_pre]:bg-[var(--color-background-soft)] [&_pre]:p-3 [&_ul]:pl-5"
                v-html="previewHtml"
              />
              <div v-if="form.link_url && form.link_text" class="mt-6">
                <el-button size="small" type="primary">{{ form.link_text }}</el-button>
              </div>
            </article>
          </UiCard>
        </aside>
      </div>

      <template #footer>
        <div class="px-6 pb-6">
          <el-button @click="contentDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="continueToSettings">下一步：发布设置</el-button>
        </div>
      </template>
    </UiDialog>

    <UiDialog v-model="settingsDialogVisible" title="发布设置" variant="wide-form">
      <div class="max-h-[72vh] overflow-auto p-6">
        <el-form label-position="top">
          <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <el-form-item label="级别">
              <el-select v-model="form.level">
                <el-option label="普通" value="info" />
                <el-option label="成功" value="success" />
                <el-option label="重要" value="warning" />
                <el-option label="紧急" value="danger" />
              </el-select>
            </el-form-item>
            <el-form-item label="可见人群">
              <el-select v-model="form.audience">
                <el-option label="所有用户" value="users" />
                <el-option label="管理员" value="admins" />
              </el-select>
            </el-form-item>
            <el-form-item label="控制项">
              <div class="flex h-8 flex-wrap items-center gap-4">
                <el-checkbox v-model="form.enabled">启用</el-checkbox>
                <el-checkbox v-model="form.pinned">置顶</el-checkbox>
                <el-checkbox v-model="form.dismissible">可忽略</el-checkbox>
              </div>
            </el-form-item>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <el-form-item label="生效时间">
              <el-date-picker
                v-model="form.starts_at"
                type="datetime"
                value-format="x"
                clearable
                placeholder="立即生效"
                class="w-full"
              />
            </el-form-item>
            <el-form-item label="过期时间">
              <el-date-picker
                v-model="form.ends_at"
                type="datetime"
                value-format="x"
                clearable
                placeholder="无期限"
                class="w-full"
              />
            </el-form-item>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <el-form-item label="链接文字">
              <el-input v-model="form.link_text" maxlength="40" />
            </el-form-item>
            <el-form-item label="链接地址">
              <el-input
                v-model="form.link_url"
                maxlength="512"
                placeholder="/oauth/apps 或 https://..."
              />
            </el-form-item>
          </div>
        </el-form>
      </div>

      <template #footer>
        <div class="px-6 pb-6">
          <el-button @click="backToContent">返回编辑</el-button>
          <el-button @click="settingsDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="saveNotice">
            {{ createMode ? '创建公告' : '保存' }}
          </el-button>
        </div>
      </template>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Bell, Delete, Edit, Plus, Refresh, Setting } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import {
  createAdminNotice,
  deleteAdminNotice,
  getAdminNotices,
  patchAdminNotice,
  type NoticeDraft,
} from '@/api/admin/notices'
import type { Notice, NoticeLevel, NoticeStatus } from '@/api/types'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { getErrorMessage } from '@/utils/error'
import { renderMarkdown } from '@/utils/markdown'

const notices = ref<Notice[]>([])
const status = ref<NoticeStatus>('all')
const limit = 15
const pagination = useCursorPagination<Notice>(limit)
const contentDialogVisible = ref(false)
const settingsDialogVisible = ref(false)
const saving = ref(false)
const editingNotice = ref<Notice | null>(null)
const createMode = ref(false)
const form = reactive<NoticeDraft>({
  title: '',
  summary: '',
  content_markdown: '',
  display_mode: 'inline',
  level: 'info',
  audience: 'users',
  enabled: true,
  pinned: false,
  dismissible: true,
  link_text: '',
  link_url: '',
  starts_at: null,
  ends_at: null,
})
const previewHtml = computed(() => renderMarkdown(form.content_markdown || ''))

function levelLabel(level: NoticeLevel) {
  return (
    {
      info: '普通',
      success: '成功',
      warning: '重要',
      danger: '紧急',
    } satisfies Record<NoticeLevel, string>
  )[level]
}

function levelTagType(level: NoticeLevel) {
  return level === 'danger'
    ? 'danger'
    : level === 'warning'
      ? 'warning'
      : level === 'success'
        ? 'success'
        : 'info'
}

function resetForm() {
  Object.assign(form, {
    title: '',
    summary: '',
    content_markdown: '',
    display_mode: 'inline',
    level: 'info',
    audience: 'users',
    enabled: true,
    pinned: false,
    dismissible: true,
    link_text: '',
    link_url: '',
    starts_at: null,
    ends_at: null,
  } satisfies NoticeDraft)
}

function formatDate(ts: number | null | undefined) {
  return ts
    ? new Date(ts).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })
    : '-'
}

function lifecycleLabel(notice: Notice) {
  const now = Date.now()
  if (!notice.enabled) return '已停用'
  if (notice.starts_at && notice.starts_at > now) return '定时发布'
  if (notice.ends_at && notice.ends_at <= now) return '已过期'
  return '展示中'
}

function openCreateDialog() {
  editingNotice.value = null
  createMode.value = true
  resetForm()
  contentDialogVisible.value = true
}

function fillForm(notice: Notice) {
  editingNotice.value = notice
  createMode.value = false
  Object.assign(form, {
    title: notice.title,
    summary: notice.summary,
    content_markdown: notice.content_markdown,
    display_mode: notice.display_mode,
    level: notice.level,
    audience: notice.audience,
    enabled: notice.enabled,
    pinned: notice.pinned,
    dismissible: notice.dismissible,
    link_text: notice.link_text,
    link_url: notice.link_url,
    starts_at: notice.starts_at,
    ends_at: notice.ends_at,
  } satisfies NoticeDraft)
}

function openEditDialog(notice: Notice) {
  fillForm(notice)
  contentDialogVisible.value = true
}

function openSettingsDialog(notice: Notice) {
  fillForm(notice)
  settingsDialogVisible.value = true
}

function validateContent() {
  if (!form.title.trim()) return '标题不能为空'
  if (!form.summary.trim())
    return form.display_mode === 'detail' ? '长公告需要填写摘要' : '短公告内容不能为空'
  if (form.display_mode === 'detail' && !form.content_markdown.trim()) return '长公告正文不能为空'
  return ''
}

function validateSettings() {
  if ((form.link_text && !form.link_url) || (!form.link_text && form.link_url))
    return '链接文字和地址需要同时填写'
  if (form.starts_at && form.ends_at && form.ends_at <= form.starts_at)
    return '结束时间必须晚于开始时间'
  return ''
}

function normalizedForm(): NoticeDraft {
  return {
    ...form,
    title: form.title.trim(),
    summary: form.summary.trim(),
    content_markdown: form.display_mode === 'detail' ? form.content_markdown.trim() : '',
    link_text: form.link_text?.trim() || '',
    link_url: form.link_url?.trim() || '',
    starts_at: form.starts_at ?? null,
    ends_at: form.ends_at ?? null,
  }
}

function continueToSettings() {
  const error = validateContent()
  if (error) {
    ElMessage.warning(error)
    return
  }
  contentDialogVisible.value = false
  settingsDialogVisible.value = true
}

function backToContent() {
  settingsDialogVisible.value = false
  contentDialogVisible.value = true
}

async function saveNotice() {
  const contentError = validateContent()
  if (contentError) {
    settingsDialogVisible.value = false
    contentDialogVisible.value = true
    ElMessage.warning(contentError)
    return
  }
  const settingsError = validateSettings()
  if (settingsError) {
    ElMessage.warning(settingsError)
    return
  }
  saving.value = true
  try {
    if (createMode.value) {
      await createAdminNotice(normalizedForm())
      ElMessage.success('已创建')
    } else {
      await patchAdminNotice(editingNotice.value!.id, normalizedForm())
      ElMessage.success('已保存')
    }
    settingsDialogVisible.value = false
    await refreshFirstPage()
  } catch (e: unknown) {
    ElMessage.error(getErrorMessage(e, createMode.value ? '创建失败' : '保存失败'))
  } finally {
    saving.value = false
  }
}

async function toggleEnabled(notice: Notice) {
  try {
    await patchAdminNotice(notice.id, { enabled: notice.enabled })
    ElMessage.success(notice.enabled ? '已启用' : '已停用')
  } catch (e: unknown) {
    notice.enabled = !notice.enabled
    ElMessage.error(getErrorMessage(e, '状态更新失败'))
  }
}

async function deleteNotice(notice: Notice) {
  try {
    await ElMessageBox.confirm(`确定删除公告「${notice.title}」吗？`, '确认删除', {
      type: 'warning',
    })
    await deleteAdminNotice(notice.id)
    ElMessage.success('已删除')
    await refreshFirstPage()
  } catch {}
}

async function loadNotices() {
  try {
    const res = await getAdminNotices({
      cursor: pagination.currentCursor.value,
      limit,
      type: 'announcement',
      status: status.value,
    })
    notices.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    ElMessage.error('加载公告失败')
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getAdminNotices({
      cursor,
      limit: pageLimit,
      type: 'announcement',
      status: status.value,
    })
    notices.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getAdminNotices({
      cursor,
      limit: pageLimit,
      type: 'announcement',
      status: status.value,
    })
    notices.value = res.data.items
    return res.data
  })
}

async function refreshFirstPage() {
  pagination.reset()
  await loadNotices()
}

onMounted(refreshFirstPage)
</script>
