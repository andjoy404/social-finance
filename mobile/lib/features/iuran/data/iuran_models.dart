import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';

/// Enum representing the status of an iuran (dues) bill.
enum IuranStatus { belumBayar, sebagian, lunas, sukarela }

/// Iuran bill model for the IURAN feature.
class IuranBill {
  final String id;
  final String householdName;
  final String iuranType;
  final String periode;
  final int nominal; // amount in rupiah (int, no floating point)
  final int paidAmount;
  final int pendingAmount;
  final int rejectedAmount;
  final IuranStatus status;
  final bool isVoluntary;

  /// True when the bill's due uses payment_rule "full": the backend only
  /// accepts a payment equal to the remaining balance.
  final bool requiresFullPayment;

  /// When isVoluntary==true, optional definition data from BackendDue.
  final String? dueName;
  final String? duePeriodType;
  final String? dueRtCode;
  final bool? dueIsVoluntary;
  final String? dueId;

  const IuranBill({
    required this.id,
    required this.householdName,
    required this.iuranType,
    required this.periode,
    required this.nominal,
    this.paidAmount = 0,
    this.pendingAmount = 0,
    this.rejectedAmount = 0,
    this.status = IuranStatus.belumBayar,
    this.isVoluntary = false,
    this.requiresFullPayment = false,
    this.dueName,
    this.duePeriodType,
    this.dueRtCode,
    this.dueIsVoluntary,
    this.dueId,
  });

  /// Returns true if this bill represents voluntary dues (Sukarela).
  bool get isVoluntaryDues =>
      isVoluntary ||
      status == IuranStatus.sukarela ||
      nominal == 0 ||
      (dueIsVoluntary ?? false);

  /// Returns true if this bill represents regular dues.
  bool get isRegularDues => !isVoluntaryDues;

  /// Returns true if this bill requires full payment.
  bool get isFullPayment => requiresFullPayment;

  /// Returns true if the nominal amount is locked for this bill.
  /// Nominal is locked ONLY when the dues is Regular AND payment method is Full.
  bool get isAmountLocked => isRegularDues && isFullPayment;

  /// Returns the remaining balance for display purposes.
  /// Calculated as nominal minus APPROVED payments only.
  /// Pending and rejected payments do NOT reduce the displayed balance.
  int get remainingAmount =>
      nominal - paidAmount < 0 ? 0 : nominal - paidAmount;

  /// Returns the maximum amount that can still be paid.
  /// Accounts for approved and pending payments to prevent overpayment.
  /// Rejected payments do NOT affect capacity.
  int get paymentCapacity => nominal - paidAmount - pendingAmount < 0
      ? 0
      : nominal - paidAmount - pendingAmount;

  /// Returns true if this bill is in arrears (not yet paid or partially paid).
  /// Voluntary Rp0 items are never in arrears.
  bool get isInArrears =>
      status != IuranStatus.lunas && status != IuranStatus.sukarela;
}

/// Format a nominal amount in rupiah to Indonesian currency string.
/// e.g. 50000 → "Rp50.000", 1500000 → "Rp1.500.000"
///
/// Shared Iuran money formatter — delegates to [RupiahFormatter.formatIdr].
String formatRupiah(int amount) => RupiahFormatter.formatIdr(amount);

/// Format a backend NUMERIC money string (e.g. "50000.00") as "Rp50.000".
String formatRupiahString(String amount) =>
    RupiahFormatter.formatMoneyString(amount);

/// Convert IuranStatus enum to a display label.
String getStatusLabel(IuranStatus status) {
  return switch (status) {
    IuranStatus.belumBayar => 'Belum Bayar',
    IuranStatus.sebagian => 'Sebagian',
    IuranStatus.lunas => 'Lunas',
    IuranStatus.sukarela => 'Sukarela',
  };
}

/// Return the color associated with the given status.
Color getStatusColor(IuranStatus status, BuildContext context) {
  return switch (status) {
    IuranStatus.belumBayar => AppColors.danger,
    IuranStatus.sebagian => AppColors.warning,
    IuranStatus.lunas => AppColors.success,
    IuranStatus.sukarela => AppColors.info,
  };
}

/// Convert a status string from API to enum (for future API integration).
IuranStatus parseStatus(String value) {
  return switch (value.toLowerCase()) {
    'belum_bayar' || 'belum bayar' => IuranStatus.belumBayar,
    'sebagian' => IuranStatus.sebagian,
    'lunas' => IuranStatus.lunas,
    'sukarela' => IuranStatus.sukarela,
    _ => IuranStatus.belumBayar,
  };
}
