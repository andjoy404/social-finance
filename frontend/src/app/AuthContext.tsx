import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react'
import { apiLogin, apiMe, apiRefresh, apiLogout, setTokenManager, clearSessionPair, getSessionPair, persistSessionPair, type ApiUser } from '@/app/api'

export { ApiError } from '@/app/api'

export const ALLOWED_RT_CODES = ['002', '003']
export const ALLOWED_RT_IDS = [
  'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b', // RT 002
  '8dfa1e36-6727-45be-8fea-24d958efc155', // RT 003
]

export interface AuthUser {
  id: string
  name: string
  email: string
  phone?: string | null
  systemRole: string | null
  role: string
  jabatan: string | null
  effectivePermissions?: string[]
  rt: { id: string; name: string } | null
  rtCode?: string | null
  rtId?: string | null
}

export function isSuperAdminUser(user: AuthUser | null | undefined): boolean {
  if (!user) return false
  const sysRole = (user.systemRole ?? '').trim().toLowerCase()
  const tenantRole = (user.role ?? '').trim().toLowerCase()
  return sysRole === 'super_admin' || tenantRole === 'super_admin'
}

export function isKetuaRTUser(user: AuthUser | null | undefined): boolean {
  if (!user) return false
  const jabatan = (user.jabatan ?? '').trim().toLowerCase()
  return jabatan === 'ketua' || jabatan === 'ketua_rt' || jabatan === 'ketua rt'
}

export function canAccessNotificationMenu(user: AuthUser | null | undefined): boolean {
  return isSuperAdminUser(user) || isKetuaRTUser(user)
}

export function isAllowedRT(user: AuthUser | null): boolean {
  if (!user) return false
  if (isSuperAdminUser(user)) return true
  if (user.rtCode && ALLOWED_RT_CODES.includes(user.rtCode)) return true
  if (user.rtId && ALLOWED_RT_IDS.includes(user.rtId)) return true
  if (user.rt?.id && ALLOWED_RT_IDS.includes(user.rt.id)) return true
  return false
}

function mapUser(raw: ApiUser): AuthUser {
  const rtCode = raw.rt_rt ?? (raw.rt as { rt?: string } | null)?.rt ?? null
  const rtId = raw.rt_id ?? raw.rt?.id ?? null
  if (rtId && typeof sessionStorage !== 'undefined') {
    try {
      sessionStorage.setItem('sf_active_rt_id', rtId)
    } catch { /* ignore */ }
  }
  return {
    id: raw.id,
    name: raw.full_name,
    email: raw.email,
    phone: raw.phone ?? null,
    systemRole: raw.system_role ?? null,
    role: raw.role,
    jabatan: raw.jabatan ?? null,
    effectivePermissions: raw.effective_permissions ?? [],
    rt: raw.rt ?? (raw.rt_id ? { id: raw.rt_id, name: raw.rt_name ?? '' } : null),
    rtCode,
    rtId,
  }
}


interface AuthContextValue {
  user: AuthUser | null
  isAuthenticated: boolean
  isInitializing: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
  refreshUser?: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  isAuthenticated: false,
  isInitializing: true,
  login: async () => {},
  logout: () => {},
  refreshUser: async () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [isInitializing, setIsInitializing] = useState(true)
  const mountedRef = useRef<boolean>(true)

  useEffect(() => {
    return () => { mountedRef.current = false }
  }, [])

  const clearAll = useCallback(() => {
    setUser(null)
    clearSessionPair()
    try {
      sessionStorage.removeItem('sf_active_rt_id')
    } catch { /* ignore */ }
  }, [])

  useEffect(() => {
    const getToken = () => {
      const pair = getSessionPair()
      return pair?.accessToken ?? undefined
    }
    const getRefreshToken = () => {
      const pair = getSessionPair()
      return pair?.refreshToken ?? undefined
    }
    const setToken = (pair: { accessToken: string; refreshToken: string }) => {
      persistSessionPair(pair)
    }
    const clearToken = () => {
      clearSessionPair()
    }
    setTokenManager(getToken, getRefreshToken, setToken, clearToken)
  }, [])

  useEffect(() => {
    let cancelled = false
    const restore = async () => {
      const pair = getSessionPair()
      if (!pair) {
        setIsInitializing(false)
        return
      }

      try {
        const meResp = await apiMe(pair.accessToken)
        if (!cancelled) {
          setUser(mapUser(meResp))
          setIsInitializing(false)
        }
        return
      } catch {
        if (!pair.refreshToken || pair.refreshToken.length === 0) {
          if (!cancelled) {
            clearAll()
            setIsInitializing(false)
          }
          return
        }
        try {
          const refreshed = await apiRefresh(pair.refreshToken)
          if (cancelled) return
          persistSessionPair(refreshed)
          const meResp = await apiMe(refreshed.accessToken)
          if (!cancelled) {
            setUser(mapUser(meResp))
            setIsInitializing(false)
          }
        } catch {
          if (!cancelled) clearAll()
          setIsInitializing(false)
        }
      }
    }
    restore()
    return () => { cancelled = true }
  }, [clearAll])

  const login = useCallback(async (email: string, password: string) => {
    const resp = await apiLogin(email, password)
    persistSessionPair({ accessToken: resp.access_token, refreshToken: resp.refresh_token })
    try {
      const me = await apiMe(resp.access_token)
      setUser(mapUser(me))
    } catch {
      setUser(mapUser(resp.user))
    }
  }, [])

  const logout = useCallback(async () => {
    try {
      const token = getSessionPair()?.accessToken
      if (token && typeof token === 'string' && token.length > 0) {
        try {
          await apiLogout(token)
        } catch {
          /* best-effort — always clear locally below */
        }
      }
    } catch {
      /* unreachable but safe */
    }

    setUser(null)
    clearSessionPair()
    window.location.href = '/login'
  }, [])

  const refreshUser = useCallback(async () => {
    const pair = getSessionPair()
    if (pair?.accessToken) {
      const me = await apiMe(pair.accessToken)
      setUser(mapUser(me))
    }
  }, [])

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, isInitializing, login, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => useContext(AuthContext)
