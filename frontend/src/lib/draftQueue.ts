export type DraftQueueEntry = {
  key: string
  path: string
  method: 'POST' | 'PUT'
  payload: unknown
  queuedAt: number
}

const DB_NAME = 'pkbmti-lms-drafts'
const STORE_NAME = 'outbox'
const DB_VERSION = 1

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new Error('IndexedDB tidak tersedia pada browser ini.'))
      return
    }
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) db.createObjectStore(STORE_NAME, { keyPath: 'key' })
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error || new Error('Gagal membuka penyimpanan lokal.'))
  })
}

export async function enqueueDraft(entry: DraftQueueEntry) {
  const db = await openDB()
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).put(entry)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error || new Error('Draf offline tidak dapat disimpan.'))
  })
  db.close()
}

export async function listDrafts(): Promise<DraftQueueEntry[]> {
  const db = await openDB()
  const values = await new Promise<DraftQueueEntry[]>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const request = tx.objectStore(STORE_NAME).getAll()
    request.onsuccess = () => resolve((request.result || []) as DraftQueueEntry[])
    request.onerror = () => reject(request.error || new Error('Draf offline tidak dapat dibaca.'))
  })
  db.close()
  return values.sort((a, b) => a.queuedAt - b.queuedAt)
}

export async function removeDraft(key: string) {
  const db = await openDB()
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).delete(key)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error || new Error('Draf offline tidak dapat dihapus.'))
  })
  db.close()
}

export async function countDrafts() {
  try {
    const db = await openDB()
    const count = await new Promise<number>((resolve, reject) => {
      const tx = db.transaction(STORE_NAME, 'readonly')
      const request = tx.objectStore(STORE_NAME).count()
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error || new Error('Gagal membaca antrean.'))
    })
    db.close()
    return count
  } catch {
    return 0
  }
}
