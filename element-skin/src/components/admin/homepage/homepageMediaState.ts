import type { HomepageMedia } from '@/api/types'

export type HomepageMediaPatch = Partial<
  Pick<
    HomepageMedia,
    | 'title'
    | 'enabled'
    | 'duration_ms'
    | 'overlay_opacity_light'
    | 'overlay_opacity_dark'
    | 'start_yaw'
    | 'start_pitch'
    | 'yaw_speed_dps'
    | 'pitch_speed_dps'
  >
>

export function homepageMediaUrl(item: HomepageMedia, baseUrl: string, face?: string) {
  const suffix = face ? `${item.storage_path}/${face}` : item.storage_path
  return `${baseUrl}static/carousel/${suffix}`.replace(/\/+/g, '/')
}

export function homepageMediaPreviewUrl(item: HomepageMedia, baseUrl: string) {
  return item.type === 'panorama'
    ? homepageMediaUrl(item, baseUrl, 'panorama_0.png')
    : homepageMediaUrl(item, baseUrl)
}

export function normalizeHomepageMedia(item: HomepageMedia): HomepageMedia {
  return {
    ...item,
    duration_ms: Number(item.duration_ms),
    enabled: Boolean(item.enabled),
    overlay_opacity_light: Number(item.overlay_opacity_light),
    overlay_opacity_dark: Number(item.overlay_opacity_dark),
    start_yaw: Number(item.start_yaw),
    start_pitch: Number(item.start_pitch),
    yaw_speed_dps: Number(item.yaw_speed_dps),
    pitch_speed_dps: Number(item.pitch_speed_dps),
  }
}

export function cloneHomepageMediaItems(source: HomepageMedia[]) {
  return source.map((item) => ({ ...item }))
}

export function homepageMediaSnapshot(source: HomepageMedia[]) {
  return JSON.stringify(source.map(homepageMediaSnapshotItem))
}

export function homepageMediaSnapshotItem(item: HomepageMedia) {
  return {
    id: item.id,
    title: item.title,
    enabled: Boolean(item.enabled),
    duration_ms: Number(item.duration_ms),
    overlay_opacity_light: Number(item.overlay_opacity_light),
    overlay_opacity_dark: Number(item.overlay_opacity_dark),
    start_yaw: Number(item.start_yaw),
    start_pitch: Number(item.start_pitch),
    yaw_speed_dps: Number(item.yaw_speed_dps),
    pitch_speed_dps: Number(item.pitch_speed_dps),
  }
}

export function isHomepageMediaDirty(
  current: HomepageMedia | undefined,
  saved: HomepageMedia | undefined,
) {
  if (!current || !saved) return false
  return (
    JSON.stringify(homepageMediaSnapshotItem(current)) !==
    JSON.stringify(homepageMediaSnapshotItem(saved))
  )
}

export function homepageMediaOrderChanged(items: HomepageMedia[], savedItems: HomepageMedia[]) {
  return items.map((item) => item.id).join(',') !== savedItems.map((item) => item.id).join(',')
}

export function changedHomepageMediaItems(items: HomepageMedia[], savedItems: HomepageMedia[]) {
  const savedMap = new Map(savedItems.map((item) => [item.id, item]))
  return items.filter((item) => {
    const saved = savedMap.get(item.id)
    return !saved || isHomepageMediaDirty(item, saved)
  })
}

export function buildHomepageMediaPatch(item: HomepageMedia): HomepageMediaPatch {
  const body: HomepageMediaPatch = {
    title: item.title,
    enabled: item.enabled,
    duration_ms: Number(item.duration_ms),
    overlay_opacity_light: Number(item.overlay_opacity_light),
    overlay_opacity_dark: Number(item.overlay_opacity_dark),
  }
  if (item.type === 'panorama') {
    body.start_yaw = Number(item.start_yaw)
    body.start_pitch = Number(item.start_pitch)
    body.yaw_speed_dps = Number(item.yaw_speed_dps)
    body.pitch_speed_dps = Number(item.pitch_speed_dps)
  }
  return body
}
