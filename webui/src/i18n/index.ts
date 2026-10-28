import { createSignal } from 'solid-js'
import type { Locale, I18nKey } from './types'
import { zhCN } from './zh-CN'
import { enUS } from './en-US'

export * from './types'

const STORAGE_KEY = 'subrefine_locale'

function getInitialLocale(): Locale {
  let initial: Locale = 'zh-CN'
  if (typeof window !== 'undefined' && window.localStorage) {
    try {
      const saved = window.localStorage.getItem(STORAGE_KEY)
      if (saved === 'zh-CN' || saved === 'en-US') {
        initial = saved
      }
    } catch {
      // ignore
    }
  } else if (typeof navigator !== 'undefined' && typeof navigator.language === 'string') {
    initial = navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.lang = initial === 'zh-CN' ? 'zh' : 'en'
  }
  return initial
}
const [currentLocale, setCurrentLocale] = createSignal<Locale>(getInitialLocale())

export function locale(): Locale {
  return currentLocale()
}

export function setLocale(l: Locale) {
  setCurrentLocale(l)
  if (typeof window !== 'undefined' && window.localStorage) {
    window.localStorage.setItem(STORAGE_KEY, l)
  }
  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.lang = l === 'zh-CN' ? 'zh' : 'en'
  }
}

export function toggleLocale() {
  setLocale(currentLocale() === 'zh-CN' ? 'en-US' : 'zh-CN')
}

export function t(path: I18nKey, params?: Record<string, string | number>): string {
  const loc = currentLocale()
  const dict = loc === 'en-US' ? enUS : zhCN
  let val: unknown = dict

  for (const seg of path.split('.')) {
    if (val && typeof val === 'object' && seg in val) {
      val = (val as Record<string, unknown>)[seg]
    } else {
      val = undefined
      break
    }
  }

  // Fallback to zhCN if missing in enUS
  if (typeof val !== 'string' && loc === 'en-US') {
    let fallback: unknown = zhCN
    for (const seg of path.split('.')) {
      if (fallback && typeof fallback === 'object' && seg in fallback) {
        fallback = (fallback as Record<string, unknown>)[seg]
      } else {
        fallback = undefined
        break
      }
    }
    if (typeof fallback === 'string') {
      val = fallback
    }
  }

  if (typeof val !== 'string') {
    return path
  }

  if (!params) return val
  return val.replace(/\{(\w+)\}/g, (_, k) => (k in params ? String(params[k]) : `{${k}}`))
}

export function useI18n() {
  return {
    locale,
    setLocale,
    toggleLocale,
    t,
  }
}
