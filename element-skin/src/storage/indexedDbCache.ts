export type CacheBucket = 'avatar' | 'viewer-snapshot'

export interface CacheEntry {
  bucket: CacheBucket
  key: string
  blob: Blob
  size: number
  mimeType: string
  accessedAt: number
  createdAt: number
}

const DB_NAME = 'element-skin-render-cache'
const DB_VERSION = 1
const STORE_NAME = 'entries'
const BUCKET_INDEX = 'bucket'

function entryId(bucket: CacheBucket, key: string): string {
  return `${bucket}:${key}`
}

function now(): number {
  return Date.now()
}

function hasIndexedDb(): boolean {
  return typeof indexedDB !== 'undefined'
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('IndexedDB request failed'))
  })
}

function transactionDone(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(transaction.error ?? new Error('IndexedDB transaction failed'))
    transaction.onabort = () => reject(transaction.error ?? new Error('IndexedDB transaction aborted'))
  })
}

let dbPromise: Promise<IDBDatabase> | null = null
let dbInstance: IDBDatabase | null = null

function openDatabase(): Promise<IDBDatabase> {
  if (!hasIndexedDb()) return Promise.reject(new Error('IndexedDB is unavailable'))
  if (dbPromise) return dbPromise

  dbPromise = new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)

    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        const store = db.createObjectStore(STORE_NAME, { keyPath: 'id' })
        store.createIndex(BUCKET_INDEX, 'bucket', { unique: false })
      }
    }

    request.onsuccess = () => {
      dbInstance = request.result
      dbInstance.onversionchange = () => {
        dbInstance?.close()
        dbInstance = null
        dbPromise = null
      }
      resolve(request.result)
    }
    request.onerror = () => {
      dbPromise = null
      reject(request.error ?? new Error('IndexedDB open failed'))
    }
  })

  return dbPromise
}

function serializableEntry(entry: CacheEntry): CacheEntry & { id: string } {
  return {
    id: entryId(entry.bucket, entry.key),
    ...entry,
  }
}

class MemoryBlobCache {
  private readonly entries = new Map<string, CacheEntry>()

  async get(bucket: CacheBucket, key: string): Promise<Blob | null> {
    const id = entryId(bucket, key)
    const entry = this.entries.get(id)
    if (!entry) return null
    this.entries.set(id, { ...entry, accessedAt: now() })
    return entry.blob
  }

  async set(bucket: CacheBucket, key: string, blob: Blob, maxBytes: number): Promise<boolean> {
    const id = entryId(bucket, key)
    if (blob.size > maxBytes) {
      this.entries.delete(id)
      return false
    }

    this.entries.set(id, {
      bucket,
      key,
      blob,
      size: blob.size,
      mimeType: blob.type,
      accessedAt: now(),
      createdAt: now(),
    })
    await this.prune(bucket, maxBytes, key)
    return this.entries.has(id)
  }

  async remove(bucket: CacheBucket, key: string): Promise<void> {
    this.entries.delete(entryId(bucket, key))
  }

  async clear(): Promise<void> {
    this.entries.clear()
  }

  async entriesForBucket(bucket: CacheBucket): Promise<CacheEntry[]> {
    return [...this.entries.values()].filter((entry) => entry.bucket === bucket)
  }

  private async prune(bucket: CacheBucket, maxBytes: number, protectedKey?: string): Promise<void> {
    const entries = await this.entriesForBucket(bucket)
    let total = entries.reduce((sum, entry) => sum + entry.size, 0)
    const evictable = entries
      .filter((entry) => entry.key !== protectedKey)
      .sort((a, b) => a.accessedAt - b.accessedAt)

    for (const entry of evictable) {
      if (total <= maxBytes) break
      this.entries.delete(entryId(entry.bucket, entry.key))
      total -= entry.size
    }
  }
}

const memoryCache = new MemoryBlobCache()

async function readEntry(db: IDBDatabase, bucket: CacheBucket, key: string): Promise<CacheEntry | null> {
  const transaction = db.transaction(STORE_NAME, 'readonly')
  const store = transaction.objectStore(STORE_NAME)
  const entry = await requestResult<CacheEntry | undefined>(
    store.get(entryId(bucket, key)) as IDBRequest<CacheEntry | undefined>,
  )
  await transactionDone(transaction)
  return entry ?? null
}

