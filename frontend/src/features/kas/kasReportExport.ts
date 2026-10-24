/**
 * Ekspor Laporan KAS (PDF + XLSX) dari `KasReportModel`.
 *
 * jspdf-autotable v5 TIDAK lagi menambahkan `doc.autoTable` ke instance jsPDF
 * (plugin side-effect dihapus sejak v5). Penyebab error
 * "autoTable is not a function" adalah pemanggilan `doc.autoTable(...)`.
 * Cara yang benar: `import { autoTable } from 'jspdf-autotable'` lalu
 * `autoTable(doc, options)`; posisi akhir tabel dibaca dari
 * `(doc as any).lastAutoTable.finalY`.
 */
import type { ApiTransaction } from '@/app/api'
import {
  buildCategoryRecap,
  buildStatementRows,
  formatNum,
  formatRp,
  parseCents,
  reportFileBase,
  bucketOf,
  type KasReportModel,
} from './kasReportModel'
import { formatTanggal } from './kasTypes'

const DATE_ID = (d: Date) =>
  d.toLocaleDateString('id-ID', { day: '2-digit', month: 'long', year: 'numeric' })

function titleLines(m: KasReportModel): string[] {
  const id = m.identity
  const l1 = ['LAPORAN KEUANGAN', id.rtLabel, id.rwLabel].filter(Boolean).join(' ')
  const lines = [l1, id.orgName].filter((s) => !!s && s.trim() !== '')

  // Pisahkan alamat: jika ada "Kecamatan" dan "Kabupaten", pisah menjadi dua baris
  const address = id.address
  if (address) {
    const kecamatanMatch = /(Kecamatan\s+\S+)/.exec(address)
    const kabupatenMatch = /(Kabupaten\s+\S+)/.exec(address)
    if (kecamatanMatch && kabupatenMatch) {
      lines.push(address.substring(0, kabupatenMatch.index).trim())
      lines.push(address.substring(kabupatenMatch.index))
    } else {
      lines.push(address)
    }
  }

  // Gunakan "Tahun YYYY" untuk periode tahunan, "Periode XXX" untuk lainnya
  if (m.period && m.period.trim()) {
    if (/^\d{4}$/.test(m.period)) {
      lines.push(`Tahun ${m.period}`)
    } else {
      lines.push(`Periode ${m.periodLabel}`)
    }
  } else {
    lines.push('Semua Periode')
  }
  return lines
}

// ---------------------------------------------------------------------------
// PDF
// ---------------------------------------------------------------------------

