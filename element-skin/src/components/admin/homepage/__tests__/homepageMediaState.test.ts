import { describe, expect, it } from 'vitest'
import type { HomepageMedia } from '@/api/types'
import {
  buildHomepageMediaPatch,
  changedHomepageMediaItems,
  cloneHomepageMediaItems,
  homepageMediaOrderChanged,
  homepageMediaPreviewUrl,
  homepageMediaSnapshotItem,
  homepageMediaUrl,
  isHomepageMediaDirty,
  normalizeHomepageMedia,
} from '../homepageMediaState'

function media(overrides: Partial<HomepageMedia> = {}): HomepageMedia {
  return {
    id: 'media-1',
    type: 'image',
    title: 'Village',
    storage_path: 'village/day.png',
    overlay_opacity_light: 0.2,
    overlay_opacity_dark: 0.4,
    start_yaw: 10,
    start_pitch: 5,
    yaw_speed_dps: 1,
    pitch_speed_dps: 2,
    sort_order: 1,
    enabled: true,
    duration_ms: 8000,
    created_at: 100,
    updated_at: 200,
    ...overrides,
  }
}

describe('homepageMediaState', () => {
  it('builds exact image and panorama preview URLs from base path', () => {
    expect(homepageMediaUrl(media(), '/skin/')).toBe('/skin/static/carousel/village/day.png')
    expect(homepageMediaUrl(media(), '/skin/', 'panorama_0.png')).toBe(
      '/skin/static/carousel/village/day.png/panorama_0.png',
    )
    expect(
      homepageMediaPreviewUrl(media({ type: 'panorama', storage_path: 'pano/sunset' }), '/'),
    ).toBe('/static/carousel/pano/sunset/panorama_0.png')
  })

  it('normalizes numeric and enabled fields exactly', () => {
    const normalized = normalizeHomepageMedia(
      media({
        enabled: 0 as unknown as boolean,
        duration_ms: '9000' as unknown as number,
        overlay_opacity_light: '0.25' as unknown as number,
        overlay_opacity_dark: '0.5' as unknown as number,
        start_yaw: '12' as unknown as number,
        start_pitch: '-3' as unknown as number,
        yaw_speed_dps: '1.5' as unknown as number,
        pitch_speed_dps: '2.5' as unknown as number,
      }),
    )

    expect(normalized.enabled).toBe(false)
    expect(normalized.duration_ms).toBe(9000)
    expect(normalized.overlay_opacity_light).toBe(0.25)
    expect(normalized.overlay_opacity_dark).toBe(0.5)
    expect(normalized.start_yaw).toBe(12)
    expect(normalized.start_pitch).toBe(-3)
    expect(normalized.yaw_speed_dps).toBe(1.5)
    expect(normalized.pitch_speed_dps).toBe(2.5)
  })

  it('clones media items without retaining object identity', () => {
    const original = [media()]
    const cloned = cloneHomepageMediaItems(original)

    expect(cloned).toEqual(original)
    expect(cloned[0]).not.toBe(original[0])
  })

  it('compares only persisted editable snapshot fields for dirty state', () => {
    const saved = media()

    expect(isHomepageMediaDirty(media({ storage_path: 'other/file.png' }), saved)).toBe(false)
    expect(isHomepageMediaDirty(media({ title: 'Changed' }), saved)).toBe(true)
    expect(isHomepageMediaDirty(undefined, saved)).toBe(false)
    expect(homepageMediaSnapshotItem(media({ enabled: 1 as unknown as boolean }))).toEqual({
      id: 'media-1',
      title: 'Village',
      enabled: true,
      duration_ms: 8000,
      overlay_opacity_light: 0.2,
      overlay_opacity_dark: 0.4,
      start_yaw: 10,
      start_pitch: 5,
      yaw_speed_dps: 1,
      pitch_speed_dps: 2,
    })
  })

  it('returns exact changed items and order change state', () => {
    const saved = [media({ id: 'media-1' }), media({ id: 'media-2', title: 'Second' })]
    const current = [
      media({ id: 'media-2', title: 'Second changed' }),
      media({ id: 'media-1' }),
      media({ id: 'media-3', title: 'New' }),
    ]

    expect(homepageMediaOrderChanged(current, saved)).toBe(true)
    expect(changedHomepageMediaItems(current, saved).map((item) => item.id)).toEqual([
      'media-2',
      'media-3',
    ])
    expect(homepageMediaOrderChanged(saved, cloneHomepageMediaItems(saved))).toBe(false)
  })

  it('builds exact image and panorama patch bodies', () => {
    expect(buildHomepageMediaPatch(media())).toEqual({
      title: 'Village',
      enabled: true,
      duration_ms: 8000,
      overlay_opacity_light: 0.2,
      overlay_opacity_dark: 0.4,
    })
    expect(buildHomepageMediaPatch(media({ type: 'panorama' }))).toEqual({
      title: 'Village',
      enabled: true,
      duration_ms: 8000,
      overlay_opacity_light: 0.2,
      overlay_opacity_dark: 0.4,
      start_yaw: 10,
      start_pitch: 5,
      yaw_speed_dps: 1,
      pitch_speed_dps: 2,
    })
  })
})
