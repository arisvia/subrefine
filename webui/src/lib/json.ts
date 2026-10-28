// JSON 安全窄化助手（AGENTS.md §1：前端禁 any，统一 unknown + 类型守卫）。
// 跨协议响应 / SSE 解析与本地配置读取共用。

export function asRecord(v: unknown): Record<string, unknown> | null {
  return v !== null && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : null
}

export function asArray(v: unknown): unknown[] | null {
  return Array.isArray(v) ? v : null
}

export function str(v: unknown): string | null {
  return typeof v === 'string' ? v : null
}

export function num(v: unknown): number | null {
  return typeof v === 'number' && Number.isFinite(v) ? v : null
}

/** 沿对象/数组路径安全取值：dig(obj, 'choices', 0, 'message', 'content') */
export function dig(v: unknown, ...path: (string | number)[]): unknown {
  let cur: unknown = v
  for (const step of path) {
    if (typeof step === 'number') {
      const arr = asArray(cur)
      if (!arr) return null
      cur = arr[step]
    } else {
      const rec = asRecord(cur)
      if (!rec) return null
      cur = rec[step]
    }
  }
  return cur
}
