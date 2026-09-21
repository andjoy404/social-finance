import { useState, useRef, useEffect } from 'react'
import { FilterOutlined, DownOutlined } from '@ant-design/icons'

export interface FilterOption<T extends string = string> {
  value: T
  label: string
}

interface FilterDropdownProps<T extends string = string> {
  value: T
  options: FilterOption<T>[]
  onChange: (value: T) => void
  ariaLabel?: string
}

export function FilterDropdown<T extends string = string>({
  value,
  options,
  onChange,
  ariaLabel = 'Filter status',
}: FilterDropdownProps<T>) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const isFiltered = Boolean(value && value !== 'semua')
  const triggerLabel = isFiltered ? 'Status' : 'Semua'

  useEffect(() => {
    if (!open) return
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
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

  return (
    <div className="sf-filter-dropdown" ref={containerRef}>
      <button
        type="button"
        className={`sf-filter-dropdown-trigger ${open ? 'sf-filter-dropdown-trigger-open' : ''} ${isFiltered ? 'sf-filter-dropdown-trigger-active' : ''}`}
        onClick={() => setOpen((prev) => !prev)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={`${ariaLabel}: ${triggerLabel}`}
      >
        <FilterOutlined style={{ fontSize: 12, color: 'var(--sf-accent)' }} />
        <span className="sf-filter-dropdown-label">{triggerLabel}</span>
        <DownOutlined
          style={{
            fontSize: 10,
            color: 'var(--sf-text-muted)',
            transition: 'transform 150ms ease',
            transform: open ? 'rotate(180deg)' : 'none',
          }}
        />
      </button>

      <div
        className={`sf-filter-dropdown-menu ${open ? 'sf-filter-dropdown-menu-open' : ''}`}
        role="listbox"
        aria-label={ariaLabel}
      >
        {options.map((opt) => (
          <button
            key={opt.value}
            type="button"
            className={`sf-filter-dropdown-item ${opt.value === value ? 'sf-filter-dropdown-item-active' : ''}`}
            onClick={() => {
              onChange(opt.value)
              setOpen(false)
            }}
            role="button"
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  )
}

