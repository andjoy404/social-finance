/// KAS Keluar form — connected to real backend API.
///
/// Fetches categories from GET /api/v1/categories, maps selected category
/// UUID to the POST /api/v1/transactions body.

import 'dart:async';
import 'dart:math';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';

/// Generates a v4-style random UUID for idempotency keys.
String _generateUuid() {
  final r = Random();
  String hex(int count) =>
      List.generate(count, (_) => '0123456789abcdef'[r.nextInt(16)]).join();
  return '${hex(8)}-${hex(4)}-4${hex(3)}-'
      '${'89ab'[r.nextInt(4)]}${hex(3)}-'
      '${hex(12)}';
}

class KasKeluarScreen extends ConsumerStatefulWidget {
  const KasKeluarScreen({super.key});

  @override
  ConsumerState<KasKeluarScreen> createState() => _KasKeluarScreenState();
}

class _KasKeluarScreenState extends ConsumerState<KasKeluarScreen> {
  final _formKey = GlobalKey<FormState>();
  final _tanggalCtrl = TextEditingController();
  final _nominalCtrl = TextEditingController();
  final _keteranganCtrl = TextEditingController();

  List<BackendCategory> _expenseCategories = [];
  String? _selectedCategoryId;
  bool _loadingCategories = true;

  String _paymentMethod = 'CASH';
  bool _saving = false;
  String? _error;
  List<String> _validationErrors = [];

  @override
  void initState() {
    super.initState();
    final now = DateTime.now();
    _tanggalCtrl.text =
        '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
    _loadCategories();
  }

  Future<void> _loadCategories() async {
    try {
      final repo = ref.read(kasRepositoryProvider);
      final cats = await repo.fetchCategories(isActive: true);
      final expenseCats = cats.where((c) => c.type == 'expense').toList();
      String? defaultId;
      for (final c in expenseCats) {
        if (c.name.toLowerCase().contains('operasional')) {
          defaultId = c.id;
          break;
        }
      }
      if (defaultId == null && expenseCats.isNotEmpty) {
        defaultId = expenseCats.first.id;
      }
      if (mounted) {
        setState(() {
          _expenseCategories = expenseCats;
          _selectedCategoryId = defaultId;
          _loadingCategories = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _loadingCategories = false;
        });
      }
    }
  }

  @override
  void dispose() {
    _tanggalCtrl.dispose();
    _nominalCtrl.dispose();
    _keteranganCtrl.dispose();
    super.dispose();
  }

