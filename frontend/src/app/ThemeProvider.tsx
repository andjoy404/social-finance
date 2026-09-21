import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react'

export type ThemeMode = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'social-finance-theme'

function getPreferredTheme(): ThemeMode {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'system' || stored === 'light' || stored === 'dark') {
    return stored
  }
  return 'system'
}

function resolveActiveTheme(preference: ThemeMode): 'light' | 'dark' {
  if (preference === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return preference
}

type ThemeContextValue = {
  preference: ThemeMode
  activeTheme: 'light' | 'dark'
  setPreference: (p: ThemeMode) => void
}

const ThemeContext = createContext<ThemeContextValue>({
  preference: 'system',
  activeTheme: 'light',
  setPreference: () => {},
})

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreferenceRaw] = useState<ThemeMode>(getPreferredTheme)
  const [activeTheme, setActiveTheme] = useState<'light' | 'dark'>(() =>
    resolveActiveTheme(preference),
  )

  const setPreference = (p: ThemeMode) => {
    setPreferenceRaw(p)
    localStorage.setItem(STORAGE_KEY, p)
  }

  useEffect(() => {
    const theme = resolveActiveTheme(preference)
    setActiveTheme(theme)
    document.documentElement.setAttribute('data-theme', theme)
  }, [preference])

  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
      if (preference === 'system') {
        setActiveTheme(mq.matches ? 'dark' : 'light')
        document.documentElement.setAttribute('data-theme', mq.matches ? 'dark' : 'light')
      }
    }
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [preference])

  return (
    <ThemeContext.Provider value={{ preference, activeTheme, setPreference }}>
      {children}
    </ThemeContext.Provider>
  )
}

export const useTheme = () => useContext(ThemeContext)
