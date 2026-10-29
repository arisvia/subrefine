
import type { I18nKey } from '@/i18n'

export type DeliveryTarget = 'sing-box' | 'clash' | 'uri'

export interface DeliveryOption {
  target: DeliveryTarget
  label: string
  /** i18n key describing what the target serves; resolved with t() at render. */
  hintKey: I18nKey
}

/** The three shapes internal/api.DetectTarget negotiates, in client popularity order. */
export const DELIVERY_OPTIONS: DeliveryOption[] = [
  { target: 'sing-box', label: 'sing-box', hintKey: 'subs.hintSingbox' },
  { target: 'clash', label: 'Clash.Meta', hintKey: 'subs.hintClash' },
  { target: 'uri', label: 'Base64', hintKey: 'subs.hintUri' },
]

/**
 * Absolute delivery URL. The sub token rides in the query because that is what
 * every proxy client supports when a subscription is pasted as a URL. A non-
 * empty mode asks the server for that full-config template instead of the
 * plain fragment; nothing about the choice is stored on the subscription.
 */
export function deliveryUrl(id: string, token: string, target: DeliveryTarget, mode = ''): string {
  const q = new URLSearchParams({ token, target })
  if (mode) q.set('mode', mode)
  return `${location.origin}/s/${encodeURIComponent(id)}?${q.toString()}`
}
