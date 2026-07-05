import { describe, expect, it } from 'vitest'
import {
  homepageMediaReorderKey,
  moveHomepageMediaItem,
  shouldReorderHomepageMedia,
  type RectLike,
} from '@/components/admin/homepage/homepageMediaDrag'

const items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }]

const rect = (left: number, top: number, width = 100, height = 80): RectLike => ({
  left,
  top,
  width,
  height,
})

describe('homepage media drag helpers', () => {
  it('builds exact reorder guard keys from movement direction', () => {
    expect(homepageMediaReorderKey(0, 2, 'c')).toBe('c:after')
    expect(homepageMediaReorderKey(2, 0, 'a')).toBe('a:before')
  })

  it('moves a media item without mutating the original list', () => {
    const movedForward = moveHomepageMediaItem(items, 'a', 'c')
    const movedBackward = moveHomepageMediaItem(items, 'c', 'a')

    expect(movedForward).toEqual({
      from: 0,
      to: 2,
      items: [{ id: 'b' }, { id: 'c' }, { id: 'a' }],
    })
    expect(movedBackward).toEqual({
      from: 2,
      to: 0,
      items: [{ id: 'c' }, { id: 'a' }, { id: 'b' }],
    })
    expect(items).toEqual([{ id: 'a' }, { id: 'b' }, { id: 'c' }])
  })

  it('does not move when source and target are missing or equal', () => {
    expect(moveHomepageMediaItem(items, 'a', 'a')).toBeNull()
    expect(moveHomepageMediaItem(items, 'missing', 'a')).toBeNull()
    expect(moveHomepageMediaItem(items, 'a', 'missing')).toBeNull()
  })

  it('blocks duplicate reorder guard keys exactly', () => {
    expect(
      shouldReorderHomepageMedia({
        from: 0,
        to: 2,
        targetId: 'c',
        lastReorderKey: 'c:after',
      }),
    ).toBe(false)
  })

  it('allows reorder when there is no geometry information', () => {
    expect(
      shouldReorderHomepageMedia({
        from: 0,
        to: 2,
        targetId: 'c',
        lastReorderKey: null,
      }),
    ).toBe(true)
  })

  it('uses horizontal midpoint for same-row movement', () => {
    const targetRect = rect(200, 20)
    const sourceRect = rect(0, 24)

    expect(
      shouldReorderHomepageMedia({
        from: 0,
        to: 2,
        targetId: 'c',
        lastReorderKey: null,
        targetRect,
        sourceRect,
        clientX: 249,
        clientY: 40,
      }),
    ).toBe(false)
    expect(
      shouldReorderHomepageMedia({
        from: 0,
        to: 2,
        targetId: 'c',
        lastReorderKey: null,
        targetRect,
        sourceRect,
        clientX: 251,
        clientY: 40,
      }),
    ).toBe(true)
    expect(
      shouldReorderHomepageMedia({
        from: 2,
        to: 0,
        targetId: 'a',
        lastReorderKey: null,
        targetRect,
        sourceRect,
        clientX: 249,
        clientY: 40,
      }),
    ).toBe(true)
  })

  it('uses vertical midpoint for cross-row movement', () => {
    const targetRect = rect(0, 120, 100, 80)
    const sourceRect = rect(0, 0, 100, 80)

    expect(
      shouldReorderHomepageMedia({
        from: 0,
        to: 2,
        targetId: 'c',
        lastReorderKey: null,
        targetRect,
        sourceRect,
        clientX: 40,
        clientY: 159,
      }),
    ).toBe(false)
    expect(
      shouldReorderHomepageMedia({
        from: 0,
        to: 2,
        targetId: 'c',
        lastReorderKey: null,
        targetRect,
        sourceRect,
        clientX: 40,
        clientY: 161,
      }),
    ).toBe(true)
  })
})
