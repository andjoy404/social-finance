import { useState, useCallback } from 'react'

const DEFAULTS: Record<string, number> = {
  'social-finance:table-page-size:warga': 10,
  'social-finance:table-page-size:rt': 10,
}

export function usePersistedPageSize(storageKey: string, allowed: number[], initial?: number): [number, (v: number) => void] {
  const defaultValue = initial ?? DEFAULTS[storageKey] ?? 10

  const [pageSize, setPageSizeState] = useState(() => {
    try {
      const raw = localStorage.getItem(storageKey)
      if (!raw) return defaultValue
      const parsed = Number(raw)
      if (!Number.isFinite(parsed)) return defaultValue
      if (!(allowed as readonly number[]).includes(Math.round(parsed))) return defaultValue
      return Math.round(parsed)
    } catch {
      return defaultValue
    }
  })

  const setPageSize = useCallback((v: number) => {
    setPageSizeState(v)
    try {
      localStorage.setItem(storageKey, String(v))
    } catch {
      /* storage full or unavailable — UI continues normally */
    }
  }, [storageKey])

  return [pageSize, setPageSize]
}
