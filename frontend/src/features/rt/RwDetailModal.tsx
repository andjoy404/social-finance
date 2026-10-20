import { useCallback, useEffect, useRef, useState } from 'react'
import {
  apiListRTs,
  apiListRWMemberships,
  apiSetRWJabatan,
  apiRemoveRWJabatan,
  apiListRTMemberships,
  apiUpdateRW,
  getSessionPair,
  ApiError,
  type ApiRT,
  type ApiRW,
  type ApiRWMembership,
  type ApiRTMembership,
} from '@/app/api'
import { Badge } from '@/components/Badge'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { positionVariant, rwPositionLabel } from './position-badge'

const rwPositions = [
  ['ketua', 'Ketua'], ['wakil_ketua', 'Wakil Ketua'], ['sekretaris', 'Sekretaris'], ['bendahara', 'Bendahara'],
]
const label = rwPositionLabel

export function RwDetailModal({
  open,
  rw,
  onClose,
  onUpdated,
}: {
  open: boolean
  rw: ApiRW | null
  onClose: () => void
  onEdit?: () => void
  onUpdated?: (rw: ApiRW) => void
}) {
  const [rts, setRts] = useState<ApiRT[]>([])
  const [rtsLoading, setRtsLoading] = useState(false)
  const [rtsError, setRtsError] = useState<string | null>(null)

  const [members, setMembers] = useState<ApiRWMembership[]>([])
  const [membersLoading, setMembersLoading] = useState(false)
  const [membersError, setMembersError] = useState<string | null>(null)

  // Assignment form state
  const [selectedJabatan, setSelectedJabatan] = useState('')
  const [selectedRt, setSelectedRt] = useState<ApiRT | null>(null)
  const [selectedMember, setSelectedMember] = useState<ApiRTMembership | null>(null)
  const [rtSelectorOpen, setRtSelectorOpen] = useState(false)
  const [rtSearch, setRtSearch] = useState('')
  const [memberSelectorOpen, setMemberSelectorOpen] = useState(false)
  const [memberSearch, setMemberSearch] = useState('')
  const [rtMembers, setRtMembers] = useState<ApiRTMembership[]>([])
  const [rtMembersLoading, setRtMembersLoading] = useState(false)
  const [modalSaving, setModalSaving] = useState(false)
  const [modalError, setModalError] = useState<string | null>(null)
  const [confirmRevoke, setConfirmRevoke] = useState(false)
  const [revokeTarget, setRevokeTarget] = useState<ApiRWMembership | null>(null)
  const [activeTab, setActiveTab] = useState<'ubah' | 'jabatan'>('jabatan')
  const [rwNumber, setRwNumber] = useState('')
  const [rwName, setRwName] = useState('')
  const [rwHead, setRwHead] = useState('')
  const [rwAddress, setRwAddress] = useState('')
  const [editSaving, setEditSaving] = useState(false)
  const [editError, setEditError] = useState<string | null>(null)
  const [editSuccess, setEditSuccess] = useState<string | null>(null)
  const editLock = useRef(false)
  const editTabRef = useRef<HTMLButtonElement>(null)
  const jabatanTabRef = useRef<HTMLButtonElement>(null)

  const loadRts = useCallback(async (rwId: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    setRtsLoading(true)
    setRtsError(null)
    try {
      const resp = await apiListRTs(session.accessToken, { page: 1, page_size: 100, rw_id: rwId })
      setRts(resp.data ?? [])
    } catch (err) {
      setRtsError(err instanceof Error ? err.message : 'Gagal memuat daftar RT')
    } finally {
      setRtsLoading(false)
    }
  }, [])

  const loadMembers = useCallback(async (rwId: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    setMembersLoading(true)
    setMembersError(null)
    try {
      setMembers(await apiListRWMemberships(session.accessToken, rwId))
    } catch (err) {
      setMembersError(err instanceof Error ? err.message : 'Gagal memuat pengurus RW')
    } finally {
      setMembersLoading(false)
    }
  }, [])

  const loadRtMembers = useCallback(async (rtId: string) => {
    const session = getSessionPair()
    if (!session?.accessToken) return
    setRtMembersLoading(true)
    try {
      setRtMembers(await apiListRTMemberships(session.accessToken, rtId))
    } catch {
      setRtMembers([])
    } finally {
      setRtMembersLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open && rw?.id) {
      void loadRts(rw.id)
      void loadMembers(rw.id)
      setSelectedJabatan('')
      setSelectedRt(null)
      setSelectedMember(null)
      setModalError(null)
      setConfirmRevoke(false)
      setActiveTab('jabatan')
      setRwNumber(String(rw.rw_number))
      setRwName(rw.name ?? '')
      setRwHead(rw.head_name ?? '')
      setRwAddress(rw.address ?? '')
      setEditError(null)
      setEditSuccess(null)
    } else {
      setRts([])
      setMembers([])
      setSelectedRt(null)
      setSelectedMember(null)
      setSelectedJabatan('')
      setActiveTab('jabatan')
      setEditError(null)
      setEditSuccess(null)
    }
  }, [open, rw?.id, loadRts, loadMembers])

  const handleUpdateRw = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!rw || editLock.current) return
    const number = Number(rwNumber)
    if (!rwNumber || !Number.isInteger(number) || number <= 0) {
      setEditError('Nomor RW harus angka bulat positif.')
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
      const updatedRw = await apiUpdateRW(session.accessToken, rw.id, {
        rw_number: number,
        name: rwName.trim() || `RW ${String(number).padStart(3, '0')}`,
        head_name: rwHead.trim() || null,
        address: rwAddress.trim() || null,
      })
      setRwNumber(String(updatedRw.rw_number))
      setRwName(updatedRw.name)
      setRwHead(updatedRw.head_name ?? '')
      setRwAddress(updatedRw.address ?? '')
      setEditSuccess('Informasi RW berhasil diperbarui.')
      onUpdated?.(updatedRw)
    } catch (err) {
      setEditError(err instanceof ApiError ? err.message : 'Gagal memperbarui RW.')
    } finally {
      editLock.current = false
      setEditSaving(false)
    }
  }

  const handleTabKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return
    e.preventDefault()
    const nextTab = activeTab === 'ubah' ? 'jabatan' : 'ubah'
    setActiveTab(nextTab)
    if (nextTab === 'ubah') editTabRef.current?.focus()
    else jabatanTabRef.current?.focus()
  }

  const filteredRts = rts.filter((rt) => {
    if (!rtSearch.trim()) return true
    const q = rtSearch.toLowerCase()
    return (rt.name ?? '').toLowerCase().includes(q) || String(rt.rt).includes(q)
  })

  const rwMemberIds = new Set(members.filter((m) => m.jabatan).map((m) => m.user_id))

  const filteredMembers = rtMembers.filter((m) => {
    if (m.jabatan) return false
    if (rwMemberIds.has(m.user_id)) return false
    if (!memberSearch.trim()) return true
    const q = memberSearch.toLowerCase()
    return (m.full_name ?? '').toLowerCase().includes(q) ||
      (m.email ?? '').toLowerCase().includes(q) ||
      (m.phone ?? '').toLowerCase().includes(q)
  })

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedMember || !rw?.id || !selectedJabatan || !selectedRt) return
    const session = getSessionPair()
    if (!session?.accessToken) return

    setModalSaving(true)
    setModalError(null)
    try {
      await apiSetRWJabatan(session.accessToken, rw.id, selectedMember.user_id, selectedRt.id, selectedJabatan)
      await loadMembers(rw.id)
      setSelectedMember(null)
      setSelectedRt(null)
      setSelectedJabatan('')
    } catch (err) {
      setModalError(err instanceof Error ? err.message : 'Gagal menyimpan jabatan RW')
    } finally {
      setModalSaving(false)
    }
  }

  const handleRevoke = async () => {
    if (!revokeTarget || !rw?.id) return
    const session = getSessionPair()
    if (!session?.accessToken) return
    setModalSaving(true)
    setModalError(null)
    try {
      await apiRemoveRWJabatan(session.accessToken, rw.id, revokeTarget.user_id)
      await loadMembers(rw.id)
      setConfirmRevoke(false)
      setRevokeTarget(null)
    } catch (err) {
      setModalError(err instanceof Error ? err.message : 'Gagal mencabut jabatan RW')
    } finally {
      setModalSaving(false)
    }
  }

  if (!open || !rw) return null

  const canSave = selectedJabatan !== '' && selectedRt !== null && selectedMember !== null

  return (
    <>
      <Modal open={open} onClose={onClose} width={720}>
        <ModalHeader title={`Detail RW ${String(rw.rw_number).padStart(3, '0')}`} subtitle={rw.name} onClose={onClose} />
        <div role="tablist" aria-label="Pengaturan RW" style={{ display: 'flex', justifyContent: 'center', gap: 6, margin: '0 20px', padding: '10px 0 12px', borderBottom: '1px solid var(--sf-border)' }}>
          <button
            ref={editTabRef}
            type="button"
            role="tab"
            id="rw-tab-ubah"
            aria-selected={activeTab === 'ubah'}
            tabIndex={activeTab === 'ubah' ? 0 : -1}
            className={`sf-btn ${activeTab === 'ubah' ? 'sf-btn-primary' : 'sf-btn-ghost'}`}
            style={{ minWidth: 88, padding: '4px 12px', fontSize: 11 }}
            onClick={() => setActiveTab('ubah')}
            onKeyDown={handleTabKeyDown}
          >
            UBAH
          </button>
          <button
            ref={jabatanTabRef}
            type="button"
            role="tab"
            id="rw-tab-jabatan"
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
          <form onSubmit={handleUpdateRw}>
            <div id="rw-panel-ubah" role="tabpanel" aria-labelledby="rw-tab-ubah" tabIndex={0}>
              <ModalBody>
                {editError && <div className="sf-error" role="alert">{editError}</div>}
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
                  Status: {rw.is_active ? 'Aktif' : 'Nonaktif'}
                </div>
                <div className="sf-form-grid-2">
                  <div className="sf-form-field">
                    <label className="sf-form-label" htmlFor="rw-detail-number">Nomor RW *</label>
                    <input
                      id="rw-detail-number"
                      className="sf-form-input"
                      type="number"
                      min="1"
                      value={rwNumber}
                      onChange={(e) => { setRwNumber(e.target.value); setEditError(null); setEditSuccess(null) }}
                      required
                      disabled={editSaving}
                    />
                  </div>
                  <div className="sf-form-field">
                    <label className="sf-form-label" htmlFor="rw-detail-name">Nama RW</label>
                    <input
                      id="rw-detail-name"
                      className="sf-form-input"
                      value={rwName}
                      onChange={(e) => { setRwName(e.target.value); setEditError(null); setEditSuccess(null) }}
                      disabled={editSaving}
                    />
                  </div>
                  <div className="sf-form-field">
                    <label className="sf-form-label" htmlFor="rw-detail-head">Ketua RW</label>
                    <input
                      id="rw-detail-head"
                      className="sf-form-input"
                      value={rwHead}
                      onChange={(e) => { setRwHead(e.target.value); setEditError(null); setEditSuccess(null) }}
                      disabled={editSaving}
                    />
                  </div>
                  <div className="sf-form-field">
                    <label className="sf-form-label" htmlFor="rw-detail-address">Alamat</label>
                    <input
                      id="rw-detail-address"
                      className="sf-form-input"
                      value={rwAddress}
                      onChange={(e) => { setRwAddress(e.target.value); setEditError(null); setEditSuccess(null) }}
                      disabled={editSaving}
                    />
                  </div>
                </div>
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
        <div id="rw-panel-jabatan" role="tabpanel" aria-labelledby="rw-tab-jabatan" tabIndex={0}>
        <ModalBody>
          <div style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
              Status: {rw.is_active ? 'Aktif' : 'Nonaktif'}
            </div>
            {rw.head_name && <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Ketua: {rw.head_name}</div>}
            {rw.address && <div style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Alamat: {rw.address}</div>}
          </div>

          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Daftar RT di RW ini</div>
          {rtsLoading && <div className="sf-loading">Memuat RT...</div>}
          {rtsError && <div className="sf-error">{rtsError}</div>}
          {!rtsLoading && !rtsError && rts.length === 0 && (
            <div className="sf-empty">Belum ada RT terdaftar di RW ini.</div>
          )}
          {!rtsLoading && !rtsError && rts.length > 0 && (
            <div style={{ maxHeight: 160, overflowY: 'auto', marginBottom: 12 }}>
              {rts.map((rt) => (
                <div key={rt.id} style={{ padding: '4px 0', borderBottom: '1px solid var(--sf-border)', fontSize: 13 }}>
                  RT {rt.rt} — {rt.name} ({rt.is_active ? 'Aktif' : 'Nonaktif'})
                </div>
              ))}
            </div>
          )}

          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, marginTop: 12 }}>Pengurus RW</div>
          {membersLoading && <div className="sf-loading">Memuat pengurus RW...</div>}
          {membersError && <div className="sf-error">{membersError}</div>}
          {!membersLoading && !membersError && members.length === 0 && (
            <div className="sf-empty">Belum ada pengurus RW yang ditetapkan.</div>
          )}
          {!membersLoading && !membersError && members.filter((m) => m.jabatan).length === 0 && members.length > 0 && (
            <div className="sf-empty">Belum ada pengurus RW yang ditetapkan.</div>
          )}
          {!membersLoading && !membersError && members.filter((m) => m.jabatan).length > 0 && (
            <div style={{ maxHeight: 160, overflowY: 'auto', marginBottom: 12 }}>
              {members.filter((m) => m.jabatan).map((m) => (
                <div key={m.user_id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--sf-border)' }}>
                  <div>
                    <div style={{ fontWeight: 600, fontSize: 13 }}>{m.full_name || m.user_id}</div>
                    <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                      {label(m.jabatan)}
                      {m.rt_name ? ` · ${m.rt_name}` : ''}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
                    <Badge variant={positionVariant(m.jabatan)}>{label(m.jabatan)}</Badge>
                    <button
                      className="sf-btn sf-btn-danger sf-btn-sm"
                      onClick={() => { setRevokeTarget(m); setConfirmRevoke(true) }}
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
          <form onSubmit={handleSave} style={{ marginTop: 8, padding: 12, border: '1px solid var(--sf-border)', borderRadius: 8 }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8 }}>Tetapkan Jabatan RW</div>
            {modalError && <div className="sf-form-error-list" style={{ marginBottom: 8 }}>{modalError}</div>}

            <div style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>Jabatan</label>
              <select
                value={selectedJabatan}
                onChange={(e) => setSelectedJabatan(e.target.value)}
                className="sf-form-input"
                style={{ width: '100%' }}
                disabled={modalSaving}
              >
                <option value="">Pilih jabatan RW...</option>
                {rwPositions.map(([key, lbl]) => (
                  <option key={key} value={key}>{lbl}</option>
                ))}
              </select>
            </div>

            <div style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>RT</label>
              <input
                type="text"
                className="sf-form-input"
                readOnly
                value={selectedRt ? `RT ${selectedRt.rt} — ${selectedRt.name}` : 'Pilih RT...'}
                onClick={() => setRtSelectorOpen(true)}
                disabled={modalSaving}
                style={{ cursor: 'pointer' }}
              />
            </div>

            <div style={{ marginBottom: 8 }}>
              <label style={{ fontSize: 12, fontWeight: 600, display: 'block', marginBottom: 4 }}>Warga</label>
              <input
                type="text"
                className="sf-form-input"
                readOnly
                value={selectedMember ? (selectedMember.full_name || selectedMember.user_id) : 'Pilih Warga...'}
                onClick={() => { if (selectedRt) setMemberSelectorOpen(true) }}
                disabled={modalSaving || !selectedRt}
                style={{ width: '100%', cursor: 'pointer' }}
              />
            </div>

            {selectedRt && selectedMember && (
              <div style={{ marginBottom: 8, padding: 8, background: 'var(--sf-bg-muted)', borderRadius: 6, fontSize: 13 }}>
                <div style={{ fontWeight: 600 }}>{selectedMember.full_name || selectedMember.user_id}</div>
                <div style={{ color: 'var(--sf-text-muted)', fontSize: 12 }}>
                  RT {selectedRt.rt} · {selectedRt.name}
                  {selectedMember.email && <span> · {selectedMember.email}</span>}
                </div>
              </div>
            )}

            <div style={{ display: 'flex', gap: 8 }}>
              <button
                type="submit"
                className="sf-btn sf-btn-primary"
                disabled={modalSaving || !canSave}
              >
                {modalSaving ? 'Menyimpan...' : 'Simpan'}
              </button>
            </div>
          </form>

          {confirmRevoke && revokeTarget && (
            <div style={{ marginTop: 8, padding: 8, border: '1px solid var(--sf-danger)', borderRadius: 6 }}>
              <div style={{ fontSize: 13, marginBottom: 8 }}>
                Cabut jabatan {label(revokeTarget.jabatan)} dari {revokeTarget.full_name || revokeTarget.user_id}?
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <button className="sf-btn sf-btn-danger sf-btn-sm" onClick={() => void handleRevoke()} disabled={modalSaving}>
                  {modalSaving ? 'Mencabut...' : 'Ya, Cabut'}
                </button>
                <button className="sf-btn sf-btn-ghost" onClick={() => { setConfirmRevoke(false); setRevokeTarget(null) }} disabled={modalSaving}>
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

      {/* RT selector modal */}
      {rtSelectorOpen && (
        <Modal open={rtSelectorOpen} onClose={() => setRtSelectorOpen(false)} width={480}>
          <ModalHeader title="Pilih RT" subtitle={`RT dalam RW ${rw.rw_number}`} onClose={() => setRtSelectorOpen(false)} />
          <ModalBody>
            <input
              type="text"
              className="sf-form-input"
              placeholder="Cari RT..."
              value={rtSearch}
              onChange={(e) => setRtSearch(e.target.value)}
              style={{ marginBottom: 12, width: '100%' }}
            />
            <div style={{ maxHeight: 320, overflowY: 'auto' }}>
              {rtsLoading && <div className="sf-loading">Memuat RT...</div>}
              {!rtsLoading && rts.length === 0 && <div className="sf-empty">Belum ada RT di RW ini.</div>}
              {filteredRts.map((rt) => (
                <div
                  key={rt.id}
                  style={{ padding: '8px 0', borderBottom: '1px solid var(--sf-border)', cursor: 'pointer' }}
                  onClick={() => {
                    setSelectedRt(rt)
                    setSelectedMember(null)
                    setRtSelectorOpen(false)
                    setRtSearch('')
                    void loadRtMembers(rt.id)
                  }}
                >
                  <div style={{ fontWeight: 600, fontSize: 13 }}>RT {rt.rt} — {rt.name}</div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    {rt.is_active ? 'Aktif' : 'Nonaktif'} {rt.head_name ? `· ${rt.head_name}` : ''}
                  </div>
                </div>
              ))}
              {!rtsLoading && filteredRts.length === 0 && rts.length > 0 && (
                <div className="sf-empty">Tidak ada RT yang cocok.</div>
              )}
            </div>
          </ModalBody>
          <ModalFooter>
            <button className="sf-btn sf-btn-ghost" onClick={() => { setRtSelectorOpen(false); setRtSearch('') }}>Batal</button>
          </ModalFooter>
        </Modal>
      )}

      {/* Warga selector modal */}
      {memberSelectorOpen && (
        <Modal open={memberSelectorOpen} onClose={() => setMemberSelectorOpen(false)} width={520}>
          <ModalHeader title="Pilih Warga" subtitle={selectedRt ? `Warga RT ${selectedRt.rt}` : ''} onClose={() => setMemberSelectorOpen(false)} />
          <ModalBody>
            <input
              type="text"
              className="sf-form-input"
              placeholder="Cari nama, email, telepon..."
              value={memberSearch}
              onChange={(e) => setMemberSearch(e.target.value)}
              style={{ marginBottom: 12, width: '100%' }}
            />
            <div style={{ maxHeight: 320, overflowY: 'auto' }}>
              {rtMembersLoading && <div className="sf-loading">Memuat warga...</div>}
              {!rtMembersLoading && rtMembers.length === 0 && <div className="sf-empty">Belum ada warga di RT ini.</div>}
              {filteredMembers.map((m) => (
                <div
                  key={m.user_id}
                  style={{ padding: '8px 0', borderBottom: '1px solid var(--sf-border)', cursor: 'pointer' }}
                  onClick={() => {
                    setSelectedMember(m)
                    setMemberSelectorOpen(false)
                    setMemberSearch('')
                  }}
                >
                  <div style={{ fontWeight: 600, fontSize: 13 }}>{m.full_name || m.user_id}</div>
                  <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    {m.email && <span>{m.email}</span>}
                    {m.phone && <span> · {m.phone}</span>}
                  </div>
                </div>
              ))}
              {!rtMembersLoading && filteredMembers.length === 0 && rtMembers.length > 0 && (
                <div className="sf-empty">Tidak ada warga yang cocok.</div>
              )}
            </div>
          </ModalBody>
          <ModalFooter>
            <button className="sf-btn sf-btn-ghost" onClick={() => { setMemberSelectorOpen(false); setMemberSearch('') }}>Batal</button>
          </ModalFooter>
        </Modal>
      )}
    </>
  )
}
