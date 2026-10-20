import { useCallback, useEffect, useRef, useState } from 'react'
import {
  apiListRTMemberships,
  apiListRWs,
  apiSetRTJabatan,
  apiRemoveRTJabatan,
  apiUpdateRT,
  clearSessionPair,
  getSessionPair,
  ApiError,
  type ApiRT,
  type ApiRW,
  type ApiUpdateRTBody,
  type ApiRTMembership,
} from '@/app/api'
import { Badge } from '@/components/Badge'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { positionVariant, rtPositionLabel } from './position-badge'
import { RtFormFields, type RtFormState } from './RtCreate'

const positions = [
  ['ketua', 'Ketua'], ['wakil_ketua', 'Wakil Ketua'], ['sekretaris', 'Sekretaris'], ['bendahara', 'Bendahara'],
  ['keamanan', 'Keamanan'], ['sosial', 'Sosial'], ['kebersihan_pembangunan', 'Operasional'],
]
const label = rtPositionLabel
const emptyRtForm: RtFormState = { name: '', rw_id: '', rw: '', rt: '', address: '', head_name: '' }

function toRtForm(rt: ApiRT, rwList: ApiRW[]): RtFormState {
  return {
    name: rt.name,
    rw_id: rt.rw_id || rwList.find((rw) => rw.rw_number === rt.rw)?.id || '',
    rw: String(rt.rw),
    rt: rt.rt,
    address: rt.address ?? '',
    head_name: rt.head_name ?? '',
  }
}

