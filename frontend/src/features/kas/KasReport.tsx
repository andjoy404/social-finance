import { useState, useEffect, useMemo, useCallback } from 'react'
import {
  apiListTransactions,
  apiGetBalance,
  apiGetRT,
  type ApiTransaction,
  getSessionPair,
} from '@/app/api'

import { useAuth } from '@/app/AuthContext'
import logoUrl from '@/assets/logo.png'
import {
  buildCategoryRecap,
  buildKasReport,
  buildStatementRows,
  formatNum,
  formatRp,
  periodLabel,
  bucketOf,
  type KasReportIdentity,
} from './kasReportModel'

/**
 * Aturan akses Laporan KAS.
 *  - Lihat: semua pengguna login kecuali jabatan keamanan/operasional;
 *    super_admin selalu boleh.
 *  - Ekspor: jabatan ketua / wakil_ketua, atau role bendahara.
 */
export function getKasReportAccess(
  user: { role?: unknown; jabatan?: unknown; systemRole?: unknown } | null,
) {
  const norm = (v: unknown) => (typeof v === 'string' ? v.toLowerCase().trim() : '')
  const role = norm(user?.role)
  const jabatan = norm(user?.jabatan)
  const isSuperAdmin = user?.systemRole === 'super_admin'
  const blocked = jabatan === 'keamanan' || jabatan === 'operasional'
  return {
    canView: isSuperAdmin || (user !== null && !blocked),
    canExport: jabatan === 'ketua' || jabatan === 'wakil_ketua' || role === 'bendahara',
  }
}

const PAGE_LIMIT = 100

