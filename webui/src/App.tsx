import { type Component, type JSX, For, Show, createSignal, onMount, onCleanup, createEffect, lazy, Suspense } from 'solid-js'
import { HashRouter, Route, A, useLocation } from '@solidjs/router'
import { useAuthStore } from './stores/auth'
import { useSubscriptionsStore } from './stores/subscriptions'
import { ThemeToggle, LanguageToggle } from './components/layout/Sidebar'
import { useI18n } from './i18n'
import {
  ToastHost, ConfirmDialogHost, SubrefineLogo, Skeleton, Button, IconButton, BackToTop, Badge,
  IconHome, IconLayers, IconSettings, IconClose, IconMenu, IconLogout, IconZap,
} from './components/ui'
import { LoginModal } from './components/layout/LoginModal'
import Home from './pages/Home'
const Subscriptions = lazy(() => import('./pages/Subscriptions'))
const Settings = lazy(() => import('./pages/Settings'))

const NavIcons = {
  overview: () => <IconHome class="w-4 h-4 shrink-0" />,
  subscriptions: () => <IconLayers class="w-4 h-4 shrink-0" />,
}

const App: Component = () => {
  const auth = useAuthStore()
  const subs = useSubscriptionsStore()
  const { t } = useI18n()
  const [open, setOpen] = createSignal(false)

  onMount(() => {
    void auth.probe().then(() => {
      if (auth.authenticated()) void subs.load()
    })
  })

  // Reload the list whenever the session becomes valid (login, token re-entry).
  createEffect(() => {
    if (auth.authenticated()) void subs.load(true)
  })

  // Layout is passed as the Router root so the sidebar <A> links stay inside router context.
  const Layout: Component<{ children?: JSX.Element }> = props => {
    const location = useLocation()
    const pageTitle = () => {
      const titles = {
        subscriptions: 'nav.subscriptions',
        settings: 'nav.settings',
      } as const
      const route = location.pathname.split('/')[1] as keyof typeof titles
      return t(titles[route] ?? 'nav.overview')
    }
    let drawer: HTMLElement | undefined

    createEffect(() => {
      location.pathname
      window.scrollTo(0, 0)
      setOpen(false)
      document.documentElement.setAttribute('data-scrolled', 'false')
    })

    createEffect(() => {
      if (!open()) return
      const previousFocus = document.activeElement as HTMLElement | null
      const previousOverflow = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      queueMicrotask(() => drawer?.querySelector<HTMLElement>('button, a[href]')?.focus())
      const onKey = (event: KeyboardEvent) => {
        if (event.key === 'Escape') setOpen(false)
        if (event.key !== 'Tab') return
        const controls = drawer?.querySelectorAll<HTMLElement>('a[href], button:not(:disabled)')
        if (!controls?.length) return
        const first = controls[0]
        const last = controls[controls.length - 1]
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault()
          last.focus()
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault()
          first.focus()
        }
      }
      const desktop = window.matchMedia('(min-width: 768px)')
      const onResize = () => { if (desktop.matches) setOpen(false) }
      desktop.addEventListener('change', onResize)
      document.addEventListener('keydown', onKey)
      onCleanup(() => {
        document.body.style.overflow = previousOverflow
        document.removeEventListener('keydown', onKey)
        desktop.removeEventListener('change', onResize)
        previousFocus?.focus()
      })
    })

    onMount(() => {
      let ticking = false
      const onScroll = () => {
        if (!ticking) {
          window.requestAnimationFrame(() => {
            document.documentElement.setAttribute('data-scrolled', window.scrollY > 8 ? 'true' : 'false')
            ticking = false
          })
          ticking = true
        }
      }
      window.addEventListener('scroll', onScroll, { passive: true })
      onScroll()
      onCleanup(() => {
        window.removeEventListener('scroll', onScroll)
        document.documentElement.removeAttribute('data-scrolled')
      })
    })

    const brand = (
      <>
        <SubrefineLogo class="w-8 h-8 shrink-0" />
        <div class="min-w-0">
          <div class="text-sm font-bold leading-tight truncate text-foreground">subrefine</div>
          <div class="text-[10px] leading-tight text-faint truncate">subscription refinery</div>
        </div>
      </>
    )

    return (
      <div class="min-h-screen text-text relative selection:bg-accent/25 app-root-shell bg-bg">
        <div class="app-ambient" aria-hidden="true" />
        <ToastHost />
        <ConfirmDialogHost />

        {/* Desktop sidebar */}
        <aside class="hidden md:flex flex-col fixed inset-y-0 left-0 w-(--sidebar-w) z-40 app-sidebar">
          <div class="h-19 flex items-center gap-3 px-5 shrink-0">{brand}</div>
          <SidebarNav onNavigate={() => setOpen(false)} />
          <div class="h-14 px-4 border-t border-glass-border flex items-center justify-between">
            <div class="flex items-center gap-1">
              <ThemeToggle />
              <LanguageToggle />
            </div>
            <A
              href="/settings"
              class="flex h-9 w-9 items-center justify-center rounded-control text-muted hover:text-foreground hover:bg-hover transition-colors"
              title={t('nav.settings')}
              aria-label={t('nav.settings')}
            >
              <IconSettings size={16} />
            </A>
          </div>
        </aside>

        {/* Mobile drawer */}
        <Show when={open()}>
          <div class="md:hidden fixed inset-0 z-50">
            <Button
              variant="ghost"
              aria-label={t('nav.closeMenuOverlay')}
              tabindex={-1}
              class="absolute inset-0 w-full h-full! rounded-none! bg-overlay! backdrop-blur-sm animate-fade-in border-none! active:scale-100!"
              onClick={() => setOpen(false)}
            />
            <aside ref={drawer} role="dialog" aria-modal="true" aria-label={t('nav.groupMain')} class="absolute inset-y-0 left-0 w-68 max-w-[85vw] app-sidebar flex flex-col animate-slide-up">
              <div class="h-19 flex items-center gap-3 px-5 shrink-0">
                {brand}
                <IconButton size="lg" class="ml-auto" onClick={() => setOpen(false)} aria-label={t('nav.closeMenu')}>
                  <IconClose size={18} />
                </IconButton>
              </div>
              <SidebarNav onNavigate={() => setOpen(false)} />
              <div class="h-14 px-4 border-t border-glass-border flex items-center justify-between shrink-0">
                <div class="flex items-center gap-1">
                  <ThemeToggle />
                  <LanguageToggle />
                </div>
              </div>
            </aside>
          </div>
        </Show>

        {/* Main column */}
        <div class="flex flex-col md:pl-(--sidebar-w) min-h-dvh relative z-10">
          <header class="sticky top-0 z-30 px-4 lg:px-8 pt-3 pb-2 pointer-events-none">
            <div class="max-w-7xl mx-auto h-13 flex items-center justify-between gap-3 px-3 sm:px-4 rounded-2xl glass-sticky pointer-events-auto">
              <IconButton
                size="lg"
                class="md:hidden"
                onClick={() => setOpen(true)}
                aria-label={t('nav.openMenu')}
                aria-expanded={open()}
              >
                <IconMenu size={18} />
              </IconButton>
              <div class="min-w-0 flex items-center gap-2 text-xs text-muted">
                <span class="truncate font-medium">{pageTitle()}</span>
                <span class="text-faint hidden lg:inline" aria-hidden="true">/</span>
                <span class="text-faint hidden lg:inline">{t('nav.tagline')}</span>
              </div>
              <div class="ml-auto flex items-center gap-2 shrink-0">
                <Show when={auth.authenticated()}>
                  <Badge tone="green">{t('overview.subsEnabled')}: {subs.totals().enabled}</Badge>
                  <span class="hidden sm:flex items-center gap-1.5 text-xs text-faint font-mono">
                    <IconZap size={12} class="text-accent" />
                    {subs.totals().subs}
                  </span>
                  <IconButton
                    onClick={() => auth.logout()}
                    title={t('login.logout')}
                    aria-label={t('login.logout')}
                    class="hover:text-danger"
                  >
                    <IconLogout size={15} />
                  </IconButton>
                </Show>
              </div>
            </div>
          </header>

          <main class="flex-1 min-w-0 max-w-7xl w-full mx-auto px-4 lg:px-8 py-6 lg:py-8">
            <Suspense fallback={
              <div class="space-y-5" role="status" aria-label={t('common.loading')} aria-busy="true">
                <Skeleton class="h-7 w-40" />
                <Skeleton class="h-48 w-full" />
              </div>
            }>
              {props.children}
            </Suspense>
          </main>
          <BackToTop />
        </div>

        {/* Admin token gate */}
        <Show when={auth.checked() && !auth.authenticated()}>
          <LoginModal />
        </Show>
      </div>
    )
  }

  return (
    <HashRouter root={Layout}>
      <Route path="/" component={Home} />
      <Route path="/subscriptions" component={Subscriptions} />
      <Route path="/settings" component={Settings} />
      <Route path="*" component={Home} />
    </HashRouter>
  )
}

function SidebarNav(props: { onNavigate?: () => void }) {
  const { t } = useI18n()
  const navGroups = () => [
    {
      group: t('nav.groupMain'),
      items: [
        { href: '/', label: t('nav.overview'), end: true, icon: NavIcons.overview },
        { href: '/subscriptions', label: t('nav.subscriptions'), icon: NavIcons.subscriptions },
      ],
    },
    {
      group: t('nav.groupSystem'),
      items: [
        { href: '/settings', label: t('nav.settings'), icon: () => <IconSettings class="w-4 h-4 shrink-0" /> },
      ],
    },
  ]

  return (
    <nav class="flex-1 overflow-y-auto px-3.5 py-4 space-y-6">
      <For each={navGroups()}>
        {group => (
          <div>
            <div class="px-3 pb-2 text-[11px] font-semibold uppercase tracking-wider text-faint">{group.group}</div>
            <div class="space-y-1">
              <For each={group.items}>
                {item => (
                  <A href={item.href} end={item.end} onClick={props.onNavigate} class="nav-link" activeClass="nav-active">
                    <item.icon />
                    <span>{item.label}</span>
                  </A>
                )}
              </For>
            </div>
          </div>
        )}
      </For>
    </nav>
  )
}

export default App
