import { type Component, For, Show, createMemo, createSignal } from 'solid-js'
import { useI18n } from '@/i18n'
import { Badge, Button, Empty, IconButton, Input, Modal, Skeleton, IconClipboard, IconRotateCcw } from '@/components/ui'
import { toast } from '@/lib/toast'
import { copyToClipboard } from '@/lib/clipboard'
import { PROTOCOL_LABELS, PROTOCOL_TONES, type ProxyNode } from '@/types/domain'

export interface NodePreviewModalProps {
  open: boolean
  id: string
  loading: boolean
  error: string
  nodes: ProxyNode[]
  onClose: () => void
  onRefresh: () => void
}

/** Live view of what a subscription actually serves: sources + pipeline output. */
export const NodePreviewModal: Component<NodePreviewModalProps> = props => {
  const { t } = useI18n()
  const [query, setQuery] = createSignal('')

  const visible = createMemo(() => {
    const q = query().trim().toLowerCase()
    if (!q) return props.nodes
    return props.nodes.filter(n =>
      `${n.tag} ${n.type} ${n.server} ${n.port}`.toLowerCase().includes(q))
  })

  const mix = createMemo(() => {
    const counts = new Map<string, number>()
    for (const n of props.nodes) counts.set(n.type, (counts.get(n.type) ?? 0) + 1)
    return [...counts.entries()].sort((a, b) => b[1] - a[1])
  })

  return (
    <Modal
      open={props.open}
      onClose={props.onClose}
      size="2xl"
      title={t('preview.title', { id: props.id })}
      footer={
        <div class="flex items-center justify-between gap-3 w-full">
          <span class="text-xs text-faint tabular-nums">{t('preview.total', { count: props.nodes.length })}</span>
          <div class="flex items-center gap-2">
            <Button size="sm" variant="secondary" onClick={props.onRefresh}>
              <IconRotateCcw size={13} />
              {t('subs.refreshCache')}
            </Button>
            <Button size="sm" onClick={props.onClose}>{t('common.close')}</Button>
          </div>
        </div>
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <For each={mix()}>
            {([proto, count]) => (
              <Badge tone={(PROTOCOL_TONES[proto as keyof typeof PROTOCOL_TONES] ?? 'gray')}>
                {PROTOCOL_LABELS[proto as keyof typeof PROTOCOL_LABELS] ?? proto} · {count}
              </Badge>
            )}
          </For>
          <div class="ml-auto w-56 max-w-full">
            <Input size="sm" ariaLabel={t('preview.searchPlaceholder')} placeholder={t('preview.searchPlaceholder')}
              value={query()} onInput={setQuery} />
          </div>
        </div>

        <Show when={props.loading}>
          <div class="space-y-2" role="status" aria-busy="true" aria-label={t('preview.loading')}>
            <For each={[0, 1, 2, 3]}>{() => <Skeleton class="h-12 w-full" />}</For>
          </div>
        </Show>

        <Show when={!props.loading && props.error}>
          <Empty title={t('preview.failed', { reason: props.error })} />
        </Show>

        <Show when={!props.loading && !props.error}>
          <Show when={visible().length} fallback={
            <Empty title={props.nodes.length ? t('preview.empty') : t('preview.emptyAll')} />
          }>
            <div class="space-y-1.5">
              <For each={visible()}>
                {node => (
                  <div class="flex items-center gap-3 rounded-control border border-glass-border bg-surface-inset/50 px-3 py-2">
                    <div class="min-w-0 flex-1">
                      <div class="truncate text-sm font-medium text-foreground">{node.tag}</div>
                      <div class="truncate text-[11px] text-faint font-mono">
                        {node.server}:{node.port}
                        <Show when={node.path}> · {node.path}</Show>
                        <Show when={node.network && node.network !== 'tcp'}> · {node.network}</Show>
                      </div>
                    </div>
                    <div class="flex items-center gap-1.5 shrink-0">
                      <Badge tone={PROTOCOL_TONES[node.type] ?? 'gray'}>{node.type}</Badge>
                      <Show when={node.security && node.security !== 'none'}>
                        <Badge tone="blue">{node.security}</Badge>
                      </Show>
                      <Show when={node.flow}>
                        <Badge tone="violet">{node.flow}</Badge>
                      </Show>
                      <Show when={node.detour}>
                        <Badge tone="pink">↗ {node.detour}</Badge>
                      </Show>
                      <IconButton size="sm" aria-label={t('common.copy')} title={node.tag}
                        onClick={async () => {
                          if (await copyToClipboard(JSON.stringify(node, null, 2))) toast.success(t('common.copied'))
                        }}>
                        <IconClipboard size={13} />
                      </IconButton>
                    </div>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </Show>
      </div>
    </Modal>
  )
}
