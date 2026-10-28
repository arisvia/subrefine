import { createRoot, createSignal, type Accessor } from 'solid-js'
import { api, getAdminToken, setAdminToken, setOnUnauthorized, ApiError } from '@/lib/api'
import type { AdminInfo } from '@/types/domain'

export interface AuthStore {
  /** True once a token has been accepted by the server. */
  authenticated: Accessor<boolean>
  /** True after the first probe resolved (authenticated or not). */
  checked: Accessor<boolean>
  busy: Accessor<boolean>
  error: Accessor<string>
  info: Accessor<AdminInfo | null>
  hasToken: Accessor<boolean>
  login: (token: string) => Promise<boolean>
  logout: () => void
  probe: () => Promise<void>
}

function createAuthStore(): AuthStore {
  const [authenticated, setAuthenticated] = createSignal(false)
  const [checked, setChecked] = createSignal(false)
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal('')
  const [info, setInfo] = createSignal<AdminInfo | null>(null)
  const [hasToken, setHasToken] = createSignal(getAdminToken() !== '')

  // subrefine authenticates per request; a 401 means the stored token is stale
  // or wrong, so drop the session and let the shell show the login overlay.
  setOnUnauthorized(() => {
    setAuthenticated(false)
    setInfo(null)
  })

  async function fetchInfo(silent: boolean): Promise<AdminInfo | null> {
    try {
      return await api<AdminInfo>('/api/admin/info', { silent })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return null
      throw err
    }
  }

  async function login(token: string): Promise<boolean> {
    const trimmed = token.trim()
    if (!trimmed) return false
    setBusy(true)
    setError('')
    setAdminToken(trimmed)
    setHasToken(true)
    try {
      const res = await fetchInfo(false)
      if (res) {
        setInfo(res)
        setAuthenticated(true)
        return true
      }
      // Wrong token: clear it so the next request is anonymous again.
      setAdminToken('')
      setHasToken(false)
      setAuthenticated(false)
      setError('bad')
      return false
    } catch {
      // Network/API failure: keep the token, the caller surfaces the error toast.
      setError('network')
      return false
    } finally {
      setBusy(false)
      setChecked(true)
    }
  }

  function logout() {
    setAdminToken('')
    setHasToken(false)
    setAuthenticated(false)
    setInfo(null)
    setError('')
  }

  async function probe() {
    if (!getAdminToken()) {
      setChecked(true)
      return
    }
    try {
      const res = await fetchInfo(true)
      if (res) {
        setInfo(res)
        setAuthenticated(true)
      } else {
        setAdminToken('')
        setHasToken(false)
      }
    } catch {
      // Service unreachable: stay unauthenticated but keep the stored token.
    } finally {
      setChecked(true)
    }
  }

  return { authenticated, checked, busy, error, info, hasToken, login, logout, probe }
}

let store: AuthStore | undefined

export function useAuthStore(): AuthStore {
  if (!store) store = createRoot(createAuthStore)
  return store
}

export function resetAuthStoreForTesting(): void {
  store = undefined
}
