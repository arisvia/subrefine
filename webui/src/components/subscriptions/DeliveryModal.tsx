import { type Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js'
import { useI18n } from '@/i18n'
import { Button, Modal, SegmentedControl, Checkbox, IconClipboard } from '@/components/ui'
import { DELIVERY_OPTIONS, deliveryUrl, type DeliveryTarget } from '@/lib/sublinks'
import { useTemplatesStore } from '@/stores/templates'
import { useKeysStore } from '@/stores/keys'
import { copyToClipboard } from '@/lib/clipboard'
import { toast } from '@/lib/toast'
import { maskKey } from '@/lib/format'
import type { FamilyOptions, Subscription } from '@/types/domain'

export interface DeliveryModalProps {
  /** null closes the dialog; the opener preselects a format. */
  sub: Subscription | null
  initialTarget: DeliveryTarget
  onClose: () => void
}

/** Human label for an inbound kind. Unknown kinds fall back to uppercase. */
const INBOUND_LABEL_KEY: Record<string, 'delivery.inMixed' | 'delivery.inTun' | 'delivery.inTproxy'> = {
  mixed: 'delivery.inMixed',
  tun: 'delivery.inTun',
  tproxy: 'delivery.inTproxy',
}

/**
 * Sub-Store style output dialog: pick format, delivery key, and (for
 * sing-box/clash with a loaded template) fragment-vs-full plus the full-config
 * options. Nothing is stored on the subscription - every choice lives in the
 * copied link, so one subscription serves any combination at once.
 */
export const DeliveryModal: Component<DeliveryModalProps> = props => {
  const { t } = useI18n()
  const templates = useTemplatesStore()
  const keysPool = useKeysStore()

  const [target, setTarget] = createSignal<DeliveryTarget>('uri')
  const [keyId, setKeyId] = createSignal('')
  const [full, setFull] = createSignal(false)
  const [inbounds, setInbounds] = createSignal<string[]>([])
  const [ipv6, setIpv6] = createSignal(true)

  function familyFor(tg: DeliveryTarget): FamilyOptions {
    const o = templates.options()
    if (tg === 'sing-box') return o.singbox
    if (tg === 'clash') return o.clash
    return { available: false, inbounds: [] }
  }
  const family = createMemo(() => familyFor(target()))

  // Re-seed the choices whenever a subscription is opened.
  createEffect(() => {
    const sub = props.sub
    if (!sub) return
    setTarget(props.initialTarget)
    setKeyId(sub.keys?.[0]?.id ?? '')
    setFull(false)
    setInbounds([])
    setIpv6(true)
    void templates.load()
  })

  // Switching format keeps the full selection only if the new family has a
  // template; otherwise snap back to the fragment so we never emit a link the
  // server would downgrade.
  function chooseTarget(next: DeliveryTarget) {
    setTarget(next)
    if (!full()) return
    const fam = familyFor(next)
    if (!fam.available) {
      setFull(false)
      return
    }
    setInbounds(fam.inbounds.slice())
  }

  function toggleFull(v: string) {
    const on = v === '1'
    setFull(on)
    if (on) {
      setInbounds(family().inbounds.slice())
      setIpv6(true)
    }
  }

  function toggleInbound(kind: string, on: boolean) {
    const cur = inbounds()
    setInbounds(on ? [...cur, kind] : cur.filter(x => x !== kind))
  }

  const token = () => keysPool.byId(keyId())?.token ?? props.sub?.token ?? ''
  const url = createMemo(() =>
    props.sub
      ? deliveryUrl(props.sub.id, token(), target(), { full: full(), inbounds: inbounds(), ipv6: ipv6() })
      : '')

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
              onChange={chooseTarget}
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

          <Show when={family().available}>
            <div class="space-y-3 border-t border-border/60 pt-3">
              <div>
                <div class="pb-1.5 text-xs text-muted">{t('delivery.shape')}</div>
                <SegmentedControl
                  class="w-full"
                  value={full() ? '1' : '0'}
                  onChange={toggleFull}
                  options={[
                    { value: '0', label: t('delivery.shapeFragment') },
                    { value: '1', label: t('delivery.shapeFull') },
                  ]}
                />
              </div>

              <Show when={full()}>
                <Show when={family().inbounds.length}>
                  <div>
                    <div class="pb-1.5 text-xs text-muted">{t('delivery.inbounds')}</div>
                    <div class="flex flex-wrap gap-x-4 gap-y-1.5">
                      <For each={family().inbounds}>
                        {(kind) => (
                          <Checkbox
                            checked={inbounds().includes(kind)}
                            label={INBOUND_LABEL_KEY[kind] ? t(INBOUND_LABEL_KEY[kind]) : kind.toUpperCase()}
                            onChange={on => toggleInbound(kind, on)}
                          />
                        )}
                      </For>
                    </div>
                  </div>
                </Show>
                <Checkbox
                  checked={ipv6()}
                  label={t('delivery.ipv6')}
                  onChange={setIpv6}
                />
                <div class="text-[11px] text-faint">{t('delivery.fullHint')}</div>
              </Show>
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
