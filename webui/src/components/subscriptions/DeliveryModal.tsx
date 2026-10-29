import { type Component, Show, createEffect, createMemo, createSignal } from 'solid-js'
import { useI18n } from '@/i18n'
import { Button, Modal, SegmentedControl, IconClipboard } from '@/components/ui'
import { DELIVERY_OPTIONS, deliveryUrl, type DeliveryTarget } from '@/lib/sublinks'
import { useTemplatesStore } from '@/stores/templates'
import { useKeysStore } from '@/stores/keys'
import { copyToClipboard } from '@/lib/clipboard'
import { toast } from '@/lib/toast'
import { maskKey } from '@/lib/format'
import type { Subscription } from '@/types/domain'

export interface DeliveryModalProps {
  /** null closes the dialog; the opener preselects a format. */
  sub: Subscription | null
  initialTarget: DeliveryTarget
  onClose: () => void
}

/**
 * Sub-Store style output dialog: pick format, delivery key and (for
 * sing-box/clash) fragment-vs-template mode, then copy the exact URL.
 * Nothing is stored on the subscription - the choice lives in the link.
 */
export const DeliveryModal: Component<DeliveryModalProps> = props => {
  const { t } = useI18n()
  const templates = useTemplatesStore()
  const keysPool = useKeysStore()

  const [target, setTarget] = createSignal<DeliveryTarget>('uri')
  const [keyId, setKeyId] = createSignal('')
  const [mode, setMode] = createSignal('')

  const modeList = createMemo<string[]>(() => {
    if (target() === 'sing-box') return templates.modes().singbox ?? []
    if (target() === 'clash') return templates.modes().clash ?? []
    return []
  })

  // Re-seed the choices whenever a subscription is opened.
  createEffect(() => {
    const sub = props.sub
    if (!sub) return
    setTarget(props.initialTarget)
    setKeyId(sub.keys?.[0]?.id ?? '')
    setMode('')
    void templates.load()
  })

  // A mode chosen for one family may not exist in the other; snap back to
  // the fragment instead of producing a link the server would downgrade.
  createEffect(() => {
    const list = modeList()
    if (mode() && !list.includes(mode())) setMode('')
  })

  const token = () => keysPool.byId(keyId())?.token ?? props.sub?.token ?? ''
  const url = createMemo(() =>
    props.sub ? deliveryUrl(props.sub.id, token(), target(), mode()) : '')

  const copy = async () => {
    if (await copyToClipboard(url())) {
      toast.success(t('delivery.copied'))
    }
  }

  return (
    <Modal
      open={props.sub !== null}
      onClose={props.onClose}
      size="md"
      title={t('delivery.title')}
      header={
        <div class="min-w-0">
          <div class="text-base font-semibold text-foreground">{t('delivery.title')}</div>
          <Show when={props.sub}>
            <div class="truncate text-xs text-muted">{props.sub!.name || props.sub!.id}</div>
          </Show>
        </div>
      }
      footer={
        <div class="flex w-full items-center justify-end gap-2">
          <Button variant="ghost" onClick={props.onClose}>{t('common.close')}</Button>
          <Button variant="primary" onClick={() => void copy()}>
            <IconClipboard size={14} />{t('delivery.copy')}
          </Button>
        </div>
      }
    >
      <Show when={props.sub}>
        <div class="space-y-4">
          <div>
            <div class="pb-1.5 text-xs text-muted">{t('delivery.format')}</div>
            <SegmentedControl
              class="w-full"
              value={target()}
              onChange={setTarget}
              options={DELIVERY_OPTIONS.map(o => ({ value: o.target, label: o.label }))}
            />
          </div>

          <Show when={(props.sub!.keys?.length ?? 0) > 1}>
            <div>
              <div class="pb-1.5 text-xs text-muted">{t('delivery.key')}</div>
              <SegmentedControl
                class="w-full"
                value={keyId()}
                onChange={setKeyId}
                options={(props.sub!.keys ?? []).map(r => ({
                  value: r.id,
                  label: keysPool.byId(r.id)?.name ?? r.id,
                }))}
              />
            </div>
          </Show>

          <Show when={modeList().length}>
            <div>
              <div class="pb-1.5 text-xs text-muted">{t('delivery.mode')}</div>
              <SegmentedControl
                class="w-full"
                value={mode()}
                onChange={setMode}
                options={[
                  { value: '', label: t('delivery.modeFragment') },
                  ...modeList().map(m => ({ value: m, label: m.toUpperCase() })),
                ]}
              />
              <div class="pt-1.5 text-[11px] text-faint">
                {mode() === '' ? t('delivery.modeFragmentHint') : t('delivery.modeFullHint')}
              </div>
            </div>
          </Show>

          <div>
            <div class="pb-1.5 text-xs text-muted">{t('delivery.url')}</div>
            <input
              readonly
              value={url()}
              aria-label={t('delivery.url')}
              onFocus={e => e.currentTarget.select()}
              class="ui-control border w-full rounded-control bg-surface-inset/60 px-3 py-2 font-mono text-xs text-foreground select-all"
            />
            <div class="pt-1.5 text-[11px] text-faint font-mono">
              token: {maskKey(token())}
            </div>
          </div>
        </div>
      </Show>
    </Modal>
  )
}
