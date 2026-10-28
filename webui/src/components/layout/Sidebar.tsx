import { useI18n } from '@/i18n'
import { useThemeStore } from '@/stores/theme'
import { Button, IconButton, IconSun, IconMoon, IconGlobe } from '@/components/ui'

export function ThemeToggle() {
  const { t } = useI18n()
  const theme = useThemeStore()
  const light = () => theme.mode() === 'light'
  return (
    <IconButton
      size="lg"
      class="relative"
      onClick={e => theme.toggleMode(e)}
      aria-label={t('sidebar.themeToggle')}
      aria-pressed={light()}
      title={light() ? t('sidebar.switchToDark') : t('sidebar.switchToLight')}
    >
      <IconSun size={17} class={`absolute transition-[opacity,transform] duration-200 ${light() ? 'opacity-0 scale-75 rotate-45' : 'opacity-100 scale-100 rotate-0'}`} />
      <IconMoon size={17} class={`absolute transition-[opacity,transform] duration-200 ${light() ? 'opacity-100 scale-100 rotate-0' : 'opacity-0 scale-75 -rotate-45'}`} />
    </IconButton>
  )
}

export function LanguageToggle() {
  const { locale, toggleLocale, t } = useI18n()
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={toggleLocale}
      aria-label={t('sidebar.languageToggle')}
      title={locale() === 'zh-CN' ? t('sidebar.switchToEnglish') : t('sidebar.switchToChinese')}
    >
      <IconGlobe size={15} />
      <span class="text-xs font-medium">{locale() === 'zh-CN' ? 'EN' : '中'}</span>
    </Button>
  )
}
