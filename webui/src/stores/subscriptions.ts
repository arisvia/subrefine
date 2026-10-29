import { createRoot, createSignal, createMemo, untrack, type Accessor } from 'solid-js'
import { api, apiDelete, apiPost } from '@/lib/api'
import type { NodePreview, ProxyNode, Subscription } from '@/types/domain'

export interface PreviewState {
  loading: boolean
  error: string
  nodes: ProxyNode[]
  /** Share URIs index-aligned with nodes, from the /nodes response. */
  uris: string[]
  /** Stable identities index-aligned with nodes, for per-key blacklists. */
  ids: string[]
}

const EMPTY_PREVIEW: PreviewState = { loading: false, error: '', nodes: [], uris: [], ids: [] }

export interface SubscriptionsStore {
  subs: Accessor<Subscription[]>
  loading: Accessor<boolean>
  error: Accessor<string>
  byId: (id: string) => Subscription | undefined
  preview: (id: string) => PreviewState
  load: (force?: boolean) => Promise<void>
  save: (sub: Subscription) => Promise<boolean>
  remove: (id: string) => Promise<boolean>
  runPreview: (id: string, force?: boolean) => Promise<void>
  totals: Accessor<{ subs: number; enabled: number; sources: number; operators: number }>
}

function createSubscriptionsStore(): SubscriptionsStore {
  const [subs, setSubs] = createSignal<Subscription[]>([])
  const [loading, setLoading] = createSignal(false)
  const [error, setError] = createSignal('')
  const [previews, setPreviews] = createSignal<Record<string, PreviewState>>({})

  const inFlight = new Map<string, Promise<void>>()
  let loaded = false

  // The overview fans previews out across every subscription at once; cap the
  // upstream work in the store so any caller gets the same protection and a
  // big list never opens one connection per row.
  const MAX_CONCURRENT_PREVIEWS = 2
  let activePreviews = 0
  const previewQueue: Array<() => void> = []

  function acquireSlot(): Promise<void> {
    if (activePreviews < MAX_CONCURRENT_PREVIEWS) {
      activePreviews++
      return Promise.resolve()
    }
    return new Promise<void>(res => previewQueue.push(res))
  }

  function releaseSlot() {
    const next = previewQueue.shift()
    // Hand-off keeps the count: the waiter inherits this slot.
    if (next) next()
    else activePreviews--
  }

  const byId = (id: string) => subs().find(s => s.id === id)
  const preview = (id: string): PreviewState => previews()[id] ?? EMPTY_PREVIEW

  const totals = createMemo(() => {
    const list = subs()
    return {
      subs: list.length,
      enabled: list.filter(s => s.enabled).length,
      sources: list.reduce((n, s) => n + (s.sources?.length ?? 0), 0),
      operators: list.reduce((n, s) => n + (s.pipeline?.length ?? 0), 0),
    }
  })

  async function load(force = false) {
    if (loaded && !force) return
    setLoading(true)
    setError('')
    try {
      const list = await api<Subscription[]>('/api/admin/subscriptions')
      setSubs(Array.isArray(list) ? list : [])
      loaded = true
    } catch (err) {
      setError(err instanceof Error ? err.message : 'load failed')
    } finally {
      setLoading(false)
    }
  }

  async function save(sub: Subscription): Promise<boolean> {
    try {
      await apiPost('/api/admin/subscriptions', sub)
      // Any definition change invalidates the node cache server-side; drop our
      // copies so the next preview refetches instead of showing stale nodes.
      setPreviews({})
      await load(true)
      return true
    } catch {
      return false // the api layer already surfaced the failure as a toast
    }
  }

  async function remove(id: string): Promise<boolean> {
    try {
      await apiDelete(`/api/admin/subscriptions/${encodeURIComponent(id)}`)
      const next = { ...previews() }
      delete next[id]
      setPreviews(next)
      await load(true)
      return true
    } catch {
      return false
    }
  }

  async function runPreview(id: string, force = false) {
    // Callers trigger this from createEffect, so nothing here may leave a
    // tracked read of `previews` behind: writing it would re-run the effect
    // that just read it, recursing until the stack dies.
    const existing = untrack(() => preview(id))
    if (!force && !existing.error && existing.nodes.length) return
    // A second request for the same id while one is pending must not refetch.
    const pending = inFlight.get(id)
    if (pending) return pending

    setPreviews(p => ({ ...p, [id]: { ...(p[id] ?? EMPTY_PREVIEW), loading: true, error: '' } }))
    const task = (async () => {
      await acquireSlot()
      try {
        const res = await api<NodePreview>(
          `/api/admin/subscriptions/${encodeURIComponent(id)}/nodes`,
          { silent: true },
        )
        setPreviews(p => ({ ...p, [id]: { loading: false, error: '', nodes: res.nodes ?? [], uris: res.uris ?? [], ids: res.ids ?? [] } }))
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'preview failed'
        setPreviews(p => ({ ...p, [id]: { loading: false, error: msg, nodes: [], uris: [], ids: [] } }))
      } finally {
        releaseSlot()
        inFlight.delete(id)
      }
    })()
    inFlight.set(id, task)
    return task
  }

  return { subs, loading, error, byId, preview, load, save, remove, runPreview, totals }
}

let store: SubscriptionsStore | undefined

export function useSubscriptionsStore(): SubscriptionsStore {
  if (!store) store = createRoot(createSubscriptionsStore)
  return store
}

export function resetSubscriptionsStoreForTesting(): void {
  store = undefined
}
