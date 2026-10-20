import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';

bool _hasKasWriteAccess(WidgetRef ref) {
  final authState = ref.read(authRepositoryProvider);
  final systemRole = authState?.valueOrNull?['system_role'] as String?;
  if (systemRole == 'super_admin') return true;

  final jabatan = authState?.valueOrNull?['jabatan'] as String?;
  if (jabatan != null &&
      (jabatan == 'ketua' ||
          jabatan == 'bendahara' ||
          jabatan == 'sekretaris' ||
          jabatan == 'wakil_ketua')) {
    return true;
  }

  final role = authState?.valueOrNull?['role'] as AppRole?;
  return role == AppRole.bendahara || role == AppRole.pengurus;
}

String cleanKasDescription(String desc) {
  if (desc.isEmpty) return '';
  return desc
      .replaceAll(
        RegExp(r'\s*\(Bill:\s*[a-f0-9-]+\)', caseSensitive: false),
        '',
      )
      .replaceAll(RegExp(r'\s*\(Period:\s*[^)]+\)', caseSensitive: false), '')
      .replaceAll(
        RegExp(r'\s*Payment for bill\s*[a-f0-9-]+', caseSensitive: false),
        'Pembayaran Iuran',
      )
      .replaceAll(
        RegExp(r'\s*Voluntary contribution for due\s+', caseSensitive: false),
        'Iuran Sukarela: ',
      )
      .trim();
}

/// Detail screen for a single KAS transaction — connected to real backend.
class KasDetailScreen extends ConsumerStatefulWidget {
  final String transactionId;

  const KasDetailScreen({super.key, required this.transactionId});

  @override
  ConsumerState<KasDetailScreen> createState() => _KasDetailScreenState();
}

