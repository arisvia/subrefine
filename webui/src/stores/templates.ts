import { createRoot, createSignal } from 'solid-js'
import { api } from '@/lib/api'
import type { FamilyOptions, TemplateOptions } from '@/types/domain'

const EMPTY_FAMILY: FamilyOptions = { available: false, inbounds: [] }
const EMPTY: TemplateOptions = { singbox: EMPTY_FAMILY, clash: EMPTY_FAMILY }

export interface TemplatesStore {
  options: () => TemplateOptions
  load: () => Promise<void>
}

function createTemplatesStore(): TemplatesStore {
  const [options, setOptions] = createSignal<TemplateOptions>(EMPTY)

  async function load() {
    try {
      const raw = await api<Partial<TemplateOptions>>('/api/admin/templates')
      setOptions({
        singbox: { ...EMPTY_FAMILY, ...raw.singbox },
        clash: { ...EMPTY_FAMILY, ...raw.clash },
      })
    } catch {
      setOptions(EMPTY)
    }
  }

  return { options, load }
}

let store: TemplatesStore | undefined

export function useTemplatesStore(): TemplatesStore {
  if (!store) store = createRoot(createTemplatesStore)
  return store
}
