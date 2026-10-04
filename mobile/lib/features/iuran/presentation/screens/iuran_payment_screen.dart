import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/empty_state.dart';

import '../../data/iuran_api_models.dart';
import '../../data/iuran_models.dart';
import '../../data/iuran_providers.dart';
import '../../data/iuran_repository.dart';

/// Payment form screen for making iuran payments.
class IuranPaymentScreen extends ConsumerStatefulWidget {
  final String billId;

  const IuranPaymentScreen({super.key, required this.billId});

  @override
  ConsumerState<IuranPaymentScreen> createState() =>
      _IuranPaymentScreenState();
}

class _IuranPaymentScreenState extends ConsumerState<IuranPaymentScreen> {
  final _formKey = GlobalKey<FormState>();
  final _tanggalCtrl = TextEditingController();
  final _nominalCtrl = TextEditingController();
  final _catatanCtrl = TextEditingController();

  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _tanggalCtrl.text = DateTime.now().toLocal().toString().split(' ')[0];
  }

  @override
  void dispose() {
    _tanggalCtrl.dispose();
    _nominalCtrl.dispose();
    _catatanCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (!_validate()) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    try {
      final amount = int.parse(_nominalCtrl.text.trim());

      await ref.read(iuranRepositoryProvider).createPayment(
            billId: widget.billId,
            amount: amount,
            method: PaymentMethod.cash,
            notes: _catatanCtrl.text.trim().isEmpty
                ? null
                : _catatanCtrl.text.trim(),
          );

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Pembayaran berhasil dicatat.'),
            backgroundColor: AppColors.success,
            behavior: SnackBarBehavior.floating,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppRadius.base),
            ),
          ),
        );
        context.pop();
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _saving = false;
        });
      }
    }
  }

  bool _validate() {
    if (!_formKey.currentState!.validate()) return false;

    final amount = int.tryParse(_nominalCtrl.text.trim());
    if (amount != null) {
      final state = ref.read(billDetailProvider(widget.billId));
      if (state is AsyncData<IuranBill?> && state.value != null) {
        final remaining = state.value!.remainingAmount;
        if (amount > remaining && remaining > 0) {
          if (mounted) {
            setState(() => _error = 'Nominal melebihi sisa tagihan.');
          }
          return false;
        }
      }
    }
    return true;
  }

  Future<void> _confirmCancel() async {
    final shouldCancel = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Apakah Anda yakin ingin membatalkan?'),
        content: const Text('Perubahan yang belum disimpan akan hilang.'),
        actions: [
          Align(
            alignment: Alignment.center,
            child: SizedBox(
              height: 48,
              child: OutlinedButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: AppColors.accent),
                  backgroundColor: AppColors.accentSoftColor(context),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                ),
                child: const Text('Tidak, Tetap di Halaman'),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.center,
            child: Container(
              decoration: BoxDecoration(
                boxShadow: [
                  BoxShadow(
                    color: AppColors.danger.withValues(alpha: 0.15),
                    blurRadius: 8,
                    offset: const Offset(0, 0),
                  ),
                ],
              ),
              child: SizedBox(
                height: 48,
                child: OutlinedButton(
                  onPressed: () => Navigator.of(dialogContext).pop(true),
                  style: OutlinedButton.styleFrom(
                    side: BorderSide(color: AppColors.danger),
                    backgroundColor:
                        Theme.of(dialogContext).brightness == Brightness.dark
                            ? AppColors.darkDangerSoft
                            : AppColors.danger.withValues(alpha: 0.20),
                    foregroundColor: AppColors.danger,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                  ),
                  child: const Text('Ya, Batalkan'),
                ),
              ),
            ),
          ),
        ],
      ),
    );

    if (shouldCancel == true && mounted) {
      context.pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    // Watch bill detail from API
    final billState = ref.watch(billDetailProvider(widget.billId));

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        if (!_saving) {
          await _confirmCancel();
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Pembayaran Iuran'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: _saving ? null : () => _confirmCancel(),
          ),
        ),
        body: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppSpacing.base),
            children: [
              // ── Error message ──
              if (_error != null)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(
                      alpha: isDark ? 0.2 : 0.1,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    border: Border.all(
                      color: AppColors.danger.withValues(
                        alpha: isDark ? 0.4 : 0.2,
                      ),
                    ),
                  ),
                  child: Text(
                    _error!,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: AppColors.danger,
                    ),
                  ),
                ),
              if (_error != null) const SizedBox(height: AppSpacing.md),

              // ── Bill Summary ──
              switch (billState) {
                AsyncData(:final value) when value != null => _buildBillSummary(
                    theme,
                    isDark,
                    value,
                  ),
                AsyncData() => Center(
                    child: Padding(
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
                AsyncError() => Center(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.all(AppSpacing.lg),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            'Gagal Memuat Data Tagihan',
                            style: theme.textTheme.titleLarge,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          ElevatedButton.icon(
                            onPressed: () => ref
                                .refresh(billDetailProvider(widget.billId)),
                            icon: const Icon(Icons.refresh),
                            label: const Text('Coba Lagi'),
                          ),
                        ],
                      ),
                    ),
                  ),
                _ => const Center(child: CircularProgressIndicator()),
              },

              const SizedBox(height: AppSpacing.lg),

              // ── Tanggal Pembayaran ──
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Tanggal Pembayaran',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  InkWell(
                    onTap: () async {
                      final picked = await showDatePicker(
                        context: context,
                        initialDate: DateTime.now(),
                        firstDate: DateTime(2020),
                        lastDate: DateTime.now().add(
                          const Duration(days: 365),
                        ),
                      );
                      if (picked != null && mounted) {
                        setState(() {
                          _tanggalCtrl.text = picked
                              .toLocal()
                              .toString()
                              .split(' ')[0];
                        });
                      }
                    },
                    child: Container(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      decoration: BoxDecoration(
                        border: Border.all(color: AppColors.lightBorder),
                        borderRadius:
                            BorderRadius.circular(AppRadius.base),
                        color: isDark
                            ? AppColors.darkSurface
                            : AppColors.lightSurface,
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.calendar_today,
                            size: 20,
                            color: AppColors.accent,
                          ),
                          const SizedBox(width: AppSpacing.md),
                          Text(
                            _tanggalCtrl.text.isEmpty
                                ? 'Pilih tanggal'
                                : _tanggalCtrl.text,
                            style: theme.textTheme.bodyLarge,
                          ),
                          const Spacer(),
                          const Icon(
                            Icons.arrow_forward_ios,
                            size: 16,
                            color: Colors.grey,
                          ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // ── Nominal Pembayaran ──
              AppTextField(
                controller: _nominalCtrl,
                label: 'Nominal Pembayaran',
                hintText: 'Masukkan nominal',
                keyboardType: TextInputType.number,
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Nominal harus diisi.';
                  }
                  final amount = int.tryParse(value.trim());
                  if (amount == null || amount <= 0) {
                    return 'Nominal harus berupa angka yang valid.';
                  }
                  final state = ref.read(
                    billDetailProvider(widget.billId),
                  );
                  if (state is AsyncData<IuranBill?> &&
                      state.value != null &&
                      amount > state.value!.remainingAmount &&
                      state.value!.remainingAmount > 0) {
                    return 'Nominal melebihi sisa tagihan.';
                  }
                  return null;
                },
              ),
              const SizedBox(height: AppSpacing.lg),

              // ── Catatan Pembayaran ──
              AppTextField(
                controller: _catatanCtrl,
                label: 'Catatan Pembayaran',
                hintText: 'Catatan opsional (mis: bayar via transfer)',
              ),
              const SizedBox(height: AppSpacing.xxl),

              // ── Buttons: Simpan + Batal ──
              Row(
                children: [
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.accent.withValues(alpha: 0.15),
                            blurRadius: 8,
                            offset: const Offset(0, 0),
                          ),
                        ],
                      ),
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _save,
                          style: OutlinedButton.styleFrom(
                            side: const BorderSide(color: AppColors.accent),
                            foregroundColor: AppColors.accent,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),
                          child: _saving
                              ? SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                    backgroundColor: Colors.transparent,
                                    valueColor: AlwaysStoppedAnimation<Color>(
                                      AppColors.accent,
                                    ),
                                  ),
                                )
                              : const Row(
                                  mainAxisAlignment:
                                      MainAxisAlignment.center,
                                  children: [
                                    Icon(Icons.save_outlined, size: 18),
                                    SizedBox(width: 6),
                                    Text('Simpan'),
                                  ],
                                ),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.danger.withValues(alpha: 0.15),
                            blurRadius: 8,
                            offset: const Offset(0, 0),
                          ),
                        ],
                      ),
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed:
                              _saving ? null : () => _confirmCancel(),
                          style: OutlinedButton.styleFrom(
                            side: BorderSide(color: AppColors.danger),
                            foregroundColor: AppColors.danger,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),
                          child: const Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.close, size: 18),
                              SizedBox(width: 6),
                              Text('Batal'),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.base),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBillSummary(
    ThemeData theme,
    bool isDark,
    IuranBill bill,
  ) {
    return AppCard(
      useEmphasis: true,
      padding: const EdgeInsets.all(AppSpacing.base),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Ringkasan Tagihan',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Icon(
                Icons.groups_outlined,
                size: 18,
                color: AppColors.accent,
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  'Kepala Keluarga',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
                  ),
                ),
              ),
              Text(
                bill.householdName,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(
                Icons.receipt_long_outlined,
                size: 18,
                color: AppColors.accent,
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  'Jenis Iuran',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
                  ),
                ),
              ),
              Text(
                bill.iuranType,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(
                Icons.calendar_today_outlined,
                size: 18,
                color: AppColors.accent,
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  'Periode',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: isDark
                        ? AppColors.darkTextMuted
                        : AppColors.lightTextMuted,
                  ),
                ),
              ),
              Text(
                bill.periode,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
          const Divider(height: 24),
          Row(
            children: [
              Icon(
                Icons.account_balance_wallet_outlined,
                size: 18,
                color: AppColors.accent,
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  'Sisa Tagihan',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              Text(
                formatRupiah(bill.remainingAmount),
                style: theme.textTheme.titleLarge?.copyWith(
                  color: AppColors.accent,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
