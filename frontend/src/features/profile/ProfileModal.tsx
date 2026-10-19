import { useState, useEffect } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  apiUpdateProfile,
  apiChangePassword,
  getSessionPair,
  clearSessionPair,
  ApiError,
} from '@/app/api'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { formatPhoneDisplay } from '@/styles/format'
import {
  UserOutlined,
  PhoneOutlined,
  MailOutlined,
  LockOutlined,
  CheckCircleFilled,
  SafetyOutlined,
  HomeOutlined,
} from '@ant-design/icons'
import { Avatar } from 'antd'

interface ProfileModalProps {
  open: boolean
  onClose: () => void
}

function getInitials(name?: string): string {
  if (!name || !name.trim()) return ''
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

export function ProfileModal({ open, onClose }: ProfileModalProps) {
  const { user, refreshUser } = useAuth()
  const navigate = useNavigate()

  // Profile form state
  const [fullName, setFullName] = useState('')
  const [phone, setPhone] = useState('')
  const [email, setEmail] = useState('')
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileSuccess, setProfileSuccess] = useState<string | null>(null)
  const [profileError, setProfileError] = useState<string | null>(null)

  // Password form state
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordSuccess, setPasswordSuccess] = useState<string | null>(null)
  const [passwordError, setPasswordError] = useState<string | null>(null)

  // Active tab: 'profile' | 'password'
  const [activeTab, setActiveTab] = useState<'profile' | 'password'>('profile')

  useEffect(() => {
    if (user && open) {
      setFullName(user.name || '')
      const formatted = user.phone ? formatPhoneDisplay(user.phone) : ''
      setPhone(formatted === '-' ? '' : formatted)
      setEmail(user.email || '')
      setProfileSuccess(null)
      setProfileError(null)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setPasswordSuccess(null)
      setPasswordError(null)
    }
  }, [user, open])

  // Refresh the canonical profile from the server whenever the modal is opened,
  // so changes made by an administrator on another device/session (e.g. a
  // resident's phone number) are reflected without requiring logout/login.
  useEffect(() => {
    if (open) {
      void refreshUser?.()
    }
  }, [open, refreshUser])

  if (!open || !user) return null

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    setProfileSuccess(null)
    setProfileError(null)

    if (!fullName.trim()) {
      setProfileError('Nama lengkap harus diisi.')
      return
    }

    const pair = getSessionPair()
    if (!pair?.accessToken) {
      clearSessionPair()
      navigate('/login')
      return
    }

    setProfileSaving(true)
    try {
      await apiUpdateProfile(pair.accessToken, {
        full_name: fullName.trim(),
        phone: phone.trim() || undefined,
        email: email.trim() || undefined,
      })
      await refreshUser?.()
      setProfileSuccess('Profil berhasil diperbarui.')
    } catch (err: any) {
      if (err instanceof ApiError) {
        setProfileError(err.message)
      } else {
        setProfileError('Gagal memperbarui profil.')
      }
    } finally {
      setProfileSaving(false)
    }
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setPasswordSuccess(null)
    setPasswordError(null)

    if (!currentPassword) {
      setPasswordError('Kata sandi saat ini harus diisi.')
      return
    }
    if (!newPassword) {
      setPasswordError('Kata sandi baru harus diisi.')
      return
    }
    if (newPassword.length < 8) {
      setPasswordError('Kata sandi baru minimal 8 karakter.')
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError('Konfirmasi kata sandi tidak cocok.')
      return
    }

    const pair = getSessionPair()
    if (!pair?.accessToken) {
      clearSessionPair()
      navigate('/login')
      return
    }

    setPasswordSaving(true)
    try {
      await apiChangePassword(pair.accessToken, {
        current_password: currentPassword,
        new_password: newPassword,
      })
      setPasswordSuccess('Kata sandi berhasil diubah.')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      if (err instanceof ApiError) {
        setPasswordError(err.message)
      } else {
        setPasswordError('Gagal mengubah kata sandi.')
      }
    } finally {
      setPasswordSaving(false)
    }
  }

  const rtLabel = user.rt?.name
    ? user.rt.name
    : user.rtCode
    ? `RT ${user.rtCode}`
    : null

  return (
    <Modal open={open} onClose={onClose} width={540}>
      <ModalHeader
        title="Profil Pengguna"
        subtitle="Kelola informasi akun dan keamanan Anda"
        onClose={onClose}
      />
      <ModalBody>
        {/* User Card Summary */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 16,
            padding: '16px',
            borderRadius: '8px',
            backgroundColor: 'var(--sf-bg-surface-secondary, rgba(255, 255, 255, 0.04))',
            marginBottom: '20px',
            border: '1px solid var(--sf-border-subtle, rgba(255, 255, 255, 0.08))',
          }}
        >
          <Avatar
            size={54}
            style={{
              backgroundColor: 'var(--sf-accent, #3b82f6)',
              fontSize: 20,
              fontWeight: 600,
              flexShrink: 0,
            }}
            icon={<UserOutlined />}
          >
            {getInitials(user.name)}
          </Avatar>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 16, fontWeight: 600, color: 'var(--sf-text-primary)' }}>
              {user.name}
            </div>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 4, flexWrap: 'wrap' }}>
              <span
                style={{
                  display: 'inline-block',
                  padding: '2px 8px',
                  borderRadius: 4,
                  fontSize: 11,
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.04em',
                  backgroundColor: 'rgba(59, 130, 246, 0.15)',
                  color: '#60a5fa',
                }}
              >
                {user.role}
              </span>
              {user.jabatan && (
                <span
                  style={{
                    display: 'inline-block',
                    padding: '2px 8px',
                    borderRadius: 4,
                    fontSize: 11,
                    fontWeight: 600,
                    backgroundColor: 'rgba(16, 185, 129, 0.15)',
                    color: '#34d399',
                  }}
                >
                  {user.jabatan}
                </span>
              )}
              {rtLabel && (
                <span
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 4,
                    fontSize: 12,
                    color: 'var(--sf-text-muted, #94a3b8)',
                  }}
                >
                  <HomeOutlined style={{ fontSize: 11 }} />
                  {rtLabel}
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Tab navigation */}
        <div
          style={{
            display: 'flex',
            borderBottom: '1px solid var(--sf-border-subtle, rgba(255, 255, 255, 0.08))',
            marginBottom: 20,
          }}
        >
          <button
            type="button"
            onClick={() => setActiveTab('profile')}
            style={{
              padding: '8px 16px',
              background: 'none',
              border: 'none',
              borderBottom: activeTab === 'profile' ? '2px solid var(--sf-accent, #3b82f6)' : '2px solid transparent',
              color: activeTab === 'profile' ? 'var(--sf-text-primary)' : 'var(--sf-text-muted)',
              fontWeight: activeTab === 'profile' ? 600 : 400,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              fontSize: 13,
            }}
          >
            <UserOutlined />
            Informasi Profil
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('password')}
            style={{
              padding: '8px 16px',
              background: 'none',
              border: 'none',
              borderBottom: activeTab === 'password' ? '2px solid var(--sf-accent, #3b82f6)' : '2px solid transparent',
              color: activeTab === 'password' ? 'var(--sf-text-primary)' : 'var(--sf-text-muted)',
              fontWeight: activeTab === 'password' ? 600 : 400,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              fontSize: 13,
            }}
          >
            <SafetyOutlined />
            Keamanan (Kata Sandi)
          </button>
        </div>

        {/* Tab 1: Profile Information */}
        {activeTab === 'profile' && (
          <form onSubmit={handleUpdateProfile}>
            {profileSuccess && (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '10px 14px',
                  borderRadius: 6,
                  backgroundColor: 'rgba(16, 185, 129, 0.12)',
                  color: '#34d399',
                  fontSize: 13,
                  marginBottom: 16,
                  border: '1px solid rgba(16, 185, 129, 0.25)',
                }}
              >
                <CheckCircleFilled />
                {profileSuccess}
              </div>
            )}
            {profileError && (
              <div className="sf-form-error-list" style={{ marginBottom: 16 }}>
                {profileError}
              </div>
            )}

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="prof-name">
                <UserOutlined style={{ marginRight: 4 }} />
                Nama Lengkap <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="prof-name"
                className="sf-form-input"
                type="text"
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
                required
                disabled={profileSaving}
              />
            </div>

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="prof-phone">
                <PhoneOutlined style={{ marginRight: 4 }} />
                Nomor Telepon
              </label>
              <input
                id="prof-phone"
                className="sf-form-input"
                type="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="Contoh: 08123456789"
                disabled={profileSaving}
              />
            </div>

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="prof-email">
                <MailOutlined style={{ marginRight: 4 }} />
                Email
              </label>
              <input
                id="prof-email"
                className="sf-form-input"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="nama@email.com"
                disabled={profileSaving}
              />
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 24, gap: 10 }}>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={profileSaving}
              >
                {profileSaving ? 'Menyimpan...' : 'Simpan Profil'}
              </button>
            </div>
          </form>
        )}

        {/* Tab 2: Change Password */}
        {activeTab === 'password' && (
          <form onSubmit={handleChangePassword}>
            {passwordSuccess && (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '10px 14px',
                  borderRadius: 6,
                  backgroundColor: 'rgba(16, 185, 129, 0.12)',
                  color: '#34d399',
                  fontSize: 13,
                  marginBottom: 16,
                  border: '1px solid rgba(16, 185, 129, 0.25)',
                }}
              >
                <CheckCircleFilled />
                {passwordSuccess}
              </div>
            )}
            {passwordError && (
              <div className="sf-form-error-list" style={{ marginBottom: 16 }}>
                {passwordError}
              </div>
            )}

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="pass-current">
                <LockOutlined style={{ marginRight: 4 }} />
                Kata Sandi Saat Ini <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="pass-current"
                className="sf-form-input"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="Masukkan kata sandi lama"
                required
                disabled={passwordSaving}
              />
            </div>

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="pass-new">
                <LockOutlined style={{ marginRight: 4 }} />
                Kata Sandi Baru <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="pass-new"
                className="sf-form-input"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Minimal 8 karakter"
                required
                disabled={passwordSaving}
              />
            </div>

            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="pass-confirm">
                <LockOutlined style={{ marginRight: 4 }} />
                Konfirmasi Kata Sandi Baru <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="pass-confirm"
                className="sf-form-input"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Ulangi kata sandi baru"
                required
                disabled={passwordSaving}
              />
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 24, gap: 10 }}>
              <button
                type="submit"
                className="sf-btn-primary"
                disabled={passwordSaving}
              >
                {passwordSaving ? 'Mengubah...' : 'Ubah Kata Sandi'}
              </button>
            </div>
          </form>
        )}
      </ModalBody>
      <ModalFooter>
        <button type="button" className="sf-btn-ghost" onClick={onClose}>
          Tutup
        </button>
      </ModalFooter>
    </Modal>
  )
}

