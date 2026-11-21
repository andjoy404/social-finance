import { useEffect } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'

/**
 * Deep link fallback: /paket-detail?reportId=... redirects to
 * /home/notifikasi?openPaket=... so that NotificationScreen can
 * open the PaketDetailModal. This preserves browser history and
 * keeps the user in the notification context.
 */
export function PaketDetailPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const reportId = searchParams.get('reportId')

  useEffect(() => {
    if (reportId) {
      // Redirect to notification page with openPaket param
      navigate(`/home/notifikasi?openPaket=${encodeURIComponent(reportId)}`, { replace: true })
    } else {
      navigate('/home/notifikasi', { replace: true })
    }
  }, [reportId, navigate])

  return (
    <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>
      Memuat detail paket...
    </div>
  )
}
