export interface RectLike {
  left: number
  top: number
  width: number
  height: number
}

export interface HomepageMediaMove<T> {
  from: number
  to: number
  items: T[]
}

export interface HomepageMediaReorderDecision {
  from: number
  to: number
  targetId: string
  lastReorderKey: string | null
  targetRect?: RectLike | null
  sourceRect?: RectLike | null
  clientX?: number
  clientY?: number
}

export function homepageMediaReorderKey(from: number, to: number, targetId: string) {
  return `${targetId}:${from < to ? 'after' : 'before'}`
}

export function moveHomepageMediaItem<T extends { id: string }>(
  items: T[],
  sourceId: string,
  targetId: string,
): HomepageMediaMove<T> | null {
  if (sourceId === targetId) return null

  const from = items.findIndex((item) => item.id === sourceId)
  const to = items.findIndex((item) => item.id === targetId)
  if (from < 0 || to < 0 || from === to) return null

  const next = items.slice()
  const [item] = next.splice(from, 1)
  if (!item) return null
  next.splice(to, 0, item)
  return { from, to, items: next }
}

export function shouldReorderHomepageMedia({
  from,
  to,
  targetId,
  lastReorderKey,
  targetRect,
  sourceRect,
  clientX,
  clientY,
}: HomepageMediaReorderDecision) {
  const key = homepageMediaReorderKey(from, to, targetId)
  if (key === lastReorderKey) return false
  if (!targetRect || clientX === undefined || clientY === undefined) return true

  const sameRow = sourceRect
    ? Math.abs(sourceRect.top - targetRect.top) <
      Math.min(sourceRect.height, targetRect.height) / 2
    : true

  if (sameRow) {
    const pointerX = clientX - targetRect.left
    return from < to ? pointerX > targetRect.width / 2 : pointerX < targetRect.width / 2
  }

  const pointerY = clientY - targetRect.top
  return from < to ? pointerY > targetRect.height / 2 : pointerY < targetRect.height / 2
}
