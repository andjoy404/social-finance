import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowLeftOutlined } from '@ant-design/icons'
import {
  KAS_TRANSACTIONS,
  getTotalKasMasuk,
  getTotalKasKeluar,
  getFilteredTransactions,
} from './kasMockData'
import {
  KAS_KATEGORI_OPTIONS,
  formatRupiah,
  formatTanggal,
  getJenisVariant,
  getJenisLabel,
  formatKeteranganKas,
} from './kasTypes'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { RowActionMenu } from '@/components/RowActionMenu'

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

const jenisValueOptions = [
  { value: 'masuk', label: 'Kas Masuk' },
  { value: 'keluar', label: 'Kas Keluar' },
]

export function KasReport() {
  const navigate = useNavigate()

  const [search, setSearch] = useState('')
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [jenisValue, setJenisValue] = useState<string | null>(null)
  const [kategoriValue, setKategoriValue] = useState<string | null>(null)
  const [periodeValue, setPeriodeValue] = useState<string | null>(null)

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:kas-report', [10, 25, 50, 100])

  const allFiltered = useMemo(
    () => getFilteredTransactions(search, jenisValue, kategoriValue, periodeValue, KAS_TRANSACTIONS),
    [search, jenisValue, kategoriValue, periodeValue]
  )

  const totalPages = Math.max(1, Math.ceil(allFiltered.length / pageSize))
  const paginated = useMemo(
    () => allFiltered.slice((page - 1) * pageSize, page * pageSize),
    [allFiltered, page, pageSize]
  )

  const totalMasuk = getTotalKasMasuk()
  const totalKeluar = getTotalKasKeluar()
  const saldo = totalMasuk - totalKeluar

  // Category breakdown
  const categoryBreakdown = useMemo(() => {
    const map: Record<string, { masuk: number; keluar: number }> = {}
    for (const t of KAS_TRANSACTIONS) {
      if (!map[t.kategori]) map[t.kategori] = { masuk: 0, keluar: 0 }
      if (t.jenis === 'masuk') map[t.kategori].masuk += t.nominal
      else map[t.kategori].keluar += t.nominal
    }
    return Object.entries(map)
      .map(([kategori, amounts]) => ({ kategori, ...amounts }))
      .sort((a, b) => b.masuk + b.keluar - (a.masuk + a.keluar))
  }, [])

  // Period filter values
  const periodeValues = useMemo(() => {
    const set = new Set<string>()
    for (const t of KAS_TRANSACTIONS) set.add(t.tanggal.substring(0, 7))
    return Array.from(set).sort().reverse()
  }, [])

  const kategoriValues = useMemo(() => {
    const set = new Set<string>()
    for (const t of KAS_TRANSACTIONS) set.add(t.kategori)
    return Array.from(set).sort()
  }, [])

  const handleDetail = (id: string) => {
    navigate(`/kas/${id}`)
  }

  const handlePage = (p: number) => {
    if (p >= 1 && p <= totalPages) setPage(p)
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }

  const handleFilter = (filter: FilterType) => {
    setFilterType(filter)
    setPage(1)
    if (filter === 'semua') {
      setJenisValue(null)
      setKategoriValue(null)
      setPeriodeValue(null)
    }
  }

  const handleSearch = (value: string) => {
    setSearch(value)
    setPage(1)
  }

  const handleJenisSelect = (val: string) => {
    setJenisValue(val)
    setPage(1)
  }

  const handleJenisClear = () => setJenisValue(null)

  const handleKategoriSelect = (val: string) => {
    setKategoriValue(val)
    setPage(1)
  }

  const handleKategoriClear = () => setKategoriValue(null)

  const handlePeriodeSelect = (val: string) => {
    setPeriodeValue(val)
    setPage(1)
  }

  const handlePeriodeClear = () => setPeriodeValue(null)

  return (
    <>
      {/* Back button */}
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 12 }}>
        <button
          type="button"
          className="sf-btn-ghost"
          onClick={() => navigate('/kas')}
          style={{ display: 'flex', alignItems: 'center', gap: 4, padding: '6px 12px' }}
        >
          ← Kembali
        </button>
      </div>

      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">Laporan KAS</div>
            <div className="sf-page-header-subtitle">
              Ringkasan dan rincian arus kas RT
            </div>
          </div>
        </div>
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, marginTop: 12, marginBottom: 12 }}>
        <div style={{ background: 'var(--sf-surface)', borderRadius: 'var(--sf-radius-md)', border: '1px solid var(--sf-border)', padding: '16px' }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Saldo Berjalan
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: saldo >= 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
            {formatRupiah(saldo)}
          </div>
        </div>
        <div style={{ background: 'var(--sf-surface)', borderRadius: 'var(--sf-radius-md)', border: '1px solid var(--sf-border)', padding: '16px' }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Kas Masuk
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-success)' }}>
            {formatRupiah(totalMasuk)}
          </div>
        </div>
        <div style={{ background: 'var(--sf-surface)', borderRadius: 'var(--sf-radius-md)', border: '1px solid var(--sf-border)', padding: '16px' }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Kas Keluar
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-danger)' }}>
            {formatRupiah(totalKeluar)}
          </div>
        </div>
      </div>

      {/* Category Breakdown */}
      <div style={{ marginTop: 12, marginBottom: 12 }}>
        <h3 style={{ fontSize: 14, fontWeight: 700, color: 'var(--sf-text)', margin: '0 0 10px 0', textTransform: 'uppercase' }}>
          Rincian per Kategori
        </h3>
        <div className="sf-content-surface" style={{ padding: 0 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr>
                <th style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                  Kategori
                </th>
                <th style={{ textAlign: 'right', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                  Pemasukan
                </th>
                <th style={{ textAlign: 'right', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                  Pengeluaran
                </th>
                <th style={{ textAlign: 'right', padding: '10px 16px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                  Saldo
                </th>
              </tr>
            </thead>
            <tbody>
              {categoryBreakdown.map((cat) => (
                <tr key={cat.kategori}>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', color: 'var(--sf-text)' }}>
                    {cat.kategori}
                  </td>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', color: 'var(--sf-success)', fontWeight: 600 }}>
                    {formatRupiah(cat.masuk)}
                  </td>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', color: 'var(--sf-danger)', fontWeight: 600 }}>
                    {formatRupiah(cat.keluar)}
                  </td>
                  <td style={{ padding: '10px 16px', borderBottom: '1px solid var(--sf-border)', textAlign: 'right', fontFamily: 'monospace', fontWeight: 700, color: cat.masuk - cat.keluar >= 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                    {formatRupiah(cat.masuk - cat.keluar)}
                  </td>
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
            <FilterDropdown
              value={filterType}
              options={jenisFilterOptions}
              onChange={handleFilter}
              ariaLabel="Tipe filter"
            />

            <SearchBox
              value={search}
              onChange={(v) => setSearch(v)}
              onSearch={handleSearch}
              placeholder="Cari keterangan, kategori..."
              statusControl={
                filterType === 'jenis' ? (
                  <div className="sf-status-selector">
                    {filterLabel('jenis', jenisValue) ? (
                      <span className={`sf-status-badge ${jenisValue === 'masuk' ? 'sf-status-badge__active' : 'sf-status-badge__inactive'}`}>
                        {filterLabel('jenis', jenisValue)}
                        <button type="button" className="sf-status-badge-clear" onClick={handleJenisClear}>×</button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button type="button" className="sf-status-selector-trigger" onClick={() => {}}>
                          Pilih Jenis <span style={{ fontSize: 9, color: 'var(--sf-text-muted)' }}>▼</span>
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {jenisValueOptions.map((opt) => (
                            <button key={opt.value} type="button" className={`sf-status-dropdown-item ${jenisValue === opt.value ? 'sf-status-dropdown-item-active' : ''}`} onClick={() => handleJenisSelect(opt.value)}>{opt.label}</button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : filterType === 'kategori' ? (
                  <div className="sf-status-selector">
                    {filterLabel('kategori', kategoriValue) ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {filterLabel('kategori', kategoriValue)}
                        <button type="button" className="sf-status-badge-clear" onClick={handleKategoriClear}>×</button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button type="button" className="sf-status-selector-trigger" onClick={() => {}}>
                          Pilih Kategori <span style={{ fontSize: 9, color: 'var(--sf-text-muted)' }}>▼</span>
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {kategoriValues.map((kat) => (
                            <button key={kat} type="button" className={`sf-status-dropdown-item ${kategoriValue === kat ? 'sf-status-dropdown-item-active' : ''}`} onClick={() => handleKategoriSelect(kat)}>{kat}</button>
                          ))}
                        </div>
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
                      <div className="sf-status-dropdown">
                        <button type="button" className="sf-status-selector-trigger" onClick={() => {}}>
                          Pilih Periode <span style={{ fontSize: 9, color: 'var(--sf-text-muted)' }}>▼</span>
                        </button>
                        <div className="sf-status-dropdown-menu" role="listbox">
                          {periodeValues.map((p) => {
                            const [year, month] = p.split('-')
                            const months = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
                            return (
                              <button key={p} type="button" className={`sf-status-dropdown-item ${periodeValue === p ? 'sf-status-dropdown-item-active' : ''}`} onClick={() => handlePeriodeSelect(p)}>
                                {months[parseInt(month, 10) - 1]} {year}
                              </button>
                            )
                          })}
                        </div>
                      </div>
                    )}
                  </div>
                ) : null
              }
            />
          </div>
        </div>

        {/* Transaction Table */}
        {paginated.length > 0 && (
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
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13 }}>{formatTanggal(t.tanggal)}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)' }}>
                      <Badge variant={getJenisVariant(t.jenis)}>{getJenisLabel(t.jenis)}</Badge>
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13 }}>{t.kategori}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, maxWidth: 250, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{t.keterangan}</td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, fontWeight: 600, textAlign: 'right', fontFamily: 'monospace', color: t.jenis === 'masuk' ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                      {formatKeteranganKas(t.nominal, t.jenis)}
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
        {paginated.length > 0 && (
          <Pagination
            page={page}
            pageSize={pageSize}
            total={allFiltered.length}
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
