/// Iuran API models — Backend DTOs matching the Go backend contracts.
///
/// All monetary values are kept as `String` to avoid floating-point issues.
/// Conversion to display integers uses helper functions in `iuran_models.dart`.

import 'package:flutter/foundation.dart';

import 'iuran_models.dart' show IuranStatus;

// ─── Pagination (reusable) ────────────────────────────────────────────

/// Mirrors `PaginatedResponse` from Warga pattern.
/// Kept separate to avoid circular deps; both use `ServerException`.
@immutable
class PaginatedResponse<T> {
  final List<T> data;
  final PaginationInfo pagination;

  const PaginatedResponse({
    required this.data,
    required this.pagination,
  });
}

@immutable
class PaginationInfo {
  final int page;
  final int pageSize;
  final int total;
  final int totalPages;

  const PaginationInfo({
    required this.page,
    required this.pageSize,
    required this.total,
    required this.totalPages,
  });
}

// ─── Enums ────────────────────────────────────────────────────────────

enum CategoryType { income, expense }

enum DuePeriodType { monthly, yearly, oneTime }

enum BillStatus { unpaid, partial, paid, cancelled }

enum PaymentMethod { cash, transfer }

enum PaymentOrigin { selfSubmitted, staffRecorded }

enum PaymentStatus { pending, approved, rejected }

enum TransactionType { income, expense }

enum TransactionStatus { draft, posted, cancelled }

// ─── BackendCategory ──────────────────────────────────────────────────

@immutable
class BackendCategory {
  final String id;
  final String rtId;
  final String name;
  final CategoryType type;
  final bool isActive;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  const BackendCategory({
    required this.id,
    required this.rtId,
    required this.name,
    required this.type,
    required this.isActive,
    this.createdAt,
    this.updatedAt,
  });

  factory BackendCategory.fromJson(Map<String, dynamic> json) {
    return BackendCategory(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      type: _parseCategoryType(json['type'] as String?),
      isActive: json['is_active'] as bool? ?? true,
      createdAt: _parseDateTime(json['created_at'] as String?),
      updatedAt: _parseDateTime(json['updated_at'] as String?),
    );
  }
}

CategoryType _parseCategoryType(String? value) {
  switch (value) {
    case 'income':
      return CategoryType.income;
    case 'expense':
      return CategoryType.expense;
    default:
      return CategoryType.income;
  }
}

// ─── BackendDue ───────────────────────────────────────────────────────

@immutable
class BackendDue {
  final String id;
  final String rtId;
  final String name;
  final String amount; // raw NUMERIC string
  final DuePeriodType periodType;
  final bool isActive;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  const BackendDue({
    required this.id,
    required this.rtId,
    required this.name,
    required this.amount,
    required this.periodType,
    required this.isActive,
    this.createdAt,
    this.updatedAt,
  });

  factory BackendDue.fromJson(Map<String, dynamic> json) {
    return BackendDue(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      amount: json['amount'] as String? ?? '',
      periodType: _parseDuePeriodType(json['period_type'] as String?),
      isActive: json['is_active'] as bool? ?? true,
      createdAt: _parseDateTime(json['created_at'] as String?),
      updatedAt: _parseDateTime(json['updated_at'] as String?),
    );
  }
}

DuePeriodType _parseDuePeriodType(String? value) {
  switch (value) {
    case 'monthly':
      return DuePeriodType.monthly;
    case 'yearly':
      return DuePeriodType.yearly;
    case 'one_time':
      return DuePeriodType.oneTime;
    default:
      return DuePeriodType.monthly;
  }
}

// ─── BackendBill ──────────────────────────────────────────────────────

@immutable
class BackendBill {
  final String id;
  final String rtId;
  final String householdOccupancyId;
  final String dueId;
  final String amount; // raw NUMERIC string
  final String period; // e.g. "2026-10"
  final String dueDate; // YYYY-MM-DD
  final BillStatus status;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  // Enriched fields from JOIN queries
  final String? dueName;
  final String? houseNumber;
  final String? headName;

  const BackendBill({
    required this.id,
    required this.rtId,
    required this.householdOccupancyId,
    required this.dueId,
    required this.amount,
    required this.period,
    required this.dueDate,
    required this.status,
    this.createdAt,
    this.updatedAt,
    this.dueName,
    this.houseNumber,
    this.headName,
  });

  factory BackendBill.fromJson(Map<String, dynamic> json) {
    return BackendBill(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      householdOccupancyId: json['household_occupancy_id'] as String? ?? '',
      dueId: json['due_id'] as String? ?? '',
      amount: json['amount'] as String? ?? '',
      period: json['period'] as String? ?? '',
      dueDate: json['due_date'] as String? ?? '',
      status: _parseBillStatus(json['status'] as String?),
      createdAt: _parseDateTime(json['created_at'] as String?),
      updatedAt: _parseDateTime(json['updated_at'] as String?),
      dueName: json['due_name'] as String?,
      houseNumber: json['house_number'] as String?,
      headName: json['head_name'] as String?,
    );
  }
}

BillStatus _parseBillStatus(String? value) {
  switch (value) {
    case 'unpaid':
      return BillStatus.unpaid;
    case 'partial':
      return BillStatus.partial;
    case 'paid':
      return BillStatus.paid;
    case 'cancelled':
      return BillStatus.cancelled;
    default:
      return BillStatus.unpaid;
  }
}

// ─── BackendPayment ───────────────────────────────────────────────────

@immutable
class BackendPayment {
  final String id;
  final String rtId;
  final String billId;
  final String amount;
  final PaymentMethod method;
  final PaymentOrigin origin;
  final PaymentStatus status;
  final String? proofPath;
  final DateTime? paidAt;
  final String? verifiedBy;
  final DateTime? verifiedAt;
  final String? rejectionReason;
  final String? notes;
  final DateTime? createdAt;
  final DateTime? updatedAt;

