/// Model agregasi Laporan KAS (Android) — kontrak data & rumus identik dengan
/// `frontend/src/features/kas/kasReportModel.ts`.
///
/// ATURAN: Debet = penerimaan (income), Kredit = pengeluaran (expense); hanya
/// transaksi `posted` dihitung; metode CASH/TRANSFER/lainnya (tanpa metode);
/// saldo akhir = saldo awal + penerimaan - pengeluaran; semua dalam sen (int),
/// tanpa floating point. Tarik tunai tidak dimodelkan backend, jadi tidak
/// ditampilkan. Saldo awal tidak dapat dipecah per metode sehingga laporan
/// menampilkan mutasi kas tunai & mutasi rekening.
library;

import 'package:social_finance/features/kas/data/kas_api_models.dart';

class KasReportIdentity {
  final String orgName;
  final String? rtLabel;
  final String? rwLabel;
  final String? address;

  const KasReportIdentity({
    required this.orgName,
    this.rtLabel,
    this.rwLabel,
    this.address,
  });
}

class BucketAmounts {
  int cash = 0;
  int transfer = 0;
  int other = 0;
  int get total => cash + transfer + other;

  void add(String bucket, int cents) {
    switch (bucket) {
      case 'cash':
        cash += cents;
      case 'transfer':
        transfer += cents;
      default:
        other += cents;
    }
  }

  int of(String bucket) => bucket == 'cash'
      ? cash
      : bucket == 'transfer'
      ? transfer
      : other;
}

class KasCategoryRow {
  final String category;
  final int cash;
  final int transfer;
  final int other;
  const KasCategoryRow(this.category, this.cash, this.transfer, this.other);
  int get total => cash + transfer + other;
}

enum KasRowKind { row, subtotal, total, grand }

class KasStatementRow {
  final int? no;
  final String label;
  final int? debit;
  final int? credit;
  final int? saldo;
  final KasRowKind kind;
  const KasStatementRow({
    this.no,
    required this.label,
    this.debit,
    this.credit,
    this.saldo,
    required this.kind,
  });
}

class KasReportModel {
  final KasReportIdentity identity;
  final String? period;
  final DateTime generatedAt;
  final int openingCents;
  final BucketAmounts income;
  final BucketAmounts expense;
  final Map<String, List<KasCategoryRow>> incomeCategories;
  final List<KasCategoryRow> expenseCategories;
  final int closingCents;
  final int? apiClosingCents;
  final int transactionCount;

  KasReportModel({
    required this.identity,
    required this.period,
    required this.generatedAt,
    required this.openingCents,
    required this.income,
    required this.expense,
    required this.incomeCategories,
    required this.expenseCategories,
    required this.closingCents,
    required this.apiClosingCents,
    required this.transactionCount,
  });

  String get periodLabel => kasPeriodLabel(period);
  int get totalIncomeCents => income.total;
  int get totalExpenseCents => expense.total;
  int get cashMovementCents => income.cash - expense.cash;
  int get transferMovementCents => income.transfer - expense.transfer;

  /// null bila saldo akhir server tidak tersedia.
  bool? get reconciles =>
      apiClosingCents == null ? null : apiClosingCents == closingCents;

  bool get expenseHasOther => expense.other != 0;

  String get fileBase => 'Laporan-KAS-${period ?? 'semua-periode'}';

