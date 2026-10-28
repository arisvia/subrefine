import { createSignal, Show, type Component } from 'solid-js'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from '@/i18n'
import { SubrefineLogo, Input, Button, Alert, IconKey } from '@/components/ui'
import { ThemeToggle, LanguageToggle } from './Sidebar'

export const LoginModal: Component = () => {
  const auth = useAuthStore()
  const { t } = useI18n()
  const [token, setToken] = createSignal('')
  const [loading, setLoading] = createSignal(false)
  const [error, setError] = createSignal('')

  const handleSubmit = async (e?: Event) => {
    e?.preventDefault()
    const val = token().trim()
    if (!val || loading()) return

    setLoading(true)
    setError('')
    try {
      const ok = await auth.login(val)
      if (ok) {
        setToken('')
      } else {
        setError(auth.error() === 'network' ? t('api.networkError') : t('login.error'))
      }
    } catch {
      setError(t('login.error'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-overlay backdrop-blur-sm animate-fade-in">
      <div class="absolute top-4 right-4 flex items-center gap-2 z-10">
        <LanguageToggle />
        <ThemeToggle />
      </div>

      <div class="relative w-full max-w-md rounded-card glass-float p-6 sm:p-8 flex flex-col items-center text-center animate-slide-up">
        <div class="w-14 h-14 flex items-center justify-center mb-5 shrink-0">
          <SubrefineLogo class="w-11 h-11" pulsing />
        </div>

        <h1 class="text-xl font-bold text-text tracking-tight mb-2">{t('login.title')}</h1>
        <p class="text-sm text-muted leading-relaxed mb-6 max-w-xs">{t('login.subtitle')}</p>

        <Show when={error()}>
          <Alert variant="danger" class="w-full mb-4 text-left">{error()}</Alert>
        </Show>

        <form onSubmit={handleSubmit} class="w-full space-y-4">
          <div class="relative">
            <Input
              type="password"
              class="w-full h-11 pr-10"
              ariaLabel={t('login.placeholder')}
              placeholder={t('login.placeholder')}
              value={token()}
              onInput={setToken}
              disabled={loading()}
            />
            <div class="absolute right-3.5 top-1/2 -translate-y-1/2 text-muted pointer-events-none">
              <IconKey size={16} />
            </div>
          </div>

          <Button type="submit" variant="primary" loading={loading()} disabled={!token().trim()} class="w-full justify-center">
            {t('login.submit')}
          </Button>
        </form>

        <p class="mt-5 text-xs text-faint leading-relaxed">{t('login.hint')}</p>
      </div>
    </div>
  )
}
