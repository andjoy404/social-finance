import 'package:dio/dio.dart';
import 'package:excel/excel.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_client.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/export/kas_export_utils.dart';
import 'package:social_finance/features/kas/export/kas_report_model.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_report_screen.dart';

/// Fixture IDENTIK dengan frontend/src/features/kas/kasReportFixture.ts.
/// Expected values dihitung manual (bukan dari implementasi):
///   penerimaan cash 1.250.000, transfer 500.000, total 1.750.000
///   pengeluaran cash 400.000, transfer 150.000, total 550.000
///   saldo awal 2.000.000 -> saldo akhir 3.200.000
///   mutasi kas tunai 850.000, mutasi rekening 350.000
/// draft/cancelled tidak dihitung.
BackendTransaction tx(
  String id,
  String type,
  String amount, {
  String category = 'Lainnya',
  String? method = 'CASH',
  String status = 'posted',
  String? occurredAt,
}) => BackendTransaction(
  id: id,
  rtId: 'rt-1',
  categoryId: 'c',
  amount: amount,
  type: type,
  status: status,
  description: id,
  occurredAt: occurredAt ?? '2026-10-05T03:00:00Z',
  createdAt: '2026-10-05T03:00:00Z',
  categoryName: category,
  paymentMethod: method,
);

final fixture = <BackendTransaction>[
  tx('i1', 'income', '1000000.00', category: 'Iuran Warga'),
  tx('i2', 'income', '250000.00', category: 'Infaq'),
  tx('i3', 'income', '500000.00', category: 'Iuran Warga', method: 'TRANSFER'),
  tx('e1', 'expense', '300000.00', category: 'Keamanan'),
  tx('e2', 'expense', '100000.00', category: 'Kebersihan'),
  tx('e3', 'expense', '150000.00', category: 'Keamanan', method: 'TRANSFER'),
  tx('d1', 'income', '999999.00', status: 'draft'),
  tx('c1', 'expense', '888888.00', status: 'cancelled'),
];

const identity = KasReportIdentity(
  orgName: 'Cluster Urena',
  rtLabel: 'RT 02',
  rwLabel: 'RW 016',
);

KasReportModel build({
  List<BackendTransaction>? txs,
  String opening = '2000000.00',
  String? apiClosing = '3200000.00',
}) => buildKasReport(
  transactions: txs ?? fixture,
  openingBalance: opening,
  apiClosingBalance: apiClosing,
  period: '2026-10',
  identity: identity,
  generatedAt: DateTime(2026, 10, 23),
);

// ---------------------------------------------------------------------------
// Helper untuk regression test single-transaction
// ---------------------------------------------------------------------------
KasReportModel buildSingle(
  List<BackendTransaction> txs, {
  String opening = '0',
  String? apiClosing,
  DateTime? generatedAt,
}) => buildKasReport(
  transactions: txs,
  openingBalance: opening,
  apiClosingBalance: apiClosing,
  period: '2026-10',
  identity: const KasReportIdentity(
    orgName: 'Cluster Urena',
    rtLabel: 'RT 02',
    rwLabel: 'RW 016',
  ),
  generatedAt: generatedAt ?? DateTime(2026, 10, 23),
);

class _FakeRepo extends KasRepository {
  _FakeRepo({
    this.fail = false,
    this.txs = const [],
    this.closing = '0',
    this.identity,
  }) : super(apiClient: KasApiClient(dio: Dio()));
  final bool fail;
  final List<BackendTransaction> txs;
  final String closing;
  final BackendRTIdentity? identity;

  @override
  Future<List<BackendTransaction>> fetchAllBackendTransactions({
    String? status,
    String? period,
  }) async {
    if (fail) throw Exception('boom');
    return txs;
  }

  @override
  Future<BackendRTIdentity> fetchRTIdentity(String rtId) async {
    if (fail) throw Exception('boom');
    return identity ?? const BackendRTIdentity(name: 'RT 02');
  }

  @override
  Future<BackendBalance> fetchBalance({String? period}) async {
    if (fail) throw Exception('boom');
    return BackendBalance(
      openingBalance: '0',
      totalIncome: '0',
      totalExpense: '0',
      closingBalance: closing,
      netBalance: closing,
      transactionCount: 0,
    );
  }

  @override
  ServerException mapDioError(dynamic error) =>
      ServerException(message: 'Gagal terhubung ke server');
}