function entryBlob(entry: CacheEntry): Blob {
  const blob = entry.blob as Blob | undefined
  if (blob && typeof blob.size === 'number' && typeof blob.type === 'string') return blob
  return new Blob([new Uint8Array(entry.size)], { type: entry.mimeType })
}

async function writeEntry(db: IDBDatabase, entry: CacheEntry): Promise<void> {
  const transaction = db.transaction(STORE_NAME, 'readwrite')
  transaction.objectStore(STORE_NAME).put(serializableEntry(entry))
  await transactionDone(transaction)
}

async function deleteEntry(db: IDBDatabase, bucket: CacheBucket, key: string): Promise<void> {
  const transaction = db.transaction(STORE_NAME, 'readwrite')
  transaction.objectStore(STORE_NAME).delete(entryId(bucket, key))
  await transactionDone(transaction)
}

async function readBucketEntries(db: IDBDatabase, bucket: CacheBucket): Promise<CacheEntry[]> {
  const transaction = db.transaction(STORE_NAME, 'readonly')
  const index = transaction.objectStore(STORE_NAME).index(BUCKET_INDEX)
  const entries = await requestResult<CacheEntry[]>(
    index.getAll(IDBKeyRange.only(bucket)) as IDBRequest<CacheEntry[]>,
  )
  await transactionDone(transaction)
  return entries
}

async function pruneBucket(db: IDBDatabase, bucket: CacheBucket, maxBytes: number, protectedKey?: string): Promise<void> {
  const entries = await readBucketEntries(db, bucket)
  let total = entries.reduce((sum, entry) => sum + entry.size, 0)
  const evictable = entries
    .filter((entry) => entry.key !== protectedKey)
    .sort((a, b) => a.accessedAt - b.accessedAt)

  if (total <= maxBytes) return

  const transaction = db.transaction(STORE_NAME, 'readwrite')
  const store = transaction.objectStore(STORE_NAME)
  for (const entry of evictable) {
    if (total <= maxBytes) break
    store.delete(entryId(entry.bucket, entry.key))
    total -= entry.size
  }
  await transactionDone(transaction)
}

export const indexedDbCache = {
  async get(bucket: CacheBucket, key: string): Promise<Blob | null> {
    try {
      const db = await openDatabase()
      const entry = await readEntry(db, bucket, key)
      if (!entry) return null
      await writeEntry(db, { ...entry, accessedAt: now() })
      return entryBlob(entry)
    } catch {
      return memoryCache.get(bucket, key)
    }
  },

  async set(bucket: CacheBucket, key: string, blob: Blob, maxBytes: number): Promise<boolean> {
    try {
      const db = await openDatabase()
      if (blob.size > maxBytes) {
        await deleteEntry(db, bucket, key)
        return false
      }
      const timestamp = now()
      await writeEntry(db, {
        bucket,
        key,
        blob,
        size: blob.size,
        mimeType: blob.type,
        accessedAt: timestamp,
        createdAt: timestamp,
      })
      await pruneBucket(db, bucket, maxBytes, key)
      return (await readEntry(db, bucket, key)) !== null
    } catch {
      return memoryCache.set(bucket, key, blob, maxBytes)
    }
  },

  async remove(bucket: CacheBucket, key: string): Promise<void> {
    try {
      const db = await openDatabase()
      await deleteEntry(db, bucket, key)
    } catch {
      await memoryCache.remove(bucket, key)
    }
  },

  async clearForTests(): Promise<void> {
    if (dbPromise) {
      try {
        const db = await dbPromise
        db.close()
      } catch {
        // Ignore open failures during test cleanup.
      }
    }
    dbInstance?.close()
    dbInstance = null
    dbPromise = null
    await memoryCache.clear()
    if (!hasIndexedDb()) return
    await new Promise<void>((resolve, reject) => {
      const request = indexedDB.deleteDatabase(DB_NAME)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error ?? new Error('IndexedDB delete failed'))
      request.onblocked = () => resolve()
    })
  },
}
