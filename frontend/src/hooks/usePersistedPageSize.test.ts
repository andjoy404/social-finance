import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { usePersistedPageSize } from './usePersistedPageSize'

const ALLOWED = [10, 25, 50]
const KEY = 'social-finance:table-page-size:test'

describe('usePersistedPageSize', () => {
  beforeEach(() => {
    Object.keys(localStorage).forEach(k => delete localStorage[k])
    vi.clearAllMocks()
  })

  function mockLocalStorage(items: Record<string, string>) {
    Object.entries(items).forEach(([k, v]) => {
      Object.defineProperty(localStorage, k, { value: v, writable: true })
    })
  }

  describe('no stored preference', () => {
    it('returns default 10 when no storage value exists', () => {
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(10)
    })
  })

  describe('stored valid value', () => {
    it('initializes with 25', () => {
      mockLocalStorage({ [KEY]: '25' })
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(25)
    })

    it('initializes with 50', () => {
      mockLocalStorage({ [KEY]: '50' })
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(50)
    })
  })

  describe('persistence', () => {
    it('writes to localStorage after set', () => {
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      act(() => { result.current[1](25) })
      expect(result.current[0]).toBe(25)
      expect(localStorage.getItem(KEY)).toBe('25')
    })

    it('restores value after unmount and remount to new hook', () => {
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      act(() => { result.current[1](50) })
      expect(result.current[0]).toBe(50)
      expect(localStorage.getItem(KEY)).toBe('50')
    })
  })

  describe('invalid stored value', () => {
    it('falls back to 10 for "abc"', () => {
      mockLocalStorage({ [KEY]: 'abc' })
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(10)
    })

    it('falls back to 10 for NaN string', () => {
      mockLocalStorage({ [KEY]: 'NaN' })
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(10)
    })
  })

  describe('unsupported value', () => {
    it('falls back to 10 for 999', () => {
      mockLocalStorage({ [KEY]: '999' })
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(10)
    })
  })

  describe('storage failure', () => {
    it('does not crash when localStorage.getItem throws', () => {
      const origGet = localStorage.getItem
      Object.defineProperty(localStorage, 'getItem', {
        value: () => { throw new Error('storage unavailable') },
        writable: true,
      })
      const { result } = renderHook(() => usePersistedPageSize(KEY, ALLOWED))
      expect(result.current[0]).toBe(10)
      Object.defineProperty(localStorage, 'getItem', { value: origGet, writable: true })
    })
  })

  describe('independent keys', () => {
    it('Warga and RT keys remain independent', () => {
      const wargaKey = 'social-finance:table-page-size:warga'
      const rtKey = 'social-finance:table-page-size:rt'

      mockLocalStorage({ [wargaKey]: '50', [rtKey]: '25' })

      const { result: resultWarga } = renderHook(() => usePersistedPageSize(wargaKey, ALLOWED))
      const { result: resultRt } = renderHook(() => usePersistedPageSize(rtKey, ALLOWED))

      expect(resultWarga.current[0]).toBe(50)
      expect(resultRt.current[0]).toBe(25)

      act(() => { resultWarga.current[1](10) })
      act(() => { resultRt.current[1](10) })

      expect(resultWarga.current[0]).toBe(10)
      expect(resultRt.current[0]).toBe(10)
      expect(localStorage.getItem(wargaKey)).toBe('10')
      expect(localStorage.getItem(rtKey)).toBe('10')
    })
  })
})
