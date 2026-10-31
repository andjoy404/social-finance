/**
 * RW Fund Location List — Pengelolaan & Monitoring Lokasi Dana RW.
 *
 * Mendukung:
 *   - Rekening Bank, Brankas Fisik, Kas Di Tangan (COH)
 *   - CRUD Lokasi Dana RW (untuk role berwenang: Superadmin, Bendahara RW, Ketua RW)
 *   - Transfer Antar-Lokasi RW
 *   - Riwayat Mutasi (Ledger LBC)
 *   - Rekonsiliasi Saldo Kas vs Total Lokasi
 */

import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  BankOutlined,
  WalletOutlined,
  SafetyCertificateOutlined,
  InfoCircleOutlined,
  PlusOutlined,
  SwapOutlined,
  HistoryOutlined,
  EditOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  PoweroffOutlined,
} from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { useAuth } from '@/app/AuthContext'
import { getSessionPair } from '@/app/api'
import { formatRupiah } from '@/styles/format'
import {
  apiListRWLocations,
  apiCreateRWLocation,
  apiUpdateRWLocation,
  apiToggleRWLocationStatus,
  apiCreateRWTransfer,
  apiListRWTransfers,
  apiReverseRWTransfer,
  apiGetRWLocationMutations,
  apiGetRWReconciliation,
  type ApiRWLocation,
  type ApiRWFundTransfer,
  type ApiRWLocationMutation,
  type ApiRWReconciliation,
} from './rw-api'

