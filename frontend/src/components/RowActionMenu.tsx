import { useState, useRef, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { MoreOutlined } from '@ant-design/icons'

interface RowActionMenuItem {
  label: string
  onClick: () => void
}

interface RowActionMenuProps {
  items: RowActionMenuItem[]
}

export function RowActionMenu({ items }: RowActionMenuProps) {
  const [open, setOpen] = useState(false)
  const [menuPosition, setMenuPosition] = useState<{ top: number; right: number } | null>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  const handleOpen = useCallback(() => {
    setOpen((prev) => {
      if (prev) return false
      const rect = triggerRef.current?.getBoundingClientRect()
      if (rect) {
        setMenuPosition({ top: rect.bottom + 4, right: window.innerWidth - rect.right })
      }
      return true
    })
  }, [])

  useEffect(() => {
    if (!open) return
    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node
      if (
        !triggerRef.current?.contains(target) &&
        !menuRef.current?.contains(target)
      ) {
        setOpen(false)
      }
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    const handleScroll = () => {
      const rect = triggerRef.current?.getBoundingClientRect()
      if (rect) {
        setMenuPosition({ top: rect.bottom + 4, right: window.innerWidth - rect.right })
      }
    }
    window.addEventListener('scroll', handleScroll, { passive: true })
    window.addEventListener('resize', handleScroll, { passive: true })
    return () => {
      window.removeEventListener('scroll', handleScroll)
      window.removeEventListener('resize', handleScroll)
    }
  }, [open])

  return (
    <div className="sf-row-action">
      <button
        ref={triggerRef}
        type="button"
        className={`sf-row-action-trigger ${open ? 'sf-row-action-trigger-open' : ''}`}
        onClick={handleOpen}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Aksi"
        title="Aksi"
      >
        <MoreOutlined />
      </button>

      {open && menuPosition && createPortal(
        <div
          ref={menuRef}
          className="sf-row-action-menu"
          role="menu"
          style={{
            position: 'fixed',
            top: menuPosition.top,
            right: menuPosition.right,
          }}
        >
          {items.map((item) => (
            <button
              key={item.label}
              type="button"
              className="sf-row-action-item"
              role="menuitem"
              onClick={() => {
                item.onClick()
                setOpen(false)
              }}
            >
              {item.label}
            </button>
          ))}
        </div>,
        document.body
      )}
    </div>
  )
}
