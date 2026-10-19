import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';

/// Monthly finance data point for the bar chart.
class MonthlyFinancePoint {
  final String month;
  final int income;
  final int expense;
  final int? incomeCount;
  final int? expenseCount;

  const MonthlyFinancePoint({
    required this.month,
    required this.income,
    required this.expense,
    this.incomeCount,
    this.expenseCount,
  });
}

/// Dues progress summary for the Iuran Bulan Ini section.
class DuesProgress {
  final int paid;
  final int target;

  const DuesProgress({required this.paid, required this.target});

  int get percentage => target > 0 ? ((paid / target) * 100).round() : 0;

  double get ratio => target > 0 ? paid / target : 0.0;
}

/// Payment method breakdown for the Beranda Donut Chart (Transfer vs Tunai).
class PaymentMethodBreakdown {
  final int cashAmount;
  final int transferAmount;
  final int cashCount;
  final int transferCount;

  const PaymentMethodBreakdown({
    this.cashAmount = 0,
    this.transferAmount = 0,
    this.cashCount = 0,
    this.transferCount = 0,
  });

  int get totalAmount => cashAmount + transferAmount;
  int get totalCount => cashCount + transferCount;

  int get transferPercentage {
    if (totalAmount > 0) {
      return ((transferAmount / totalAmount) * 100).round();
    }
    if (totalCount > 0) {
      return ((transferCount / totalCount) * 100).round();
    }
    return 0;
  }

  int get cashPercentage {
    if (totalAmount > 0 || totalCount > 0) {
      return 100 - transferPercentage;
    }
    return 0;
  }
}

/// Dashboard summary data.
class DashboardSummary {
  final int saldoKas;
  final int pemasukanBulanIni;
  final int pengeluaranBulanIni;
  final DuesProgress duesProgress;
  final PaymentMethodBreakdown paymentMethodBreakdown;
  final List<MonthlyFinancePoint> monthlyData;
  final List<Transaction> recentTransactions;
  final List<KasTransaction> recentKasTransactions;

  const DashboardSummary({
    required this.saldoKas,
    required this.pemasukanBulanIni,
    required this.pengeluaranBulanIni,
    required this.duesProgress,
    this.paymentMethodBreakdown = const PaymentMethodBreakdown(),
    required this.monthlyData,
    this.recentTransactions = const [],
    this.recentKasTransactions = const [],
  });