export function RtDetailModal({
  open,
  rt,
  onClose,
  onUpdated,
}: {
  open: boolean
  rt: ApiRT | null
  onClose: () => void
  onUpdated?: (rt: ApiRT) => void
}) {
  const [members, setMembers] = useState<ApiRTMembership[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [activeTab, setActiveTab] = useState<'ubah' | 'jabatan'>('jabatan')
  const [rwList, setRwList] = useState<ApiRW[]>([])
  const [rwLoaded, setRwLoaded] = useState(false)
  const [editLoading, setEditLoading] = useState(false)
  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)
  const [editSuccess, setEditSuccess] = useState<string | null>(null)
  const [editValidationErrors, setEditValidationErrors] = useState<string[]>([])
  const [rtForm, setRtForm] = useState<RtFormState>(emptyRtForm)
  // Assignment form state
  const [selectedJabatan, setSelectedJabatan] = useState('')
  const [selectedMember, setSelectedMember] = useState<ApiRTMembership | null>(null)
  const [selectorOpen, setSelectorOpen] = useState(false)
  const [selectorSearch, setSelectorSearch] = useState('')
  const [modalSaving, setModalSaving] = useState(false)
  const [modalError, setModalError] = useState<string | null>(null)
  const [actionMessage, setActionMessage] = useState<string | null>(null)
  const [revokeTarget, setRevokeTarget] = useState<ApiRTMembership | null>(null)
  const submitLock = useRef(false)
  const editLock = useRef(false)
  const editTabRef = useRef<HTMLButtonElement>(null)
  const jabatanTabRef = useRef<HTMLButtonElement>(null)

  const loadRws = useCallback(async (currentRt: ApiRT) => {
    const session = getSessionPair()
    if (!session?.accessToken) {
      setEditError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }
    setEditLoading(true)
    setEditError(null)
    try {
      const resp = await apiListRWs(session.accessToken, { page_size: 100 })
      const options = resp.data ?? []
      setRwList(options)
      setRwLoaded(true)
      setRtForm((prev) => ({
        ...prev,
        rw_id: prev.rw_id || currentRt.rw_id || options.find((rw) => rw.rw_number === currentRt.rw)?.id || '',
      }))
    } catch (err) {
      setRwLoaded(false)
      setEditError(err instanceof Error ? err.message : 'Gagal memuat daftar RW.')
    } finally {
      setEditLoading(false)
    }
  }, [])

  const loadMembers = useCallback(async (id: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) {
      setError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }
    setLoading(true)
    setError(null)
    try {
      setMembers(await apiListRTMemberships(session.accessToken, id))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Gagal memuat anggota RT')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open && rt?.id) {
      void loadMembers(rt.id)
      setActiveTab('jabatan')
      setRwList([])
      setRwLoaded(false)
      setRtForm(toRtForm(rt, []))
      setEditValidationErrors([])
      setEditError(null)
      setEditSuccess(null)
      setSelectedJabatan('')
      setSelectedMember(null)
      setSelectorOpen(false)
      setSelectorSearch('')
      setModalError(null)
      setActionMessage(null)
      setRevokeTarget(null)
    } else {
      setMembers([])
      setError(null)
      setSelectedMember(null)
      setSelectedJabatan('')
      setSelectorOpen(false)
      setSelectorSearch('')
      setModalError(null)
      setActionMessage(null)
      setRevokeTarget(null)
      setActiveTab('jabatan')
      setRwList([])
      setRwLoaded(false)
      setRtForm(emptyRtForm)
      setEditValidationErrors([])
      setEditError(null)
      setEditSuccess(null)
    }
  }, [open, rt?.id, loadMembers, loadRws])

  const handleRtFormChange = (field: string, value: string) => {
    setRtForm((prev) => ({ ...prev, [field]: value }))
    setEditValidationErrors([])
    setEditError(null)
    setEditSuccess(null)
  }

  const handleUpdateRt = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!rt || editLock.current) return
    const validationErrors: string[] = []
    if (!rtForm.name.trim()) validationErrors.push('Nama RT harus diisi.')
    if (!rtForm.rw || !/^\d+$/.test(rtForm.rw)) validationErrors.push('RW harus angka yang valid.')
    if (rtForm.rw && /^\d+$/.test(rtForm.rw) && Number(rtForm.rw) < 0) validationErrors.push('RW tidak boleh negatif.')
    if (!rtForm.rt.trim()) validationErrors.push('Kode RT harus diisi.')
    if (validationErrors.length > 0) {
      setEditValidationErrors(validationErrors)
      return
    }

    const session = getSessionPair()
    if (!session?.accessToken) {
      setEditError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }

    editLock.current = true
    setEditSaving(true)
    setEditError(null)
    setEditSuccess(null)
    try {
      const body: ApiUpdateRTBody = {
        name: rtForm.name.trim(),
        ...(rtForm.rw_id ? { rw_id: rtForm.rw_id } : {}),
        rw: Number(rtForm.rw),
        rt: rtForm.rt.trim(),
        address: rtForm.address.trim() || null,
        head_name: rtForm.head_name.trim() || null,
      }
      const updatedRt = await apiUpdateRT(session.accessToken, rt.id, body)
      setRtForm(toRtForm(updatedRt, rwList))
      setEditSuccess('Informasi RT berhasil diperbarui.')
      onUpdated?.(updatedRt)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'auth_expired') {
        clearSessionPair()
        setEditError('Sesi telah berakhir. Silakan masuk kembali.')
      } else {
        setEditError(err instanceof Error ? err.message : 'Gagal memperbarui RT.')
      }
    } finally {
      editLock.current = false
      setEditSaving(false)
    }
  }

  const filteredMembers = members.filter((m) => {
    if (m.jabatan) return false
    if (!selectorSearch.trim()) return true
    const q = selectorSearch.toLowerCase()
    return (
      (m.full_name ?? '').toLowerCase().includes(q) ||
      (m.email ?? '').toLowerCase().includes(q) ||
      (m.phone ?? '').toLowerCase().includes(q) ||
      (m.user_id ?? '').toLowerCase().includes(q)
    )
  })

  const handleTabKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault()
      const nextTab = activeTab === 'ubah' ? 'jabatan' : 'ubah'
      setActiveTab(nextTab)
      if (nextTab === 'ubah') {
        jabatanTabRef.current?.blur()
        editTabRef.current?.focus()
        if (rt && !rwLoaded && !editLoading) void loadRws(rt)
      } else {
        jabatanTabRef.current?.focus()
      }
    }
  }

  const handleSaveJabatan = async (e: React.FormEvent) => {
    e.preventDefault()
    if (submitLock.current || !selectedMember || !rt?.id || !selectedJabatan) return
    const session = getSessionPair()
    if (!session?.accessToken) {
      setModalError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }

    submitLock.current = true
    setModalSaving(true)
    setModalError(null)
    setActionMessage(null)
    try {
      await apiSetRTJabatan(session.accessToken, rt.id, selectedMember.user_id, selectedJabatan)
      await loadMembers(rt.id)
      setSelectedMember(null)
      setSelectedJabatan('')
      setActionMessage('Jabatan berhasil disimpan.')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Mutasi jabatan gagal'
      setModalError(msg)
    } finally {
      submitLock.current = false
      setModalSaving(false)
    }
  }

  const handleRevoke = async () => {
    if (submitLock.current || !revokeTarget || !rt?.id) return
    const session = getSessionPair()
    if (!session?.accessToken) {
      setModalError('Sesi telah berakhir. Silakan masuk kembali.')
      return
    }

    submitLock.current = true
    setModalSaving(true)
    setModalError(null)
    setActionMessage(null)
    try {
      await apiRemoveRTJabatan(session.accessToken, rt.id, revokeTarget.user_id)
      await loadMembers(rt.id)
      setRevokeTarget(null)
      if (selectedMember?.user_id === revokeTarget.user_id) {
        setSelectedMember(null)
        setSelectedJabatan('')
      }
      setActionMessage('Jabatan berhasil dicabut.')
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Gagal mencabut jabatan'
      setModalError(msg)
    } finally {
      submitLock.current = false
      setModalSaving(false)
    }
  }

  if (!open || !rt) return null

  const canSave = selectedJabatan !== '' && selectedMember !== null

  return (
    <>
      <Modal open={open} onClose={onClose} width={640}>
        <ModalHeader title={`Detail RT ${rt.rt}`} subtitle={rt.name} onClose={onClose} />
        <div role="tablist" aria-label="Pengaturan RT" style={{ display: 'flex', justifyContent: 'center', gap: 6, margin: '0 20px', padding: '10px 0 12px', borderBottom: '1px solid var(--sf-border)' }}>
          <button
            ref={editTabRef}
            type="button"
            role="tab"
            id="rt-tab-ubah"
            aria-selected={activeTab === 'ubah'}
            tabIndex={activeTab === 'ubah' ? 0 : -1}
            className={`sf-btn ${activeTab === 'ubah' ? 'sf-btn-primary' : 'sf-btn-ghost'}`}
            style={{ minWidth: 88, padding: '4px 12px', fontSize: 11 }}
            onClick={() => {
              setActiveTab('ubah')
              if (!rwLoaded && !editLoading) void loadRws(rt)
            }}
            onKeyDown={handleTabKeyDown}
          >
            UBAH
          </button>
          <button
            ref={jabatanTabRef}
            type="button"
            role="tab"
            id="rt-tab-jabatan"
            aria-selected={activeTab === 'jabatan'}
            tabIndex={activeTab === 'jabatan' ? 0 : -1}
            className={`sf-btn ${activeTab === 'jabatan' ? 'sf-btn-primary' : 'sf-btn-ghost'}`}
            style={{ minWidth: 88, padding: '4px 12px', fontSize: 11 }}
            onClick={() => setActiveTab('jabatan')}
            onKeyDown={handleTabKeyDown}
          >
            JABATAN
          </button>
        </div>
        {activeTab === 'ubah' ? (
          <form onSubmit={handleUpdateRt}>
            <div id="rt-panel-ubah" role="tabpanel" aria-labelledby="rt-tab-ubah" tabIndex={0}>
            <ModalBody>
              {editLoading && <div className="sf-loading" role="status">Memuat pilihan RW...</div>}
              {editError && (
                <div className="sf-error" role="alert">
                  {editError}
                  {!rwLoaded && !editLoading && (
                    <button type="button" className="sf-btn sf-btn-ghost" onClick={() => void loadRws(rt)} style={{ marginLeft: 8 }}>
                      Coba Lagi
                    </button>
                  )}
                </div>
              )}
              {editValidationErrors.length > 0 && (
                <div className="sf-form-error-list" role="alert">
                  <ul>{editValidationErrors.map((message) => <li key={message}>{message}</li>)}</ul>
                </div>
              )}
              {editSuccess && (
                <div
                  role="status"
                  style={{
                    marginBottom: 12,
                    padding: '8px 10px',
                    borderRadius: 6,
                    color: 'var(--sf-success)',
                    background: 'var(--sf-success-soft)',
                  }}
                >
                  {editSuccess}
                </div>
              )}
              <div style={{ marginBottom: 12, fontSize: 13, color: 'var(--sf-text-muted)' }}>
                Status: {rt.is_active ? 'Aktif' : 'Tidak Aktif'}
              </div>
              <RtFormFields
                form={rtForm}
                onChange={handleRtFormChange}
                disabled={editSaving}
                rwList={rwList}
              />
            </ModalBody>
            </div>
            <ModalFooter>
              <button type="button" className="sf-btn sf-btn-ghost" onClick={onClose}>Tutup</button>
              <button type="submit" className="sf-btn sf-btn-primary" disabled={editSaving}>
                {editSaving ? 'Menyimpan...' : 'Simpan Perubahan'}
              </button>
            </ModalFooter>
          </form>
        ) : (
          <>
        <div id="rt-panel-jabatan" role="tabpanel" aria-labelledby="rt-tab-jabatan" tabIndex={0}>
        <ModalBody>
          <div style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
              RW: {String(rt.rw).padStart(3, '0')}
            </div>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
              Status: {rt.is_active ? 'Aktif' : 'Tidak Aktif'}
            </div>
            {rt.head_name && <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Ketua: {rt.head_name}</div>}
            {rt.address && <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Alamat: {rt.address}</div>}
          </div>

          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Pengurus RT</div>

          {loading && <div className="sf-loading" role="status">Memuat pengurus...</div>}
          {error && (
            <div className="sf-error" role="alert">
              {error}
              <button
                type="button"
                className="sf-btn sf-btn-ghost"
                onClick={() => void loadMembers(rt.id)}
                disabled={loading}
                style={{ marginLeft: 8 }}
              >
                Coba Lagi
              </button>
            </div>
          )}

          {!loading && !error && members.length === 0 && (
            <div className="sf-empty">Belum ada pengurus RT yang ditetapkan.</div>
          )}

          {!loading && !error && members.filter((m) => m.jabatan).length === 0 && members.length > 0 && (
            <div className="sf-empty">Belum ada pengurus RT yang ditetapkan.</div>
          )}

          {!loading && !error && members.filter((m) => m.jabatan).length > 0 && (
            <div style={{ border: '1px solid var(--sf-border)', borderRadius: 8, marginBottom: 12, padding: '0 12px', maxHeight: 200, overflowY: 'auto' }}>
              {members.filter((m) => m.jabatan).map((m) => (
                <div
                  key={m.user_id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 12,
                    padding: '6px 0',
                    borderBottom: '1px solid var(--sf-border)',
                  }}
                >
                  <div>
                    <div style={{ fontWeight: 600, fontSize: 13 }}>{m.full_name || m.user_id}</div>
                    <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                      {label(m.jabatan)}
                      {m.email ? ` · ${m.email}` : ''}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 6, alignItems: 'center', flexShrink: 0 }}>
                    <Badge variant={positionVariant(m.jabatan)}>{label(m.jabatan)}</Badge>
                    <button
                      type="button"
                      className="sf-btn sf-btn-danger sf-btn-sm"
                      onClick={() => {
                        setRevokeTarget(m)
                        setModalError(null)
                        setActionMessage(null)
                      }}
                      disabled={modalSaving}
                    >
                      Cabut
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Assignment form */}
          <form onSubmit={handleSaveJabatan} style={{ marginTop: 8, padding: 12, border: '1px solid var(--sf-border)', borderRadius: 8 }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Tetapkan Jabatan RT</div>
            {modalError && <div className="sf-form-error-list" style={{ marginBottom: 8 }}>{modalError}</div>}
            {actionMessage && (
              <div
                role="status"
                style={{
                  marginBottom: 8,
                  padding: '8px 10px',
                  borderRadius: 6,
                  color: 'var(--sf-success)',
                  background: 'var(--sf-success-soft)',
                }}
              >
                {actionMessage}
              </div>
            )}

            <div style={{ marginBottom: 8 }}>
              <label htmlFor="rt-jabatan" style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>Jabatan</label>
              <select
                id="rt-jabatan"
                value={selectedJabatan}
                onChange={(e) => {
                  setSelectedJabatan(e.target.value)
                  setModalError(null)
                  setActionMessage(null)
                }}
                className="sf-form-input"
                style={{ width: '100%' }}
                disabled={modalSaving}
                required
              >
                <option value="">Pilih jabatan...</option>
                {positions.map(([key, lbl]) => (
                  <option key={key} value={key}>{lbl}</option>
                ))}
              </select>
            </div>
            <div style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>Warga</label>
              <input
                type="text"
                className="sf-form-input"
                readOnly
                value={selectedMember ? selectedMember.full_name || selectedMember.user_id : 'Pilih Warga...'}
                onClick={() => {
                  setSelectorOpen(true)
                  setModalError(null)
                  setActionMessage(null)
                }}
                disabled={modalSaving || loading || !!error}
                style={{ cursor: 'pointer' }}
                aria-label={selectedMember ? `Ganti warga, ${selectedMember.full_name}` : 'Pilih warga'}
              />
            </div>

            {selectedMember && (
              <div style={{ marginBottom: 8, padding: 8, background: 'var(--sf-bg-muted)', borderRadius: 6, fontSize: 13 }} aria-live="polite">
                <div style={{ fontWeight: 600 }}>{selectedMember.full_name || selectedMember.user_id}</div>
                <div style={{ color: 'var(--sf-text-muted)', fontSize: 12 }}>
                  {label(selectedJabatan)}
                  {selectedMember.email && <span> · {selectedMember.email}</span>}
                  {selectedMember.phone && <span> · {selectedMember.phone}</span>}
                </div>
              </div>
            )}

            <button
              type="submit"
              className="sf-btn sf-btn-primary"
              disabled={modalSaving || !canSave}
            >
              {modalSaving ? 'Menyimpan...' : 'Simpan'}
            </button>
          </form>

          {revokeTarget && (
            <div style={{ marginTop: 8, padding: 8, border: '1px solid var(--sf-danger)', borderRadius: 6 }}>
              <div style={{ fontSize: 13, marginBottom: 8 }} role="alert">
                Cabut jabatan {label(revokeTarget.jabatan)} dari {revokeTarget.full_name || revokeTarget.user_id}?
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <button
                  type="button"
                  className="sf-btn sf-btn-danger sf-btn-sm"
                  onClick={() => void handleRevoke()}
                  disabled={modalSaving || submitLock.current}
                >
                  {modalSaving ? 'Mencabut...' : 'Ya, Cabut'}
                </button>
                <button
                  type="button"
                  className="sf-btn sf-btn-ghost"
                  onClick={() => setRevokeTarget(null)}
                  disabled={modalSaving}
                >
                  Batal
                </button>
              </div>
            </div>
          )}
        </ModalBody>
        </div>
        <ModalFooter>
          <button className="sf-btn sf-btn-ghost" onClick={onClose}>Tutup</button>
        </ModalFooter>
          </>
        )}
      </Modal>

      {/* Warga selector modal */}
      {selectorOpen && (
        <Modal open={selectorOpen} onClose={() => setSelectorOpen(false)} width={520}>
          <ModalHeader title="Pilih Warga" subtitle={`Warga RT ${rt.rt}`} onClose={() => setSelectorOpen(false)} />
          <ModalBody>
            <input
              id="rt-member-search"
              type="text"
              className="sf-form-input"
              placeholder="Cari nama, email, telepon, atau ID..."
              value={selectorSearch}
              onChange={(e) => setSelectorSearch(e.target.value)}
              style={{ marginBottom: 12, width: '100%' }}
              aria-label="Cari warga berdasarkan nama, email, telepon, atau ID"
            />
            <div style={{ maxHeight: 320, overflowY: 'auto' }}>
              {loading && <div className="sf-loading" role="status">Memuat warga...</div>}
              {error && (
                <div className="sf-error" role="alert">
                  {error}
                  <button
                    type="button"
                    className="sf-btn sf-btn-ghost"
                    onClick={() => void loadMembers(rt.id)}
                    disabled={loading}
                    style={{ marginLeft: 8 }}
                  >
                    Coba Lagi
                  </button>
                </div>
              )}
              {members.length === 0 && !loading && !error && (
                <div className="sf-empty">Belum ada warga di RT ini.</div>
              )}
              {!loading && !error && filteredMembers.map((m) => (
                <button
                  type="button"
                  key={m.user_id}
                  onClick={() => {
                    setSelectedMember(m)
                    setSelectorOpen(false)
                    setSelectorSearch('')
                    setModalError(null)
                    setActionMessage(null)
                  }}
                  disabled={modalSaving}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 12,
                    width: '100%',
                    padding: '8px 0',
                    border: 0,
                    borderBottom: '1px solid var(--sf-border)',
                    background: 'transparent',
                    color: 'var(--sf-text)',
                    textAlign: 'left',
                    cursor: 'pointer',
                  }}
                >
                  <span>
                    <span style={{ display: 'block', fontWeight: 600, fontSize: 13 }}>{m.full_name || m.user_id}</span>
                    <span style={{ display: 'block', fontSize: 12, color: 'var(--sf-text-muted)' }}>
                      {m.email || 'Email tidak tersedia'}
                      {m.phone ? ` · ${m.phone}` : ' · Telepon tidak tersedia'}
                      {m.role ? ` · ${m.role}` : ''}
                    </span>
                  </span>
                  {m.jabatan && <Badge variant={positionVariant(m.jabatan)}>{label(m.jabatan)}</Badge>}
                </button>
              ))}
              {!loading && !error && filteredMembers.length === 0 && members.length > 0 && (
                <div className="sf-empty">Tidak ada warga yang cocok.</div>
              )}
            </div>
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn sf-btn-ghost" onClick={() => { setSelectorOpen(false); setSelectorSearch('') }}>
              Batal
            </button>
          </ModalFooter>
        </Modal>
      )}
    </>
  )
}
