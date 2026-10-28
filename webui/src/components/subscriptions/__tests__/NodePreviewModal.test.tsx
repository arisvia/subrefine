import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
import { setLocale } from '@/i18n'
import { NodePreviewModal } from '../NodePreviewModal'
import type { ProxyNode } from '@/types/domain'

setLocale('zh-CN')

const copied = vi.hoisted((): string[] => [])
vi.mock('@/lib/clipboard', () => ({
  copyToClipboard: (text: string) => {
    copied.push(text)
    return Promise.resolve(true)
  },
}))

const alpha: ProxyNode = {
  tag: 'Alpha', type: 'shadowsocks', server: '1.2.3.4', port: 8389,
  method: 'aes-128-gcm', password: 'p', fingerprint: 'safari',
}
const beta: ProxyNode = {
  tag: 'Beta', type: 'trojan', server: '5.6.7.8', port: 443, password: 'q',
}
const uriA = 'ss://k@1.2.3.4:8389#Alpha'
const uriB = 'trojan://q@5.6.7.8:443#Beta'

function openModal(nodes: ProxyNode[], uris?: string[]) {
  return render(() => (
    <NodePreviewModal
      open
      id="hk"
      loading={false}
      error=""
      nodes={nodes}
      uris={uris}
      onClose={() => undefined}
      onRefresh={() => undefined}
    />
  ))
}

describe('NodePreviewModal', () => {
  beforeEach(() => copied.length = 0)

  it('renders the decorate fingerprint badge', () => {
    openModal([alpha], [uriA])
    expect(screen.getByText('safari')).toBeTruthy()
  })

  // The list renders the filtered pairs while `uris` is aligned to the full
  // node array; indexing the filtered row position into that array would hand
  // out the wrong node's credentials the moment the search box has text.
  it('copies the matching node URI while the list is filtered', async () => {
    openModal([alpha, beta], [uriA, uriB])
    fireEvent.input(screen.getByLabelText('过滤标签 / 协议 / 地址'), { target: { value: 'Beta' } })
    fireEvent.click(screen.getByLabelText('复制'))
    await waitFor(() => expect(copied).toEqual([uriB]))
  })

  it('copies the server-provided share URI when unfiltered', async () => {
    openModal([beta], [uriB])
    fireEvent.click(screen.getByLabelText('复制'))
    await waitFor(() => expect(copied).toEqual([uriB]))
  })

  it('falls back to JSON when no URI is encodable', async () => {
    openModal([alpha], [''])
    fireEvent.click(screen.getByLabelText('复制'))
    await waitFor(() => expect(copied.length).toBe(1))
    expect(JSON.parse(copied[0]).tag).toBe('Alpha')
  })
})
