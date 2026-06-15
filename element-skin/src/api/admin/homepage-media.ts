import client from '../client'
import type { HomepageMedia } from '../types'

export function listHomepageMedia(): Promise<{ data: HomepageMedia[] }> {
  return client.get('/admin/homepage-media')
}

export function uploadHomepageImage(formData: FormData): Promise<{ data: HomepageMedia }> {
  return client.post('/admin/homepage-media/image', formData)
}

export function uploadHomepagePanorama(formData: FormData): Promise<{ data: HomepageMedia }> {
  return client.post('/admin/homepage-media/panorama', formData)
}

export function patchHomepageMedia(
  id: string,
  body: Partial<Pick<HomepageMedia, 'title' | 'enabled' | 'duration_ms' | 'config'>>,
): Promise<{ data: HomepageMedia }> {
  return client.patch(`/admin/homepage-media/${id}`, body)
}

export function reorderHomepageMedia(ids: string[]): Promise<{ data: { ok: boolean } }> {
  return client.patch('/admin/homepage-media/reorder', { ids })
}

export function deleteHomepageMedia(id: string): Promise<{ data: { ok: boolean } }> {
  return client.delete(`/admin/homepage-media/${id}`)
}
