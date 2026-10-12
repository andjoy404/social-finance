import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

/// Screen for creating a new iuran (bill type / due).
class IuranFormScreen extends ConsumerStatefulWidget {
  const IuranFormScreen({super.key});

  @override
  ConsumerState<IuranFormScreen> createState() => _IuranFormScreenState();
}

class _IuranFormScreenState extends ConsumerState<IuranFormScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _amountCtrl = TextEditingController();

  DuePeriodType _periodType = DuePeriodType.monthly;
  PaymentRule _paymentRule = PaymentRule.full;
  bool _isVoluntary = false;
  bool _saving = false;
  String? _error;
  List<String> _validationErrors = [];

  @override
  void initState() {
    super.initState();
    // Auto-focus amount field with sensible default
    _amountCtrl.text = '';
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _amountCtrl.dispose();
    super.dispose();
  }

  bool _validate() {
    final errors = <String>[];

    final name = _nameCtrl.text.trim();
    if (name.isEmpty) {
      errors.add('Nama iuran harus diisi.');
    }

    final amountText = _amountCtrl.text.trim().replaceAll('.', '');
    final amount = int.tryParse(amountText);
    if (amount == null) {
      errors.add('Nominal harus angka yang valid.');
    } else if (!_isVoluntary && amount <= 0) {
      errors.add('Nominal harus lebih besar dari 0.');
    } else if (_isVoluntary && amount < 0) {
      errors.add('Nominal sumbangan sukarela harus nol atau lebih.');
    }

    setState(() => _validationErrors = errors);
    return errors.isEmpty;
  }

  Future<void> _save() async {
    if (!_validate()) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    try {
      final amountText = _amountCtrl.text.trim().replaceAll('.', '');
      final amount = int.tryParse(amountText) ?? 0;

      await ref.read(iuranRepositoryProvider).createDue(
            name: _nameCtrl.text.trim(),
            amount: amount,
            periodType: _periodType,
            paymentRule: _paymentRule,
            isVoluntary: _isVoluntary,
          );

      // Refresh dues list
      await ref.read(duesProvider.notifier).refresh();

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Iuran berhasil ditambahkan.'),
            backgroundColor: AppColors.success,
          ),
        );
        context.pop();
      }
    } catch (e) {
      final error = ref.read(iuranRepositoryProvider).mapDioError(e);
      if (mounted) {
        setState(() => _error = error.message);
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  /// Show confirmation dialog before canceling the form.
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

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        await _confirmCancel();
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Tambah Iuran'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: _confirmCancel,
          ),
        ),
        body: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppSpacing.base),
            children: [
              // Error message
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

              // Validation errors
              if (_validationErrors.isNotEmpty)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(
                      alpha: isDark ? 0.15 : 0.08,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Data belum lengkap:',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                          color: AppColors.danger,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      ..._validationErrors.map(
                        (e) => Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text(
                                '• ',
                                style: TextStyle(fontSize: 16),
                              ),
                              Expanded(child: Text(e)),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),

              // Nama Iuran
              AppTextField(
                controller: _nameCtrl,
                label: 'Nama Iuran',
                hintText: 'Contoh: Iuran Keamanan Bulanan',
                onChanged: (value) {
                  if (_validationErrors.isNotEmpty) {
                    setState(() => _validationErrors = []);
                  }
                },
              ),
              const SizedBox(height: AppSpacing.lg),

              // Nominal
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Nominal',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  TextField(
                    controller: _amountCtrl,
                    keyboardType: TextInputType.number,
                    inputFormatters: [
                      FilteringTextInputFormatter.allow(
                        RegExp(r'^\d{1,10}(\.\d{0,2})?$'),
                      ),
                    ],
                    decoration: InputDecoration(
                      hintText: 'Contoh: 50000',
                      prefixText: 'Rp ',
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                      contentPadding: const EdgeInsets.symmetric(
                        horizontal: AppSpacing.md,
                        vertical: AppSpacing.sm,
                      ),
                      helperText: 'Dalam Rupiah',
                      helperStyle: theme.textTheme.bodySmall?.copyWith(
                        color: AppColors.lightTextMuted,
                      ),
                    ),
                    onChanged: (value) {
                      if (_validationErrors.isNotEmpty) {
                        setState(() => _validationErrors = []);
                      }
                    },
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Frekuensi
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Frekuensi',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  SegmentedButton<DuePeriodType>(
                    segments: [
                      ButtonSegment(
                        value: DuePeriodType.monthly,
                        label: const Text('Bulanan'),
                        icon: const Icon(Icons.calendar_month, size: 16),
                      ),
                      ButtonSegment(
                        value: DuePeriodType.yearly,
                        label: const Text('Tahunan'),
                        icon: const Icon(Icons.calendar_today, size: 16),
                      ),
                      ButtonSegment(
                        value: DuePeriodType.oneTime,
                        label: const Text('Sekali'),
                        icon: const Icon(Icons.event_note, size: 16),
                      ),
                    ],
                    selected: {_periodType},
                    onSelectionChanged: (selected) {
                      setState(
                        () => _periodType = selected.first,
                      );
                    },
                    style: SegmentedButton.styleFrom(
                      backgroundColor: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurface,
                      foregroundColor: isDark
                          ? AppColors.darkText
                          : AppColors.lightText,
                      selectedForegroundColor: AppColors.accent,
                      selectedBackgroundColor: AppColors.accentSoftColor(
                        context,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Rule Pembayaran
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Aturan Pembayaran',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  SegmentedButton<PaymentRule>(
                    segments: [
                      ButtonSegment(
                        value: PaymentRule.full,
                        label: const Text('Lunas Penuh'),
                        icon: const Icon(Icons.payment, size: 16),
                      ),
                      ButtonSegment(
                        value: PaymentRule.partial,
                        label: const Text('Sebagian'),
                        icon: const Icon(Icons.attach_money, size: 16),
                      ),
                    ],
                    selected: {_paymentRule},
                    onSelectionChanged: (selected) {
                      setState(
                        () => _paymentRule = selected.first,
                      );
                    },
                    style: SegmentedButton.styleFrom(
                      backgroundColor: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurface,
                      foregroundColor: isDark
                          ? AppColors.darkText
                          : AppColors.lightText,
                      selectedForegroundColor: AppColors.accent,
                      selectedBackgroundColor: AppColors.accentSoftColor(
                        context,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Sumbangan Sukarela
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Sumbangan Sukarela',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      Switch(
                        value: _isVoluntary,
                        onChanged: _saving ? null : (v) => setState(() => _isVoluntary = v),
                        activeColor: AppColors.accent,
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(
                          _isVoluntary
                              ? 'Aktif — Nominal Rp0 diperbolehkan'
                              : 'Non-aktif — Nominal harus di atas Rp0',
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: _isVoluntary
                                ? AppColors.success
                                : null,
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.xxl),

              // Horizontal buttons: Simpan + Batal
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
                              borderRadius:
                                  BorderRadius.circular(AppRadius.base),
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
                  SizedBox(
                    height: 48,
                    child: OutlinedButton(
                      onPressed: _saving ? null : _confirmCancel,
                      style: OutlinedButton.styleFrom(
                        side: BorderSide(
                          color: isDark
                              ? AppColors.darkBorder
                              : AppColors.lightBorder,
                        ),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppRadius.base),
                        ),
                      ),
                      child: const Text('Batal'),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
