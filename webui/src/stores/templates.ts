import { createRoot, createSignal } from 'solid-js'
import { api } from '@/lib/api'
import type { TemplateModes } from '@/types/domain'

const EMPTY: TemplateModes = { singbox: [], clash: [] }

export interface TemplatesStore {
  modes: () => TemplateModes
  load: () => Promise<void>
}

function createTemplatesStore(): TemplatesStore {
  const [modes, setModes] = createSignal<TemplateModes>(EMPTY)

  async function load() {
    try {
      setModes(await api<TemplateModes>('/api/admin/templates'))
    } catch {
      setModes(EMPTY)
    }
  }

  return { modes, load }
}

let store: TemplatesStore | undefined

export function useTemplatesStore(): TemplatesStore {
  if (!store) store = createRoot(createTemplatesStore)
  return store
}
