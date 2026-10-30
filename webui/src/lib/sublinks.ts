
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

/** Full-config delivery switches baked into the link. Empty/undefined opts
 *  produce the plain node fragment. */
export interface DeliveryOpts {
  /** request the complete templated config instead of the fragment */
  full?: boolean
  /** inbound kinds to keep (mixed / tun / tproxy); only meaningful with full */
  inbounds?: string[]
  /** force IPv6 posture; undefined leaves the template default */
  ipv6?: boolean
}

/**
 * Absolute delivery URL. The sub token rides in the query because that is what
 * every proxy client supports when a subscription is pasted as a URL. The
 * full-config switches live here, not on the subscription: one subscription
 * serves the fragment and any option combination simultaneously.
 */
export function deliveryUrl(id: string, token: string, target: DeliveryTarget, opts: DeliveryOpts = {}): string {
  const q = new URLSearchParams({ token, target })
  if (opts.full) {
    q.set('full', '1')
    if (opts.inbounds?.length) q.set('inbounds', opts.inbounds.join(','))
    if (opts.ipv6 !== undefined) q.set('ipv6', opts.ipv6 ? '1' : '0')
  }
  return `${location.origin}/s/${encodeURIComponent(id)}?${q.toString()}`
}
