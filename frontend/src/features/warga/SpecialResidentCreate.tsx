import { useState } from 'react'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import {
  createSpecialResident,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
} from '@/app/api'
import { useNavigate } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { SpecialResidentForm, type SpecialResidentFormValues } from './SpecialResidentForm'

function isSuperAdmin(systemRole: string | null): boolean {
  return systemRole === 'super_admin'
}

export function SpecialResidentCreate() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<ApiErrorType | null>(null)

  if (!user) return null
  const canWrite = hasWargaWriteAccess(user)

  if (!canWrite) {
    return (
      <Modal open onClose={() => navigate('/warga')} width={560}>
        <ModalHeader
          title="Akses Ditolak"
          subtitle="Halaman ini hanya untuk pengurus yang berwenang"
          onClose={() => navigate('/warga')}
        />
        <ModalBody>
          <div style={{ fontSize: 13, color: 'var(--sf-text-muted)', textAlign: 'center', padding: '24px 0' }}>
            Anda tidak memiliki izin untuk menambah petugas khusus.
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={() => navigate('/warga')}>
            Kembali
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  const handleSubmit = async (values: SpecialResidentFormValues) => {
    setError(null)
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      navigate('/login')
      return
    }
    try {
      const body = {
        rt_id: isSuperAdmin(user.systemRole) ? values.rt_id : user.rt?.id ?? '',
        jabatan: values.jabatan,
        full_name: values.full_name,
        nik: values.nik,
        phone: values.phone,
        email: values.email,
        password: values.password,
        is_active: values.is_active,
      }
      await createSpecialResident(pair.accessToken, body)
      if (window.location.pathname === '/warga/baru') {
        navigate('/warga?refresh=1')
      } else {
        navigate('/warga')
      }
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
        return
      }
      setError(
        err instanceof ApiErrorType
          ? new ApiErrorType(err.code, err.message || 'Gagal menambah petugas.')
          : new ApiErrorType('unexpected', 'Gagal menambah petugas.'),
      )
      throw err
    }
  }

  return <SpecialResidentForm mode="create" onSubmit={handleSubmit} error={error} />
}
