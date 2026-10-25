import 'package:flutter/foundation.dart';

/// Categories of fund locations.
enum FundLocationCategory {
  rekening,
  brankas,
  coh;

  String get value => name;

  String get label => switch (this) {
    FundLocationCategory.rekening => 'Rekening Bank',
    FundLocationCategory.brankas => 'Brankas Fisik',
    FundLocationCategory.coh => 'Cash on Hand (COH)',
  };

  static FundLocationCategory fromString(String val) {
    switch (val.toLowerCase().trim()) {
      case 'rekening':
        return FundLocationCategory.rekening;
      case 'brankas':
        return FundLocationCategory.brankas;
      case 'coh':
        return FundLocationCategory.coh;
      default:
        return FundLocationCategory.coh;
    }
  }
}

@immutable
class FundLocation {
  final String id;
  final String rtId;
  final String name;
  final FundLocationCategory category;
  final String? bankName;
  final String? accountNumber;
  final String? accountHolder;
  final bool isActive;
  final String currentBalance; // numeric string e.g. "1500000.00"
  final DateTime createdAt;
  final DateTime updatedAt;

  const FundLocation({
    required this.id,
    required this.rtId,
    required this.name,
    required this.category,
    this.bankName,
    this.accountNumber,
    this.accountHolder,
    required this.isActive,
    required this.currentBalance,
    required this.createdAt,
    required this.updatedAt,
  });

  /// Parse balance as integer rupiah (for display and comparison)
  int get balanceRupiah {
    final parsed = double.tryParse(currentBalance);
    return parsed != null ? parsed.round() : 0;
  }

