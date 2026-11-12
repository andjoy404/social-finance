import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react'
import { apiLogin, apiMe, apiRefresh, apiLogout, setTokenManager, clearSessionPair, getSessionPair, persistSessionPair, type ApiUser } from '@/app/api'

export { ApiError } from '@/app/api'

export type RWMembershipSource = 'explicit' | 'rt_inferred'

export interface RWMembership {
  rw_id: string
  rw_name: string
  rw_number?: number
  jabatan: string
  source?: RWMembershipSource
}

export interface AuthUser {
  id: string
  name: string
  email: string
  phone?: string | null
  avatarPath?: string | null
  systemRole: string | null
  role: string
  jabatan: string | null
  effectivePermissions?: string[]
  rt: { id: string; name: string } | null
  rtCode?: string | null
  rwName?: string | null
  rwNumber?: number | null
  rtId?: string | null
  rwId?: string | null
  rwMemberships?: RWMembership[]
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

/**
 * Check whether a user has full RW write access for a specific RW.
 *
 * Checks only the membership for the given `rwId`. Returns `true` if that
 * membership has `source === 'explicit'` AND jabatan is ketua or bendahara,
 * or if the user is super_admin.
 * Returns `false` for other pengurus positions (wakil_ketua, sekretaris),
 * when the membership is `source === 'rt_inferred'`, or when no matching
 * membership is found.
 *
 * Super Admin always gets `true`.
 */
export function hasRwWriteAccess(
  user: AuthUser | null | undefined,
  rwId?: string | null,
): boolean {
  if (!user) return false
  if (user.systemRole === 'super_admin' || user.role === 'super_admin') return true
  const memberships = user.rwMemberships ?? []
  if (!rwId) return false
  const membership = memberships.find((m) => m.rw_id === rwId)
  if (!membership) return false
  if (membership.source !== 'explicit') return false
  const cleanJabatan = (membership.jabatan || '').trim().toLowerCase()
  return cleanJabatan === 'ketua' || cleanJabatan === 'bendahara'
}

/**
 * Check whether a user has access to RW sub-features (KAS, Lokasi Dana, Notifikasi RW).
 *
 * Only explicit RW members (any valid jabatan) and Super Admin have access.
 * RT-inferred members are strictly limited to Beranda RW (dashboard).
 */
export function hasRwFeatureAccess(
  user: AuthUser | null | undefined,
  rwId?: string | null,
): boolean {
  if (!user) return false
  if (user.systemRole === 'super_admin' || user.role === 'super_admin') return true
  const memberships = user.rwMemberships ?? []
  if (!rwId) return false
  const membership = memberships.find((m) => m.rw_id === rwId)
  if (!membership) return false
  return membership.source === 'explicit'
}

export function canAccessNotificationAdmin(user: AuthUser | null | undefined): boolean {
  return isSuperAdminUser(user) ||
    (user?.effectivePermissions ?? []).includes('notification.manage')
}

export function canAccessNotificationRead(user: AuthUser | null | undefined): boolean {
  return isSuperAdminUser(user) ||
    (user?.effectivePermissions ?? []).includes('notification.read')
}

export function canAccessNotificationMenu(user: AuthUser | null | undefined): boolean {
  return !!user && (canAccessNotificationAdmin(user) || canAccessNotificationRead(user))
}

/**
 * Check whether a user holds a specific permission. Super Admin passes.
 * Jabatan-based users are evaluated against their effective_permissions
 * (resolved by the backend from position_permissions — the single source
 * of truth). Role-only users without a jabatan never gain community
 * permissions implicitly.
 */
export function hasPermission(user: AuthUser | null | undefined, permission: string): boolean {
  if (!user) return false
  if (isSuperAdminUser(user)) return true
  return (user.effectivePermissions ?? []).includes(permission)
}

/** Activities management: create, update, publish, cancel, or delete drafts. */
export function canManageActivities(user: AuthUser | null | undefined): boolean {
  return hasPermission(user, 'activities.create') || hasPermission(user, 'activities.update')
}

/** Announcements management: create, update, publish, or archive/delete. */
export function canManageAnnouncements(user: AuthUser | null | undefined): boolean {
  return hasPermission(user, 'announcements.create') || hasPermission(user, 'announcements.update')
}

/**
 * Coarse RT-membership gate used only for UX routing; the backend enforces
 * actual authorization (active RT, configured scope fail-closed, jabatan and
 * permission checks). Any user with an RT membership in /me is allowed to
 * enter the RT-scoped area; the backend still decides what they can do.
 */
export function isAllowedRT(user: AuthUser | null): boolean {
  if (!user) return false
  if (isSuperAdminUser(user)) return true
  if (typeof user.rtId === 'string' && user.rtId.trim() !== '') return true
  if (typeof user.rt?.id === 'string' && user.rt.id.trim() !== '') return true
  return false
}

function mapUser(raw: ApiUser): AuthUser {
  const rtCode = raw.rt_rt ?? (raw.rt as { rt?: string } | null)?.rt ?? null
  const rtId = raw.rt_id ?? raw.rt?.id ?? null
  const rwId = raw.rw_id ?? null
  if (rtId && typeof sessionStorage !== 'undefined') {
    try {
      sessionStorage.setItem('sf_active_rt_id', rtId)
    } catch { /* ignore */ }
  }
  const rwMemberships: RWMembership[] = Array.isArray(raw.rw_memberships)
    ? raw.rw_memberships.map((m) => ({
        rw_id: m.rw_id,
        rw_name: m.rw_name,
        rw_number: m.rw_number,
        jabatan: m.jabatan,
        source: m.source === 'explicit' || m.source === 'rt_inferred'
          ? (m.source as RWMembershipSource)
          : undefined,
      }))
    : []
  return {
    id: raw.id,
    name: raw.full_name,
    email: raw.email,
    phone: raw.phone ?? null,
    avatarPath: raw.avatar_path ?? null,
    systemRole: raw.system_role ?? null,
    role: raw.role,
    jabatan: raw.jabatan ?? null,
    effectivePermissions: raw.effective_permissions ?? [],
    rt: raw.rt ?? (raw.rt_id ? { id: raw.rt_id, name: raw.rt_name ?? '' } : null),
    rtCode,
    rwName: raw.rw_name ?? null,
    rwNumber: raw.rw_number ?? raw.rt_rw ?? null,
    rtId,
    rwId,
    rwMemberships,
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
