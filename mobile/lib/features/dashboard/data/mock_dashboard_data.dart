import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Monthly finance data point for the bar chart.
class MonthlyFinancePoint {
  final String month;
  final int income;
  final int expense;

  const MonthlyFinancePoint({
    required this.month,
    required this.income,
    required this.expense,
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

/// Dashboard summary data.
class DashboardSummary {
  final int saldoKas;
  final int pemasukanBulanIni;
  final int pengeluaranBulanIni;
  final DuesProgress duesProgress;
  final List<MonthlyFinancePoint> monthlyData;
  final List<Transaction> recentTransactions;

  const DashboardSummary({
    required this.saldoKas,
    required this.pemasukanBulanIni,
    required this.pengeluaranBulanIni,
    required this.duesProgress,
    required this.monthlyData,
    this.recentTransactions = const [],
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
    final chartList = json['chart'] as List<dynamic>? ?? [];
    final txList = json['recent_transactions'] as List<dynamic>? ?? [];

    final monthly = chartList.map((c) {
      final m = c as Map<String, dynamic>;
      return MonthlyFinancePoint(
        month: m['month']?.toString() ?? '',
        income: parseMoney(m['income']),
        expense: parseMoney(m['expense']),
      );
    }).toList();

    const monthShortNames = ['', 'Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'];
    String formatTxDate(String? dtStr) {
      if (dtStr == null || dtStr.isEmpty) return '';
      final dt = DateTime.tryParse(dtStr);
      if (dt == null) return dtStr;
      return '${dt.day} ${monthShortNames[dt.month]} ${dt.year}';
    }

    final transactions = txList.map((t) {
      final tx = t as Map<String, dynamic>;
      final isInc = tx['type'] == 'income';
      final desc = tx['due_name'] != null && tx['citizen_name'] != null
          ? '[${tx['due_name']}] ${tx['citizen_name']}${tx['house_number'] != null && tx['house_number'].toString().isNotEmpty ? ' · ${tx['house_number']}' : ''}'
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

    return DashboardSummary(
      saldoKas: parseMoney(json['balance']),
      pemasukanBulanIni: parseMoney(json['monthly_income']),
      pengeluaranBulanIni: parseMoney(json['monthly_expense']),
      duesProgress: DuesProgress(
        paid: (dues?['paid_households'] as num?)?.toInt() ?? 0,
        target: (dues?['total_households'] as num?)?.toInt() ?? 0,
      ),
      monthlyData: monthly,
      recentTransactions: transactions,
    );
  }

  factory DashboardSummary.empty() {
    return const DashboardSummary(
      saldoKas: 0,
      pemasukanBulanIni: 0,
      pengeluaranBulanIni: 0,
      duesProgress: DuesProgress(paid: 0, target: 0),
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
    MonthlyFinancePoint(month: 'Apr', income: 3200000, expense: 1800000),
    MonthlyFinancePoint(month: 'Mei', income: 4100000, expense: 2300000),
    MonthlyFinancePoint(month: 'Jun', income: 3800000, expense: 2100000),
    MonthlyFinancePoint(month: 'Jul', income: 4600000, expense: 2800000),
    MonthlyFinancePoint(month: 'Agu', income: 3900000, expense: 2450000),
    MonthlyFinancePoint(month: 'Sep', income: 4250000, expense: 2175000),
  ];

  static const List<Transaction> recentTransactions = [
    Transaction(
      id: 'tx-001',
      description: 'Iuran Bulanan',
      category: 'Warga A',
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
      category: 'Warga B',
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

  static DashboardSummary getSummary() {
    return DashboardSummary(
      saldoKas: _saldoKas,
      pemasukanBulanIni: _pemasukanBulanIni,
      pengeluaranBulanIni: _pengeluaranBulanIni,
      duesProgress: _duesProgress,
      monthlyData: _monthlyData,
    );
  }
}

final dashboardSummaryProvider = Provider<DashboardSummary>((ref) {
  return DashboardMockData.getSummary();
});

final recentTransactionsProvider = Provider<List<Transaction>>((ref) {
  return DashboardMockData.recentTransactions;
});
