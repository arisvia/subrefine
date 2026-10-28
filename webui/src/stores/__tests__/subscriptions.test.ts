import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createEffect, createRoot } from 'solid-js'
import { setAdminToken } from '@/lib/api'
import { resetSubscriptionsStoreForTesting, useSubscriptionsStore } from '@/stores/subscriptions'
import type { ProxyNode, Subscription } from '@/types/domain'

function sub(over: Partial<Subscription> = {}): Subscription {
  return {
    id: 'hk', name: 'HK', token: 'tok', sources: ['https://example.test/sub'],
    pipeline: [], enabled: true, created_at: 1, updated_at: 2, ...over,
  }
}

function nodes(...tags: string[]): ProxyNode[] {
  return tags.map(tag => ({ tag, type: 'shadowsocks', server: '1.2.3.4', port: 8388 }) as ProxyNode)
}

function json(payload: unknown) {
  return { ok: true, status: 200, text: () => Promise.resolve(JSON.stringify(payload)) }
}

describe('subscriptions store', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    resetSubscriptionsStoreForTesting()
    setAdminToken('admin-token')
  })

  it('attaches the admin bearer token to every request', async () => {
    const fetchMock = vi.fn().mockResolvedValue(json([sub()]))
    global.fetch = fetchMock
    await useSubscriptionsStore().load()

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer admin-token')
  })

  it('drops cached node previews after a save so the edit takes effect', async () => {
    const store = useSubscriptionsStore()
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(json([sub()]))          // load()
      .mockResolvedValueOnce(json({ id: 'hk', count: 2, nodes: nodes('a', 'b') })) // preview
      .mockResolvedValueOnce(json({}))               // save()
      .mockResolvedValueOnce(json([sub()]))          // load(force)
      .mockResolvedValueOnce(json({ id: 'hk', count: 1, nodes: nodes('a') }))      // preview again
    global.fetch = fetchMock

    await store.load()
    await store.runPreview('hk')
    expect(store.preview('hk').nodes).toHaveLength(2)

    await store.save(sub({ pipeline: [{ type: 'deduplicate' }] }))
    await store.runPreview('hk')

    expect(store.preview('hk').nodes).toHaveLength(1)
    const previewCalls = fetchMock.mock.calls.filter(call => String(call[0]).endsWith('/nodes'))
    expect(previewCalls).toHaveLength(2)
  })

  it('coalesces concurrent previews of the same subscription into one request', async () => {
    const store = useSubscriptionsStore()
    const gate = Promise.withResolvers<unknown>()
    const fetchMock = vi.fn().mockReturnValue(gate.promise)
    global.fetch = fetchMock

    const first = store.runPreview('hk')
    const second = store.runPreview('hk')
    gate.resolve(json({ id: 'hk', count: 1, nodes: nodes('a') }))
    await Promise.all([first, second])

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(store.preview('hk').nodes).toHaveLength(1)
  })

  // The overview fans out across every subscription at once; the store, not
  // the page, owns the bound — five queued previews must never mean five
  // simultaneous upstream requests. Concurrency is driven deterministically:
  // every /nodes promise is held until the test resolves it by hand.
  it('caps concurrent previews across different subscriptions at two', async () => {
    const store = useSubscriptionsStore()
    let inflight = 0
    let peak = 0
    let started = 0
    const gates: Array<(payload: unknown) => void> = []

    global.fetch = vi.fn().mockImplementation((path: string) => {
      if (String(path).includes('/nodes')) {
        started++
        inflight++
        peak = Math.max(peak, inflight)
        return new Promise(resolve => {
          gates.push((payload: unknown) => {
            inflight--
            resolve(payload)
          })
        })
      }
      return Promise.resolve(json([]))
    })

    // Deterministic drain: each await releases one queued microtask, enough
    // ticks for the store's promise chains to run without wall-clock waiting.
    const flush = async () => {
      for (let i = 0; i < 32; i++) await Promise.resolve()
    }
    const ids = ['a', 'b', 'c', 'd', 'e']
    const all = Promise.all(ids.map(id => store.runPreview(id)))

    // Keep releasing whichever previews are held until all five have run;
    // queueMicrotask-level flushing would race the promise chain inside the
    // store, so yield through the macrotask queue between waves.
    for (let done = 0; done < ids.length; done++) {
      await flush()
      const release = gates.shift()
      if (release) release(json({ id: 'x', count: 1, nodes: nodes('x'), uris: ['ss://x'] }))
    }
    await all

    expect(started).toBe(5)
    expect(peak).toBe(2)
    for (const id of ids) expect(store.preview(id).nodes).toHaveLength(1)
  })

  it('surfaces a preview failure without blocking the list', async () => {
    const store = useSubscriptionsStore()
    global.fetch = vi.fn().mockImplementation((path: string) => Promise.resolve(
      String(path).includes('/nodes')
        ? { ok: false, status: 403, json: () => Promise.resolve({ error: 'forbidden' }), text: () => Promise.resolve('') }
        : json([sub({ id: 'other' })]),
    ))

    await store.load()
    await store.runPreview('hk')

    expect(store.preview('hk').error).toContain('forbidden')
    expect(store.preview('hk').nodes).toEqual([])
    expect(store.subs()).toHaveLength(1)
  })

  // The overview fans previews out from a createEffect. Preview writes must not
  // be a dependency of that effect: a subscription resolving with an error or
  // with zero nodes used to re-trigger it, recursing until the stack died.
  it('runs the preview fan-out once per subscription list', async () => {
    const store = useSubscriptionsStore()
    const paths: string[] = []
    global.fetch = vi.fn().mockImplementation((path: string) => {
      const target = String(path)
      paths.push(target)
      if (target.endsWith('/nodes')) {
        return Promise.resolve(target.includes('/boom/')
          ? { ok: false, status: 500, json: () => Promise.resolve({ error: 'upstream failed' }), text: () => Promise.resolve('') }
          : json({ id: 'ok', count: 0, nodes: [] as ProxyNode[] }))
      }
      return Promise.resolve(json([sub({ id: 'ok' }), sub({ id: 'boom' })]))
    })

    await store.load()

    let runs = 0
    const finished = Promise.withResolvers<void>()
    createRoot(dispose => {
      createEffect(() => {
        const list = store.subs()
        runs++
        void Promise.all(list.map(s => store.runPreview(s.id))).then(() => {
          dispose()
          finished.resolve()
        })
      })
    })
    await finished.promise

    expect(runs).toBe(1)
    expect(paths.filter(p => p.endsWith('/nodes'))).toHaveLength(2)
    expect(store.preview('boom').error).toContain('upstream failed')
  })
})
