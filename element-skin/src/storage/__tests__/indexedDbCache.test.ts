import 'fake-indexeddb/auto'

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { indexedDbCache } from '../indexedDbCache'

function blobOfSize(size: number, label: string): Blob {
  return new Blob([label + '.'.repeat(size - label.length)], { type: 'image/png' })
}

function blobSummary(blob: Blob | null): { size: number; type: string } | null {
  if (!blob) return null
  return { size: blob.size, type: blob.type }
}

describe('indexedDbCache', () => {
  beforeEach(async () => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    await indexedDbCache.clearForTests()
  })

  it('stores and retrieves blobs by bucket and key exactly', async () => {
    const avatar = new Blob(['avatar-image'], { type: 'image/png' })
    const snapshot = new Blob(['snapshot-image-longer'], { type: 'image/png' })

    await expect(indexedDbCache.set('avatar', 'shared-key', avatar, 1024)).resolves.toBe(true)
    await expect(indexedDbCache.set('viewer-snapshot', 'shared-key', snapshot, 1024)).resolves.toBe(true)

    expect(blobSummary(await indexedDbCache.get('avatar', 'shared-key'))).toEqual({
      size: 'avatar-image'.length,
      type: 'image/png',
    })
    expect(blobSummary(await indexedDbCache.get('viewer-snapshot', 'shared-key'))).toEqual({
      size: 'snapshot-image-longer'.length,
      type: 'image/png',
    })
  })

  it('evicts least-recently-used entries within the same bucket only', async () => {
    let currentTime = 0
    vi.spyOn(Date, 'now').mockImplementation(() => currentTime)
    const first = blobOfSize(400, 'first')
    const second = blobOfSize(400, 'second')
    const third = blobOfSize(400, 'third')
    const snapshot = blobOfSize(900, 'snapshot')

    currentTime = 1_000
    await indexedDbCache.set('avatar', 'first', first, 900)
    currentTime = 2_000
    await indexedDbCache.set('avatar', 'second', second, 900)
    currentTime = 3_000
    await indexedDbCache.get('avatar', 'first')
    currentTime = 4_000
    await indexedDbCache.set('viewer-snapshot', 'snapshot', snapshot, 900)
    await indexedDbCache.set('avatar', 'third', third, 900)

    expect(blobSummary(await indexedDbCache.get('avatar', 'first'))).toEqual({
      size: first.size,
      type: first.type,
    })
    expect(await indexedDbCache.get('avatar', 'second')).toBeNull()
    expect(blobSummary(await indexedDbCache.get('avatar', 'third'))).toEqual({
      size: third.size,
      type: third.type,
    })
    expect(blobSummary(await indexedDbCache.get('viewer-snapshot', 'snapshot'))).toEqual({
      size: snapshot.size,
      type: snapshot.type,
    })
  })

  it('rejects entries larger than the bucket limit and removes any previous value', async () => {
    await indexedDbCache.set('avatar', 'too-large', new Blob(['old']), 10)

    await expect(indexedDbCache.set('avatar', 'too-large', blobOfSize(11, 'oversized'), 10)).resolves.toBe(
      false,
    )
    expect(await indexedDbCache.get('avatar', 'too-large')).toBeNull()
  })

  it('removes cached entries explicitly', async () => {
    await indexedDbCache.set('viewer-snapshot', 'skin-card', new Blob(['rendered']), 1024)
    await indexedDbCache.remove('viewer-snapshot', 'skin-card')

    expect(await indexedDbCache.get('viewer-snapshot', 'skin-card')).toBeNull()
  })
})
