import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render } from '@solidjs/testing-library'
import { setLocale } from '@/i18n'
import { PipelineEditor } from '../PipelineEditor'
import type { PipelineStep } from '@/types/domain'

setLocale('zh-CN')

function editor(steps: PipelineStep[], onChange = vi.fn()) {
  const view = render(() => <PipelineEditor steps={steps} onChange={onChange} />)
  return { ...view, onChange }
}

describe('PipelineEditor', () => {
  it('renders one row per step and the empty state for none', () => {
    const empty = editor([])
    expect(empty.getByText('未配置算子：原样输出所有源节点')).toBeTruthy()
    expect(empty.queryAllByLabelText('移除')).toHaveLength(0)
    empty.unmount()

    const filled = editor([{ type: 'deduplicate' }, { type: 'sort', by: 'tag', order: 'asc' }])
    expect(filled.queryByText('未配置算子：原样输出所有源节点')).toBeNull()
    expect(filled.queryAllByLabelText('移除')).toHaveLength(2)
  })

  // The editor is controlled: it must report changes through onChange and never
  // keep a mirror, or its reset effect re-triggers itself until the stack dies.
  it('appends a blank filter step without mutating the incoming array', () => {
    const steps: PipelineStep[] = [{ type: 'deduplicate' }]
    const { onChange, getByText } = editor(steps)

    fireEvent.click(getByText('添加算子'))

    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith([{ type: 'deduplicate' }, { type: 'filter', include: '', exclude: '' }])
    expect(steps).toHaveLength(1)
  })

  it('reorders by copying, so the parent array stays untouched', () => {
    const steps: PipelineStep[] = [{ type: 'filter', include: 'HK' }, { type: 'sort', by: 'tag', order: 'asc' }]
    const { onChange, getAllByLabelText } = editor(steps)

    fireEvent.click(getAllByLabelText('下移')[0])

    expect(onChange).toHaveBeenCalledWith([{ type: 'sort', by: 'tag', order: 'asc' }, { type: 'filter', include: 'HK' }])
    expect(steps[0].type).toBe('filter')
  })

  it('removes a step by index', () => {
    const steps: PipelineStep[] = [{ type: 'deduplicate' }, { type: 'rename', pattern: '^HK', repl: '[$1] ' }]
    const { onChange, getAllByLabelText } = editor(steps)

    fireEvent.click(getAllByLabelText('移除')[0])

    expect(onChange).toHaveBeenCalledWith([{ type: 'rename', pattern: '^HK', repl: '[$1] ' }])
  })
})