  const BackendPayment({
    required this.id,
    required this.rtId,
    required this.billId,
    required this.amount,
    required this.method,
    required this.origin,
    required this.status,
    this.proofPath,
    this.paidAt,
    this.verifiedBy,
    this.verifiedAt,
    this.rejectionReason,
    this.notes,
    this.createdAt,
    this.updatedAt,
  });

  factory BackendPayment.fromJson(Map<String, dynamic> json) {
    return BackendPayment(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      billId: json['bill_id'] as String? ?? '',
      amount: json['amount'] as String? ?? '',
      method: _parsePaymentMethod(json['method'] as String?),
      origin: _parsePaymentOrigin(json['origin'] as String?),
      status: _parsePaymentStatus(json['status'] as String?),
      proofPath: json['proof_path'] as String?,
      paidAt: _parseDateTime(json['paid_at'] as String?),
      verifiedBy: json['verified_by'] as String?,
      verifiedAt: _parseDateTime(json['verified_at'] as String?),
      rejectionReason: json['rejection_reason'] as String?,
      notes: json['notes'] as String?,
      createdAt: _parseDateTime(json['created_at'] as String?),
      updatedAt: _parseDateTime(json['updated_at'] as String?),
    );
  }
}

PaymentMethod _parsePaymentMethod(String? value) {
  switch (value) {
    case 'CASH':
      return PaymentMethod.cash;
    case 'TRANSFER':
      return PaymentMethod.transfer;
    default:
      return PaymentMethod.transfer;
  }
}

PaymentOrigin _parsePaymentOrigin(String? value) {
  switch (value) {
    case 'SELF_SUBMITTED':
      return PaymentOrigin.selfSubmitted;
    case 'STAFF_RECORDED':
      return PaymentOrigin.staffRecorded;
    default:
      return PaymentOrigin.selfSubmitted;
  }
}

PaymentStatus _parsePaymentStatus(String? value) {
  switch (value) {
    case 'PENDING':
      return PaymentStatus.pending;
    case 'APPROVED':
      return PaymentStatus.approved;
    case 'REJECTED':
      return PaymentStatus.rejected;
    default:
      return PaymentStatus.pending;
  }
}

// ─── BackendBalance ───────────────────────────────────────────────────

@immutable
class BackendBalance {
  final String totalIncome;
  final String totalExpense;
  final String netBalance;
  final int transactionCount;

  const BackendBalance({
    required this.totalIncome,
    required this.totalExpense,
    required this.netBalance,
    required this.transactionCount,
  });

  factory BackendBalance.fromJson(Map<String, dynamic> json) {
    return BackendBalance(
      totalIncome: json['total_income'] as String? ?? '0',
      totalExpense: json['total_expense'] as String? ?? '0',
      netBalance: json['net_balance'] as String? ?? '0',
      transactionCount: (json['transaction_count'] as num?)?.toInt() ?? 0,
    );
  }
}

// ─── Utility helpers ──────────────────────────────────────────────────

DateTime? _parseDateTime(String? value) {
  if (value == null || value.isEmpty) return null;
  try {
    return DateTime.parse(value);
  } catch (_) {
    return null;
  }
}

/// Map backend [BillStatus] to UI status name for filter matching.
/// Backend: 'unpaid'/'partial'/'paid'/'cancelled'
/// UI:     'belumBayar'/'sebagian'/'lunas'
String billStatusToFilter(BillStatus status) {
  switch (status) {
    case BillStatus.unpaid:
      return 'belumBayar';
    case BillStatus.partial:
      return 'sebagian';
    case BillStatus.paid:
      return 'lunas';
    case BillStatus.cancelled:
      return 'lunas';
  }
}

/// Map backend [BillStatus] to [IuranStatus] (used by iuran_models.dart).
/// Returns the display status enum matching existing IuranStatus.
IuranStatus _billStatusToIuranStatus(BillStatus status) {
  switch (status) {
    case BillStatus.unpaid:
      return IuranStatus.belumBayar;
    case BillStatus.partial:
      return IuranStatus.sebagian;
    case BillStatus.paid:
      return IuranStatus.lunas;
    case BillStatus.cancelled:
      return IuranStatus.lunas;
  }
}

/// Parse a backend money string (e.g. "50000.00") to int cents/smallest unit.
/// Returns `null` if parsing fails.
int? parseMoneyToInt(String? value) {
  if (value == null || value.isEmpty) return null;
  try {
    final parts = value.split('.');
    final whole = int.tryParse(parts[0]) ?? 0;
    final fractional = parts.length > 1 && parts[1].length >= 2
        ? int.tryParse('${parts[1][0]}${parts[1][1]}') ?? 0
        : 0;
    return whole * 100 + fractional;
  } catch (_) {
    return null;
  }
}

/// Parse a backend money string to display int (rupiah as stored in mock).
/// The mock uses raw rupiah as int (50000 for Rp 50.000).
/// Backend NUMERIC(15,2) stores rupiah with 2 decimal places.
int moneyToDisplayRupiah(String value) {
  try {
    final parts = value.split('.');
    final whole = int.tryParse(parts[0]) ?? 0;
    return whole; // Display as whole rupiah (e.g. 50000 for Rp 50.000)
  } catch (_) {
    return 0;
  }
}

/// Format backend period string ("2026-10") to Indonesian ("Oktober 2026").
String formatPeriod(String period) {
  final parts = period.split('-');
  if (parts.length < 2) return period;
  final year = parts[0];
  final monthIndex = int.tryParse(parts[1]) ?? 0;
  const months = [
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
  if (monthIndex < 0 || monthIndex >= months.length) return period;
  return '${months[monthIndex]} $year';
}