  factory FundLocation.fromJson(Map<String, dynamic> json) {
    return FundLocation(
      id: json['id'] as String,
      rtId: json['rt_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      category: FundLocationCategory.fromString(json['category'] as String? ?? ''),
      bankName: json['bank_name'] as String?,
      accountNumber: json['account_number'] as String?,
      accountHolder: json['account_holder'] as String?,
      isActive: json['is_active'] as bool? ?? true,
      currentBalance: json['current_balance']?.toString() ?? '0.00',
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
      updatedAt: json['updated_at'] != null
          ? DateTime.tryParse(json['updated_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
    );
  }
}

@immutable
class FundTransfer {
  final String id;
  final String rtId;
  final String fromLocationId;
  final String toLocationId;
  final String amount;
  final DateTime transferDate;
  final String? notes;
  final String? reversesTransferId;
  final String? createdBy;
  final DateTime createdAt;
  final String? fromLocationName;
  final String? toLocationName;
  final String? createdByName;

  const FundTransfer({
    required this.id,
    required this.rtId,
    required this.fromLocationId,
    required this.toLocationId,
    required this.amount,
    required this.transferDate,
    this.notes,
    this.reversesTransferId,
    this.createdBy,
    required this.createdAt,
    this.fromLocationName,
    this.toLocationName,
    this.createdByName,
  });

  int get amountRupiah {
    final parsed = double.tryParse(amount);
    return parsed != null ? parsed.round() : 0;
  }

  bool get isReversal => reversesTransferId != null && reversesTransferId!.isNotEmpty;

  factory FundTransfer.fromJson(Map<String, dynamic> json) {
    return FundTransfer(
      id: json['id'] as String,
      rtId: json['rt_id'] as String? ?? '',
      fromLocationId: json['from_location_id'] as String? ?? '',
      toLocationId: json['to_location_id'] as String? ?? '',
      amount: json['amount']?.toString() ?? '0.00',
      transferDate: json['transfer_date'] != null
          ? DateTime.tryParse(json['transfer_date'].toString()) ?? DateTime.now()
          : DateTime.now(),
      notes: json['notes'] as String?,
      reversesTransferId: json['reverses_transfer_id'] as String?,
      createdBy: json['created_by'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
      fromLocationName: json['from_location_name'] as String?,
      toLocationName: json['to_location_name'] as String?,
      createdByName: json['created_by_name'] as String?,
    );
  }
}

@immutable
class LocationBalanceChange {
  final String id;
  final String locationId;
  final String changeType;
  final String amountDelta;
  final String balanceAfter;
  final String referenceType;
  final String? referenceId;
  final String? notes;
  final DateTime createdAt;

  const LocationBalanceChange({
    required this.id,
    required this.locationId,
    required this.changeType,
    required this.amountDelta,
    required this.balanceAfter,
    required this.referenceType,
    this.referenceId,
    this.notes,
    required this.createdAt,
  });

  int get deltaRupiah {
    final parsed = double.tryParse(amountDelta);
    return parsed != null ? parsed.round() : 0;
  }

  int get balanceAfterRupiah {
    final parsed = double.tryParse(balanceAfter);
    return parsed != null ? parsed.round() : 0;
  }

  String get changeTypeLabel => switch (changeType) {
    'opening_balance' => 'Saldo Awal',
    'income' => 'Pemasukan Kas',
    'expense' => 'Pengeluaran Kas',
    'transfer_out' => 'Transfer Keluar',
    'transfer_in' => 'Transfer Masuk',
    'reversal_transfer_out' => 'Batal Transfer Keluar',
    'reversal_transfer_in' => 'Batal Transfer Masuk',
    'adjustment' => 'Penyesuaian',
    _ => changeType,
  };

  factory LocationBalanceChange.fromJson(Map<String, dynamic> json) {
    return LocationBalanceChange(
      id: json['id'] as String,
      locationId: json['location_id'] as String? ?? '',
      changeType: json['change_type'] as String? ?? '',
      amountDelta: json['amount_delta']?.toString() ?? '0.00',
      balanceAfter: json['balance_after']?.toString() ?? '0.00',
      referenceType: json['reference_type'] as String? ?? '',
      referenceId: json['reference_id'] as String?,
      notes: json['notes'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.tryParse(json['created_at'].toString()) ?? DateTime.now()
          : DateTime.now(),
    );
  }
}

@immutable
class FundLocationReconciliationReport {
  final String rtId;
  final String kasBalance;
  final String totalAllocatedBalance;
  final String unallocatedBalance;
  final String discrepancy;
  final String status;
  final DateTime asOf;
  final bool anomaliesDetected;

  const FundLocationReconciliationReport({
    required this.rtId,
    required this.kasBalance,
    required this.totalAllocatedBalance,
    required this.unallocatedBalance,
    required this.discrepancy,
    required this.status,
    required this.asOf,
    required this.anomaliesDetected,
  });

  int get kasBalanceRupiah => (double.tryParse(kasBalance) ?? 0).round();
  int get allocatedBalanceRupiah => (double.tryParse(totalAllocatedBalance) ?? 0).round();
  int get unallocatedBalanceRupiah => (double.tryParse(unallocatedBalance) ?? 0).round();
  int get discrepancyRupiah => (double.tryParse(discrepancy) ?? 0).round();

  bool get isBalanced => status == 'Cocok' || status == 'Cocok dengan catatan';

  factory FundLocationReconciliationReport.fromJson(Map<String, dynamic> json) {
    return FundLocationReconciliationReport(
      rtId: json['rt_id'] as String? ?? '',
      kasBalance: json['kas_balance']?.toString() ?? '0.00',
      totalAllocatedBalance: json['total_allocated_balance']?.toString() ?? '0.00',
      unallocatedBalance: json['unallocated_balance']?.toString() ?? '0.00',
      discrepancy: json['discrepancy']?.toString() ?? '0.00',
      status: json['status'] as String? ?? 'Cocok',
      asOf: json['as_of'] != null
          ? DateTime.tryParse(json['as_of'].toString()) ?? DateTime.now()
          : DateTime.now(),
      anomaliesDetected: json['anomalies_detected'] as bool? ?? false,
    );
  }
}
