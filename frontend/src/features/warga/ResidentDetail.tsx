import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import {
  getResident,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiResident,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { ResidentDetailFields } from './ResidentDetailFields'

export function ResidentDetail() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  if (!user || !id) return null

  const isPengurus = user.role === 'pengurus'
  const isSuperAdmin = user.systemRole === 'super_admin' || user.role === 'super_admin'
  const showNik = isPengurus || isSuperAdmin

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [resident, setResident] = useState<ApiResident | null>(null)
  const [notFound, setNotFound] = useState(false)

  const handleClose = useCallback(() => {
    navigate('/warga')
  }, [navigate])

  const fetchResident = useCallback(async () => {
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      setError(new ApiError('auth_expired', 'Sesi telah berakhir. Silakan masuk kembali.'))
      return
    }

    setLoading(true)
    setError(null)
    setNotFound(false)

    try {
      const data = await getResident(pair.accessToken, id)
      setResident(data)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiError && (err.code === 'not_found' || err.code === 'unauthorized')) {
        setNotFound(true)
      } else if (err instanceof ApiError && err.code === 'forbidden') {
        setError(new ApiError('access_denied', 'Anda tidak memiliki akses ke halaman ini.'))
      } else {
        setError(new ApiError('unexpected', 'Gagal memuat detail warga.'))
      }
    } finally {
      setLoading(false)
    }
  }, [id, navigate])

  useEffect(() => {
    void fetchResident()
  }, [fetchResident])

  return (
    <Modal open onClose={handleClose} width={560}>
      <ModalHeader
        title="Detail Warga"
        subtitle={resident?.full_name}
        onClose={handleClose}
      />
      {loading && (
        <>
          <ModalBody>
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
              Memuat detail warga...
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Batal
            </button>
          </ModalFooter>
        </>
      )}
      {notFound && (
        <>
          <ModalBody>
            <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
              Warga tidak ditemukan.
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Batal
            </button>
          </ModalFooter>
        </>
      )}
      {error && (
        <>
          <ModalBody>
            <div style={{
              background: 'var(--color-expense-subtle)',
              color: 'var(--color-expense)',
              border: '1px solid rgba(255,59,48,0.3)',
              borderRadius: 'var(--radius-sm)',
              padding: '10px 14px',
              fontSize: '13px',
            }}>
              {error.message}
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Batal
            </button>
          </ModalFooter>
        </>
      )}
      {resident && (
        <>
          <ModalBody>
            <ResidentDetailFields resident={resident} showNik={showNik} />
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Tutup
            </button>
          </ModalFooter>
        </>
      )}
    </Modal>
  )
}
