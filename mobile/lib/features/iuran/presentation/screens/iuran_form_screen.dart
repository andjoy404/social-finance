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

/// Enum representing the type of due (iuran definition).
enum DueType { regular, voluntary }

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
  final _periodCtrl = TextEditingController();
  final _dueDateCtrl = TextEditingController();
  final _validUntilCtrl = TextEditingController();
  BackendDue? _createdDue;

  DuePeriodType _periodType = DuePeriodType.monthly;
  PaymentRule _paymentRule = PaymentRule.full;
  DueType _dueType = DueType.regular;
  bool _saving = false;
  bool _dueCreated = false;
  String _generationKey = '';
  String? _error;
  List<String> _validationErrors = [];

  @override
  void initState() {
    super.initState();
    // Auto-focus amount field with sensible default
    final now = DateTime.now();
    _periodCtrl.text = _formatPeriod(now, _periodType);
    _dueDateCtrl.text = _formatDate(now);
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _amountCtrl.dispose();
    _periodCtrl.dispose();
    _dueDateCtrl.dispose();
    _validUntilCtrl.dispose();
    super.dispose();
  }

  String _formatDate(DateTime date) =>
      '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';

  String _formatPeriod(DateTime date, DuePeriodType type) {
    switch (type) {
      case DuePeriodType.monthly:
        return '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}';
      case DuePeriodType.yearly:
        return date.year.toString();
      case DuePeriodType.oneTime:
        return _formatDate(date);
    }
  }

  void _setPeriodForType(DuePeriodType type) {
    final parsed = DateTime.tryParse(_dueDateCtrl.text) ?? DateTime.now();
    _periodCtrl.text = _formatPeriod(parsed, type);
  }

  Future<void> _pickDueDate() async {
    final initial = DateTime.tryParse(_dueDateCtrl.text) ?? DateTime.now();
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
    );
    if (picked != null) {
      setState(() {
        _dueDateCtrl.text = _formatDate(picked);
        _setPeriodForType(_periodType);
      });
    }
  }

  /// Show date picker for the validity end date of a voluntary due.
  Future<void> _pickValidUntil() async {
    final initial = DateTime.tryParse(_validUntilCtrl.text) ??
        DateTime.now().add(const Duration(days: 365));
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime.now(),
      lastDate: DateTime(2100),
      helpText: 'PILIH TANGGAL BERLAKU SAMPAI',
    );
    if (picked != null) {
      setState(() => _validUntilCtrl.text = _formatDate(picked));
    }
  }

  Future<void> _pickPeriod() async {
    final now = DateTime.now();
    DateTime initial = now;
    if (_periodType == DuePeriodType.monthly) {
      final parts = _periodCtrl.text.split('-');
      if (parts.length >= 2) {
        initial = DateTime(
          int.tryParse(parts[0]) ?? now.year,
          int.tryParse(parts[1]) ?? now.month,
        );
      }
    } else if (_periodType == DuePeriodType.yearly) {
      final y = int.tryParse(_periodCtrl.text);
      if (y != null) {
        initial = DateTime(y);
      }
    } else {
      final parsed = DateTime.tryParse(_periodCtrl.text);
      if (parsed != null) initial = parsed;
    }
    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime(2000),
      lastDate: DateTime(2100),
      initialDatePickerMode: DatePickerMode.year,
      helpText: _periodType == DuePeriodType.monthly
          ? 'PILIH BULAN PERIODE'
          : 'PILIH PERIODE',
    );
    if (picked != null) {
      setState(() {
        _periodCtrl.text = _formatPeriod(picked, _periodType);
        if (_validationErrors.isNotEmpty) {
          _validationErrors = [];
        }
      });
    }
  }

  /// Reset form when switching due type.
  void _switchDueType(DueType type) {
    setState(() {
      _dueType = type;
      if (type == DueType.voluntary) {
        _amountCtrl.text = '0';
        _paymentRule = PaymentRule.partial;
        _validUntilCtrl.clear();
      } else {
        _amountCtrl.text = '';
        _validUntilCtrl.clear();
      }
      if (_validationErrors.isNotEmpty) {
        _validationErrors = [];
      }
    });
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
    } else if (_dueType == DueType.voluntary && amount < 0) {
      errors.add('Nominal sumbangan sukarela harus nol atau lebih.');
    } else if (_dueType == DueType.regular && amount <= 0) {
      errors.add('Nominal harus lebih besar dari 0.');
    }

    final period = _periodCtrl.text.trim();
    if (period.isEmpty) {
      errors.add('Periode harus diisi.');
    } else {
      final validPeriod = switch (_periodType) {
        DuePeriodType.monthly => RegExp(r'^\d{4}-\d{2}$').hasMatch(period),
        DuePeriodType.yearly => RegExp(r'^\d{4}$').hasMatch(period),
        DuePeriodType.oneTime => RegExp(
          r'^\d{4}-\d{2}-\d{2}$',
        ).hasMatch(period),
      };
      if (!validPeriod) errors.add('Format periode tidak sesuai frekuensi.');
    }

    if (DateTime.tryParse(_dueDateCtrl.text.trim()) == null ||
        !RegExp(r'^\d{4}-\d{2}-\d{2}$').hasMatch(_dueDateCtrl.text.trim())) {
      errors.add('Tanggal jatuh tempo harus berformat YYYY-MM-DD.');
    }

    setState(() => _validationErrors = errors);
    return errors.isEmpty;
  }

  String _generateOperationKey(String dueId) {
    if (_generationKey.isEmpty) {
      _generationKey = 'iuran-$dueId-${DateTime.now().microsecondsSinceEpoch}';
    }
    return _generationKey;
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

      final due = _createdDue ??= await ref
          .read(iuranRepositoryProvider)
          .createDue(
            name: _nameCtrl.text.trim(),
            amount: amount,
            periodType: _periodType,
            paymentRule: _dueType == DueType.voluntary
                ? PaymentRule.partial
                : _paymentRule,
            isVoluntary: _dueType == DueType.voluntary,
            validUntil: _dueType == DueType.voluntary &&
                    _validUntilCtrl.text.isNotEmpty
                ? _validUntilCtrl.text.trim()
                : null,
          );

      _dueCreated = true;
      final generated = await ref
          .read(iuranRepositoryProvider)
          .generateBills(
            dueId: due.id,
            period: _periodCtrl.text.trim(),
            dueDate: _dueDateCtrl.text.trim(),
            idempotencyKey: _generateOperationKey(due.id),
          );

      await Future.wait([
        ref.read(duesProvider.notifier).refresh(),
        ref.read(billsProvider.notifier).refresh(),
      ]);

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              generated.generatedCount == 0
                  ? 'Iuran berhasil disimpan. Belum ada tagihan baru yang dibuat.'
                  : 'Iuran dan ${generated.generatedCount} tagihan berhasil dibuat.',
            ),
            backgroundColor: AppColors.success,
          ),
        );
        context.pop();
      }
    } catch (e) {
      final error = ref.read(iuranRepositoryProvider).mapDioError(e);
      if (mounted) {
        setState(
          () => _error = _dueCreated
              ? 'Iuran berhasil disimpan, tetapi tagihan belum berhasil dibuat. Silakan coba proses pembuatan tagihan kembali.'
              : error.message,
        );
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
                              const Text('• ', style: TextStyle(fontSize: 16)),
                              Expanded(child: Text(e)),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),

              // ── Jenis Iuran — badge toggle ──
              Text(
                'Jenis Iuran',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  // Reguler badge
                  GestureDetector(
                    onTap: _saving
                        ? null
                        : () => _switchDueType(DueType.regular),
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: _dueType == DueType.regular
                            ? AppColors.accent.withValues(
                                alpha: isDark ? 0.20 : 0.12,
                              )
                            : Colors.transparent,
                        border: Border.all(
                          color: _dueType == DueType.regular
                              ? AppColors.accent
                              : (isDark
                                    ? AppColors.darkBorder
                                    : AppColors.lightBorder),
                          width: 1,
                        ),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        'Reguler',
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: _dueType == DueType.regular
                              ? AppColors.accent
                              : (isDark
                                    ? AppColors.darkTextMuted
                                    : AppColors.lightTextMuted),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  // Sukarela badge
                  GestureDetector(
                    onTap: _saving
                        ? null
                        : () => _switchDueType(DueType.voluntary),
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 6,
                      ),
                      decoration: BoxDecoration(
                        color: _dueType == DueType.voluntary
                            ? AppColors.accent.withValues(
                                alpha: isDark ? 0.20 : 0.12,
                              )
                            : Colors.transparent,
                        border: Border.all(
                          color: _dueType == DueType.voluntary
                              ? AppColors.accent
                              : (isDark
                                    ? AppColors.darkBorder
                                    : AppColors.lightBorder),
                          width: 1,
                        ),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        'Sukarela',
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: _dueType == DueType.voluntary
                              ? AppColors.accent
                              : (isDark
                                    ? AppColors.darkTextMuted
                                    : AppColors.lightTextMuted),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

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
                    'Nominal${_dueType == DueType.regular ? '' : ' (opsional, default Rp0)'}',
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
                      hintText: _dueType == DueType.voluntary
                          ? 'Contoh: 0 untuk sumbangan sukarela'
                          : 'Contoh: 50000',
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

              // Info sukarela
              if (_dueType == DueType.voluntary)
                Container(
                  width: double.maxFinite,
                  padding: const EdgeInsets.all(AppSpacing.sm),
                  decoration: BoxDecoration(
                    color: AppColors.info.withValues(
                      alpha: isDark ? 0.15 : 0.08,
                    ),
                    border: Border.all(
                      color: AppColors.info.withValues(
                        alpha: isDark ? 0.3 : 0.2,
                      ),
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.info_outline, size: 16, color: AppColors.info),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(
                          'Iuran sukarela bersifat opsional. Tidak ada tunggakan atau penalti jika tidak berkontribusi.',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: isDark
                                ? AppColors.darkTextMuted
                                : AppColors.lightTextMuted,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              if (_dueType == DueType.voluntary)
                const SizedBox(height: AppSpacing.lg),

              // Tanggal berlaku sampai — sukarela only
              if (_dueType == DueType.voluntary)
                AppTextField(
                  controller: _validUntilCtrl,
                  label: 'Berlaku Sampai',
                  hintText: 'YYYY-MM-DD',
                  readOnly: true,
                  onTap: _pickValidUntil,
                  suffixIcon: const Icon(Icons.calendar_today, size: 20),
                  onChanged: (value) {
                    if (_validationErrors.isNotEmpty) {
                      setState(() => _validationErrors = []);
                    }
                  },
                ),
              if (_dueType == DueType.voluntary)
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
                      setState(() {
                        _periodType = selected.first;
                        _setPeriodForType(_periodType);
                      });
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

              // Rule Pembayaran — only for reguler
              if (_dueType == DueType.regular) ...[
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
                        setState(() => _paymentRule = selected.first);
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
              ],

              // Periode
              AppTextField(
                controller: _periodCtrl,
                label: 'Periode',
                hintText: 'YYYY-MM, YYYY, atau YYYY-MM-DD',
                readOnly: true,
                onTap: _pickPeriod,
                suffixIcon: const Icon(Icons.calendar_month, size: 20),
                onChanged: (_) {
                  if (_validationErrors.isNotEmpty) {
                    setState(() => _validationErrors = []);
                  }
                },
              ),
              const SizedBox(height: AppSpacing.lg),

              // Tanggal jatuh tempo
              AppTextField(
                controller: _dueDateCtrl,
                label: 'Tanggal Jatuh Tempo',
                hintText: 'YYYY-MM-DD',
                readOnly: true,
                onTap: _pickDueDate,
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
                          key: const ValueKey('iuran-save-button'),
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
                                  mainAxisAlignment: MainAxisAlignment.center,
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
                    child: SizedBox(
                      height: 48,
                      child: OutlinedButton(
                        onPressed: _saving ? null : _confirmCancel,
                        style: OutlinedButton.styleFrom(
                          side: const BorderSide(color: AppColors.danger),
                          foregroundColor: AppColors.danger,
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                        ),
                        child: const Text('Batal'),
                      ),
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
