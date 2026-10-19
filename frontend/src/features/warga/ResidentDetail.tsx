import { useState, useEffect, useCallback } from 'react'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import {
  getResident,
  updateResident,
  updateSpecialResident,
  getHousehold,
  updateHousehold,
  ApiError,
  clearSessionPair,
  getSessionPair,
  type ApiResident,
  type ApiUpdateHouseholdBody,
  type ApiHousehold,
} from '@/app/api'
import { useNavigate, useParams } from 'react-router-dom'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { ResidentDetailFields } from './ResidentDetailFields'
import {
  HomeOutlined,
  UserOutlined,
  IdcardOutlined,
  PhoneOutlined,
  MailOutlined,
  EditOutlined,
  LockOutlined,
  CalendarOutlined,
  EnvironmentOutlined,
} from '@ant-design/icons'
import { formatPhoneDisplay } from '@/styles/format'

type OccupancyStatus = 'OWNER' | 'TENANT'

export function ResidentDetail() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  if (!user || !id) return null

  const isPengurus = user.role === 'pengurus'
  const isSuperAdmin = user.systemRole === 'super_admin' || user.role === 'super_admin'
  const showNik = isPengurus || isSuperAdmin
  const canWrite = hasWargaWriteAccess(user)

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ApiError | null>(null)
  const [resident, setResident] = useState<ApiResident | null>(null)
  const [notFound, setNotFound] = useState(false)

  // Edit mode state
  const [isEditing, setIsEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [editForm, setEditForm] = useState({
    fullName: '',
    nik: '',
    phone: '',
    email: '',
    relationshipToHead: '',
    isActive: true,
    password: '',
    // Household-level fields (from "Tambah Warga" form)
    houseNumber: '',
    occupancyStatus: 'OWNER' as OccupancyStatus,
    address: '',
    startDate: '',
  })

  const handleStartEdit = () => {
    if (!resident) return
    const rawPhone = resident.phone || ''
    const formattedPhone = rawPhone ? formatPhoneDisplay(rawPhone) : ''
    setEditForm({
      fullName: resident.full_name || '',
      nik: resident.nik || '',
      phone: formattedPhone === '-' ? '' : formattedPhone,
      email: resident.email || '',
      relationshipToHead: resident.relationship_to_head || '',
      isActive: resident.is_active,
      password: '',
      // Household-level fields
      houseNumber: resident.house_number ?? '',
      occupancyStatus: (resident.occupancy_status as OccupancyStatus) || 'OWNER',
      address: '',
      startDate: resident.start_date ? resident.start_date.slice(0, 10) : '',
    })
    setValidationErrors([])
    setEditError(null)
    setIsEditing(true)
  }

  const handleCancelEdit = () => {
    setIsEditing(false)
    setValidationErrors([])
    setEditError(null)
  }

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

      // Fetch household data to populate address field (from "Tambah Warga" form)
      if (data.household_id) {
        try {
          const household: ApiHousehold = await getHousehold(pair.accessToken, data.household_id)
          if (editForm.address === '') {
            setEditForm((prev) => ({ ...prev, address: household.address ?? '' }))
          }
        } catch {
          // Household fetch failure is non-fatal; address stays empty
        }
      }
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
      {resident && !isEditing && (
        <>
          <ModalBody>
            <ResidentDetailFields resident={resident} showNik={showNik} />
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={handleClose}>
              Tutup
            </button>
            {canWrite && (
              <button
                type="button"
                className="sf-btn-primary"
                onClick={handleStartEdit}
              >
                <EditOutlined style={{ marginRight: 4 }} />
                Ubah
              </button>
            )}
          </ModalFooter>
        </>
      )}

      {resident && isEditing && (
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            const errs: string[] = []
            if (!editForm.fullName.trim()) errs.push('Nama lengkap harus diisi.')
            if (showNik && editForm.nik.trim() && !/^[0-9]{16}$/.test(editForm.nik.trim())) {
              errs.push('KTP / NIK harus berupa 16 digit angka.')
            }
            if (editForm.email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(editForm.email.trim())) {
              errs.push('Format email tidak valid.')
            }
            if (!editForm.houseNumber.trim()) errs.push('Nomor rumah harus diisi.')
            if (!editForm.startDate.trim()) errs.push('Tanggal mulai hunian harus diisi.')
            if (editForm.password && editForm.password.trim().length > 0 && editForm.password.trim().length < 8) {
              errs.push('Kata sandi minimal 8 karakter.')
            }
            if (errs.length > 0) {
              setValidationErrors(errs)
              return
            }

            setSaving(true)
            setEditError(null)
            try {
              const pair = getSessionPair()
              if (!pair?.accessToken) {
                clearSessionPair()
                navigate('/login')
                return
              }

              // ── Update resident (resident-level fields) ──
              if (resident.jabatan) {
                await updateSpecialResident(pair.accessToken, resident.id, {
                  full_name: editForm.fullName.trim(),
                  nik: editForm.nik.trim() || undefined,
                  phone: editForm.phone.trim() || undefined,
                  email: editForm.email.trim() || undefined,
                  jabatan: resident.jabatan as 'keamanan' | 'kebersihan_pembangunan',
                  is_active: editForm.isActive,
                })
              } else {
                await updateResident(pair.accessToken, resident.id, {
                  full_name: editForm.fullName.trim(),
                  nik: editForm.nik.trim() || undefined,
                  phone: editForm.phone.trim() || undefined,
                  email: editForm.email.trim() || undefined,
                  relationship_to_head: editForm.relationshipToHead.trim() || null,
                  is_active: editForm.isActive,
                  password: editForm.password.trim() || undefined,
                })
              }

              // ── Update household (household-level fields from "Tambah Warga" form) ──
              if (resident.household_id) {
                const householdBody: ApiUpdateHouseholdBody = {
                  house_number: editForm.houseNumber.trim(),
                  head_name: editForm.fullName.trim(),
                  phone: editForm.phone.trim() || undefined,
                  email: editForm.email.trim() || undefined,
                  address: editForm.address.trim() || null,
                  occupancy_status: editForm.occupancyStatus,
                  start_date: editForm.startDate.trim(),
                  is_active: editForm.isActive,
                }
                await updateHousehold(pair.accessToken, resident.household_id, householdBody)
              }

              // Refresh resident details from API
              const updated = await getResident(pair.accessToken, resident.id)
              setResident(updated)
              setIsEditing(false)
              window.dispatchEvent(new CustomEvent('warga:refresh'))
            } catch (err: any) {
              setEditError(err?.message || 'Gagal menyimpan perubahan data warga.')
            } finally {
              setSaving(false)
            }
          }}
        >
          <ModalBody>
            {validationErrors.length > 0 && (
              <div className="sf-form-error-list">
                <ul>{validationErrors.map((err, i) => <li key={i}>{err}</li>)}</ul>
              </div>
            )}
            {editError && (
              <div className="sf-form-error-list">{editError}</div>
            )}

            {/* Nama Lengkap */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="edit-name">
                <UserOutlined style={{ marginRight: 4 }} />
                Nama Lengkap <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="edit-name"
                className="sf-form-input"
                type="text"
                value={editForm.fullName}
                onChange={(e) => setEditForm((prev) => ({ ...prev, fullName: e.target.value }))}
                required
                disabled={saving}
                autoFocus
              />
            </div>

            {/* Nomor Rumah + Status Hunian */}
            <div className="sf-form-grid-2">
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="edit-house-number">
                  <HomeOutlined />
                  Nomor Rumah <span style={{ color: 'var(--sf-danger)' }}>*</span>
                </label>
                <input
                  id="edit-house-number"
                  className="sf-form-input"
                  type="text"
                  value={editForm.houseNumber}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, houseNumber: e.target.value }))}
                  required
                  disabled={saving}
                  placeholder="Contoh: 001"
                />
              </div>
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="edit-occupancy">
                  Status Hunian <span style={{ color: 'var(--sf-danger)' }}>*</span>
                </label>
                <select
                  id="edit-occupancy"
                  className="sf-form-select"
                  value={editForm.occupancyStatus}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, occupancyStatus: e.target.value as OccupancyStatus }))}
                  disabled={saving}
                >
                  <option value="OWNER">Pemilik</option>
                  <option value="TENANT">Penyewa</option>
                </select>
              </div>
            </div>

            {/* NIK + Telepon */}
            <div className="sf-form-grid-2">
              {showNik && (
                <div className="sf-form-field">
                  <label className="sf-form-label" htmlFor="edit-nik">
                    <IdcardOutlined style={{ marginRight: 4 }} />
                    NIK
                  </label>
                  <input
                    id="edit-nik"
                    className="sf-form-input"
                    type="text"
                    maxLength={16}
                    value={editForm.nik}
                    onChange={(e) => setEditForm((prev) => ({ ...prev, nik: e.target.value.replace(/\D/g, '') }))}
                    disabled={saving}
                  />
                </div>
              )}
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="edit-phone">
                  <PhoneOutlined style={{ marginRight: 4 }} />
                  Telepon
                </label>
                <input
                  id="edit-phone"
                  className="sf-form-input"
                  type="text"
                  value={editForm.phone}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, phone: e.target.value }))}
                  disabled={saving}
                  placeholder="Contoh: 08123456789"
                />
              </div>
            </div>

            {/* Email + Alamat */}
            <div className="sf-form-grid-2">
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="edit-email">
                  <MailOutlined style={{ marginRight: 4 }} />
                  Email
                </label>
                <input
                  id="edit-email"
                  className="sf-form-input"
                  type="email"
                  value={editForm.email}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, email: e.target.value }))}
                  disabled={saving}
                  placeholder="nama@email.com"
                />
              </div>
              <div className="sf-form-field">
                <label className="sf-form-label" htmlFor="edit-address">
                  <EnvironmentOutlined style={{ marginRight: 4 }} />
                  Alamat
                </label>
                <input
                  id="edit-address"
                  className="sf-form-input"
                  type="text"
                  value={editForm.address}
                  onChange={(e) => setEditForm((prev) => ({ ...prev, address: e.target.value }))}
                  disabled={saving}
                  placeholder="Tidak diisi jika kosong"
                />
              </div>
            </div>

            {/* Tanggal Mulai Hunian */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="edit-start-date">
                <CalendarOutlined style={{ marginRight: 4 }} />
                Tanggal Mulai Hunian <span style={{ color: 'var(--sf-danger)' }}>*</span>
              </label>
              <input
                id="edit-start-date"
                className="sf-form-input"
                type="date"
                value={editForm.startDate}
                onChange={(e) => setEditForm((prev) => ({ ...prev, startDate: e.target.value }))}
                required
                disabled={saving}
              />
            </div>

            {/* Hubungan dengan KK */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="edit-relationship">
                Hubungan dengan KK
              </label>
              <select
                id="edit-relationship"
                className="sf-form-select"
                value={editForm.relationshipToHead}
                onChange={(e) => setEditForm((prev) => ({ ...prev, relationshipToHead: e.target.value }))}
                disabled={saving}
              >
                <option value="">-- Pilih Hubungan --</option>
                <option value="HEAD">Kepala Keluarga</option>
                <option value="SPOUSE">Suami / Istri</option>
                <option value="CHILD">Anak</option>
                <option value="PARENT">Orang Tua</option>
                <option value="OTHER">Lainnya</option>
              </select>
            </div>

            {/* Kata Sandi Baru (Opsional) */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="edit-password">
                <LockOutlined style={{ marginRight: 4 }} />
                Kata Sandi Baru (Opsional)
              </label>
              <input
                id="edit-password"
                className="sf-form-input"
                type="password"
                placeholder="Kosongkan jika tidak ingin mengubah kata sandi"
                value={editForm.password}
                onChange={(e) => setEditForm((prev) => ({ ...prev, password: e.target.value }))}
                disabled={saving}
              />
            </div>

            {/* Status Aktif */}
            <div className="sf-form-field">
              <label className="sf-form-label" htmlFor="edit-status">Status</label>
              <select
                id="edit-status"
                className="sf-form-select"
                value={editForm.isActive ? 'true' : 'false'}
                onChange={(e) => setEditForm((prev) => ({ ...prev, isActive: e.target.value === 'true' }))}
                disabled={saving}
              >
                <option value="true">Aktif</option>
                <option value="false">Tidak Aktif</option>
              </select>
            </div>
          </ModalBody>
          <ModalFooter>
            <button
              type="button"
              className="sf-btn-ghost sf-btn-cancel-red"
              onClick={handleCancelEdit}
              disabled={saving}
            >
              Batal
            </button>
            <button
              type="submit"
              className="sf-btn-primary"
              disabled={saving}
            >
              {saving ? 'Menyimpan...' : 'Simpan Perubahan'}
            </button>
          </ModalFooter>
        </form>
      )}
    </Modal>
  )
}
