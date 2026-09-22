import { ConfigProvider, theme } from 'antd'
import { BrowserRouter } from 'react-router-dom'
import type { ReactNode } from 'react'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { AuthProvider } from '@/app/AuthContext'
import { ThemeProvider, useTheme } from '@/app/ThemeProvider'
import { App } from '@/app/App'
import '@/styles/global.css'
import '@/styles/login.css'

const antdBase = {
  token: {
    fontFamily: 'var(--sf-font-sans)',
    colorPrimary: '#7c5ac7',
    borderRadius: 8,
    colorBgContainer: '#ffffff',
    colorBgElevated: '#f4f5f7',
  },
  components: {
    Button: { algorithm: true },
  },
}

const antdDark = {
  ...antdBase,
  algorithm: [theme.darkAlgorithm],
  token: {
    ...antdBase.token,
    colorBgContainer: '#1a1a1a',
    colorBgElevated: '#141414',
  },
}

function ThemeAwareConfigProvider({ children }: { children: ReactNode }) {
  const { activeTheme } = useTheme()
  return (
    <ConfigProvider theme={activeTheme === 'dark' ? antdDark : antdBase}>
      {children}
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <ThemeProvider>
          <ThemeAwareConfigProvider>
            <App />
          </ThemeAwareConfigProvider>
        </ThemeProvider>
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
)
