import { type Component, For, Show } from 'solid-js'
import { useI18n } from '@/i18n'
import {
  Badge, Button, Checkbox, IconButton, Input, Select, IconChevronDown, IconChevronUp, IconSparkles, IconTrash,
} from '@/components/ui'
import {
  FINGERPRINTS, PROTOCOLS, PROTOCOL_LABELS,
  type PipelineStep, type PipelineType, type Protocol,
} from '@/types/domain'

const STEP_ORDER: PipelineType[] = ['filter', 'rename', 'deduplicate', 'decorate', 'sort', 'flag']

export interface PipelineEditorProps {
  steps: PipelineStep[]
  onChange: (next: PipelineStep[]) => void
}

/** Blank operator of the given kind, with the fields that step actually reads. */
function blankStep(type: PipelineType): PipelineStep {
  switch (type) {
    case 'filter': return { type, include: '', exclude: '' }
    case 'rename': return { type, pattern: '', repl: '' }
    case 'decorate': return { type, fingerprint: '', detour: '' }
    case 'sort': return { type, by: 'tag', order: 'asc' }
    case 'flag': return { type, flag_mode: 'add' }
    default: return { type }
  }
}

export const PipelineEditor: Component<PipelineEditorProps> = props => {
  const { t } = useI18n()

  // Fully controlled: SubscriptionEditor owns the pipeline signal, so this
  // component keeps no copy. A local mirror plus a sync effect can never
  // settle, because the reset writes a fresh array identity while reading the
  // same signal it depends on.
  const emit = (next: PipelineStep[]) => props.onChange(next)

  const patch = (index: number, part: Partial<PipelineStep>) => {
    emit(props.steps.map((s, i) => (i === index ? { ...s, ...part } : s)))
  }

  const move = (from: number, to: number) => {
    if (to < 0 || to >= props.steps.length) return
    const next = [...props.steps]
    ;[next[from], next[to]] = [next[to], next[from]]
    emit(next)
  }

  const typeLabel = (type: PipelineType) => t(`pipeline.type.${type}`)
  const typeOptions = STEP_ORDER.map(value => ({ value, label: typeLabel(value) }))

  return (
    <div class="space-y-2.5">
      <div class="flex items-center justify-between gap-3">
        <span class="text-xs font-medium text-muted">{t('pipeline.title')}</span>
        <Button size="sm" variant="secondary" onClick={() => emit([...props.steps, blankStep('filter')])}>
          <IconSparkles size={13} />
          {t('pipeline.add')}
        </Button>
      </div>

      <Show when={props.steps.length} fallback={
        <div class="rounded-control border border-dashed border-glass-border px-3 py-4 text-center text-xs text-faint">
          {t('pipeline.empty')}
        </div>
      }>
        <div class="space-y-2">
          <For each={props.steps}>
            {(step, index) => (
              <div class="rounded-control border border-glass-border bg-surface-inset/60 p-2.5">
                {/* One shared rhythm for every kind: fixed type column, equal
                    half-width parameter slots, actions vertically centered. */}
                <div class="flex items-center gap-2">
                  <div class="w-36 shrink-0">
                    <Select
                      size="sm"
                      ariaLabel={t('pipeline.title')}
                      value={step.type}
                      options={typeOptions}
                      onChange={v => patch(index(), blankStep(v as PipelineType))}
                    />
                  </div>

                  <div class="grid min-w-0 flex-1 grid-cols-2 gap-2">
                    <Show when={step.type === 'filter'}>
                      <Input size="sm" class="min-w-0" ariaLabel={t('pipeline.field.include')}
                        placeholder={t('pipeline.placeholder.include')}
                        value={step.include ?? ''} onInput={v => patch(index(), { include: v })} />
                      <Input size="sm" class="min-w-0" ariaLabel={t('pipeline.field.exclude')}
                        placeholder={t('pipeline.placeholder.exclude')}
                        value={step.exclude ?? ''} onInput={v => patch(index(), { exclude: v })} />
                    </Show>

                    <Show when={step.type === 'rename'}>
                      <Input size="sm" class="min-w-0" ariaLabel={t('pipeline.field.pattern')}
                        placeholder={t('pipeline.placeholder.pattern')}
                        value={step.pattern ?? ''} onInput={v => patch(index(), { pattern: v })} />
                      <Input size="sm" class="min-w-0" ariaLabel={t('pipeline.field.repl')}
                        placeholder={t('pipeline.placeholder.repl')}
                        value={step.repl ?? ''} onInput={v => patch(index(), { repl: v })} />
                    </Show>

                    <Show when={step.type === 'decorate'}>
                      <Select
                        size="sm"
                        ariaLabel={t('pipeline.field.fingerprint')}
                        value={step.fingerprint ?? ''}
                        options={[{ value: '', label: t('pipeline.noFingerprint') },
                          ...FINGERPRINTS.map(f => ({ value: f, label: f }))]}
                        onChange={v => patch(index(), { fingerprint: v })} />
                      <Input size="sm" class="min-w-0" ariaLabel={t('pipeline.field.detour')}
                        placeholder={t('pipeline.placeholder.detour')}
                        value={step.detour ?? ''} onInput={v => patch(index(), { detour: v })} />
                    </Show>

                    <Show when={step.type === 'sort'}>
                      <Select size="sm" ariaLabel={t('pipeline.field.by')} value={step.by ?? 'tag'}
                        options={[{ value: 'tag', label: t('pipeline.byTag') }, { value: 'protocol', label: t('pipeline.byProtocol') }]}
                        onChange={v => patch(index(), { by: v as PipelineStep['by'] })} />
                      <Select size="sm" ariaLabel={t('pipeline.field.order')} value={step.order ?? 'asc'}
                        options={[{ value: 'asc', label: t('pipeline.orderAsc') }, { value: 'desc', label: t('pipeline.orderDesc') }]}
                        onChange={v => patch(index(), { order: v as PipelineStep['order'] })} />
                    </Show>

                    <Show when={step.type === 'flag'}>
                      <Select size="sm" ariaLabel={t('pipeline.field.flagMode')} value={step.flag_mode ?? 'add'}
                        options={[
                          { value: 'add', label: t('pipeline.flagAdd') },
                          { value: 'remove', label: t('pipeline.flagRemove') },
                          { value: 'normalize', label: t('pipeline.flagNormalize') },
                        ]}
                        onChange={v => patch(index(), { flag_mode: v as PipelineStep['flag_mode'] })} />
                    </Show>
                  </div>

                  <div class="flex shrink-0 items-center gap-0.5">
                    <IconButton size="sm" aria-label={t('pipeline.moveUp')} disabled={index() === 0}
                      onClick={() => move(index(), index() - 1)}>
                      <IconChevronUp size={14} />
                    </IconButton>
                    <IconButton size="sm" aria-label={t('pipeline.moveDown')} disabled={index() === props.steps.length - 1}
                      onClick={() => move(index(), index() + 1)}>
                      <IconChevronDown size={14} />
                    </IconButton>
                    <IconButton size="sm" class="hover:text-danger" aria-label={t('pipeline.remove')}
                      onClick={() => emit(props.steps.filter((_, i) => i !== index()))}>
                      <IconTrash size={14} />
                    </IconButton>
                  </div>
                </div>

                <Show when={step.type === 'filter'}>
                  <div class="mt-2 space-y-1.5">
                    <div class="flex items-center gap-1.5 text-[11px] font-medium text-faint">
                      {t('pipeline.field.types')}
                      <Show when={!(step.types ?? []).length}>
                        <Badge tone="gray">{t('pipeline.keepAll')}</Badge>
                      </Show>
                    </div>
                    <div class="grid grid-cols-2 gap-x-3 gap-y-1.5 sm:grid-cols-3">
                      <For each={PROTOCOLS}>
                        {(p: Protocol) => (
                          <Checkbox
                            checked={(step.types ?? []).includes(p)}
                            label={PROTOCOL_LABELS[p]}
                            onChange={on => {
                              const cur = step.types ?? []
                              const types = on ? [...cur, p] : cur.filter(x => x !== p)
                              patch(index(), { types: types.length ? types : undefined })
                            }}
                          />
                        )}
                      </For>
                    </div>
                  </div>
                </Show>

                <div class="mt-1.5 pl-[9.5rem] text-[11px] text-faint">
                  {t(`pipeline.desc.${step.type}`)}
                </div>
              </div>
            )}
          </For>
        </div>
      </Show>
    </div>
  )
}
