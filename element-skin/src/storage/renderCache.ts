import { indexedDbCache, type CacheBucket } from '@/storage/indexedDbCache'

const AVATAR_CACHE_MAX_BYTES = 2 * 1024 * 1024
const VIEWER_SNAPSHOT_CACHE_MAX_BYTES = 20 * 1024 * 1024

const objectUrls = new Map<string, string>()

function objectUrlKey(bucket: CacheBucket, key: string): string {
  return `${bucket}:${key}`
}

function blobToObjectUrl(bucket: CacheBucket, key: string, blob: Blob): string {
  const id = objectUrlKey(bucket, key)
  const existing = objectUrls.get(id)
  if (existing) return existing
  const url = URL.createObjectURL(blob)
  objectUrls.set(id, url)
  return url
}

export function canvasToBlob(canvas: HTMLCanvasElement): Promise<Blob | null> {
  return new Promise((resolve) => {
    canvas.toBlob((blob) => resolve(blob), 'image/png')
  })
}

export async function getCachedImageUrl(bucket: CacheBucket, key: string): Promise<string | null> {
  const blob = await indexedDbCache.get(bucket, key)
  if (!blob) return null
  return blobToObjectUrl(bucket, key, blob)
}

export async function setCachedImageUrl(bucket: CacheBucket, key: string, blob: Blob): Promise<string | null> {
  const maxBytes = bucket === 'avatar' ? AVATAR_CACHE_MAX_BYTES : VIEWER_SNAPSHOT_CACHE_MAX_BYTES
  const stored = await indexedDbCache.set(bucket, key, blob, maxBytes)
  if (!stored) return null
  return blobToObjectUrl(bucket, key, blob)
}

export function avatarCacheKey(hash: string, model: string): string {
  return `avatar:${hash}:${model}:256x256`
}

export function skinSnapshotCacheKey(options: {
  skinUrl: string
  capeUrl?: string | null
  model: string
  width: number
  height: number
}): string {
  return JSON.stringify({
    type: 'skin',
    skinUrl: options.skinUrl,
    capeUrl: options.capeUrl ?? null,
    model: options.model,
    width: options.width,
    height: options.height,
  })
}

export function capeSnapshotCacheKey(options: { capeUrl: string; width: number; height: number }): string {
  return JSON.stringify({
    type: 'cape',
    capeUrl: options.capeUrl,
    width: options.width,
    height: options.height,
  })
}
