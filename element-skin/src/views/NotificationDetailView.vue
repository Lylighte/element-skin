<template>
  <div class="max-w-[760px] mx-auto py-5 animate-fade-in">
    <PageHeader title="通知详情" subtitle="公告全文与相关链接">
      <template #icon><Bell /></template>
      <template #actions>
        <el-button :icon="Back" plain class="hover-lift" @click="router.push('/notifications')">
          返回
        </el-button>
      </template>
    </PageHeader>

    <UiCard v-if="notice" shadow="never">
      <article class="p-1">
        <div class="flex flex-wrap items-center gap-2 mb-3">
          <el-tag v-if="notice.pinned" size="small" type="warning">置顶</el-tag>
          <el-tag size="small" :type="levelTagType(notice.level)">{{
            levelLabel(notice.level)
          }}</el-tag>
          <span class="text-xs text-[var(--color-text-light)]">{{
            formatDate(notice.created_at)
          }}</span>
        </div>
        <h1 class="m-0 text-3xl font-semibold text-[var(--color-heading)]">{{ notice.title }}</h1>
        <p v-if="notice.summary" class="mt-4 mb-0 text-[var(--color-text-light)] leading-7">
          {{ notice.summary }}
        </p>
        <div
          class="mt-8 text-[var(--color-text)] leading-8 [&_p]:my-4 [&_h1]:text-2xl [&_h1]:font-semibold [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:text-lg [&_h3]:font-semibold [&_ul]:pl-6 [&_ol]:pl-6 [&_li]:my-1 [&_blockquote]:border-l-4 [&_blockquote]:border-[var(--el-color-primary)] [&_blockquote]:pl-4 [&_blockquote]:text-[var(--color-text-light)] [&_code]:rounded [&_code]:bg-[var(--color-background-soft)] [&_code]:px-1.5 [&_code]:py-0.5 [&_pre]:overflow-auto [&_pre]:rounded-xl [&_pre]:bg-[var(--color-background-soft)] [&_pre]:p-4 [&_a]:text-[var(--el-color-primary)]"
          v-html="renderedContent"
        />
        <div v-if="notice.link_url && notice.link_text" class="mt-8">
          <el-button
            type="primary"
            tag="a"
            :href="notice.link_url"
            target="_blank"
            rel="noreferrer"
          >
            {{ notice.link_text }}
          </el-button>
        </div>
      </article>
    </UiCard>

    <UiCard v-else-if="!loading" shadow="never">
      <el-empty description="通知不存在或已不可见">
        <el-button @click="router.push('/notifications')">返回通知中心</el-button>
      </el-empty>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Back, Bell } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { getNotice } from '@/api/notices'
import type { NoticeLevel, NoticeView } from '@/api/types'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const router = useRouter()
const notice = ref<NoticeView | null>(null)
const loading = ref(false)

const renderedContent = computed(() => renderMarkdown(notice.value?.content_markdown || ''))

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

function formatDate(ts: number) {
  return new Date(ts).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

async function loadNotice() {
  const id = String(route.params.id || '')
  if (!id) return
  loading.value = true
  try {
    const res = await getNotice(id)
    notice.value = res.data
  } catch {
    notice.value = null
    ElMessage.error('加载通知失败')
  } finally {
    loading.value = false
  }
}

onMounted(loadNotice)
</script>
