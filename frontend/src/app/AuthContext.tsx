import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react'
import { apiLogin, apiMe, apiRefresh, apiLogout, setTokenManager, clearSessionPair, getSessionPair, persistSessionPair, type ApiUser } from '@/app/api'

export { ApiError } from '@/app/api'

export interface AuthUser {
  id: string
  name: string
  email: string
  systemRole: string | null
  role: string
  rt: { id: string; name: string } | null
}

function mapUser(raw: ApiUser): AuthUser {
  return {
    id: raw.id,
    name: raw.full_name,
    email: raw.email,
    systemRole: raw.system_role ?? null,
    role: raw.role,
    rt: raw.rt ?? null,
  }
}

interface AuthContextValue {
  user: AuthUser | null
  isAuthenticated: boolean
  isInitializing: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  isAuthenticated: false,
  isInitializing: true,
  login: async () => {},
  logout: () => {},
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
    setUser(mapUser(resp.user))
    persistSessionPair({ accessToken: resp.access_token, refreshToken: resp.refresh_token })
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

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, isInitializing, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => useContext(AuthContext)
