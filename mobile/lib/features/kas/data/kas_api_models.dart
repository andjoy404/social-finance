/// KAS API models — Backend DTOs matching the Go backend contracts.
///
/// All monetary values are kept as `String` to avoid floating-point issues.
/// Conversion to display integers uses helper functions in `kas_models.dart`.

import 'package:flutter/foundation.dart';

// ─── BackendTransaction ───────────────────────────────────────────────

/// Mirrors the Go `Transaction` struct in `backend/internal/finance/model.go`.
@immutable
class BackendTransaction {
  final String id;
  final String rtId;
  final String categoryId;
  final String amount; // raw NUMERIC string
  final String type; // 'income' | 'expense'
  final String status; // 'draft' | 'posted' | 'cancelled'
  final String? reversesTransactionId;
  final String? paymentId;
  final String? paymentNo;
  final String description;
  final String? occurredAt; // ISO 8601 timestamptz
  final String? createdAt; // ISO 8601 timestamptz
  final String? categoryName;
  final String? officerName;
  final String? dueName;
  final String? citizenName;
  final String? houseNumber;
  final String? paymentMethod;

  const BackendTransaction({
    required this.id,
    required this.rtId,
    required this.categoryId,
    required this.amount,
    required this.type,
    required this.status,
    this.reversesTransactionId,
    this.paymentId,
    this.paymentNo,
    required this.description,
    this.occurredAt,
    this.createdAt,
    this.categoryName,
    this.officerName,
    this.dueName,
    this.citizenName,
    this.houseNumber,
    this.paymentMethod,
  });

  factory BackendTransaction.fromJson(Map<String, dynamic> json) {
    return BackendTransaction(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      categoryId: json['category_id'] as String? ?? '',
      amount: json['amount'] as String? ?? '',
      type: json['type'] as String? ?? '',
      status: json['status'] as String? ?? '',
      reversesTransactionId: json['reverses_transaction_id'] as String?,
      paymentId: json['payment_id'] as String?,
      paymentNo: (json['payment_no'] ?? json['reference_no']) as String?,
      description: json['description'] as String? ?? '',
      occurredAt: json['occurred_at'] as String?,
      createdAt: json['created_at'] as String?,
      categoryName: json['category_name'] as String?,
      officerName: json['officer_name'] as String?,
      dueName: json['due_name'] as String?,
      citizenName: json['citizen_name'] as String?,
      houseNumber: json['house_number'] as String?,
      paymentMethod: json['payment_method'] as String?,
    );
  }
}

// ─── BackendBalance ───────────────────────────────────────────────────

@immutable
class BackendBalance {
  final String openingBalance;
  final String totalIncome;
  final String totalExpense;
  final String netMovement;
  final String closingBalance;
  final String netBalance;
  final int transactionCount;

  const BackendBalance({
    this.openingBalance = '0',
    required this.totalIncome,
    required this.totalExpense,
    this.netMovement = '0',
    this.closingBalance = '0',
    required this.netBalance,
    required this.transactionCount,
  });

  factory BackendBalance.fromJson(Map<String, dynamic> json) {
    return BackendBalance(
      openingBalance: json['opening_balance'] as String? ?? '0',
      totalIncome: json['total_income'] as String? ?? '0',
      totalExpense: json['total_expense'] as String? ?? '0',
      netMovement: json['net_movement'] as String? ?? '0',
      closingBalance: json['closing_balance'] as String? ?? json['net_balance'] as String? ?? '0',
      netBalance: json['net_balance'] as String? ?? '0',
      transactionCount: (json['transaction_count'] as num?)?.toInt() ?? 0,
    );
  }
}

// ─── BackendCategory ──────────────────────────────────────────────────

@immutable
class BackendCategory {
  final String id;
  final String rtId;
  final String name;
  final String type; // 'income' | 'expense'
  final bool isActive;

  const BackendCategory({
    required this.id,
    required this.rtId,
    required this.name,
    required this.type,
    required this.isActive,
  });

  factory BackendCategory.fromJson(Map<String, dynamic> json) {
    return BackendCategory(
      id: json['id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      type: json['type'] as String? ?? 'income',
      isActive: json['is_active'] as bool? ?? true,
    );
  }
}

// ─── Pagination ───────────────────────────────────────────────────────

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
