import { type Component, Show, createEffect, createMemo, createSignal } from 'solid-js'
import { useI18n } from '@/i18n'
import { Alert, Button, Field, Input, Modal, Textarea, Toggle } from '@/components/ui'
import { PipelineEditor } from './PipelineEditor'
import type { PipelineStep, Subscription } from '@/types/domain'

const ID_PATTERN = /^[A-Za-z0-9_-]+$/

export interface SubscriptionEditorProps {
  open: boolean
  /** null → create mode; a subscription → edit mode (ID is immutable). */
  initial: Subscription | null
  knownIds: string[]
  saving: boolean
  onClose: () => void
  onSubmit: (sub: Subscription) => void
}

function emptyDraft(): Subscription {
  return { id: '', name: '', token: '', sources: [], pipeline: [], enabled: true, created_at: 0, updated_at: 0 }
}

export const SubscriptionEditor: Component<SubscriptionEditorProps> = props => {
  const { t } = useI18n()
  const [draft, setDraft] = createSignal<Subscription>(emptyDraft())
  const [sourcesText, setSourcesText] = createSignal('')
  const [pipeline, setPipeline] = createSignal<PipelineStep[]>([])
  const [touched, setTouched] = createSignal(false)

  createEffect(() => {
    if (!props.open) return
    const src = props.initial ?? emptyDraft()
    setDraft({ ...src })
    setSourcesText((src.sources ?? []).join('\n'))
    setPipeline((src.pipeline ?? []).map(s => ({ ...s })))
    setTouched(false)
  })

  const editing = () => !!props.initial
  const sources = createMemo(() => sourcesText().split('\n').map(s => s.trim()).filter(Boolean))

  const problems = createMemo(() => {
    const out: string[] = []
    const d = draft()
    if (!d.id.trim()) out.push(t('subs.idRequired'))
    else if (!ID_PATTERN.test(d.id.trim())) out.push(t('subs.idPattern'))
    if (!d.token.trim()) out.push(t('subs.tokenRequired'))
    if (!sources().length) out.push(t('subs.sourcesRequired'))
    return out
  })

  const overwrites = createMemo(() =>
    !editing() && !!draft().id.trim() && props.knownIds.includes(draft().id.trim()))

  const submit = () => {
    setTouched(true)
    if (problems().length) return
    props.onSubmit({
      ...draft(),
      id: draft().id.trim(),
      name: draft().name.trim() || draft().id.trim(),
      token: draft().token.trim(),
      sources: sources(),
      pipeline: pipeline(),
    })
  }

  return (
    <Modal
      open={props.open}
      onClose={props.onClose}
      size="2xl"
      title={editing() ? t('subs.editTitle', { id: props.initial!.id }) : t('subs.createTitle')}
      footer={
        <div class="flex items-center justify-between gap-3 w-full">
          <span class="text-xs text-faint">{t('subs.pipelineHint')}</span>
          <div class="flex items-center gap-2">
            <Button size="sm" variant="ghost" onClick={props.onClose}>{t('common.cancel')}</Button>
            <Button size="sm" variant="primary" loading={props.saving} disabled={touched() && problems().length > 0} onClick={submit}>
              {t('common.save')}
            </Button>
          </div>
        </div>
      }
    >
      <div class="space-y-4">
        <Show when={touched() && problems().length}>
          <Alert variant="danger">
            <ul class="list-disc pl-4 space-y-0.5">
              {problems().map(msg => <li>{msg}</li>)}
            </ul>
          </Alert>
        </Show>

        <div class="grid gap-4 sm:grid-cols-2">
          <Field label={t('common.id')}>
            <Input
              value={draft().id}
              disabled={editing()}
              placeholder="all-in-one"
              ariaLabel={t('common.id')}
              onInput={v => setDraft({ ...draft(), id: v })}
            />
          </Field>
          <Field label={t('common.name')}>
            <Input
              value={draft().name}
              placeholder={draft().id || t('common.name')}
              ariaLabel={t('common.name')}
              onInput={v => setDraft({ ...draft(), name: v })}
            />
          </Field>
          <Field label={t('subs.token')} hint={t('subs.tokenHint')}>
            <Input
              value={draft().token}
              placeholder="my_secret_999"
              ariaLabel={t('subs.token')}
              onInput={v => setDraft({ ...draft(), token: v })}
            />
          </Field>
          <Field label={t('subs.status')}>
            <div class="flex items-center gap-3 h-9">
              <Toggle
                ariaLabel={t('subs.status')}
                checked={draft().enabled}
                onChange={v => setDraft({ ...draft(), enabled: v })}
              />
              <span class="text-sm text-muted">{draft().enabled ? t('common.enabled') : t('common.disabled')}</span>
            </div>
          </Field>
        </div>

        <Show when={overwrites()}>
          <Alert variant="warning">{t('subs.idConflict')}</Alert>
        </Show>

        <Field label={t('subs.sources')} hint={t('subs.sourcesHint')}>
          <Textarea
            rows={5}
            value={sourcesText()}
            placeholder={'https://airport.example.com/api/v1/client/subscribe?token=xxx\nvless://uuid@host:443?security=reality#self-hosted'}
            ariaLabel={t('subs.sources')}
            onInput={setSourcesText}
          />
          <div class="pt-1 text-[11px] text-faint tabular-nums">{t('subs.sourcesCount', { count: sources().length })}</div>
        </Field>

        <div class="rounded-control border border-glass-border bg-surface-inset/40 p-3">
          <PipelineEditor steps={pipeline()} onChange={setPipeline} />
        </div>
      </div>
    </Modal>
  )
}
