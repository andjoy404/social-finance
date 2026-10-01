import { useState, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { UserOutlined, PlusOutlined, ArrowUpOutlined, ArrowDownOutlined, WalletOutlined } from '@ant-design/icons'
import { Badge } from '@/components/Badge'
import { SearchBox } from '@/components/SearchBox'
import { FilterDropdown } from '@/components/FilterDropdown'
import { Pagination } from '@/components/Pagination'
import { usePersistedPageSize } from '@/hooks/usePersistedPageSize'
import { RowActionMenu } from '@/components/RowActionMenu'
import { useAuth } from '@/app/AuthContext'
import { hasWargaWriteAccess } from '@/app/wargaAuth'
import {
  KAS_TRANSACTIONS,
  getTotalKasMasuk,
  getTotalKasKeluar,
  getFilteredTransactions,
} from './kasMockData'
import type { KasTransaction, KasJenis, KategoriKas } from './kasTypes'
import { formatRupiah, formatTanggal, getJenisVariant, getJenisLabel, formatKeteranganKas } from './kasTypes'

type FilterType = 'semua' | 'periode' | 'jenis' | 'kategori'

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

export function KasList() {
  const { user } = useAuth()
  const navigate = useNavigate()
  if (!user) return null

  const canWrite = hasWargaWriteAccess(user)

  const [loading] = useState(false)
  const [search, setSearch] = useState('')
  const [filterType, setFilterType] = useState<FilterType>('semua')
  const [jenisValue, setJenisValue] = useState<string | null>(null)
  const [kategoriValue, setKategoriValue] = useState<string | null>(null)
  const [periodeValue, setPeriodeValue] = useState<string | null>(null)
  const [jenisOpen, setJenisOpen] = useState(false)
  const [kategoriOpen, setKategoriOpen] = useState(false)
  const [periodeOpen, setPeriodeOpen] = useState(false)

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = usePersistedPageSize('social-finance:table-page-size:kas', [10, 25, 50, 100])

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
    setJenisOpen(false)
    setPage(1)
  }

  const handleJenisClear = () => {
    setJenisValue(null)
  }

  const handleKategoriSelect = (val: string) => {
    setKategoriValue(val)
    setKategoriOpen(false)
    setPage(1)
  }

  const handleKategoriClear = () => {
    setKategoriValue(null)
  }

  const handlePeriodeSelect = (val: string) => {
    setPeriodeValue(val)
    setPeriodeOpen(false)
    setPage(1)
  }

  const handlePeriodeClear = () => {
    setPeriodeValue(null)
  }

  const handleDetail = useCallback((id: string) => {
    navigate(`/kas/${id}`)
  }, [navigate])

  const handleCreate = useCallback(() => {
    navigate('/kas/baru')
  }, [navigate])

  const handlePage = (p: number) => {
    if (p >= 1 && p <= totalPages) setPage(p)
  }

  const handlePageSizeChange = (newSize: number) => {
    setPageSize(newSize)
    setPage(1)
  }

  return (
    <>
      {/* PageHeader */}
      <div className="sf-page-header-surface">
        <div className="sf-page-header-left">
          <span className="sf-page-header-icon">
            <WalletOutlined style={{ fontSize: 14 }} />
          </span>
          <div className="sf-page-header-copy">
            <div className="sf-page-header-title-text">KAS</div>
            <div className="sf-page-header-subtitle">
              Catatan arus kas masuk dan keluar RT
            </div>
          </div>
        </div>
      </div>

      {/* Summary Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, marginTop: 12, marginBottom: 12 }}>
        <div style={{
          background: 'var(--sf-surface)',
          borderRadius: 'var(--sf-radius-md)',
          border: '1px solid var(--sf-border)',
          padding: '16px',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Saldo Berjalan
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: saldo >= 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
            {formatRupiah(saldo)}
          </div>
        </div>
        <div style={{
          background: 'var(--sf-surface)',
          borderRadius: 'var(--sf-radius-md)',
          border: '1px solid var(--sf-border)',
          padding: '16px',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Kas Masuk
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-success)' }}>
            {formatRupiah(totalMasuk)}
          </div>
        </div>
        <div style={{
          background: 'var(--sf-surface)',
          borderRadius: 'var(--sf-radius-md)',
          border: '1px solid var(--sf-border)',
          padding: '16px',
        }}>
          <div style={{ fontSize: 11, color: 'var(--sf-text-muted)', textTransform: 'uppercase', fontWeight: 600, marginBottom: 4 }}>
            Total Kas Keluar
          </div>
          <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--sf-danger)' }}>
            {formatRupiah(totalKeluar)}
          </div>
        </div>
      </div>

      {/* Content surface */}
      <div className="sf-content-surface">
        {/* Toolbar */}
        <div className="sf-list-toolbar">
          <div className="sf-list-toolbar-left">
            {/* Filter type dropdown */}
            <FilterDropdown
              value={filterType}
              options={jenisFilterOptions}
              onChange={handleFilter}
              ariaLabel="Tipe filter"
            />

            {/* Search */}
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
                        <button type="button" className="sf-status-badge-clear" onClick={handleJenisClear} aria-label="Hapus filter jenis">
                          ×
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button type="button" className="sf-status-selector-trigger" onClick={() => setJenisOpen(!jenisOpen)} aria-haspopup="listbox">
                          Pilih Jenis
                          <span style={{ fontSize: 9, color: 'var(--sf-text-muted)', marginLeft: 4 }}>▼</span>
                        </button>
                        {jenisOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {jenisValueOptions.map((opt) => (
                              <button
                                key={opt.value}
                                type="button"
                                className={`sf-status-dropdown-item ${jenisValue === opt.value ? 'sf-status-dropdown-item-active' : ''}`}
                                onClick={() => handleJenisSelect(opt.value)}
                              >
                                {opt.label}
                              </button>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ) : filterType === 'kategori' ? (
                  <div className="sf-status-selector">
                    {filterLabel('kategori', kategoriValue) ? (
                      <span className="sf-status-badge sf-status-badge__active">
                        {filterLabel('kategori', kategoriValue)}
                        <button type="button" className="sf-status-badge-clear" onClick={handleKategoriClear} aria-label="Hapus filter kategori">
                          ×
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button type="button" className="sf-status-selector-trigger" onClick={() => setKategoriOpen(!kategoriOpen)} aria-haspopup="listbox">
                          Pilih Kategori
                          <span style={{ fontSize: 9, color: 'var(--sf-text-muted)', marginLeft: 4 }}>▼</span>
                        </button>
                        {kategoriOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {KAS_TRANSACTIONS.map((t) => t.kategori).filter((v, i, a) => a.indexOf(v) === i).map((kat) => (
                              <button
                                key={kat}
                                type="button"
                                className={`sf-status-dropdown-item ${kategoriValue === kat ? 'sf-status-dropdown-item-active' : ''}`}
                                onClick={() => handleKategoriSelect(kat)}
                              >
                                {kat}
                              </button>
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
                        <button type="button" className="sf-status-badge-clear" onClick={handlePeriodeClear} aria-label="Hapus filter periode">
                          ×
                        </button>
                      </span>
                    ) : (
                      <div className="sf-status-dropdown">
                        <button type="button" className="sf-status-selector-trigger" onClick={() => setPeriodeOpen(!periodeOpen)} aria-haspopup="listbox">
                          Pilih Periode
                          <span style={{ fontSize: 9, color: 'var(--sf-text-muted)', marginLeft: 4 }}>▼</span>
                        </button>
                        {periodeOpen && (
                          <div className="sf-status-dropdown-menu" role="listbox">
                            {KAS_TRANSACTIONS.map((t) => t.tanggal.substring(0, 7)).filter((v, i, a) => a.indexOf(v) === i).sort().reverse().map((p) => {
                              const [year, month] = p.split('-')
                              const months = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']
                              return (
                                <button
                                  key={p}
                                  type="button"
                                  className={`sf-status-dropdown-item ${periodeValue === p ? 'sf-status-dropdown-item-active' : ''}`}
                                  onClick={() => handlePeriodeSelect(p)}
                                >
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

          {/* Create button */}
          {canWrite && (
            <button className="sf-create-btn" onClick={handleCreate}>
              <PlusOutlined style={{ marginRight: 4, fontSize: 12 }} />
              Transaksi
            </button>
          )}
        </div>

        {/* Error */}
        <div className="sf-error-banner" role="alert" style={{ display: 'none' }}>
          Tidak ada data yang ditampilkan.
        </div>

        {/* Loading */}
        {loading && (
          <div className="sf-loading-banner">
            Memuat data kas...
          </div>
        )}

        {/* Empty */}
        {!loading && paginated.length === 0 && (
          <div className="sf-empty-banner">
            Tidak ada data kas.
          </div>
        )}

        {/* Transaction Table */}
        {!loading && paginated.length > 0 && (
          <div className="table-wrapper">
            <table className="sf-warga-table" style={{ width: 'max-content', minWidth: '100%', tableLayout: 'auto' }}>
              <thead>
                <tr>
                  <th style={{ width: 120, minWidth: 120, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    TANGGAL
                  </th>
                  <th style={{ width: 120, minWidth: 120, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    JENIS
                  </th>
                  <th style={{ width: 150, minWidth: 150, textAlign: 'left', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    KATEGORI
                  </th>
                  <th style={{ minWidth: 220, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    KETERANGAN
                  </th>
                  <th style={{ minWidth: 180, textAlign: 'left', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    REFERENSI
                  </th>
                  <th style={{ width: 150, minWidth: 150, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    NOMINAL
                  </th>
                  <th style={{ width: 150, minWidth: 150, textAlign: 'right', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    SALDO
                  </th>
                  <th style={{ width: 80, minWidth: 80, textAlign: 'center', whiteSpace: 'nowrap', padding: '10px 12px', fontSize: 11, fontWeight: 600, textTransform: 'uppercase', color: 'var(--sf-text-muted)', borderBottom: '1px solid var(--sf-border)' }}>
                    AKSI
                  </th>
                </tr>
              </thead>
              <tbody>
                {paginated.map((t) => (
                  <tr key={t.id}>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)' }}>
                      {formatTanggal(t.tanggal)}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)' }}>
                      <Badge variant={getJenisVariant(t.jenis)}>
                        {getJenisLabel(t.jenis)}
                      </Badge>
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', whiteSpace: 'nowrap' }}>
                      {t.kategori}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, color: 'var(--sf-text)', maxWidth: 250, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {t.keterangan}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 12, color: 'var(--sf-text-muted)', maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {t.referensi ?? '\u2014'}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, fontWeight: 600, textAlign: 'right', fontFamily: 'monospace', whiteSpace: 'nowrap', color: t.jenis === 'masuk' ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                      {formatKeteranganKas(t.nominal, t.jenis)}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', fontSize: 13, fontWeight: 600, textAlign: 'right', fontFamily: 'monospace', whiteSpace: 'nowrap', color: t.saldo >= 0 ? 'var(--sf-success)' : 'var(--sf-danger)' }}>
                      {formatRupiah(t.saldo)}
                    </td>
                    <td style={{ padding: '10px 12px', borderBottom: '1px solid var(--sf-border)', textAlign: 'center' }}>
                      <RowActionMenu
                        items={[
                          { label: 'Detail', onClick: () => handleDetail(t.id) },
                        ]}
                      />
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
