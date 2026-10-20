import { useState, useEffect, useMemo, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  apiListTransactions,
  apiGetBalance,
  type ApiTransaction,
  getSessionPair,
} from '@/app/api'
import {
  formatRupiah,
  formatTanggal,
  getJenisVariant,
  getJenisLabel,
  formatKeteranganKas,
} from './kasTypes'
import { FileTextOutlined } from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { RowActionMenu } from '@/components/RowActionMenu'
import { useAuth } from '@/app/AuthContext'

type FilterType = 'semua' | 'jenis' | 'kategori' | 'periode'

function filterLabel(type: FilterType, value: string | null): string | null {
  if (!value) return null
  switch (type) {
    case 'jenis':
      return value === 'masuk' ? 'Kas Masuk' : 'Kas Keluar'
    case 'kategori':
      return value
    case 'periode':
      {
        const [year, month] = value.split('-')
        const months = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
        return `${months[parseInt(month, 10) - 1]} ${year}`
      }
    default:
      return null
  }
}

const jenisFilterOptions = [
  { value: 'semua' as const, label: 'Semua' },
  { value: 'jenis' as const, label: 'Jenis' },
]

export function KasReport() {
  const { user } = useAuth()
  const navigate = useNavigate()

  const [search, setSearch] = useState('')
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [jenisValue, setJenisValue] = useState<string | null>(null)
  const [kategoriValue, setKategoriValue] = useState<string | null>(null)
  const [periodeValue, setPeriodeValue] = useState<string | null>(null)
  const [kategoriOpen, setKategoriOpen] = useState(false)
  const [periodeOpen, setPeriodeOpen] = useState(false)
  const kategoriDropdownRef = useRef<HTMLDivElement>(null)
  const periodeDropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!kategoriOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (kategoriDropdownRef.current && !kategoriDropdownRef.current.contains(e.target as Node)) {
        setKategoriOpen(false)
      }
    }
    const handleKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape') setKategoriOpen(false)
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [kategoriOpen])

  useEffect(() => {
    if (!periodeOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (periodeDropdownRef.current && !periodeDropdownRef.current.contains(e.target as Node)) {
        setPeriodeOpen(false)
      }
    }
    const handleKeyDown = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape') setPeriodeOpen(false)
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [periodeOpen])

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:kas-report', [10, 25, 50, 100])

  const [allTransactions, setAllTransactions] = useState<ApiTransaction[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [balance, setBalance] = useState<{ total_income: string; total_expense: string; net_balance: string } | null>(null)
  const [error, setError] = useState<string | null>(null)

  const fetchData = useCallback(async () => {
    if (!user?.rt?.id) return
    setLoading(true)
    setError(null)
    try {
      const token = getSessionPair()?.accessToken ?? ''
      const [res, bal] = await Promise.all([
        apiListTransactions(token, {
          page: 1,
          page_size: 1000, // fetch all for report
          type: jenisValue === 'masuk' ? 'income' : jenisValue === 'keluar' ? 'expense' : undefined,
          start_date: periodeValue ? `${periodeValue}-01` : undefined,
          end_date: periodeValue ? `${periodeValue}-28` : undefined,
        }),
        apiGetBalance(token),
      ])
      setAllTransactions(res.data ?? [])
      setTotal(res.pagination.total)
      setBalance({ total_income: bal.total_income, total_expense: bal.total_expense, net_balance: bal.net_balance })
    } catch (e: any) {
      setError(e?.message ?? 'Gagal memuat laporan')
    } finally {
      setLoading(false)
    }
  }, [user?.rt?.id, jenisValue, periodeValue])

  useEffect(() => {
    void fetchData()
  }, [fetchData])

  // Client-side search + category filter
  const filtered = useMemo(() => {
    let result = allTransactions
    if (kategoriValue) {
      result = result.filter(t => t.category_name === kategoriValue)
    }
    if (search.trim()) {
      const q = search.toLowerCase()
      result = result.filter(t =>
        t.description.toLowerCase().includes(q) ||
        (t.category_name ?? '').toLowerCase().includes(q)
      )
    }
    return result
  }, [allTransactions, kategoriValue, search])

  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const paginated = filtered.slice((page - 1) * pageSize, page * pageSize)

  const totalMasuk = balance?.total_income ?? '0'
  const totalKeluar = balance?.total_expense ?? '0'
  const saldo = balance?.net_balance ?? '0'

  // Category breakdown from API data
  const categoryBreakdown = useMemo(() => {
    const map: Record<string, { masuk: number; keluar: number }> = {}
    for (const t of allTransactions) {
      const cat = t.category_name || 'Lainnya'
      if (!map[cat]) map[cat] = { masuk: 0, keluar: 0 }
      if (t.type === 'income') map[cat].masuk += Number(t.amount)
      else map[cat].keluar += Number(t.amount)
    }
    return Object.entries(map)
      .map(([kategori, amounts]) => ({ kategori, ...amounts }))
      .sort((a, b) => b.masuk + b.keluar - (a.masuk + a.keluar))
  }, [allTransactions])

  const allKategoriValues = useMemo(() => {
    const set = new Set<string>()
    for (const t of allTransactions) if (t.category_name) set.add(t.category_name)
    return Array.from(set).sort()
  }, [allTransactions])

  const allPeriodeValues = useMemo(() => {
    const set = new Set<string>()
    for (const t of allTransactions) set.add(t.occurred_at.substring(0, 7))
    return Array.from(set).sort().reverse()
  }, [allTransactions])

  const handleDetail = (id: string) => navigate(`/kas/${id}`)

  const handlePage = (p: number) => { if (p >= 1 && p <= totalPages) setPage(p) }
  const handlePageSizeChange = (newSize: number) => { setPageSize(newSize); setPage(1) }
  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    setKategoriOpen(false)
    setPeriodeOpen(false)
    if (filter === 'semua') {
      setJenisValue(null)
      setKategoriValue(null)
      setPeriodeValue(null)
    }
  }
  const handleSearch = (value: string) => { setSearch(value); setPage(1) }
  const handleKategoriSelect = (val: string) => { setKategoriValue(val); setKategoriOpen(false); setPage(1) }
  const handleKategoriClear = () => { setKategoriValue(null); setKategoriOpen(false) }
  const handlePeriodeSelect = (val: string) => { setPeriodeValue(val); setPeriodeOpen(false); setPage(1) }
  const handlePeriodeClear = () => { setPeriodeValue(null); setPeriodeOpen(false) }

  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 12 }}>
        <button type="button" className="sf-btn-ghost" onClick={() => navigate('/kas')} style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '6px 12px' }}>
          ← Kembali
        </button>
      </div>

      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <FileTextOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Laporan KAS</div>
            <div className="sf-page-header-subtitle">Ringkasan dan rincian arus kas RT</div>
          </div>
        </div>
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, marginTop: 12, marginBottom: 12 }}>
        <div style={{ background: 'color-mix(in srgb, var(--sf-info) 8%, var(--sf-bg))', borderRadius: 'var(--sf-radius-md)', border: '1px solid color-mix(in srgb, var(--sf-info) 35%, var(--sf-border))', padding: '16px', boxShadow: '0 0 10px rgba(87, 148, 242, 0.08)' }}>
          <div style={{ fontSize: 11, color: 'var(--sf-info)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>Saldo Berjalan</div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-info)' }}>Rp {formatRupiah(saldo)}</div>
        </div>
        <div style={{ background: 'color-mix(in srgb, var(--sf-success) 8%, var(--sf-bg))', borderRadius: 'var(--sf-radius-md)', border: '1px solid color-mix(in srgb, var(--sf-success) 35%, var(--sf-border))', padding: '16px', boxShadow: '0 0 10px rgba(115, 191, 105, 0.08)' }}>
          <div style={{ fontSize: 11, color: 'var(--sf-success)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>Total Kas Masuk</div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-success)' }}>+Rp {formatRupiah(totalMasuk)}</div>
        </div>
        <div style={{ background: 'color-mix(in srgb, var(--sf-danger) 8%, var(--sf-bg))', borderRadius: 'var(--sf-radius-md)', border: '1px solid color-mix(in srgb, var(--sf-danger) 35%, var(--sf-border))', padding: '16px', boxShadow: '0 0 10px rgba(242, 73, 92, 0.08)' }}>
          <div style={{ fontSize: 11, color: 'var(--sf-danger)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>Total Kas Keluar</div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-danger)' }}>-Rp {formatRupiah(totalKeluar)}</div>
        </div>
      </div>

      {/* Category Breakdown */}
      <div style={{ marginTop: 12, marginBottom: 12 }}>
        <h3 style={{ fontSize: 14, fontWeight: 700, color: 'var(--sf-text)', margin: '0 0 10px 0', textTransform: 'uppercase' }}>Rincian per Kategori</h3>
        <div className="sf-content-surface" style={{ padding: 0 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr>
                <th style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>Kategori</th>
                <th style={{ textAlign: 'right', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>Pemasukan</th>
                <th style={{ textAlign: 'right', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>Pengeluaran</th>
                <th style={{ textAlign: 'right', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>Saldo</th>
              </tr>
            </thead>
            <tbody>
              {categoryBreakdown.map((cat) => (
                <tr key={cat.kategori}>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', color: 'var(--sf-text)' }}>{cat.kategori}</td>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', color: 'var(--sf-success)', fontWeight: 600 }}>{formatRupiah(String(cat.masuk))}</td>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', color: 'var(--sf-danger)', fontWeight: 600 }}>{formatRupiah(String(cat.keluar))}</td>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', fontWeight: 700, color: cat.masuk - cat.keluar >= 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>{formatRupiah(String(cat.masuk - cat.keluar))}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Transaction List */}
      <div className="sf-page-header-surface" style={{ marginTop: 16, marginBottom: 0 }}>
        <div className="sf-page-header-copy">
          <div className="sf-page-header-title-text">Riwayat Transaksi</div>
        </div>
      </div>
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            <FilterDropdown value={filterType} options={jenisFilterOptions} onChange={handleFilter} ariaLabel="Tipe filter" />

            <SearchBox
              value={search}
              onChange={(v) => setSearch(v)}
              onSearch={handleSearch}
              placeholder="Cari keterangan, kategori..."
              statusControl={
                filterType === 'kategori' ? (
                  <div className="sf-status-selector">
                    {filterLabel('kategori', kategoriValue) ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {filterLabel('kategori', kategoriValue)}
                        <button type="button" className="sf-status-badge-clear" onClick={handleKategoriClear}>×</button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown" ref={kategoriDropdownRef}>
                        <button
                          type="button"
                          className="sf-status-selector-trigger"
                          onClick={() => setKategoriOpen((prev) => !prev)}
                          aria-haspopup="listbox"
                          aria-expanded={kategoriOpen}
                        >
                          Pilih Kategori<span style={{ fontSize: 9, color: 'var(--sf-text-muted)' }}>▼</span>
                        </button>
                        {kategoriOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {allKategoriValues.map((kat) => (
                              <button key={kat} type="button" className={`sf-status-dropdown-item ${kategoriValue === kat ? 'sf-status-dropdown-item-active' : ''}`} onClick={() => handleKategoriSelect(kat)}>{kat}</button>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ) : filterType === 'periode' ? (
                  <div className="sf-status-selector">
                    {filterLabel('periode', periodeValue) ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {filterLabel('periode', periodeValue)}
                        <button type="button" className="sf-status-badge-clear" onClick={handlePeriodeClear}>×</button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown" ref={periodeDropdownRef}>
                        <button
                          type="button"
                          className="sf-status-selector-trigger"
                          onClick={() => setPeriodeOpen((prev) => !prev)}
                          aria-haspopup="listbox"
                          aria-expanded={periodeOpen}
                        >
                          Pilih Periode<span style={{ fontSize: 9, color: 'var(--sf-text-muted)' }}>▼</span>
                        </button>
                        {periodeOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {allPeriodeValues.map((p) => {
                              const [year, month] = p.split('-')
                              const months = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
                              return (
                                <button key={p} type="button" className={`sf-status-dropdown-item ${periodeValue === p ? 'sf-status-dropdown-item-active' : ''}`} onClick={() => handlePeriodeSelect(p)}>
                                  {months[parseInt(month, 10) - 1]} {year}
                                </button>
                              )
                            })}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ) : null
              }
            />
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="sf-error-banner" role="alert">
            {error}
          </div>
        )}

        {/* Loading */}
        {loading && <div className="sf-loading-banner">Memuat laporan...</div>}

        {/* Transaction Table */}
        {!loading && !error && paginated.length > 0 && (
          <div className="table-wrapper">
            <table style={{ width: 'max-content', minWidth: '100%' }}>
              <thead>
                <tr>
                  <th style={{ width: 120, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>TANGGAL</th>
                  <th style={{ width: 120, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>JENIS</th>
                  <th style={{ width: 150, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>KATEGORI</th>
                  <th style={{ minWidth: 220, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>KETERANGAN</th>
                  <th style={{ width: 150, textAlign: 'right', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>NOMINAL</th>
                  <th style={{ width: 80, textAlign: 'center', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>AKSI</th>
                </tr>
              </thead>
              <tbody>
                {paginated.map((t) => (
                  <tr key={t.id}>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13 }}>{formatTanggal(t.occurred_at)}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)' }}>
                      <Badge variant={getJenisVariant(t.type === 'income' ? 'masuk' : 'keluar')}>{getJenisLabel(t.type === 'income' ? 'masuk' : 'keluar')}</Badge>
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13 }}>{t.category_name ?? '\u2014'}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, maxWidth: 250, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.description}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, fontWeight: 600, textAlign: 'right', fontFamily: 'monospace', color: t.type === 'income' ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                      {formatKeteranganKas(t.amount, t.type === 'income' ? 'masuk' : 'keluar')}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'center' }}>
                      <RowActionMenu items={[{ label: 'Detail', onClick: () => handleDetail(t.id) }]} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {!loading && paginated.length > 0 && (
          <Pagination
            page={page}
            pageSize={pageSize}
            total={total}
            totalPages={totalPages}
            onPageChange={handlePage}
            onPageSizeChange={handlePageSizeChange}
            pageSizeOptions={[10, 25, 50, 100]}
          />
        )}
      </div>
    </>
  )
}
