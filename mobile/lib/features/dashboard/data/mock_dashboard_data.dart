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

/// Dashboard summary data (mock for Phase A).
class DashboardSummary {
  final int saldoKas;
  final int pemasukanBulanIni;
  final int pengeluaranBulanIni;
  final DuesProgress duesProgress;
  final List<MonthlyFinancePoint> monthlyData;

  const DashboardSummary({
    required this.saldoKas,
    required this.pemasukanBulanIni,
    required this.pengeluaranBulanIni,
    required this.duesProgress,
    required this.monthlyData,
  });
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
