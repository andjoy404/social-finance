/// KAS export utility — PDF dan XLSX untuk laporan Android.
///
/// Keduanya dibangun dari [KasReportModel] yang sama dengan tampilan layar
/// (dan setara dengan versi Web).
library;

import 'dart:typed_data';

import 'package:excel/excel.dart';
import 'package:intl/intl.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;

import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/export/kas_report_model.dart';

List<String> kasTitleLines(KasReportModel m) {
  final id = m.identity;
  final first = [
    'LAPORAN KEUANGAN',
    id.rtLabel,
    id.rwLabel,
  ].where((s) => s != null && s.isNotEmpty).join(' ');
  final lines = <String>[first];
  if (id.orgName.trim().isNotEmpty) lines.add(id.orgName);

  // Pisahkan alamat: jika ada "Kecamatan" dan "Kabupaten", pisah menjadi dua baris
  final address = id.address;
  if (address != null && address.isNotEmpty) {
    // Cek apakah address mengandung "Kecamatan" dan "Kabupaten"
    final kecamatanMatch = RegExp(r'(Kecamatan\s+\S+)').firstMatch(address);
    final kabupatenMatch = RegExp(r'(Kabupaten\s+\S+)').firstMatch(address);
    if (kecamatanMatch != null && kabupatenMatch != null) {
      // Pisah sebelum "Kabupaten"
      final beforeKabupaten = address.substring(0, kabupatenMatch.start);
      lines.add(beforeKabupaten.trim());
      lines.add(address.substring(kabupatenMatch.start));
    } else {
      lines.add(address);
    }
  }

  // Gunakan "Tahun YYYY" untuk periode tahunan, "Periode XXX" untuk lainnya
  if (m.period != null && m.period!.isNotEmpty) {
    if (RegExp(r'^\d{4}$').hasMatch(m.period!)) {
      lines.add('Tahun ${m.period}');
    } else {
      lines.add('Periode ${m.periodLabel}');
    }
  } else {
    lines.add('Semua Periode');
  }
  return lines;
}

String _methodLabel(String? method) {
  final b = kasBucketOf(method);
  return b == 'cash' ? 'Tunai' : (b == 'transfer' ? 'Transfer' : '-');
}

String _dateLabel(String? iso) {
  final d = iso == null ? null : DateTime.tryParse(iso);
  if (d == null) return iso ?? '-';
  return DateFormat(
    'dd/MM/yyyy',
  ).format(d.toUtc().add(const Duration(hours: 7)));
}

// ---------------------------------------------------------------------------
// PDF
// ---------------------------------------------------------------------------

