import client from '../client'
import type {
  CursorPageResponse,
  Notice,
  NoticeAudience,
  NoticeDisplayMode,
  NoticeLevel,
} from '../types'

export interface AdminNoticeListParams {
  cursor?: string | null
  limit?: number
  type?: string
  status?: 'all' | 'enabled' | 'disabled' | 'scheduled' | 'expired'
}

export interface NoticeDraft {
  type?: string
  title: string
  summary: string
  content_markdown: string
  display_mode: NoticeDisplayMode
  level: NoticeLevel
  audience: NoticeAudience
  enabled: boolean
  pinned: boolean
  dismissible: boolean
  link_text?: string
  link_url?: string
  starts_at?: number | null
  ends_at?: number | null
}

export type NoticePatch = Partial<NoticeDraft>

export function getAdminNotices(
  params: AdminNoticeListParams = {},
): Promise<{ data: CursorPageResponse<Notice> }> {
  return client.get('/admin/notices', { params })
}

export function createAdminNotice(body: NoticeDraft): Promise<{ data: Notice }> {
  return client.post('/admin/notices', body)
}

export function patchAdminNotice(id: string, body: NoticePatch): Promise<{ data: Notice }> {
  return client.patch(`/admin/notices/${id}`, body)
}

export function deleteAdminNotice(id: string): Promise<{ data: void }> {
  return client.delete(`/admin/notices/${id}`)
}
