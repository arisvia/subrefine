import { type Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js'
import { useI18n } from '@/i18n'
import { Alert, Button, Checkbox, Field, Input, Modal, Select, Textarea, Toggle } from '@/components/ui'
import { formatBytes, formatExpiry, maskKey } from '@/lib/format'
import { copyToClipboard } from '@/lib/clipboard'
import { toast } from '@/lib/toast'
import { deliveryUrl } from '@/lib/sublinks'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import { useKeysStore } from '@/stores/keys'
import { PipelineEditor } from './PipelineEditor'
import type { PipelineStep, SubKeyRef, Subscription } from '@/types/domain'

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
  const keysPool = useKeysStore()
  const [draft, setDraft] = createSignal<Subscription>(emptyDraft())
  const [sourcesText, setSourcesText] = createSignal('')
  const [pipeline, setPipeline] = createSignal<PipelineStep[]>([])
  const [touched, setTouched] = createSignal(false)
  // Attached global keys (references only); credentials live on the Keys page.
  const [refs, setRefs] = createSignal<SubKeyRef[]>([])
  const [expandedRef, setExpandedRef] = createSignal<string | null>(null)

  createEffect(() => {
    if (!props.open) return
    const src = props.initial ?? emptyDraft()
    setDraft({ ...src })
    setSourcesText((src.sources ?? []).join('\n'))
    setPipeline((src.pipeline ?? []).map(s => ({ ...s })))
    setRefs((src.keys ?? []).map(r => ({ id: r.id, disabled: [...(r.disabled ?? [])] })))
    setExpandedRef(null)
    setTouched(false)
    void keysPool.load()
  })

  // Expanding a key's node list needs the preview; the store dedupes and
  // single-flights it, so calling on every expand is cheap.
  createEffect(() => {
    if (expandedRef() !== null && props.initial) void subs.runPreview(props.initial.id)
  })

  const attachable = createMemo(() =>
    keysPool.keys().filter(k => !refs().some(r => r.id === k.id)))

  const attachKey = (id: string) => {
    if (!id) return
    setRefs(prev => [...prev, { id, disabled: [] }])
  }

  const detachKey = (id: string) => {
    setRefs(prev => prev.filter(r => r.id !== id))
    if (expandedRef() === id) setExpandedRef(null)
  }

  const toggleNode = (refId: string, identity: string, on: boolean) =>
    setRefs(prev => prev.map(r => {
      if (r.id !== refId) return r
      const cur = r.disabled ?? []
      return { ...r, disabled: on ? cur.filter(d => d !== identity) : [...cur, identity] }
    }))

  const copyRefLink = async (refId: string) => {
    const id = props.initial?.id ?? draft().id.trim()
    const token = keysPool.byId(refId)?.token
    if (!id || !token) return
    if (await copyToClipboard(deliveryUrl(id, token, 'uri'))) {
      toast.success(t('subs.linkCopied', { target: keysPool.byId(refId)!.name }))
    }
  }

  const editing = () => !!props.initial
  const sources = createMemo(() => sourcesText().split('\n').map(s => s.trim()).filter(Boolean))

  const problems = createMemo(() => {
    const out: string[] = []
    const d = draft()
    if (!d.id.trim()) out.push(t('subs.idRequired'))
    else if (!ID_PATTERN.test(d.id.trim())) out.push(t('subs.idPattern'))
    if (!refs().length) out.push(t('subs.keysRequired'))
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
      // References only; the server re-syncs the mirrored primary token.
      keys: refs().map(r => ({ id: r.id, disabled: (r.disabled ?? []).filter(Boolean) })),
      token: keysPool.byId(refs()[0]?.id ?? '')?.token ?? draft().token.trim(),
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

        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
            <div class="w-52 shrink-0">
              <Select
                size="sm"
                value=""
                options={[
                  { value: '', label: attachable().length ? t('subs.attachKey') : t('subs.noAttachable') },
                  ...attachable().map(k => ({ value: k.id, label: k.name })),
                ]}
                onChange={attachKey}
                ariaLabel={t('subs.attachKey')}
                disabled={!attachable().length}
              />
            </div>
          </div>
          <Show when={!refs().length}>
            <div class="text-xs text-warning">{t('subs.noKeysYet')}</div>
          </Show>
          <div class="space-y-2.5">
            <For each={refs()}>
              {r => {
                const key = () => keysPool.byId(r.id)
                const stale = createMemo(() => {
                  const known = new Set(subs.preview(props.initial?.id ?? '').ids)
                  return (r.disabled ?? []).filter(d => !known.has(d)).length
                })
                return (
                  <div class="rounded-control border border-glass-border/60 bg-surface/40 p-2.5 space-y-2">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="text-sm font-medium text-foreground">{key()?.name ?? r.id}</span>
                      <Show when={key()}>
                        <span class="text-xs font-mono text-faint">{maskKey(key()!.token)}</span>
                      </Show>
                      <span class="ml-auto flex items-center gap-3 text-xs">
                        <Show when={editing() && key()}>
                          <button type="button" class="text-accent hover:underline" onClick={() => void copyRefLink(r.id)}>
                            {t('subs.copyKeyLink')}
                          </button>
                        </Show>
                        <button
                          type="button"
                          class="text-muted hover:text-foreground"
                          onClick={() => setExpandedRef(expandedRef() === r.id ? null : r.id)}
                        >
                          {t('subs.keyNodes')}
                          <Show when={(r.disabled ?? []).length}>
                            <span class="text-warning"> · {t('subs.keyOffCount', { count: (r.disabled ?? []).length })}</span>
                          </Show>
                        </button>
                        <Show when={refs().length > 1}>
                          <Button size="sm" variant="ghost" onClick={() => detachKey(r.id)}>{t('subs.detachKey')}</Button>
                        </Show>
                      </span>
                    </div>
                    <Show when={expandedRef() === r.id}>
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
                                      checked={item.id !== '' && !(r.disabled ?? []).includes(item.id)}
                                      onChange={v => toggleNode(r.id, item.id, v)}
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

        <Show when={editing() && (subs.preview(props.initial!.id).sources?.length ?? 0) > 0}>
          <div class="rounded-control border border-glass-border/60 bg-surface-inset/30 p-2.5 space-y-1">
            <div class="text-xs text-muted">{t('subs.sourceInfo')}</div>
            <For each={subs.preview(props.initial!.id).sources}>
              {s => {
                const used = (s.upload ?? 0) + (s.download ?? 0)
                return (
                  <div class="flex items-baseline justify-between gap-2 text-[11px] font-mono">
                    <span class="truncate text-faint" title={s.source}>{s.source.replace(/^https?:\/\//, '').slice(0, 42)}</span>
                    <span class="shrink-0 text-muted">
                      <Show when={s.expire}>{t('subs.expires')} {formatExpiry(s.expire)}</Show>
                      {' '}
                      <Show when={(s.total ?? 0) > 0} fallback={
                        <Show when={used > 0}>{t('subs.trafficUsed')} {formatBytes(used)}</Show>
                      }>
                        {t('subs.trafficLeft')} {formatBytes((s.total ?? 0) - used)} / {formatBytes(s.total)}
                      </Show>
                    </span>
                  </div>
                )
              }}
            </For>
          </div>
        </Show>

        <div class="rounded-control border border-glass-border bg-surface-inset/40 p-3">
          <PipelineEditor steps={pipeline()} onChange={setPipeline} />
        </div>
      </div>
    </Modal>
  )
}
