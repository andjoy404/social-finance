import { useEffect } from 'react'

interface CancelConfirmModalProps {
  open: boolean
  onStay: () => void
  onConfirmCancel: () => void
}

export function CancelConfirmModal({ open, onStay, onConfirmCancel }: CancelConfirmModalProps) {
  useEffect(() => {
    if (!open) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        e.stopImmediatePropagation()
        onStay()
      }
    }
    window.addEventListener('keydown', handleKeyDown, true)
    return () => window.removeEventListener('keydown', handleKeyDown, true)
  }, [open, onStay])

  if (!open) return null

  return (
    <div
      className="sf-modal-overlay"
      data-testid="unsaved-confirm-overlay"
      style={{
        zIndex: 2500,
        backgroundColor: 'rgba(0, 0, 0, 0.65)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="sf-cancel-confirm-title"
      aria-describedby="sf-cancel-confirm-desc"
      onClick={(e) => {
        e.stopPropagation()
      }}
    >
      <div
        className="sf-modal"
        style={{
          width: 440,
          maxWidth: 'calc(100vw - 32px)',
          padding: 24,
          borderRadius: 12,
          background: 'var(--sf-surface)',
          border: '1px solid var(--sf-border)',
          boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.5), 0 10px 10px -5px rgba(0, 0, 0, 0.2)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3
          id="sf-cancel-confirm-title"
          style={{
            margin: '0 0 8px',
            fontSize: 16,
            fontWeight: 600,
            color: 'var(--sf-text)',
          }}
        >
          Apakah Anda yakin ingin membatalkan?
        </h3>
        <p
          id="sf-cancel-confirm-desc"
          style={{
            margin: '0 0 20px',
            fontSize: 14,
            color: 'var(--sf-text-muted)',
            lineHeight: 1.5,
          }}
        >
          Perubahan yang belum disimpan akan hilang.
        </p>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
          <button
            type="button"
            className="sf-btn-outline"
            data-testid="unsaved-stay-btn"
            style={{
              height: 42,
              fontWeight: 600,
              fontSize: 13,
              borderRadius: 8,
              border: '1px solid var(--sf-border)',
              backgroundColor: 'var(--sf-surface)',
              color: 'var(--sf-text)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
            onClick={onStay}
          >
            Tetap di Halaman
          </button>
          <button
            type="button"
            data-testid="unsaved-confirm-btn"
            style={{
              height: 42,
              fontWeight: 600,
              fontSize: 13,
              borderRadius: 8,
              border: 'none',
              backgroundColor: 'var(--sf-danger, #EF4444)',
              color: '#ffffff',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
            onClick={onConfirmCancel}
          >
            Ya, Batalkan
          </button>
        </div>
      </div>
    </div>
  )
}