export async function generateKasReportPdf(
  m: KasReportModel,
  transactions: ApiTransaction[],
) {
  const { jsPDF } = await import('jspdf')
  const { autoTable } = await import('jspdf-autotable')
  const doc = new jsPDF({ orientation: 'portrait', unit: 'mm', format: 'a4' })
  const pageW = doc.internal.pageSize.getWidth()
  const margin = 14

  // Header: logo (opsional) + judul terpusat
  if (m.identity.logoDataUrl) {
    try {
      doc.addImage(m.identity.logoDataUrl, 'PNG', margin, 10, 22, 22)
    } catch (e) {
      console.warn('Logo tidak dapat disisipkan ke PDF:', e)
    }
  }
  let y = 14
  doc.setFont('helvetica', 'bold')
  titleLines(m).forEach((line, i) => {
    doc.setFontSize(i === 0 ? 13 : 11)
    doc.text(line, pageW / 2, y, { align: 'center' })
    y += i === 0 ? 6 : 5.5
  })
  y = Math.max(y, 36) + 2

  // Tabel utama
  const rows = buildStatementRows(m)
  autoTable(doc, {
    startY: y,
    margin: { left: margin, right: margin },
    head: [['No', 'Uraian', 'Debet', 'Kredit', 'Saldo']],
    body: rows.map((r) => [
      r.no ? String(r.no) : '',
      r.label,
      r.debit ? formatRp(r.debit) : '',
      r.credit ? formatRp(r.credit) : '',
      r.saldo !== null && r.saldo !== undefined ? formatRp(r.saldo) : '',
    ]),
    theme: 'grid',
    styles: { font: 'helvetica', fontSize: 9, cellPadding: 1.6, lineColor: [0, 0, 0], lineWidth: 0.2, textColor: [0, 0, 0] },
    headStyles: { fillColor: [255, 255, 255], textColor: [0, 0, 0], fontStyle: 'bold' },
    columnStyles: {
      0: { cellWidth: 10, halign: 'right' },
      1: { cellWidth: 'auto' },
      2: { cellWidth: 30, halign: 'right' },
      3: { cellWidth: 30, halign: 'right' },
      4: { cellWidth: 32, halign: 'right' },
    },
    showHead: 'everyPage',
    rowPageBreak: 'avoid',
    didParseCell: (data) => {
      if (data.section !== 'body') return
      const kind = rows[data.row.index]?.kind
      if (kind === 'subtotal' || kind === 'total') data.cell.styles.fontStyle = 'bold'
      if (kind === 'total') data.cell.styles.fontSize = 10
      if (kind === 'grand') {
        data.cell.styles.fontStyle = 'bold'
        data.cell.styles.fontSize = 11
      }
    },
  })

  // Rekap pengeluaran per kategori
  const recap = buildCategoryRecap(m)
  let y2 = ((doc as unknown as { lastAutoTable: { finalY: number } }).lastAutoTable.finalY) + 10
  if (y2 > doc.internal.pageSize.getHeight() - 50) {
    doc.addPage()
    y2 = 20
  }
  doc.setFont('helvetica', 'bold')
  doc.setFontSize(11)
  doc.text(['Rekap Pengeluaran per Kategori', m.period ? `Periode ${m.periodLabel}` : 'Semua Periode'], pageW / 2, y2, { align: 'center' })
  const recapHead = recap.hasOther ? ['Kategori', 'Tunai', 'Transfer', 'Tanpa Metode'] : ['Kategori', 'Tunai', 'Transfer']
  const recapRow = (c: { category?: string; cash: number; transfer: number; other: number }, label: string) =>
    recap.hasOther
      ? [label, formatNum(c.cash), formatNum(c.transfer), formatNum(c.other)]
      : [label, formatNum(c.cash), formatNum(c.transfer)]
  const recapBody = recap.rows.map((r) => recapRow(r, r.category))
  recapBody.push(recapRow(recap.grand, 'Total Keseluruhan'))
  autoTable(doc, {
    startY: y2 + 8,
    margin: { left: margin, right: margin },
    head: [recapHead],
    body: recapBody,
    theme: 'plain',
    styles: { font: 'helvetica', fontSize: 9, cellPadding: 1.4 },
    headStyles: { fillColor: [221, 235, 247], fontStyle: 'bold' },
    columnStyles: { 1: { halign: 'right' }, 2: { halign: 'right' }, 3: { halign: 'right' } },
    showHead: 'everyPage',
    rowPageBreak: 'avoid',
    didParseCell: (data) => {
      if (data.section === 'body' && data.row.index === recapBody.length - 1) {
        data.cell.styles.fontStyle = 'bold'
        data.cell.styles.fillColor = [221, 235, 247]
      }
    },
  })

  // Lampiran rincian transaksi
  const posted = transactions.filter((t) => t.status === 'posted')
  if (posted.length > 0) {
    doc.addPage()
    doc.setFont('helvetica', 'bold')
    doc.setFontSize(11)
    doc.text('Lampiran: Rincian Transaksi', margin, 16)
    autoTable(doc, {
      startY: 20,
      margin: { left: margin, right: margin },
      head: [['Tanggal', 'Jenis', 'Kategori', 'Metode Pembayaran', 'Uraian', 'Debet', 'Kredit']],
      body: posted.map((t) => [
        formatTanggal(t.occurred_at),
        t.type === 'income' ? 'Masuk' : 'Keluar',
        t.category_name ?? '-',
        methodLabel(t.payment_method),
        t.description || '-',
        t.type === 'income' ? formatRp(parseCents(t.amount)) : '',
        t.type === 'expense' ? formatRp(parseCents(t.amount)) : '',
      ]),
      theme: 'grid',
      styles: { font: 'helvetica', fontSize: 7.5, cellPadding: 1.2, lineColor: [0, 0, 0], lineWidth: 0.15, overflow: 'linebreak' },
      headStyles: { fillColor: [235, 235, 235], textColor: [0, 0, 0], fontStyle: 'bold' },
      columnStyles: {
        0: { cellWidth: 20 },
        1: { cellWidth: 13 },
        2: { cellWidth: 24 },
        3: { cellWidth: 16 },
        4: { cellWidth: 'auto' },
        5: { cellWidth: 24, halign: 'right' },
        6: { cellWidth: 24, halign: 'right' },
      },
      showHead: 'everyPage',
      rowPageBreak: 'avoid',
    })
  }

  // Nomor halaman + tanggal cetak
  const total = doc.getNumberOfPages()
  const pageH = doc.internal.pageSize.getHeight()
  for (let i = 1; i <= total; i++) {
    doc.setPage(i)
    doc.setFont('helvetica', 'normal')
    doc.setFontSize(8)
    doc.text(`Dicetak ${DATE_ID(m.generatedAt)}`, margin, pageH - 8)
    doc.text(`Halaman ${i} dari ${total}`, pageW - margin, pageH - 8, { align: 'right' })
  }
  return doc
}

