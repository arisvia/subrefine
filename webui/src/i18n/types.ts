import type { zhCN } from './zh-CN'

export type Locale = 'zh-CN' | 'en-US'

export type TranslationDict = typeof zhCN

type Primitive = string | number | boolean | null | undefined

export type NestedKeyOf<T> = T extends Primitive
  ? never
  : {
      [K in keyof T & (string | number)]: T[K] extends Primitive
        ? `${K}`
        : `${K}.${NestedKeyOf<T[K]>}`
    }[keyof T & (string | number)]

export type I18nKey = NestedKeyOf<typeof zhCN>

export interface I18nContext {
  locale: () => Locale
  setLocale: (locale: Locale) => void
  toggleLocale: () => void
  t: (path: I18nKey, params?: Record<string, string | number>) => string
}
