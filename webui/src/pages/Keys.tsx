import { type Component, For, Show, createSignal, onMount } from 'solid-js'
import { useI18n } from '@/i18n'
import { useKeysStore } from '@/stores/keys'
import {
  Badge, Button, Card, Empty, IconButton, Input, PageHeader, Skeleton,
  IconCheck, IconClipboard, IconClose, IconEdit, IconEye, IconKey, IconRotateCcw, IconTrash,
} from '@/components/ui'
import { confirm } from '@/lib/confirm'
import { toast } from '@/lib/toast'
import { copyToClipboard } from '@/lib/clipboard'
import { maskKey } from '@/lib/format'
import { deliveryUrl } from '@/lib/sublinks'
import type { AdminKey } from '@/types/domain'

const KeysPage: Component = () => {
  const { t } = useI18n()
  const store = useKeysStore()
  const [newName, setNewName] = createSignal('')
  const [busy, setBusy] = createSignal('')
  const [revealed, setRevealed] = createSignal<Record<string, boolean>>({})
  const [editingId, setEditingId] = createSignal('')
  const [editName, setEditName] = createSignal('')

  onMount(() => { void store.load() })

  const create = async () => {
    const name = newName().trim()
    if (!name) return
    setBusy('create')
    const ok = await store.create(name)
    setBusy('')
    if (ok) {
      setNewName('')
      toast.success(t('keys.created', { name }))
    } else {
      toast.error(t('keys.failed'))
    }
  }

  const rename = async (k: AdminKey) => {
    const name = editName().trim()
    setEditingId('')
    if (!name || name === k.name) return
    setBusy(k.id)
    const ok = await store.update(k.id, { name })
    setBusy('')
    if (!ok) toast.error(t('keys.failed'))
  }

  const rotate = async (k: AdminKey) => {
    const ok = await confirm({
      title: t('keys.rotate'),
      message: t('keys.rotateConfirm', { name: k.name, count: k.in_use.length }),
      variant: 'warning',
    })
    if (!ok) return
    setBusy(k.id)
    // An empty token means "generate a fresh one" server-side.
    const next = await store.update(k.id, { token: '' })
    setBusy('')
    if (next) {
      setRevealed(r => ({ ...r, [k.id]: true }))
      toast.success(t('keys.rotated'))
    } else {
      toast.error(t('keys.failed'))
    }
  }

  const remove = async (k: AdminKey) => {
    const attached = k.in_use.length > 0
    const ok = await confirm({
      title: t('keys.delete'),
      message: attached
        ? t('keys.deleteConfirmUsed', { name: k.name, count: k.in_use.length })
        : t('keys.deleteConfirmUnused', { name: k.name }),
      variant: 'danger',
    })
    if (!ok) return
    setBusy(k.id)
    const done = await store.remove(k.id, attached)
    setBusy('')
    if (done) toast.success(t('keys.deleted'))
    else toast.error(t('keys.failed'))
  }

  const copyToken = async (k: AdminKey) => {
    if (await copyToClipboard(k.token)) toast.success(t('keys.tokenCopied'))
  }

  const copyLink = async (k: AdminKey, subId: string) => {
    if (await copyToClipboard(deliveryUrl(subId, k.token, 'uri'))) {
      toast.success(t('keys.linkCopied', { id: subId }))
    }
  }

  return (
    <div class="space-y-5 stagger">
      <PageHeader
        title={t('keys.title')}
        subtitle={t('keys.subtitle')}
        badge={<Show when={store.keys().length}><Badge tone="green">{store.keys().length}</Badge></Show>}
      />

      <Card class="p-4">
        <div class="flex flex-wrap items-end gap-2">
          <div class="min-w-40 flex-1">
            <div class="pb-1 text-xs text-muted">{t('keys.name')}</div>
            <Input
              value={newName()}
              placeholder={t('keys.namePlaceholder')}
              ariaLabel={t('keys.name')}
              onInput={setNewName}
              onKeyDown={e => { if (e.key === 'Enter') void create() }}
            />
          </div>
          <Button variant="primary" loading={busy() === 'create'} disabled={!newName().trim()} onClick={() => void create()}>
            <IconKey size={14} />{t('keys.add')}
          </Button>
        </div>
        <div class="pt-2 text-[11px] text-faint">{t('keys.tokenHint')}</div>
      </Card>

      <Show when={store.loading() && !store.keys().length}>
        <div class="space-y-3" role="status" aria-busy="true">
          <Skeleton class="h-24 w-full" />
          <Skeleton class="h-24 w-full" />
        </div>
      </Show>

      <Show when={!store.loading() && !store.keys().length}>
        <Card class="p-8">
          <Empty title={t('keys.emptyTitle')} description={t('keys.emptyDesc')} />
        </Card>
      </Show>

      <div class="space-y-3">
        <For each={store.keys()}>
          {k => (
            <Card class="p-4 space-y-2.5">
              <div class="flex flex-wrap items-center gap-2">
                <Show when={editingId() !== k.id} fallback={
                  <>
                    <Input
                      size="sm"
                      class="max-w-48"
                      value={editName()}
                      ariaLabel={t('keys.name')}
                      onInput={setEditName}
                      onKeyDown={e => { if (e.key === 'Enter') void rename(k) }}
                    />
                    <IconButton size="sm" onClick={() => void rename(k)} aria-label={t('common.save')}><IconCheck size={14} /></IconButton>
                    <IconButton size="sm" onClick={() => setEditingId('')} aria-label={t('common.cancel')}><IconClose size={14} /></IconButton>
                  </>
                }>
                  <span class="text-sm font-medium text-foreground">{k.name}</span>
                  <IconButton
                    size="sm"
                    onClick={() => { setEditingId(k.id); setEditName(k.name) }}
                    aria-label={t('keys.rename')}
                  >
                    <IconEdit size={13} />
                  </IconButton>
                </Show>
                <span class="ml-auto flex items-center gap-1.5">
                  <Show when={busy() === k.id}>
                    <span class="text-[11px] text-faint">…</span>
                  </Show>
                  <IconButton size="sm" onClick={() => void rotate(k)} aria-label={t('keys.rotate')} title={t('keys.rotate')}>
                    <IconRotateCcw size={13} />
                  </IconButton>
                  <IconButton size="sm" class="hover:text-danger" onClick={() => void remove(k)} aria-label={t('keys.delete')} title={t('keys.delete')}>
                    <IconTrash size={13} />
                  </IconButton>
                </span>
              </div>

              <div class="flex flex-wrap items-center gap-2 text-xs font-mono">
                <span class="text-muted">{t('keys.token')}:</span>
                <span class="text-foreground">{revealed()[k.id] ? k.token : maskKey(k.token)}</span>
                <button type="button" class="text-faint hover:text-foreground" onClick={() => setRevealed(r => ({ ...r, [k.id]: !r[k.id] }))} aria-label={revealed()[k.id] ? t('keys.hide') : t('keys.show')}>
                  <IconEye size={13} />
                </button>
                <IconButton size="sm" onClick={() => void copyToken(k)} aria-label={t('keys.copyToken')} title={t('keys.copyToken')}>
                  <IconClipboard size={13} />
                </IconButton>
              </div>

              <div class="flex flex-wrap items-center gap-1.5 text-xs">
                <Show when={k.in_use.length} fallback={
                  <span class="text-faint">{t('keys.notInUse')}</span>
                }>
                  <span class="text-muted">{t('keys.inUse')}:</span>
                  <For each={k.in_use}>
                    {id => (
                      <button
                        type="button"
                        class="pill hover:text-foreground"
                        title={t('keys.copyLinkFor', { id })}
                        onClick={() => void copyLink(k, id)}
                      >
                        {id} <IconClipboard size={11} class="inline" />
                      </button>
                    )}
                  </For>
                </Show>
              </div>
            </Card>
          )}
        </For>
      </div>
    </div>
  )
}

export default KeysPage
