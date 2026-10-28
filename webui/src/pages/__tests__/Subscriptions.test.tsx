import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
import { MemoryRouter, Route } from '@solidjs/router'
import { setLocale } from '@/i18n'
import { setAdminToken } from '@/lib/api'
import { resetSubscriptionsStoreForTesting } from '@/stores/subscriptions'
import { resetAuthStoreForTesting } from '@/stores/auth'
import Subscriptions from '../Subscriptions'
import type { Subscription } from '@/types/domain'

setLocale('zh-CN')

const hk: Subscription = {
  id: 'hk', name: 'HK', token: 'tok', sources: ['ss://p:p@1.2.3.4:8389#A'],
  pipeline: [], enabled: true, created_at: 1, updated_at: 2,
}

function json(payload: unknown) {
  return { ok: true, status: 200, text: () => Promise.resolve(JSON.stringify(payload)) }
}

describe('Subscriptions page', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    resetSubscriptionsStoreForTesting()
    resetAuthStoreForTesting()
    setAdminToken('admin-token')
  })

  it('keeps an added pipeline step in the editor', async () => {
    global.fetch = vi.fn().mockImplementation((path: string) => {
      const target = String(path)
      if (target.endsWith('/nodes')) return Promise.resolve(json({ id: 'hk', count: 1, nodes: [] }))
      return Promise.resolve(json([hk]))
    })

    render(() => (
      <MemoryRouter>
        <Route path="*" component={Subscriptions} />
      </MemoryRouter>
    ))

    await waitFor(() => expect(screen.queryAllByLabelText('编辑').length).toBeGreaterThan(0))
    fireEvent.click(screen.getAllByLabelText('编辑')[0])
    await waitFor(() => expect(screen.getByText('添加算子')).toBeTruthy())

    fireEvent.click(screen.getByText('添加算子'))

    await waitFor(() => expect(screen.queryAllByLabelText('移除')).toHaveLength(1))
  })
})
