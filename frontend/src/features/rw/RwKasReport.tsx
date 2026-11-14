import { useState, useEffect, useMemo, useCallback } from 'react'
import { createPortal } from 'react-dom'
import './RwKasReport.css'
import { getSessionPair } from '@/app/api'
import {
  apiGetRWKASReportDetail,
  apiListRWTransactions,
  type ApiRWTransaction,
} from './rw-api'

import { useAuth } from '@/app/AuthContext'
import logoUrl from '@/assets/logo.png'
import {
  buildCategoryRecap,
  buildKasReport,
  buildStatementRows,
  formatNum,
  formatRp,
  periodLabel,
  type KasReportIdentity,
} from '@/features/kas/kasReportModel'
import type { ApiTransaction } from '@/app/api'

/**
 * Aturan akses Laporan KAS RW (keselarasan dengan backend
 * `RequireRWScope` + `rw.kas.read` dan dengan Android):
 *  - Lihat: super_admin ATAU membership `explicit` untuk rwId ini.
 *  - Ekspor PDF: sama dengan lihat.
 *  - Ekspor XLSX & Cetak: super_admin ATAU jabatan explicit RW {ketua,
 *    wakil_ketua, sekretaris, bendahara}.
 *  - rt_inferred (Pengurus RT): ditolak seluruhnya.
 */
export function getRwKasReportAccess(
  user:
    | {
        role?: unknown
        systemRole?: unknown
        system_role?: unknown
        rwMemberships?: Array<{
          rw_id?: string
          jabatan?: string
          source?: string
        }>
      }
    | null,
  rwId: string,
) {
  const norm = (v: unknown) => (typeof v === 'string' ? v.toLowerCase().trim() : '')
  const role = norm(user?.role)
  const sysRole = norm(user?.systemRole ?? user?.system_role)
  const isSuperAdmin = sysRole === 'super_admin' || role === 'super_admin'

  const memberships = user?.rwMemberships ?? []
  const membership = memberships.find((m) => m?.rw_id === rwId)
  const isExplicitMember = !!membership && membership.source !== 'rt_inferred'

  const canView = isSuperAdmin || isExplicitMember

  const privileged =
    canView &&
    (isSuperAdmin ||
      ['ketua', 'wakil_ketua', 'sekretaris', 'bendahara'].includes(
        norm(membership?.jabatan),
      ))

  return {
    canView,
    canExport: canView,
    canExportPdf: canView,
    canExportXlsx: privileged,
    canPrint: privileged,
  }
}

const PAGE_LIMIT = 100

function rwToApiTransaction(t: ApiRWTransaction): ApiTransaction {
  return {
    ...t,
    id: t.id,
    rt_id: t.rt_id ?? '',
    type: t.type,
    amount: t.amount,
    description: t.description,
    status: t.status,
    category_id: t.category_id ?? '',
    category_name: t.category_name ?? null,
    payment_method: t.payment_method ?? null,
    occurred_at: t.occurred_at,
    created_at: t.created_at,
    reverses_transaction_id: null,
    payment_no: t.reference_no ?? null,
    expense_category: null,
    reference_no: t.reference_no ?? null,
  } as unknown as ApiTransaction
}

async function fetchAllPostedTransactions(
  token: string,
  rwId: string,
  period: string | null,
  startDate?: string | null,
  endDate?: string | null,
): Promise<ApiTransaction[]> {
  const all: ApiTransaction[] = []
  let page = 1
  let totalPages = 1
  do {
    const res = await apiListRWTransactions(token, rwId, {
      page,
      page_size: PAGE_LIMIT,
      period: period ?? undefined,
      start_date: !period && startDate ? startDate : undefined,
      end_date: !period && endDate ? endDate : undefined,
    })
    all.push(...res.data.map(rwToApiTransaction))
    totalPages = res.pagination?.total_pages ?? 1
    page++
  } while (page <= totalPages)
  return all
}