  /// Baris tabel utama (No, Uraian, Debet, Kredit, Saldo) — identik dengan
  /// Web [buildStatementRows].
  ///
  /// PERHITUNGAN SALDO BERJALAN:
  ///   saldo = saldo_sebelumnya + debet - kredit
  /// Baris ringkasan (subtotal / total) tidak mengubah saldo berjalan — kolom
  /// Saldo tetap dikosongkan agar tidak tertukar dengan nilai mutasi atau total.
  List<KasStatementRow> statementRows() {
    final rows = <KasStatementRow>[];
    var n = 0;
    int runningBalance = 0;

    // Saldo awal
    rows.add(
      KasStatementRow(
        no: ++n,
        label: 'Saldo awal periode',
        debit: openingCents,
        saldo: openingCents,
        kind: KasRowKind.row,
      ),
    );
    runningBalance = openingCents;

    void section(String bucket, String title, String subtotalLabel) {
      final list = incomeCategories[bucket] ?? const <KasCategoryRow>[];
      final bucketAmount = income.of(bucket);
      if (list.isEmpty && bucketAmount == 0) return;
      for (final r in list) {
        final v = bucket == 'cash'
            ? r.cash
            : bucket == 'transfer'
            ? r.transfer
            : r.other;
        if (v != 0) {
          runningBalance += v;
          // Kosongkan saldo: baris agregat kategori tidak dapat mewakili
          // titik kronologis yang valid (kategori diurutkan alfabetis,
          // bukan tanggal; satu kategori bisa mencakup transaksi dari banyak tanggal).
          rows.add(
            KasStatementRow(
              no: ++n,
              label: '${r.category} ($title)',
              debit: v,
              kind: KasRowKind.row,
            ),
          );
        } else {
          rows.add(
            KasStatementRow(
              no: ++n,
              label: '${r.category} ($title)',
              kind: KasRowKind.row,
            ),
          );
        }
      }
      // Baris subtotal tidak mengubah saldo berjalan — kolom Saldo dikosongkan
      rows.add(
        KasStatementRow(
          label: subtotalLabel,
          kind: KasRowKind.subtotal,
        ),
      );
    }

    section('cash', 'tunai', 'Total penerimaan tunai');
    section('transfer', 'transfer', 'Total penerimaan transfer');
    section('other', 'tanpa metode', 'Total penerimaan tanpa metode');

    // Baris agregasi Tunai + Transfer — Debet = total penerimaan (cash + transfer)
    rows.add(
      KasStatementRow(
        label: 'Total penerimaan Tunai + Transfer',
        debit: totalIncomeCents,
        kind: KasRowKind.total,
      ),
    );

    // Baris pengeluaran individual
    void addExpenseRow(String label, int credit) {
      if (credit == 0) return;
      runningBalance -= credit;
      // Kosongkan saldo: baris agregat pengeluaran tidak dapat mewakili
      // titik kronologis yang valid (sama dengan penerimaan agregat).
      rows.add(
        KasStatementRow(
          no: ++n,
          label: label,
          credit: credit,
          kind: KasRowKind.row,
        ),
      );
    }
    addExpenseRow('Pengeluaran tunai', expense.cash);
    addExpenseRow('Pengeluaran transfer', expense.transfer);
    if (expense.other != 0) {
      runningBalance -= expense.other;
      rows.add(
        KasStatementRow(
          no: ++n,
          label: 'Pengeluaran tanpa metode',
          credit: expense.other,
          kind: KasRowKind.row,
        ),
      );
    }

    // Baris agregasi pengeluaran (tidak mengubah saldo berjalan) — Kredit = total pengeluaran (cash + transfer)
    rows.add(
      KasStatementRow(
        label: 'Total pengeluaran Tunai + Transfer',
        credit: totalExpenseCents,
        kind: KasRowKind.total,
      ),
    );

    // Baris mutasi (tidak mengubah saldo berjalan — bukan transaksi)
    // Mutasi kas tunai: neto penerimaan tunai. Jika positif → Debet, jika negatif → Kredit.
    final cashMovementNeto = income.cash - expense.cash;
    if (cashMovementNeto > 0) {
      rows.add(
        KasStatementRow(
          label: 'Mutasi kas tunai (penerimaan tunai - pengeluaran tunai)',
          debit: cashMovementNeto,
          kind: KasRowKind.subtotal,
        ),
      );
    } else if (cashMovementNeto < 0) {
      rows.add(
        KasStatementRow(
          label: 'Mutasi kas tunai (penerimaan tunai - pengeluaran tunai)',
          credit: cashMovementNeto.abs(),
          kind: KasRowKind.subtotal,
        ),
      );
    } else {
      rows.add(
        KasStatementRow(
          label: 'Mutasi kas tunai (penerimaan tunai - pengeluaran tunai)',
          kind: KasRowKind.subtotal,
        ),
      );
    }
    // Mutasi rekening: neto penerimaan transfer. Jika positif → Debet, jika negatif → Kredit.
    final transferMovementNeto = income.transfer - expense.transfer;
    if (transferMovementNeto > 0) {
      rows.add(
        KasStatementRow(
          label: 'Mutasi rekening (penerimaan transfer - pengeluaran transfer)',
          debit: transferMovementNeto,
          kind: KasRowKind.subtotal,
        ),
      );
    } else if (transferMovementNeto < 0) {
      rows.add(
        KasStatementRow(
          label: 'Mutasi rekening (penerimaan transfer - pengeluaran transfer)',
          credit: transferMovementNeto.abs(),
          kind: KasRowKind.subtotal,
        ),
      );
    } else {
      rows.add(
        KasStatementRow(
          label: 'Mutasi rekening (penerimaan transfer - pengeluaran transfer)',
          kind: KasRowKind.subtotal,
        ),
      );
    }

    // Saldo akhir — satu-satunya baris yang menampilkan nilai akhir sebenarnya
    rows.add(
      KasStatementRow(
        label: 'Saldo akhir (saldo awal + penerimaan - pengeluaran)',
        saldo: runningBalance,
        kind: KasRowKind.grand,
      ),
    );
    return rows;
  }
}

const _months = [
  'Januari',
  'Februari',
  'Maret',
  'April',
  'Mei',
  'Juni',
  'Juli',
  'Agustus',
  'September',
  'Oktober',
  'November',
  'Desember',
];

String kasPeriodLabel(String? period) {
  if (period == null || period.isEmpty) return 'Semua Periode';
  final parts = period.split('-');
  if (parts.length < 2) return period;
  final m = int.tryParse(parts[1]);
  if (m == null || m < 1 || m > 12) return period;
  return '${_months[m - 1]} ${parts[0]}';
}