function methodLabel(method: string | null | undefined): string {
  const b = bucketOf(method)
  return b === 'cash' ? 'Tunai' : b === 'transfer' ? 'Transfer' : '-'
}

export async function exportKasReportPdf(m: KasReportModel, transactions: ApiTransaction[]) {
  const doc = await generateKasReportPdf(m, transactions)
  doc.save(`${reportFileBase(m)}.pdf`)
}

// ---------------------------------------------------------------------------
// XLSX (exceljs: mendukung style, freeze pane, filter, number format)
// ---------------------------------------------------------------------------

const RP_FMT = '"Rp"#,##0;[Red]-"Rp"#,##0'

export async function generateKasReportXlsx(
  m: KasReportModel,
  transactions: ApiTransaction[],
): Promise<ArrayBuffer> {
  const ExcelJS = (await import('exceljs')).default
  const wb = new ExcelJS.Workbook()
  wb.created = m.generatedAt
  const cur = (c: number) => c / 100

  // ---- Sheet 1: Laporan Keuangan
  const s1 = wb.addWorksheet('Laporan Keuangan')
  s1.columns = [
    { width: 6 }, { width: 62 }, { width: 18 }, { width: 18 }, { width: 20 },
  ]
  titleLines(m).forEach((line, i) => {
    const r = s1.addRow([line])
    s1.mergeCells(r.number, 1, r.number, 5)
    r.font = { bold: true, size: i === 0 ? 13 : 11 }
    r.alignment = { horizontal: 'center' }
  })
  s1.addRow([])
  const head = s1.addRow(['No', 'Uraian', 'Debet', 'Kredit', 'Saldo'])
  head.font = { bold: true }
  head.eachCell((c) => {
    c.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FFEFEFEF' } }
    c.border = { top: { style: 'thin' }, bottom: { style: 'thin' }, left: { style: 'thin' }, right: { style: 'thin' } }
  })
  const headerRowNo = head.number
  for (const r of buildStatementRows(m)) {
    const row = s1.addRow([
      r.no ?? null,
      r.label,
      r.debit ? cur(r.debit) : null,
      r.credit ? cur(r.credit) : null,
      r.saldo !== null && r.saldo !== undefined ? cur(r.saldo) : null,
    ])
    for (const col of [3, 4, 5]) {
      row.getCell(col).numFmt = RP_FMT
      row.getCell(col).alignment = { horizontal: 'right' }
    }
    row.eachCell({ includeEmpty: true }, (c) => {
      c.border = { top: { style: 'thin' }, bottom: { style: 'thin' }, left: { style: 'thin' }, right: { style: 'thin' } }
    })
    if (r.kind === 'subtotal' || r.kind === 'total' || r.kind === 'grand') row.font = { bold: true, size: r.kind === 'grand' ? 12 : 11 }
  }
  s1.views = [{ state: 'frozen', ySplit: headerRowNo }]

  // ---- Sheet 2: Rekap Kategori
  const s2 = wb.addWorksheet('Rekap Kategori')
  s2.columns = [{ width: 34 }, { width: 18 }, { width: 18 }, { width: 18 }, { width: 18 }]
  const h2 = s2.addRow(['Kategori', 'Tunai', 'Transfer', 'Tanpa Metode', 'Total'])
  h2.font = { bold: true }
  h2.eachCell((c) => { c.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FFDDEBF7' } } })
  const recap = buildCategoryRecap(m)
  for (const r of recap.rows) {
    const row = s2.addRow([r.category, cur(r.cash), cur(r.transfer), cur(r.other), cur(r.total)])
    ;[2, 3, 4, 5].forEach((c) => (row.getCell(c).numFmt = RP_FMT))
  }
  const g = s2.addRow(['Total Keseluruhan', cur(recap.grand.cash), cur(recap.grand.transfer), cur(recap.grand.other), cur(recap.grand.total)])
  g.font = { bold: true }
  ;[2, 3, 4, 5].forEach((c) => (g.getCell(c).numFmt = RP_FMT))
  s2.views = [{ state: 'frozen', ySplit: 1 }]

  // ---- Sheet 3: Detail Transaksi
  const s3 = wb.addWorksheet('Detail Transaksi')
  s3.columns = [
    { width: 13 }, { width: 22 }, { width: 10 }, { width: 24 }, { width: 46 },
    { width: 12 }, { width: 18 }, { width: 18 }, { width: 10 },
  ]
  const h3 = s3.addRow(['Tanggal', 'Referensi', 'Jenis', 'Kategori', 'Keterangan', 'Metode', 'Debet', 'Kredit', 'Status'])
  h3.font = { bold: true }
  h3.eachCell((c) => { c.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FFEFEFEF' } } })
  for (const t of transactions) {
    if (t.status !== 'posted') continue
    const cents = parseCents(t.amount)
    const d = new Date(t.occurred_at)
    const row = s3.addRow([
      Number.isNaN(d.getTime()) ? t.occurred_at : d,
      t.payment_no ?? t.reference_no ?? '',
      t.type === 'income' ? 'Masuk' : 'Keluar',
      t.category_name ?? '',
      t.description ?? '',
      methodLabel(t.payment_method),
      t.type === 'income' ? cur(cents) : null,
      t.type === 'expense' ? cur(cents) : null,
      t.status,
    ])
    row.getCell(1).numFmt = 'dd/mm/yyyy'
    row.getCell(7).numFmt = RP_FMT
    row.getCell(8).numFmt = RP_FMT
  }
  const last = s3.rowCount
  const tot = s3.addRow(['', '', '', '', 'TOTAL', '', { formula: `SUBTOTAL(109,G2:G${Math.max(last, 2)})` }, { formula: `SUBTOTAL(109,H2:H${Math.max(last, 2)})` }, ''])
  tot.font = { bold: true }
  tot.getCell(7).numFmt = RP_FMT
  tot.getCell(8).numFmt = RP_FMT
  s3.autoFilter = { from: { row: 1, column: 1 }, to: { row: 1, column: 9 } }
  s3.views = [{ state: 'frozen', ySplit: 1 }]

  return (await wb.xlsx.writeBuffer()) as ArrayBuffer
}

export async function exportKasReportXlsx(m: KasReportModel, transactions: ApiTransaction[]) {
  const buf = await generateKasReportXlsx(m, transactions)
  const blob = new Blob([buf], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${reportFileBase(m)}.xlsx`
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