Future<Uint8List> generateKasReportPdf({
  required KasReportModel model,
  required List<BackendTransaction> transactions,
  Uint8List? logoBytes,
}) async {
  final pdf = pw.Document(
    title: model.fileBase,
    author: model.identity.orgName,
  );
  final lines = kasTitleLines(model);
  final border = pw.TableBorder.all(width: 0.5);

  pw.Widget cell(
    String text, {
    bool right = false,
    bool bold = false,
    double size = 9,
    PdfColor? bg,
  }) {
    return pw.Container(
      color: bg,
      padding: const pw.EdgeInsets.symmetric(horizontal: 4, vertical: 2),
      alignment: right ? pw.Alignment.centerRight : pw.Alignment.centerLeft,
      child: pw.Text(
        text,
        textAlign: right ? pw.TextAlign.right : pw.TextAlign.left,
        style: pw.TextStyle(
          fontSize: size,
          fontWeight: bold ? pw.FontWeight.bold : pw.FontWeight.normal,
        ),
      ),
    );
  }

  final rows = model.statementRows();
  final mainTable = pw.Table(
    border: border,
    columnWidths: {
      0: const pw.FixedColumnWidth(26),
      1: const pw.FlexColumnWidth(),
      2: const pw.FixedColumnWidth(82),
      3: const pw.FixedColumnWidth(82),
      4: const pw.FixedColumnWidth(88),
    },
    children: [
      pw.TableRow(
        repeat: true,
        children: [
          cell('No', bold: true, right: true),
          cell('Uraian', bold: true),
          cell('Debet', bold: true, right: true),
          cell('Kredit', bold: true, right: true),
          cell('Saldo', bold: true, right: true),
        ],
      ),
      for (final r in rows)
        pw.TableRow(
          children: [
            cell(r.no?.toString() ?? '', right: true),
            cell(
              r.label,
              bold: r.kind != KasRowKind.row,
              size: r.kind == KasRowKind.grand
                  ? 11
                  : (r.kind == KasRowKind.total ? 10 : 9),
            ),
            cell(
              r.debit != null && r.debit != 0 ? formatRp(r.debit!) : '',
              right: true,
            ),
            cell(
              r.credit != null && r.credit != 0 ? formatRp(r.credit!) : '',
              right: true,
            ),
            cell(
              r.saldo != null ? formatRp(r.saldo!) : '',
              right: true,
              bold: r.kind != KasRowKind.row,
              size: r.kind == KasRowKind.grand
                  ? 11
                  : (r.kind == KasRowKind.total ? 10 : 9),
            ),
          ],
        ),
    ],
  );

  final recapBg = PdfColor.fromInt(0xFFDDEBF7);
  final hasOther = model.expenseHasOther;
  pw.TableRow recapRow(
    String label,
    int cash,
    int transfer,
    int other, {
    bool bold = false,
    PdfColor? bg,
  }) {
    return pw.TableRow(
      children: [
        cell(label, bold: bold, bg: bg),
        cell(formatNum(cash), right: true, bold: bold, bg: bg),
        cell(formatNum(transfer), right: true, bold: bold, bg: bg),
        if (hasOther) cell(formatNum(other), right: true, bold: bold, bg: bg),
      ],
    );
  }

  final recapHeader = pw.TableRow(
    repeat: true,
    children: [
      cell('Kategori', bold: true, bg: recapBg),
      cell('Tunai', bold: true, right: true, bg: recapBg),
      cell('Transfer', bold: true, right: true, bg: recapBg),
      if (hasOther) cell('Tanpa Metode', bold: true, right: true, bg: recapBg),
    ],
  );

  final recapTable = pw.Table(
    columnWidths: {
      0: const pw.FlexColumnWidth(3),
      1: const pw.FlexColumnWidth(2),
      2: const pw.FlexColumnWidth(2),
      if (hasOther) 3: const pw.FlexColumnWidth(2),
    },
    children: [
      recapHeader,
      for (final r in model.expenseCategories)
        recapRow(r.category, r.cash, r.transfer, r.other),
      recapRow(
        'Grand Total',
        model.expense.cash,
        model.expense.transfer,
        model.expense.other,
        bold: true,
        bg: recapBg,
      ),
    ],
  );

  final posted = transactions.where((t) => t.status == 'posted').toList();

  pdf.addPage(
    pw.MultiPage(
      pageFormat: PdfPageFormat.a4,
      margin: const pw.EdgeInsets.all(32),
      footer: (ctx) => pw.Row(
        mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
        children: [
          pw.Text(
            'Dicetak ${DateFormat('dd/MM/yyyy').format(model.generatedAt)}',
            style: const pw.TextStyle(fontSize: 8),
          ),
          pw.Text(
            'Halaman ${ctx.pageNumber} dari ${ctx.pagesCount}',
            style: const pw.TextStyle(fontSize: 8),
          ),
        ],
      ),
      build: (ctx) => [
        pw.Row(
          crossAxisAlignment: pw.CrossAxisAlignment.center,
          children: [
            if (logoBytes != null)
              pw.Image(pw.MemoryImage(logoBytes), width: 56, height: 56)
            else
              pw.SizedBox(width: 56),
            pw.Expanded(
              child: pw.Column(
                children: [
                  for (var i = 0; i < lines.length; i++)
                    pw.Text(
                      lines[i],
                      textAlign: pw.TextAlign.center,
                      style: pw.TextStyle(
                        fontSize: i == 0 ? 13 : 11,
                        fontWeight: pw.FontWeight.bold,
                      ),
                    ),
                ],
              ),
            ),
            pw.SizedBox(width: 56),
          ],
        ),
        pw.SizedBox(height: 12),
        mainTable,
        pw.SizedBox(height: 18),
        pw.Center(
          child: pw.Text(
            'Rekap Pengeluaran per Kategori\n${model.period != null ? 'Periode ${model.periodLabel}' : 'Semua Periode'}',
            textAlign: pw.TextAlign.center,
            style: pw.TextStyle(fontSize: 11, fontWeight: pw.FontWeight.bold),
          ),
        ),
        pw.SizedBox(height: 8),
        recapTable,
        if (posted.isNotEmpty) ...[
          pw.NewPage(),
          pw.Text(
            'Lampiran: Rincian Transaksi',
            style: pw.TextStyle(fontSize: 11, fontWeight: pw.FontWeight.bold),
          ),
          pw.SizedBox(height: 6),
          pw.Table(
            border: pw.TableBorder.all(width: 0.4),
            columnWidths: {
              0: const pw.FixedColumnWidth(52),
              1: const pw.FixedColumnWidth(34),
              2: const pw.FixedColumnWidth(62),
              3: const pw.FixedColumnWidth(44),
              4: const pw.FlexColumnWidth(),
              5: const pw.FixedColumnWidth(66),
              6: const pw.FixedColumnWidth(66),
            },
            children: [
              pw.TableRow(
                repeat: true,
                children: [
                  for (final h in const [
                    'Tanggal',
                    'Jenis',
                    'Kategori',
                    'Metode',
                    'Keterangan',
                    'Debet',
                    'Kredit',
                  ])
                    cell(h, bold: true, size: 7.5),
                ],
              ),
              for (final t in posted)
                pw.TableRow(
                  children: [
                    cell(_dateLabel(t.occurredAt), size: 7.5),
                    cell(t.type == 'income' ? 'Masuk' : 'Keluar', size: 7.5),
                    cell(t.categoryName ?? '-', size: 7.5),
                    cell(_methodLabel(t.paymentMethod), size: 7.5),
                    cell(
                      t.description.isEmpty ? '-' : t.description,
                      size: 7.5,
                    ),
                    cell(
                      t.type == 'income' ? formatRp(parseCents(t.amount)) : '',
                      right: true,
                      size: 7.5,
                    ),
                    cell(
                      t.type == 'expense' ? formatRp(parseCents(t.amount)) : '',
                      right: true,
                      size: 7.5,
                    ),
                  ],
                ),
            ],
          ),
        ],
      ],
    ),
  );

  return pdf.save();
}