class _KasDetailScreenState extends ConsumerState<KasDetailScreen> {
  KasTransaction? _transaction;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadTransaction();
  }

  Future<void> _loadTransaction() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });

    final repository = ref.read(kasRepositoryProvider);

    try {
      final tx = await repository.getTransaction(widget.transactionId);
      if (!mounted) return;
      setState(() {
        _loading = false;
        _transaction = tx;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = repository.mapDioError(e).message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    if (_loading) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => context.pop(),
            tooltip: 'Kembali',
          ),
          title: const Text('Detail Transaksi'),
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_error != null || _transaction == null) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => context.pop(),
            tooltip: 'Kembali',
          ),
          title: const Text('Detail Transaksi'),
        ),
        body: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.error_outline, size: 48, color: Colors.red),
                const SizedBox(height: AppSpacing.md),
                Text(
                  _transaction == null
                      ? 'Transaksi tidak ditemukan'
                      : 'Gagal memuat data',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                if (_error != null)
                  Text(
                    _error!,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                const SizedBox(height: AppSpacing.lg),
                ElevatedButton.icon(
                  onPressed: () => context.pop(),
                  icon: const Icon(Icons.arrow_back),
                  label: const Text('Kembali'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    final tx = _transaction!;
    final isIncoming = tx.jenis == KasJenis.masuk;
    final accentColor = isIncoming ? AppColors.success : AppColors.danger;
    final icon = isIncoming ? Icons.arrow_downward : Icons.arrow_upward;
    final canWrite = _hasKasWriteAccess(ref);
    final isFromIuran =
        (tx.dueName != null && tx.dueName!.isNotEmpty) ||
        (tx.referensi != null && tx.referensi!.startsWith('PAY-')) ||
        tx.kategori == KasCategory.iuranWarga ||
        tx.kategori == KasCategory.iuranPaksa;
    final isManual = !isFromIuran;

    final String formattedKeterangan = isFromIuran
        ? ''
        : cleanKasDescription(tx.keterangan);

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
          tooltip: 'Kembali',
        ),
        title: const Text('Detail Transaksi'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.base),
        children: [
          // Header with amount
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: accentColor.withValues(alpha: isDark ? 0.15 : 0.08),
              borderRadius: BorderRadius.circular(AppRadius.lg),
              border: Border.all(
                color: accentColor.withValues(alpha: isDark ? 0.3 : 0.2),
                width: 1,
              ),
            ),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: accentColor.withValues(alpha: isDark ? 0.25 : 0.15),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                  ),
                  child: Icon(icon, size: 24, color: accentColor),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        isIncoming ? 'Pemasukan' : 'Pengeluaran',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          color: accentColor,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        '${isIncoming ? '+' : '-'}${RupiahFormatter.formatMoneyString(tx.nominal)}',
                        style: theme.textTheme.headlineSmall?.copyWith(
                          color: accentColor,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          const SizedBox(height: AppSpacing.lg),

          // Detail rows
          AppCard(
            padding: const EdgeInsets.all(AppSpacing.base),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _DetailRow(
                  label: 'Tanggal',
                  value: formatTanggal(tx.tanggal),
                  isDark: isDark,
                  theme: theme,
                ),
                const Divider(height: 24),
                _DetailRow(
                  label: 'Jenis',
                  value: isIncoming ? 'Masuk' : 'Keluar',
                  isDark: isDark,
                  theme: theme,
                  badge: true,
                  badgeColor: accentColor,
                ),
                const Divider(height: 24),
                _DetailRow(
                  label: 'Kategori',
                  value: tx.kategori.label,
                  isDark: isDark,
                  theme: theme,
                ),
                const Divider(height: 24),
                _DetailRow(
                  label: 'Keterangan',
                  value: formattedKeterangan,
                  isDark: isDark,
                  theme: theme,
                ),
                if (tx.paymentMethod != null &&
                    tx.paymentMethod!.isNotEmpty) ...[
                  const Divider(height: 24),
                  _DetailRow(
                    label: 'Metode',
                    value: tx.paymentMethod == 'CASH'
                        ? 'Tunai (Cash)'
                        : 'Transfer Bank',
                    isDark: isDark,
                    theme: theme,
                  ),
                ],
                if (tx.referensi != null && tx.referensi!.isNotEmpty) ...[
                  const Divider(height: 24),
                  _DetailRow(
                    label: 'Referensi',
                    value: tx.referensi!,
                    isDark: isDark,
                    theme: theme,
                  ),
                ],
                if (tx.officerName != null && tx.officerName!.isNotEmpty) ...[
                  const Divider(height: 24),
                  _DetailRow(
                    label: 'Pengurus',
                    value: tx.officerName!,
                    isDark: isDark,
                    theme: theme,
                  ),
                ],
                const Divider(height: 24),
                _DetailRow(
                  label: 'Status',
                  value: 'Posted',
                  isDark: isDark,
                  theme: theme,
                  badge: true,
                  badgeColor: AppColors.success,
                ),
              ],
            ),
          ),

          if (canWrite && isManual) ...[
            const SizedBox(height: AppSpacing.lg),
            SizedBox(
              height: 48,
              child: OutlinedButton.icon(
                onPressed: () => _showEditTransactionDialog(tx),
                icon: const Icon(Icons.edit_outlined, size: 18),
                label: const Text('Ubah Transaksi'),
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: AppColors.accent),
                  foregroundColor: AppColors.accent,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                ),
              ),
            ),
          ],

          const SizedBox(height: AppSpacing.xxl),
        ],
      ),
    );
  }

  Future<void> _showEditTransactionDialog(KasTransaction tx) async {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final nominalCtrl = TextEditingController(
      text: tx.nominal.replaceAll(RegExp(r'[^0-9]'), ''),
    );
    final keteranganCtrl = TextEditingController(text: tx.keterangan);
    final tanggalCtrl = TextEditingController(
      text:
          '${tx.tanggal.year}-${tx.tanggal.month.toString().padLeft(2, '0')}-${tx.tanggal.day.toString().padLeft(2, '0')}',
    );
    String paymentMethod = (tx.paymentMethod ?? 'CASH').toUpperCase();
    if (paymentMethod != 'CASH' && paymentMethod != 'TRANSFER') {
      paymentMethod = 'CASH';
    }

    List<BackendCategory> categories = [];
    String? selectedCategoryId;
    bool loadingCats = true;
    bool saving = false;
    String? error;

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (statefulCtx, setDialogState) {
            if (loadingCats) {
              final repo = ref.read(kasRepositoryProvider);
              repo
                  .fetchCategories(isActive: true)
                  .then((cats) {
                    final targetType = tx.jenis == KasJenis.masuk
                        ? 'income'
                        : 'expense';
                    final filtered = cats
                        .where((c) => c.type == targetType)
                        .toList();
                    String? defaultCatId;
                    for (final c in filtered) {
                      if (c.name.toLowerCase() ==
                          tx.kategori.label.toLowerCase()) {
                        defaultCatId = c.id;
                        break;
                      }
                    }
                    if (defaultCatId == null && filtered.isNotEmpty) {
                      defaultCatId = filtered.first.id;
                    }
                    setDialogState(() {
                      categories = filtered;
                      selectedCategoryId = defaultCatId;
                      loadingCats = false;
                    });
                  })
                  .catchError((_) {
                    setDialogState(() {
                      loadingCats = false;
                    });
                  });
            }

            return AlertDialog(
              title: const Text('Ubah Transaksi Manual'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (error != null) ...[
                      Text(
                        error!,
                        style: const TextStyle(
                          color: AppColors.danger,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.sm),
                    ],
                    // Tanggal
                    Text(
                      'Tanggal',
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    InkWell(
                      onTap: () async {
                        final picked = await showDatePicker(
                          context: context,
                          initialDate:
                              DateTime.tryParse(tanggalCtrl.text) ??
                              DateTime.now(),
                          firstDate: DateTime(2020),
                          lastDate: DateTime(2035),
                        );
                        if (picked != null) {
                          setDialogState(() {
                            tanggalCtrl.text =
                                '${picked.year}-${picked.month.toString().padLeft(2, '0')}-${picked.day.toString().padLeft(2, '0')}';
                          });
                        }
                      },
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 10,
                        ),
                        decoration: BoxDecoration(
                          border: Border.all(
                            color: isDark
                                ? AppColors.darkBorder
                                : AppColors.lightBorder,
                          ),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                        ),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text(tanggalCtrl.text),
                            const Icon(Icons.calendar_today, size: 16),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    // Kategori
                    if (categories.isNotEmpty) ...[
                      Text(
                        'Kategori',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 12),
                        decoration: BoxDecoration(
                          border: Border.all(
                            color: isDark
                                ? AppColors.darkBorder
                                : AppColors.lightBorder,
                          ),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                        ),
                        child: DropdownButtonHideUnderline(
                          child: DropdownButton<String>(
                            value: selectedCategoryId,
                            isExpanded: true,
                            items: categories
                                .map(
                                  (c) => DropdownMenuItem(
                                    value: c.id,
                                    child: Text(c.name),
                                  ),
                                )
                                .toList(),
                            onChanged: (val) {
                              if (val != null) {
                                setDialogState(() => selectedCategoryId = val);
                              }
                            },
                          ),
                        ),
                      ),
                      const SizedBox(height: AppSpacing.md),
                    ],
                    // Nominal
                    AppTextField(
                      controller: nominalCtrl,
                      label: 'Nominal (Rp)',
                      keyboardType: TextInputType.number,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    // Keterangan
                    AppTextField(
                      controller: keteranganCtrl,
                      label: 'Keterangan',
                    ),
                    const SizedBox(height: AppSpacing.md),
                    // Metode
                    Text(
                      'Metode Pembayaran',
                      style: theme.textTheme.bodySmall?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        ChoiceChip(
                          label: const Text('Tunai'),
                          selected: paymentMethod == 'CASH',
                          onSelected: (val) =>
                              setDialogState(() => paymentMethod = 'CASH'),
                        ),
                        const SizedBox(width: 8),
                        ChoiceChip(
                          label: const Text('Transfer'),
                          selected: paymentMethod == 'TRANSFER',
                          onSelected: (val) =>
                              setDialogState(() => paymentMethod = 'TRANSFER'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: saving ? null : () => Navigator.pop(dialogCtx),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  onPressed: saving
                      ? null
                      : () async {
                          final amountInt =
                              int.tryParse(nominalCtrl.text.trim()) ?? 0;
                          if (amountInt <= 0) {
                            setDialogState(
                              () => error = 'Nominal harus lebih besar dari 0',
                            );
                            return;
                          }
                          setDialogState(() {
                            saving = true;
                            error = null;
                          });
                          try {
                            final repo = ref.read(kasRepositoryProvider);
                            final occ =
                                DateTime.tryParse(tanggalCtrl.text) ??
                                tx.tanggal;
                            await repo.updateTransaction(
                              id: tx.id,
                              categoryId: selectedCategoryId,
                              amount: amountInt,
                              jenis: tx.jenis,
                              description: keteranganCtrl.text.trim(),
                              occurredAt: occ,
                              paymentMethod: paymentMethod,
                            );
                            if (dialogCtx.mounted) {
                              Navigator.pop(dialogCtx);
                            }
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text(
                                    'Transaksi berhasil diperbarui.',
                                  ),
                                  backgroundColor: AppColors.success,
                                ),
                              );
                              _loadTransaction();
                            }
                          } catch (e) {
                            setDialogState(() {
                              saving = false;
                              error = 'Gagal memperbarui transaksi: $e';
                            });
                          }
                        },
                  child: saving
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Simpan'),
                ),
              ],
            );
          },
        );
      },
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isDark;
  final ThemeData theme;
  final bool badge;
  final Color? badgeColor;

  const _DetailRow({
    required this.label,
    required this.value,
    required this.isDark,
    required this.theme,
    this.badge = false,
    this.badgeColor,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: badge && badgeColor != null
              ? AppBadge(
                  label: value,
                  backgroundColor: badgeColor!.withValues(alpha: 0.12),
                  textColor: badgeColor,
                )
              : Text(value, style: theme.textTheme.bodyMedium),
        ),
      ],
    );
  }
}