  bool _validate() {
    final errors = <String>[];

    if (_tanggalCtrl.text.trim().isEmpty) {
      errors.add('Tanggal harus diisi.');
    }
    if (_nominalCtrl.text.trim().isEmpty) {
      errors.add('Nominal harus diisi.');
    } else {
      final parsed = parseRupiahInput(_nominalCtrl.text.trim());
      if (parsed == null || parsed <= 0) {
        errors.add('Nominal harus berupa angka yang lebih besar dari 0.');
      }
    }
    if (_keteranganCtrl.text.trim().isEmpty) {
      errors.add('Keterangan harus diisi.');
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
      final nominalInt = parseRupiahInput(_nominalCtrl.text.trim()) ?? 0;
      if (nominalInt <= 0) {
        setState(() {
          _error = 'Nominal harus lebih besar dari 0.';
          _saving = false;
        });
        return;
      }

      final repository = ref.read(kasRepositoryProvider);
      final occurredAt =
          DateTime.tryParse(_tanggalCtrl.text.trim()) ?? DateTime.now();

      final desc = _keteranganCtrl.text.trim();

      await repository.createTransaction(
        categoryId: _selectedCategoryId,
        amount: nominalInt,
        jenis: KasJenis.keluar,
        description: desc,
        occurredAt: occurredAt,
        paymentMethod: _paymentMethod,
        idempotencyKey: _generateUuid(),
      );

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Transaksi KAS berhasil disimpan.'),
            backgroundColor: AppColors.success,
          ),
        );
        // Refresh parent KAS list by popping with success
        context.pop(true);
      }
    } catch (e) {
      if (!mounted) return;
      final repository = ref.read(kasRepositoryProvider);
      setState(() {
        _error = repository.mapDioError(e).message;
        _saving = false;
      });
    }
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

    return PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, result) async {
          if (didPop) return;
          await _confirmCancel();
        },
        child: Scaffold(
          appBar: AppBar(
            centerTitle: false,
            title: const Text('Tambah Kas Keluar'),
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
                      color: AppColors.danger.withAlpha(isDark ? 51 : 26),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                        color: AppColors.danger.withAlpha(isDark ? 102 : 51),
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
                      color: AppColors.danger.withAlpha(isDark ? 38 : 20),
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

                const SectionHeader(title: 'Detail Transaksi'),
                const SizedBox(height: AppSpacing.sm),

                // Kategori
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Kategori',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    if (_loadingCategories)
                      const LinearProgressIndicator()
                    else if (_expenseCategories.isEmpty)
                      Container(
                        padding: const EdgeInsets.all(AppSpacing.md),
                        decoration: BoxDecoration(
                          border: Border.all(
                            color: isDark
                                ? AppColors.darkBorder
                                : AppColors.lightBorder,
                          ),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          color: isDark
                              ? AppColors.darkSurface
                              : AppColors.lightSurface,
                        ),
                        child: Text(
                          'Operasional RT',
                          style: theme.textTheme.bodyLarge,
                        ),
                      )
                    else
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.md,
                        ),
                        decoration: BoxDecoration(
                          border: Border.all(
                            color: isDark
                                ? AppColors.darkBorder
                                : AppColors.lightBorder,
                          ),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          color: isDark
                              ? AppColors.darkSurface
                              : AppColors.lightSurface,
                        ),
                        child: DropdownButtonHideUnderline(
                          child: DropdownButton<String>(
                            value: _selectedCategoryId,
                            isExpanded: true,
                            dropdownColor: isDark
                                ? AppColors.darkSurface
                                : AppColors.lightSurface,
                            items: _expenseCategories.map((c) {
                              return DropdownMenuItem<String>(
                                value: c.id,
                                child: Text(c.name),
                              );
                            }).toList(),
                            onChanged: (val) {
                              if (val != null) {
                                setState(() => _selectedCategoryId = val);
                              }
                            },
                          ),
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),

                // Tanggal
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Tanggal',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    InkWell(
                      onTap: () async {
                        final picked = await showDatePicker(
                          context: context,
                          initialDate: _tanggalCtrl.text.isNotEmpty
                              ? (DateTime.tryParse(_tanggalCtrl.text.trim()) ??
                                    DateTime.now())
                              : DateTime.now(),
                          firstDate: DateTime(2020),
                          lastDate: DateTime.now().add(
                            const Duration(days: 365),
                          ),
                        );
                        if (picked != null) {
                          setState(() {
                            _tanggalCtrl.text = picked
                                .toLocal()
                                .toString()
                                .split(' ')
                                .first;
                          });
                        }
                      },
                      child: Container(
                        padding: const EdgeInsets.all(AppSpacing.md),
                        decoration: BoxDecoration(
                          border: Border.all(color: AppColors.lightBorder),
                          borderRadius: BorderRadius.circular(AppRadius.base),
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

                // Metode Pembayaran (Tunai / Transfer)
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Metode Pembayaran',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Row(
                      children: [
                        Expanded(
                          child: InkWell(
                            onTap: () =>
                                setState(() => _paymentMethod = 'CASH'),
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            child: Container(
                              padding: const EdgeInsets.symmetric(vertical: 12),
                              decoration: BoxDecoration(
                                borderRadius: BorderRadius.circular(
                                  AppRadius.base,
                                ),
                                border: Border.all(
                                  color: _paymentMethod == 'CASH'
                                      ? AppColors.accent
                                      : (isDark
                                            ? AppColors.darkBorder
                                            : AppColors.lightBorder),
                                  width: _paymentMethod == 'CASH' ? 2 : 1,
                                ),
                                color: _paymentMethod == 'CASH'
                                    ? AppColors.accent.withAlpha(
                                        isDark ? 50 : 25,
                                      )
                                    : (isDark
                                          ? AppColors.darkSurface
                                          : AppColors.lightSurface),
                              ),
                              child: Row(
                                mainAxisAlignment: MainAxisAlignment.center,
                                children: [
                                  Icon(
                                    Icons.payments_outlined,
                                    color: AppColors.info,
                                    size: 16,
                                  ),
                                  const SizedBox(width: 8),
                                  Text(
                                    'Tunai (Cash)',
                                    style: TextStyle(
                                      fontWeight: _paymentMethod == 'CASH'
                                          ? FontWeight.bold
                                          : FontWeight.normal,
                                      color: _paymentMethod == 'CASH'
                                          ? AppColors.accent
                                          : null,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(width: AppSpacing.md),
                        Expanded(
                          child: InkWell(
                            onTap: () =>
                                setState(() => _paymentMethod = 'TRANSFER'),
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            child: Container(
                              padding: const EdgeInsets.symmetric(vertical: 12),
                              decoration: BoxDecoration(
                                borderRadius: BorderRadius.circular(
                                  AppRadius.base,
                                ),
                                border: Border.all(
                                  color: _paymentMethod == 'TRANSFER'
                                      ? AppColors.accent
                                      : (isDark
                                            ? AppColors.darkBorder
                                            : AppColors.lightBorder),
                                  width: _paymentMethod == 'TRANSFER' ? 2 : 1,
                                ),
                                color: _paymentMethod == 'TRANSFER'
                                    ? AppColors.accent.withAlpha(
                                        isDark ? 50 : 25,
                                      )
                                    : (isDark
                                          ? AppColors.darkSurface
                                          : AppColors.lightSurface),
                              ),
                              child: Row(
                                mainAxisAlignment: MainAxisAlignment.center,
                                children: [
                                  Icon(
                                    Icons.account_balance_outlined,
                                    color: AppColors.accent,
                                    size: 16,
                                  ),
                                  const SizedBox(width: 8),
                                  Text(
                                    'Transfer Bank',
                                    style: TextStyle(
                                      fontWeight: _paymentMethod == 'TRANSFER'
                                          ? FontWeight.bold
                                          : FontWeight.normal,
                                      color: _paymentMethod == 'TRANSFER'
                                          ? AppColors.accent
                                          : null,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),

                // Nominal
                AppTextField(
                  controller: _nominalCtrl,
                  label: 'Nominal (Rp)',
                  hintText: 'Masukkan nominal',
                  keyboardType: TextInputType.number,
                  inputFormatters: [RupiahInputFormatter()],
                  onChanged: (_) => setState(() {}),
                ),
                const SizedBox(height: AppSpacing.sm),
                // Live preview formatted amount
                if (_nominalCtrl.text.trim().isNotEmpty &&
                    int.tryParse(_nominalCtrl.text.trim()) != null)
                  Padding(
                    padding: const EdgeInsets.only(
                      left: 16,
                      bottom: AppSpacing.lg,
                    ),
                    child: Text(
                      'Format: -${RupiahFormatter.format(int.parse(_nominalCtrl.text.trim()))}',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: AppColors.danger,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),

                // Keterangan
                AppTextField(
                  controller: _keteranganCtrl,
                  label: 'Keterangan',
                  hintText: 'Deskripsi transaksi',
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
                              color: AppColors.danger.withValues(alpha: 0.15),
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
                              backgroundColor: AppColors.accent.withValues(
                                alpha: isDark ? 0.15 : 0.08,
                              ),
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
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _confirmCancel,
                          style: OutlinedButton.styleFrom(
                            side: const BorderSide(color: AppColors.danger),
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
                  ],
                ),
              ],
            ),
        ),
      ),
    );
  }
}
