import { useState, useCallback } from 'react'

const DEFAULTS: Record<string, number> = {
  'social-finance:table-page-size:warga': 10,
  'social-finance:table-page-size:rt': 10,
  'social-finance:table-page-size:iuran': 10,
}

export function usePersistedPageSize(storageKey: string, allowed: number[], initial?: number): [number, (v: number) => void] {
  const fallback = DEFAULTS[storageKey] ?? 10

  const [pageSize, setPageSizeState] = useState(() => {
    if (initial !== undefined && Number.isFinite(initial) && (allowed as readonly number[]).includes(Math.round(initial))) {
      const validInitial = Math.round(initial)
      try {
        localStorage.setItem(storageKey, String(validInitial))
      } catch {
        /* storage full or unavailable */
      }
      return validInitial
    }
    try {
      const raw = localStorage.getItem(storageKey)
      if (!raw) return fallback
      const parsed = Number(raw)
      if (!Number.isFinite(parsed)) return fallback
      if (!(allowed as readonly number[]).includes(Math.round(parsed))) return fallback
      return Math.round(parsed)
    } catch {
      return fallback
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
