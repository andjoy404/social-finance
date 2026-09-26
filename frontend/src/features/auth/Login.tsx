import { useState, useEffect } from 'react'
import { useAuth } from '@/app/AuthContext'
import { useNavigate } from 'react-router-dom'
import { ApiError } from '@/app/api'
import { MailOutlined, LockOutlined, EyeOutlined, EyeInvisibleOutlined } from '@ant-design/icons'
import logoAnimated from '@/assets/logo-animated.gif'
import logoStatic from '@/assets/logo.png'

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(() =>
    typeof window !== 'undefined' && window.matchMedia
      ? window.matchMedia('(prefers-reduced-motion: reduce)').matches
      : false
  )

  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return
    const query = window.matchMedia('(prefers-reduced-motion: reduce)')
    const onChange = (event: MediaQueryListEvent) => setReduced(event.matches)
    setReduced(query.matches)
    query.addEventListener('change', onChange)
    return () => query.removeEventListener('change', onChange)
  }, [])

  return reduced
}

export function Login() {
  const { login, isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const reducedMotion = usePrefersReducedMotion()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  if (isAuthenticated) {
    navigate('/', { replace: true })
    return null
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!email || !password) {
      setError('Email dan kata sandi harus diisi.')
      return
    }
    setLoading(true)
    setError('')
    try {
      await login(email, password)
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        switch (err.code) {
          case 'invalid_credentials':
            setError('Email atau kata sandi salah.')
            break
          case 'not_authorized':
            setError('Akun tidak memiliki akses aktif.')
            break
          case 'multiple_memberships':
            setError('Akun memiliki lebih dari satu keanggotaan aktif.')
            break
          case 'rate_limited':
            setError('Terlalu banyak percobaan masuk. Silakan coba lagi nanti.')
            break
          default:
            setError('Tidak dapat terhubung ke layanan. Silakan coba lagi.')
            break
        }
      } else {
        setError('Tidak dapat terhubung ke layanan. Silakan coba lagi.')
      }
    } finally {
      setLoading(false)
    }
  }

  const noFields = !email && !password && !error

  return (
    <div className="login-page">
      <div className="login-stand">
        {/* Brand lockup */}
        <div className="login-brand-lockup">
          <div className="login-logo">
            {reducedMotion ? (
              <img
                className="login-logo-static"
                src={logoStatic}
                alt="Social Finance Logo"
                width={104}
                height={104}
              />
            ) : (
              <img
                className="login-logo-animated"
                src={logoAnimated}
                alt="Social Finance Logo"
                width={104}
                height={104}
              />
            )}
          </div>
          <div className="login-product-name">Social Finance</div>
          <span className="login-tagline">Social Finance for Your Neighborhood</span>
        </div>

        {/* Login card */}
        <form onSubmit={handleSubmit} className="login-card">
          <div className="login-card-head">
            <h3>Masuk ke Akun Anda</h3>
          </div>

          {error && (
            <div className="login-error" role="alert">
              {error}
            </div>
          )}

          {/* Email field */}
          <div className="login-field">
            <label htmlFor="email">Email</label>
            <div className="login-input-wrapper">
              <span className="login-input-icon" aria-hidden="true">
                <MailOutlined />
              </span>
              <input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="Masukkan email"
                required
                autoComplete="email"
                autoFocus={noFields}
                disabled={loading}
                className="login-input"
              />
            </div>
          </div>

          {/* Password field */}
          <div className="login-field">
            <label htmlFor="password">Kata Sandi</label>
            <div className="login-input-wrapper">
              <span className="login-input-icon" aria-hidden="true">
                <LockOutlined />
              </span>
              <input
                id="password"
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Masukkan kata sandi"
                required
                autoComplete="current-password"
                disabled={loading}
                className="login-input"
              />
              <button
                type="button"
                className="login-toggle-password"
                onClick={() => setShowPassword((v) => !v)}
                tabIndex={-1}
                aria-label={showPassword ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'}
              >
                {showPassword ? <EyeInvisibleOutlined /> : <EyeOutlined />}
              </button>
            </div>
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={loading}
            className="login-submit"
          >
            {loading ? 'Memproses...' : 'Masuk'}
          </button>
        </form>
      </div>
    </div>
  )
}
