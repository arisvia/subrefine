
export type DeliveryTarget = 'sing-box' | 'clash' | 'uri'

export interface DeliveryOption {
  target: DeliveryTarget
  label: string
  hint: string
}

/** The three shapes internal/api.DetectTarget negotiates, in client popularity order. */
export const DELIVERY_OPTIONS: DeliveryOption[] = [
  { target: 'sing-box', label: 'sing-box', hint: 'JSON outbounds (1.15)' },
  { target: 'clash', label: 'Clash.Meta', hint: 'YAML proxies + groups' },
  { target: 'uri', label: 'Base64', hint: 'URI list for v2rayN / PassWall' },
]

/**
 * Absolute delivery URL. The sub token rides in the query because that is what
 * every proxy client supports when a subscription is pasted as a URL.
 */
export function deliveryUrl(id: string, token: string, target: DeliveryTarget): string {
  const q = new URLSearchParams({ token, target })
  return `${location.origin}/s/${encodeURIComponent(id)}?${q.toString()}`
}
