import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Notice } from '@/api/types'
import {
  defaultNoticeDraft,
  draftFromNotice,
  normalizedNoticeDraft,
  noticeLevelLabel,
  noticeLevelTagType,
  noticeLifecycleLabel,
  validateNoticeContent,
  validateNoticeSettings,
} from '../noticeForm'

describe('notice form helpers', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('creates the exact default notice draft', () => {
    expect(defaultNoticeDraft()).toEqual({
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
  })

  it('copies editable fields from an existing notice exactly', () => {
    const notice = {
      id: 'notice-1',
      type: 'announcement',
      title: 'Title',
      summary: 'Summary',
      content_markdown: '# Body',
      display_mode: 'detail',
      level: 'warning',
      link_text: 'Open',
      link_url: '/notifications/notice-1',
      audience: 'admins',
      enabled: false,
      pinned: true,
      dismissible: false,
      starts_at: 1000,
      ends_at: 2000,
      created_by: 'admin',
      created_at: 3000,
      updated_at: 4000,
    } satisfies Notice

    expect(draftFromNotice(notice)).toEqual({
      title: 'Title',
      summary: 'Summary',
      content_markdown: '# Body',
      display_mode: 'detail',
      level: 'warning',
      audience: 'admins',
      enabled: false,
      pinned: true,
      dismissible: false,
      link_text: 'Open',
      link_url: '/notifications/notice-1',
      starts_at: 1000,
      ends_at: 2000,
    })
  })

  it('validates content errors and valid states exactly', () => {
    const draft = defaultNoticeDraft()
    expect(validateNoticeContent(draft)).toBe('标题不能为空')

    draft.title = 'Short notice'
    expect(validateNoticeContent(draft)).toBe('短公告内容不能为空')

    draft.summary = 'Inline body'
    expect(validateNoticeContent(draft)).toBe('')

    draft.display_mode = 'detail'
    draft.summary = ''
    expect(validateNoticeContent(draft)).toBe('长公告需要填写摘要')

    draft.summary = 'Detail summary'
    expect(validateNoticeContent(draft)).toBe('长公告正文不能为空')

    draft.content_markdown = 'Detail body'
    expect(validateNoticeContent(draft)).toBe('')
  })

  it('validates publishing settings exactly', () => {
    const draft = defaultNoticeDraft()
    expect(validateNoticeSettings(draft)).toBe('')

    draft.link_text = 'Open'
    expect(validateNoticeSettings(draft)).toBe('链接文字和地址需要同时填写')

    draft.link_url = '/notifications/notice-1'
    expect(validateNoticeSettings(draft)).toBe('')

    draft.starts_at = 2000
    draft.ends_at = 2000
    expect(validateNoticeSettings(draft)).toBe('结束时间必须晚于开始时间')

    draft.ends_at = 2001
    expect(validateNoticeSettings(draft)).toBe('')
  })

  it('normalizes whitespace and strips inline body markdown exactly', () => {
    const inline = {
      ...defaultNoticeDraft(),
      title: '  Title  ',
      summary: '  Summary  ',
      content_markdown: '  Should be stripped  ',
      link_text: '  Open  ',
      link_url: '  /target  ',
      starts_at: undefined,
      ends_at: undefined,
    }
    expect(normalizedNoticeDraft(inline)).toEqual({
      ...inline,
      title: 'Title',
      summary: 'Summary',
      content_markdown: '',
      link_text: 'Open',
      link_url: '/target',
      starts_at: null,
      ends_at: null,
    })

    const detail = {
      ...inline,
      display_mode: 'detail' as const,
      content_markdown: '  **Body**  ',
    }
    expect(normalizedNoticeDraft(detail).content_markdown).toBe('**Body**')
  })

  it('returns exact level labels and tag types', () => {
    expect(noticeLevelLabel('info')).toBe('普通')
    expect(noticeLevelLabel('success')).toBe('成功')
    expect(noticeLevelLabel('warning')).toBe('重要')
    expect(noticeLevelLabel('danger')).toBe('紧急')
    expect(noticeLevelTagType('info')).toBe('info')
    expect(noticeLevelTagType('success')).toBe('success')
    expect(noticeLevelTagType('warning')).toBe('warning')
    expect(noticeLevelTagType('danger')).toBe('danger')
  })

  it('returns exact lifecycle labels', () => {
    vi.setSystemTime(new Date(2000))

    expect(noticeLifecycleLabel({ enabled: false, starts_at: null, ends_at: null })).toBe('已停用')
    expect(noticeLifecycleLabel({ enabled: true, starts_at: 3000, ends_at: null })).toBe('定时发布')
    expect(noticeLifecycleLabel({ enabled: true, starts_at: null, ends_at: 2000 })).toBe('已过期')
    expect(noticeLifecycleLabel({ enabled: true, starts_at: 1000, ends_at: 3000 })).toBe('展示中')
  })
})
