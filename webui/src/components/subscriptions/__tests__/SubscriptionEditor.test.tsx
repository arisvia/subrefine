import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@solidjs/testing-library'
import { setLocale } from '@/i18n'
import { SubscriptionEditor } from '../SubscriptionEditor'
import type { Subscription } from '@/types/domain'

setLocale('zh-CN')

const base: Subscription = {
  id: 'hk', name: 'HK', token: 'tok', sources: ['ss://p:p@1.2.3.4:8389#A'],
  pipeline: [], enabled: true, created_at: 1, updated_at: 2,
}

describe('SubscriptionEditor', () => {
  it('keeps a pipeline step after adding it', () => {
    render(() => (
      <SubscriptionEditor
        open
        initial={base}
        knownIds={['hk']}
        saving={false}
        onClose={() => {}}
        onSubmit={() => {}}
      />
    ))
    expect(screen.queryAllByLabelText('移除')).toHaveLength(0)

    fireEvent.click(screen.getByText('添加算子'))

    expect(screen.queryAllByLabelText('移除')).toHaveLength(1)
  })
})
