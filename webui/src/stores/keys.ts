import { createRoot, createSignal } from 'solid-js'
import { api, apiDelete, apiPost } from '@/lib/api'
import type { AdminKey } from '@/types/domain'

export interface KeysStore {
  keys: () => AdminKey[]
  loading: () => boolean
  error: () => string
  byId: (id: string) => AdminKey | undefined
  load: () => Promise<void>
  create: (name: string, token?: string) => Promise<AdminKey | null>
  update: (id: string, patch: { name?: string; token?: string }) => Promise<AdminKey | null>
  remove: (id: string, cascade?: boolean) => Promise<boolean>
}

function createKeysStore(): KeysStore {
  const [keys, setKeys] = createSignal<AdminKey[]>([])
  const [loading, setLoading] = createSignal(false)
  const [error, setError] = createSignal('')

  async function load() {
    setLoading(true)
    setError('')
    try {
      setKeys(await api<AdminKey[]>('/api/admin/keys'))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'keys load failed')
    } finally {
      setLoading(false)
    }
  }

  async function create(name: string, token?: string) {
    try {
      const created = await apiPost<AdminKey>('/api/admin/keys', { name, token: token ?? '' })
      setKeys(prev => [...prev, created])
      return created
    } catch {
      return null
    }
  }

  async function update(id: string, patch: { name?: string; token?: string }) {
    try {
      const next = await api<AdminKey>(`/api/admin/keys/${id}`, { method: 'PUT', body: patch })
      setKeys(prev => prev.map(k => (k.id === id ? next : k)))
      return next
    } catch {
      return null
    }
  }

  async function remove(id: string, cascade = false) {
    try {
      await apiDelete(`/api/admin/keys/${id}${cascade ? '?cascade=1' : ''}`)
      setKeys(prev => prev.filter(k => k.id !== id))
      return true
    } catch {
      return false
    }
  }

  return { keys, loading, error, byId: id => keys().find(k => k.id === id), load, create, update, remove }
}

let store: KeysStore | undefined

export function useKeysStore(): KeysStore {
  if (!store) store = createRoot(createKeysStore)
  return store
}

export function resetKeysStoreForTesting(): void {
  store = undefined
}
