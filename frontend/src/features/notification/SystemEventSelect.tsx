import { type KeyboardEvent, useEffect, useId, useRef, useState } from 'react'
import { DownOutlined } from '@ant-design/icons'
import type { ApiSystemNotificationType } from '@/app/api'
import { TriggerCategoryBadge } from './TriggerCategoryBadge'

export interface SystemEventSelectProps {
  id?: string
  value: string
  options: ApiSystemNotificationType[]
  onChange: (value: string) => void
  disabled?: boolean
  ariaLabel?: string
  placeholder?: string
}

/**
 * Accessible listbox for picking a system notification event. Each option
 * renders the event name alongside its trigger-category badge.
 */
export function SystemEventSelect({
  id,
  value,
  options,
  onChange,
  disabled,
  ariaLabel,
  placeholder = 'Pilih jenis notifikasi',
}: SystemEventSelectProps) {
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const containerRef = useRef<HTMLDivElement | null>(null)
  const listRef = useRef<HTMLUListElement | null>(null)
  const generatedId = useId()
  const listId = `${id ?? generatedId}-listbox`
  const buttonId = `${id ?? generatedId}-button`

  useEffect(() => {
    if (!open) return
    const onDocMouseDown = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false)
    }
    const onDocKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDocMouseDown)
    document.addEventListener('keydown', onDocKeyDown)
    return () => {
      document.removeEventListener('mousedown', onDocMouseDown)
      document.removeEventListener('keydown', onDocKeyDown)
    }
  }, [open])

  const selected = options.find((o) => o.event_type === value)
  const selectedLabel = selected?.name ?? placeholder

  const commit = (next: string) => {
    onChange(next)
    setOpen(false)
    setActive(-1)
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return
    const lastIndex = options.length - 1
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault()
        if (!open) {
          setOpen(true)
          setActive(0)
          return
        }
        setActive((index) => Math.min(lastIndex, index + 1))
        return
      case 'ArrowUp':
        event.preventDefault()
        if (!open) {
          setOpen(true)
          setActive(Math.max(0, lastIndex))
          return
        }
        setActive((index) => Math.max(0, index - 1))
        return
      case 'Enter':
      case ' ':
        if (open && active >= 0 && active < options.length) {
          event.preventDefault()
          commit(options[active].event_type)
        } else {
          event.preventDefault()
          setOpen((prev) => !prev)
        }
        return
      case 'Escape':
        if (open) {
          event.preventDefault()
          setOpen(false)
        }
        return
      case 'Home':
        if (open) {
          event.preventDefault()
          setActive(0)
        }
        return
      case 'End':
        if (open) {
          event.preventDefault()
          setActive(lastIndex)
        }
        return
      default:
        return
    }
  }

  return (
    <div ref={containerRef} style={{ position: 'relative' }}>
      <button
        id={buttonId}
        type="button"
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        aria-label={ariaLabel}
        onClick={() => {
          setOpen((prev) => !prev)
          setActive(options.findIndex((o) => o.event_type === value))
        }}
        onKeyDown={handleKeyDown}
        style={{
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 8,
          padding: '8px 10px',
          borderRadius: 6,
          border: '1px solid var(--sf-border)',
          background: 'var(--sf-bg)',
          color: 'var(--sf-text)',
          fontSize: 13,
          cursor: disabled ? 'not-allowed' : 'pointer',
          textAlign: 'left',
        }}
      >
        <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {selectedLabel}
          </span>
          {selected && <TriggerCategoryBadge eventType={selected.event_type} />}
        </span>
        <DownOutlined style={{ fontSize: 12 }} aria-hidden="true" />
      </button>
      {open && (
        <ul
          id={listId}
          ref={listRef}
          role="listbox"
          aria-labelledby={buttonId}
          style={{
            position: 'absolute',
            zIndex: 1000,
            listStyle: 'none',
            margin: 0,
            padding: 4,
            maxHeight: 280,
            overflowY: 'auto',
            width: '100%',
            background: 'var(--sf-card-bg, var(--sf-bg))',
            border: '1px solid var(--sf-border)',
            borderRadius: 6,
            boxShadow: '0 8px 24px rgba(0,0,0,0.14)',
          }}
        >
          {options.length === 0 && (
            <li
              role="option"
              aria-selected={false}
              style={{ padding: '8px 10px', fontSize: 13, color: 'var(--sf-text-muted)' }}
            >
              Tidak ada jenis notifikasi tersedia
            </li>
          )}
          {options.map((option, index) => {
            const isSelected = option.event_type === value
            const isActive = index === active
            return (
              <li
                key={option.event_type}
                role="option"
                aria-selected={isSelected}
                tabIndex={-1}
                ref={(el) => {
                  if (el && isActive) el.scrollIntoView({ block: 'nearest' })
                }}
                onMouseEnter={() => setActive(index)}
                onClick={() => commit(option.event_type)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 8,
                  padding: '8px 10px',
                  borderRadius: 4,
                  cursor: 'pointer',
                  background: isActive ? 'rgba(87, 148, 242, 0.14)' : 'transparent',
                  color: 'var(--sf-text)',
                  fontSize: 13,
                }}
              >
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {option.name}
                </span>
                <TriggerCategoryBadge eventType={option.event_type} />
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
