import { useState, useEffect, useCallback, useMemo } from 'react'
import {
  BankOutlined,
  WalletOutlined,
  SafetyCertificateOutlined,
  SwapOutlined,
  PlusOutlined,
  HistoryOutlined,
  EditOutlined,
  StopOutlined,
  RollbackOutlined,
  CheckCircleOutlined,
  InfoCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { Modal, ModalHeader, ModalBody, ModalFooter } from '@/components/Modal'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { useAuth } from '@/app/AuthContext'
import {
  getSessionPair,
  apiListFundLocations,
  apiCreateFundLocation,
  apiUpdateFundLocation,
  apiDeactivateFundLocation,
  apiGetFundLocationHistory,
  apiSetOpeningBalance,
  apiListFundTransfers,
  apiCreateFundTransfer,
  apiReverseFundTransfer,
  apiGetFundLocationReconciliation,
  type FundLocation,
  type FundLocationCategory,
  type FundTransfer,
  type LocationBalanceChange,
  type ReconciliationReport,
} from '@/app/api'
import { formatRibuan, parseMoney } from '../iuran/iuranTypes'

export function FundLocationList() {
  const { user } = useAuth()
  const isBendahara = user?.role === 'bendahara' || user?.jabatan === 'bendahara'
  const isPengurusOrBendahara = user?.role === 'bendahara' || user?.role === 'pengurus' || user?.systemRole === 'super_admin'

  const [activeTab, setActiveTab] = useState<'locations' | 'transfers'>('locations')
  const [locations, setLocations] = useState<FundLocation[]>([])
  const [transfers, setTransfers] = useState<FundTransfer[]>([])
  const [reconciliation, setReconciliation] = useState<ReconciliationReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Filters
  const [search, setSearch] = useState('')
  const [categoryFilter, setCategoryFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  // Modals state
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showTransferModal, setShowTransferModal] = useState(false)
  const [showOpeningBalanceModal, setShowOpeningBalanceModal] = useState(false)
  const [editingLocation, setEditingLocation] = useState<FundLocation | null>(null)
  const [historyLocation, setHistoryLocation] = useState<FundLocation | null>(null)
  const [historyData, setHistoryData] = useState<LocationBalanceChange[]>([])
  const [loadingHistory, setLoadingHistory] = useState(false)
  const [reversingTransfer, setReversingTransfer] = useState<FundTransfer | null>(null)
  const [reversalReason, setReversalReason] = useState('')

  // Form states
  const [createForm, setCreateForm] = useState<{
    name: string
    category: FundLocationCategory
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

  const [openingBalanceForm, setOpeningBalanceForm] = useState<{
    location_id: string
    amount: string
    notes: string
  }>({
    location_id: '',
    amount: '',
    notes: '',
  })

  const [formError, setFormError] = useState<string | null>(null)
  const [formSubmitting, setFormSubmitting] = useState(false)

  const loadData = useCallback(async () => {
    const token = getSessionPair()?.accessToken
    if (!token) return
    setLoading(true)
    setError(null)
    try {
      const [locs, rec, trans] = await Promise.all([
        apiListFundLocations(token),
        apiGetFundLocationReconciliation(token).catch(() => null),
        apiListFundTransfers(token, { page: 1, page_size: 50 }).catch(() => ({ data: [], pagination: {} })),
      ])
      setLocations(locs)
      setReconciliation(rec)
      setTransfers(trans.data || [])
    } catch (err: any) {
      setError(err?.message || 'Gagal memuat data lokasi dana')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Category counts & balances
  const categorySummary = useMemo(() => {
    let rekeningTotal = 0
    let rekeningCount = 0
    let brankasTotal = 0
    let brankasCount = 0
    let cohTotal = 0
    let cohCount = 0

    locations.forEach((loc) => {
      const bal = parseMoney(loc.current_balance) || 0
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
        const matchesBank = loc.bank_name?.toLowerCase().includes(q) ?? false
        const matchesAcc = loc.account_number?.toLowerCase().includes(q) ?? false
        const matchesHolder = loc.account_holder?.toLowerCase().includes(q) ?? false
        if (!matchesName && !matchesBank && !matchesAcc && !matchesHolder) return false
      }
      return true
    })
  }, [locations, categoryFilter, statusFilter, search])

  // Handle Create Location
  const handleCreateLocation = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!createForm.name.trim()) {
      setFormError('Nama lokasi wajib diisi')
      return
    }
    if (createForm.category === 'rekening' && !createForm.bank_name.trim()) {
      setFormError('Nama bank wajib diisi untuk kategori Rekening Bank')
      return
    }

    setFormSubmitting(true)
    try {
      await apiCreateFundLocation(token, {
        name: createForm.name.trim(),
        category: createForm.category,
        bank_name: createForm.bank_name.trim() || undefined,
        account_number: createForm.account_number.trim() || undefined,
        account_holder: createForm.account_holder.trim() || undefined,
      })
      setShowCreateModal(false)
      setCreateForm({ name: '', category: 'rekening', bank_name: '', account_number: '', account_holder: '' })
      await loadData()
    } catch (err: any) {
      setFormError(err?.message || 'Gagal membuat lokasi dana')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Handle Edit Location
  const handleUpdateLocation = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingLocation) return
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    setFormSubmitting(true)
    try {
      await apiUpdateFundLocation(token, editingLocation.id, {
        name: editingLocation.name.trim() || undefined,
        bank_name: editingLocation.bank_name?.trim() || undefined,
        account_number: editingLocation.account_number?.trim() || undefined,
        account_holder: editingLocation.account_holder?.trim() || undefined,
      })
      setEditingLocation(null)
      await loadData()
    } catch (err: any) {
      setFormError(err?.message || 'Gagal mengubah lokasi dana')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Handle Deactivate Location
  const handleDeactivate = async (loc: FundLocation) => {
    const bal = parseMoney(loc.current_balance) || 0
    if (bal > 0) {
      alert(`Lokasi "${loc.name}" masih memiliki saldo Rp ${formatRibuan(bal)}. Saldo harus dialokasikan/ditransfer ke Rp 0 sebelum dinonaktifkan.`)
      return
    }
    if (!confirm(`Apakah Anda yakin ingin menonaktifkan lokasi "${loc.name}"?`)) return

    const token = getSessionPair()?.accessToken
    if (!token) return
    try {
      await apiDeactivateFundLocation(token, loc.id)
      await loadData()
    } catch (err: any) {
      alert(err?.message || 'Gagal menonaktifkan lokasi')
    }
  }

  // Handle View History
  const handleViewHistory = async (loc: FundLocation) => {
    setHistoryLocation(loc)
    setLoadingHistory(true)
    const token = getSessionPair()?.accessToken
    if (!token) return
    try {
      const res = await apiGetFundLocationHistory(token, loc.id, { page: 1, page_size: 50 })
      setHistoryData(res.data || [])
    } catch (err: any) {
      setHistoryData([])
    } finally {
      setLoadingHistory(false)
    }
  }

  // Handle Create Transfer
  const handleCreateTransfer = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!transferForm.from_location_id || !transferForm.to_location_id) {
      setFormError('Lokasi asal dan tujuan wajib dipilih')
      return
    }
    if (transferForm.from_location_id === transferForm.to_location_id) {
      setFormError('Lokasi asal dan tujuan tidak boleh sama')
      return
    }
    const amt = parseMoney(transferForm.amount) || 0
    if (amt <= 0) {
      setFormError('Nominal transfer harus lebih besar dari 0')
      return
    }

    const sourceLoc = locations.find((l) => l.id === transferForm.from_location_id)
    const sourceBal = parseMoney(sourceLoc?.current_balance || '0') || 0
    if (sourceBal < amt) {
      setFormError(`Saldo pada ${sourceLoc?.name} tidak mencukupi (Saldo: Rp ${formatRibuan(sourceBal)})`)
      return
    }

    setFormSubmitting(true)
    try {
      await apiCreateFundTransfer(token, {
        from_location_id: transferForm.from_location_id,
        to_location_id: transferForm.to_location_id,
        amount: String(amt),
        notes: transferForm.notes.trim() || undefined,
      })
      setShowTransferModal(false)
      setTransferForm({ from_location_id: '', to_location_id: '', amount: '', notes: '' })
      await loadData()
    } catch (err: any) {
      setFormError(err?.message || 'Gagal melakukan pemindahan dana')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Handle Reverse Transfer
  const handleReverseTransfer = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!reversingTransfer) return
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!reversalReason.trim()) {
      setFormError('Alasan pembatalan (reversal) wajib diisi')
      return
    }

    setFormSubmitting(true)
    try {
      await apiReverseFundTransfer(token, reversingTransfer.id, {
        reason: reversalReason.trim(),
      })
      setReversingTransfer(null)
      setReversalReason('')
      await loadData()
    } catch (err: any) {
      setFormError(err?.message || 'Gagal membatalkan transfer dana')
    } finally {
      setFormSubmitting(false)
    }
  }

  // Handle Set Opening Balance
  const handleSetOpeningBalance = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    const token = getSessionPair()?.accessToken
    if (!token) return

    if (!openingBalanceForm.location_id) {
      setFormError('Pilih lokasi tujuan alokasi')
      return
    }
    const amt = parseMoney(openingBalanceForm.amount) || 0
    if (amt <= 0) {
      setFormError('Nominal alokasi saldo awal harus lebih besar dari 0')
      return
    }

    const unallocated = parseMoney(reconciliation?.unallocated_balance || '0') || 0
    if (amt > unallocated) {
      setFormError(`Nominal melebihi saldo kas yang belum teralokasi (Maksimal: Rp ${formatRibuan(unallocated)})`)
      return
    }

    setFormSubmitting(true)
    try {
      await apiSetOpeningBalance(token, openingBalanceForm.location_id, {
        amount: String(amt),
        notes: openingBalanceForm.notes.trim() || undefined,
      })
      setShowOpeningBalanceModal(false)
      setOpeningBalanceForm({ location_id: '', amount: '', notes: '' })
      await loadData()
    } catch (err: any) {
      setFormError(err?.message || 'Gagal mengalokasikan saldo awal')
    } finally {
      setFormSubmitting(false)
    }
  }

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px 16px' }}>
      {/* Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24, flexWrap: 'wrap', gap: 16 }}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 700, margin: 0, color: 'var(--sf-text)' }}>Lokasi Dana</h1>
          <p style={{ margin: '4px 0 0', color: 'var(--sf-text-muted)', fontSize: 14 }}>
            Manajemen Posisi Kas RT (Rekening Bank, Brankas, dan Cash on Hand / COH)
          </p>
        </div>
        {isPengurusOrBendahara && (
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button
              type="button"
              className="sf-btn"
              onClick={() => { setFormError(null); setShowCreateModal(true); }}
              style={{
                background: 'var(--sf-primary)',
                color: '#fff',
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                padding: '8px 14px',
                borderRadius: 6,
                fontWeight: 600,
                fontSize: 13,
                cursor: 'pointer',
              }}
            >
              <PlusOutlined /> Tambah Lokasi
            </button>
            {isBendahara && (
              <button
                type="button"
                className="sf-btn"
                onClick={() => { setFormError(null); setShowTransferModal(true); }}
                style={{
                  background: 'var(--sf-surface)',
                  color: 'var(--sf-text)',
                  border: '1px solid var(--sf-border)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '8px 14px',
                  borderRadius: 6,
                  fontWeight: 600,
                  fontSize: 13,
                  cursor: 'pointer',
                }}
              >
                <SwapOutlined /> Transfer Dana
              </button>
            )}
            <button
              type="button"
              className="sf-btn"
              onClick={() => { setFormError(null); setShowOpeningBalanceModal(true); }}
              style={{
                background: 'var(--sf-surface)',
                color: 'var(--sf-text)',
                border: '1px solid var(--sf-border)',
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                padding: '8px 14px',
                borderRadius: 6,
                fontWeight: 600,
                fontSize: 13,
                cursor: 'pointer',
              }}
            >
              <HistoryOutlined /> Alokasi Saldo Awal
            </button>
          </div>
        )}
      </div>

      {error && (
        <div style={{ padding: 12, background: 'rgba(242, 73, 92, 0.1)', border: '1px solid var(--sf-danger)', borderRadius: 6, color: 'var(--sf-danger)', marginBottom: 16 }}>
          {error}
        </div>
      )}

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 16, marginBottom: 24 }}>
        {/* Rekening Card */}
        <div style={{ background: 'var(--sf-surface)', border: '1px solid var(--sf-border)', borderRadius: 8, padding: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{ fontSize: 13, color: 'var(--sf-text-muted)', fontWeight: 600 }}>REKENING BANK</span>
            <BankOutlined style={{ fontSize: 18, color: 'var(--sf-primary)' }} />
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-text)', marginBottom: 4 }}>
            Rp {formatRibuan(categorySummary.rekening.total)}
          </div>
          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
            {categorySummary.rekening.count} rekening bank aktif
          </div>
        </div>

        {/* Brankas Card */}
        <div style={{ background: 'var(--sf-surface)', border: '1px solid var(--sf-border)', borderRadius: 8, padding: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{ fontSize: 13, color: 'var(--sf-text-muted)', fontWeight: 600 }}>BRANKAS KAS</span>
            <SafetyCertificateOutlined style={{ fontSize: 18, color: 'var(--sf-warning)' }} />
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-text)', marginBottom: 4 }}>
            Rp {formatRibuan(categorySummary.brankas.total)}
          </div>
          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
            {categorySummary.brankas.count} brankas fisik aktif
          </div>
        </div>

        {/* COH Card */}
        <div style={{ background: 'var(--sf-surface)', border: '1px solid var(--sf-border)', borderRadius: 8, padding: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{ fontSize: 13, color: 'var(--sf-text-muted)', fontWeight: 600 }}>CASH ON HAND (COH)</span>
            <WalletOutlined style={{ fontSize: 18, color: 'var(--sf-success)' }} />
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-text)', marginBottom: 4 }}>
            Rp {formatRibuan(categorySummary.coh.total)}
          </div>
          <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
            {categorySummary.coh.count} kas tunai dipegang pengurus
          </div>
        </div>
      </div>

      {/* Reconciliation Card */}
      {reconciliation && (
        <div style={{
          background: 'var(--sf-surface)',
          border: '1px solid var(--sf-border)',
          borderRadius: 8,
          padding: 16,
          marginBottom: 24,
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8, marginBottom: 12 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontWeight: 600, fontSize: 14 }}>Status Rekonsiliasi Kas:</span>
              {reconciliation.status === 'MATCHED' && (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, background: 'rgba(115, 191, 105, 0.15)', color: 'var(--sf-success)', padding: '2px 8px', borderRadius: 12, fontSize: 12, fontWeight: 600 }}>
                  <CheckCircleOutlined /> Cocok Sempurna (100% Teralokasi)
                </span>
              )}
              {reconciliation.status === 'MATCH_WITH_NOTES' && (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, background: 'rgba(56, 132, 255, 0.15)', color: '#3884ff', padding: '2px 8px', borderRadius: 12, fontSize: 12, fontWeight: 600 }}>
                  <InfoCircleOutlined /> Cocok dengan Catatan (Ada Kas Historis Belum Teralokasi)
                </span>
              )}
              {reconciliation.status === 'DISCREPANCY' && (
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, background: 'rgba(242, 73, 92, 0.15)', color: 'var(--sf-danger)', padding: '2px 8px', borderRadius: 12, fontSize: 12, fontWeight: 600 }}>
                  <WarningOutlined /> Selisih Ditemukan
                </span>
              )}
            </div>
            <div style={{ fontSize: 12, color: 'var(--sf-text-muted)' }}>
              Invariant: K = L + U ({formatRibuan(reconciliation.kas_balance)} = {formatRibuan(reconciliation.total_allocated_balance)} + {formatRibuan(reconciliation.unallocated_balance)})
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 12, background: 'rgba(0,0,0,0.03)', padding: 12, borderRadius: 6 }}>
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase' }}>Total Saldo Kas (K)</div>
              <div style={{ fontSize: 15, fontWeight: 600 }}>Rp {formatRibuan(reconciliation.kas_balance)}</div>
            </div>
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase' }}>Total Terdistribusi (L)</div>
              <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--sf-primary)' }}>Rp {formatRibuan(reconciliation.total_allocated_balance)}</div>
            </div>
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase' }}>Belum Teralokasi (U)</div>
              <div style={{ fontSize: 15, fontWeight: 600, color: parseMoney(reconciliation.unallocated_balance) > 0 ? '#e6a23c' : 'var(--sf-text)' }}>
                Rp {formatRibuan(reconciliation.unallocated_balance)}
              </div>
            </div>
            <div>
              <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase' }}>Selisih / Discrepancy (D)</div>
              <div style={{ fontSize: 15, fontWeight: 600, color: parseMoney(reconciliation.discrepancy) === 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                Rp {formatRibuan(reconciliation.discrepancy)}
              </div>
            </div>
          </div>

          {reconciliation.anomalies_detected && reconciliation.anomalies_detected.length > 0 && (
            <div style={{ marginTop: 12, padding: 8, background: 'rgba(242, 73, 92, 0.1)', border: '1px solid var(--sf-danger)', borderRadius: 4, color: 'var(--sf-danger)', fontSize: 12 }}>
              <strong>Anomali Terdeteksi:</strong>
              <ul style={{ margin: '4px 0 0 16px', padding: 0 }}>
                {reconciliation.anomalies_detected.map((a, i) => (
                  <li key={i}>{a}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {/* Tabs */}
      <div style={{ display: 'flex', borderBottom: '1px solid var(--sf-border)', marginBottom: 16 }}>
        <button
          type="button"
          onClick={() => setActiveTab('locations')}
          style={{
            padding: '10px 16px',
            border: 'none',
            background: 'none',
            fontWeight: 600,
            fontSize: 14,
            cursor: 'pointer',
            borderBottom: activeTab === 'locations' ? '2px solid var(--sf-primary)' : '2px solid transparent',
            color: activeTab === 'locations' ? 'var(--sf-primary)' : 'var(--sf-text-muted)',
          }}
        >
          Daftar Lokasi Dana ({locations.length})
        </button>
        <button
          type="button"
          onClick={() => setActiveTab('transfers')}
          style={{
            padding: '10px 16px',
            border: 'none',
            background: 'none',
            fontWeight: 600,
            fontSize: 14,
            cursor: 'pointer',
            borderBottom: activeTab === 'transfers' ? '2px solid var(--sf-primary)' : '2px solid transparent',
            color: activeTab === 'transfers' ? 'var(--sf-primary)' : 'var(--sf-text-muted)',
          }}
        >
          Riwayat Transfer Dana ({transfers.length})
        </button>
      </div>

      {activeTab === 'locations' ? (
        <>
          {/* Filters & Search */}
          <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 16, alignItems: 'center' }}>
            <div style={{ flex: 1, minWidth: 200 }}>
              <SearchBox value={search} onChange={setSearch} placeholder="Cari nama, bank, atau rekening..." />
            </div>
            <select
              value={categoryFilter}
              onChange={(e) => setCategoryFilter(e.target.value)}
              style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)', fontSize: 13 }}
            >
              <option value="all">Semua Kategori</option>
              <option value="rekening">Rekening Bank</option>
              <option value="brankas">Brankas</option>
              <option value="coh">Cash on Hand (COH)</option>
            </select>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              style={{ padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)', fontSize: 13 }}
            >
              <option value="all">Semua Status</option>
              <option value="active">Aktif</option>
              <option value="inactive">Nonaktif</option>
            </select>
          </div>

          {/* Table */}
          <div style={{ background: 'var(--sf-surface)', border: '1px solid var(--sf-border)', borderRadius: 8, overflow: 'hidden' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
              <thead>
                <tr style={{ background: 'rgba(0,0,0,0.02)', borderBottom: '1px solid var(--sf-border)' }}>
                  <th style={{ padding: '12px 16px' }}>Nama Lokasi</th>
                  <th style={{ padding: '12px 16px' }}>Kategori</th>
                  <th style={{ padding: '12px 16px' }}>Detail Rekening / Kasir</th>
                  <th style={{ padding: '12px 16px', textAlign: 'right' }}>Saldo Saat Ini</th>
                  <th style={{ padding: '12px 16px', textAlign: 'center' }}>Status</th>
                  <th style={{ padding: '12px 16px', textAlign: 'right' }}>Aksi</th>
                </tr>
              </thead>
              <tbody>
                {loading ? (
                  <tr>
                    <td colSpan={6} style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>Memuat data...</td>
                  </tr>
                ) : filteredLocations.length === 0 ? (
                  <tr>
                    <td colSpan={6} style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>Belum ada lokasi dana yang terdaftar.</td>
                  </tr>
                ) : (
                  filteredLocations.map((loc) => {
                    const balNum = parseMoney(loc.current_balance) || 0
                    return (
                      <tr key={loc.id} style={{ borderBottom: '1px solid var(--sf-border)' }}>
                        <td style={{ padding: '12px 16px', fontWeight: 600 }}>{loc.name}</td>
                        <td style={{ padding: '12px 16px' }}>
                          {loc.category === 'rekening' && <Badge variant="blue">Rekening Bank</Badge>}
                          {loc.category === 'brankas' && <Badge variant="amber">Brankas</Badge>}
                          {loc.category === 'coh' && <Badge variant="green">Cash On Hand</Badge>}
                        </td>
                        <td style={{ padding: '12px 16px', color: 'var(--sf-text-muted)' }}>
                          {loc.category === 'rekening' ? (
                            <>
                              <div>{loc.bank_name || '-'} {loc.account_number ? `(${loc.account_number})` : ''}</div>
                              {loc.account_holder && <div style={{ fontSize: 11 }}>a.n. {loc.account_holder}</div>}
                            </>
                          ) : (
                            <div>{loc.account_holder ? `Pemegang: ${loc.account_holder}` : '-'}</div>
                          )}
                        </td>
                        <td style={{ padding: '12px 16px', textAlign: 'right', fontWeight: 700, fontSize: 14 }}>
                          Rp {formatRibuan(balNum)}
                        </td>
                        <td style={{ padding: '12px 16px', textAlign: 'center' }}>
                          {loc.is_active ? (
                            <span style={{ color: 'var(--sf-success)', fontSize: 12, fontWeight: 600 }}>Aktif</span>
                          ) : (
                            <span style={{ color: 'var(--sf-danger)', fontSize: 12, fontWeight: 600 }}>Nonaktif</span>
                          )}
                        </td>
                        <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                          <div style={{ display: 'inline-flex', gap: 6 }}>
                            <button
                              type="button"
                              className="sf-btn-ghost"
                              onClick={() => handleViewHistory(loc)}
                              title="Riwayat Mutasi"
                              style={{ padding: '4px 8px', fontSize: 12 }}
                            >
                              <HistoryOutlined />
                            </button>
                            {isPengurusOrBendahara && (
                              <>
                                <button
                                  type="button"
                                  className="sf-btn-ghost"
                                  onClick={() => { setFormError(null); setEditingLocation(loc); }}
                                  title="Edit"
                                  style={{ padding: '4px 8px', fontSize: 12 }}
                                >
                                  <EditOutlined />
                                </button>
                                {loc.is_active && (
                                  <button
                                    type="button"
                                    className="sf-btn-ghost"
                                    onClick={() => handleDeactivate(loc)}
                                    title="Nonaktifkan"
                                    style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-danger)' }}
                                  >
                                    <StopOutlined />
                                  </button>
                                )}
                              </>
                            )}
                          </div>
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </>
      ) : (
        /* Transfers Tab */
        <div style={{ background: 'var(--sf-surface)', border: '1px solid var(--sf-border)', borderRadius: 8, overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: 13 }}>
            <thead>
              <tr style={{ background: 'rgba(0,0,0,0.02)', borderBottom: '1px solid var(--sf-border)' }}>
                <th style={{ padding: '12px 16px' }}>Tanggal</th>
                <th style={{ padding: '12px 16px' }}>Dari Lokasi</th>
                <th style={{ padding: '12px 16px' }}>Ke Lokasi</th>
                <th style={{ padding: '12px 16px', textAlign: 'right' }}>Nominal</th>
                <th style={{ padding: '12px 16px' }}>Catatan</th>
                <th style={{ padding: '12px 16px', textAlign: 'center' }}>Status</th>
                <th style={{ padding: '12px 16px', textAlign: 'right' }}>Aksi</th>
              </tr>
            </thead>
            <tbody>
              {transfers.length === 0 ? (
                <tr>
                  <td colSpan={7} style={{ padding: 32, textAlign: 'center', color: 'var(--sf-text-muted)' }}>Belum ada riwayat transfer dana.</td>
                </tr>
              ) : (
                transfers.map((t) => {
                  const amtNum = parseMoney(t.amount) || 0
                  const isReversal = !!t.reverses_transfer_id
                  return (
                    <tr key={t.id} style={{ borderBottom: '1px solid var(--sf-border)' }}>
                      <td style={{ padding: '12px 16px', color: 'var(--sf-text-muted)' }}>
                        {t.transfer_date ? t.transfer_date.slice(0, 10) : '-'}
                      </td>
                      <td style={{ padding: '12px 16px', fontWeight: 600 }}>{t.from_location_name || t.from_location_id}</td>
                      <td style={{ padding: '12px 16px', fontWeight: 600 }}>{t.to_location_name || t.to_location_id}</td>
                      <td style={{ padding: '12px 16px', textAlign: 'right', fontWeight: 700 }}>
                        Rp {formatRibuan(amtNum)}
                      </td>
                      <td style={{ padding: '12px 16px', color: 'var(--sf-text-muted)' }}>{t.notes || '-'}</td>
                      <td style={{ padding: '12px 16px', textAlign: 'center' }}>
                        {isReversal ? (
                          <span style={{ color: 'var(--sf-warning)', fontSize: 12, fontWeight: 600 }}>Reversal</span>
                        ) : (
                          <span style={{ color: 'var(--sf-success)', fontSize: 12, fontWeight: 600 }}>Selesai</span>
                        )}
                      </td>
                      <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                        {isBendahara && !isReversal && (
                          <button
                            type="button"
                            className="sf-btn-ghost"
                            onClick={() => { setFormError(null); setReversingTransfer(t); }}
                            style={{ padding: '4px 8px', fontSize: 12, color: 'var(--sf-danger)' }}
                            title="Batalkan Transfer (Reversal)"
                          >
                            <RollbackOutlined /> Batalkan
                          </button>
                        )}
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Location Modal */}
      {showCreateModal && (
        <Modal open onClose={() => setShowCreateModal(false)} width={480}>
          <ModalHeader title="Tambah Lokasi Dana" onClose={() => setShowCreateModal(false)} />
          <form onSubmit={handleCreateLocation}>
            <ModalBody>
              {formError && (
                <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                  {formError}
                </div>
              )}
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Kategori Lokasi *</label>
                <select
                  value={createForm.category}
                  onChange={(e) => setCreateForm({ ...createForm, category: e.target.value as FundLocationCategory })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                >
                  <option value="rekening">Rekening Bank</option>
                  <option value="brankas">Brankas Fisik</option>
                  <option value="coh">Cash on Hand (COH)</option>
                </select>
              </div>
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nama Lokasi *</label>
                <input
                  type="text"
                  placeholder="Contoh: Rekening BCA RT, Brankas Kantor, COH Bendahara"
                  value={createForm.name}
                  onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
              {createForm.category === 'rekening' && (
                <>
                  <div style={{ marginBottom: 12 }}>
                    <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nama Bank *</label>
                    <input
                      type="text"
                      placeholder="Contoh: BCA, Mandiri, BRI, BNI"
                      value={createForm.bank_name}
                      onChange={(e) => setCreateForm({ ...createForm, bank_name: e.target.value })}
                      style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                    />
                  </div>
                  <div style={{ marginBottom: 12 }}>
                    <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nomor Rekening</label>
                    <input
                      type="text"
                      placeholder="Nomor rekening bank"
                      value={createForm.account_number}
                      onChange={(e) => setCreateForm({ ...createForm, account_number: e.target.value })}
                      style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                    />
                  </div>
                </>
              )}
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>
                  {createForm.category === 'rekening' ? 'Nama Pemilik Rekening (a.n.)' : 'Nama Pemegang Kas / Penanggung Jawab'}
                </label>
                <input
                  type="text"
                  placeholder="Nama pengurus atau rekening"
                  value={createForm.account_holder}
                  onChange={(e) => setCreateForm({ ...createForm, account_holder: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
            </ModalBody>
            <ModalFooter>
              <button type="button" className="sf-btn-ghost" onClick={() => setShowCreateModal(false)}>Batal</button>
              <button type="submit" className="sf-btn" disabled={formSubmitting} style={{ background: 'var(--sf-primary)', color: '#fff' }}>
                {formSubmitting ? 'Menyimpan...' : 'Simpan'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* Edit Location Modal */}
      {editingLocation && (
        <Modal open onClose={() => setEditingLocation(null)} width={480}>
          <ModalHeader title="Edit Lokasi Dana" onClose={() => setEditingLocation(null)} />
          <form onSubmit={handleUpdateLocation}>
            <ModalBody>
              {formError && (
                <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                  {formError}
                </div>
              )}
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nama Lokasi *</label>
                <input
                  type="text"
                  value={editingLocation.name}
                  onChange={(e) => setEditingLocation({ ...editingLocation, name: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
              {editingLocation.category === 'rekening' && (
                <>
                  <div style={{ marginBottom: 12 }}>
                    <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nama Bank</label>
                    <input
                      type="text"
                      value={editingLocation.bank_name || ''}
                      onChange={(e) => setEditingLocation({ ...editingLocation, bank_name: e.target.value })}
                      style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                    />
                  </div>
                  <div style={{ marginBottom: 12 }}>
                    <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nomor Rekening</label>
                    <input
                      type="text"
                      value={editingLocation.account_number || ''}
                      onChange={(e) => setEditingLocation({ ...editingLocation, account_number: e.target.value })}
                      style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                    />
                  </div>
                </>
              )}
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nama Pemegang / Atas Nama</label>
                <input
                  type="text"
                  value={editingLocation.account_holder || ''}
                  onChange={(e) => setEditingLocation({ ...editingLocation, account_holder: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
            </ModalBody>
            <ModalFooter>
              <button type="button" className="sf-btn-ghost" onClick={() => setEditingLocation(null)}>Batal</button>
              <button type="submit" className="sf-btn" disabled={formSubmitting} style={{ background: 'var(--sf-primary)', color: '#fff' }}>
                {formSubmitting ? 'Menyimpan...' : 'Simpan Perubahan'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* Transfer Modal */}
      {showTransferModal && (
        <Modal open onClose={() => setShowTransferModal(false)} width={480}>
          <ModalHeader title="Transfer Dana Antar-Lokasi" onClose={() => setShowTransferModal(false)} />
          <form onSubmit={handleCreateTransfer}>
            <ModalBody>
              {formError && (
                <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                  {formError}
                </div>
              )}
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Dari Lokasi Asal *</label>
                <select
                  value={transferForm.from_location_id}
                  onChange={(e) => setTransferForm({ ...transferForm, from_location_id: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                >
                  <option value="">-- Pilih Lokasi Asal --</option>
                  {locations.filter((l) => l.is_active).map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name} ({l.category.toUpperCase()}) - Saldo: Rp {formatRibuan(l.current_balance)}
                    </option>
                  ))}
                </select>
              </div>
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Ke Lokasi Tujuan *</label>
                <select
                  value={transferForm.to_location_id}
                  onChange={(e) => setTransferForm({ ...transferForm, to_location_id: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                >
                  <option value="">-- Pilih Lokasi Tujuan --</option>
                  {locations.filter((l) => l.is_active && l.id !== transferForm.from_location_id).map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name} ({l.category.toUpperCase()}) - Saldo: Rp {formatRibuan(l.current_balance)}
                    </option>
                  ))}
                </select>
              </div>
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nominal Pemindahan (Rp) *</label>
                <input
                  type="text"
                  placeholder="Contoh: 500000"
                  value={transferForm.amount}
                  onChange={(e) => setTransferForm({ ...transferForm, amount: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Catatan / Keterangan</label>
                <textarea
                  placeholder="Contoh: Tarik tunai dari BCA untuk kasir brankas"
                  value={transferForm.notes}
                  onChange={(e) => setTransferForm({ ...transferForm, notes: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)', minHeight: 60 }}
                />
              </div>
            </ModalBody>
            <ModalFooter>
              <button type="button" className="sf-btn-ghost" onClick={() => setShowTransferModal(false)}>Batal</button>
              <button type="submit" className="sf-btn" disabled={formSubmitting} style={{ background: 'var(--sf-primary)', color: '#fff' }}>
                {formSubmitting ? 'Memproses...' : 'Kirim Transfer'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* Opening Balance Modal */}
      {showOpeningBalanceModal && (
        <Modal open onClose={() => setShowOpeningBalanceModal(false)} width={480}>
          <ModalHeader title="Alokasi Saldo Awal Kas Historis" onClose={() => setShowOpeningBalanceModal(false)} />
          <form onSubmit={handleSetOpeningBalance}>
            <ModalBody>
              <div style={{ padding: 10, background: 'rgba(56, 132, 255, 0.1)', borderRadius: 6, fontSize: 12, color: 'var(--sf-text)', marginBottom: 12 }}>
                Saldo awal adalah <strong>alokasi dari sisa saldo kas historis ($U$)</strong> yang belum terpetakan ke instrumen mana pun. Alokasi ini tidak menambah pemasukan baru, melainkan mendistribusikan kas yang sudah ada ke rekening/brankas.
                <div style={{ marginTop: 4, fontWeight: 700 }}>
                  Saldo Kas Belum Teralokasi Saat Ini: Rp {formatRibuan(reconciliation?.unallocated_balance || '0')}
                </div>
              </div>

              {formError && (
                <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                  {formError}
                </div>
              )}

              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Lokasi Tujuan Alokasi *</label>
                <select
                  value={openingBalanceForm.location_id}
                  onChange={(e) => setOpeningBalanceForm({ ...openingBalanceForm, location_id: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                >
                  <option value="">-- Pilih Lokasi --</option>
                  {locations.filter((l) => l.is_active).map((l) => (
                    <option key={l.id} value={l.id}>
                      {l.name} ({l.category.toUpperCase()})
                    </option>
                  ))}
                </select>
              </div>

              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Nominal Alokasi (Rp) *</label>
                <input
                  type="text"
                  placeholder="Contoh: 1000000"
                  value={openingBalanceForm.amount}
                  onChange={(e) => setOpeningBalanceForm({ ...openingBalanceForm, amount: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>

              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Catatan</label>
                <input
                  type="text"
                  placeholder="Contoh: Saldo awal pembukaan rekening"
                  value={openingBalanceForm.notes}
                  onChange={(e) => setOpeningBalanceForm({ ...openingBalanceForm, notes: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
            </ModalBody>
            <ModalFooter>
              <button type="button" className="sf-btn-ghost" onClick={() => setShowOpeningBalanceModal(false)}>Batal</button>
              <button type="submit" className="sf-btn" disabled={formSubmitting} style={{ background: 'var(--sf-primary)', color: '#fff' }}>
                {formSubmitting ? 'Memproses...' : 'Simpan Alokasi'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}

      {/* History Modal */}
      {historyLocation && (
        <Modal open onClose={() => setHistoryLocation(null)} width={680}>
          <ModalHeader
            title={`Riwayat Mutasi: ${historyLocation.name}`}
            subtitle={`Saldo Saat Ini: Rp ${formatRibuan(historyLocation.current_balance)}`}
            onClose={() => setHistoryLocation(null)}
          />
          <ModalBody>
            {loadingHistory ? (
              <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>Memuat riwayat...</div>
            ) : historyData.length === 0 ? (
              <div style={{ padding: 24, textAlign: 'center', color: 'var(--sf-text-muted)' }}>Belum ada mutasi pada lokasi ini.</div>
            ) : (
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid var(--sf-border)', background: 'rgba(0,0,0,0.02)' }}>
                    <th style={{ padding: '8px 12px' }}>Tanggal</th>
                    <th style={{ padding: '8px 12px' }}>Jenis Mutasi</th>
                    <th style={{ padding: '8px 12px', textAlign: 'right' }}>Perubahan</th>
                    <th style={{ padding: '8px 12px' }}>Keterangan</th>
                  </tr>
                </thead>
                <tbody>
                  {historyData.map((h) => {
                    const delta = parseMoney(h.amount_delta) || 0
                    const isPositive = delta > 0
                    return (
                      <tr key={h.id} style={{ borderBottom: '1px solid var(--sf-border)' }}>
                        <td style={{ padding: '8px 12px', color: 'var(--sf-text-muted)' }}>
                          {h.created_at ? h.created_at.slice(0, 16).replace('T', ' ') : '-'}
                        </td>
                        <td style={{ padding: '8px 12px', fontWeight: 600 }}>{h.change_type}</td>
                        <td style={{
                          padding: '8px 12px',
                          textAlign: 'right',
                          fontWeight: 700,
                          color: isPositive ? 'var(--sf-success)' : 'var(--sf-danger)',
                        }}>
                          {isPositive ? '+' : ''}Rp {formatRibuan(delta)}
                        </td>
                        <td style={{ padding: '8px 12px', color: 'var(--sf-text-muted)' }}>{h.notes || '-'}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}
          </ModalBody>
          <ModalFooter>
            <button type="button" className="sf-btn-ghost" onClick={() => setHistoryLocation(null)}>Tutup</button>
          </ModalFooter>
        </Modal>
      )}

      {/* Reverse Transfer Confirmation Modal */}
      {reversingTransfer && (
        <Modal open onClose={() => setReversingTransfer(null)} width={460}>
          <ModalHeader title="Batalkan Transfer Dana (Reversal)" onClose={() => setReversingTransfer(null)} />
          <form onSubmit={handleReverseTransfer}>
            <ModalBody>
              <div style={{ marginBottom: 12, fontSize: 13, color: 'var(--sf-text)' }}>
                Apakah Anda yakin ingin membatalkan transfer sebesar <strong>Rp {formatRibuan(reversingTransfer.amount)}</strong> dari <strong>{reversingTransfer.from_location_name}</strong> ke <strong>{reversingTransfer.to_location_name}</strong>?
              </div>
              <div style={{ marginBottom: 12, fontSize: 12, color: 'var(--sf-text-muted)' }}>
                Dana akan dikembalikan ke lokasi asal dan ditarik dari lokasi tujuan. Operasi ini akan ditolak jika saldo lokasi tujuan tidak mencukupi.
              </div>
              {formError && (
                <div style={{ padding: 8, background: 'rgba(242, 73, 92, 0.1)', color: 'var(--sf-danger)', borderRadius: 4, marginBottom: 12, fontSize: 13 }}>
                  {formError}
                </div>
              )}
              <div style={{ marginBottom: 12 }}>
                <label className="sf-label" style={{ display: 'block', marginBottom: 4, fontWeight: 600, fontSize: 13 }}>Alasan Pembatalan *</label>
                <input
                  type="text"
                  placeholder="Contoh: Salah nominal transfer, duplikasi input"
                  value={reversalReason}
                  onChange={(e) => setReversalReason(e.target.value)}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: 6, border: '1px solid var(--sf-border)', background: 'var(--sf-surface)', color: 'var(--sf-text)' }}
                />
              </div>
            </ModalBody>
            <ModalFooter>
              <button type="button" className="sf-btn-ghost" onClick={() => setReversingTransfer(null)}>Batal</button>
              <button type="submit" className="sf-btn" disabled={formSubmitting} style={{ background: 'var(--sf-danger)', color: '#fff' }}>
                {formSubmitting ? 'Memproses...' : 'Konfirmasi Reversal'}
              </button>
            </ModalFooter>
          </form>
        </Modal>
      )}
    </div>
  )
}
