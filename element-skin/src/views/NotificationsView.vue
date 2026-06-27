<template>
  <div class="max-w-[860px] mx-auto py-5 animate-fade-in">
    <PageHeader title="通知中心" subtitle="查看站点公告与重要通知">
      <template #icon><Bell /></template>
      <template #actions>
        <el-button
          :icon="Refresh"
          :loading="loading"
          plain
          class="hover-lift"
          @click="refreshFirstPage"
        >
          刷新
        </el-button>
      </template>
    </PageHeader>

    <div class="flex flex-col gap-4">
      <UiCard v-for="notice in notices" :key="notice.id" shadow="hover" hoverable>
        <button class="w-full text-left p-1" @click="openNotice(notice.id)">
          <div class="flex flex-wrap items-center gap-2 mb-2">
            <el-tag v-if="notice.pinned" size="small" type="warning">置顶</el-tag>
            <el-tag size="small" :type="levelTagType(notice.level)">{{
              levelLabel(notice.level)
            }}</el-tag>
            <span v-if="notice.read" class="text-xs text-[var(--color-text-light)]">已读</span>
          </div>
          <h2 class="m-0 text-xl font-semibold text-[var(--color-heading)]">{{ notice.title }}</h2>
          <div
            class="mt-3 text-sm text-[var(--color-text)] leading-7 [&_p]:m-0 [&_a]:text-[var(--el-color-primary)]"
            v-html="noticePreview(notice)"
          />
          <div
            class="mt-4 flex items-center justify-between gap-3 text-xs text-[var(--color-text-light)]"
          >
            <span>{{ formatDate(notice.created_at) }}</span>
            <el-button v-if="notice.dismissible" size="small" text @click.stop="dismiss(notice.id)">
              忽略
            </el-button>
          </div>
        </button>
      </UiCard>
      <UiCard v-if="!notices.length && !loading" shadow="never">
        <el-empty description="暂无通知" />
      </UiCard>
    </div>

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
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Bell, Refresh } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { dismissNotice, getNotices } from '@/api/notices'
import type { NoticeLevel, NoticeView } from '@/api/types'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { renderMarkdown } from '@/utils/markdown'

const router = useRouter()
const notices = ref<NoticeView[]>([])
const loading = ref(false)
const limit = 10
const pagination = useCursorPagination<NoticeView>(limit)

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

function noticePreview(notice: NoticeView) {
  return renderMarkdown(notice.display_mode === 'detail' ? notice.summary : notice.content_markdown)
}

function formatDate(ts: number) {
  return new Date(ts).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function openNotice(id: string) {
  router.push(`/notifications/${id}`)
}

async function loadNotices() {
  loading.value = true
  try {
    const res = await getNotices({
      cursor: pagination.currentCursor.value,
      limit,
      type: 'announcement',
      include_read: true,
    })
    notices.value = res.data.items
    pagination.setPageData(res.data)
  } catch {
    ElMessage.error('加载通知失败')
  } finally {
    loading.value = false
  }
}

async function handleNextPage() {
  await pagination.goToNextPage(async (cursor, pageLimit) => {
    const res = await getNotices({
      cursor,
      limit: pageLimit,
      type: 'announcement',
      include_read: true,
    })
    notices.value = res.data.items
    return res.data
  })
}

async function handlePrevPage() {
  await pagination.goToPrevPage(async (cursor, pageLimit) => {
    const res = await getNotices({
      cursor,
      limit: pageLimit,
      type: 'announcement',
      include_read: true,
    })
    notices.value = res.data.items
    return res.data
  })
}

async function refreshFirstPage() {
  pagination.reset()
  await loadNotices()
}

async function dismiss(id: string) {
  try {
    await dismissNotice(id)
    notices.value = notices.value.filter((item) => item.id !== id)
    ElMessage.success('已忽略')
  } catch {
    ElMessage.error('忽略通知失败')
  }
}

onMounted(refreshFirstPage)
</script>
