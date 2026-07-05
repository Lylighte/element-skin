import type { Notice, NoticeLevel } from '@/api/types'
import type { NoticeDraft } from '@/api/admin/notices'

export function defaultNoticeDraft(): NoticeDraft {
  return {
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
  }
}

export function draftFromNotice(notice: Notice): NoticeDraft {
  return {
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
  }
}

export function validateNoticeContent(form: NoticeDraft) {
  if (!form.title.trim()) return '标题不能为空'
  if (!form.summary.trim())
    return form.display_mode === 'detail' ? '长公告需要填写摘要' : '短公告内容不能为空'
  if (form.display_mode === 'detail' && !form.content_markdown.trim()) return '长公告正文不能为空'
  return ''
}

export function validateNoticeSettings(form: NoticeDraft) {
  if ((form.link_text && !form.link_url) || (!form.link_text && form.link_url))
    return '链接文字和地址需要同时填写'
  if (form.starts_at && form.ends_at && form.ends_at <= form.starts_at)
    return '结束时间必须晚于开始时间'
  return ''
}

export function normalizedNoticeDraft(form: NoticeDraft): NoticeDraft {
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

export function noticeLevelLabel(level: NoticeLevel) {
  return (
    {
      info: '普通',
      success: '成功',
      warning: '重要',
      danger: '紧急',
    } satisfies Record<NoticeLevel, string>
  )[level]
}

export function noticeLevelTagType(level: NoticeLevel) {
  return level === 'danger'
    ? 'danger'
    : level === 'warning'
      ? 'warning'
      : level === 'success'
        ? 'success'
        : 'info'
}

export function noticeLifecycleLabel(notice: Pick<Notice, 'enabled' | 'starts_at' | 'ends_at'>) {
  const now = Date.now()
  if (!notice.enabled) return '已停用'
  if (notice.starts_at && notice.starts_at > now) return '定时发布'
  if (notice.ends_at && notice.ends_at <= now) return '已过期'
  return '展示中'
}
