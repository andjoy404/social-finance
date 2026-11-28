import { createContext, useCallback, useContext, useEffect, useRef } from 'react'
import type { ReactNode } from 'react'
import { useUnsavedChangesGuard } from '@/hooks/useUnsavedChangesGuard'

const ModalCloseContext = createContext<(() => void) | null>(null)

/**
 * Returns the nearest `<Modal>`'s guarded close handler (honoring `isDirty`
 * and `isSaving`). Falls back to [fallback] if rendered outside `<Modal>`.
 */
export function useModalClose(fallback: () => void): () => void {
  const ctxClose = useContext(ModalCloseContext)
  return ctxClose ?? fallback
}

interface ModalProps {
  open: boolean
  onClose: () => void
  /** Width in px. Defaults to 520. */
  width?: number
  /** Custom class for the overlay element. */
  overlayClassName?: string
  /** Whether the modal form currently has unsaved user changes. */
  isDirty?: boolean
  /** Whether the modal form is currently saving/uploading; blocks all close paths. */
  isSaving?: boolean
  children: React.ReactNode
}

export function Modal({
  open,
  onClose,
  width = 520,
  overlayClassName,
  isDirty = false,
  isSaving = false,
  children,
}: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const { guardedClose, confirmModal } = useUnsavedChangesGuard({
    active: open,
    isDirty,
    isSaving,
  })

  const handleRequestClose = useCallback(() => {
    void guardedClose(onClose)
  }, [guardedClose, onClose])

  /* Close on Escape */
  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        handleRequestClose()
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open, handleRequestClose])

  /* Prevent body scroll while open */
  useEffect(() => {
    if (open) {
      const prev = document.body.style.overflow
      document.body.style.overflow = 'hidden'
      return () => {
        document.body.style.overflow = prev
      }
    }
  }, [open])

  /* Move focus into modal when opened */
  useEffect(() => {
    if (open && dialogRef.current) {
      const first = dialogRef.current.querySelector<HTMLElement>(
        'input, button, select, textarea, [tabindex]:not([tabindex="-1"])',
      )
      first?.focus()
    }
  }, [open])

  if (!open) return null

  return (
    <ModalCloseContext.Provider value={handleRequestClose}>
      <div
        className={`sf-modal-overlay ${overlayClassName || ''}`.trim()}
        aria-modal="true"
        role="dialog"
        onClick={(e) => {
          if (e.target === e.currentTarget) handleRequestClose()
        }}
      >
        <div
          ref={dialogRef}
          className="sf-modal"
          style={{ width, maxWidth: 'calc(100vw - 32px)' }}
        >
          {children}
        </div>
      </div>
      {confirmModal}
    </ModalCloseContext.Provider>
  )
}

export function ModalHeader({
  title,
  subtitle,
  onClose,
  icon,
}: {
  title: string
  subtitle?: string
  onClose: () => void
  icon?: ReactNode
}) {
  const guardedModalClose = useContext(ModalCloseContext)

  return (
    <div className="sf-modal-header">
      {icon && <span className="sf-modal-header-icon">{icon}</span>}
      <div className="sf-modal-header-copy">
        {title && <div className="sf-modal-header-title">{title}</div>}
        {subtitle && <div className="sf-modal-subtitle">{subtitle}</div>}
      </div>
      <a
        href="#close"
        className="sf-modal-close"
        onClick={(e) => {
          e.preventDefault()
          if (guardedModalClose) {
            guardedModalClose()
          } else {
            onClose()
          }
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
