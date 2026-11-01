/// Models untuk response dari endpoint PR 4 — RW-scoped aggregated data.
///
/// Response STRUKTURNYA BERBEDA dengan model RT biasa.
/// Semua response dibungkus { "data": {...} }.

// =============================================================================
// RW Management Models (CRUD untuk RW admin)
// =============================================================================

class RwModel {
  final String id;
  final int rwNumber;
  final String name;
  final String? headName;
  final String? headId;
  final String? address;
  final bool? isActive;

  const RwModel({
    required this.id,
    required this.rwNumber,
    required this.name,
    this.headName,
    this.headId,
    this.address,
    this.isActive,
  });

  String get displayName {
    if (name.trim().isNotEmpty) return name;
    return 'RW ${rwNumber.toString().padLeft(3, '0')}';
  }

  factory RwModel.fromJson(Map<String, dynamic> json) {
    return RwModel(
      id: json['id'] as String? ?? '',
      rwNumber: (json['rw_number'] as num?)?.toInt() ?? 0,
      name: json['name'] as String? ?? '',
      headName: json['head_name'] as String?,
      headId: json['head_id'] as String?,
      address: json['address'] as String?,
      isActive: json['is_active'] as bool?,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'rw_number': rwNumber,
      'name': name,
      'head_name': headName,
      'head_id': headId,
      'is_active': isActive,
    };
  }
}

// =============================================================================
// PR 4 RW Aggregated Data Models
// =============================================================================


// ---------------------------------------------------------------------------
// Dashboard — aggregated across all RTs in the RW
// ---------------------------------------------------------------------------

/// Dashboard summary dari seluruh RT di bawah RW.
/// STRUKTUR BERBEDA dari DashboardSummary (RT):
///   - TIDAK ada chart, dues_progress, payment_method_breakdown
///   - Flat aggregate: total_rts, total_locations, total_balance, dll
class RwDashboardSummary {
  final int totalRts;
  final int totalLocations;
  final String totalOpeningBalance;
  final String totalBalance;
  final String totalIncome;
  final String totalExpense;
  final int transactionCount;

  const RwDashboardSummary({
    required this.totalRts,
    required this.totalLocations,
    this.totalOpeningBalance = '0.00',
    required this.totalBalance,
    required this.totalIncome,
    required this.totalExpense,
    required this.transactionCount,
  });

  factory RwDashboardSummary.fromJson(Map<String, dynamic> json) {
    return RwDashboardSummary(
      totalRts: (json['total_rts'] as num?)?.toInt() ?? 0,
      totalLocations: (json['total_locations'] as num?)?.toInt() ?? 0,
      totalOpeningBalance: json['total_opening_balance'] as String? ?? '0.00',
      totalBalance: json['net_balance'] as String? ?? json['total_balance'] as String? ?? '0.00',
      totalIncome: json['total_income'] as String? ?? '0.00',
      totalExpense: json['total_expense'] as String? ?? '0.00',
      transactionCount: (json['transaction_count'] as num?)?.toInt() ?? 0,
    );
  }
}

// ---------------------------------------------------------------------------
// KAS — RW Ledger Financial Report
// ---------------------------------------------------------------------------

/// KAS report dari ledger mandiri RW.
class RwKasReport {
  final String openingBalance;
  final String totalIncome;
  final String totalExpense;
  final String netBalance;
  final int transactionCount;

  const RwKasReport({
    this.openingBalance = '0.00',
    required this.totalIncome,
    required this.totalExpense,
    required this.netBalance,
    required this.transactionCount,
  });

  factory RwKasReport.fromJson(Map<String, dynamic> json) {
    return RwKasReport(
      openingBalance: json['opening_balance'] as String? ?? '0.00',
      totalIncome: json['total_income'] as String? ?? '0.00',
      totalExpense: json['total_expense'] as String? ?? '0.00',
      netBalance: json['net_balance'] as String? ?? '0.00',
      transactionCount: (json['transaction_count'] as num?)?.toInt() ?? 0,
    );
  }
}

