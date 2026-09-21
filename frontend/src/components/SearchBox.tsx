import React, { type ReactNode } from 'react'
import { SearchOutlined, CloseOutlined } from '@ant-design/icons'

interface SearchBoxProps {
  value: string
  onChange: (value: string) => void
  onSearch?: (value: string) => void
  placeholder?: string
  style?: React.CSSProperties
  className?: string
  statusControl?: ReactNode
}

export function SearchBox({
  value,
  onChange,
  onSearch,
  placeholder = 'Cari…',
  style,
  className: _className,
  statusControl,
}: SearchBoxProps) {
  return (
    <form
      className={`sf-search-box ${_className ?? ''}`}
      style={style}
      data-testid="search-box"
      onSubmit={(e) => {
        e.preventDefault()
        onSearch?.(value)
      }}
    >
      <button
        type="submit"
        className="sf-search-box-icon-btn"
        aria-label="Cari"
        title="Cari"
      >
        <SearchOutlined style={{ fontSize: 13, color: 'var(--sf-text-muted)' }} />
      </button>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="sf-search-box-input"
        data-testid="search-input"
      />
      {value && (
        <button
          type="button"
          className="sf-search-box-clear"
          onClick={() => {
            onChange('')
            onSearch?.('')
          }}
          data-testid="search-clear"
          aria-label="Bersihkan"
          title="Bersihkan"
        >
          <CloseOutlined style={{ fontSize: 13 }} />
        </button>
      )}
      {statusControl && <span className="sf-search-box-status-slot">{statusControl}</span>}
    </form>
  )
}