async function fetchAllPostedTransactions(token: string, period: string | null) {
  const all: ApiTransaction[] = []
  let page = 1
  let totalPages = 1
  do {
    const res = await apiListTransactions(token, {
      page,
      page_size: PAGE_LIMIT,
      status: 'posted',
      period: period ?? undefined,
    })
    all.push(...(res.data ?? []))
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

export function KasReport(props: { onClose?: () => void; onNavigateBack?: () => void; period?: string | null } = {}) {
  const { user } = useAuth()
  const { canView: canViewReport, canExport } = getKasReportAccess(user)

  const [periodeValue, setPeriodeValue] = useState<string | null>(props.period ?? null)

  // Sinkronkan state lokal dengan prop saat parent mengubah bulan/periode
  useEffect(() => {
    setPeriodeValue(props.period ?? null)
  }, [props.period])

  const [allTransactions, setAllTransactions] = useState<ApiTransaction[]>([])
  const [loading, setLoading] = useState(true)
  const [balance, setBalance] = useState<{ opening: string; closing: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [exportError, setExportError] = useState<string | null>(null)
  const [exporting, setExporting] = useState<'pdf' | 'xlsx' | null>(null)
  const [identityExtra, setIdentityExtra] = useState<Partial<KasReportIdentity>>({})

  const rtId = user?.rt?.id
  const fetchData = useCallback(async () => {
    if (!rtId || !canViewReport) return
    setLoading(true)
    setError(null)
    try {
      const token = getSessionPair()?.accessToken ?? ''
      const [txs, bal] = await Promise.all([
        fetchAllPostedTransactions(token, periodeValue),
        periodeValue ? apiGetBalance(token, { period: periodeValue }) : apiGetBalance(token),
      ])
      setAllTransactions(txs)
      setBalance({
        opening: bal.opening_balance ?? '0',
        closing: bal.closing_balance ?? bal.net_balance ?? '0',
      })
    } catch (e) {
      setBalance(null)
      setAllTransactions([])
      setError(e instanceof Error ? e.message : 'Gagal memuat laporan')
    } finally {
      setLoading(false)
    }
  }, [rtId, canViewReport, periodeValue])

  useEffect(() => { void fetchData() }, [fetchData])

  // Identitas organisasi opsional
  useEffect(() => {
    if (!rtId || !canViewReport) return
    let cancelled = false
    void (async () => {
      try {
        const token = getSessionPair()?.accessToken ?? ''
        const rt = await apiGetRT(token, rtId)
        if (cancelled) return
        setIdentityExtra({
          rtLabel: rt.rt ? `RT ${rt.rt}` : null,
          rwLabel: rt.rw ? `RW ${String(rt.rw).padStart(3, '0')}` : null,
          address: rt.address ?? null,
        })
      } catch (e) {
        console.warn('Detail RT tidak tersedia untuk header laporan:', e)
      }
    })()
    return () => { cancelled = true }
  }, [rtId, canViewReport])

  const identity: KasReportIdentity = useMemo(() => ({
    orgName: user?.rt?.name ?? '',
    rtLabel: identityExtra.rtLabel ?? (user?.rtCode ? `RT ${user.rtCode}` : null),
    rwLabel: identityExtra.rwLabel ?? null,
    address: identityExtra.address ?? null,
  }), [user?.rt?.name, user?.rtCode, identityExtra])

  const report = useMemo(() => {
    if (!balance) return null
    try {
      const m = buildKasReport({
        transactions: allTransactions,
        openingBalance: balance.opening,
        apiClosingBalance: balance.closing,
        period: periodeValue,
        identity,
      })
      // ---- INTRUMEN DEBUG: tampilkan transaksi expense → Transfer ----
      const expenseTransfer = allTransactions
        .filter(
          (t) =>
            t.status === 'posted' &&
            t.type === 'expense' &&
            t.category_name === 'Operasional RT',
        )
        .map((t) => {
          const b = bucketOf(t.payment_method)
          return {
            id: t.id,
            amount: t.amount,
            raw_method: t.payment_method,
            bucket: b,
            description: t.description,
          }
        })
      if (expenseTransfer.length > 0) {
        console.group('⚠️ KAS DEBUG — Operasional RT')
        console.log('Periode:', periodeValue || 'Semua')
        console.log('Total transaksi:', allTransactions.length)
        console.table(expenseTransfer, ['id', 'amount', 'raw_method', 'bucket', 'description'])
        console.log('Agregasi model:', { income: m.income, expense: m.expense, totalExpenseCents: m.totalExpenseCents })
        console.groupEnd()
      }
      // ---- END INTRUMEN DEBUG ----
      return m
    } catch (e) {
      console.error('Gagal menyusun laporan:', e)
      return null
    }
  }, [allTransactions, balance, periodeValue, identity])

  const statementRows = useMemo(() => (report ? buildStatementRows(report) : []), [report])
  const recap = useMemo(() => (report ? buildCategoryRecap(report) : null), [report])

  const handleExport = useCallback(async (kind: 'pdf' | 'xlsx') => {
    if (!canExport || !report) return
    setExportError(null)
    setExporting(kind)
    try {
      const logo = await loadLogoDataUrl()
      const m = { ...report, identity: { ...report.identity, logoDataUrl: logo } }
      const mod = await import('./kasReportExport')
      if (kind === 'pdf') await mod.exportKasReportPdf(m, allTransactions)
      else await mod.exportKasReportXlsx(m, allTransactions)
    } catch (e) {
      console.error(`Failed to export ${kind.toUpperCase()}:`, e)
      setExportError(`Gagal mengekspor ${kind.toUpperCase()}: ${e instanceof Error ? e.message : 'kesalahan tidak diketahui'}`)
    } finally {
      setExporting(null)
    }
  }, [canExport, report, allTransactions])

  if (!canViewReport) {
    return (
      <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--sf-danger)' }}>
        <div style={{ fontSize: 48, marginBottom: 16 }}>🔒</div>
        <h3 style={{ fontSize: 16, fontWeight: 700, marginBottom: 8 }}>Akses Ditolak</h3>
        <p style={{ fontSize: 13, color: 'var(--sf-text-muted)' }}>
          Anda tidak memiliki izin untuk mengakses Laporan KAS.
        </p>
      </div>
    )
  }

  const titleParts = ['LAPORAN KEUANGAN', identity.rtLabel, identity.rwLabel].filter(Boolean).join(' ')

  // Helper untuk memisahkan alamat dan menentukan label periode
  const addressLines = useMemo(() => {
    if (!identity.address) return []
    const address = identity.address
    const kecamatanMatch = /(Kecamatan\s+\S+)/.exec(address)
    const kabupatenMatch = /(Kabupaten\s+\S+)/.exec(address)
    if (kecamatanMatch && kabupatenMatch) {
      return [
        address.substring(0, kabupatenMatch.index).trim(),
        address.substring(kabupatenMatch.index),
      ]
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
      {/* Header — sf-kas-report-header (tanpa ikon; ikon dipindahkan ke sf-modal-header di KasList.tsx) */}
      <div className="sf-kas-report-header sf-no-print" style={{ marginTop: 20, marginBottom: 24 }}>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', maxWidth: 900, margin: '0 auto' }}>
            {canExport && (
              <>
                <button type="button" className="sf-kas-export-pdf" disabled={!report || exporting !== null} onClick={() => void handleExport('pdf')}>
                  {exporting === 'pdf' ? 'Membuat PDF…' : 'Ekspor PDF'}
                </button>
                <button type="button" className="sf-kas-export-xlsx" disabled={!report || exporting !== null} onClick={() => void handleExport('xlsx')}>
                  {exporting === 'xlsx' ? 'Membuat XLSX…' : 'Ekspor XLSX'}
                </button>
                <button type="button" className="sf-btn-ghost sf-kas-export-print" disabled={!report} onClick={() => window.print()} style={{ fontSize: 11, padding: '4px 12px' }}>Cetak</button>
              </>
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

      {/* Laporan KAS — Statement */}
      {!loading && report && (
        <div className="sf-kas-doc" data-testid="kas-report-document">
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
      )}
    </>
  )
}
