import { type Component, For, Show, createEffect, createMemo, createResource, createSignal } from 'solid-js'
import { useI18n } from '@/i18n'
import { Alert, Button, Checkbox, Field, Input, Modal, Select, Textarea, Toggle } from '@/components/ui'
import { api } from '@/lib/api'
import { copyToClipboard } from '@/lib/clipboard'
import { toast } from '@/lib/toast'
import { deliveryUrl } from '@/lib/sublinks'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { PipelineEditor } from './PipelineEditor'
import type { PipelineStep, SubKey, Subscription, TemplateModes } from '@/types/domain'

// Mirrors storage.ValidateID (`^[A-Za-z0-9_-]{1,64}$`) — keep the two in sync.
const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/

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
  const subs = useSubscriptionsStore()
  const [draft, setDraft] = createSignal<Subscription>(emptyDraft())
  const [sourcesText, setSourcesText] = createSignal('')
  const [pipeline, setPipeline] = createSignal<PipelineStep[]>([])
  const [touched, setTouched] = createSignal(false)
  // Delivery keys; a legacy single-token subscription seeds one row so the
  // editor always shows at least the primary link.
  const [keys, setKeys] = createSignal<SubKey[]>([{ id: '', name: 'Default', token: '', disabled: [] }])
  const [expandedKey, setExpandedKey] = createSignal<number | null>(null)

  createEffect(() => {
    if (!props.open) return
    const src = props.initial ?? emptyDraft()
    setDraft({ ...src })
    setSourcesText((src.sources ?? []).join('\n'))
    setPipeline((src.pipeline ?? []).map(s => ({ ...s })))
    const seeded = (src.keys ?? []).map(k => ({ ...k, disabled: [...(k.disabled ?? [])] }))
    setKeys(seeded.length ? seeded : [{ id: '', name: 'Default', token: src.token ?? '', disabled: [] }])
    setExpandedKey(null)
    setTouched(false)
  })

  // Expanding a key's node list needs the preview; the store dedupes and
  // single-flights it, so calling on every expand is cheap.
  createEffect(() => {
    const i = expandedKey()
    if (i !== null && props.initial) void subs.runPreview(props.initial.id)
  })

  const patchKey = (i: number, patch: Partial<SubKey>) =>
    setKeys(prev => prev.map((k, idx) => (idx === i ? { ...k, ...patch } : k)))

  const addKey = () =>
    setKeys(prev => [...prev, { id: '', name: '', token: '', disabled: [] }])

  const removeKey = (i: number) => {
    setKeys(prev => prev.filter((_, idx) => idx !== i))
    setExpandedKey(null)
  }

  const toggleNode = (i: number, identity: string, on: boolean) =>
    setKeys(prev => prev.map((k, idx) => {
      if (idx !== i) return k
      const cur = k.disabled ?? []
      return { ...k, disabled: on ? cur.filter(d => d !== identity) : [...cur, identity] }
    }))

  const copyKeyLink = async (i: number) => {
    const k = keys()[i]
    const id = props.initial?.id ?? draft().id.trim()
    if (!id || !k?.token.trim()) return
    if (await copyToClipboard(deliveryUrl(id, k.token.trim(), 'uri'))) {
      toast.success(t('subs.linkCopied', { target: k.name.trim() || t('subs.keyToken') }))
    }
  }

  const editing = () => !!props.initial
  const sources = createMemo(() => sourcesText().split('\n').map(s => s.trim()).filter(Boolean))

  // Full-config modes come from the server's template directory; a console
  // without them (feature off / fetch failure) still offers the fragment.
  const [modes] = createResource(() => props.open, open =>
    open
      ? api<TemplateModes>('/api/admin/templates').catch(() => ({ singbox: [], clash: [] } as TemplateModes))
      : Promise.resolve({ singbox: [], clash: [] } as TemplateModes))
  const modeOptions = (list: string[] | undefined) => [
    { value: '', label: t('subs.modeFragment') },
    ...(list ?? []).map(m => ({ value: m, label: m })),
  ]
  const sbOptions = createMemo(() => modeOptions(modes()?.singbox))
  const clOptions = createMemo(() => modeOptions(modes()?.clash))
  const noTemplates = createMemo(() => !(modes()?.singbox?.length ?? 0) && !(modes()?.clash?.length ?? 0))

  const problems = createMemo(() => {
    const out: string[] = []
    const d = draft()
    if (!d.id.trim()) out.push(t('subs.idRequired'))
    else if (!ID_PATTERN.test(d.id.trim())) out.push(t('subs.idPattern'))
    if (!keys().length) out.push(t('subs.keysRequired'))
    const toks = keys().map(k => k.token.trim()).filter(Boolean)
    if (new Set(toks).size !== toks.length) out.push(t('subs.keyDup'))
    if (!sources().length) out.push(t('subs.sourcesRequired'))
    return out
  })

  const overwrites = createMemo(() =>
    !editing() && !!draft().id.trim() && props.knownIds.includes(draft().id.trim()))

  const submit = () => {
    setTouched(true)
    if (problems().length) return
    const payload = keys().map(k => ({
      ...k,
      name: k.name.trim(),
      token: k.token.trim(),
      disabled: (k.disabled ?? []).filter(Boolean),
    }))
    props.onSubmit({
      ...draft(),
      id: draft().id.trim(),
      name: draft().name.trim() || draft().id.trim(),
      // The server syncs the primary token to keys[0] (and generates any
      // empty tokens); mirror that here so the list updates instantly.
      keys: payload,
      token: payload[0]?.token ?? draft().token.trim(),
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
          <Field label={t('subs.singboxMode')} hint={noTemplates() ? t('subs.noTemplates') : t('subs.singboxModeHint')}>
            <Select
              value={draft().singbox_mode ?? ''}
              options={sbOptions()}
              ariaLabel={t('subs.singboxMode')}
              onChange={v => setDraft({ ...draft(), singbox_mode: v })}
            />
          </Field>
          <Field label={t('subs.clashMode')} hint={noTemplates() ? t('subs.noTemplates') : t('subs.clashModeHint')}>
            <Select
              value={draft().clash_mode ?? ''}
              options={clOptions()}
              ariaLabel={t('subs.clashMode')}
              onChange={v => setDraft({ ...draft(), clash_mode: v })}
            />
          </Field>
        </div>

        <Show when={overwrites()}>
          <Alert variant="warning">{t('subs.idConflict')}</Alert>
        </Show>

        <div class="rounded-control border border-glass-border bg-surface-inset/40 p-3 space-y-3">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-medium text-foreground">{t('subs.keys')}</div>
              <div class="text-xs text-faint">{t('subs.keysHint')}</div>
            </div>
            <Button size="sm" variant="ghost" onClick={addKey}>{t('subs.addKey')}</Button>
          </div>
          <div class="space-y-2.5">
            <For each={keys()}>
              {(k, i) => {
                const stale = createMemo(() => {
                  const known = new Set(subs.preview(props.initial?.id ?? '').ids)
                  return (k.disabled ?? []).filter(d => !known.has(d)).length
                })
                return (
                  <div class="rounded-control border border-glass-border/60 bg-surface/40 p-2.5 space-y-2">
                    <div class="flex flex-wrap items-end gap-2">
                      <div class="min-w-28 flex-1">
                        <div class="pb-1 text-xs text-muted">{t('subs.keyName')}</div>
                        <Input
                          size="sm"
                          value={k.name}
                          placeholder={i() === 0 ? t('subs.keyPrimaryName') : t('subs.keyNamePlaceholder', { n: i() + 1 })}
                          ariaLabel={t('subs.keyName')}
                          onInput={v => patchKey(i(), { name: v })}
                        />
                      </div>
                      <div class="min-w-40 flex-[2]">
                        <div class="pb-1 text-xs text-muted">{t('subs.keyToken')}</div>
                        <Input
                          size="sm"
                          class="font-mono"
                          value={k.token}
                          placeholder={t('subs.keyGenPlaceholder')}
                          ariaLabel={t('subs.keyToken')}
                          onInput={v => patchKey(i(), { token: v })}
                        />
                      </div>
                      <Show when={keys().length > 1}>
                        <Button size="sm" variant="ghost" onClick={() => removeKey(i())}>{t('subs.removeKey')}</Button>
                      </Show>
                    </div>
                    <div class="flex flex-wrap items-center gap-3 text-xs">
                      <Show when={editing() && k.token.trim()}>
                        <button type="button" class="text-accent hover:underline" onClick={() => void copyKeyLink(i())}>
                          {t('subs.copyKeyLink')}
                        </button>
                      </Show>
                      <button
                        type="button"
                        class="text-muted hover:text-foreground"
                        onClick={() => setExpandedKey(expandedKey() === i() ? null : i())}
                      >
                        {t('subs.keyNodes')}
                        <Show when={(k.disabled ?? []).length}>
                          <span class="text-warning"> · {t('subs.keyOffCount', { count: (k.disabled ?? []).length })}</span>
                        </Show>
                      </button>
                    </div>
                    <Show when={expandedKey() === i()}>
                      <Show when={editing()} fallback={
                        <div class="border-t border-glass-border/50 pt-2 text-xs text-faint">{t('subs.keyNodesNeedSave')}</div>
                      }>
                        {(() => {
                          const p = subs.preview(props.initial!.id)
                          return (
                            <div class="space-y-1.5 border-t border-glass-border/50 pt-2">
                              <div class="text-[11px] text-faint">{t('subs.keyNodesHint')}</div>
                              <Show when={p.loading}>
                                <div class="text-xs text-faint">{t('preview.loading')}</div>
                              </Show>
                              <Show when={!p.loading && p.error}>
                                <div class="text-xs text-danger">{p.error}</div>
                              </Show>
                              <Show when={!p.loading && !p.error && !p.nodes.length}>
                                <div class="text-xs text-faint">{t('preview.emptyAll')}</div>
                              </Show>
                              <div class="max-h-56 space-y-1.5 overflow-y-auto pr-1">
                                <For each={p.nodes.map((n, idx) => ({ tag: n.tag, type: n.type, id: p.ids[idx] ?? '' }))}>
                                  {item => (
                                    <Checkbox
                                      checked={item.id !== '' && !(k.disabled ?? []).includes(item.id)}
                                      onChange={v => toggleNode(i(), item.id, v)}
                                      label={
                                        <span class="text-xs">
                                          <span class="text-foreground">{item.tag}</span>{' '}
                                          <span class="text-faint">({item.type})</span>
                                        </span>
                                      }
                                    />
                                  )}
                                </For>
                              </div>
                              <Show when={stale() > 0}>
                                <div class="text-[11px] text-faint">{t('subs.keyStale', { count: stale() })}</div>
                              </Show>
                            </div>
                          )
                        })()}
                      </Show>
                    </Show>
                  </div>
                )
              }}
            </For>
          </div>
        </div>

        <Field label={t('subs.sources')} hint={t('subs.sourcesHint')}>
          <Textarea
            rows={5}
            class="font-mono text-xs leading-relaxed resize-y"
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
