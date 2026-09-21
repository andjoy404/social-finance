import { useEffect, useRef } from 'react'

interface ModalProps {
  open: boolean
  onClose: () => void
  /** Width in px. Defaults to 520. */
  width?: number
  children: React.ReactNode
}

export function Modal({ open, onClose, width = 520, children }: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null)

  /* Close on Escape */
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open, onClose])

  /* Prevent body scroll while open */
  useEffect(() => {
    if (open) {
      const prev = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      return () => { document.body.style.overflow = prev }
    }
  }, [open])

  /* Move focus into modal when opened */
  useEffect(() => {
    if (open && dialogRef.current) {
      const first = dialogRef.current.querySelector<HTMLElement>(
        'input, button, select, textarea, [tabindex]:not([tabindex="-1"])'
      )
      first?.focus()
    }
  }, [open])

  if (!open) return null

  return (
    <div
      className="sf-modal-overlay"
      aria-modal="true"
      role="dialog"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        ref={dialogRef}
        className="sf-modal"
        style={{ width, maxWidth: 'calc(100vw - 32px)' }}
      >
        {children}
      </div>
    </div>
  )
}

export function ModalHeader({
  title,
  subtitle,
  onClose,
}: {
  title: string
  subtitle?: string
  onClose: () => void
}) {
  return (
    <div className="sf-modal-header">
      <div className="sf-modal-header-copy">
        <div className="sf-modal-title">{title}</div>
        {subtitle && <div className="sf-modal-subtitle">{subtitle}</div>}
      </div>
      <a
        href="#close"
        className="sf-modal-close"
        onClick={(e) => {
          e.preventDefault()
          onClose()
        }}
        aria-label="Tutup"
        title="Tutup (Escape)"
      >
        ×
      </a>
    </div>
  )
}

export function ModalBody({ children }: { children: React.ReactNode }) {
  return <div className="sf-modal-body">{children}</div>
}

export function ModalFooter({ children }: { children: React.ReactNode }) {
  return <div className="sf-modal-footer">{children}</div>
}

