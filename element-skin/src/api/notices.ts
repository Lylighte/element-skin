import client from './client'
import type { CursorPageResponse, NoticeView } from './types'

export interface NoticeListParams {
  cursor?: string | null
  limit?: number
  type?: 'announcement'
  include_read?: boolean
  dashboard?: boolean
}

export function getNotices(
  params: NoticeListParams = {},
): Promise<{ data: CursorPageResponse<NoticeView> }> {
  return client.get('/notices', { params })
}

export function getNotice(id: string): Promise<{ data: NoticeView }> {
  return client.get(`/notices/${id}`)
}

export function markNoticeRead(id: string): Promise<{ data: NoticeView }> {
  return client.post(`/notices/${id}/read`)
}

export function dismissNotice(id: string): Promise<{ data: { ok: boolean } }> {
  return client.post(`/notices/${id}/dismiss`)
}
