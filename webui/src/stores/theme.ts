import { createSignal, createRoot, type Accessor } from 'solid-js'
import type { I18nKey } from '@/i18n/types'

export type ThemeMode = 'dark' | 'light'
export type ThemeColorId = 'indigo' | 'cyan' | 'mint' | 'violet'

export interface ThemeColorOption {
  id: ThemeColorId
  nameKey: I18nKey
  descKey: I18nKey
  primary: string
  secondary: string
  gradient: string
}

export const THEME_COLOR_OPTIONS: ThemeColorOption[] = [
  {
    id: 'indigo',
    nameKey: 'settings.appearance.themeColorIndigo',
    descKey: 'settings.appearance.themeColorIndigoDesc',
    primary: '#6366f1',
    secondary: '#00d3f2',
    gradient: 'from-[#6366f1] via-[#818cf8] to-[#8b5cf6]',
  },
  {
    id: 'cyan',
    nameKey: 'settings.appearance.themeColorCyan',
    descKey: 'settings.appearance.themeColorCyanDesc',
    primary: '#00d3f2',
    secondary: '#05df72',
    gradient: 'from-[#00d3f2] via-[#38bdf8] to-[#05df72]',
  },
  {
    id: 'mint',
    nameKey: 'settings.appearance.themeColorMint',
    descKey: 'settings.appearance.themeColorMintDesc',
    primary: '#05df72',
    secondary: '#00d3f2',
    gradient: 'from-[#05df72] via-[#10b981] to-[#00d3f2]',
  },
  {
    id: 'violet',
    nameKey: 'settings.appearance.themeColorViolet',
    descKey: 'settings.appearance.themeColorVioletDesc',
    primary: '#8b5cf6',
    secondary: '#6366f1',
    gradient: 'from-[#8b5cf6] via-[#a855f7] to-[#ec4899]',
  },
]



export interface TransitionOrigin {
  x: number
  y: number
}

export type ThemeToggleEvent = MouseEvent | TransitionOrigin | undefined

export interface ThemeStore {
  mode: Accessor<ThemeMode>
  color: Accessor<ThemeColorId>
  setMode: (mode: ThemeMode, event?: ThemeToggleEvent) => void
  toggleMode: (event?: ThemeToggleEvent) => void
  setColor: (color: ThemeColorId) => void
  colorOptions: ThemeColorOption[]
}

function safeGetItem(key: string): string | null {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      return window.localStorage.getItem(key)
    }
  } catch {}
  return null
}

function safeSetItem(key: string, value: string): void {
  try {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem(key, value)
    }
  } catch {}
}

function getStoredMode(): ThemeMode {
  if (typeof document === 'undefined') return 'dark'
  const saved = safeGetItem('subrefine-theme')
  if (saved === 'light' || saved === 'dark') return saved
  return document.documentElement?.classList?.contains('light') ? 'light' : 'dark'
}

function getStoredColor(): ThemeColorId {
  if (typeof document === 'undefined') return 'indigo'
  const saved = safeGetItem('subrefine-theme-color')
  if (saved === 'indigo' || saved === 'cyan' || saved === 'mint' || saved === 'violet') {
    return saved
  }
  return 'indigo'
}

function createThemeStoreInternal(): ThemeStore {
  const [mode, setModeSignal] = createSignal<ThemeMode>(getStoredMode())
  const [color, setColorSignal] = createSignal<ThemeColorId>(getStoredColor())

  function applyMode(newMode: ThemeMode) {
    setModeSignal(newMode)
    if (typeof document !== 'undefined' && document.documentElement) {
      document.documentElement.classList.toggle('light', newMode === 'light')
      safeSetItem('subrefine-theme', newMode)
    }
  }

  function setMode(newMode: ThemeMode, event?: ThemeToggleEvent) {
    if (newMode === mode()) return

    const hasDoc = typeof document !== 'undefined'
    const doc = hasDoc ? document : null
    const win = typeof window !== 'undefined' ? window : null

    const hasStartVT = Boolean(doc && 'startViewTransition' in doc && typeof doc.startViewTransition === 'function')
    const prefersReducedMotion = Boolean(win?.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches)

    if (!doc || !hasStartVT || prefersReducedMotion) {
      applyMode(newMode)
      return
    }

    let x = win ? win.innerWidth / 2 : 0
    let y = win ? win.innerHeight / 2 : 0

    if (event) {
      if ('currentTarget' in event && event.currentTarget instanceof HTMLElement) {
        const rect = event.currentTarget.getBoundingClientRect()
        x = rect.left + rect.width / 2
        y = rect.top + rect.height / 2
      } else if ('clientX' in event && typeof event.clientX === 'number' && (event.clientX !== 0 || event.clientY !== 0)) {
        x = event.clientX
        y = event.clientY
      } else if ('x' in event && typeof event.x === 'number') {
        x = event.x
        y = event.y
      }
    }

    const maxW = win ? Math.max(x, win.innerWidth - x) : x
    const maxH = win ? Math.max(y, win.innerHeight - y) : y
    const endRadius = Math.hypot(maxW, maxH)

    // Set origin coordinates and radius into root CSS variables for synchronous Frame-0 keyframe rendering
    doc.documentElement.style.setProperty('--theme-switch-x', `${x}px`)
    doc.documentElement.style.setProperty('--theme-switch-y', `${y}px`)
    doc.documentElement.style.setProperty('--theme-switch-r', `${endRadius}px`)

    const cleanup = () => {
      doc.documentElement.style.removeProperty('--theme-switch-x')
      doc.documentElement.style.removeProperty('--theme-switch-y')
      doc.documentElement.style.removeProperty('--theme-switch-r')
    }

    try {
      const transition = doc.startViewTransition(() => {
        applyMode(newMode)
      })

      transition.finished.then(cleanup, cleanup)
    } catch {
      cleanup()
      applyMode(newMode)
    }
  }

  function toggleMode(event?: ThemeToggleEvent) {
    setMode(mode() === 'light' ? 'dark' : 'light', event)
  }

  function setColor(newColor: ThemeColorId) {
    setColorSignal(newColor)
    if (typeof document !== 'undefined' && document.documentElement) {
      document.documentElement.dataset.themeColor = newColor
      safeSetItem('subrefine-theme-color', newColor)
    }
  }

  // Ensure initial dataset attribute is applied
  if (typeof document !== 'undefined' && document.documentElement) {
    document.documentElement.dataset.themeColor = color()
  }

  return {
    mode,
    color,
    setMode,
    toggleMode,
    setColor,
    colorOptions: THEME_COLOR_OPTIONS,
  }
}

let _themeStore: ThemeStore | null = null

export function useThemeStore(): ThemeStore {
  if (!_themeStore) {
    _themeStore = createRoot(createThemeStoreInternal)
  }
  return _themeStore
}
