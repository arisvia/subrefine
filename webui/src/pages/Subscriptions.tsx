import { type Component, For, Show, createMemo, createSignal, onMount } from 'solid-js'
import { useSearchParams } from '@solidjs/router'
import { useI18n } from '@/i18n'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import {
  Badge, Button, Card, Empty, IconButton, Input, PageHeader, Skeleton,
  IconEdit, IconEye, IconSparkles, IconSearch, IconTrash, IconZap,
} from '@/components/ui'
import { confirm } from '@/lib/confirm'
import { toast } from '@/lib/toast'
import { copyToClipboard } from '@/lib/clipboard'
import { timeAgo, timeUnitsFor } from '@/lib/format'
import { DELIVERY_OPTIONS, deliveryUrl } from '@/lib/sublinks'
import { SubscriptionEditor } from '@/components/subscriptions/SubscriptionEditor'
import { NodePreviewModal } from '@/components/subscriptions/NodePreviewModal'
import type { PipelineStep, Subscription } from '@/types/domain'

const STEP_LABEL_KEYS = {
  deduplicate: 'pipeline.type.deduplicate',
  filter: 'pipeline.type.filter',
  rename: 'pipeline.type.rename',
  decorate: 'pipeline.type.decorate',
  sort: 'pipeline.type.sort',
} as const satisfies Record<PipelineStep['type'], string>

