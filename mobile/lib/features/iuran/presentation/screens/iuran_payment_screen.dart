import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';

import '../../data/iuran_api_models.dart';
import '../../data/iuran_models.dart';
import '../../data/iuran_providers.dart';
import '../../data/iuran_repository.dart';

/// Payment form screen for making iuran payments.
class IuranPaymentScreen extends ConsumerStatefulWidget {
  final String billId;

  const IuranPaymentScreen({super.key, required this.billId});

  @override
  ConsumerState<IuranPaymentScreen> createState() => _IuranPaymentScreenState();
}

class _IuranPaymentScreenState extends ConsumerState<IuranPaymentScreen> {
  final _formKey = GlobalKey<FormState>();
  final _tanggalCtrl = TextEditingController();
  final _nominalCtrl = TextEditingController();
  final _catatanCtrl = TextEditingController();

  PaymentMethod? _selectedMethod;
  Uint8List? _proofBytes;
  String? _proofFileName;
  bool _saving = false;
  String? _error;
  bool _nominalPrefilled = false;

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

  Future<void> _pickImage(ImageSource source) async {
    try {
      final picker = ImagePicker();
      final pickedFile = await picker.pickImage(
        source: source,
        maxWidth: 1920,
        maxHeight: 1920,
        imageQuality: 85,
      );
      if (pickedFile != null) {
        final bytes = await pickedFile.readAsBytes();
        setState(() {
          _proofBytes = bytes;
          _proofFileName = pickedFile.name;
        });
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gagal memilih gambar: $e'),
            backgroundColor: AppColors.warning,
          ),
        );
      }
    }
  }

  void _removeProof() {
    setState(() {
      _proofBytes = null;
      _proofFileName = null;
    });
  }

  Future<void> _save() async {
    if (!_validate()) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    try {
      final billState = ref.read(billDetailProvider(widget.billId));
      final bill = switch (billState) {
        AsyncData(:final value) => value,
        _ => null,
      };
      final isRegularDues = bill != null && bill.isRegularDues;
      final isFullPayment = bill != null && bill.isFullPayment;
      final isAmountLocked = isRegularDues && isFullPayment;
      final amount = isAmountLocked
          ? (bill.paymentCapacity > 0 ? bill.paymentCapacity : bill.nominal)
          : parseRupiahInput(_nominalCtrl.text);
      if (amount == null || amount <= 0) {
        if (mounted) {
          setState(
            () => _error = 'Nominal pembayaran harus berupa angka yang valid.',
          );
        }
        return;
      }
      final repo = ref.read(iuranRepositoryProvider);

      final payment = await repo.createPayment(
        billId: widget.billId,
        amount: amount,
        method: _selectedMethod ?? PaymentMethod.cash,
        notes: _catatanCtrl.text.trim().isEmpty
            ? null
            : _catatanCtrl.text.trim(),
      );

      // If proof image was picked, upload it for the newly created payment
      if (_proofBytes != null && _proofFileName != null) {
        try {
          await repo.uploadPaymentProof(
            paymentId: payment.id,
            fileBytes: _proofBytes!,
            filename: _proofFileName!,
          );
        } catch (proofErr) {
          // Payment is created; notify user about proof upload retry if needed
          if (mounted) {
            final msg = proofErr is ServerException
                ? proofErr.message
                : proofErr.toString();
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text(
                  'Pembayaran tersimpan, namun bukti gagal diunggah: $msg',
                ),
                backgroundColor: AppColors.warning,
              ),
            );
          }
        }
      }

      // Invalidate cached providers so list/detail screens fetch fresh data
      if (mounted) {
        ref.invalidate(billDetailProvider(widget.billId));
        ref.invalidate(billPaymentsProvider(widget.billId));
        ref.invalidate(billsProvider);
      }

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
        final repo = ref.read(iuranRepositoryProvider);
        final mapped = repo.mapDioError(e);
        setState(() {
          _error = mapped.message;
          _saving = false;
        });
      }
    }
  }

  bool _validate() {
    if (!_formKey.currentState!.validate()) return false;

    final amount = parseRupiahInput(_nominalCtrl.text);
    if (amount != null) {
      final state = ref.read(billDetailProvider(widget.billId));
      if (state is AsyncData<IuranBill?> && state.value != null) {
        final bill = state.value!;
        if (bill.isRegularDues) {
          final capacity = bill.paymentCapacity;
          if (amount > capacity && capacity > 0) {
            if (mounted) {
              setState(() => _error = 'Nominal melebihi sisa tagihan.');
            }
            return false;
          }
          if (bill.isFullPayment && amount != capacity && capacity > 0) {
            if (mounted) {
              setState(
                () => _error =
                    'Iuran ini mewajibkan pembayaran lunas penuh (${formatRupiah(capacity)}).',
              );
            }
            return false;
          }
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

    // Listen to bill detail once to pre-fill nominal with remaining balance if empty
    ref.listen<AsyncValue<IuranBill?>>(billDetailProvider(widget.billId), (
      prev,
      next,
    ) {
      if (!_nominalPrefilled &&
          next is AsyncData<IuranBill?> &&
          next.value != null) {
        final b = next.value!;
        if (b.isRegularDues && _nominalCtrl.text.isEmpty && b.paymentCapacity > 0) {
          _nominalPrefilled = true;
          _nominalCtrl.text = RupiahInputFormatter.formatForInput(
            b.paymentCapacity,
          );
        }
      }
    });

    // Watch bill detail from API
    final billState = ref.watch(billDetailProvider(widget.billId));
    final currentBill = switch (billState) {
      AsyncData(:final value) => value,
      _ => null,
    };
    final isRegularDues = currentBill != null && currentBill.isRegularDues;
    final isFullPayment = currentBill != null && currentBill.isFullPayment;
    final isAmountLocked = isRegularDues && isFullPayment;

    if (currentBill != null) {
      final targetAmount = currentBill.paymentCapacity > 0
          ? currentBill.paymentCapacity
          : currentBill.nominal;
      if (isAmountLocked && targetAmount > 0) {
        final formattedFull = RupiahInputFormatter.formatForInput(targetAmount);
        if (_nominalCtrl.text != formattedFull) {
          _nominalPrefilled = true;
          _nominalCtrl.text = formattedFull;
        }
      } else if (!_nominalPrefilled &&
          _nominalCtrl.text.isEmpty &&
          isRegularDues &&
          currentBill.paymentCapacity > 0) {
        _nominalPrefilled = true;
        _nominalCtrl.text = RupiahInputFormatter.formatForInput(
          currentBill.paymentCapacity,
        );
      }
    }

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
                          onPressed: () =>
                              ref.refresh(billDetailProvider(widget.billId)),
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

              // ── Tanggal Pembayaran (otomatis hari ini) ──
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
                  Container(
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
                        Expanded(
                          child: Text(
                            _tanggalCtrl.text.isEmpty
                                ? 'Pilih tanggal'
                                : _tanggalCtrl.text,
                            style: theme.textTheme.bodyLarge?.copyWith(
                              color: isDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                            ),
                          ),
                        ),
                        const Icon(
                          Icons.lock_outline,
                          size: 16,
                          color: Colors.grey,
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // ── Metode Pembayaran ──
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
                  SegmentedButton<PaymentMethod>(
                    segments: [
                      ButtonSegment(
                        value: PaymentMethod.cash,
                        label: const Text('Tunai'),
                        icon: const Icon(
                          Icons.payments_outlined,
                          color: AppColors.info,
                        ),
                      ),
                      ButtonSegment(
                        value: PaymentMethod.transfer,
                        label: const Text('Transfer'),
                        icon: const Icon(
                          Icons.account_balance_outlined,
                          color: AppColors.accent,
                        ),
                      ),
                    ],
                    selected: {_selectedMethod ?? PaymentMethod.cash},
                    onSelectionChanged: (selection) {
                      setState(() {
                        _selectedMethod = selection.first;
                      });
                    },
                    style: SegmentedButton.styleFrom(
                      side: const BorderSide(color: AppColors.lightBorder),
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
                readOnly: isAmountLocked,
                enabled: !isAmountLocked,
                suffixIcon: isAmountLocked
                    ? const Icon(Icons.lock_outline, size: 18)
                    : null,
                keyboardType: TextInputType.number,
                inputFormatters: [RupiahInputFormatter()],
                validator: (value) {
                  if (value == null || value.trim().isEmpty) {
                    return 'Nominal harus diisi.';
                  }
                  final amount = parseRupiahInput(value);
                  if (amount == null || amount <= 0) {
                    return 'Nominal harus berupa angka.';
                  }
                  final state = ref.read(billDetailProvider(widget.billId));
                  if (state is AsyncData<IuranBill?> && state.value != null) {
                    final bill = state.value!;
                    if (bill.isRegularDues) {
                      if (amount > bill.paymentCapacity &&
                          bill.paymentCapacity > 0) {
                        return 'Nominal melebihi sisa tagihan.';
                      }
                      if (bill.isFullPayment &&
                          amount != bill.paymentCapacity &&
                          bill.paymentCapacity > 0) {
                        return 'Iuran ini mewajibkan pembayaran lunas penuh (${formatRupiah(bill.paymentCapacity)}).';
                      }
                    }
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
              const SizedBox(height: AppSpacing.lg),

              // ── Bukti Pembayaran (Upload Bukti) ──
              Text(
                'Bukti Pembayaran (Opsional)',
                style: theme.textTheme.titleSmall?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(
                'Unggah foto resi atau bukti transfer (JPG, PNG, WebP maks 5MB)',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: isDark
                      ? AppColors.darkTextMuted
                      : AppColors.lightTextMuted,
                ),
              ),
              const SizedBox(height: AppSpacing.sm),
              if (_proofBytes != null) ...[
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.darkSurface
                        : AppColors.lightSurfaceSubtle,
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    border: Border.all(
                      color: AppColors.accent.withValues(alpha: 0.3),
                    ),
                  ),
                  child: Row(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        child: Image.memory(
                          _proofBytes!,
                          width: 56,
                          height: 56,
                          fit: BoxFit.cover,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              _proofFileName ?? 'bukti_pembayaran.jpg',
                              style: theme.textTheme.bodyMedium?.copyWith(
                                fontWeight: FontWeight.w500,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                            const SizedBox(height: 2),
                            Text(
                              '${(_proofBytes!.lengthInBytes / 1024).toStringAsFixed(1)} KB',
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: isDark
                                    ? AppColors.darkTextMuted
                                    : AppColors.lightTextMuted,
                              ),
                            ),
                          ],
                        ),
                      ),
                      IconButton(
                        icon: const Icon(
                          Icons.delete_outline,
                          color: AppColors.danger,
                        ),
                        onPressed: _removeProof,
                        tooltip: 'Hapus Bukti',
                      ),
                    ],
                  ),
                ),
              ] else ...[
                Row(
                  children: [
                    Expanded(
                      child: OutlinedButton.icon(
                        onPressed: () => _pickImage(ImageSource.gallery),
                        icon: const Icon(
                          Icons.photo_library_outlined,
                          size: 18,
                        ),
                        label: const Text('Galeri'),
                        style: OutlinedButton.styleFrom(
                          foregroundColor: AppColors.accent,
                          side: const BorderSide(color: AppColors.accent),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: OutlinedButton.icon(
                        onPressed: () => _pickImage(ImageSource.camera),
                        icon: const Icon(Icons.camera_alt_outlined, size: 18),
                        label: const Text('Kamera'),
                        style: OutlinedButton.styleFrom(
                          foregroundColor: AppColors.accent,
                          side: const BorderSide(color: AppColors.accent),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
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
                          key: const Key('btn_save'),
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
                              ? const SizedBox(
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
                                    Icon(Icons.payment, size: 18),
                                    SizedBox(width: 6),
                                    Text('Bayar'),
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
                          key: const Key('btn_cancel'),
                          onPressed: _saving ? null : () => _confirmCancel(),
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

  Widget _buildBillSummary(ThemeData theme, bool isDark, IuranBill bill) {
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
              Icon(Icons.home_outlined, size: 18, color: AppColors.accent),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  'Rumah',
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
              AppBadge(
                label: bill.periode.toUpperCase(),
                backgroundColor: isDark
                    ? AppColors.darkAccentSoft
                    : AppColors.lightAccentSoft,
                textColor: AppColors.accent,
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
                  bill.isVoluntaryDues ? 'Sifat Tagihan' : 'Sisa Tagihan',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              Text(
                bill.isVoluntaryDues
                    ? 'Sukarela'
                    : formatRupiah(bill.remainingAmount),
                style: theme.textTheme.titleLarge?.copyWith(
                  color: bill.isVoluntaryDues
                      ? AppColors.info
                      : AppColors.accent,
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