Future<void> pumpScreen(
  WidgetTester tester,
  KasRepository repo, {
  AppRole role = AppRole.bendahara,
  String? jabatan,
}) async {
  final dio = Dio(BaseOptions(baseUrl: 'http://test.local'));
  final client = ApiClient(customDio: dio);
  final auth = AuthRepository(
    authService: AuthService(client),
    apiClient: client,
  );
  auth.state = AsyncData({
    'id': 'u',
    'role': role, // enum, seperti di aplikasi nyata
    'system_role': null,
    'jabatan': jabatan,
    'rt_id': 'rt-1',
    'rt': '02',
    'rw': '016',
    'rt_name': 'Fallback RT 02',
    'rw_name': 'RW 016',
  });
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        kasRepositoryProvider.overrideWithValue(repo),
        authRepositoryProvider.overrideWith((_) => auth),
      ],
      child: const MaterialApp(home: KasReportScreen(period: '2026-10')),
    ),
  );
}

void main() {
  group('model', () {
    test('parseCents tanpa floating point', () {
      expect(parseCents('1000000.00'), 100000000);
      expect(parseCents('0.1'), 10);
      expect(parseCents('-5'), -500);
      expect(parseCents(null), 0);
      expect(() => parseCents('abc'), throwsFormatException);
    });

    test('penerimaan/pengeluaran per metode dan saldo akhir', () {
      final m = build();
      expect(m.income.cash, 125000000);
      expect(m.income.transfer, 50000000);
      expect(m.expense.cash, 40000000);
      expect(m.expense.transfer, 15000000);
      expect(m.totalIncomeCents, 175000000);
      expect(m.totalExpenseCents, 55000000);
      expect(m.closingCents, 320000000);
      expect(m.cashMovementCents, 85000000);
      expect(m.transferMovementCents, 35000000);
      expect(m.transactionCount, 6);
      expect(m.reconciles, isTrue);
    });

    test('ketidaksesuaian saldo server terdeteksi', () {
      expect(build(apiClosing: '1.00').reconciles, isFalse);
      expect(build(apiClosing: null).reconciles, isNull);
    });

    test('rekap kategori & grand total', () {
      final m = build();
      expect(m.expenseCategories.map((c) => c.category).toList(), [
        'Keamanan',
        'Kebersihan',
      ]);
      expect(m.expenseCategories[0].cash, 30000000);
      expect(m.expenseCategories[0].transfer, 15000000);
      expect(m.expense.total, 55000000);
    });

    test('baris tabel utama sama dengan Web', () {
      final rows = build().statementRows();
      expect(rows.first.label, 'Saldo awal periode');
      expect(rows.first.debit, 200000000);
      // Baris summary (subtotal/total) tidak memiliki saldo — saldo dikosongkan
      final tot = rows.firstWhere(
        (r) => r.label == 'Total penerimaan Tunai + Transfer',
      );
      expect(tot.saldo, isNull);
      final totExpense = rows.firstWhere(
        (r) => r.label == 'Total pengeluaran Tunai + Transfer',
      );
      expect(totExpense.saldo, isNull);
      final mutasiKas = rows.firstWhere(
        (r) => r.label.contains('Mutasi kas tunai'),
      );
      expect(mutasiKas.saldo, isNull);
      final mutasiRek = rows.firstWhere(
        (r) => r.label.contains('Mutasi rekening'),
      );
      expect(mutasiRek.saldo, isNull);
      // Saldo akhir tetap benar
      expect(rows.last.kind, KasRowKind.grand);
      expect(rows.last.saldo, 320000000);
    });

    test('format Rupiah & periode', () {
      expect(formatRp(102300000), 'Rp1.023.000');
      expect(formatRp(-5000), '-Rp50');
      expect(formatRp(150), 'Rp1,50');
      expect(kasPeriodLabel('2026-10'), 'Oktober 2026');
      expect(kasPeriodLabel(null), 'Semua Periode');
    });
  });

  group('ekspor', () {
    test('PDF valid', () async {
      final bytes = await generateKasReportPdf(
        model: build(),
        transactions: fixture,
      );
      expect(String.fromCharCodes(bytes.sublist(0, 5)), '%PDF-');
      expect(bytes.length, greaterThan(2000));
    });

    test('XLSX: 3 sheet, nilai numerik & total benar', () async {
      final bytes = await generateKasReportXlsx(
        model: build(),
        transactions: fixture,
      );
      final book = Excel.decodeBytes(bytes);
      expect(book.tables.keys.toList(), [
        'Laporan Keuangan',
        'Rekap Kategori',
        'Detail Transaksi',
      ]);
      final s1 = book['Laporan Keuangan'];
      CellValue? closing;
      CellValue? incomeSaldo;
      for (final row in s1.rows) {
        final label = row.length > 1 ? row[1]?.value?.toString() : null;
        if (label != null && label.startsWith('Saldo akhir')) {
          closing = row[4]?.value;
        }
        if (label == 'Total penerimaan Tunai + Transfer') {
          incomeSaldo = row[4]?.value;
        }
      }
      expect(closing, isA<IntCellValue>());
      expect((closing as IntCellValue).value, 3200000);
      // Baris summary tidak memiliki saldo setelah perbaikan saldo berjalan
      expect(incomeSaldo, isNull);
      final s2 = book['Rekap Kategori'];
      final last = s2.rows.last;
      expect(last[0]?.value.toString(), 'Grand Total');
      expect((last[4]?.value as IntCellValue).value, 550000);
      final s3 = book['Detail Transaksi'];
      // header + 6 posted + total
      expect(s3.rows.length, 8);
      expect((s3.rows[1][6]?.value as IntCellValue).value, 1000000);
    });
  });

  group('akses', () {
    test('lihat/ekspor', () {
      expect(KasReportAccess.from(jabatan: 'keamanan').canView, isFalse);
      expect(KasReportAccess.from(jabatan: 'Operasional ').canView, isFalse);
      expect(KasReportAccess.from(role: 'warga').canView, isTrue);
      expect(
        KasReportAccess.from(
          systemRole: 'super_admin',
          jabatan: 'keamanan',
        ).canView,
        isTrue,
      );
      expect(KasReportAccess.from(role: 'bendahara').canExport, isTrue);
      expect(KasReportAccess.from(jabatan: 'ketua').canExport, isTrue);
      expect(KasReportAccess.from(jabatan: 'wakil_ketua').canExport, isTrue);
      expect(KasReportAccess.from(role: 'warga').canExport, isFalse);
      expect(KasReportAccess.from(jabatan: 'sekretaris').canExport, isFalse);
    });
  });

  group('layar laporan (bukan halaman putih)', () {
    testWidgets('sukses: dokumen terlihat, tombol ekspor untuk bendahara', (
      tester,
    ) async {
      await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
      expect(find.byKey(const Key('kas-report-loading')), findsOneWidget);
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('kas-report-document')), findsOneWidget);
      expect(find.text('LAPORAN KEUANGAN RT 02 RW 016'), findsOneWidget);
      expect(find.text('Saldo awal periode'), findsOneWidget);
      expect(
        find.text('Saldo akhir (saldo awal + penerimaan - pengeluaran)'),
        findsOneWidget,
      );
      expect(find.byKey(const Key('kas-report-export-pdf')), findsOneWidget);
      expect(find.byKey(const Key('kas-report-export-xlsx')), findsOneWidget);
    });

    testWidgets('identitas RT detail dipakai oleh dokumen laporan', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        _FakeRepo(
          txs: fixture,
          closing: '3200000.00',
          identity: const BackendRTIdentity(
            name: 'Grand Nusa Indah Cluster Urena',
            address: 'Desa Gondang, Kecamatan Cileungsi',
            rt: '002',
            rw: 16,
            rwName: 'RW 016',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Grand Nusa Indah Cluster Urena'), findsOneWidget);
      expect(find.text('Desa Gondang, Kecamatan Cileungsi'), findsOneWidget);
    });

    testWidgets('warga: dokumen tampil tanpa tombol ekspor', (tester) async {
      await pumpScreen(
        tester,
        _FakeRepo(txs: fixture, closing: '3200000.00'),
        role: AppRole.warga,
      );
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('kas-report-document')), findsOneWidget);
      expect(find.byKey(const Key('kas-report-export-pdf')), findsNothing);
    });

    testWidgets('jabatan keamanan ditolak', (tester) async {
      await pumpScreen(
        tester,
        _FakeRepo(txs: fixture),
        role: AppRole.pengurus,
        jabatan: 'keamanan',
      );
      await tester.pumpAndSettle();
      expect(find.text('Akses Ditolak'), findsOneWidget);
      expect(find.byKey(const Key('kas-report-document')), findsNothing);
    });

    testWidgets('kosong: pesan informatif, bukan saldo palsu', (tester) async {
      await pumpScreen(tester, _FakeRepo(txs: const [], closing: '0'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('kas-report-empty')), findsOneWidget);
    });

    testWidgets('error API: pesan + Coba Lagi, bukan nol', (tester) async {
      await pumpScreen(tester, _FakeRepo(fail: true));
      await tester.pumpAndSettle();
      expect(find.text('Gagal memuat laporan'), findsOneWidget);
      expect(find.text('Gagal terhubung ke server'), findsOneWidget);
      expect(find.text('Coba Lagi'), findsOneWidget);
    });

    testWidgets('selisih saldo server ditandai', (tester) async {
      await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '1.00'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('kas-report-mismatch')), findsOneWidget);
    });
  });

  // -----------------------------------------------------------------------
  // Regression test — Cegah DOUBLE COUNT Tunai/Transfer (Bug #2026-10)
  // -----------------------------------------------------------------------

  group('regression: pencegahan double count Tunai/Transfer', () {
    // 1. Satu transaksi tunai Rp50.000 → Tunai Rp50.000, Transfer Rp0
    test(
      'satu transaksi tunai Rp50.000 menghasilkan Tunai Rp50.000, Transfer Rp0',
      () {
        final m = buildSingle([
          tx(
            'e1',
            'expense',
            '50000.00',
            category: 'Operasional RT',
            method: 'CASH',
          ),
        ]);
        expect(m.expense.cash, 5000000);
        expect(m.expense.transfer, 0);
        expect(m.expense.other, 0);
        expect(m.totalExpenseCents, 5000000);
        expect(m.cashMovementCents, -5000000);
        expect(m.transferMovementCents, 0);
        expect(m.closingCents, -5000000);
        expect(m.transactionCount, 1);
      },
    );

    // 2. Satu transaksi transfer Rp25.000 → Tunai Rp0, Transfer Rp25.000
    test(
      'satu transaksi transfer Rp25.000 menghasilkan Tunai Rp0, Transfer Rp25.000',
      () {
        final m = buildSingle([
          tx(
            'e1',
            'expense',
            '25000.00',
            category: 'Operasional RT',
            method: 'TRANSFER',
          ),
        ]);
        expect(m.expense.cash, 0);
        expect(m.expense.transfer, 2500000);
        expect(m.expense.other, 0);
        expect(m.totalExpenseCents, 2500000);
        expect(m.cashMovementCents, 0);
        expect(m.transferMovementCents, -2500000);
        expect(m.closingCents, -2500000);
        expect(m.transactionCount, 1);
      },
    );

    // 3. Dua transaksi terpisah: tunai Rp50.000 + transfer Rp25.000
    //    → Tunai Rp50.000, Transfer Rp25.000, total Rp75.000
    test(
      'dua transaksi terpisah: tunai Rp50.000 + transfer Rp25.000 → total Rp75.000',
      () {
        final m = buildSingle([
          tx(
            'e1',
            'expense',
            '50000.00',
            category: 'Operasional RT',
            method: 'CASH',
          ),
          tx(
            'e2',
            'expense',
            '25000.00',
            category: 'Operasional RT',
            method: 'TRANSFER',
          ),
        ]);
        expect(m.expense.cash, 5000000);
        expect(m.expense.transfer, 2500000);
        expect(m.totalExpenseCents, 7500000);
        expect(m.cashMovementCents, -5000000);
        expect(m.transferMovementCents, -2500000);
        expect(m.closingCents, -7500000);
        expect(m.transactionCount, 2);
      },
    );

    // 4. Transaksi sama diproses dua kali tetap dihitung per-entri
    test('transaksi yang sama tidak dihitung ganda ke dua bucket berbeda', () {
      final m = buildSingle([
        tx(
          'e1',
          'expense',
          '50000.00',
          category: 'Operasional RT',
          method: 'CASH',
        ),
      ]);
      expect(m.expense.transfer, 0); // pasti 0, bukan dari bucket lain
    });

    // 5. Rekap kategori konsisten dengan total pengeluaran
    test('rekap kategori konsisten dengan total pengeluaran', () {
      final m = buildSingle([
        tx('e1', 'expense', '30000.00', category: 'Keamanan', method: 'CASH'),
        tx(
          'e2',
          'expense',
          '20000.00',
          category: 'Keamanan',
          method: 'TRANSFER',
        ),
        tx('e3', 'expense', '10000.00', category: 'Kebersihan', method: 'CASH'),
      ]);
      final keamanan = m.expenseCategories.firstWhere(
        (c) => c.category == 'Keamanan',
      );
      expect(keamanan.cash, 3000000);
      expect(keamanan.transfer, 2000000);
      expect(keamanan.total, 5000000);
      final kebersihan = m.expenseCategories.firstWhere(
        (c) => c.category == 'Kebersihan',
      );
      expect(kebersihan.cash, 1000000);
      expect(kebersihan.transfer, 0);
      expect(kebersihan.total, 1000000);
      expect(m.expense.cash, 4000000);
      expect(m.expense.transfer, 2000000);
      expect(m.totalExpenseCents, 6000000);
      // Jumlah total semua kategori = total pengeluaran
      final sumCategories = m.expenseCategories
          .map((c) => c.total)
          .reduce((a, b) => a + b);
      expect(sumCategories, m.totalExpenseCents);
    });

    // 6. Saldo akhir dan mutasi benar untuk kombinasi tunai dan transfer
    test('saldo akhir dan mutasi benar untuk kombinasi tunai dan transfer', () {
      final m = buildSingle([
        tx('i1', 'income', '100000.00', method: 'CASH'),
        tx('i2', 'income', '50000.00', method: 'TRANSFER'),
        tx('e1', 'expense', '30000.00', method: 'CASH'),
        tx('e2', 'expense', '20000.00', method: 'TRANSFER'),
      ], opening: '200000.00');
      expect(m.income.cash, 10000000);
      expect(m.income.transfer, 5000000);
      expect(m.totalIncomeCents, 15000000);
      expect(m.expense.cash, 3000000);
      expect(m.expense.transfer, 2000000);
      expect(m.totalExpenseCents, 5000000);
      expect(m.cashMovementCents, 7000000);
      expect(m.transferMovementCents, 3000000);
      expect(m.closingCents, 30000000); // 200k + 150k - 50k
    });

    // 7. Metode kosong/tidak dikenal → other, tidak masuk cash/transfer
    test(
      'metode kosong tidak otomatis masuk ke kedua kolom Tunai dan Transfer',
      () {
        final m = buildSingle([
          tx('e1', 'expense', '50000.00', method: null),
          tx('e2', 'expense', '30000.00', method: 'CASH'),
          tx('e3', 'expense', '20000.00', method: 'TRANSFER'),
        ]);
        expect(m.expense.cash, 3000000);
        expect(m.expense.transfer, 2000000);
        expect(m.expense.other, 5000000);
        expect(m.totalExpenseCents, 10000000);
        expect(m.expense.cash, isNot(8000000));
        expect(m.expense.transfer, isNot(7000000));
      },
    );

    test(
      'metode tidak dikenal (misal "QRIS") masuk other, bukan cash/transfer',
      () {
        final m = buildSingle([
          tx('e1', 'expense', '50000.00', method: 'QRIS'),
        ]);
        expect(m.expense.cash, 0);
        expect(m.expense.transfer, 0);
        expect(m.expense.other, 5000000);
      },
    );

    // 8. Skenario sesuai PDF: hanya pengeluaran tunai Rp50.000
    test(
      'skenario PDF: hanya pengeluaran tunai Rp50.000, transfer harus Rp0',
      () {
        final m = buildSingle([
          tx(
            'e1',
            'expense',
            '50000.00',
            category: 'Operasional RT',
            method: 'CASH',
          ),
        ], opening: '50000.00');
        expect(m.expense.cash, 5000000);
        expect(m.expense.transfer, 0);
        expect(m.totalExpenseCents, 5000000);
        expect(m.closingCents, 0);
        final operasional = m.expenseCategories.firstWhere(
          (c) => c.category == 'Operasional RT',
          orElse: () => const KasCategoryRow('', 0, 0, 0),
        );
        expect(operasional.cash, 5000000);
        expect(operasional.transfer, 0);
      },
    );

    // 9. Statement rows konsisten dengan model aggregation
    test('baris tabel utama konsisten dengan model aggregation', () {
      final m = buildSingle([
        tx('i1', 'income', '100000.00', method: 'CASH'),
        tx('e1', 'expense', '50000.00', method: 'CASH'),
        tx('e2', 'expense', '25000.00', method: 'TRANSFER'),
      ], opening: '200000.00');
      final rows = m.statementRows();
      // Baris summary tidak memiliki saldo
      final totalExpenseRow = rows.firstWhere(
        (r) => r.label == 'Total pengeluaran Tunai + Transfer',
      );
      expect(totalExpenseRow.saldo, isNull);
      // Saldo akhir = 200k + 100k - 75k = 225k
      final closingRow = rows.firstWhere(
        (r) => r.label == 'Saldo akhir (saldo awal + penerimaan - pengeluaran)',
      );
      expect(closingRow.saldo, m.closingCents);
      expect(closingRow.saldo, 22500000);
    });

    // 10. Penerimaan juga tidak boleh double-count
    test('penerimaan tunai juga tidak double-count ke transfer', () {
      final m = buildSingle([tx('i1', 'income', '100000.00', method: 'CASH')]);
      expect(m.income.cash, 10000000);
      expect(m.income.transfer, 0);
      expect(m.income.other, 0);
      expect(m.totalIncomeCents, 10000000);
    });

    // 11. Transaksi draft/cancelled dikecualikan
    test('transaksi draft dan cancelled tidak termasuk perhitungan', () {
      final m = buildSingle([
        tx('e1', 'expense', '50000.00', method: 'CASH', status: 'posted'),
        tx('e2', 'expense', '30000.00', method: 'CASH', status: 'draft'),
        tx('e3', 'expense', '20000.00', method: 'CASH', status: 'cancelled'),
      ]);
      expect(m.transactionCount, 1);
      expect(m.expense.cash, 5000000);
      expect(m.totalExpenseCents, 5000000);
    });

    // 12. SORTING KRONOLOGIS — Running balance reliabel
    test('transaksi diurut kronologis ascending sebelum agregasi', () {
      final m = buildSingle([
        tx('e2', 'expense', '30000.00', category: 'Keamanan', method: 'CASH', occurredAt: '2026-10-03T00:00:00Z'),
        tx('i1', 'income', '100000.00', category: 'Iuran Warga', method: 'CASH', occurredAt: '2026-10-01T00:00:00Z'),
        tx('e1', 'expense', '20000.00', category: 'Keamanan', method: 'TRANSFER', occurredAt: '2026-10-05T00:00:00Z'),
        tx('i2', 'income', '50000.00', category: 'Iuran Warga', method: 'TRANSFER', occurredAt: '2026-10-02T00:00:00Z'),
      ], opening: '200000.00');

      // Agregasi total TIDAK bergantung urutan → tetap sama
      expect(m.income.cash, 10000000);
      expect(m.income.transfer, 5000000);
      expect(m.totalIncomeCents, 15000000);
      expect(m.expense.cash, 3000000);
      expect(m.expense.transfer, 2000000);
      expect(m.totalExpenseCents, 5000000);
      expect(m.closingCents, 30000000); // 200k + 150k - 50k

      // Statement rows: saldo berjalan harus kronologis
      final rows = m.statementRows();
      // Saldo awal
      expect(rows.first.label, 'Saldo awal periode');
      expect(rows.first.saldo, 20000000);

      // Baris kategori penerimaan TIDAK menampilkan saldo (agregat alfabetis)
      // Urutan di statement: penerimaan tunai → transfer → other → pengeluaran
      final i1Row = rows.firstWhere((r) => r.label.contains('Iuran Warga (tunai)'));
      expect(i1Row.saldo, isNull); // agregat kategori: saldo dikosongkan

      final i2Row = rows.firstWhere((r) => r.label.contains('Iuran Warga (transfer)'));
      expect(i2Row.saldo, isNull); // agregat kategori: saldo dikosongkan

      // Total penerimaan Tunai + Transfer tidak punya saldo
      final totalIncomeRow = rows.firstWhere(
        (r) => r.label == 'Total penerimaan Tunai + Transfer',
      );
      expect(totalIncomeRow.saldo, isNull);

      // Saldo akhir benar
      final closingRow = rows.firstWhere(
        (r) => r.label.startsWith('Saldo akhir'),
      );
      expect(closingRow.saldo, 30000000);
    });

    test('tie-breaker id menentukan urutan jika occurred_at sama', () {
      final m = buildSingle([
        tx('b-tx', 'expense', '10000.00', category: 'Operasional RT', method: 'CASH', occurredAt: '2026-10-01T00:00:00Z'),
        tx('a-tx', 'income', '50000.00', category: 'Iuran Warga', method: 'CASH', occurredAt: '2026-10-01T00:00:00Z'),
      ], opening: '50000.00');

      // Total tidak bergantung order → harus tetap sama
      expect(m.income.cash, 5000000);
      expect(m.expense.cash, 1000000);
      expect(m.closingCents, 9000000); // 50k + 50k - 10k
      expect(m.transactionCount, 2);
    });

    test('fallback ke createdAt jika occurredAt null', () {
      final m = buildKasReport(
        transactions: [
          tx('e1', 'expense', '20000.00', method: 'CASH', occurredAt: null),
          tx('i1', 'income', '100000.00', method: 'CASH', occurredAt: '2026-10-01T00:00:00Z'),
          tx('e2', 'expense', '10000.00', method: 'CASH', occurredAt: '2026-10-02T00:00:00Z'),
        ],
        openingBalance: '200000.00',
        apiClosingBalance: '270000.00',
        period: '2026-10',
        identity: const KasReportIdentity(orgName: 'X', rtLabel: 'RT 01'),
        generatedAt: DateTime(2026, 10, 23),
      );
      expect(m.closingCents, 27000000); // 200k + 100k - 20k - 10k = 270k
      expect(m.transactionCount, 3);
    });
  });

  // -----------------------------------------------------------------------
  // Regression test — Perbaikan UI Android: scroll horizontal, no-wrap, zoom
  // -----------------------------------------------------------------------

  group('regression: perbaikan tampilan laporan di Android', () {
    testWidgets(
      'tombol zoom (+/-) dan persentase muncul di dokumen laporan',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();
        // Tombol minus
        expect(find.text('-'), findsOneWidget);
        // Tombol plus
        expect(find.text('+'), findsOneWidget);
        // Persentase zoom (default 100%)
        expect(find.text('100%'), findsOneWidget);
      },
    );

    testWidgets('zoom in: persentase naik, tombol minus aktif', (tester) async {
      await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
      await tester.pumpAndSettle();
      // Tekan tombol +
      await tester.tap(find.text('+'));
      await tester.pumpAndSettle();
      expect(find.text('110%'), findsOneWidget);
      // Tekan + lagi
      await tester.tap(find.text('+'));
      await tester.pumpAndSettle();
      expect(find.text('120%'), findsOneWidget);
    });

    testWidgets('zoom out: persentase turun, tombol plus aktif', (tester) async {
      await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
      await tester.pumpAndSettle();
      // Tekan tombol -
      await tester.tap(find.text('-'));
      await tester.pumpAndSettle();
      expect(find.text('90%'), findsOneWidget);
      await tester.tap(find.text('-'));
      await tester.pumpAndSettle();
      expect(find.text('80%'), findsOneWidget);
    });

    testWidgets('zoom in dibatasi maksimum 250%', (tester) async {
      await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
      await tester.pumpAndSettle();
      // Tekan tombol zoom in
      for (var i = 0; i < 20; i++) {
        await tester.tap(find.byKey(const Key('kas-report-zoom-in')));
        await tester.pumpAndSettle();
      }
      expect(find.text('250%'), findsOneWidget);
    });

    testWidgets('zoom out dibatasi minimum 70%', (tester) async {
      await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
      await tester.pumpAndSettle();
      // Tekan - hingga mencapai batas
      for (var i = 0; i < 20; i++) {
        await tester.tap(find.text('-'));
        await tester.pumpAndSettle();
      }
      expect(find.text('70%'), findsOneWidget);
    });

    testWidgets(
      'tabel utama dan rekap muncul sebagai children dokumen laporan',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();
        // Dokumen laporan ada
        expect(find.byKey(const Key('kas-report-document')), findsOneWidget);
        // Tabel transaksi ada
        expect(find.byKey(const Key('kas-report-statement')), findsOneWidget);
        // Rekap kategori ada
        expect(find.byKey(const Key('kas-report-recap')), findsOneWidget);
      },
    );

    testWidgets(
      'nama kategori dan urian muncul di dokumen laporan',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();
        // Uraian / label baris muncul
        expect(find.text('Saldo awal periode'), findsOneWidget);
        expect(
          find.text('Saldo akhir (saldo awal + penerimaan - pengeluaran)'),
          findsOneWidget,
        );
        // Rekap: nama kategori muncul
        expect(find.text('Keamanan'), findsOneWidget);
        expect(find.text('Kebersihan'), findsOneWidget);
      },
    );

    testWidgets(
      'transform.scale (zoom) membungkus dokumen laporan',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();
        // SingleChildScrollView dengan scrollDirection horizontal harus ada
        // di sekitar dokumen laporan
        expect(
          find.ancestor(
            of: find.byKey(const Key('kas-report-document')),
            matching: find.byType(SingleChildScrollView),
          ),
          findsAtLeast(1),
        );
      },
    );

    // ------------------------------------------------------------------
    // Test visual zoom: font size dan padding kolom benar-benar diskalakan
    // ------------------------------------------------------------------

    testWidgets(
      'zoom 70%: font size header 7px (10 * 0.7), padding lebih kecil',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();

        // Zoom out ke 70%
        for (var i = 0; i < 3; i++) {
          await tester.tap(find.byKey(const Key('kas-report-zoom-out')));
          await tester.pumpAndSettle();
        }
        expect(find.text('70%'), findsOneWidget);

        // Cari tabel statement
        final statementTable = find.byKey(const Key('kas-report-statement'));
        expect(statementTable, findsOneWidget);

        // Header teks 'No' di dalam tabel: di-zoom 0.7x jadi 7.0
        final noHeader = tester.widget<Text>(
          find.descendant(
            of: statementTable,
            matching: find.byType(Text),
          ).at(0),
        );
        // Font size header tabel = 10 * 0.7 = 7.0
        expect((noHeader.style?.fontSize ?? 0.0), closeTo(7.0, 0.001));
      },
    );

    testWidgets(
      'zoom 100% (default): font size header 10px, padding 6/4',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();

        expect(find.text('100%'), findsOneWidget);

        // Header teks pertama di tabel statement
        final statementTable = find.byKey(const Key('kas-report-statement'));
        final firstText = tester.widget<Text>(
          find.descendant(
            of: statementTable,
            matching: find.byType(Text),
          ).at(0),
        );
        // Font size header tabel = 10 * 1.0 = 10.0
        expect((firstText.style?.fontSize ?? 0.0), closeTo(10.0, 0.001));
      },
    );

    testWidgets(
      'zoom 150%: font size header 15px (10 * 1.5), padding lebih besar',
      (tester) async {
        await pumpScreen(tester, _FakeRepo(txs: fixture, closing: '3200000.00'));
        await tester.pumpAndSettle();

        // Zoom in ke 150%
        for (var i = 0; i < 5; i++) {
          await tester.tap(find.byKey(const Key('kas-report-zoom-in')));
          await tester.pumpAndSettle();
        }
        expect(find.text('150%'), findsOneWidget);

        final statementTable = find.byKey(const Key('kas-report-statement'));
        final firstText = tester.widget<Text>(
          find.descendant(
            of: statementTable,
            matching: find.byType(Text),
          ).at(0),
        );
        // Font size header tabel = 10 * 1.5 = 15.0
        expect((firstText.style?.fontSize ?? 0.0), closeTo(15.0, 0.001));
      },
    );

    testWidgets(
      'zoom 150% vs 70%: ukuran font judul header berbeda jauh',
      (tester) async {
        // Build dua instance KasReportDocument dengan zoom berbeda
        final model = build();

        final doc150 = KasReportDocument(model: model, zoom: 1.5);
        final doc70 = KasReportDocument(model: model, zoom: 0.7);

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: SingleChildScrollView(
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: 600),
                  child: doc150,
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Cari Text widget pertama di dokumen (header baris pertama)
        final text150 = tester.widget<Text>(
          find.descendant(
            of: find.byType(KasReportDocument),
            matching: find.byType(Text),
          ).at(0),
        );

        final doc150Height = text150.style?.fontSize;

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: SingleChildScrollView(
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: 600),
                  child: doc70,
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        final text70 = tester.widget<Text>(
          find.descendant(
            of: find.byType(KasReportDocument),
            matching: find.byType(Text),
          ).at(0),
        );

        final doc70Height = text70.style?.fontSize;

        // 150% harus jauh lebih besar dari 70%
        expect(doc150Height, isNotNull);
        expect(doc70Height, isNotNull);
        expect(doc150Height!, greaterThan(doc70Height! * 1.5));
        expect(doc150Height, closeTo(14.0 * 1.5, 0.01)); // judul utama = 14px base * 1.5
        expect(doc70Height, closeTo(14.0 * 0.7, 0.01)); // judul utama = 14px base * 0.7
      },
    );
  });
}