export function RwFundLocationList({ rwId }: { rwId: string }) {
  const { user } = useAuth()

  const isSuperAdmin = user?.systemRole === 'super_admin' || user?.role === 'super_admin'
  const rwMemberships = (user as any)?.rw_memberships || (user as any)?.rwMemberships || []
  const currentRw = rwMemberships.find((m: any) => m.rw_id === rwId)
  const jabatan = (currentRw?.jabatan || user?.jabatan || '').toLowerCase()

  const canCreate = isSuperAdmin || ['ketua', 'bendahara', 'wakil_ketua'].includes(jabatan)
  const canUpdate = isSuperAdmin || ['ketua', 'bendahara'].includes(jabatan)
  const isReadOnly = !canCreate && !canUpdate

  const [locations, setLocations] = useState<ApiRWLocation[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Filters
  const [search, setSearch] = useState('')
  const [categoryFilter, setCategoryFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [editingLocation, setEditingLocation] = useState<ApiRWLocation | null>(null)
  const [showTransferModal, setShowTransferModal] = useState(false)
  const [showReconcileModal, setShowReconcileModal] = useState(false)
  const [historyLocation, setHistoryLocation] = useState<ApiRWLocation | null>(null)
  const [historyData, setHistoryData] = useState<ApiRWLocationMutation[]>([])
  const [loadingHistory, setLoadingHistory] = useState(false)
  const [reconcileData, setReconcileData] = useState<ApiRWReconciliation | null>(null)
  const [loadingReconcile, setLoadingReconcile] = useState(false)

  // Transfer History & Reversal states
  const [showTransfersModal, setShowTransfersModal] = useState(false)
  const [transfers, setTransfers] = useState<ApiRWFundTransfer[]>([])
  const [loadingTransfers, setLoadingTransfers] = useState(false)
  const [transfersError, setTransfersError] = useState<string | null>(null)
  const [reversingTransfer, setReversingTransfer] = useState<ApiRWFundTransfer | null>(null)
  const [reversalReason, setReversalReason] = useState('')
  const [submittingReversal, setSubmittingReversal] = useState(false)
  const [reversalError, setReversalError] = useState<string | null>(null)

  // Form states
  const [createForm, setCreateForm] = useState<{
    name: string
    category: 'rekening' | 'brankas' | 'coh'
    bank_name: string
    account_number: string
    account_holder: string
  }>({
    name: '',
    category: 'rekening',
    bank_name: '',
    account_number: '',
    account_holder: '',
  })

  const [transferForm, setTransferForm] = useState<{
    from_location_id: string
    to_location_id: string
    amount: string
    notes: string
  }>({
    from_location_id: '',
    to_location_id: '',
    amount: '',
    notes: '',
  })

  const [actionError, setActionError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const loadData = useCallback(async () => {
    const token = getSessionPair()?.accessToken
    if (!token) return
    setLoading(true)
    setError(null)
    try {
      const locs = await apiListRWLocations(token, rwId)
      setLocations(locs)
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat data lokasi dana RW')
    } finally {
      setLoading(false)
    }
  }, [rwId])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Category summary
  const categorySummary = useMemo(() => {
    let rekeningTotal = 0
    let rekeningCount = 0
    let brankasTotal = 0
    let brankasCount = 0
    let cohTotal = 0
    let cohCount = 0

    locations.forEach((loc) => {
      const bal = typeof loc.current_balance === 'string'
        ? parseFloat(loc.current_balance) || 0
        : loc.current_balance || 0
      if (loc.category === 'rekening') {
        rekeningTotal += bal
        if (loc.is_active) rekeningCount++
      } else if (loc.category === 'brankas') {
        brankasTotal += bal
        if (loc.is_active) brankasCount++
      } else if (loc.category === 'coh') {
        cohTotal += bal
        if (loc.is_active) cohCount++
      }
    })

    return {
      rekening: { total: rekeningTotal, count: rekeningCount },
      brankas: { total: brankasTotal, count: brankasCount },
      coh: { total: cohTotal, count: cohCount },
    }
  }, [locations])

  // Filtered locations
  const filteredLocations = useMemo(() => {
    return locations.filter((loc) => {
      if (categoryFilter !== 'all' && loc.category !== categoryFilter) return false
      if (statusFilter === 'active' && !loc.is_active) return false
      if (statusFilter === 'inactive' && loc.is_active) return false
      if (search.trim()) {
        const q = search.toLowerCase()
        const matchesName = loc.name.toLowerCase().includes(q)
        const matchesRt = loc.rt_name?.toLowerCase().includes(q) ?? false
        const matchesBank = loc.bank_name?.toLowerCase().includes(q) ?? false
        const matchesAcc = loc.account_number?.toLowerCase().includes(q) ?? false
        const matchesHolder = loc.account_holder?.toLowerCase().includes(q) ?? false
        if (!matchesName && !matchesRt && !matchesBank && !matchesAcc && !matchesHolder) return false
      }
      return true
    })
  }, [locations, categoryFilter, statusFilter, search])

  const totalBalance = useMemo(() => {
    return locations.reduce((sum, loc) => {
      const bal = typeof loc.current_balance === 'string'
        ? parseFloat(loc.current_balance) || 0
        : loc.current_balance || 0
      return sum + bal
    }, 0)
  }, [locations])

  // Handlers
  const handleCreateLocation = async (e: React.FormEvent) => {
    e.preventDefault()
    const token = getSessionPair()?.accessToken
    if (!token) return
    if (!createForm.name.trim()) {
      setActionError('Nama lokasi wajib diisi')
      return
    }
    if (createForm.category === 'rekening' && !createForm.bank_name.trim()) {
      setActionError('Nama bank wajib diisi untuk kategori Rekening Bank')
      return
    }

    setSubmitting(true)
    setActionError(null)
    try {
      await apiCreateRWLocation(token, rwId, {
        name: createForm.name.trim(),
        category: createForm.category,
        bank_name: createForm.bank_name.trim() || undefined,
        account_number: createForm.account_number.trim() || undefined,
        account_holder: createForm.account_holder.trim() || undefined,
      })
      setShowCreateModal(false)
      setCreateForm({
        name: '',
        category: 'rekening',
        bank_name: '',
        account_number: '',
        account_holder: '',
      })
      loadData()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal menambahkan lokasi dana RW')
    } finally {
      setSubmitting(false)
    }
  }

  const handleUpdateLocation = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingLocation) return
    const token = getSessionPair()?.accessToken
    if (!token) return
    if (!editingLocation.name.trim()) {
      setActionError('Nama lokasi wajib diisi')
      return
    }

    setSubmitting(true)
    setActionError(null)
    try {
      await apiUpdateRWLocation(token, rwId, editingLocation.id, {
        name: editingLocation.name.trim(),
        bank_name: editingLocation.bank_name?.trim() || undefined,
        account_number: editingLocation.account_number?.trim() || undefined,
        account_holder: editingLocation.account_holder?.trim() || undefined,
      })
      setEditingLocation(null)
      loadData()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal memperbarui lokasi dana RW')
    } finally {
      setSubmitting(false)
    }
  }

  const handleToggleStatus = async (loc: ApiRWLocation) => {
    const bal = parseFloat(loc.current_balance) || 0
    if (loc.is_active && bal !== 0) {
      alert(`Lokasi "${loc.name}" masih memiliki saldo ${formatRupiah(bal)}. Saldo harus Rp 0 terlebih dahulu sebelum dapat dinonaktifkan.`)
      return
    }

    if (!confirm(`Apakah Anda yakin ingin ${loc.is_active ? 'menonaktifkan' : 'mengaktifkan'} lokasi "${loc.name}"?`)) {
      return
    }

    const token = getSessionPair()?.accessToken
    if (!token) return
    try {
      await apiToggleRWLocationStatus(token, rwId, loc.id, !loc.is_active)
      loadData()
    } catch (err: any) {
      alert(err?.message || 'Gagal mengubah status lokasi dana')
    }
  }

  const handleTransfer = async (e: React.FormEvent) => {
    e.preventDefault()
    const token = getSessionPair()?.accessToken
    if (!token) return
    if (!transferForm.from_location_id || !transferForm.to_location_id) {
      setActionError('Pilih lokasi sumber dan tujuan')
      return
    }
    if (transferForm.from_location_id === transferForm.to_location_id) {
      setActionError('Lokasi sumber dan tujuan tidak boleh sama')
      return
    }
    const amt = parseFloat(transferForm.amount) || 0
    if (amt <= 0) {
      setActionError('Nominal transfer harus lebih besar dari 0')
      return
    }

    const fromLoc = locations.find((l) => l.id === transferForm.from_location_id)
    const fromBal = parseFloat(fromLoc?.current_balance || '0') || 0
    if (amt > fromBal) {
      setActionError('Saldo lokasi sumber tidak mencukupi')
      return
    }

    setSubmitting(true)
    setActionError(null)
    try {
      const now = new Date()
      const dateStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
      await apiCreateRWTransfer(token, rwId, {
        from_location_id: transferForm.from_location_id,
        to_location_id: transferForm.to_location_id,
        amount: `${amt.toFixed(2)}`,
        transfer_date: dateStr,
        notes: transferForm.notes.trim() || undefined,
      })
      setShowTransferModal(false)
      setTransferForm({
        from_location_id: '',
        to_location_id: '',
        amount: '',
        notes: '',
      })
      loadData()
    } catch (err: any) {
      setActionError(err?.message || 'Gagal melakukan transfer dana')
    } finally {
      setSubmitting(false)
    }
  }

  const openHistory = async (loc: ApiRWLocation) => {
    setHistoryLocation(loc)
    setHistoryData([])
    setLoadingHistory(true)
    const token = getSessionPair()?.accessToken
    if (!token) return
    try {
      const muts = await apiGetRWLocationMutations(token, rwId, loc.id)
      setHistoryData(muts)
    } catch (err: any) {
      alert(err?.message || 'Gagal memuat mutasi lokasi dana')
    } finally {
      setLoadingHistory(false)
    }
  }

  const openReconciliation = async () => {
    setShowReconcileModal(true)
    setReconcileData(null)
    setLoadingReconcile(true)
    const token = getSessionPair()?.accessToken
    if (!token) return
    try {
      const rec = await apiGetRWReconciliation(token, rwId)
      setReconcileData(rec)
    } catch (err: any) {
      alert(err?.message || 'Gagal memuat rekonsiliasi')
    } finally {
      setLoadingReconcile(false)
    }
  }

  const openTransfersHistory = async () => {
    setShowTransfersModal(true)
    setTransfers([])
    setTransfersError(null)
    setLoadingTransfers(true)
    const token = getSessionPair()?.accessToken
    if (!token) {
      setLoadingTransfers(false)
      return
    }
    try {
      const data = await apiListRWTransfers(token, rwId)
      setTransfers(data)
    } catch (err: any) {
      setTransfersError(err?.message || 'Gagal memuat riwayat transfer')
    } finally {
      setLoadingTransfers(false)
    }
  }

  const handleOpenReverse = (t: ApiRWFundTransfer) => {
    setReversingTransfer(t)
    setReversalReason('')
    setReversalError(null)
  }

  const handleConfirmReverse = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!reversingTransfer) return
    if (!reversalReason.trim()) {
      setReversalError('Alasan pembatalan (reversal) wajib diisi')
      return
    }
    const token = getSessionPair()?.accessToken
    if (!token) return
    setSubmittingReversal(true)
    setReversalError(null)
    try {
      await apiReverseRWTransfer(token, rwId, reversingTransfer.id, reversalReason.trim())
      setReversingTransfer(null)
      // Refresh transfers list
      const updatedTransfers = await apiListRWTransfers(token, rwId)
      setTransfers(updatedTransfers)
      // Refresh locations list to update balances
      loadData()
    } catch (err: any) {
      setReversalError(err?.message || 'Gagal membatalkan transfer')
    } finally {
      setSubmittingReversal(false)
    }
  }

  if (!user) return null

  const activeLocations = locations.filter((l) => l.is_active)

  return (
    <>
      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <BankOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Lokasi Dana RW — {rwId}</div>
            <div className="sf-page-header-subtitle">
              {canCreate || canUpdate ? 'Kelola rekening, brankas fisik, dan kas di tangan tingkat RW' : 'Lokasi dana RW (Mode Read-Only)'}
            </div>
          </div>
        </div>
      </div>

      {/* Notice / Action Header */}
      {isReadOnly && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '12px 16px',
          margin: '8px 0',
          borderRadius: 'var(--sf-radius-md)',
          background: 'rgba(255, 193, 7, 0.08)',
          border: '1px solid rgba(255, 193, 7, 0.3)',
          fontSize: 13,
          color: 'var(--sf-text)',
        }}>
          <InfoCircleOutlined style={{ color: '#ffc107', fontSize: 16, flexShrink: 0 }} />
          <span>
            <strong>Mode Read-Only:</strong> Anda hanya memiliki hak akses melihat lokasi dana RW.
          </span>
        </div>
      )}

      {/* Summary Cards: Category Totals */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        gap: 12,
        marginTop: 8,
        marginBottom: 2,
      }}>
        {/* Total */}
        <div style={{
          background: 'rgba(169, 112, 255, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(169, 112, 255, 0.1)',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <BankOutlined style={{ color: 'var(--sf-accent)', fontSize: 16 }} />
            <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600 }}>
              Total Saldo
            </div>
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--sf-accent)' }}>
            {loading ? '...' : formatRupiah(totalBalance)}
          </div>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
            {locations.filter((l) => l.is_active).length} lokasi aktif
          </div>
        </div>

        {/* Rekening */}
        <div style={{
          background: 'rgba(59, 130, 246, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(59, 130, 246, 0.1)',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <BankOutlined style={{ color: '#3b82f6', fontSize: 16 }} />
            <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600 }}>
              Rekening Bank
            </div>
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: '#3b82f6' }}>
            {loading ? '...' : formatRupiah(categorySummary.rekening.total)}
          </div>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
            {categorySummary.rekening.count} lokasi aktif
          </div>
        </div>

        {/* Brankas */}
        <div style={{
          background: 'rgba(245, 158, 11, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(245, 158, 11, 0.1)',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <SafetyCertificateOutlined style={{ color: '#f59e0b', fontSize: 16 }} />
            <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600 }}>
              Brankas Fisik
            </div>
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: '#f59e0b' }}>
            {loading ? '...' : formatRupiah(categorySummary.brankas.total)}
          </div>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
            {categorySummary.brankas.count} lokasi aktif
          </div>
        </div>

        {/* COH */}
        <div style={{
          background: 'rgba(52, 211, 153, 0.05)',
          borderRadius: 'var(--sf-radius-md)',
          padding: '14px 16px',
          border: '1px solid rgba(52, 211, 153, 0.1)',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <WalletOutlined style={{ color: '#34d399', fontSize: 16 }} />
            <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600 }}>
              Kas Di Tangan (COH)
            </div>
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: '#34d399' }}>
            {loading ? '...' : formatRupiah(categorySummary.coh.total)}
          </div>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', marginTop: 2 }}>
            {categorySummary.coh.count} lokasi aktif
          </div>
        </div>
      </div>

      {/* Main Content Surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar" style={{ flexWrap: 'wrap', gap: 8 }}>
          <div className="sf-list-toolbar-left" style={{ flexWrap: 'wrap', gap: 8 }}>
            {/* Category Filter */}
            <select
              className="sf-form-select"
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
              style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 'auto', minWidth: 140 }}
              aria-label="Filter Kategori"
            >
              <option value="all">Semua Kategori</option>
              <option value="rekening">Rekening Bank</option>
              <option value="brankas">Brankas Fisik</option>
              <option value="coh">Kas Di Tangan (COH)</option>
            </select>

            {/* Status Filter */}
            <select
              className="sf-form-select"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              style={{ fontSize: 13, height: 34, padding: '4px 10px', width: 'auto', minWidth: 130 }}
              aria-label="Filter Status"
            >
              <option value="all">Semua Status</option>
              <option value="active">Aktif</option>
              <option value="inactive">Nonaktif</option>
            </select>

            {/* Search */}
            <div style={{ minWidth: 200, maxWidth: 320 }}>
              <SearchBox
                value={search}
                onChange={(v) => setSearch(v)}
                placeholder="Cari nama lokasi, RT, bank..."
              />
            </div>
          </div>

            <div className="sf-list-toolbar-right" style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              {canCreate && (
                <button
                  type="button"
                  className="sf-btn sf-btn-primary"
                  onClick={() => {
                    setActionError(null)
                    setShowCreateModal(true)
                  }}
                  style={{ fontSize: 13, display: 'inline-flex', alignItems: 'center', gap: 6 }}
                >
                  <PlusOutlined /> Tambah Lokasi
                </button>
              )}
              {canUpdate && (
                <button
                  type="button"
                  className="sf-btn sf-btn-outline"
                  onClick={() => {
                    setActionError(null)
                    setShowTransferModal(true)
                  }}
                  style={{ fontSize: 13, display: 'inline-flex', alignItems: 'center', gap: 6 }}
                >
                  <SwapOutlined /> Transfer Antar-Lokasi
                </button>
              )}
              <button
                type="button"
                className="sf-btn sf-btn-outline"
                onClick={openTransfersHistory}
                style={{ fontSize: 13, display: 'inline-flex', alignItems: 'center', gap: 6 }}
              >
                <HistoryOutlined /> Riwayat Transfer
              </button>
              <button
                type="button"
                className="sf-btn sf-btn-outline"
                onClick={openReconciliation}
                style={{ fontSize: 13, display: 'inline-flex', alignItems: 'center', gap: 6 }}
              >
                <BankOutlined /> Rekonsiliasi
              </button>
            </div>
        </div>

        {error && (
          <div className="sf-error-banner" role="alert">{error}</div>
        )}

        {loading && (
          <div className="sf-loading-banner">Memuat data lokasi dana RW...</div>
        )}

        {!loading && locations.length === 0 && !error && (
          <div className="sf-empty-banner">
            Belum ada lokasi dana pada RW ini.
          </div>
        )}

        {/* Table */}
        {!loading && filteredLocations.length > 0 && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th style={{ width: 100, minWidth: 100, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    RT/RW
                  </th>
                  <th style={{ minWidth: 200, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    NAMA LOKASI
                  </th>
                  <th style={{ width: 140, minWidth: 140, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    KATEGORI
                  </th>
                  {categoryFilter === 'rekening' && (
                    <>
                      <th style={{ width: 120, minWidth: 120, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                        BANK
                      </th>
                      <th style={{ width: 140, minWidth: 140, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                        NO. REKENING
                      </th>
                      <th style={{ width: 140, minWidth: 140, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                        PEMILIK
                      </th>
                    </>
                  )}
                  <th style={{ width: 140, minWidth: 140, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    SALDO
                  </th>
                  <th style={{ width: 100, minWidth: 100, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    STATUS
                  </th>
                  <th style={{ width: 140, minWidth: 140, textAlign: 'center', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    AKSI
                  </th>
                </tr>
              </thead>
              <tbody>
                {filteredLocations.map((loc) => {
                  const bal = typeof loc.current_balance === 'string'
                    ? parseFloat(loc.current_balance) || 0
                    : loc.current_balance || 0
                  return (
                    <tr key={loc.id}>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 12, color: 'var(--sf-text)', whiteSpace: 'nowrap' }}>
                        {loc.rt_name || loc.rt_id || 'RW'}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', fontWeight: 500, maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {loc.name}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', whiteSpace: 'nowrap' }}>
                        <Badge
                          variant={loc.category === 'rekening' ? 'blue' : loc.category === 'brankas' ? 'amber' : 'green'}
                        >
                          {loc.category === 'rekening' ? 'Rekening Bank' : loc.category === 'brankas' ? 'Brankas Fisik' : 'Kas Di Tangan (COH)'}
                        </Badge>
                      </td>
                      {categoryFilter === 'rekening' && (
                        <>
                          <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', whiteSpace: 'nowrap' }}>
                            {loc.bank_name || '—'}
                          </td>
                          <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 12, fontFamily: 'monospace', color: 'var(--sf-text-muted)', whiteSpace: 'nowrap' }}>
                            {loc.account_number || '—'}
                          </td>
                          <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', whiteSpace: 'nowrap' }}>
                            {loc.account_holder || '—'}
                          </td>
                        </>
                      )}
                      <td style={{
                        padding: '10px 12px',
                        borderBottom: '1px solid var(--sf-border)',
                        textAlign: 'right',
                        fontFamily: 'monospace',
                        fontSize: 13,
                        color: bal > 0 ? 'var(--sf-text)' : 'var(--sf-text-muted)',
                        fontWeight: 600,
                      }}>
                        {formatRupiah(bal)}
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', whiteSpace: 'nowrap' }}>
                        <Badge variant={loc.is_active ? 'green' : 'red'}>
                          {loc.is_active ? 'Aktif' : 'Nonaktif'}
                        </Badge>
                      </td>
                      <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'center', whiteSpace: 'nowrap' }}>
                        <button
                          type="button"
                          className="sf-btn sf-btn-outline sf-btn-sm"
                          onClick={() => openHistory(loc)}
                          title="Riwayat Mutasi"
                          style={{ padding: '4px 8px', marginRight: 4 }}
                        >
                          <HistoryOutlined />
                        </button>
                        {canUpdate && (
                          <>
                            <button
                              type="button"
                              className="sf-btn sf-btn-outline sf-btn-sm"
                              onClick={() => {
                                setActionError(null)
                                setEditingLocation({ ...loc })
                              }}
                              title="Edit"
                              style={{ padding: '4px 8px', marginRight: 4 }}
                            >
                              <EditOutlined />
                            </button>
                            <button
                              type="button"
                              className="sf-btn sf-btn-outline sf-btn-sm"
                              onClick={() => handleToggleStatus(loc)}
                              title={loc.is_active ? 'Nonaktifkan' : 'Aktifkan'}
                              style={{ padding: '4px 8px', color: loc.is_active ? 'var(--sf-danger)' : 'var(--sf-success)' }}
                            >
                              <PoweroffOutlined />
                            </button>
                          </>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal: Tambah Lokasi */}
      {showCreateModal && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 480 }}>
            <div className="sf-modal-header">
              <h3>Tambah Lokasi Dana RW</h3>
              <button type="button" className="sf-modal-close" onClick={() => setShowCreateModal(false)}>✕</button>
            </div>
            <form onSubmit={handleCreateLocation}>
              <div className="sf-modal-body">
                {actionError && <div className="sf-error-banner" style={{ marginBottom: 12 }}>{actionError}</div>}
                <div className="sf-form-group">
                  <label>Nama Lokasi *</label>
                  <input
                    type="text"
                    className="sf-form-input"
                    value={createForm.name}
                    onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                    placeholder="Contoh: Rekening Operasional RW, Brankas Kas RW"
                    required
                  />
                </div>
                <div className="sf-form-group">
                  <label>Kategori *</label>
                  <select
                    className="sf-form-select"
                    value={createForm.category}
                    onChange={(e) => setCreateForm({ ...createForm, category: e.target.value as any })}
                  >
                    <option value="rekening">Rekening Bank</option>
                    <option value="brankas">Brankas Fisik</option>
                    <option value="coh">Kas Di Tangan (COH)</option>
                  </select>
                </div>
                {createForm.category === 'rekening' && (
                  <>
                    <div className="sf-form-group">
                      <label>Nama Bank *</label>
                      <input
                        type="text"
                        className="sf-form-input"
                        value={createForm.bank_name}
                        onChange={(e) => setCreateForm({ ...createForm, bank_name: e.target.value })}
                        placeholder="Contoh: BCA, Mandiri, BRI"
                        required
                      />
                    </div>
                    <div className="sf-form-group">
                      <label>Nomor Rekening</label>
                      <input
                        type="text"
                        className="sf-form-input"
                        value={createForm.account_number}
                        onChange={(e) => setCreateForm({ ...createForm, account_number: e.target.value })}
                        placeholder="Nomor rekening"
                      />
                    </div>
                    <div className="sf-form-group">
                      <label>Nama Pemilik Rekening</label>
                      <input
                        type="text"
                        className="sf-form-input"
                        value={createForm.account_holder}
                        onChange={(e) => setCreateForm({ ...createForm, account_holder: e.target.value })}
                        placeholder="Atas nama"
                      />
                    </div>
                  </>
                )}
              </div>
              <div className="sf-modal-footer">
                <button type="button" className="sf-btn sf-btn-outline" onClick={() => setShowCreateModal(false)}>Batal</button>
                <button type="submit" className="sf-btn sf-btn-primary" disabled={submitting}>
                  {submitting ? 'Menyimpan...' : 'Simpan'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Edit Lokasi */}
      {editingLocation && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 480 }}>
            <div className="sf-modal-header">
              <h3>Edit Lokasi Dana: {editingLocation.name}</h3>
              <button type="button" className="sf-modal-close" onClick={() => setEditingLocation(null)}>✕</button>
            </div>
            <form onSubmit={handleUpdateLocation}>
              <div className="sf-modal-body">
                {actionError && <div className="sf-error-banner" style={{ marginBottom: 12 }}>{actionError}</div>}
                <div className="sf-form-group">
                  <label>Nama Lokasi *</label>
                  <input
                    type="text"
                    className="sf-form-input"
                    value={editingLocation.name}
                    onChange={(e) => setEditingLocation({ ...editingLocation, name: e.target.value })}
                    required
                  />
                </div>
                {editingLocation.category === 'rekening' && (
                  <>
                    <div className="sf-form-group">
                      <label>Nama Bank</label>
                      <input
                        type="text"
                        className="sf-form-input"
                        value={editingLocation.bank_name || ''}
                        onChange={(e) => setEditingLocation({ ...editingLocation, bank_name: e.target.value })}
                      />
                    </div>
                    <div className="sf-form-group">
                      <label>Nomor Rekening</label>
                      <input
                        type="text"
                        className="sf-form-input"
                        value={editingLocation.account_number || ''}
                        onChange={(e) => setEditingLocation({ ...editingLocation, account_number: e.target.value })}
                      />
                    </div>
                    <div className="sf-form-group">
                      <label>Nama Pemilik Rekening</label>
                      <input
                        type="text"
                        className="sf-form-input"
                        value={editingLocation.account_holder || ''}
                        onChange={(e) => setEditingLocation({ ...editingLocation, account_holder: e.target.value })}
                      />
                    </div>
                  </>
                )}
              </div>
              <div className="sf-modal-footer">
                <button type="button" className="sf-btn sf-btn-outline" onClick={() => setEditingLocation(null)}>Batal</button>
                <button type="submit" className="sf-btn sf-btn-primary" disabled={submitting}>
                  {submitting ? 'Menyimpan...' : 'Simpan Perubahan'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Transfer Antar-Lokasi */}
      {showTransferModal && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 480 }}>
            <div className="sf-modal-header">
              <h3>Transfer Antar-Lokasi RW</h3>
              <button type="button" className="sf-modal-close" onClick={() => setShowTransferModal(false)}>✕</button>
            </div>
            <form onSubmit={handleTransfer}>
              <div className="sf-modal-body">
                {actionError && <div className="sf-error-banner" style={{ marginBottom: 12 }}>{actionError}</div>}
                <div className="sf-form-group">
                  <label>Dari Lokasi (Sumber) *</label>
                  <select
                    className="sf-form-select"
                    value={transferForm.from_location_id}
                    onChange={(e) => setTransferForm({ ...transferForm, from_location_id: e.target.value })}
                    required
                  >
                    <option value="">-- Pilih Lokasi Sumber --</option>
                    {activeLocations.map((l) => (
                      <option key={l.id} value={l.id}>
                        {l.name} ({formatRupiah(parseFloat(l.current_balance) || 0)})
                      </option>
                    ))}
                  </select>
                </div>
                <div className="sf-form-group">
                  <label>Ke Lokasi (Tujuan) *</label>
                  <select
                    className="sf-form-select"
                    value={transferForm.to_location_id}
                    onChange={(e) => setTransferForm({ ...transferForm, to_location_id: e.target.value })}
                    required
                  >
                    <option value="">-- Pilih Lokasi Tujuan --</option>
                    {activeLocations
                      .filter((l) => l.id !== transferForm.from_location_id)
                      .map((l) => (
                        <option key={l.id} value={l.id}>
                          {l.name}
                        </option>
                      ))}
                  </select>
                </div>
                <div className="sf-form-group">
                  <label>Nominal Transfer (Rp) *</label>
                  <input
                    type="number"
                    min="1"
                    step="1"
                    className="sf-form-input"
                    value={transferForm.amount}
                    onChange={(e) => setTransferForm({ ...transferForm, amount: e.target.value })}
                    placeholder="Contoh: 500000"
                    required
                  />
                </div>
                <div className="sf-form-group">
                  <label>Catatan (Opsional)</label>
                  <input
                    type="text"
                    className="sf-form-input"
                    value={transferForm.notes}
                    onChange={(e) => setTransferForm({ ...transferForm, notes: e.target.value })}
                    placeholder="Alasan transfer..."
                  />
                </div>
              </div>
              <div className="sf-modal-footer">
                <button type="button" className="sf-btn sf-btn-outline" onClick={() => setShowTransferModal(false)}>Batal</button>
                <button type="submit" className="sf-btn sf-btn-primary" disabled={submitting}>
                  {submitting ? 'Memproses...' : 'Kirim Transfer'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal: Riwayat Mutasi */}
      {historyLocation && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 650 }}>
            <div className="sf-modal-header">
              <h3>Riwayat Mutasi: {historyLocation.name}</h3>
              <button type="button" className="sf-modal-close" onClick={() => setHistoryLocation(null)}>✕</button>
            </div>
            <div className="sf-modal-body" style={{ maxHeight: '60vh', overflowY: 'auto' }}>
              {loadingHistory ? (
                <div className="sf-loading-banner">Memuat mutasi...</div>
              ) : historyData.length === 0 ? (
                <div className="sf-empty-banner">Belum ada mutasi untuk lokasi ini.</div>
              ) : (
                <table className="sf-warga-table" style={{ width: '100%' }}>
                  <thead>
                    <tr>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>TANGGAL</th>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>TIPE MUTASI</th>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>KETERANGAN</th>
                      <th style={{ padding: '8px 10px', fontSize: 11, textAlign: 'right' }}>DELTA</th>
                    </tr>
                  </thead>
                  <tbody>
                    {historyData.map((m) => {
                      const delta = parseFloat(m.amount_delta) || 0
                      const isPositive = delta > 0
                      return (
                        <tr key={m.id}>
                          <td style={{ padding: '8px 10px', fontSize: 12, color: 'var(--sf-text-muted)' }}>
                            {new Date(m.created_at).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })}
                          </td>
                          <td style={{ padding: '8px 10px', fontSize: 12 }}>
                            <Badge variant={isPositive ? 'green' : 'red'}>{m.change_type}</Badge>
                          </td>
                          <td style={{ padding: '8px 10px', fontSize: 12 }}>
                            {m.notes || '—'}
                          </td>
                          <td style={{
                            padding: '8px 10px',
                            fontSize: 12,
                            textAlign: 'right',
                            fontFamily: 'monospace',
                            fontWeight: 600,
                            color: isPositive ? 'var(--sf-success)' : 'var(--sf-danger)',
                          }}>
                            {isPositive ? '+' : ''}{formatRupiah(delta)}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </div>
            <div className="sf-modal-footer">
              <button type="button" className="sf-btn sf-btn-outline" onClick={() => setHistoryLocation(null)}>Tutup</button>
            </div>
          </div>
        </div>
      )}

      {/* Modal: Rekonsiliasi */}
      {showReconcileModal && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 500 }}>
            <div className="sf-modal-header">
              <h3>Rekonsiliasi Kas & Lokasi Dana RW</h3>
              <button type="button" className="sf-modal-close" onClick={() => setShowReconcileModal(false)}>✕</button>
            </div>
            <div className="sf-modal-body">
              {loadingReconcile ? (
                <div className="sf-loading-banner">Memuat rekonsiliasi...</div>
              ) : reconcileData ? (
                <div>
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    padding: '12px 16px',
                    borderRadius: 'var(--sf-radius-md)',
                    background: reconcileData.is_reconciled ? 'rgba(52, 211, 153, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                    border: `1px solid ${reconcileData.is_reconciled ? 'rgba(52, 211, 153, 0.3)' : 'rgba(239, 68, 68, 0.3)'}`,
                    marginBottom: 16,
                  }}>
                    {reconcileData.is_reconciled ? (
                      <CheckCircleOutlined style={{ color: '#34d399', fontSize: 20 }} />
                    ) : (
                      <WarningOutlined style={{ color: '#ef4444', fontSize: 20 }} />
                    )}
                    <div>
                      <div style={{ fontWeight: 700, fontSize: 13, color: reconcileData.is_reconciled ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                        {reconcileData.is_reconciled ? 'TEREKONSILIASI' : 'SELISIH DITEMUKAN'}
                      </div>
                      <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
                        {reconcileData.is_reconciled
                          ? 'Total seluruh lokasi dana RW sama persis dengan saldo ledger KAS RW.'
                          : `Terdapat selisih ${formatRupiah(parseFloat(reconcileData.discrepancy) || 0)}.`}
                      </div>
                    </div>
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--sf-border)' }}>
                      <span style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Saldo Ledger KAS RW:</span>
                      <strong style={{ fontSize: 13, fontFamily: 'monospace' }}>
                        {formatRupiah(parseFloat(reconcileData.calculated_balance) || 0)}
                      </strong>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--sf-border)' }}>
                      <span style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Total Saldo Lokasi Dana RW:</span>
                      <strong style={{ fontSize: 13, fontFamily: 'monospace' }}>
                        {formatRupiah(parseFloat(reconcileData.total_location_balances) || 0)}
                      </strong>
                    </div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', padding: '6px 0', borderBottom: '1px solid var(--sf-border)' }}>
                      <span style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>Selisih:</span>
                      <strong style={{
                        fontSize: 13,
                        fontFamily: 'monospace',
                        color: reconcileData.is_reconciled ? 'var(--sf-success)' : 'var(--sf-danger)',
                      }}>
                        {formatRupiah(parseFloat(reconcileData.discrepancy) || 0)}
                      </strong>
                    </div>
                  </div>
                </div>
              ) : (
                <div>Gagal memuat data rekonsiliasi.</div>
              )}
            </div>
            <div className="sf-modal-footer">
              <button type="button" className="sf-btn sf-btn-outline" onClick={() => setShowReconcileModal(false)}>Tutup</button>
            </div>
          </div>
        </div>
      )}

      {/* Modal: Riwayat Transfer */}
      {showTransfersModal && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 850 }}>
            <div className="sf-modal-header">
              <h3>Riwayat Transfer Antar-Lokasi RW</h3>
              <button type="button" className="sf-modal-close" onClick={() => setShowTransfersModal(false)}>✕</button>
            </div>
            <div className="sf-modal-body" style={{ maxHeight: '65vh', overflowY: 'auto' }}>
              {transfersError && (
                <div className="sf-error-banner" role="alert" style={{ marginBottom: 12 }}>{transfersError}</div>
              )}
              {loadingTransfers ? (
                <div className="sf-loading-banner">Memuat riwayat transfer...</div>
              ) : transfers.length === 0 ? (
                <div className="sf-empty-banner">Belum ada riwayat transfer dana antar-lokasi.</div>
              ) : (
                <table className="sf-warga-table" style={{ width: '100%' }}>
                  <thead>
                    <tr>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>TANGGAL</th>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>DARI LOKASI</th>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>KE LOKASI</th>
                      <th style={{ padding: '8px 10px', fontSize: 11, textAlign: 'right' }}>NOMINAL</th>
                      <th style={{ padding: '8px 10px', fontSize: 11 }}>CATATAN</th>
                      <th style={{ padding: '8px 10px', fontSize: 11, textAlign: 'center' }}>STATUS</th>
                      {canUpdate && <th style={{ padding: '8px 10px', fontSize: 11, textAlign: 'center' }}>AKSI</th>}
                    </tr>
                  </thead>
                  <tbody>
                    {transfers.map((t) => {
                      const amountNum = parseFloat(t.amount) || 0
                      const isRev = t.is_reversal
                      const isReversed = !!t.reverses_transfer_id
                      return (
                        <tr key={t.id}>
                          <td style={{ padding: '8px 10px', fontSize: 12, color: 'var(--sf-text-muted)', whiteSpace: 'nowrap' }}>
                            {new Date(t.transfer_date || t.created_at).toLocaleDateString('id-ID', {
                              day: 'numeric',
                              month: 'short',
                              year: 'numeric',
                            })}
                          </td>
                          <td style={{ padding: '8px 10px', fontSize: 12 }}>
                            {t.from_location_name || t.from_location_id}
                          </td>
                          <td style={{ padding: '8px 10px', fontSize: 12 }}>
                            {t.to_location_name || t.to_location_id}
                          </td>
                          <td style={{
                            padding: '8px 10px',
                            fontSize: 12,
                            textAlign: 'right',
                            fontFamily: 'monospace',
                            fontWeight: 600,
                            whiteSpace: 'nowrap',
                          }}>
                            {formatRupiah(amountNum)}
                          </td>
                          <td style={{ padding: '8px 10px', fontSize: 12 }}>
                            {t.notes || '—'}
                          </td>
                          <td style={{ padding: '8px 10px', fontSize: 12, textAlign: 'center', whiteSpace: 'nowrap' }}>
                            {isRev ? (
                              <Badge variant="blue">Reversal</Badge>
                            ) : isReversed ? (
                              <Badge variant="red">Dibatalkan</Badge>
                            ) : (
                              <Badge variant="green">Selesai</Badge>
                            )}
                          </td>
                          {canUpdate && (
                            <td style={{ padding: '8px 10px', fontSize: 12, textAlign: 'center', whiteSpace: 'nowrap' }}>
                              {!isRev && !isReversed && (
                                <button
                                  type="button"
                                  className="sf-btn sf-btn-outline sf-btn-sm"
                                  onClick={() => handleOpenReverse(t)}
                                  style={{
                                    fontSize: 11,
                                    padding: '2px 8px',
                                    color: 'var(--sf-danger)',
                                    borderColor: 'rgba(239, 68, 68, 0.4)',
                                  }}
                                  title="Batalkan transfer ini"
                                >
                                  Batalkan (Reversal)
                                </button>
                              )}
                            </td>
                          )}
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </div>
            <div className="sf-modal-footer">
              <button type="button" className="sf-btn sf-btn-outline" onClick={() => setShowTransfersModal(false)}>Tutup</button>
            </div>
          </div>
        </div>
      )}

      {/* Modal: Konfirmasi Reversal Transfer */}
      {reversingTransfer && (
        <div className="sf-modal-overlay">
          <div className="sf-modal" style={{ maxWidth: 450 }}>
            <div className="sf-modal-header">
              <h3 style={{ color: 'var(--sf-danger)' }}>Batalkan Transfer (Reversal)</h3>
              <button type="button" className="sf-modal-close" onClick={() => setReversingTransfer(null)}>✕</button>
            </div>
            <form onSubmit={handleConfirmReverse}>
              <div className="sf-modal-body">
                {reversalError && (
                  <div className="sf-error-banner" role="alert" style={{ marginBottom: 12 }}>{reversalError}</div>
                )}
                <div style={{
                  padding: '12px 14px',
                  borderRadius: 'var(--sf-radius-sm)',
                  background: 'rgba(239, 68, 68, 0.06)',
                  border: '1px solid rgba(239, 68, 68, 0.2)',
                  fontSize: 13,
                  marginBottom: 14,
                  lineHeight: 1.5,
                }}>
                  Apakah Anda yakin ingin membatalkan transfer sebesar{' '}
                  <strong>{formatRupiah(parseFloat(reversingTransfer.amount) || 0)}</strong> dari{' '}
                  <strong>{reversingTransfer.from_location_name || reversingTransfer.from_location_id}</strong> ke{' '}
                  <strong>{reversingTransfer.to_location_name || reversingTransfer.to_location_id}</strong>?
                  <div style={{ marginTop: 6, fontSize: 12, color: 'var(--sf-text-muted)' }}>
                    Operasi ini akan membuat transaksi pembalik yang mengembalikan saldo ke lokasi semula.
                  </div>
                </div>

                <div className="sf-form-group">
                  <label>Alasan Pembatalan *</label>
                  <textarea
                    className="sf-form-input"
                    rows={3}
                    value={reversalReason}
                    onChange={(e) => setReversalReason(e.target.value)}
                    placeholder="Contoh: Salah pilih lokasi tujuan / salah nominal"
                    required
                  />
                </div>
              </div>
              <div className="sf-modal-footer">
                <button
                  type="button"
                  className="sf-btn sf-btn-outline"
                  onClick={() => setReversingTransfer(null)}
                  disabled={submittingReversal}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  className="sf-btn"
                  style={{ background: 'var(--sf-danger)', color: '#fff', borderColor: 'var(--sf-danger)' }}
                  disabled={submittingReversal}
                >
                  {submittingReversal ? 'Memproses Reversal...' : 'Konfirmasi Reversal'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
