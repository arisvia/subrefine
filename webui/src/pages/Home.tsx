import { type Component, For, Show, createEffect, createMemo, on, onMount } from 'solid-js'
import { A } from '@solidjs/router'
import { useI18n } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionsStore } from '@/stores/subscriptions'
import {
  Badge, Button, Card, Empty, PageHeader, Skeleton, IconSparkles, IconLayers, IconZap, IconLink, IconSettings,
} from '@/components/ui'
import { copyToClipboard } from '@/lib/clipboard'
import { toast } from '@/lib/toast'
import { formatNumber, formatVersion, timeAgo, timeUnitsFor } from '@/lib/format'
import { DELIVERY_OPTIONS, deliveryUrl } from '@/lib/sublinks'
import { PROTOCOL_LABELS, PROTOCOL_TONES, type Protocol } from '@/types/domain'

const Home: Component = () => {
  const { t } = useI18n()
  const auth = useAuthStore()
  const subs = useSubscriptionsStore()

  onMount(() => { void subs.load() })

  // Refresh every subscription once so the overview can aggregate real nodes.
  // on() runs the handler untracked: preview writes must never re-trigger this
  // fan-out, or each resolved/failed preview re-requests every subscription.
  createEffect(on(() => subs.subs(), list => {
    for (const s of list) void subs.runPreview(s.id)
  }))

  const totals = subs.totals

  const nodeMix = createMemo(() => {
    const counts = new Map<Protocol, number>()
    let nodes = 0
    for (const s of subs.subs()) {
      const list = subs.preview(s.id).nodes
      nodes += list.length
      for (const n of list) counts.set(n.type, (counts.get(n.type) ?? 0) + 1)
    }
    return {
      nodes,
      mix: [...counts.entries()].sort((a, b) => b[1] - a[1]),
    }
  })

  const recent = createMemo(() =>
    [...subs.subs()].sort((a, b) => b.updated_at - a.updated_at).slice(0, 5))

  const kpis = createMemo(() => [
    { label: t('overview.subsTotal'), value: totals().subs, icon: <IconLayers size={15} /> },
    { label: t('overview.subsEnabled'), value: totals().enabled, icon: <IconZap size={15} /> },
    { label: t('overview.sourcesTotal'), value: totals().sources, icon: <IconLink size={15} /> },
    { label: t('overview.operatorsTotal'), value: totals().operators, icon: <IconSettings size={15} /> },
  ])

  const copy = async (text: string, label: string) => {
    if (await copyToClipboard(text)) toast.success(t('subs.linkCopied', { target: label }))
  }

  return (
    <>
      <PageHeader
        title={t('overview.title')}
        subtitle={t('overview.subtitle')}
        badge={<Badge tone="green">{formatVersion(auth.info()?.version)}</Badge>}
        actions={
          <A href="/subscriptions?new=1">
            <Button size="sm" variant="primary"><IconSparkles size={14} />{t('subs.create')}</Button>
          </A>
        }
      />

      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <For each={kpis()}>
          {kpi => (
            <Card class="p-4 flex items-center gap-3">
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-control bg-surface-inset text-accent">
                {kpi.icon}
              </span>
              <div class="min-w-0">
                <div class="text-xl font-semibold text-foreground tabular-nums leading-none">{formatNumber(kpi.value)}</div>
                <div class="mt-1 text-[11px] text-faint truncate">{kpi.label}</div>
              </div>
            </Card>
          )}
        </For>
        <Card class="p-4 flex items-center gap-3 sm:col-span-2 lg:col-span-1">
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-control bg-surface-inset text-accent">
            <IconZap size={15} />
          </span>
          <div class="min-w-0">
            <div class="text-xl font-semibold text-foreground tabular-nums leading-none">
              {subs.subs().length && !nodeMix().nodes && subs.loading()
                ? <Skeleton class="h-5 w-10" />
                : formatNumber(nodeMix().nodes)}
            </div>
            <div class="mt-1 text-[11px] text-faint truncate">{t('overview.nodesTotal')}</div>
          </div>
        </Card>
      </div>

      <div class="mt-4 grid gap-3 lg:grid-cols-2">
        <Card class="p-5 space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-sm font-semibold text-foreground">{t('overview.protocols')}</h2>
            <Badge tone="indigo">{nodeMix().nodes}</Badge>
          </div>
          <Show when={nodeMix().mix.length} fallback={<div class="text-xs text-faint">{t('common.empty')}</div>}>
            <div class="flex flex-wrap gap-1.5">
              <For each={nodeMix().mix}>
                {([proto, count]) => (
                  <Badge tone={PROTOCOL_TONES[proto] ?? 'gray'}>
                    {PROTOCOL_LABELS[proto] ?? proto} · {count}
                  </Badge>
                )}
              </For>
            </div>
          </Show>

          <div class="pt-1 space-y-2">
            <h3 class="text-xs font-medium text-muted">{t('overview.recentSubs')}</h3>
            <Show when={recent().length} fallback={<div class="text-xs text-faint">{t('overview.noSubs')}</div>}>
              <div class="space-y-1.5">
                <For each={recent()}>
                  {sub => (
                    <div class="flex items-center gap-2 rounded-control border border-glass-border bg-surface-inset/50 px-3 py-2">
                      <span class="min-w-0 flex-1 truncate text-xs font-medium text-foreground">{sub.name || sub.id}</span>
                      <span class="shrink-0 text-[11px] text-faint tabular-nums">
                        {sub.updated_at ? timeAgo(new Date(sub.updated_at * 1000).toISOString(), timeUnitsFor(t)) : t('subs.never')}
                      </span>
                      <A href={`/subscriptions?edit=${encodeURIComponent(sub.id)}`} class="pill shrink-0">{t('common.edit')}</A>
                    </div>
                  )}
                </For>
              </div>
            </Show>
          </div>
        </Card>

        <Card class="p-5 space-y-3">
          <h2 class="text-sm font-semibold text-foreground">{t('overview.serviceInfo')}</h2>
          <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
            <dt class="text-faint">{t('common.version')}</dt>
            <dd class="text-right font-mono text-foreground">{formatVersion(auth.info()?.version)}</dd>
            <dt class="text-faint">{t('overview.listenAddr')}</dt>
            <dd class="text-right font-mono text-foreground">{auth.info()?.host ?? '—'}:{auth.info()?.port ?? 0}</dd>
            <dt class="text-faint">{t('overview.subscribePath')}</dt>
            <dd class="text-right font-mono text-foreground">/s/{'{id}'}</dd>
          </dl>
          <p class="text-[11px] text-faint">{t('overview.runtimeHint')}</p>

          <div class="space-y-1.5 pt-1">
            <h3 class="text-xs font-medium text-muted">{t('settings.endpoints')}</h3>
            <Show when={subs.subs().length} fallback={
              <div class="text-[11px] text-faint">{t('overview.noSubs')}</div>
            }>
              <For each={subs.subs().slice(0, 3)}>
                {sub => (
                  <div class="flex items-center gap-1.5">
                    <span class="w-24 shrink-0 truncate text-[11px] text-faint">{sub.id}</span>
                    <For each={DELIVERY_OPTIONS}>
                      {opt => (
                        <button type="button" class="pill" title={opt.hint}
                          onClick={() => copy(deliveryUrl(sub.id, sub.token, opt.target), opt.label)}>
                          {opt.label}
                        </button>
                      )}
                    </For>
                  </div>
                )}
              </For>
            </Show>
          </div>
        </Card>
      </div>

      <Show when={!subs.subs().length}>
        <Card class="mt-4 p-8">
          <Empty
            title={t('overview.quickStart')}
            description={t('overview.quickStartDesc')}
            action={<A href="/subscriptions?new=1"><Button size="sm" variant="primary"><IconSparkles size={14} />{t('overview.goCreate')}</Button></A>}
          />
        </Card>
      </Show>
    </>
  )
}

export default Home
