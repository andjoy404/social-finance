import { describe, it, expect } from 'vitest'
import { mkdirSync, writeFileSync, statSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import ExcelJS from 'exceljs'
import { buildKasReport } from './kasReportModel'
import { generateKasReportPdf, generateKasReportXlsx } from './kasReportExport'
import { FIXTURE } from './kasReportFixture'

const out = resolve(process.cwd(), '../artifacts/kas-report-acceptance')
const logoDataUrl = `data:image/png;base64,${readFileSync(resolve(process.cwd(), 'src/assets/logo.png')).toString('base64')}`
const model = buildKasReport({
  transactions: FIXTURE,
  openingBalance: '2000000.00',
  apiClosingBalance: '3200000.00',
  period: '2026-10',
  identity: { orgName: 'Cluster Urena', rtLabel: 'RT 02', rwLabel: 'RW 016', logoDataUrl },
  generatedAt: new Date('2026-10-23T00:00:00Z'),
})

describe('KAS acceptance artifacts', () => {
  it('writes and independently reopens PDF and XLSX', async () => {
    mkdirSync(out, { recursive: true })
    const pdf = await generateKasReportPdf(model, FIXTURE)
    const pdfPath = resolve(out, 'Laporan-KAS-2026-10.pdf')
    writeFileSync(pdfPath, new Uint8Array(pdf.output('arraybuffer')))
    const xlsx = await generateKasReportXlsx(model, FIXTURE)
    const xlsxPath = resolve(out, 'Laporan-KAS-2026-10.xlsx')
    writeFileSync(xlsxPath, new Uint8Array(xlsx))

    const pdfBytes = readFileSync(pdfPath)
    expect(pdfBytes.subarray(0, 5).toString()).toBe('%PDF-')
    expect(pdf.getNumberOfPages()).toBe(2)
    expect(pdf.output()).toContain('LAPORAN KEUANGAN')
    expect(pdf.output()).toContain('Periode Oktober 2026')
    expect(pdf.output()).toContain('Saldo akhir')
    expect(pdf.output()).toContain('Total Keseluruhan')

    const wb = new ExcelJS.Workbook()
    await wb.xlsx.load(readFileSync(xlsxPath))
    expect(wb.worksheets.map((s) => s.name)).toEqual(['Laporan Keuangan', 'Rekap Kategori', 'Detail Transaksi'])
    const s1 = wb.getWorksheet('Laporan Keuangan')!
    let closingRow = 0
    let incomeRow = 0
    s1.eachRow((row, rowNumber) => {
      const label = String(row.getCell(2).value ?? '')
      if (label.startsWith('Saldo akhir')) closingRow = rowNumber
      if (label === 'Total penerimaan Tunai + Transfer') incomeRow = rowNumber
    })
    expect(closingRow).toBeGreaterThan(0)
    expect(incomeRow).toBeGreaterThan(0)
    expect(s1.getCell(closingRow, 5).value).toBe(3200000)
    // Baris summary tidak memiliki saldo setelah perbaikan saldo berjalan
    expect(s1.getCell(incomeRow, 5).value).toBeNull()
    const s2 = wb.getWorksheet('Rekap Kategori')!
    expect(s2.getRow(s2.rowCount).values).toContain(550000)
    expect(statSync(pdfPath).size).toBeGreaterThan(1000)
    expect(statSync(xlsxPath).size).toBeGreaterThan(1000)
    console.log(JSON.stringify({ pdfPath, pdfBytes: statSync(pdfPath).size, pdfPages: pdf.getNumberOfPages(), xlsxPath, xlsxBytes: statSync(xlsxPath).size, sheets: wb.worksheets.map((s) => s.name) }))
  })
})
