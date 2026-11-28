import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { CancelConfirmModal } from '@/components/CancelConfirmModal'

export interface GuardEntry {
  isDirty: () => boolean
  isSaving: () => boolean
}

export interface UnsavedChangesContextValue {
  registerForm: (id: string, entry: GuardEntry) => () => void
  notifyStateChanged: () => void
  isAnyDirty: () => boolean
  isAnySaving: () => boolean
  requestConfirm: () => Promise<boolean>
  confirmNavigation: (action: () => void) => Promise<boolean>
  markApprovedNavigation: () => void
}

const UnsavedChangesContext = createContext<UnsavedChangesContextValue | null>(null)

const GUARD_SENTINEL_KEY = '__sfUnsavedChangesSentinel'

export function UnsavedChangesProvider({ children }: { children: ReactNode }) {
  const entriesRef = useRef<Map<string, GuardEntry>>(new Map())
  const [confirmOpen, setConfirmOpen] = useState(false)
  const activeConfirmRef = useRef<{
    promise: Promise<boolean>
    resolve: (value: boolean) => void
  } | null>(null)
  const approvedPopRef = useRef(false)
  const sentinelPushedRef = useRef(false)
  const [, setVersion] = useState(0)

  const isAnySaving = useCallback(() => {
    for (const entry of entriesRef.current.values()) {
      if (entry.isSaving()) return true
    }
    return false
  }, [])

  const isAnyDirty = useCallback(() => {
    for (const entry of entriesRef.current.values()) {
      if (entry.isDirty()) return true
    }
    return false
  }, [])

  const syncHistorySentinel = useCallback(() => {
    if (typeof window === 'undefined' || !window.history?.pushState) return
    const shouldGuard = isAnyDirty() || isAnySaving()
    const currentState = window.history.state as Record<string, unknown> | null
    const hasSentinel = Boolean(currentState && currentState[GUARD_SENTINEL_KEY])
    if (shouldGuard && !hasSentinel && !sentinelPushedRef.current) {
      try {
        window.history.pushState(
          { ...(currentState || {}), [GUARD_SENTINEL_KEY]: true },
          '',
          window.location.href,
        )
        sentinelPushedRef.current = true
      } catch {
        // Ignore security errors in restricted test environments.
      }
    } else if (!shouldGuard && sentinelPushedRef.current) {
      sentinelPushedRef.current = false
    }
  }, [isAnyDirty, isAnySaving])

  const notifyStateChanged = useCallback(() => {
    syncHistorySentinel()
    setVersion((v) => v + 1)
  }, [syncHistorySentinel])

  const registerForm = useCallback(
    (id: string, entry: GuardEntry) => {
      entriesRef.current.set(id, entry)
      syncHistorySentinel()
      return () => {
        entriesRef.current.delete(id)
        syncHistorySentinel()
      }
    },
    [syncHistorySentinel],
  )

  const resolveConfirm = useCallback((confirmed: boolean) => {
    setConfirmOpen(false)
    const active = activeConfirmRef.current
    activeConfirmRef.current = null
    active?.resolve(confirmed)
  }, [])

  const requestConfirm = useCallback((): Promise<boolean> => {
    if (activeConfirmRef.current) {
      return activeConfirmRef.current.promise
    }
    let resolveFn!: (value: boolean) => void
    const promise = new Promise<boolean>((resolve) => {
      resolveFn = resolve
    })
    activeConfirmRef.current = { promise, resolve: resolveFn }
    setConfirmOpen(true)
    return promise
  }, [])

  const markApprovedNavigation = useCallback(() => {
    approvedPopRef.current = true
    sentinelPushedRef.current = false
    window.setTimeout(() => {
      approvedPopRef.current = false
    }, 300)
  }, [])

  const confirmNavigation = useCallback(
    async (action: () => void): Promise<boolean> => {
      if (isAnySaving()) return false
      if (!isAnyDirty()) {
        action()
        return true
      }
      const confirmed = await requestConfirm()
      if (!confirmed) return false
      markApprovedNavigation()
      action()
      return true
    },
    [isAnyDirty, isAnySaving, requestConfirm, markApprovedNavigation],
  )

  // Browser tab close / refresh guard
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isAnyDirty() || isAnySaving()) {
        e.preventDefault()
        e.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [isAnyDirty, isAnySaving])

  // Browser back/forward guard via history sentinel
  useEffect(() => {
    const handlePopState = () => {
      if (approvedPopRef.current) {
        approvedPopRef.current = false
        sentinelPushedRef.current = false
        return
      }
      if (isAnySaving()) {
        try {
          window.history.pushState(
            { ...(window.history.state || {}), [GUARD_SENTINEL_KEY]: true },
            '',
            window.location.href,
          )
          sentinelPushedRef.current = true
        } catch {
          /* ignore */
        }
        return
      }
      if (isAnyDirty()) {
        try {
          window.history.pushState(
            { ...(window.history.state || {}), [GUARD_SENTINEL_KEY]: true },
            '',
            window.location.href,
          )
          sentinelPushedRef.current = true
        } catch {
          /* ignore */
        }
        void requestConfirm().then((confirmed) => {
          if (confirmed) {
            approvedPopRef.current = true
            sentinelPushedRef.current = false
            window.history.go(-2)
          }
        })
      } else {
        sentinelPushedRef.current = false
      }
    }

    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [isAnyDirty, isAnySaving, requestConfirm])

  const value = useMemo<UnsavedChangesContextValue>(
    () => ({
      registerForm,
      notifyStateChanged,
      isAnyDirty,
      isAnySaving,
      requestConfirm,
      confirmNavigation,
      markApprovedNavigation,
    }),
    [
      registerForm,
      notifyStateChanged,
      isAnyDirty,
      isAnySaving,
      requestConfirm,
      confirmNavigation,
      markApprovedNavigation,
    ],
  )

  return (
    <UnsavedChangesContext.Provider value={value}>
      {children}
      <CancelConfirmModal
        open={confirmOpen}
        onStay={() => resolveConfirm(false)}
        onConfirmCancel={() => resolveConfirm(true)}
      />
    </UnsavedChangesContext.Provider>
  )
}

export function useUnsavedChangesContext(): UnsavedChangesContextValue | null {
  return useContext(UnsavedChangesContext)
}
