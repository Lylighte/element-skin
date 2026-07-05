import { getCurrentInstance, onBeforeUnmount, ref, type Ref } from 'vue'
import {
  homepageMediaReorderKey,
  moveHomepageMediaItem,
  shouldReorderHomepageMedia,
} from '@/components/admin/homepage/homepageMediaDrag'

const longPressDelay = 350

interface PendingPress {
  id: string
  pointerId: number
  startX: number
  startY: number
  card: HTMLElement
}

export function useHomepageMediaDrag<T extends { id: string }>(items: Ref<T[]>) {
  const draggingId = ref<string | null>(null)
  const dragOverId = ref<string | null>(null)
  const suppressNextClick = ref(false)

  let longPressTimer = 0
  let pendingPress: PendingPress | null = null
  let pointerDragging = false
  let dragGhost: HTMLElement | null = null
  let ghostOffsetX = 0
  let ghostOffsetY = 0
  let lastReorderKey: string | null = null

  function consumeSuppressedClick() {
    if (!suppressNextClick.value) return false
    suppressNextClick.value = false
    return true
  }

  function setDragOver(id: string | null) {
    dragOverId.value = id
  }

  function startDrag(id: string, event: DragEvent) {
    draggingId.value = id
    dragOverId.value = id
    suppressNextClick.value = true
    event.dataTransfer?.setData('text/plain', id)
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      const card = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
      if (card) event.dataTransfer.setDragImage(card, card.offsetWidth / 2, card.offsetHeight / 2)
    }
  }

  function startLongPress(id: string, event: PointerEvent) {
    if (event.pointerType === 'mouse' || event.button !== 0) return
    const card = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
    if (!card) return
    clearLongPressTimer()
    pendingPress = {
      id,
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      card,
    }
    longPressTimer = window.setTimeout(() => {
      beginPointerDrag(event.clientX, event.clientY)
    }, longPressDelay)
    document.addEventListener('pointermove', handlePointerMove, { passive: false })
    document.addEventListener('pointerup', finishPointerDrag, { passive: false })
    document.addEventListener('pointercancel', finishPointerDrag, { passive: false })
  }

  function beginPointerDrag(clientX: number, clientY: number) {
    if (!pendingPress || pointerDragging) return
    pointerDragging = true
    draggingId.value = pendingPress.id
    dragOverId.value = pendingPress.id
    suppressNextClick.value = true

    const rect = pendingPress.card.getBoundingClientRect()
    ghostOffsetX = clientX - rect.left
    ghostOffsetY = clientY - rect.top
    dragGhost = pendingPress.card.cloneNode(true) as HTMLElement
    dragGhost.classList.add('is-touch-ghost')
    dragGhost.style.position = 'fixed'
    dragGhost.style.left = '0'
    dragGhost.style.top = '0'
    dragGhost.style.width = `${rect.width}px`
    dragGhost.style.height = `${rect.height}px`
    dragGhost.style.margin = '0'
    dragGhost.style.pointerEvents = 'none'
    dragGhost.style.zIndex = '2147483001'
    dragGhost.style.opacity = '0.92'
    dragGhost.style.transition = 'none'
    document.body.appendChild(dragGhost)
    moveGhost(clientX, clientY)
  }

  function handlePointerMove(event: PointerEvent) {
    if (!pendingPress || event.pointerId !== pendingPress.pointerId) return

    if (!pointerDragging) {
      const moved = Math.hypot(
        event.clientX - pendingPress.startX,
        event.clientY - pendingPress.startY,
      )
      if (moved > 8) cancelLongPress()
      return
    }

    event.preventDefault()
    moveGhost(event.clientX, event.clientY)

    const target = document
      .elementFromPoint(event.clientX, event.clientY)
      ?.closest<HTMLElement>('[data-homepage-media-id]')
    const targetId = target?.dataset.homepageMediaId
    if (targetId) moveDraggedTo(targetId, event)
    if (!targetId) dragOverId.value = null
  }

  function moveGhost(clientX: number, clientY: number) {
    if (!dragGhost) return
    dragGhost.style.transform = `translate3d(${clientX - ghostOffsetX}px, ${clientY - ghostOffsetY}px, 0)`
  }

  function finishPointerDrag(event: PointerEvent) {
    if (pendingPress && event.pointerId !== pendingPress.pointerId) return
    if (pointerDragging) event.preventDefault()
    cleanupPointerDrag()
  }

  function cancelLongPress() {
    cleanupPointerDrag(false)
  }

  function cleanupPointerDrag(finishActive = true) {
    clearLongPressTimer()
    document.removeEventListener('pointermove', handlePointerMove)
    document.removeEventListener('pointerup', finishPointerDrag)
    document.removeEventListener('pointercancel', finishPointerDrag)
    dragGhost?.remove()
    dragGhost = null
    pendingPress = null
    resetReorderGuard()
    if (pointerDragging && finishActive) endDrag()
    pointerDragging = false
  }

  function clearLongPressTimer() {
    if (!longPressTimer) return
    window.clearTimeout(longPressTimer)
    longPressTimer = 0
  }

  function moveDraggedTo(targetId: string, event?: DragEvent | PointerEvent) {
    const sourceId = draggingId.value
    if (!sourceId || sourceId === targetId) {
      dragOverId.value = targetId
      return
    }
    const move = moveHomepageMediaItem(items.value, sourceId, targetId)
    if (!move) return
    if (!shouldReorder(move.from, move.to, targetId, event)) {
      dragOverId.value = targetId
      return
    }
    items.value = move.items
    dragOverId.value = targetId
    lastReorderKey = homepageMediaReorderKey(move.from, move.to, targetId)
  }

  function endDrag() {
    draggingId.value = null
    dragOverId.value = null
    resetReorderGuard()
    window.setTimeout(() => {
      suppressNextClick.value = false
    })
  }

  function resetReorderGuard() {
    lastReorderKey = null
  }

  function shouldReorder(
    from: number,
    to: number,
    targetId: string,
    event?: DragEvent | PointerEvent,
  ) {
    const target = event ? mediaCardElement(targetId, event.clientX, event.clientY) : null
    const source = draggingId.value ? mediaCardElement(draggingId.value) : null
    return shouldReorderHomepageMedia({
      from,
      to,
      targetId,
      lastReorderKey,
      targetRect: target?.getBoundingClientRect(),
      sourceRect: source?.getBoundingClientRect(),
      clientX: event?.clientX,
      clientY: event?.clientY,
    })
  }

  function mediaCardElement(id: string, x?: number, y?: number) {
    const selector = `[data-homepage-media-id="${id}"]`
    if (x !== undefined && y !== undefined) {
      return document.elementFromPoint(x, y)?.closest<HTMLElement>(selector) || null
    }
    return document.querySelector<HTMLElement>(selector)
  }

  if (getCurrentInstance()) {
    onBeforeUnmount(() => cleanupPointerDrag())
  }

  return {
    draggingId,
    dragOverId,
    consumeSuppressedClick,
    endDrag,
    moveDraggedTo,
    setDragOver,
    startDrag,
    startLongPress,
  }
}
