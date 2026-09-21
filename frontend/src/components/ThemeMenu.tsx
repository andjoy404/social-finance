import { useState, useRef, useEffect } from 'react'
import { useTheme, type ThemeMode } from '@/app/ThemeProvider'
import {
  SunOutlined,
  MoonOutlined,
  DesktopOutlined,
} from '@ant-design/icons'

const themes: { value: ThemeMode; icon: React.ReactNode; label: string }[] = [
  { value: 'system', icon: <DesktopOutlined />, label: 'Ikuti Sistem' },
  { value: 'light', icon: <SunOutlined />, label: 'Terang' },
  { value: 'dark', icon: <MoonOutlined />, label: 'Gelap' },
]

export function ThemeMenu() {
  const { preference, setPreference } = useTheme()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  return (
    <div ref={ref} style={{ position: 'relative', display: 'inline-block' }}>
      <button
        className="header-btn"
        onClick={() => setOpen(!open)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        {preference === 'dark' ? <MoonOutlined /> : preference === 'light' ? <SunOutlined /> : <DesktopOutlined />}
      </button>

      {open && (
        <div
          role="listbox"
          style={{
            position: 'absolute',
            top: '100%',
            right: 0,
            marginTop: 4,
            background: 'var(--sf-surface)',
            border: '1px solid var(--sf-border)',
            borderRadius: 'var(--sf-radius-md)',
            boxShadow: 'var(--sf-shadow-sm)',
            minWidth: 160,
            zIndex: 100,
            overflow: 'hidden',
          }}
        >
          {themes.map((t) => (
            <button
              key={t.value}
              role="option"
              aria-selected={preference === t.value}
              onClick={() => {
                setPreference(t.value)
                setOpen(false)
              }}
              style={{
                display: 'flex',
                width: '100%',
                padding: '8px 14px',
                fontSize: 13,
                color: preference === t.value ? 'var(--sf-accent)' : 'var(--sf-text)',
                background: preference === t.value ? 'var(--sf-accent-soft)' : 'transparent',
                border: 'none',
                textAlign: 'left',
                cursor: 'pointer',
                alignItems: 'center',
                gap: 8,
              }}
            >
              {t.icon}
              {t.label}
              {preference === t.value && (
                <span style={{ marginLeft: 'auto', color: 'var(--sf-accent)', fontWeight: 600 }}>
                  {'\u2713'}
                </span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
