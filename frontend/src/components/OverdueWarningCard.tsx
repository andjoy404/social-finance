import { useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { WarningOutlined, ArrowRightOutlined } from '@ant-design/icons'
import { useAuth } from '@/app/AuthContext'
import {
  apiGetOverdueWarningSummary,
  getSessionPair,
  IURAN_DATA_UPDATED_EVENT,
  type ApiOverdueWarningSummary,
} from '@/app/api'
import { formatRupiah } from '@/features/iuran/iuranTypes'

const FOCUS_REFRESH_THROTTLE_MS = 2000

/**
 * Global overdue warning based on the authenticated user's OWN household bills.
 * The summary is personal for every user (warga, pengurus, ketua, sekretaris,
 * bendahara — regardless of role or permission). The backend resolves the
 * occupancy/bills from the validated AuthContext and never aggregates across
 * households or RTs.
 */
export function OverdueWarningCard() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [summary, setSummary] = useState<ApiOverdueWarningSummary | null>(null)
  const requestSeqRef = useRef(0)
  const lastFetchedAtRef = useRef(0)

  const userId = user?.id ?? null
  const rtId = user?.rtId ?? user?.rt?.id ?? null
  const hasUser = !!userId

  const fetchSummary = useCallback(
    async (force = false) => {
      if (!hasUser || !userId) {
        setSummary(null)
        return
      }
      const now = Date.now()
      if (!force && now - lastFetchedAtRef.current < FOCUS_REFRESH_THROTTLE_MS) {
        return
      }
      const token = getSessionPair()?.accessToken
      if (!token) {
        setSummary(null)
        return
      }

      lastFetchedAtRef.current = now
      const reqId = ++requestSeqRef.current
      try {
        const res = await apiGetOverdueWarningSummary(token)
        if (reqId === requestSeqRef.current) {
          setSummary(res)
        }
      } catch {
        // Fail silently: never render a broken warning card on network/API errors
        if (reqId === requestSeqRef.current) {
          setSummary((prev) => prev)
        }
      }
    },
    [hasUser, userId, rtId],
  )

  // Initial load on login / user or RT context change
  useEffect(() => {
    if (!hasUser || !userId) {
      setSummary(null)
      return
    }
    void fetchSummary(true)
  }, [hasUser, userId, rtId, fetchSummary])

  // Refresh on window focus / visibilitychange and on explicit iuran data updates
  useEffect(() => {
    if (!hasUser || !userId) return

    const handleVisibilityOrFocus = () => {
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') {
        return
      }
      void fetchSummary(false)
    }

    const handleIuranDataUpdated = () => {
      void fetchSummary(true)
    }

    window.addEventListener('focus', handleVisibilityOrFocus)
    document.addEventListener('visibilitychange', handleVisibilityOrFocus)
    window.addEventListener(IURAN_DATA_UPDATED_EVENT, handleIuranDataUpdated)

    return () => {
      window.removeEventListener('focus', handleVisibilityOrFocus)
      document.removeEventListener('visibilitychange', handleVisibilityOrFocus)
      window.removeEventListener(IURAN_DATA_UPDATED_EVENT, handleIuranDataUpdated)
    }
  }, [hasUser, userId, fetchSummary])

  if (
    !hasUser ||
    !summary ||
    !summary.has_overdue_3_months ||
    summary.total_outstanding <= 0 ||
    summary.overdue_bill_count <= 0
  ) {
    return null
  }

  return (
    <section
      role="region"
      aria-label="Peringatan Tunggakan Iuran"
      data-testid="overdue-warning-card"
      style={{
        marginBottom: 16,
        padding: '14px 18px',
        borderRadius: 10,
        background: 'rgba(239, 68, 68, 0.08)',
        border: '1px solid rgba(239, 68, 68, 0.45)',
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 14,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12, flex: '1 1 320px', minWidth: 0 }}>
        <div
          style={{
            width: 36,
            height: 36,
            borderRadius: 8,
            background: 'rgba(239, 68, 68, 0.16)',
            color: '#dc2626',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
            marginTop: 2,
          }}
        >
          <WarningOutlined style={{ fontSize: 18 }} />
        </div>

        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8, marginBottom: 4 }}>
            <span style={{ fontWeight: 700, fontSize: 14, color: 'var(--sf-text-primary, #1f2937)' }}>
              Peringatan Tunggakan Iuran
            </span>
            <span
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                padding: '2px 8px',
                borderRadius: 999,
                fontSize: 11,
                fontWeight: 600,
                background: 'rgba(239, 68, 68, 0.14)',
                color: '#b91c1c',
                border: '1px solid rgba(239, 68, 68, 0.35)',
              }}
            >
              Tunggakan 3 Bulan+
            </span>
          </div>

          <p
            style={{
              margin: '0 0 8px 0',
              fontSize: 13,
              lineHeight: 1.45,
              color: 'var(--sf-text-secondary, #4b5563)',
            }}
          >
            Anda memiliki tagihan iuran yang belum lunas selama 3 bulan atau lebih. Mohon segera lakukan pembayaran agar kewajiban iuran tidak semakin menumpuk.
          </p>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: 12,
              fontSize: 13,
            }}
          >
            <span style={{ color: 'var(--sf-text-secondary, #4b5563)' }}>
              Total Tunggakan:{' '}
              <strong style={{ color: '#b91c1c', fontWeight: 700 }}>
                {formatRupiah(summary.total_outstanding)}
              </strong>
            </span>
            <span style={{ color: 'var(--sf-border, #d1d5db)' }}>•</span>
            <span style={{ fontWeight: 600, color: 'var(--sf-text-primary, #374151)' }}>
              {summary.overdue_bill_count} tagihan
            </span>
          </div>
        </div>
      </div>

      <div style={{ flexShrink: 0, display: 'flex', alignItems: 'center' }}>
        <button
          type="button"
          onClick={() => navigate('/iuran?tab=tagihan')}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 6,
            padding: '8px 14px',
            borderRadius: 8,
            border: '1px solid #dc2626',
            background: '#dc2626',
            color: '#ffffff',
            fontWeight: 600,
            fontSize: 13,
            cursor: 'pointer',
            whiteSpace: 'nowrap',
          }}
        >
          <span>Lihat Tagihan Saya</span>
          <ArrowRightOutlined style={{ fontSize: 12 }} />
        </button>
      </div>
    </section>
  )
}
