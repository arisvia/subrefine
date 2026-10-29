import { type Component, For } from 'solid-js'
import { useI18n, setLocale, type Locale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore, THEME_COLOR_OPTIONS } from '@/stores/theme'
import { Badge, Button, Card, PageHeader, SegmentedControl, IconGlobe, IconMoon, IconSun, IconTrash } from '@/components/ui'
import { toast } from '@/lib/toast'
import { formatVersion } from '@/lib/format'

const Settings: Component = () => {
  const { t, locale } = useI18n()
  const auth = useAuthStore()
  const theme = useThemeStore()

  const localeOptions: { value: Locale; label: string }[] = [
    { value: 'zh-CN', label: '简体中文' },
    { value: 'en-US', label: 'English' },
  ]

  return (
    <div class="space-y-5 stagger">
      <PageHeader title={t('settings.title')} subtitle={t('settings.subtitle')} />

      <div class="space-y-3">
        <Card class="p-5 space-y-4">
          <h2 class="text-sm font-semibold text-foreground">{t('settings.groupAppearance')}</h2>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <span class="text-xs text-muted">{t('settings.theme')}</span>
            <SegmentedControl
              value={theme.mode()}
              onChange={mode => theme.setMode(mode)}
              options={[
                { value: 'dark', label: t('settings.themeDark'), icon: <IconMoon size={13} /> },
                { value: 'light', label: t('settings.themeLight'), icon: <IconSun size={13} /> },
              ]}
            />
          </div>

          <div class="space-y-2">
            <span class="text-xs text-muted">{t('settings.accentColor')}</span>
            <div class="grid gap-2 sm:grid-cols-2">
              <For each={THEME_COLOR_OPTIONS}>
                {option => {
                  const active = () => theme.color() === option.id
                  return (
                    <button
                      type="button"
                      aria-pressed={active()}
                      onClick={() => theme.setColor(option.id)}
                      class={`flex items-center gap-3 rounded-control border px-3 py-2.5 text-left transition-colors ${
                        active()
                          ? 'border-accent/50 bg-accent/10 text-foreground'
                          : 'border-glass-border bg-surface-inset/50 text-muted hover:border-glass-border-hover hover:text-foreground'
                      }`}
                    >
                      <span class={`h-7 w-7 shrink-0 rounded-lg bg-gradient-to-br ${option.gradient}`} />
                      <span class="min-w-0">
                        <span class="block text-xs font-medium truncate">{t(option.nameKey)}</span>
                        <span class="block text-[11px] text-faint truncate">{t(option.descKey)}</span>
                      </span>
                    </button>
                  )
                }}
              </For>
            </div>
          </div>

          <div class="flex flex-wrap items-center justify-between gap-3">
            <span class="inline-flex items-center gap-2 text-xs text-muted">
              <IconGlobe size={13} />
              {t('settings.language')}
            </span>
            <SegmentedControl value={locale()} onChange={setLocale} options={localeOptions} />
          </div>
        </Card>

        <Card class="p-5 space-y-3">
          <h2 class="text-sm font-semibold text-foreground">{t('settings.security')}</h2>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="text-xs text-muted">{t('settings.adminToken')}</div>
              <div class="mt-1 inline-flex items-center gap-2">
                <Badge tone={auth.hasToken() ? 'green' : 'gray'}>
                  {auth.hasToken() ? t('settings.adminTokenStored') : t('settings.adminTokenNone')}
                </Badge>
              </div>
            </div>
            <Button
              size="sm"
              variant="danger"
              disabled={!auth.hasToken()}
              onClick={() => {
                auth.logout()
                toast.success(t('login.loggedOut'))
              }}
            >
              <IconTrash size={13} />
              {t('settings.clearToken')}
            </Button>
          </div>
        </Card>

        <Card class="p-5 space-y-3">
          <h2 class="text-sm font-semibold text-foreground">{t('settings.about')}</h2>
          <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-xs">
            <dt class="text-faint">{t('common.version')}</dt>
            <dd class="text-right font-mono text-foreground">{formatVersion(auth.info()?.version)}</dd>
            <dt class="text-faint">{t('settings.endpointAdmin')}</dt>
            <dd class="text-right font-mono text-foreground">/api/admin/*</dd>
            <dt class="text-faint">{t('settings.endpointSub')}</dt>
            <dd class="text-right font-mono text-foreground">/s/{'{id}?token=…'}</dd>
            <dt class="text-faint">{t('settings.endpointHealth')}</dt>
            <dd class="text-right font-mono text-foreground">/healthz</dd>
          </dl>
        </Card>
      </div>
    </div>
  )
}

export default Settings