async function loadLogoDataUrl(): Promise<string | null> {
  try {
    const res = await fetch(logoUrl)
    if (!res.ok) return null
    const blob = await res.blob()
    return await new Promise<string>((resolve, reject) => {
      const fr = new FileReader()
      fr.onload = () => resolve(String(fr.result))
      fr.onerror = () => reject(fr.error)
      fr.readAsDataURL(blob)
    })
  } catch (e) {
    console.warn('Logo laporan tidak dapat dimuat:', e)
    return null
  }
}

export function RwKasReport(
  props: {
    onClose?: () => void
    onNavigateBack?: () => void
    rwId: string
    rwName?: string | null
    period?: string | null
    startDate?: string | null
    endDate?: string | null
  } = { rwId: '' },
) {
  const { user } = useAuth()
  const { rwId, rwName, startDate, endDate } = props
  const { canView: canViewReport, canExportPdf, canExportXlsx, canPrint } =
    getRwKasReportAccess(user, rwId)

  const [periodeValue, setPeriodeValue] = useState<string | null>(props.period ?? null)

  // Sinkronkan state lokal dengan prop saat parent mengubah periode
  useEffect(() => {
    setPeriodeValue(props.period ?? null)
  }, [props.period])

  const [allTransactions, setAllTransactions] = useState<ApiTransaction[]>([])
  const [loading, setLoading] = useState(true)
  const [balance, setBalance] = useState<{ opening: string; closing: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [exportError, setExportError] = useState<string | null>(null)
  const [exporting, setExporting] = useState<'pdf' | 'xlsx' | null>(null)

  const membership = user?.rwMemberships?.find((m) => m.rw_id === rwId)
  const rwNumberText = membership?.rw_number != null
    ? String(membership.rw_number).padStart(3, '0')
    : null
  const rwLabel = rwNumberText ? `RW ${rwNumberText}` : (rwName ?? 'RW')
  const effectiveName = rwName ?? membership?.rw_name ?? rwLabel

  const fetchData = useCallback(async () => {
    if (!canViewReport || !rwId) {
      setLoading(false)
      return
    }
    setLoading(true)
    setError(null)
    try {
      const token = getSessionPair()?.accessToken
      if (!token) {
        throw new Error('Sesi tidak ditemukan. Silakan login kembali.')
      }

      const periodParam = periodeValue && /^\d{4}-\d{2}$/.test(periodeValue) ? periodeValue : null
      const [txs, bal] = await Promise.all([
        fetchAllPostedTransactions(token, rwId, periodParam, startDate ?? null, endDate ?? null),
        periodParam
          ? apiGetRWKASReportDetail(token, rwId, { period: periodParam })
          : startDate && endDate
            ? apiGetRWKASReportDetail(token, rwId, { startDate, endDate })
            : apiGetRWKASReportDetail(token, rwId, {}),
      ])
      setAllTransactions(txs)
      setBalance({
        opening: bal.opening_balance ?? '0',
        closing: bal.total_balance ?? bal.net_balance ?? '0',
      })
    } catch (e) {
      setBalance(null)
      setAllTransactions([])
      setError(e instanceof Error ? e.message : 'Gagal memuat laporan')
    } finally {
      setLoading(false)
    }
  }, [rwId, canViewReport, periodeValue, startDate, endDate])

  useEffect(() => { void fetchData() }, [fetchData])

  const identity: KasReportIdentity = useMemo(() => ({
    orgName: effectiveName,
    rtLabel: null,
    rwLabel,
    address: null,
  }), [effectiveName, rwLabel])

  const report = useMemo(() => {
    if (!balance) return null
    try {
      return buildKasReport({
        transactions: allTransactions,
        openingBalance: balance.opening,
        apiClosingBalance: balance.closing,
        period: periodeValue ?? null,
        identity,
      })
    } catch (e) {
      console.error('Gagal menyusun laporan:', e)
      return null
    }
  }, [allTransactions, balance, periodeValue, identity])

  const statementRows = useMemo(() => (report ? buildStatementRows(report) : []), [report])
  const recap = useMemo(() => (report ? buildCategoryRecap(report) : null), [report])


  // Dokumen laporan — IDENTIK antara tampilan di layar dan dokumen cetak
  // (portal print), sesuai requirement "Konten yang dicetak harus sama dengan
  // laporan yang sedang dilihat pengguna".
  const StatementDocument = ({ testId }: { testId?: string }) => {
    if (!report) return null
    return (
      <div className="sf-kas-doc" data-testid={testId ?? 'kas-report-document'}>
        <div className="sf-kas-doc-head">
          <img src={logoUrl} alt="" className="sf-kas-doc-logo" />
          <div className="sf-kas-doc-title">
            <div className="sf-kas-doc-title-main">{titleParts}</div>
            {identity.orgName && <div>{identity.orgName}</div>}
            {addressLines.map((line, i) => <div key={i}>{line}</div>)}
            <div>{periodLine}</div>
          </div>
        </div>

        {report.reconciles === false && (
          <div className="sf-error-banner sf-no-print" role="alert">
            Saldo akhir hasil hitung ({formatRp(report.closingCents)}) berbeda dari saldo akhir server ({formatRp(report.apiClosingCents ?? 0)}). Periksa data sebelum mencetak.
          </div>
        )}

        {report.transactionCount === 0 && (
          <div className="sf-kas-doc-empty">Belum ada transaksi KAS pada {periodLabel(periodeValue)}.</div>
        )}

        <div className="sf-kas-doc-scroll">
          <table className="sf-kas-doc-table">
            <thead>
              <tr><th style={{ width: 36 }}>No</th><th>Uraian</th><th className="num">Debet</th><th className="num">Kredit</th><th className="num">Saldo</th></tr>
            </thead>
            <tbody>
              {statementRows.map((r, i) => (
                <tr key={i} className={`kind-${r.kind}`}>
                  <td className="num">{r.no ?? ''}</td>
                  <td>{r.label}</td>
                  <td className="num">{r.debit ? formatRp(r.debit) : ''}</td>
                  <td className="num">{r.credit ? formatRp(r.credit) : ''}</td>
                  <td className="num">{r.saldo !== null && r.saldo !== undefined ? formatRp(r.saldo) : ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {recap && (
          <div className="sf-kas-doc-recap">
            <div className="sf-kas-doc-title-main" style={{ textAlign: 'center', margin: '18px 0 8px' }}>
              Rekap Pengeluaran per Kategori<br />{periodeValue ? `Periode ${periodLabel(periodeValue)}` : 'Semua Periode'}
            </div>
            <div className="sf-kas-doc-scroll">
              <table className="sf-kas-doc-table recap">
                <thead>
                  <tr><th>Kategori</th><th className="num">Tunai</th><th className="num">Transfer</th>{recap.hasOther && <th className="num">Tanpa Metode</th>}</tr>
                </thead>
                <tbody>
                  {recap.rows.map((r) => (
                    <tr key={r.category}><td>{r.category}</td><td className="num">{formatNum(r.cash)}</td><td className="num">{formatNum(r.transfer)}</td>{recap.hasOther && <td className="num">{formatNum(r.other)}</td>}</tr>
                  ))}
                  <tr className="kind-grand"><td>Total Keseluruhan</td><td className="num">{formatNum(recap.grand.cash)}</td><td className="num">{formatNum(recap.grand.transfer)}</td>{recap.hasOther && <td className="num">{formatNum(recap.grand.other)}</td>}</tr>
                </tbody>
              </table>
            </div>
          </div>
        )}
        <div className="sf-kas-doc-foot">Dicetak {report.generatedAt.toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })}</div>
      </div>
    )
  }

  const handleExport = useCallback(async (kind: 'pdf' | 'xlsx') => {
    if (kind === 'pdf' && !canExportPdf) return
    if (kind === 'xlsx' && !canExportXlsx) return
    if (!report) return
    setExportError(null)
    setExporting(kind)
    try {
      const logo = await loadLogoDataUrl()
      const m = { ...report, identity: { ...report.identity, logoDataUrl: logo } }
      const mod = await import('@/features/kas/kasReportExport')
      if (kind === 'pdf') await mod.exportKasReportPdf(m, allTransactions)
      else await mod.exportKasReportXlsx(m, allTransactions)
    } catch (e) {
      console.error(`Failed to export ${kind.toUpperCase()}:`, e)
      setExportError(`Gagal mengekspor ${kind.toUpperCase()}: ${e instanceof Error ? e.message : 'kesalahan tidak diketahui'}`)
    } finally {
      setExporting(null)
    }
  }, [canExportPdf, canExportXlsx, report, allTransactions])

  if (!canViewReport) {
    return (
      <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--sf-danger)' }}>
        <div style={{ fontSize: 48, marginBottom: 16 }}>🔒</div>
        <h3 style={{ fontSize: 16, fontWeight: 700, marginBottom: 8 }}>Akses Ditolak</h3>
        <p style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
          Anda tidak memiliki izin untuk mengakses Laporan KAS RW ini.
        </p>
      </div>
    )
  }

  const titleParts = ['LAPORAN KEUANGAN', identity.rtLabel, identity.rwLabel].filter(Boolean).join(' ')

  const addressLines = useMemo(() => {
    if (!identity.address) return []
    const address = identity.address
    const kecamatanMatch = /(Kecamatan\s+\S+)/.exec(address)
    const kabupatenMatch = /(Kabupaten\s+\S+)/.exec(address)
    if (kecamatanMatch && kabupatenMatch) {
      return [address.substring(0, kabupatenMatch.index).trim(), address.substring(kabupatenMatch.index)]
    }
    return [address]
  }, [identity.address])

  const periodLine = useMemo(() => {
    if (!periodeValue) return 'Semua Periode'
    if (/^\d{4}$/.test(periodeValue)) return `Tahun ${periodeValue}`
    return `Periode ${periodLabel(periodeValue)}`
  }, [periodeValue])

  return (
    <>
      {/* Header — sf-kas-report-header (tanpa ikon; ikon dipindahkan ke modal header) */}
      <div className="sf-kas-report-header sf-no-print" style={{ marginTop: 20, marginBottom: 24 }}>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', maxWidth: 900, margin: '0 auto' }}>
          {canExportPdf && (
            <button type="button" className="sf-kas-export-pdf" disabled={!report || exporting !== null} onClick={() => void handleExport('pdf')}>
              {exporting === 'pdf' ? 'Membuat PDF…' : 'Ekspor PDF'}
            </button>
          )}
          {canExportXlsx && (
            <button type="button" className="sf-kas-export-xlsx" disabled={!report || exporting !== null} onClick={() => void handleExport('xlsx')}>
              {exporting === 'xlsx' ? 'Membuat XLSX…' : 'Ekspor XLSX'}
            </button>
          )}
          {canPrint && (
            <button type="button" className="sf-btn-ghost sf-kas-export-print" disabled={!report} onClick={() => window.print()} style={{ fontSize: 11, padding: '4px 12px' }}>Cetak</button>
          )}
        </div>
      </div>
      {exportError && <div className="sf-error-banner sf-no-print" role="alert">{exportError}</div>}
      {error && (
        <div className="sf-error-banner sf-no-print" role="alert">
          {error}{' '}
          <button type="button" className="sf-btn-ghost" onClick={() => void fetchData()} style={{ padding: '2px 8px', fontSize: 12 }}>Coba lagi</button>
        </div>
      )}
      {loading && <div className="sf-loading-banner sf-no-print">Memuat laporan...</div>}

      {/* Laporan KAS RW — Statement */}
      {!loading && report && <StatementDocument testId="kas-report-document" />}
      {createPortal(
        <section className="sf-kas-print-report" aria-label="Laporan KAS RW">
          {report ? (
            <StatementDocument testId="kas-print-report-document" />
          ) : !loading ? (
            <div className="sf-kas-doc-empty">Data laporan belum tersedia untuk dicetak. Muat ulang laporan KAS RW.</div>
          ) : null}
        </section>,
        document.body,
      )}
    </>
  )
}
