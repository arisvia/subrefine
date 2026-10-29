import { type Component, For, Show } from 'solid-js'
import { useI18n } from '@/i18n'
import {
  Badge, Button, Checkbox, IconButton, Input, Select, IconChevronDown, IconChevronUp, IconSparkles, IconTrash,
} from '@/components/ui'
import {
  FINGERPRINTS, PIPELINE_TONES, PROTOCOLS, PROTOCOL_LABELS,
  type PipelineStep, type PipelineType, type Protocol,
} from '@/types/domain'

const STEP_ORDER: PipelineType[] = ['filter', 'rename', 'deduplicate', 'decorate', 'sort']

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
                <div class="flex items-start gap-2">
                  <div class="shrink-0 w-40">
                    <Select
                      size="sm"
                      ariaLabel={t('pipeline.title')}
                      value={step.type}
                      options={typeOptions}
                      onChange={v => patch(index(), blankStep(v as PipelineType))}
                    />
                  </div>

                  <div class="flex-1 min-w-0 flex flex-wrap items-center gap-2">
                    <Show when={step.type === 'filter'}>
                      <Input size="sm" class="flex-1 min-w-36" ariaLabel={t('pipeline.field.include')}
                        placeholder={t('pipeline.placeholder.include')}
                        value={step.include ?? ''} onInput={v => patch(index(), { include: v })} />
                      <Input size="sm" class="flex-1 min-w-36" ariaLabel={t('pipeline.field.exclude')}
                        placeholder={t('pipeline.placeholder.exclude')}
                        value={step.exclude ?? ''} onInput={v => patch(index(), { exclude: v })} />
                    </Show>

                    <Show when={step.type === 'rename'}>
                      <Input size="sm" class="flex-1 min-w-40" ariaLabel={t('pipeline.field.pattern')}
                        placeholder={t('pipeline.placeholder.pattern')}
                        value={step.pattern ?? ''} onInput={v => patch(index(), { pattern: v })} />
                      <Input size="sm" class="flex-1 min-w-32" ariaLabel={t('pipeline.field.repl')}
                        placeholder={t('pipeline.placeholder.repl')}
                        value={step.repl ?? ''} onInput={v => patch(index(), { repl: v })} />
                    </Show>

                    <Show when={step.type === 'decorate'}>
                      <div class="w-36 shrink-0">
                        <Select
                          size="sm"
                          ariaLabel={t('pipeline.field.fingerprint')}
                          value={step.fingerprint ?? ''}
                          options={[{ value: '', label: t('pipeline.noFingerprint') },
                            ...FINGERPRINTS.map(f => ({ value: f, label: f }))]}
                          onChange={v => patch(index(), { fingerprint: v })}
                        />
                      </div>
                      <Input size="sm" class="flex-1 min-w-28" ariaLabel={t('pipeline.field.detour')}
                        placeholder={t('pipeline.placeholder.detour')}
                        value={step.detour ?? ''} onInput={v => patch(index(), { detour: v })} />
                    </Show>

                    <Show when={step.type === 'sort'}>
                      <div class="w-32 shrink-0">
                        <Select size="sm" ariaLabel={t('pipeline.field.by')} value={step.by ?? 'tag'}
                          options={[{ value: 'tag', label: t('pipeline.byTag') }, { value: 'protocol', label: t('pipeline.byProtocol') }]}
                          onChange={v => patch(index(), { by: v as PipelineStep['by'] })} />
                      </div>
                      <div class="w-32 shrink-0">
                        <Select size="sm" ariaLabel={t('pipeline.field.order')} value={step.order ?? 'asc'}
                          options={[{ value: 'asc', label: t('pipeline.orderAsc') }, { value: 'desc', label: t('pipeline.orderDesc') }]}
                          onChange={v => patch(index(), { order: v as PipelineStep['order'] })} />
                      </div>
                    </Show>

                    <Show when={step.type === 'deduplicate'}>
                      <span class="text-xs text-faint">{t('pipeline.desc.deduplicate')}</span>
                    </Show>
                  </div>

                  <div class="flex items-center gap-0.5 shrink-0">
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
                  <div class="mt-2.5 flex flex-wrap items-center gap-x-3.5 gap-y-2 pl-0.5">
                    <span class="text-[11px] font-medium text-faint mr-1">{t('pipeline.field.types')}</span>
                    <For each={PROTOCOLS}>
                      {(p: Protocol) => {
                        const checked = () => (step.types ?? []).includes(p)
                        return (
                          <Checkbox
                            checked={checked()}
                            label={PROTOCOL_LABELS[p]}
                            onChange={on => {
                              const cur = step.types ?? []
                              const types = on ? [...cur, p] : cur.filter(x => x !== p)
                              patch(index(), { types: types.length ? types : undefined })
                            }}
                          />
                        )
                      }}
                    </For>
                    <Show when={!(step.types ?? []).length}>
                      <Badge tone="gray">{t('pipeline.keepAll')}</Badge>
                    </Show>
                  </div>
                </Show>

                <div class="mt-1.5 flex items-center gap-2">
                  <Badge tone={PIPELINE_TONES[step.type]}>{typeLabel(step.type)}</Badge>
                  <span class="text-[11px] text-faint">{t(`pipeline.desc.${step.type}`)}</span>
                </div>
              </div>
            )}
          </For>
        </div>
      </Show>
    </div>
  )
}
