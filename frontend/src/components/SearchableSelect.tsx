import { useRef, useState, type ReactNode } from 'react'

interface Option {
  id: string
  display: string
}

interface SearchableSelectProps {
  id: string
  label: ReactNode
  required?: boolean
  options: Option[]
  value: string
  emptyMessage: string
  placeholder: string
  disabled?: boolean
  onChange: (value: string) => void
}

export function SearchableSelect({
  id,
  label,
  required,
  options,
  value,
  emptyMessage,
  placeholder,
  disabled,
  onChange,
}: SearchableSelectProps) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const listboxRef = useRef<HTMLUListElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const filtered = options.filter((o) =>
    o.display.toLowerCase().includes(query.toLowerCase()),
  )

  const handleOpen = () => {
    setOpen(true)
    setQuery('')
    setTimeout(() => inputRef.current?.focus(), 0)
  }

  const handleClose = () => {
    setOpen(false)
    setQuery('')
  }

  const handleSelect = (optId: string) => {
    onChange(optId)
    handleClose()
  }

  const [focusedIndex, setFocusedIndex] = useState(-1)

  const labelText = typeof label === 'string' ? label : 'RT Tujuan'

  return (
    <div style={{ marginBottom: 'var(--sf-sp-md)', position: 'relative', zIndex: 10 }}>
      <label
        style={{
          display: 'block',
          fontSize: '11px',
          fontWeight: 600,
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
          color: 'var(--sf-text-muted)',
          marginBottom: '6px',
        }}
      >
        {label}
        {required && <span style={{ color: 'var(--sf-danger)' }}> *</span>}
      </label>
      <div
        role="combobox"
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-owns={`${id}-listbox`}
        aria-controls={`${id}-listbox`}
        aria-label={labelText}
        style={{
          position: 'relative',
          width: '100%',
          borderRadius: 'var(--sf-radius-sm)',
          border: open ? '1px solid var(--sf-accent)' : '1px solid var(--sf-border)',
          fontSize: '13px',
          color: 'var(--sf-text)',
          background: 'var(--sf-bg)',
          cursor: disabled ? 'not-allowed' : 'pointer',
          opacity: disabled ? 0.5 : 1,
          overflow: 'visible',
        }}
        onClick={() => {
          if (!disabled) handleOpen()
        }}
        tabIndex={disabled ? undefined : 0}
        onKeyDown={(e) => {
          if (disabled) return
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            handleOpen()
          }
          if (e.key === 'ArrowDown' && !open) {
            e.preventDefault()
            handleOpen()
          }
          if (e.key === 'Escape') {
            e.preventDefault()
            handleClose()
          }
        }}
      >
        <input
          ref={inputRef}
          id={id}
          aria-autocomplete="none"
          value={
            open
              ? query
              : (value ? (options.find((o) => o.id === value)?.display ?? '') : '')
          }
          placeholder={placeholder}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.preventDefault()
              handleClose()
            }
            if (!open) return
            if (e.key === 'ArrowDown') {
              e.preventDefault()
              setFocusedIndex((prev) => Math.min(prev + 1, filtered.length - 1))
            } else if (e.key === 'ArrowUp') {
              e.preventDefault()
              setFocusedIndex((prev) => Math.max(prev - 1, 0))
            } else if (e.key === 'Enter') {
              e.preventDefault()
              if (focusedIndex >= 0 && focusedIndex < filtered.length) {
                handleSelect(filtered[focusedIndex].id)
              } else if (filtered.length === 1) {
                handleSelect(filtered[0].id)
              } else {
                handleClose()
              }
            }
          }}
          style={{
            width: '100%',
            padding: '8px 10px',
            border: 'none',
            outline: 'none',
            fontSize: '13px',
            color: 'var(--sf-text)',
            background: 'transparent',
            position: 'relative',
            zIndex: 2,
          }}
        />
        {open && (
          <ul
            ref={listboxRef}
            id={`${id}-listbox`}
            role="listbox"
            style={{
              listStyle: 'none',
              margin: 0,
              padding: 0,
              maxHeight: '200px',
              overflowY: 'auto',
              borderTop: '1px solid var(--sf-border)',
              position: 'absolute',
              left: 0,
              right: 0,
              zIndex: 20,
              background: 'var(--sf-surface)',
              border: '1px solid var(--sf-border)',
              borderRadius: 'var(--sf-radius-sm)',
              boxShadow: '0 4px 16px rgba(0, 0, 0, 0.5)',
              whiteSpace: 'nowrap',
              width: 'fit-content',
              maxWidth: 'calc(100vw - 16px)',
              minWidth: '100%',
              pointerEvents: 'none',
            }}
          >
            {filtered.length === 0 ? (
              <li
                role="option"
                aria-selected={false}
                style={{
                  padding: '8px 10px',
                  fontSize: '13px',
                  color: 'var(--sf-text-muted)',
                  pointerEvents: 'none',
                }}
              >
                {emptyMessage}
              </li>
            ) : (
              filtered.map((opt, idx) => (
                <li
                  key={opt.id}
                  role="option"
                  aria-selected={opt.id === value}
                  data-value={opt.id}
                  tabIndex={focusedIndex === idx ? 0 : -1}
                  onClick={(e) => {
                    e.stopPropagation()
                    handleSelect(opt.id)
                  }}
                  onMouseEnter={() => setFocusedIndex(idx)}
                  style={{
                    padding: '8px 10px',
                    fontSize: '13px',
                    cursor: 'pointer',
                    whiteSpace: 'nowrap',
                    background: idx === focusedIndex
                      ? 'var(--sf-surface-hover)'
                      : opt.id === value
                        ? 'var(--sf-accent-soft)'
                        : 'transparent',
                    color: 'var(--sf-text)',
                    pointerEvents: 'auto',
                  }}
                >
                  {opt.display}
                </li>
              ))
            )}
          </ul>
        )}
      </div>
    </div>
  )
}