// ---------------------------------------------------------------------------
// Transaction — from an RT within the RW
// ---------------------------------------------------------------------------

/// Satu transaksi dari RT tertentu di bawah RW.
class RwTransaction {
  final String id;
  final String rwId;
  final String rtId;
  final String rtName;
  final String type; // 'income' | 'expense'
  final String amount;
  final String description;
  final String? categoryId;
  final String? categoryName;
  final String? locationId;
  final String? locationName;
  final String? sourceRtId;
  final String? sourceRtName;
  final String status;
  final String? paymentId;
  final String? paymentMethod;
  final String? proofPath;
  final String? proofUrl;
  final DateTime occurredAt;
  final DateTime createdAt;

  const RwTransaction({
    required this.id,
    this.rwId = '',
    required this.rtId,
    required this.rtName,
    required this.type,
    required this.amount,
    required this.description,
    this.categoryId,
    this.categoryName,
    this.locationId,
    this.locationName,
    this.sourceRtId,
    this.sourceRtName,
    required this.status,
    this.paymentId,
    this.paymentMethod,
    this.proofPath,
    this.proofUrl,
    required this.occurredAt,
    required this.createdAt,
  });

  factory RwTransaction.fromJson(Map<String, dynamic> json) {
    DateTime parseDate(String? s) {
      if (s == null || s.isEmpty) return DateTime.now();
      try {
        return DateTime.parse(s);
      } catch (_) {
        return DateTime.now();
      }
    }

    return RwTransaction(
      id: json['id'] as String? ?? '',
      rwId: json['rw_id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      rtName: json['rt_name'] as String? ?? '',
      type: json['type'] as String? ?? '',
      amount: json['amount'] as String? ?? '0',
      description: json['description'] as String? ?? '',
      categoryId: json['category_id'] as String?,
      categoryName: json['category_name'] as String?,
      locationId: json['location_id'] as String?,
      locationName: json['location_name'] as String?,
      sourceRtId: json['source_rt_id'] as String?,
      sourceRtName: json['source_rt_name'] as String?,
      status: json['status'] as String? ?? '',
      paymentId: json['payment_id'] as String?,
      paymentMethod: json['payment_method'] as String?,
      proofPath: json['proof_path'] as String?,
      proofUrl: json['proof_url'] as String?,
      occurredAt: parseDate(json['occurred_at'] as String?),
      createdAt: parseDate(json['created_at'] as String?),
    );
  }
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

class RwPaginationInfo {
  final int page;
  final int pageSize;
  final int total;
  final int totalPages;

  const RwPaginationInfo({
    required this.page,
    required this.pageSize,
    required this.total,
    required this.totalPages,
  });

  factory RwPaginationInfo.fromJson(Map<String, dynamic> json) {
    return RwPaginationInfo(
      page: (json['page'] as num?)?.toInt() ?? 1,
      pageSize: (json['page_size'] as num?)?.toInt() ?? 20,
      total: (json['total'] as num?)?.toInt() ?? 0,
      totalPages: (json['total_pages'] as num?)?.toInt() ?? 1,
    );
  }
}

class RwTransactionsResponse {
  final List<RwTransaction> data;
  final RwPaginationInfo pagination;

  const RwTransactionsResponse({
    required this.data,
    required this.pagination,
  });

  factory RwTransactionsResponse.fromJson(Map<String, dynamic> json) {
    final rawList = json['data'] as List<dynamic>?;
    final items = <RwTransaction>[];
    if (rawList != null) {
      for (final item in rawList) {
        if (item is Map<String, dynamic>) {
          items.add(RwTransaction.fromJson(item));
        }
      }
    }

    final pagRaw = json['pagination'] as Map<String, dynamic>?;
    final pag = pagRaw != null
        ? RwPaginationInfo.fromJson(pagRaw)
        : const RwPaginationInfo(page: 1, pageSize: 20, total: 0, totalPages: 0);

    return RwTransactionsResponse(data: items, pagination: pag);
  }
}

// ---------------------------------------------------------------------------
// Fund Location — from an RT within the RW
// ---------------------------------------------------------------------------

/// Satu lokasi dana dari RT tertentu di bawah RW.
class RwFundLocation {
  final String id;
  final String rwId;
  final String rtId;
  final String rtName;
  final String name;
  final String category; // 'rekening' | 'brankas' | 'coh'
  final String? bankName;
  final String? accountNumber;
  final String? accountHolder;
  final bool isActive;
  final String currentBalance;
  final DateTime createdAt;

  const RwFundLocation({
    required this.id,
    this.rwId = '',
    this.rtId = '',
    this.rtName = '',
    required this.name,
    required this.category,
    this.bankName,
    this.accountNumber,
    this.accountHolder,
    required this.isActive,
    required this.currentBalance,
    required this.createdAt,
  });

  factory RwFundLocation.fromJson(Map<String, dynamic> json) {
    DateTime parseDate(String? s) {
      if (s == null || s.isEmpty) return DateTime.now();
      try {
        return DateTime.parse(s);
      } catch (_) {
        return DateTime.now();
      }
    }

    return RwFundLocation(
      id: json['id'] as String? ?? '',
      rwId: json['rw_id'] as String? ?? '',
      rtId: json['rt_id'] as String? ?? '',
      rtName: json['rt_name'] as String? ?? '',
      name: json['name'] as String? ?? '',
      category: json['category'] as String? ?? '',
      bankName: json['bank_name'] as String?,
      accountNumber: json['account_number'] as String?,
      accountHolder: json['account_holder'] as String?,
      isActive: json['is_active'] as bool? ?? false,
      currentBalance: json['current_balance'] as String? ?? '0.00',
      createdAt: parseDate(json['created_at'] as String?),
    );
  }
}

// ---------------------------------------------------------------------------
// Fund Transfers — between locations within the RW
// ---------------------------------------------------------------------------

class RwFundTransfer {
  final String id;
  final String rwId;
  final String fromLocationId;
  final String? fromLocationName;
  final String toLocationId;
  final String? toLocationName;
  final String amount;
  final String transferDate;
  final String? notes;
  final String? createdBy;
  final String? reversesTransferId;
  final bool isReversal;
  final DateTime createdAt;

  const RwFundTransfer({
    required this.id,
    required this.rwId,
    required this.fromLocationId,
    this.fromLocationName,
    required this.toLocationId,
    this.toLocationName,
    required this.amount,
    required this.transferDate,
    this.notes,
    this.createdBy,
    this.reversesTransferId,
    required this.isReversal,
    required this.createdAt,
  });

  factory RwFundTransfer.fromJson(Map<String, dynamic> json) {
    DateTime parseDate(String? s) {
      if (s == null || s.isEmpty) return DateTime.now();
      try {
        return DateTime.parse(s);
      } catch (_) {
        return DateTime.now();
      }
    }

    return RwFundTransfer(
      id: json['id'] as String? ?? '',
      rwId: json['rw_id'] as String? ?? '',
      fromLocationId: json['from_location_id'] as String? ?? '',
      fromLocationName: json['from_location_name'] as String?,
      toLocationId: json['to_location_id'] as String? ?? '',
      toLocationName: json['to_location_name'] as String?,
      amount: json['amount'] as String? ?? '0.00',
      transferDate: json['transfer_date'] as String? ?? '',
      notes: json['notes'] as String?,
      createdBy: json['created_by'] as String?,
      reversesTransferId: json['reverses_transfer_id'] as String?,
      isReversal: json['is_reversal'] as bool? ?? false,
      createdAt: parseDate(json['created_at'] as String?),
    );
  }
}

// ---------------------------------------------------------------------------
// Fund Location Mutations (Ledger)
// ---------------------------------------------------------------------------

class RwLocationMutation {
  final String id;
  final String rwId;
  final String locationId;
  final String? transactionId;
  final String? transferId;
  final String changeType;
  final String amountDelta;
  final String? notes;
  final String? createdBy;
  final DateTime createdAt;

  const RwLocationMutation({
    required this.id,
    required this.rwId,
    required this.locationId,
    this.transactionId,
    this.transferId,
    required this.changeType,
    required this.amountDelta,
    this.notes,
    this.createdBy,
    required this.createdAt,
  });

  factory RwLocationMutation.fromJson(Map<String, dynamic> json) {
    DateTime parseDate(String? s) {
      if (s == null || s.isEmpty) return DateTime.now();
      try {
        return DateTime.parse(s);
      } catch (_) {
        return DateTime.now();
      }
    }

    return RwLocationMutation(
      id: json['id'] as String? ?? '',
      rwId: json['rw_id'] as String? ?? '',
      locationId: json['location_id'] as String? ?? '',
      transactionId: json['transaction_id'] as String?,
      transferId: json['transfer_id'] as String?,
      changeType: json['change_type'] as String? ?? '',
      amountDelta: json['amount_delta'] as String? ?? '0.00',
      notes: json['notes'] as String?,
      createdBy: json['created_by'] as String?,
      createdAt: parseDate(json['created_at'] as String?),
    );
  }
}

// ---------------------------------------------------------------------------
// Reconciliation Report
// ---------------------------------------------------------------------------

class RwCategorySummary {
  final String category;
  final String totalBalance;
  final int locationCount;

  const RwCategorySummary({
    required this.category,
    required this.totalBalance,
    required this.locationCount,
  });

  factory RwCategorySummary.fromJson(Map<String, dynamic> json) {
    return RwCategorySummary(
      category: json['category'] as String? ?? '',
      totalBalance: json['total_balance'] as String? ?? '0.00',
      locationCount: (json['locations_count'] as num?)?.toInt() ??
          (json['location_count'] as num?)?.toInt() ?? 0,
    );
  }
}

class RwReconciliationReport {
  final String openingBalance;
  final String totalIncome;
  final String totalExpense;
  final String calculatedBalance;
  final String totalLocationBalances;
  final bool isReconciled;
  final String discrepancy;
  final List<RwCategorySummary> categoryBreakdown;

  const RwReconciliationReport({
    required this.openingBalance,
    required this.totalIncome,
    required this.totalExpense,
    required this.calculatedBalance,
    required this.totalLocationBalances,
    required this.isReconciled,
    required this.discrepancy,
    required this.categoryBreakdown,
  });

  factory RwReconciliationReport.fromJson(Map<String, dynamic> json) {
    final list = (json['locations_summary'] ?? json['category_breakdown']) as List?;
    final categories = <RwCategorySummary>[];
    if (list != null) {
      for (final item in list) {
        if (item is Map<String, dynamic>) {
          categories.add(RwCategorySummary.fromJson(item));
        }
      }
    }
    final status = json['status'] as String? ?? '';
    final isReconciled = status == 'matched' || (json['is_reconciled'] as bool? ?? false);
    final calculatedBal = json['kas_balance'] as String? ?? json['calculated_balance'] as String? ?? '0.00';
    final totalAllocated = json['total_allocated_balance'] as String? ?? json['total_location_balances'] as String? ?? '0.00';

    return RwReconciliationReport(
      openingBalance: json['opening_balance'] as String? ?? '0.00',
      totalIncome: json['total_income'] as String? ?? '0.00',
      totalExpense: json['total_expense'] as String? ?? '0.00',
      calculatedBalance: calculatedBal,
      totalLocationBalances: totalAllocated,
      isReconciled: isReconciled,
      discrepancy: json['discrepancy'] as String? ?? '0.00',
      categoryBreakdown: categories,
    );
  }
}
