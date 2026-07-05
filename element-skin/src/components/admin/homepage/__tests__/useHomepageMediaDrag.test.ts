import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useHomepageMediaDrag } from '@/components/admin/homepage/useHomepageMediaDrag'

const mediaItems = () => ref([{ id: 'a' }, { id: 'b' }, { id: 'c' }])

function dragEvent(currentTarget?: HTMLElement) {
  const dataTransfer = {
    effectAllowed: 'none',
    setData: vi.fn(),
    setDragImage: vi.fn(),
  }
  return {
    currentTarget,
    dataTransfer,
  } as unknown as DragEvent
}

describe('useHomepageMediaDrag', () => {
  it('starts native drag with exact state and consumes the suppressed click once', () => {
    const items = mediaItems()
    const card = document.createElement('div')
    Object.defineProperty(card, 'offsetWidth', { value: 120 })
    Object.defineProperty(card, 'offsetHeight', { value: 80 })
    const event = dragEvent(card)
    const drag = useHomepageMediaDrag(items)

    drag.startDrag('a', event)

    const dataTransfer = event.dataTransfer as DataTransfer & {
      setData: ReturnType<typeof vi.fn>
      setDragImage: ReturnType<typeof vi.fn>
    }
    expect(drag.draggingId.value).toBe('a')
    expect(drag.dragOverId.value).toBe('a')
    expect(dataTransfer.effectAllowed).toBe('move')
    expect(dataTransfer.setData).toHaveBeenCalledExactlyOnceWith('text/plain', 'a')
    expect(dataTransfer.setDragImage).toHaveBeenCalledExactlyOnceWith(card, 60, 40)
    expect(drag.consumeSuppressedClick()).toBe(true)
    expect(drag.consumeSuppressedClick()).toBe(false)
  })

  it('moves dragged items exactly and resets drag state on end', () => {
    const items = mediaItems()
    const drag = useHomepageMediaDrag(items)

    drag.startDrag('a', dragEvent())
    drag.moveDraggedTo('c')

    expect(items.value).toEqual([{ id: 'b' }, { id: 'c' }, { id: 'a' }])
    expect(drag.draggingId.value).toBe('a')
    expect(drag.dragOverId.value).toBe('c')

    drag.endDrag()

    expect(drag.draggingId.value).toBeNull()
    expect(drag.dragOverId.value).toBeNull()
  })

  it('keeps item order when hovering the dragged item itself', () => {
    const items = mediaItems()
    const drag = useHomepageMediaDrag(items)

    drag.startDrag('b', dragEvent())
    drag.moveDraggedTo('b')

    expect(items.value).toEqual([{ id: 'a' }, { id: 'b' }, { id: 'c' }])
    expect(drag.draggingId.value).toBe('b')
    expect(drag.dragOverId.value).toBe('b')
  })

  it('sets drag-over state without mutating items', () => {
    const items = mediaItems()
    const drag = useHomepageMediaDrag(items)

    drag.setDragOver('c')
    expect(drag.dragOverId.value).toBe('c')
    expect(items.value).toEqual([{ id: 'a' }, { id: 'b' }, { id: 'c' }])

    drag.setDragOver(null)
    expect(drag.dragOverId.value).toBeNull()
    expect(items.value).toEqual([{ id: 'a' }, { id: 'b' }, { id: 'c' }])
  })
})