  factory DashboardSummary.fromJson(Map<String, dynamic> json) {
    int parseMoney(dynamic val) {
      if (val == null) return 0;
      if (val is num) return val.toInt();
      final s = val.toString().trim();
      if (s.isEmpty) return 0;
      return int.tryParse(s.split('.')[0]) ?? 0;
    }

    final dues = json['dues_progress'] as Map<String, dynamic>?;
    final pmb = json['payment_method_breakdown'] as Map<String, dynamic>?;
    final chartList = json['chart'] as List<dynamic>? ?? [];
    final txList = json['recent_transactions'] as List<dynamic>? ?? [];

    final monthly = chartList.map((c) {
      final m = c as Map<String, dynamic>;
      return MonthlyFinancePoint(
        month: m['month']?.toString() ?? '',
        income: parseMoney(m['income']),
        expense: parseMoney(m['expense']),
        incomeCount: (m['income_count'] as num?)?.toInt(),
        expenseCount: (m['expense_count'] as num?)?.toInt(),
      );
    }).toList();

    const monthShortNames = ['', 'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'];
    String formatTxDate(String? dtStr) {
      if (dtStr == null || dtStr.isEmpty) return '';
      final dt = DateTime.tryParse(dtStr);
      if (dt == null) return dtStr;
      return '${dt.day} ${monthShortNames[dt.month]} ${dt.year}';
    }

    String? extractReference(Map<String, dynamic> tx) {
      final rawPayNo = (tx['payment_no'] ?? tx['reference_no'])?.toString().trim();
      if (rawPayNo != null &&
          rawPayNo.isNotEmpty &&
          rawPayNo.toLowerCase() != 'null' &&
          rawPayNo.toLowerCase() != 'unknown') {
        return rawPayNo;
      }
      final rawPayId = tx['payment_id']?.toString().trim();
      if (rawPayId != null &&
          rawPayId.isNotEmpty &&
          rawPayId.toLowerCase() != 'null') {
        return rawPayId.length >= 8
            ? 'PAY-${rawPayId.substring(0, 8)}'
            : rawPayId;
      }
      final rawRevId = tx['reverses_transaction_id']?.toString().trim();
      if (rawRevId != null &&
          rawRevId.isNotEmpty &&
          rawRevId.toLowerCase() != 'null') {
        return 'Reversal';
      }
      return null;
    }

    final transactions = txList.map((t) {
      final tx = t as Map<String, dynamic>;
      final isInc = tx['type'] == 'income';
      final ref = extractReference(tx);
      final dueName = tx['due_name']?.toString().trim();
      final desc = isInc && dueName != null && dueName.isNotEmpty
          ? (dueName.toUpperCase() == 'IPL'
              ? (ref ?? 'Tanpa Referensi')
              : '$dueName · ${ref ?? 'Tanpa Referensi'}')
          : (tx['description']?.toString() ?? '')
              .replaceAll(RegExp(r'\s*\(Bill:\s*[a-f0-9-]+\)', caseSensitive: false), '')
              .replaceAll(RegExp(r'Payment for bill [a-f0-9-]+\s*\(Period:\s*([^)]+)\)', caseSensitive: false), 'Pembayaran Iuran (\$1)');
      return Transaction(
        id: tx['id']?.toString() ?? '',
        description: desc,
        category: tx['category_name']?.toString() ?? (isInc ? 'Pemasukan' : 'Pengeluaran'),
        amount: parseMoney(tx['amount']),
        isIncome: isInc,
        date: formatTxDate(tx['occurred_at']?.toString() ?? tx['created_at']?.toString()),
      );
    }).toList();

    KasCategory resolveCategory(String name) {
      final normalized = name.trim().toLowerCase();
      if (normalized.contains('operasional')) return KasCategory.operasionalRT;
      if (normalized.contains('iuran warga') || normalized.contains('iuranwarga')) return KasCategory.iuranWarga;
      if (normalized.contains('iuran paksa') || normalized.contains('iuranpaksa')) return KasCategory.iuranPaksa;
      if (normalized.contains('perlengkapan')) return KasCategory.perlengkapan;
      if (normalized.contains('konsumsi')) return KasCategory.konsumsi;
      if (normalized.contains('perbaikan')) return KasCategory.perbaikan;
      if (normalized.contains('kebersihan')) return KasCategory.kebersihan;
      if (normalized.contains('keamanan')) return KasCategory.keamanan;
      return KasCategory.lainLain;
    }

    final kasTransactions = txList.map((t) {
      final tx = t as Map<String, dynamic>;
      final isInc = tx['type'] == 'income';
      final catName = tx['category_name']?.toString() ?? (isInc ? 'Iuran Warga' : 'Operasional RT');
      final amountStr = tx['amount']?.toString() ?? '0';
      final dt = DateTime.tryParse(tx['occurred_at']?.toString() ?? tx['created_at']?.toString() ?? '') ?? DateTime.now();

      return KasTransaction(
        id: tx['id']?.toString() ?? '',
        tanggal: dt,
        jenis: isInc ? KasJenis.masuk : KasJenis.keluar,
        kategori: resolveCategory(catName),
        keterangan: tx['description']?.toString() ?? '',
        referensi: extractReference(tx),
        dueName: tx['due_name']?.toString(),
        citizenName: isInc ? null : tx['citizen_name']?.toString(),
        houseNumber: isInc ? null : tx['house_number']?.toString(),
        paymentMethod: tx['payment_method']?.toString(),
        nominal: amountStr,
        saldo: '0',
      );
    }).toList();

    return DashboardSummary(
      saldoKas: parseMoney(json['balance']),
      pemasukanBulanIni: parseMoney(json['monthly_income']),
      pengeluaranBulanIni: parseMoney(json['monthly_expense']),
      duesProgress: DuesProgress(
        paid: (dues?['paid_households'] as num?)?.toInt() ?? 0,
        target: (dues?['total_households'] as num?)?.toInt() ?? 0,
      ),
      paymentMethodBreakdown: PaymentMethodBreakdown(
        cashAmount: parseMoney(pmb?['cash_amount']),
        transferAmount: parseMoney(pmb?['transfer_amount']),
        cashCount: (pmb?['cash_count'] as num?)?.toInt() ?? 0,
        transferCount: (pmb?['transfer_count'] as num?)?.toInt() ?? 0,
      ),
      monthlyData: monthly,
      recentTransactions: transactions,
      recentKasTransactions: kasTransactions,
    );
  }

  factory DashboardSummary.empty() {
    return const DashboardSummary(
      saldoKas: 0,
      pemasukanBulanIni: 0,
      pengeluaranBulanIni: 0,
      duesProgress: DuesProgress(paid: 0, target: 0),
      paymentMethodBreakdown: PaymentMethodBreakdown(),
      monthlyData: [],
      recentTransactions: [],
    );
  }
}

/// Transaction row model for recent transactions.
class Transaction {
  final String id;
  final String description;
  final String category;
  final int amount;
  final bool isIncome;
  final String date;

