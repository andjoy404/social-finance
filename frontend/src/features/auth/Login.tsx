import { useState, useEffect } from 'react'
import { useAuth } from '@/app/AuthContext'
import { useNavigate } from 'react-router-dom'
import { ApiError, apiResetPasswordWithCode } from '@/app/api'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { MailOutlined, LockOutlined, EyeOutlined, EyeInvisibleOutlined, KeyOutlined } from '@ant-design/icons'
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
  const [email, setEmail] = useState(() => {
    try {
      return localStorage.getItem('sf_remember_identifier') || ''
    } catch {
      return ''
    }
  })
  const [rememberMe, setRememberMe] = useState(true)
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Reset password modal state
  const [showResetModal, setShowResetModal] = useState(false)
  const [resetIdentifier, setResetIdentifier] = useState('')
  const [resetCode, setResetCode] = useState('')
  const [resetNewPassword, setResetNewPassword] = useState('')
  const [resetConfirmPassword, setResetConfirmPassword] = useState('')
  const [resetError, setResetError] = useState('')
  const [resetSuccess, setResetSuccess] = useState('')
  const [resetLoading, setResetLoading] = useState(false)
  const [showResetNewPassword, setShowResetNewPassword] = useState(false)

  if (isAuthenticated) {
    navigate('/', { replace: true })
    return null
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!email || !password) {
      setError('Email / nomor HP dan kata sandi harus diisi.')
      return
    }
    setLoading(true)
    setError('')
    try {
      if (rememberMe) {
        try {
          localStorage.setItem('sf_remember_identifier', email.trim())
        } catch {}
      } else {
        try {
          localStorage.removeItem('sf_remember_identifier')
        } catch {}
      }

      await login(email, password)
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        switch (err.code) {
          case 'reset_code_entered':
            setResetIdentifier(email.trim())
            setResetCode(password.trim())
            setShowResetModal(true)
            setResetError(err.message)
            break
          case 'invalid_credentials':
            setError('Email, nomor HP, atau kata sandi salah.')
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

  const handleResetSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setResetError('')
    setResetSuccess('')

    if (!resetIdentifier.trim()) {
      setResetError('Email atau nomor HP harus diisi.')
      return
    }
    const cleanCode = resetCode.trim()
    if (cleanCode.length !== 4 || !/^\d{4}$/.test(cleanCode)) {
      setResetError('Kode reset harus berupa 4 digit angka.')
      return
    }
    if (resetNewPassword.length < 8) {
      setResetError('Kata sandi baru minimal 8 karakter.')
      return
    }
    if (resetNewPassword !== resetConfirmPassword) {
      setResetError('Konfirmasi kata sandi tidak cocok.')
      return
    }

    setResetLoading(true)
    try {
      const res = await apiResetPasswordWithCode({
        identifier: resetIdentifier.trim(),
        code: cleanCode,
        new_password: resetNewPassword,
        confirm_password: resetConfirmPassword,
      })
      setResetSuccess(res.message || 'Kata sandi berhasil diubah. Silakan masuk menggunakan kata sandi baru Anda.')
      setEmail(resetIdentifier.trim())
      setPassword('')
      setTimeout(() => {
        setShowResetModal(false)
      }, 2000)
    } catch (err: any) {
      setResetError(err.message || 'Gagal mengatur ulang kata sandi.')
    } finally {
      setResetLoading(false)
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

          {/* Identifier field */}
          <div className="login-field">
            <label htmlFor="email">Email atau Nomor HP</label>
            <div className="login-input-wrapper">
              <span className="login-input-icon" aria-hidden="true">
                <MailOutlined />
              </span>
              <input
                id="email"
                name="identifier"
                type="text"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="Masukkan email atau 081298918220"
                required
                autoComplete="username"
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

          {/* Remember me & Forgot Password */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer', fontSize: 13, color: 'var(--sf-text)' }}>
              <input
                type="checkbox"
                checked={rememberMe}
                onChange={(e) => setRememberMe(e.target.checked)}
                disabled={loading}
                style={{ accentColor: 'var(--sf-accent)', width: 16, height: 16, cursor: 'pointer' }}
              />
              Ingat saya
            </label>
            <button
              type="button"
              onClick={() => {
                setResetIdentifier(email.trim())
                setResetCode('')
                setResetNewPassword('')
                setResetConfirmPassword('')
                setResetError('')
                setResetSuccess('')
                setShowResetModal(true)
              }}
              style={{
                background: 'none',
                border: 'none',
                color: 'var(--sf-accent, #3b82f6)',
                cursor: 'pointer',
                fontSize: 13,
                textDecoration: 'underline',
                padding: 0,
              }}
            >
              Lupa Kata Sandi?
            </button>
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

        <Modal open={showResetModal} onClose={() => !resetLoading && setShowResetModal(false)} width={460}>
          <ModalHeader title="Reset Kata Sandi Warga" onClose={() => !resetLoading && setShowResetModal(false)} />
          <form onSubmit={handleResetSubmit}>
            <ModalBody>
              <div style={{ fontSize: 13, color: 'var(--sf-text-muted, #94a3b8)', marginBottom: 16 }}>
                Masukkan email atau nomor HP Anda beserta 4 digit kode reset yang telah diberikan oleh Pengurus RT.
              </div>

              {resetSuccess && (
                <div style={{
                  background: 'rgba(34, 197, 94, 0.1)',
                  color: '#22c55e',
                  border: '1px solid rgba(34, 197, 94, 0.3)',
                  borderRadius: '6px',
                  padding: '10px 14px',
                  fontSize: '13px',
                  marginBottom: 16,
                }}>
                  {resetSuccess}
                </div>
              )}

              {resetError && (
                <div style={{
                  background: 'rgba(239, 68, 68, 0.1)',
                  color: '#ef4444',
                  border: '1px solid rgba(239, 68, 68, 0.3)',
                  borderRadius: '6px',
                  padding: '10px 14px',
                  fontSize: '13px',
                  marginBottom: 16,
                }}>
                  {resetError}
                </div>
              )}

              {/* Identifier */}
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="reset-identifier">
                  <MailOutlined style={{ marginRight: 4 }} />
                  Email atau Nomor HP
                </label>
                <input
                  id="reset-identifier"
                  className="sf-form-input"
                  type="text"
                  placeholder="Contoh: warga@email.com atau 081298918220"
                  value={resetIdentifier}
                  onChange={(e) => setResetIdentifier(e.target.value)}
                  disabled={resetLoading}
                  required
                />
              </div>

              {/* Kode 4 Digit */}
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="reset-code">
                  <KeyOutlined style={{ marginRight: 4 }} />
                  Kode Reset (4 Digit dari Pengurus)
                </label>
                <input
                  id="reset-code"
                  className="sf-form-input"
                  type="text"
                  maxLength={4}
                  placeholder="Contoh: 1234"
                  value={resetCode}
                  onChange={(e) => setResetCode(e.target.value.replace(/\D/g, '').slice(0, 4))}
                  disabled={resetLoading}
                  required
                  style={{
                    letterSpacing: '6px',
                    fontWeight: 'bold',
                    fontSize: '18px',
                    fontFamily: 'monospace',
                  }}
                />
              </div>

              {/* Password Baru */}
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="reset-new-password">
                  <LockOutlined style={{ marginRight: 4 }} />
                  Kata Sandi Baru (Min. 8 Karakter)
                </label>
                <div className="login-input-wrapper">
                  <input
                    id="reset-new-password"
                    className="login-input"
                    type={showResetNewPassword ? 'text' : 'password'}
                    placeholder="Masukkan kata sandi baru"
                    value={resetNewPassword}
                    onChange={(e) => setResetNewPassword(e.target.value)}
                    disabled={resetLoading}
                    required
                  />
                  <button
                    type="button"
                    className="login-toggle-password"
                    onClick={() => setShowResetNewPassword((v) => !v)}
                    tabIndex={-1}
                    aria-label={showResetNewPassword ? 'Sembunyikan kata sandi' : 'Tampilkan kata sandi'}
                  >
                    {showResetNewPassword ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                  </button>
                </div>
              </div>

              {/* Konfirmasi Password */}
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="reset-confirm-password">
                  <LockOutlined style={{ marginRight: 4 }} />
                  Konfirmasi Kata Sandi Baru
                </label>
                <input
                  id="reset-confirm-password"
                  className="sf-form-input"
                  type={showResetNewPassword ? 'text' : 'password'}
                  placeholder="Ulangi kata sandi baru"
                  value={resetConfirmPassword}
                  onChange={(e) => setResetConfirmPassword(e.target.value)}
                  disabled={resetLoading}
                  required
                />
              </div>
            </ModalBody>
            <ModalFooter>
              <button
                type="button"
                className="sf-btn-ghost"
                onClick={() => setShowResetModal(false)}
                disabled={resetLoading}
              >
                Batal
              </button>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={resetLoading}
              >
                {resetLoading ? 'Menyimpan...' : 'Ubah Kata Sandi'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      </div>
    </div>
  )
}