/// Parse string NUMERIC ("1234.50", "-5") ke sen tanpa floating point.
/// Melempar [FormatException] bila format tidak valid (tidak menelan error).
int parseCents(String? value) {
  final s = (value ?? '').trim();
  if (s.isEmpty) return 0;
  final m = RegExp(r'^(-?)(\d+)(?:\.(\d{1,2})\d*)?$').firstMatch(s);
  if (m == null) throw FormatException('Nilai nominal tidak valid: "$s"');
  final whole = int.parse(m.group(2)!);
  final frac = int.parse(
    (m.group(3) ?? '').padRight(2, '0').isEmpty
        ? '0'
        : (m.group(3) ?? '').padRight(2, '0'),
  );
  final cents = whole * 100 + frac;
  return m.group(1) == '-' ? -cents : cents;
}

String kasBucketOf(String? method) {
  final v = (method ?? '').trim().toUpperCase();
  if (v == 'CASH') return 'cash';
  if (v == 'TRANSFER') return 'transfer';
  return 'other';
}

String _group(int whole) {
  final s = whole.toString();
  final buf = StringBuffer();
  for (var i = 0; i < s.length; i++) {
    if (i > 0 && (s.length - i) % 3 == 0) buf.write('.');
    buf.write(s[i]);
  }
  return buf.toString();
}

/// "Rp1.023.000" (tanpa spasi, sesuai dokumen referensi).
String formatRp(int cents) {
  final neg = cents < 0;
  final abs = cents.abs();
  final frac = abs % 100;
  return '${neg ? '-' : ''}Rp${_group(abs ~/ 100)}'
      '${frac != 0 ? ',${frac.toString().padLeft(2, '0')}' : ''}';
}

/// Angka tanpa simbol; kosong untuk nol (tabel rekap kategori).
String formatNum(int cents) =>
    cents == 0 ? '' : formatRp(cents).replaceFirst('Rp', '');

Map<String, BucketAmounts> _emptyMap() => {};

/// Sort transaksi secara kronologis ascending sebelum agregasi.
/// Urutan memastikan saldo berjalan (running balance) pada baris statement
/// reliabel untuk transaksi yang berasal dari tanggal/waktu berbeda.
///
/// Tie-breaker deterministik: [BackendTransaction.id].
List<BackendTransaction> _sortTransactions(List<BackendTransaction> txs) {
  return [...txs]
    ..sort((a, b) {
      final atA = a.occurredAt ?? '';
      final atB = b.occurredAt ?? '';
      final cmp = atA.compareTo(atB);
      if (cmp != 0) return cmp;
      return a.id.compareTo(b.id);
    });
}

KasReportModel buildKasReport({
  required List<BackendTransaction> transactions,
  required String openingBalance,
  String? apiClosingBalance,
  required String? period,
  required KasReportIdentity identity,
  DateTime? generatedAt,
}) {
  // Sort kronologis ascending sebelum agregasi → saldo berjalan reliabel
  final sorted = _sortTransactions(transactions);

  final income = BucketAmounts();
  final expense = BucketAmounts();
  final incByCat = <String, Map<String, BucketAmounts>>{
    'cash': _emptyMap(),
    'transfer': _emptyMap(),
    'other': _emptyMap(),
  };
  final expByCat = <String, BucketAmounts>{};
  var count = 0;

  for (final t in sorted) {
    if (t.status != 'posted') continue;
    final cents = parseCents(t.amount);
    final bucket = kasBucketOf(t.paymentMethod);
    final name = (t.categoryName ?? '').trim();
    final cat = name.isEmpty ? 'Lainnya' : name;
    count++;
    if (t.type == 'income') {
      income.add(bucket, cents);
      incByCat[bucket]!.putIfAbsent(cat, BucketAmounts.new).add(bucket, cents);
    } else {
      expense.add(bucket, cents);
      expByCat.putIfAbsent(cat, BucketAmounts.new).add(bucket, cents);
    }
  }

  List<KasCategoryRow> rows(Map<String, BucketAmounts> m) {
    final list =
        m.entries
            .map(
              (e) => KasCategoryRow(
                e.key,
                e.value.cash,
                e.value.transfer,
                e.value.other,
              ),
            )
            .toList()
          ..sort(
            (a, b) =>
                a.category.toLowerCase().compareTo(b.category.toLowerCase()),
          );
    return list;
  }

  final opening = parseCents(openingBalance);
  return KasReportModel(
    identity: identity,
    period: period,
    generatedAt: generatedAt ?? DateTime.now(),
    openingCents: opening,
    income: income,
    expense: expense,
    incomeCategories: {
      'cash': rows(incByCat['cash']!),
      'transfer': rows(incByCat['transfer']!),
      'other': rows(incByCat['other']!),
    },
    expenseCategories: rows(expByCat),
    closingCents: opening + income.total - expense.total,
    apiClosingCents: apiClosingBalance == null
        ? null
        : parseCents(apiClosingBalance),
    transactionCount: count,
  );
}