  const Transaction({
    required this.id,
    required this.description,
    required this.category,
    required this.amount,
    required this.isIncome,
    required this.date,
  });
}

/// Mock dashboard data factory.
class DashboardMockData {
  static const _saldoKas = 12450000;
  static const _pemasukanBulanIni = 4250000;
  static const _pengeluaranBulanIni = 2175000;

  static const _duesProgress = DuesProgress(paid: 42, target: 50);

  static const List<MonthlyFinancePoint> _monthlyData = [
    MonthlyFinancePoint(month: 'Apr', income: 3200000, expense: 1800000, incomeCount: 32, expenseCount: 4),
    MonthlyFinancePoint(month: 'Mei', income: 4100000, expense: 2300000, incomeCount: 41, expenseCount: 6),
    MonthlyFinancePoint(month: 'Jun', income: 3800000, expense: 2100000, incomeCount: 38, expenseCount: 5),
    MonthlyFinancePoint(month: 'Jul', income: 4600000, expense: 2800000, incomeCount: 46, expenseCount: 7),
    MonthlyFinancePoint(month: 'Agu', income: 3900000, expense: 2450000, incomeCount: 39, expenseCount: 5),
    MonthlyFinancePoint(month: 'Sep', income: 4250000, expense: 2175000, incomeCount: 42, expenseCount: 5),
  ];

  static const List<Transaction> recentTransactions = [
    Transaction(
      id: 'tx-001',
      description: 'Iuran Bulanan',
      category: 'Iuran Warga',
      amount: 85000,
      isIncome: true,
      date: '15 Sep 2025',
    ),
    Transaction(
      id: 'tx-002',
      description: 'Pembelian Perlengkapan',
      category: 'Kebersihan Lingkungan',
      amount: 350000,
      isIncome: false,
      date: '14 Sep 2025',
    ),
    Transaction(
      id: 'tx-003',
      description: 'Iuran Bulanan',
      category: 'Iuran Warga',
      amount: 85000,
      isIncome: true,
      date: '13 Sep 2025',
    ),
    Transaction(
      id: 'tx-004',
      description: 'Donasi Warga',
      category: 'Kesehatan',
      amount: 500000,
      isIncome: true,
      date: '12 Sep 2025',
    ),
    Transaction(
      id: 'tx-005',
      description: 'Perbaikan Fasilitas',
      category: 'Lampu Taman',
      amount: 125000,
      isIncome: false,
      date: '11 Sep 2025',
    ),
  ];

  static final List<KasTransaction> recentKasTransactions = [
    KasTransaction(
      id: 'tx-001',
      tanggal: DateTime(2025, 9, 15),
      jenis: KasJenis.masuk,
      kategori: KasCategory.iuranWarga,
      keterangan: 'Pembayaran Iuran IPL',
      referensi: 'INV-2025-001',
      dueName: 'IPL',
      paymentMethod: 'TRANSFER',
      nominal: '85000',
      saldo: '0',
    ),
    KasTransaction(
      id: 'tx-002',
      tanggal: DateTime(2025, 9, 14),
      jenis: KasJenis.keluar,
      kategori: KasCategory.kebersihan,
      keterangan: 'Pembelian Perlengkapan',
      paymentMethod: 'CASH',
      nominal: '350000',
      saldo: '0',
    ),
    KasTransaction(
      id: 'tx-003',
      tanggal: DateTime(2025, 9, 13),
      jenis: KasJenis.masuk,
      kategori: KasCategory.iuranWarga,
      keterangan: 'Pembayaran Iuran IPL',
      referensi: 'INV-2025-003',
      dueName: 'IPL',
      paymentMethod: 'CASH',
      nominal: '85000',
      saldo: '0',
    ),
  ];

  static const _paymentMethodBreakdown = PaymentMethodBreakdown(
    cashAmount: 1700000,
    transferAmount: 2550000,
    cashCount: 18,
    transferCount: 27,
  );

  static DashboardSummary getSummary() {
    return DashboardSummary(
      saldoKas: _saldoKas,
      pemasukanBulanIni: _pemasukanBulanIni,
      pengeluaranBulanIni: _pengeluaranBulanIni,
      duesProgress: _duesProgress,
      paymentMethodBreakdown: _paymentMethodBreakdown,
      monthlyData: _monthlyData,
      recentTransactions: recentTransactions,
      recentKasTransactions: recentKasTransactions,
    );
  }
}

final dashboardSummaryProvider = Provider<DashboardSummary>((ref) {
  return DashboardMockData.getSummary();
});

final recentTransactionsProvider = Provider<List<Transaction>>((ref) {
  return DashboardMockData.recentTransactions;
});

final recentKasTransactionsProvider = Provider<List<KasTransaction>>((ref) {
  return DashboardMockData.recentKasTransactions;
});