const Subscriptions: Component = () => {
  const { t } = useI18n()
  const store = useSubscriptionsStore()
  const [params, setParams] = useSearchParams<{ new?: string; edit?: string }>()

  const [editorOpen, setEditorOpen] = createSignal(false)
  const [editing, setEditing] = createSignal<Subscription | null>(null)
  const [saving, setSaving] = createSignal(false)
  const [query, setQuery] = createSignal('')
  const [previewId, setPreviewId] = createSignal('')

  onMount(() => {
    void store.load()
    if (params.new !== undefined) openCreate()
    else if (params.edit) openEdit(params.edit)
  })

  const closeOverlays = () => {
    setEditorOpen(false)
    setEditing(null)
    setPreviewId('')
    if (params.new !== undefined || params.edit) setParams({})
  }

  const openCreate = () => {
    setEditing(null)
    setEditorOpen(true)
  }

  const openEdit = (id: string) => {
    const sub = store.byId(id)
    if (!sub) return
    setEditing(sub)
    setEditorOpen(true)
  }

  const filtered = createMemo(() => {
    const q = query().trim().toLowerCase()
    const list = store.subs()
    if (!q) return list
    return list.filter(s =>
      `${s.id} ${s.name} ${(s.sources ?? []).join(' ')}`.toLowerCase().includes(q))
  })

  const chain = (pipeline: PipelineStep[] | undefined) => {
    if (!pipeline?.length) return t('pipeline.empty')
    return pipeline.map(step => t(STEP_LABEL_KEYS[step.type])).join(' → ')
  }

  const submit = async (sub: Subscription) => {
    setSaving(true)
    const ok = await store.save(sub)
    setSaving(false)
    if (!ok) return
    toast.success(t('subs.saved'))
    closeOverlays()
    // Warm the preview so the card shows real node counts immediately.
    void store.runPreview(sub.id, true)
  }

  const remove = async (sub: Subscription) => {
    const ok = await confirm({
      title: t('common.confirmDelete'),
      message: t('subs.deleteConfirm', { name: sub.name || sub.id }),
      variant: 'danger',
    })
    if (!ok) return
    if (await store.remove(sub.id)) toast.success(t('subs.deleted'))
  }

  const toggleEnabled = async (sub: Subscription) => {
    if (await store.save({ ...sub, enabled: !sub.enabled })) {
      toast.success(sub.enabled ? t('common.disabled') : t('common.enabled'))
    }
  }

  const copyLink = async (sub: Subscription, target: (typeof DELIVERY_OPTIONS)[number]['target']) => {
    if (await copyToClipboard(deliveryUrl(sub.id, sub.token, target))) {
      toast.success(t('subs.linkCopied', { target }))
    }
  }

  const previewState = createMemo(() => (previewId() ? store.preview(previewId()) : { loading: false, error: '', nodes: [] }))

  return (
    <>
      <PageHeader
        title={t('subs.title')}
        subtitle={t('subs.subtitle')}
        badge={<Badge tone="indigo">{store.subs().length}</Badge>}
        actions={
          <div class="flex items-center gap-2">
            <div class="relative">
              <Input
                size="sm"
                class="pl-8 w-52"
                ariaLabel={t('subs.searchPlaceholder')}
                placeholder={t('subs.searchPlaceholder')}
                value={query()}
                onInput={setQuery}
              />
              <span class="absolute left-2.5 top-1/2 -translate-y-1/2 text-faint pointer-events-none">
                <IconSearch size={14} />
              </span>
            </div>
            <Button size="sm" variant="primary" onClick={openCreate}>
              <IconSparkles size={14} />
              {t('subs.create')}
            </Button>
          </div>
        }
      />

      <Show when={store.loading() && !store.subs().length} fallback={
        <Show when={!store.error()} fallback={
          <Card class="p-6"><Empty title={t('common.loadFailed')} description={store.error()} action={
            <Button size="sm" variant="secondary" onClick={() => store.load(true)}>{t('common.retry')}</Button>
          } /></Card>
        }>
          <Show when={filtered().length} fallback={
            <Card class="p-8">
              <Empty
                title={query() ? t('preview.empty') : t('subs.emptyTitle')}
                description={query() ? undefined : t('subs.emptyDesc')}
                action={query() ? undefined : (
                  <Button size="sm" variant="primary" onClick={openCreate}><IconSparkles size={14} />{t('subs.create')}</Button>
                )}
              />
            </Card>
          }>
            <div class="space-y-3">
              <For each={filtered()}>
                {sub => {
                  const state = createMemo(() => store.preview(sub.id))
                  return (
                    <Card class="p-4 space-y-3" hover>
                      <div class="flex items-start justify-between gap-3">
                        <div class="min-w-0">
                          <div class="flex items-center gap-2 flex-wrap">
                            <span class="text-sm font-semibold text-foreground truncate">{sub.name || sub.id}</span>
                            <Badge tone="blue">{sub.id}</Badge>
                            <Badge tone={sub.enabled ? 'green' : 'gray'}>
                              {sub.enabled ? t('common.enabled') : t('common.disabled')}
                            </Badge>
                          </div>
                          <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-faint font-mono">
                            <span>{t('subs.sourcesCount', { count: (sub.sources ?? []).length })}</span>
                            <span>key: {sub.token}</span>
                            <Show when={sub.updated_at}>
                              <span>{t('subs.lastUpdated', {
                                time: timeAgo(new Date(sub.updated_at * 1000).toISOString(), timeUnitsFor(t)),
                              })}</span>
                            </Show>
                          </div>
                          <div class="mt-1.5 text-[11px] text-muted truncate">{chain(sub.pipeline)}</div>
                        </div>

                        <div class="flex items-center gap-1 shrink-0">
                          <IconButton size="sm" aria-label={t('subs.nodesPreview')} title={t('subs.nodesPreview')}
                            onClick={() => { setPreviewId(sub.id); void store.runPreview(sub.id) }}>
                            <IconEye size={14} />
                          </IconButton>
                          <IconButton size="sm" aria-label={t('common.edit')} title={t('common.edit')} onClick={() => openEdit(sub.id)}>
                            <IconEdit size={14} />
                          </IconButton>
                          <Button size="sm" variant="ghost" onClick={() => toggleEnabled(sub)}>
                            {sub.enabled ? t('common.disable') : t('common.enable')}
                          </Button>
                          <IconButton size="sm" class="hover:text-danger" aria-label={t('common.delete')}
                            title={t('common.delete')} onClick={() => remove(sub)}>
                            <IconTrash size={14} />
                          </IconButton>
                        </div>
                      </div>

                      <div class="flex flex-wrap items-center gap-1.5">
                        <For each={DELIVERY_OPTIONS}>
                          {opt => (
                            <>
                              <a class="pill" href={deliveryUrl(sub.id, sub.token, opt.target)} target="_blank" rel="noreferrer" title={opt.hint}>
                                {opt.label}
                              </a>
                              <button type="button" class="pill" onClick={() => copyLink(sub, opt.target)}>
                                {t('common.copy')}
                              </button>
                            </>
                          )}
                        </For>
                        <Show when={state().nodes.length}>
                          <span class="ml-auto inline-flex items-center gap-1 text-[11px] text-faint tabular-nums">
                            <IconZap size={11} class="text-accent" />
                            {t('subs.nodeCount', { count: state().nodes.length })}
                          </span>
                        </Show>
                      </div>
                    </Card>
                  )
                }}
              </For>
            </div>
          </Show>
        </Show>
      }>
        <div class="space-y-3" role="status" aria-busy="true" aria-label={t('common.loading')}>
          <For each={[0, 1, 2]}>{() => <Skeleton class="h-28 w-full" />}</For>
        </div>
      </Show>

      <SubscriptionEditor
        open={editorOpen()}
        initial={editing()}
        knownIds={store.subs().map(s => s.id)}
        saving={saving()}
        onClose={closeOverlays}
        onSubmit={submit}
      />

      <Show when={previewId()}>
        <NodePreviewModal
          open={!!previewId()}
          id={previewId()}
          loading={previewState().loading}
          error={previewState().error}
          nodes={previewState().nodes}
          onClose={() => setPreviewId('')}
          onRefresh={() => void store.runPreview(previewId(), true)}
        />
      </Show>
    </>
  )
}

export default Subscriptions
