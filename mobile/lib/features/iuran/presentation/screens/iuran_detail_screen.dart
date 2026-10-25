import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';

import '../../data/iuran_models.dart';
import '../../data/iuran_providers.dart';
import '../../data/iuran_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import '../widgets/payment_proof_thumbnail.dart';

/// Detail screen for a single iuran bill.
class IuranDetailScreen extends ConsumerStatefulWidget {
  final String billId;
  final String? initialPaymentId;

  const IuranDetailScreen({
    super.key,
    required this.billId,
    this.initialPaymentId,
  });

  @override
  ConsumerState<IuranDetailScreen> createState() => _IuranDetailScreenState();
}

class _IuranDetailScreenState extends ConsumerState<IuranDetailScreen> {
  String get _billId => widget.billId;
  IuranBill? _cachedBill;
  bool _autoOpenedPayment = false;

  void _checkAutoOpenPayment(List<BackendPayment> payments) {
    if (_autoOpenedPayment || widget.initialPaymentId == null) return;
    if (payments.isEmpty) return;

    _autoOpenedPayment = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      BackendPayment? target;
      try {
        target = payments.firstWhere((p) => p.id == widget.initialPaymentId);
      } catch (_) {
        target = payments.first;
      }
      if (mounted) {
        _showPaymentDetailDialog(context, target);
      }
    });
  }

  Future<void> _onRefresh() async {
    try {
      ref.invalidate(billDetailProvider(_billId));
      ref.invalidate(billPaymentsProvider(_billId));
      ref.invalidate(pendingPaymentsProvider(true));
      await Future.wait([
        ref.read(billDetailProvider(_billId).future),
        ref.read(billPaymentsProvider(_billId).future),
      ]);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gagal memperbarui data: $e'),
            behavior: SnackBarBehavior.floating,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final accentSoft = AppColors.accentSoftColor(context);

    // Fetch bill detail from provider (computed paidAmount from approved payments)
    final billState = ref.watch(billDetailProvider(_billId));
    if (billState.hasValue && billState.value != null) {
      _cachedBill = billState.value;
    }

    final currentBill = _cachedBill ?? billState.valueOrNull;

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
          tooltip: 'Kembali',
        ),
        title: const MenuAppBarTitle(title: 'Detail Tagihan'),
      ),
      floatingActionButton:
          currentBill != null &&
              (currentBill.status != IuranStatus.lunas ||
                  currentBill.isVoluntary)
          ? _buildAndroidFAB(context, accentSoft)
          : null,
      body: currentBill != null
          ? _buildBillDetail(context, theme, isDark, currentBill, accentSoft)
          : (billState.isLoading
                ? const Center(child: CircularProgressIndicator())
                : (billState.hasError
                      ? RefreshIndicator(
                          onRefresh: _onRefresh,
                          child: Center(
                            child: SingleChildScrollView(
                              physics: const AlwaysScrollableScrollPhysics(),
                              padding: const EdgeInsets.all(AppSpacing.lg),
                              child: Column(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(
                                    Icons.error_outline,
                                    size: 64,
                                    color: AppColors.warning,
                                  ),
                                  const SizedBox(height: AppSpacing.md),
                                  Text(
                                    'Gagal Memuat Data',
                                    style: theme.textTheme.titleLarge,
                                  ),
                                  const SizedBox(height: AppSpacing.sm),
                                  Text(
                                    billState.error.toString(),
                                    textAlign: TextAlign.center,
                                    style: theme.textTheme.bodyMedium?.copyWith(
                                      color: theme.colorScheme.onSurfaceVariant,
                                    ),
                                  ),
                                  const SizedBox(height: AppSpacing.lg),
                                  ElevatedButton.icon(
                                    onPressed: _onRefresh,
                                    icon: const Icon(Icons.refresh),
                                    label: const Text('Coba Lagi'),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        )
                      : RefreshIndicator(
                          onRefresh: _onRefresh,
                          child: Center(
                            child: SingleChildScrollView(
                              physics: const AlwaysScrollableScrollPhysics(),
                              padding: const EdgeInsets.all(AppSpacing.lg),
                              child: EmptyState(
                                icon: Icons.receipt_long_outlined,
                                title: 'Tagihan Tidak Ditemukan',
                                description: 'Detail tagihan tidak tersedia.',
                                actionText: 'Kembali',
                                onAction: () => context.pop(),
                              ),
                            ),
                          ),
                        ))),
    );
  }

  Widget _buildBillDetail(
    BuildContext context,
    ThemeData theme,
    bool isDark,
    IuranBill bill,
    Color accentSoft,
  ) {
    return RefreshIndicator(
      onRefresh: _onRefresh,
      child: SingleChildScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(AppSpacing.base),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Status Header ──
            AppCard(
              useEmphasis: true,
              padding: const EdgeInsets.all(AppSpacing.lg),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.receipt_long,
                        size: 24,
                        color: AppColors.accent,
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Text(
                          bill.householdName,
                          style: theme.textTheme.headlineSmall?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                      AppBadge(
                        label: getStatusLabel(bill.status),
                        textColor: getStatusColor(bill.status, context),
                        backgroundColor: getStatusColor(
                          bill.status,
                          context,
                        ).withValues(alpha: isDark ? 0.18 : 0.10),
                      ),
                    ],
                  ),
                  // ── Progress bar for partial payment (in header card) ──
                  if (bill.status == IuranStatus.sebagian) ...[
                    const SizedBox(height: AppSpacing.md),
                    _buildProgressBar(bill),
                  ],
                ],
              ),
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Bill Details ──
            SectionHeader(title: 'Informasi Tagihan'),
            const SizedBox(height: AppSpacing.sm),

            AppCard(
              padding: const EdgeInsets.all(AppSpacing.base),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildDetailRow(
                    context,
                    icon: Icons.home_outlined,
                    label: 'Rumah',
                    value: bill.householdName,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  _buildDetailRow(
                    context,
                    icon: Icons.category_outlined,
                    label: 'Jenis Iuran',
                    value: bill.iuranType,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  _buildDetailRow(
                    context,
                    icon: Icons.calendar_today_outlined,
                    label: 'Periode',
                    trailing: _buildPeriodeBadge(bill.periode),
                  ),
                ],
              ),
            ),

            const SizedBox(height: AppSpacing.lg),

            // ── Payment Details ──
            SectionHeader(title: 'Detail Pembayaran'),
            const SizedBox(height: AppSpacing.sm),

            AppCard(
              padding: const EdgeInsets.all(AppSpacing.base),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildPaymentRow(
                    context,
                    icon: Icons.account_balance_wallet_outlined,
                    label: 'Nominal Tagihan',
                    value: formatRupiah(bill.nominal),
                    highlight: true,
                  ),
                  const Divider(height: 24),
                  _buildPaymentRow(
                    context,
                    icon: Icons.check_circle_outline,
                    label: 'Jumlah Dibayar',
                    value: formatRupiah(bill.paidAmount),
                    valueColor: AppColors.success,
                  ),
                  const SizedBox(height: AppSpacing.md),
                  if (bill.isVoluntary) ...[
                    _buildPaymentRow(
                      context,
                      icon: Icons.volunteer_activism_outlined,
                      label: 'Status',
                      value: 'Sukarela',
                      valueColor: AppColors.info,
                    ),
                  ] else if (bill.isInArrears) ...[
                    _buildPaymentRow(
                      context,
                      icon: Icons.warning_outlined,
                      label: 'Sisa Tagihan',
                      value: formatRupiah(bill.remainingAmount),
                      valueColor: AppColors.warning,
                      highlight: true,
                    ),
                  ] else ...[
                    _buildPaymentRow(
                      context,
                      icon: Icons.celebration_outlined,
                      label: 'Status',
                      value: 'Sudah Lunas',
                      valueColor: AppColors.success,
                    ),
                  ],
                ],
              ),
            ),

            const SizedBox(height: AppSpacing.lg),
            SectionHeader(title: 'Riwayat Pembayaran'),
            const SizedBox(height: AppSpacing.sm),
            ref
                .watch(billPaymentsProvider(bill.id))
                .when(
                  data: (payments) {
                    _checkAutoOpenPayment(payments);
                    if (payments.isEmpty) {
                      return AppCard(
                        padding: const EdgeInsets.all(AppSpacing.base),
                        child: Center(
                          child: Text(
                            'Belum ada riwayat pembayaran.',
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: isDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                            ),
                          ),
                        ),
                      );
                    }
                    return AppCard(
                      padding: const EdgeInsets.all(AppSpacing.base),
                      child: Column(
                        children: payments.map((p) {
                          return InkWell(
                            onTap: () => _showPaymentDetailDialog(context, p),
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            child: Padding(
                              padding: const EdgeInsets.symmetric(
                                vertical: AppSpacing.xs,
                              ),
                              child: Row(
                                children: [
                                  Icon(
                                    p.status == PaymentStatus.approved
                                        ? Icons.check_circle_outline
                                        : p.status == PaymentStatus.rejected
                                        ? Icons.cancel_outlined
                                        : Icons.access_time_outlined,
                                    size: 20,
                                    color: p.status == PaymentStatus.approved
                                        ? AppColors.success
                                        : p.status == PaymentStatus.rejected
                                        ? AppColors.danger
                                        : AppColors.warning,
                                  ),
                                  const SizedBox(width: AppSpacing.sm),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Row(
                                          children: [
                                            Flexible(
                                              child: Text(
                                                'Rp ${_formatAmount(p.amount)} · ${_formatMethod(p.method)}${p.submitterName != null && p.submitterName!.isNotEmpty ? ' · ${p.submitterName}' : ''}',
                                                style: theme
                                                    .textTheme
                                                    .bodyMedium
                                                    ?.copyWith(
                                                      fontWeight:
                                                          FontWeight.w600,
                                                    ),
                                                overflow: TextOverflow.ellipsis,
                                                maxLines: 1,
                                              ),
                                            ),
                                            if (p.paymentNo != null &&
                                                p.paymentNo!.isNotEmpty) ...[
                                              const SizedBox(
                                                width: AppSpacing.xs,
                                              ),
                                              Flexible(
                                                child: Text(
                                                  p.paymentNo!,
                                                  style: theme
                                                      .textTheme
                                                      .bodySmall
                                                      ?.copyWith(
                                                        fontWeight:
                                                            FontWeight.w500,
                                                        color: isDark
                                                            ? AppColors
                                                                  .darkTextMuted
                                                            : AppColors
                                                                  .lightTextMuted,
                                                      ),
                                                  overflow:
                                                      TextOverflow.ellipsis,
                                                  maxLines: 1,
                                                ),
                                              ),
                                            ],
                                          ],
                                        ),
                                        Text(
                                          '${_formatDate(p.paidAt)} · ${p.status.name.toUpperCase()}',
                                          style: theme.textTheme.bodySmall
                                              ?.copyWith(
                                                color: isDark
                                                    ? AppColors.darkTextMuted
                                                    : AppColors.lightTextMuted,
                                              ),
                                        ),
                                      ],
                                    ),
                                  ),
                                  if (p.proofPath != null &&
                                      p.proofPath!.isNotEmpty) ...[
                                    const SizedBox(width: AppSpacing.sm),
                                    PaymentProofThumbnail(
                                      paymentId: p.id,
                                      size: 40,
                                      label:
                                          'Bukti Pembayaran (${_formatDate(p.paidAt)})',
                                    ),
                                  ],
                                ],
                              ),
                            ),
                          );
                        }).toList(),
                      ),
                    );
                  },
                  loading: () => const Center(
                    child: Padding(
                      padding: EdgeInsets.all(AppSpacing.md),
                      child: CircularProgressIndicator(),
                    ),
                  ),
                  error: (err, _) => AppCard(
                    padding: const EdgeInsets.all(AppSpacing.base),
                    child: Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(
                            Icons.error_outline,
                            size: 32,
                            color: AppColors.warning,
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            'Gagal memuat riwayat pembayaran',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            err.toString(),
                            textAlign: TextAlign.center,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: isDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          TextButton.icon(
                            onPressed: () =>
                                ref.refresh(billPaymentsProvider(bill.id)),
                            icon: const Icon(Icons.refresh, size: 16),
                            label: const Text('Coba Lagi'),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),

            const SizedBox(height: AppSpacing.xxl),
          ], // Column children
        ), // Column
      ), // SingleChildScrollView
    ); // RefreshIndicator + _buildBillDetail
  }

  // ── UI Helpers ──

  Widget _buildNeonVerifyButton({
    required BuildContext context,
    required String label,
    required IconData icon,
    required Color neonColor,
    required Color softBg,
    required VoidCallback onPressed,
    EdgeInsetsGeometry? padding,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      decoration: BoxDecoration(
        color: softBg,
        borderRadius: BorderRadius.circular(AppRadius.base),
        border: Border.all(color: neonColor, width: 1.5),
        boxShadow: [
          BoxShadow(
            color: neonColor.withValues(alpha: isDark ? 0.35 : 0.22),
            blurRadius: 8,
            spreadRadius: 1,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onPressed,
          borderRadius: BorderRadius.circular(AppRadius.base),
          child: Padding(
            padding: padding ??
                const EdgeInsets.symmetric(
                  horizontal: AppSpacing.md,
                  vertical: 12,
                ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(icon, size: 18, color: neonColor),
                const SizedBox(width: AppSpacing.xs),
                Flexible(
                  child: Text(
                    label,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: neonColor,
                      letterSpacing: 0.3,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _approvePayment(
    BuildContext context,
    BackendPayment payment, {
    VoidCallback? onSuccess,
  }) async {
    final notesController = TextEditingController();
    final isCash = payment.method == PaymentMethod.cash;

    // Fetch eligible fund locations according to GAP B
    List<FundLocation> eligibleLocs = [];
    try {
      final locRepo = ref.read(fundLocationRepositoryProvider);
      final allLocs = await locRepo.fetchLocations(isActive: true);
      eligibleLocs = allLocs.where((l) {
        if (isCash) {
          return l.category == FundLocationCategory.coh ||
              l.category == FundLocationCategory.brankas;
        } else {
          return l.category == FundLocationCategory.rekening;
        }
      }).toList();
    } catch (_) {}

    String? selectedLocationId =
        eligibleLocs.length == 1 ? eligibleLocs.first.id : null;

    if (!context.mounted) return;

    showDialog(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            final hasEligible = eligibleLocs.isNotEmpty;
            final canApprove = hasEligible && selectedLocationId != null;

            return AlertDialog(
              title: const Text('Konfirmasi Persetujuan'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      'Apakah Anda yakin ingin menyetujui pembayaran ini?',
                      style: TextStyle(fontSize: 14),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    if (!hasEligible)
                      Container(
                        padding: const EdgeInsets.all(AppSpacing.sm),
                        margin: const EdgeInsets.only(bottom: AppSpacing.md),
                        decoration: BoxDecoration(
                          color: AppColors.danger.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                          border: Border.all(
                            color: AppColors.danger.withValues(alpha: 0.3),
                          ),
                        ),
                        child: Text(
                          'Tidak ada lokasi dana yang sesuai untuk metode ${isCash ? "Tunai (Kas Tunai/Brankas)" : "Transfer (Rekening Bank)"}. Tambahkan lokasi dana terlebih dahulu sebelum menyetujui.',
                          style: const TextStyle(
                            color: AppColors.danger,
                            fontSize: 12,
                          ),
                        ),
                      )
                    else ...[
                      DropdownButtonFormField<String>(
                        value: selectedLocationId,
                        decoration: const InputDecoration(
                          labelText: 'Lokasi Dana Tujuan (Wajib)',
                          border: OutlineInputBorder(),
                        ),
                        items: eligibleLocs.map((loc) {
                          return DropdownMenuItem<String>(
                            value: loc.id,
                            child: Text(
                              '${loc.name} (${loc.category.label})',
                            ),
                          );
                        }).toList(),
                        onChanged: (val) {
                          setDialogState(() => selectedLocationId = val);
                        },
                      ),
                      const SizedBox(height: AppSpacing.md),
                    ],
                    TextField(
                      controller: notesController,
                      maxLines: 3,
                      decoration: const InputDecoration(
                        labelText: 'Catatan Pengurus (opsional)',
                        border: OutlineInputBorder(),
                        alignLabelWithHint: true,
                      ),
                    ),
                  ],
                ),
              ),
              actionsPadding: const EdgeInsets.fromLTRB(
                AppSpacing.base,
                0,
                AppSpacing.base,
                AppSpacing.base,
              ),
              actions: [
                Row(
                  children: [
                    Expanded(
                      child: _buildNeonVerifyButton(
                        context: dialogContext,
                        label: 'Setuju',
                        icon: Icons.check_circle_outline,
                        neonColor: canApprove ? AppColors.success : Colors.grey,
                        softBg: canApprove
                            ? (Theme.of(dialogContext).brightness ==
                                    Brightness.dark
                                ? AppColors.darkSuccessSoft
                                : AppColors.lightSuccessSoft)
                            : Colors.grey.withValues(alpha: 0.1),
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.sm,
                          vertical: 12,
                        ),
                        onPressed: () async {
                          if (!canApprove) return;
                          final notes = notesController.text.trim();
                          Navigator.pop(dialogContext);

                          const snackBar = SnackBar(
                            content: Text('Memverifikasi pembayaran...'),
                            duration: Duration(seconds: 1),
                          );
                          ScaffoldMessenger.of(
                            context,
                          ).showSnackBar(snackBar);

                          try {
                            final repository = ref.read(
                              iuranRepositoryProvider,
                            );
                            await repository.verifyPayment(
                              payment.id,
                              action: 'approve',
                              officerNotes:
                                  notes.isNotEmpty ? notes : null,
                              locationId: selectedLocationId,
                            );

                            if (context.mounted) {
                              ScaffoldMessenger.of(
                                context,
                              ).hideCurrentSnackBar();
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text(
                                    'Pembayaran berhasil diverifikasi',
                                  ),
                                  backgroundColor: AppColors.success,
                                ),
                              );
                              ref.invalidate(
                                billDetailProvider(widget.billId),
                              );
                              ref.invalidate(
                                billPaymentsProvider(widget.billId),
                              );
                              ref.invalidate(pendingPaymentsProvider);
                              onSuccess?.call();
                            }
                          } catch (e) {
                            if (context.mounted) {
                              ScaffoldMessenger.of(
                                context,
                              ).hideCurrentSnackBar();
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text('Gagal memverifikasi: $e'),
                                  backgroundColor: AppColors.warning,
                                ),
                              );
                            }
                          }
                        },
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: _buildNeonVerifyButton(
                        context: dialogContext,
                        label: 'Batal',
                        icon: Icons.cancel_outlined,
                        neonColor: AppColors.danger,
                        softBg: Theme.of(dialogContext).brightness ==
                                Brightness.dark
                            ? AppColors.darkDangerSoft
                            : AppColors.lightDangerSoft,
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.sm,
                          vertical: 12,
                        ),
                        onPressed: () => Navigator.pop(dialogContext),
                      ),
                    ),
                  ],
                ),
              ],
            );
          },
        );
      },
    );
  }

  void _showRejectDialog(
    BuildContext context,
    String paymentId, {
    VoidCallback? onSuccess,
  }) {
    final reasonController = TextEditingController();
    final notesController = TextEditingController();
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          title: const Text('Konfirmasi Penolakan'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Apakah Anda yakin ingin menolak pembayaran ini?',
                  style: TextStyle(fontSize: 14),
                ),
                const SizedBox(height: AppSpacing.md),
                TextField(
                  controller: notesController,
                  maxLines: 3,
                  decoration: const InputDecoration(
                    labelText: 'Catatan Pengurus (opsional)',
                    border: OutlineInputBorder(),
                    alignLabelWithHint: true,
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                TextField(
                  controller: reasonController,
                  maxLines: 3,
                  decoration: const InputDecoration(
                    labelText: 'Alasan Penolakan',
                    border: OutlineInputBorder(),
                    alignLabelWithHint: true,
                  ),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('Batal'),
            ),
            ElevatedButton(
              onPressed: () async {
                final reason = reasonController.text.trim();
                if (reason.isEmpty) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(
                      content: Text('Alasan penolakan harus diisi'),
                      backgroundColor: AppColors.warning,
                    ),
                  );
                  return;
                }

                final notes = notesController.text.trim();
                Navigator.pop(dialogContext);

                final snackBar = SnackBar(
                  content: const Text('Memverifikasi pembayaran...'),
                  duration: const Duration(seconds: 1),
                );
                ScaffoldMessenger.of(context).showSnackBar(snackBar);

                try {
                  final repository = ref.read(iuranRepositoryProvider);
                  await repository.verifyPayment(
                    paymentId,
                    action: 'reject',
                    rejectionReason: reason,
                    officerNotes: notes.isNotEmpty ? notes : null,
                  );

                  if (context.mounted) {
                    ScaffoldMessenger.of(context).hideCurrentSnackBar();
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(
                        content: Text('Pembayaran ditolak'),
                        backgroundColor: AppColors.success,
                      ),
                    );
                    ref.invalidate(billDetailProvider(widget.billId));
                    ref.invalidate(billPaymentsProvider(widget.billId));
                    ref.invalidate(pendingPaymentsProvider);
                    onSuccess?.call();
                  }
                } catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).hideCurrentSnackBar();
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text('Gagal menolak: $e'),
                        backgroundColor: AppColors.warning,
                      ),
                    );
                  }
                }
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.warning,
                foregroundColor: Colors.white,
              ),
              child: const Text('Tolak'),
            ),
          ],
        );
      },
    );
  }

  String _formatAmount(String amount) {
    try {
      final parts = amount.split('.');
      final whole = int.tryParse(parts[0]) ?? 0;
      return whole.toString().replaceAllMapped(
        RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'),
        (Match m) => '${m[1]}.',
      );
    } catch (_) {
      return amount;
    }
  }

  String _formatDate(DateTime? date) {
    if (date == null) return '';
    return '${date.day} ${_monthName(date.month)} ${date.year}';
  }

  String _monthName(int month) {
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
    return months[(month - 1).clamp(0, 11)];
  }

  String _formatMethod(PaymentMethod? method) {
    switch (method) {
      case PaymentMethod.cash:
        return 'Cash';
      case PaymentMethod.transfer:
        return 'Transfer';
      default:
        return '';
    }
  }

  Widget _buildDetailRow(
    BuildContext context, {
    required IconData icon,
    required String label,
    String? value,
    Widget? trailing,
  }) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: AppColors.accent),
        const SizedBox(width: AppSpacing.sm),
        SizedBox(
          width: 120,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark
                  ? AppColors.darkTextMuted
                  : AppColors.lightTextMuted,
            ),
          ),
        ),
        Expanded(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (value != null) ...[
                Expanded(
                  child: Text(
                    value,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.w500,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                if (trailing != null) ...[
                  const SizedBox(width: AppSpacing.sm),
                  trailing,
                ],
              ] else if (trailing != null) ...[
                trailing,
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPeriodeBadge(String periode) {
    final theme = Theme.of(context);
    final parts = periode.split('-');
    if (parts.length != 2) return Text(periode);

    final year = parts[0];
    final monthIndex = int.tryParse(parts[1]) ?? 0;
    final monthName = _monthName(monthIndex);

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.sm,
        vertical: 2,
      ),
      decoration: BoxDecoration(
        color: AppColors.accent.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppRadius.base),
        border: Border.all(color: AppColors.accent.withValues(alpha: 0.3)),
      ),
      child: Text(
        '$monthName $year',
        style: theme.textTheme.labelLarge?.copyWith(
          fontWeight: FontWeight.w600,
          fontSize: 12,
          color: AppColors.accent,
        ),
      ),
    );
  }

  Widget _buildPaymentRow(
    BuildContext context, {
    required IconData icon,
    required String label,
    required String value,
    Color? valueColor,
    bool highlight = false,
    Widget? trailing,
  }) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: valueColor ?? AppColors.accent),
        const SizedBox(width: AppSpacing.sm),
        SizedBox(
          width: 120,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark
                  ? AppColors.darkTextMuted
                  : AppColors.lightTextMuted,
            ),
          ),
        ),
        Expanded(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              Text(
                value,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: highlight ? FontWeight.w700 : FontWeight.w500,
                  color:
                      valueColor ??
                      (isDark ? AppColors.darkText : AppColors.lightText),
                  fontSize: highlight ? 16 : null,
                ),
                overflow: TextOverflow.ellipsis,
              ),
              if (trailing != null) ...[
                const SizedBox(width: AppSpacing.sm),
                trailing,
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildProgressBar(IuranBill bill) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final pct = bill.paidAmount / bill.nominal;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              '${(pct * 100).toStringAsFixed(0)}%',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w600,
                color: AppColors.accent,
              ),
            ),
            Text(
              '${bill.paidAmount} / ${bill.nominal}',
              style: theme.textTheme.bodySmall?.copyWith(
                color: AppColors.lightTextMuted,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.sm),
        ClipRRect(
          borderRadius: BorderRadius.circular(AppRadius.xs),
          child: LinearProgressIndicator(
            value: pct,
            minHeight: 8,
            borderRadius: BorderRadius.circular(AppRadius.xs),
            backgroundColor: isDark
                ? AppColors.darkSurface
                : AppColors.lightSurfaceSubtle,
            valueColor: const AlwaysStoppedAnimation<Color>(AppColors.accent),
          ),
        ),
      ],
    );
  }

  Widget _buildDialogDetailRow(
    BuildContext context,
    bool isDark, {
    required IconData icon,
    required String label,
    required String value,
    Color? valueColor,
  }) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: valueColor ?? AppColors.accent),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  value,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                    color:
                        valueColor ??
                        (isDark ? AppColors.darkText : AppColors.lightText),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAndroidFAB(BuildContext context, Color accentSoft) {
    return Container(
      decoration: BoxDecoration(
        color: accentSoft,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: AppColors.accent, width: 1),
        boxShadow: [
          BoxShadow(
            color: AppColors.accent.withValues(alpha: 0.15),
            blurRadius: 8,
            offset: const Offset(0, 0),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        clipBehavior: Clip.none,
        child: InkWell(
          onTap: () => context.push('/home/iuran/pembayaran/$_billId'),
          borderRadius: BorderRadius.circular(24),
          child: const Padding(
            padding: EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.payment, size: 20, color: AppColors.accent),
                SizedBox(width: 6),
                Text(
                  'Bayar',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: AppColors.accent,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  // ── Payment detail dialog ──

  void _showPaymentDetailDialog(BuildContext context, BackendPayment payment) {
    final canVerify = ref.read(canVerifyPaymentProvider);
    showDialog(
      context: context,
      builder: (dialogCtx) {
        final dialogTheme = Theme.of(dialogCtx);
        final dialogIsDark = dialogTheme.brightness == Brightness.dark;
        final isApproved = payment.status == PaymentStatus.approved;
        final isRejected = payment.status == PaymentStatus.rejected;
        return Dialog(
          backgroundColor: dialogTheme.colorScheme.surface,
          insetPadding: const EdgeInsets.all(AppSpacing.md),
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Header
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      'Detail Pembayaran',
                      style: dialogTheme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close),
                      onPressed: () => Navigator.of(dialogCtx).pop(),
                    ),
                  ],
                ),
                const Divider(height: 20),

                // Amount
                Row(
                  children: [
                    Icon(
                      isApproved
                          ? Icons.check_circle_outline
                          : isRejected
                          ? Icons.cancel_outlined
                          : Icons.access_time_outlined,
                      color: isApproved
                          ? AppColors.success
                          : isRejected
                          ? AppColors.danger
                          : AppColors.warning,
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Text(
                      'Rp ${_formatAmount(payment.amount)}',
                      style: dialogTheme.textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.md),

                // Payment reference number
                if (payment.paymentNo != null &&
                    payment.paymentNo!.isNotEmpty) ...[
                  _buildDialogDetailRow(
                    dialogCtx,
                    dialogIsDark,
                    icon: Icons.receipt_outlined,
                    label: 'No. Pembayaran',
                    value: payment.paymentNo!,
                    valueColor: AppColors.accent,
                  ),
                  const SizedBox(height: AppSpacing.sm),
                ],

                // Details list
                _buildDialogDetailRow(
                  dialogCtx,
                  dialogIsDark,
                  icon: Icons.calendar_today_outlined,
                  label: 'Tanggal',
                  value: _formatDate(payment.paidAt),
                ),
                const SizedBox(height: AppSpacing.sm),
                _buildDialogDetailRow(
                  dialogCtx,
                  dialogIsDark,
                  icon: Icons.payment_outlined,
                  label: 'Metode',
                  value: _formatMethod(payment.method),
                ),
                const SizedBox(height: AppSpacing.sm),
                _buildDialogDetailRow(
                  dialogCtx,
                  dialogIsDark,
                  icon: Icons.tag_outlined,
                  label: 'Status',
                  value: payment.status.name.toUpperCase(),
                  valueColor: isApproved
                      ? AppColors.success
                      : isRejected
                      ? AppColors.danger
                      : AppColors.warning,
                ),

                // Catatan (warga note)
                if (payment.notes != null && payment.notes!.isNotEmpty) ...[
                  const SizedBox(height: AppSpacing.sm),
                  const Divider(height: 20),
                  _buildDialogDetailRow(
                    dialogCtx,
                    dialogIsDark,
                    icon: Icons.note_outlined,
                    label: 'Catatan Warga',
                    value: payment.notes!,
                  ),
                ],

                // Rejection reason
                if (isRejected &&
                    payment.rejectionReason != null &&
                    payment.rejectionReason!.isNotEmpty) ...[
                  const SizedBox(height: AppSpacing.sm),
                  const Divider(height: 20),
                  _buildDialogDetailRow(
                    dialogCtx,
                    dialogIsDark,
                    icon: Icons.error_outline,
                    label: 'Alasan Penolakan',
                    value: payment.rejectionReason!,
                    valueColor: AppColors.danger,
                  ),
                ],

                // Officer notes (bendahara/pengurus notes — separate from rejection)
                if (payment.officerNotes != null &&
                    payment.officerNotes!.isNotEmpty) ...[
                  const SizedBox(height: AppSpacing.sm),
                  const Divider(height: 20),
                  Container(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    decoration: BoxDecoration(
                      color: isApproved
                          ? (dialogIsDark
                                ? AppColors.darkSuccessSoft
                                : AppColors.success.withValues(alpha: 0.08))
                          : (dialogIsDark
                                ? AppColors.darkWarningSoft
                                : AppColors.warning.withValues(alpha: 0.08)),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                        color: isApproved
                            ? AppColors.success.withValues(alpha: 0.2)
                            : AppColors.warning.withValues(alpha: 0.2),
                      ),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Icon(
                              isApproved
                                  ? Icons.check_circle_outline
                                  : Icons.info_outline,
                              size: 18,
                              color: isApproved
                                  ? AppColors.success
                                  : AppColors.warning,
                            ),
                            const SizedBox(width: AppSpacing.sm),
                            Text(
                              'Catatan Pengurus',
                              style: dialogTheme.textTheme.bodyMedium?.copyWith(
                                fontWeight: FontWeight.w600,
                                color: isApproved
                                    ? AppColors.success
                                    : AppColors.warning,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: AppSpacing.xs),
                        Text(
                          payment.officerNotes!,
                          style: dialogTheme.textTheme.bodyMedium?.copyWith(
                            color: dialogIsDark
                                ? AppColors.darkText
                                : AppColors.lightText,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],

                // Verified by (name if available, fall back to UUID)
                if ((payment.verifiedByName != null &&
                        payment.verifiedByName!.isNotEmpty) ||
                    (payment.verifiedBy != null &&
                        payment.verifiedBy!.isNotEmpty)) ...[
                  const SizedBox(height: AppSpacing.sm),
                  const Divider(height: 20),
                  _buildDialogDetailRow(
                    dialogCtx,
                    dialogIsDark,
                    icon: Icons.verified_user_outlined,
                    label: 'Diverifikasi Oleh',
                    value: payment.verifiedByName ?? payment.verifiedBy!,
                    valueColor: AppColors.success,
                  ),
                  if (payment.verifiedAt != null) ...[
                    const SizedBox(height: 4),
                    Padding(
                      padding: const EdgeInsets.only(left: 36),
                      child: Text(
                        _formatDate(payment.verifiedAt),
                        style: dialogTheme.textTheme.bodySmall?.copyWith(
                          color: dialogIsDark
                              ? AppColors.darkTextMuted
                              : AppColors.lightTextMuted,
                        ),
                      ),
                    ),
                  ],
                ],

                // Bukti transfer
                const Divider(height: 20),
                Text(
                  'Bukti Transfer',
                  style: dialogTheme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                if (payment.proofPath != null &&
                    payment.proofPath!.isNotEmpty) ...[
                  PaymentProofThumbnail(
                    paymentId: payment.id,
                    size: 120,
                    label:
                        'Bukti Pembayaran Rp ${_formatAmount(payment.amount)} pada ${_formatDate(payment.paidAt)}',
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Center(
                    child: Text(
                      'Klik gambar untuk memperbesar',
                      style: dialogTheme.textTheme.bodySmall?.copyWith(
                        color: AppColors.lightTextMuted,
                      ),
                    ),
                  ),
                ] else ...[
                  Container(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    decoration: BoxDecoration(
                      color:
                          (dialogIsDark
                                  ? AppColors.darkSurfaceSubtle
                                  : AppColors.lightSurfaceSubtle)
                              .withValues(alpha: 0.5),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                        color:
                            (dialogIsDark
                                    ? AppColors.darkBorder
                                    : AppColors.lightBorder)
                                .withValues(alpha: 0.5),
                      ),
                    ),
                    child: Center(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.image_not_supported_outlined,
                            size: 32,
                            color: dialogIsDark
                                ? AppColors.darkTextMuted
                                : AppColors.lightTextMuted,
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            'Tidak ada bukti transfer',
                            style: dialogTheme.textTheme.bodySmall?.copyWith(
                              color: dialogIsDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ],

                // Action buttons (Terima & Tolak) in neon style for authorized staff on pending payment
                if (payment.status == PaymentStatus.pending && canVerify) ...[
                  const SizedBox(height: AppSpacing.lg),
                  const Divider(height: 24),
                  LayoutBuilder(
                    builder: (layoutCtx, constraints) {
                      final isNarrow = constraints.maxWidth < 280;
                      final terimaBtn = _buildNeonVerifyButton(
                        context: dialogCtx,
                        label: 'Terima',
                        icon: Icons.check_circle_outline,
                        neonColor: AppColors.success,
                        softBg: dialogIsDark
                            ? AppColors.darkSuccessSoft
                            : AppColors.lightSuccessSoft,
                        onPressed: () {
                          _approvePayment(
                            context,
                            payment,
                            onSuccess: () {
                              if (Navigator.of(dialogCtx).canPop()) {
                                Navigator.of(dialogCtx).pop();
                              }
                            },
                          );
                        },
                      );
                      final tolakBtn = _buildNeonVerifyButton(
                        context: dialogCtx,
                        label: 'Tolak',
                        icon: Icons.cancel_outlined,
                        neonColor: AppColors.danger,
                        softBg: dialogIsDark
                            ? AppColors.darkDangerSoft
                            : AppColors.lightDangerSoft,
                        onPressed: () {
                          _showRejectDialog(
                            context,
                            payment.id,
                            onSuccess: () {
                              if (Navigator.of(dialogCtx).canPop()) {
                                Navigator.of(dialogCtx).pop();
                              }
                            },
                          );
                        },
                      );

                      if (isNarrow) {
                        return Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            terimaBtn,
                            const SizedBox(height: AppSpacing.sm),
                            tolakBtn,
                          ],
                        );
                      }

                      return Row(
                        children: [
                          Expanded(child: terimaBtn),
                          const SizedBox(width: AppSpacing.md),
                          Expanded(child: tolakBtn),
                        ],
                      );
                    },
                  ),
                ],

                // Action button: Batalkan Pembayaran (Reversal) for approved payments
                if (payment.status == PaymentStatus.approved && canVerify) ...[
                  const SizedBox(height: AppSpacing.lg),
                  const Divider(height: 24),
                  SizedBox(
                    width: double.infinity,
                    child: OutlinedButton.icon(
                      style: OutlinedButton.styleFrom(
                        foregroundColor: AppColors.danger,
                        side: const BorderSide(color: AppColors.danger),
                        padding: const EdgeInsets.symmetric(vertical: 12),
                      ),
                      onPressed: () {
                        Navigator.of(dialogCtx).pop();
                        _showReversePaymentDialog(context, payment);
                      },
                      icon: const Icon(Icons.undo, size: 16),
                      label: const Text('Batalkan Pembayaran (Reversal)'),
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }

  Future<void> _showReversePaymentDialog(
    BuildContext context,
    BackendPayment payment,
  ) async {
    final reasonController = TextEditingController();
    bool submitting = false;
    String? error;

    showDialog(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text('Batalkan Pembayaran (Reversal)'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (error != null) ...[
                      Text(error!, style: const TextStyle(color: AppColors.danger)),
                      const SizedBox(height: 8),
                    ],
                    Text(
                      'Pembayaran senilai Rp ${_formatAmount(payment.amount)} akan dibatalkan melalui transaksi reversal KAS kompensasi.',
                      style: const TextStyle(fontSize: 13),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    TextField(
                      controller: reasonController,
                      maxLines: 3,
                      decoration: const InputDecoration(
                        labelText: 'Alasan Pembatalan (Wajib, min. 5 karakter)',
                        border: OutlineInputBorder(),
                        alignLabelWithHint: true,
                      ),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(backgroundColor: AppColors.danger),
                  onPressed: submitting
                      ? null
                      : () async {
                          final reason = reasonController.text.trim();
                          if (reason.length < 5) {
                            setDialogState(() => error = 'Alasan pembatalan minimal 5 karakter.');
                            return;
                          }

                          setDialogState(() {
                            submitting = true;
                            error = null;
                          });

                          try {
                            final repository = ref.read(iuranRepositoryProvider);
                            await repository.reversePayment(
                              payment.id,
                              reason: reason,
                            );

                            if (context.mounted) {
                              Navigator.pop(dialogContext);
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text('Pembayaran berhasil dibatalkan (reversal)'),
                                  backgroundColor: AppColors.success,
                                ),
                              );
                              ref.invalidate(billDetailProvider(widget.billId));
                              ref.invalidate(billPaymentsProvider(widget.billId));
                              ref.invalidate(pendingPaymentsProvider);
                            }
                          } catch (e) {
                            setDialogState(() {
                              submitting = false;
                              error = 'Gagal membatalkan pembayaran: $e';
                            });
                          }
                        },
                  child: submitting
                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Batalkan Pembayaran', style: TextStyle(color: Colors.white)),
                ),
              ],
            );
          },
        );
      },
    );
  }
}