// ---------------------------------------------------------------------------
// XLSX
// ---------------------------------------------------------------------------

final _rpFormat = NumFormat.custom(formatCode: '"Rp"#,##0;-"Rp"#,##0');

CellValue _num(int cents) => cents % 100 == 0
    ? IntCellValue(cents ~/ 100)
    : DoubleCellValue(cents / 100);

Future<Uint8List> generateKasReportXlsx({
  required KasReportModel model,
  required List<BackendTransaction> transactions,
}) async {
  final excel = Excel.createExcel();
  final headerStyle = CellStyle(
    bold: true,
    backgroundColorHex: ExcelColor.fromHexString('#EFEFEF'),
  );
  final boldStyle = CellStyle(bold: true);

  void put(
    Sheet s,
    int col,
    int row,
    CellValue v, {
    CellStyle? style,
    NumFormat? fmt,
  }) {
    final c = s.cell(
      CellIndex.indexByColumnRow(columnIndex: col, rowIndex: row),
    );
    c.value = v;
    if (style != null) c.cellStyle = style;
    if (fmt != null) {
      c.cellStyle = (style ?? CellStyle()).copyWith(numberFormat: fmt);
    }
  }

  // ---- Sheet 1: Laporan Keuangan
  final s1 = excel['Laporan Keuangan'];
  excel.setDefaultSheet('Laporan Keuangan');
  var r = 0;
  for (final line in kasTitleLines(model)) {
    put(s1, 0, r, TextCellValue(line), style: boldStyle);
    r++;
  }
  r++;
  for (var c = 0; c < 5; c++) {
    put(
      s1,
      c,
      r,
      TextCellValue(const ['No', 'Uraian', 'Debet', 'Kredit', 'Saldo'][c]),
      style: headerStyle,
    );
  }
  r++;
  for (final row in model.statementRows()) {
    final bold = row.kind != KasRowKind.row;
    final st = bold ? boldStyle : null;
    if (row.no != null) put(s1, 0, r, IntCellValue(row.no!), style: st);
    put(s1, 1, r, TextCellValue(row.label), style: st);
    if (row.debit != null && row.debit != 0) {
      put(s1, 2, r, _num(row.debit!), style: st, fmt: _rpFormat);
    }
    if (row.credit != null && row.credit != 0) {
      put(s1, 3, r, _num(row.credit!), style: st, fmt: _rpFormat);
    }
    if (row.saldo != null) {
      put(s1, 4, r, _num(row.saldo!), style: st, fmt: _rpFormat);
    }
    r++;
  }
  s1.setColumnWidth(0, 6);
  s1.setColumnWidth(1, 62);
  s1.setColumnWidth(2, 18);
  s1.setColumnWidth(3, 18);
  s1.setColumnWidth(4, 20);

  // ---- Sheet 2: Rekap Kategori
  final s2 = excel['Rekap Kategori'];
  for (var c = 0; c < 5; c++) {
    put(
      s2,
      c,
      0,
      TextCellValue(
        const ['Kategori', 'Tunai', 'Transfer', 'Tanpa Metode', 'Total'][c],
      ),
      style: headerStyle,
    );
  }
  var r2 = 1;
  for (final cat in model.expenseCategories) {
    put(s2, 0, r2, TextCellValue(cat.category));
    put(s2, 1, r2, _num(cat.cash), fmt: _rpFormat);
    put(s2, 2, r2, _num(cat.transfer), fmt: _rpFormat);
    put(s2, 3, r2, _num(cat.other), fmt: _rpFormat);
    put(s2, 4, r2, _num(cat.total), fmt: _rpFormat);
    r2++;
  }
  put(s2, 0, r2, TextCellValue('Grand Total'), style: boldStyle);
  put(s2, 1, r2, _num(model.expense.cash), style: boldStyle, fmt: _rpFormat);
  put(
    s2,
    2,
    r2,
    _num(model.expense.transfer),
    style: boldStyle,
    fmt: _rpFormat,
  );
  put(s2, 3, r2, _num(model.expense.other), style: boldStyle, fmt: _rpFormat);
  put(s2, 4, r2, _num(model.expense.total), style: boldStyle, fmt: _rpFormat);
  s2.setColumnWidth(0, 34);
  for (var c = 1; c < 5; c++) {
    s2.setColumnWidth(c, 18);
  }

  // ---- Sheet 3: Detail Transaksi
  final s3 = excel['Detail Transaksi'];
  const h3 = [
    'Tanggal',
    'Referensi',
    'Jenis',
    'Kategori',
    'Keterangan',
    'Metode',
    'Debet',
    'Kredit',
    'Status',
  ];
  for (var c = 0; c < h3.length; c++) {
    put(s3, c, 0, TextCellValue(h3[c]), style: headerStyle);
  }
  var r3 = 1;
  var sumDebit = 0;
  var sumCredit = 0;
  for (final t in transactions.where((t) => t.status == 'posted')) {
    final cents = parseCents(t.amount);
    put(s3, 0, r3, TextCellValue(_dateLabel(t.occurredAt)));
    put(s3, 1, r3, TextCellValue(t.paymentNo ?? ''));
    put(s3, 2, r3, TextCellValue(t.type == 'income' ? 'Masuk' : 'Keluar'));
    put(s3, 3, r3, TextCellValue(t.categoryName ?? ''));
    put(
      s3,
      4,
      r3,
      TextCellValue(
        t.description.isEmpty || t.description.trim().isEmpty
            ? '-'
            : t.description,
      ),
    );
    put(s3, 5, r3, TextCellValue(_methodLabel(t.paymentMethod)));
    if (t.type == 'income') {
      put(s3, 6, r3, _num(cents), fmt: _rpFormat);
      sumDebit += cents;
    } else {
      put(s3, 7, r3, _num(cents), fmt: _rpFormat);
      sumCredit += cents;
    }
    put(s3, 8, r3, TextCellValue(t.status));
    r3++;
  }
  put(s3, 4, r3, TextCellValue('TOTAL'), style: boldStyle);
  put(s3, 6, r3, _num(sumDebit), style: boldStyle, fmt: _rpFormat);
  put(s3, 7, r3, _num(sumCredit), style: boldStyle, fmt: _rpFormat);
  const widths = [13.0, 22.0, 10.0, 24.0, 46.0, 12.0, 18.0, 18.0, 10.0];
  for (var c = 0; c < widths.length; c++) {
    s3.setColumnWidth(c, widths[c]);
  }

  excel.delete('Sheet1');
  final bytes = excel.encode();
  if (bytes == null) {
    throw StateError('Gagal menyusun berkas XLSX');
  }
  return Uint8List.fromList(bytes);
}

String getKasReportXlsxFileName(KasReportModel model) =>
    '${model.fileBase}.xlsx';
String getKasReportPdfFileName(KasReportModel model) => '${model.fileBase}.pdf';
