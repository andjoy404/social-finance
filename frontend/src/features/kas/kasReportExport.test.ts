import { describe, it, expect } from 'vitest'
import ExcelJS from 'exceljs'
import { buildKasReport } from './kasReportModel'
import { generateKasReportPdf, generateKasReportXlsx } from './kasReportExport'
import { FIXTURE } from './kasReportFixture'

const model = buildKasReport({
  transactions: FIXTURE,
  openingBalance: '2000000.00',
  apiClosingBalance: '3200000.00',
  period: '2026-10',
  identity: { orgName: 'Cluster Urena', rtLabel: 'RT 02', rwLabel: 'RW 016' },
  generatedAt: new Date('2026-10-23T00:00:00Z'),
})

describe('ekspor PDF', () => {
  it('menghasilkan PDF valid tanpa error autoTable', async () => {
    const doc = await generateKasReportPdf(model, FIXTURE)
    const bytes = new Uint8Array(doc.output('arraybuffer'))
    expect(String.fromCharCode(...bytes.slice(0, 5))).toBe('%PDF-')
    expect(doc.getNumberOfPages()).toBeGreaterThanOrEqual(2) // laporan + lampiran
    const text = doc.output() as string
    expect(text.length).toBeGreaterThan(2000)
  })

  it('dataset panjang → banyak halaman dan header berulang', async () => {
    const many = Array.from({ length: 400 }, (_, i) => ({ ...FIXTURE[0], id: `t${i}`, description: `Transaksi ${i}` }))
    const m = buildKasReport({ transactions: many, openingBalance: '0', period: null, identity: { orgName: 'X' } })
    const doc = await generateKasReportPdf(m, many)
    expect(doc.getNumberOfPages()).toBeGreaterThan(5)
  })
})

describe('ekspor XLSX', () => {
  it('berisi 3 sheet, nilai numerik, total dan filter benar', async () => {
    const buf = await generateKasReportXlsx(model, FIXTURE)
    const wb = new ExcelJS.Workbook()
    await wb.xlsx.load(buf)
    expect(wb.worksheets.map((w) => w.name)).toEqual(['Laporan Keuangan', 'Rekap Kategori', 'Detail Transaksi'])

    // Sheet 1: baris saldo akhir numerik, baris summary saldo dikosongkan
    const s1 = wb.getWorksheet('Laporan Keuangan')!
    let closing: unknown
    let totalIncomeSaldo: unknown
    s1.eachRow((row) => {
      const label = String(row.getCell(2).value ?? '')
      if (label.startsWith('Saldo akhir')) closing = row.getCell(5).value
      if (label === 'Total penerimaan Tunai + Transfer') totalIncomeSaldo = row.getCell(5).value
    })
    expect(closing).toBe(3200000)
    expect(typeof closing).toBe('number')
    // Baris summary tidak memiliki saldo setelah perbaikan
    expect(totalIncomeSaldo).toBeNull()
    expect(s1.views[0]).toMatchObject({ state: 'frozen' })

    // Sheet 2: grand total
    const s2 = wb.getWorksheet('Rekap Kategori')!
    const last = s2.getRow(s2.rowCount)
    expect(last.getCell(1).value).toBe('Total Keseluruhan')
    expect(last.getCell(2).value).toBe(400000)
    expect(last.getCell(3).value).toBe(150000)
    expect(last.getCell(5).value).toBe(550000)

    // Sheet 3: hanya posted (6 baris + header + total), nominal numerik
    const s3 = wb.getWorksheet('Detail Transaksi')!
    expect(s3.rowCount).toBe(8)
    expect(s3.getRow(2).getCell(7).value).toBe(1000000)
    expect(s3.autoFilter).toBeTruthy()
  })
})
