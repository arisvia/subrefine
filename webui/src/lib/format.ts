import type { t as translate } from '@/i18n'

// Keep the caller's `() => uptimeUnitsFor(t)` shape: reading `t` eagerly would
// freeze the strings against the locale that was active at setup.
export function uptimeUnitsFor(t: typeof translate): UptimeUnits {
  return {
    s: t('units.second'),
    m: t('units.minuteShort'),
    h: t('units.hourShort'),
    d: t('units.day'),
    mLong: t('units.minute'),
    hLong: t('units.hour'),
  }
}

export function formatNumber(n: number | undefined | null): string {
  if (n == null || isNaN(n)) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  // 若包含非整型微量浮点，四舍五入至多保留 1 位小数，避免 42.0059 撑破布局
  if (!Number.isInteger(n)) return (Math.round(n * 10) / 10).toString()
  return String(n)
}

export function formatCost(cost: number | undefined | null): string {
  if (cost == null) return '$0'
  if (cost < 0.01) return '$' + cost.toFixed(4)
  return '$' + cost.toFixed(2)
}

export function formatBytes(bytes: number | undefined | null): string {
  const b = Number(bytes) || 0
  if (b < 1024) return `${b} B`
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`
  return `${(b / (1024 * 1024)).toFixed(2)} MB`
}

export type TimeAgoUnits = {
  justNow: string
  minutesAgo: (m: number) => string
  hoursAgo: (h: number) => string
  daysAgo: (d: number) => string
}

// Keep the caller's `() => timeUnitsFor(t)` shape: reading `t` eagerly would
// freeze the strings against the locale that was active at setup.
export function timeUnitsFor(t: typeof translate): TimeAgoUnits {
  return {
    justNow: t('time.justNow'),
    minutesAgo: m => t('time.minutesAgo', { m }),
    hoursAgo: h => t('time.hoursAgo', { h }),
    daysAgo: d => t('time.daysAgo', { d }),
  }
}

const DEFAULT_TIME_AGO: TimeAgoUnits = {
  justNow: 'just now',
  minutesAgo: m => `${m}m ago`,
  hoursAgo: h => `${h}h ago`,
  daysAgo: d => `${d}d ago`,
}

export function timeAgo(dateStr: string | undefined | null, units: TimeAgoUnits = DEFAULT_TIME_AGO): string {
  if (!dateStr) return '—'
  const diff = Date.now() - new Date(dateStr).getTime()
  if (Number.isNaN(diff)) return '—'
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return units.justNow
  if (mins < 60) return units.minutesAgo(mins)
  const hours = Math.floor(mins / 60)
  if (hours < 24) return units.hoursAgo(hours)
  return units.daysAgo(Math.floor(hours / 24))
}

export function maskKey(key: string): string {
  if (!key || key.length < 12) return key || ''
  return key.slice(0, 8) + '…' + key.slice(-4)
}
export type UptimeUnits = { s: string; m: string; h: string; d: string; mLong: string; hLong: string }

const DEFAULT_UPTIME: UptimeUnits = { s: 's', m: 'm', h: 'h', d: 'd', mLong: 'm', hLong: 'h' }

export function formatUptime(seconds: number | undefined | null, units: UptimeUnits = DEFAULT_UPTIME): string {
  const sec = Math.max(0, Math.floor(Number(seconds) || 0))
  if (sec < 60) return `${sec} ${units.s}`
  const mins = Math.floor(sec / 60)
  if (mins < 60) return `${mins} ${units.mLong}`
  const hours = Math.floor(mins / 60)
  const remMins = mins % 60
  if (hours < 24) return `${hours} ${units.hLong}${remMins > 0 ? ` ${remMins} ${units.m}` : ''}`
  const days = Math.floor(hours / 24)
  const remHours = hours % 24
  return `${days} ${units.d}${remHours > 0 ? ` ${remHours} ${units.h}` : ''}`
}

export function formatVersion(v: string | undefined | null): string {
  if (!v) return 'dev'
  const match = v.match(/^v?(\d+\.\d+\.\d+)(?:-\d{14}-([a-f0-9]{7,}))?/)
  if (match) {
    const semver = match[1]
    const commit = match[2]
    if (commit) return `${semver} (${commit.slice(0, 7)})`
    return semver
  }
  return v
}
