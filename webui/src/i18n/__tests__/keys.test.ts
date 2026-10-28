import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { zhCN } from '../zh-CN'
import { enUS } from '../en-US'

const SRC = join(__dirname, '..', '..')

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap(entry => {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) return walk(full)
    return /\.(ts|tsx)$/.test(entry) ? [full] : []
  })
}

function flatKeys(value: Record<string, unknown>, prefix = ''): string[] {
  return Object.entries(value).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof child === 'string') return [path]
    if (child && typeof child === 'object') return flatKeys(child as Record<string, unknown>, path)
    return []
  })
}

// A key counts as referenced when its literal appears anywhere in the source:
// not only t('a.b') but also keys stored in data tables and resolved later
// (e.g. option.nameKey / STEP_LABEL_KEYS).
const keyLiteral = /['"]([A-Za-z0-9_]+(?:\.[A-Za-z0-9_]+)+)['"]/g
const templateKey = /(^|[^A-Za-z0-9_$.])t\(\s*`([^`$]*)\$\{/g

function scanSource() {
  const referenced = new Set<string>()
  const prefixes = new Set<string>()
  for (const file of walk(SRC)) {
    if (relative(SRC, file).startsWith('i18n')) continue
    if (/\.test\.tsx?$/.test(file)) continue
    const text = readFileSync(file, 'utf8')
    for (const m of text.matchAll(keyLiteral)) referenced.add(m[1])
    for (const m of text.matchAll(templateKey)) prefixes.add(m[2])
  }
  return { referenced, prefixes }
}

const zhKeys = flatKeys(zhCN as unknown as Record<string, unknown>)
const enKeys = flatKeys(enUS as unknown as Record<string, unknown>)

describe('i18n dictionaries', () => {
  it('zh and en carry exactly the same key set', () => {
    expect({
      onlyInZh: zhKeys.filter(k => !enKeys.includes(k)),
      onlyInEn: enKeys.filter(k => !zhKeys.includes(k)),
    }).toEqual({ onlyInZh: [], onlyInEn: [] })
  })

  it('every declared key is referenced by the app', () => {
    const { referenced, prefixes } = scanSource()
    const dead = zhKeys.filter(k =>
      !referenced.has(k) && ![...prefixes].some(prefix => `${k}.`.startsWith(prefix)))
    expect({ deadCount: dead.length, dead }).toEqual({ deadCount: 0, dead: [] })
  })

  it('the dead-key scanner itself resolves data-table keys', () => {
    const { referenced } = scanSource()
    expect(referenced.has('settings.appearance.themeColorIndigo')).toBe(true)
    expect(referenced.has('pipeline.type.deduplicate')).toBe(true)
    expect(zhKeys.length).toBeGreaterThan(120)
  })
})
