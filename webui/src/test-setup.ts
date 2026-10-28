import { setLocale } from '@/i18n'

// Ensure all Vitest component tests default to 'zh-CN' so assertions on localized UI remain deterministic
setLocale('zh-CN')
