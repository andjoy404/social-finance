import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import {
  getResident,
  updateSpecialResident,
  ApiError as ApiErrorType,
  clearSessionPair,
  getSessionPair,
  type ApiResident,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { SpecialResidentForm, type SpecialResidentFormValues, type JabatanValue } from './SpecialResidentForm'

interface Props {
  mode?: 'edit' | 'detail'
}

const validJabatans: Record<string, boolean> = {
  keamanan: true,
  operasional: true,
  kebersihan: true,
}

export function SpecialResidentEdit({ mode = 'edit' }: Props) {
  const { user } = useAuth()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [loadingData, setLoadingData] = useState(true)
  const [savingError, setSavingError] = useState<ApiErrorType | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [resident, setResident] = useState<ApiResident | null>(null)

  const fetchResident = useCallback(async () => {
    if (!id) return
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      navigate('/login')
      return
    }
    setLoadingData(true)
    setNotFound(false)
    setSavingError(null)
    try {
      const data: ApiResident = await getResident(pair.accessToken, id)
      if (data.jabatan && validJabatans[data.jabatan] && !data.household_id) {
        setResident(data)
      } else {
        if (data.household_id) {
          navigate(`/warga/${data.household_id}/edit`, { replace: true })
        } else {
          navigate(`/warga/${data.id}/edit`, { replace: true })
        }
      }
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
      } else if (err instanceof ApiErrorType && err.code === 'not_found') {
        setNotFound(true)
      } else if (err instanceof ApiErrorType) {
        setSavingError(new ApiErrorType(err.code, err.message || 'Gagal memuat data.'))
      } else {
        setSavingError(new ApiErrorType('unexpected', 'Gagal memuat data.'))
      }
    } finally {
      setLoadingData(false)
    }
  }, [id, navigate])

  useEffect(() => {
    void fetchResident()
  }, [fetchResident])

  if (!user || !id) return null

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
            Anda tidak memiliki izin untuk mengubah data petugas ini.
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

  const handleClose = () => navigate('/warga')

  if (loadingData) {
    return (
      <Modal open onClose={handleClose} width={560}>
        <ModalHeader title="Ubah Petugas" subtitle="Memuat..." onClose={handleClose} />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
            Memuat data petugas...
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Batal
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  if (notFound || !resident) {
    return (
      <Modal open onClose={handleClose} width={560}>
        <ModalHeader title="Ubah Petugas" onClose={handleClose} />
        <ModalBody>
          <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--sf-text-muted)', fontSize: 13 }}>
            {notFound ? 'Petugas tidak ditemukan.' : savingError?.message || 'Gagal memuat data.'}
          </div>
        </ModalBody>
        <ModalFooter>
          <button type="button" className="sf-btn-ghost" onClick={handleClose}>
            Batal
          </button>
        </ModalFooter>
      </Modal>
    )
  }

  const handleSubmit = async (values: SpecialResidentFormValues) => {
    setSavingError(null)
    const pair = getSessionPair()
    if (!pair?.accessToken) {
      navigate('/login')
      return
    }
    try {
      const body: {
        full_name: string
        nik: string
        phone: string
        email: string
        jabatan: JabatanValue
        is_active: boolean
        password?: string
      } = {
        full_name: values.full_name,
        nik: values.nik,
        phone: values.phone,
        email: values.email,
        jabatan: values.jabatan,
        is_active: values.is_active,
      }
      if (values.password.trim()) {
        body.password = values.password.trim()
      }
      await updateSpecialResident(pair.accessToken, id, body)
      navigate('/warga')
    } catch (err) {
      if (err instanceof ApiErrorType && err.code === 'auth_expired') {
        clearSessionPair()
        navigate('/login')
        return
      }
      setSavingError(
        err instanceof ApiErrorType
          ? new ApiErrorType(err.code, err.message || 'Gagal memperbarui petugas.')
          : new ApiErrorType('unexpected', 'Gagal memperbarui petugas.'),
      )
      throw err
    }
  }

  return (
    <SpecialResidentForm
      key={resident.id}
      mode={mode}
      canEdit={canWrite}
      residentId={resident.id}
      subtitle={resident.full_name}
      error={savingError}
      initialValues={{
        full_name: resident.full_name || '',
        nik: resident.nik || '',
        phone: resident.phone || '',
        email: resident.email || '',
        jabatan: (resident.jabatan as JabatanValue) || 'keamanan',
        is_active: resident.is_active,
        rt_id: resident.rt_id || '',
        password: '',
      }}
      onSubmit={handleSubmit}
    />
  )
}
