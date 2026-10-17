import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';

bool _hasKasWriteAccess(WidgetRef ref) {
  final authState = ref.read(authRepositoryProvider);
  final systemRole = authState?.valueOrNull?['system_role'] as String?;
  if (systemRole == 'super_admin') return true;

  final jabatan = authState?.valueOrNull?['jabatan'] as String?;
  if (jabatan != null && (jabatan == 'ketua' || jabatan == 'bendahara' || jabatan == 'sekretaris' || jabatan == 'wakil_ketua')) {
    return true;
  }

  final role = authState?.valueOrNull?['role'] as AppRole?;
  return role == AppRole.bendahara || role == AppRole.pengurus;
}

/// Main KAS (Cash) screen with transaction list — connected to real backend.
class KasScreen extends ConsumerStatefulWidget {
  const KasScreen({super.key});

  @override
  ConsumerState<KasScreen> createState() => _KasScreenState();
}

class _KasScreenState extends ConsumerState<KasScreen> {
  late final TextEditingController _searchController;

  static const _monthNames = [
    'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
    'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember',
  ];

  DateTime _activeDate = DateTime.now();
  bool _isAllPeriods = false;

  // Filter states
  String _filterArus = 'all'; // 'all', 'masuk', 'keluar'
  String _filterMetode = 'all'; // 'all', 'CASH', 'TRANSFER'

  // Data state
  bool _loading = true;
  String? _error;
  List<KasTransaction> _allTransactions = [];
  List<KasTransaction> _filteredTransactions = [];
  BackendBalance? _balance;
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    _loadData();
  }

  @override
  void dispose() {
    _searchController.dispose();
    _debounce?.cancel();
    super.dispose();
  }

  String _formatMonthYear(DateTime dt) {
    return '${_monthNames[dt.month - 1]} ${dt.year}';
  }

  void _prevMonth() {
    setState(() {
      _isAllPeriods = false;
      _activeDate = DateTime(_activeDate.year, _activeDate.month - 1, 1);
    });
    _loadData();
  }

  void _nextMonth() {
    setState(() {
      _isAllPeriods = false;
      _activeDate = DateTime(_activeDate.year, _activeDate.month + 1, 1);
    });
    _loadData();
  }

  Future<void> _pickMonth() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _activeDate,
      firstDate: DateTime(2020),
      lastDate: DateTime(2035),
      initialDatePickerMode: DatePickerMode.year,
      helpText: 'PILIH PERIODE BULAN',
    );
    if (picked != null) {
      setState(() {
        _isAllPeriods = false;
        _activeDate = DateTime(picked.year, picked.month, 1);
      });
      _loadData();
    }
  }

  Future<void> _loadData() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });

    final repository = ref.read(kasRepositoryProvider);
    final periodParam = _isAllPeriods
        ? null
        : '${_activeDate.year}-${_activeDate.month.toString().padLeft(2, '0')}';

    try {
      final futureTransactions = repository.fetchAllTransactions(period: periodParam);
      final futureBalance = repository.fetchBalance(period: periodParam);

      final results = await Future.wait([
        futureTransactions,
        futureBalance,
      ]);

      if (!mounted) return;

      _allTransactions = results[0] as List<KasTransaction>;
      _balance = results[1] as BackendBalance;

      setState(() {
        _loading = false;
        _applyFilters(_allTransactions);
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = repository.mapDioError(e).message;
      });
    }
  }

  void _clearSearch() {
    _searchController.clear();
    _refreshFiltered();
  }

  void _onSearchChanged(String query) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 300), () {
      if (mounted) _refreshFiltered();
    });
  }

  void _refreshFiltered() {
    setState(() {
      _applyFilters(_allTransactions);
    });
  }

  void _applyFilters(List<KasTransaction> all) {
    final query = _searchController.text.trim().toLowerCase();
    _filteredTransactions = all.where((tx) {
      if (query.isNotEmpty &&
          !tx.keterangan.toLowerCase().contains(query) &&
          !tx.kategori.label.toLowerCase().contains(query) &&
          !(tx.citizenName ?? '').toLowerCase().contains(query) &&
          !(tx.dueName ?? '').toLowerCase().contains(query) &&
          !(tx.houseNumber ?? '').toLowerCase().contains(query)) {
        return false;
      }
      if (_filterArus == 'masuk' && tx.jenis != KasJenis.masuk) {
        return false;
      }
      if (_filterArus == 'keluar' && tx.jenis != KasJenis.keluar) {
        return false;
      }
      if (_filterMetode != 'all') {
        final m = (tx.paymentMethod ?? '').toUpperCase();
        if (_filterMetode == 'CASH' && m != 'CASH' && m != 'TUNAI') {
          return false;
        }
        if (_filterMetode == 'TRANSFER' && m != 'TRANSFER' && m != 'BANK') {
          return false;
        }
      }
      return true;
    }).toList();
  }

  Future<void> _showEditOpeningBalanceDialog(int currentOpening) async {
    final periodKey = '${_activeDate.year}-${_activeDate.month.toString().padLeft(2, '0')}';
    final nominalCtrl = TextEditingController(text: currentOpening > 0 ? currentOpening.toString() : '0');
    final reasonCtrl = TextEditingController();
    bool saving = false;
    String? error;

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (statefulCtx, setDialogState) {
            return AlertDialog(
              title: Text('Ubah Saldo Awal (${_formatMonthYear(_activeDate)})'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      padding: const EdgeInsets.all(AppSpacing.sm),
                      decoration: BoxDecoration(
                        color: AppColors.info.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        border: Border.all(color: AppColors.info.withValues(alpha: 0.3)),
                      ),
                      child: Text(
                        'Mengubah saldo awal memengaruhi saldo akhir dan riwayat neraca periode ini.',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: AppColors.info,
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.md),
                    if (error != null) ...[
                      Text(error!, style: const TextStyle(color: AppColors.danger, fontSize: 12)),
                      const SizedBox(height: AppSpacing.sm),
                    ],
                    AppTextField(
                      controller: nominalCtrl,
                      label: 'Saldo Awal (Rp)',
                      keyboardType: TextInputType.number,
                    ),
                    const SizedBox(height: AppSpacing.md),
                    AppTextField(
                      controller: reasonCtrl,
                      label: 'Alasan Penyesuaian (opsional)',
                      hintText: 'Misal: Penyesuaian kas fisik',
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
                          final val = int.tryParse(nominalCtrl.text.trim()) ?? 0;
                          setDialogState(() {
                            saving = true;
                            error = null;
                          });
                          try {
                            final repo = ref.read(kasRepositoryProvider);
                            await repo.updateOpeningBalance(
                              period: periodKey,
                              openingBalance: val.toString(),
                              reason: reasonCtrl.text.trim().isNotEmpty
                                  ? reasonCtrl.text.trim()
                                  : null,
                            );
                            if (dialogCtx.mounted) {
                              Navigator.pop(dialogCtx);
                            }
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text('Saldo awal berhasil diperbarui.'),
                                  backgroundColor: AppColors.success,
                                ),
                              );
                              _loadData();
                            }
                          } catch (e) {
                            setDialogState(() {
                              saving = false;
                              error = 'Gagal menyimpan saldo awal: $e';
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

  void _showAddTransactionDialog(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.lg)),
      ),
      builder: (bottomSheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                ListTile(
                  leading: Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.success.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: const Icon(Icons.arrow_downward, color: AppColors.success),
                  ),
                  title: const Text('Catat Kas Masuk'),
                  subtitle: const Text('Catat pemasukan kas baru di luar iuran warga'),
                  onTap: () {
                    Navigator.pop(bottomSheetContext);
                    context.push('/home/kas/masuk').then((_) => _loadData());
                  },
                ),
                ListTile(
                  leading: Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.danger.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: const Icon(Icons.arrow_upward, color: AppColors.danger),
                  ),
                  title: const Text('Catat Kas Keluar'),
                  subtitle: const Text('Catat pengeluaran kas atau biaya operasional'),
                  onTap: () {
                    Navigator.pop(bottomSheetContext);
                    context.push('/home/kas/keluar').then((_) => _loadData());
                  },
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    if (_loading) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Kas'),
          actions: [
            IconButton(
              icon: const Icon(Icons.bar_chart_outlined, size: 20),
              tooltip: 'Laporan KAS',
              onPressed: () => context.push('/home/kas/laporan'),
            ),
            const SizedBox(width: 8),
          ],
        ),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_error != null) {
      return Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Kas'),
          actions: [
            IconButton(
              icon: const Icon(Icons.bar_chart_outlined, size: 20),
              tooltip: 'Laporan KAS',
              onPressed: () => context.push('/home/kas/laporan'),
            ),
            const SizedBox(width: 8),
          ],
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
                  'Gagal memuat data',
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  _error!,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                ElevatedButton.icon(
                  onPressed: _loadData,
                  icon: const Icon(Icons.refresh),
                  label: const Text('Coba Lagi'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    // Calculate summary from real balance
    final openingBalance = RupiahFormatter.parseMoneyString(_balance?.openingBalance ?? '0');
    final totalMasuk = RupiahFormatter.parseMoneyString(_balance?.totalIncome ?? '0');
    final totalKeluar = RupiahFormatter.parseMoneyString(_balance?.totalExpense ?? '0');
    final netMovement = RupiahFormatter.parseMoneyString(_balance?.netMovement ?? (_balance?.netBalance ?? '0'));
    final closingBalance = RupiahFormatter.parseMoneyString(_balance?.closingBalance ?? (_balance?.netBalance ?? '0'));
    final canWrite = _hasKasWriteAccess(ref);

    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Kas'),
          actions: [
            IconButton(
              icon: const Icon(Icons.bar_chart_outlined, size: 20),
              tooltip: 'Laporan KAS',
              onPressed: () => context.push('/home/kas/laporan'),
            ),
            const SizedBox(width: 8),
          ],
        ),
        floatingActionButton: canWrite
            ? Container(
                decoration: BoxDecoration(
                  color: AppColors.accentSoftColor(context),
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
                    onTap: () => _showAddTransactionDialog(context),
                    borderRadius: BorderRadius.circular(24),
                    child: const SizedBox(
                      width: 48,
                      height: 48,
                      child: Icon(Icons.add, size: 24, color: AppColors.accent),
                    ),
                  ),
                ),
              )
            : null,
        body: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const SizedBox(height: AppSpacing.sm),
            // ── 5 Neraca Summary Cards (Positioned at top under title KAS) ──
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
              child: Column(
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: SummaryChip(
                          label: 'Saldo Awal',
                          value: RupiahFormatter.format(openingBalance),
                          color: theme.colorScheme.onSurface,
                          isEditable: canWrite && !_isAllPeriods,
                          onEdit: () => _showEditOpeningBalanceDialog(openingBalance),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: SummaryChip(
                          label: 'Saldo Akhir',
                          value: RupiahFormatter.format(closingBalance),
                          color: AppColors.accent,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Row(
                    children: [
                      Expanded(
                        child: SummaryChip(
                          label: 'Total Masuk',
                          value: '+${RupiahFormatter.format(totalMasuk)}',
                          color: AppColors.success,
                          icon: Icons.arrow_downward,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: SummaryChip(
                          label: 'Total Keluar',
                          value: '-${RupiahFormatter.format(totalKeluar)}',
                          color: AppColors.danger,
                          icon: Icons.arrow_upward,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: SummaryChip(
                          label: 'Mutasi Bersih',
                          value: '${netMovement >= 0 ? '+' : '-'}${RupiahFormatter.format(netMovement.abs())}',
                          color: netMovement >= 0 ? AppColors.success : AppColors.danger,
                          icon: netMovement >= 0 ? Icons.arrow_downward : Icons.arrow_upward,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.sm),

            // ── Month Navigator & Filter Periode ──
            Container(
              margin: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                borderRadius: BorderRadius.circular(AppRadius.base),
                border: Border.all(
                  color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
                ),
              ),
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: AppSpacing.xs),
              child: Row(
                children: [
                  IconButton(
                    icon: const Icon(Icons.chevron_left),
                    onPressed: _isAllPeriods ? null : _prevMonth,
                    tooltip: 'Bulan Sebelumnya',
                    visualDensity: VisualDensity.compact,
                  ),
                  Expanded(
                    child: InkWell(
                      onTap: _isAllPeriods ? null : _pickMonth,
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 4, horizontal: 8),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Text(
                              _isAllPeriods
                                  ? 'Semua Periode'
                                  : _formatMonthYear(_activeDate),
                              style: theme.textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                                color: _isAllPeriods ? theme.colorScheme.onSurfaceVariant : AppColors.accent,
                              ),
                            ),
                            if (!_isAllPeriods) ...[
                              const SizedBox(width: 4),
                              const Icon(Icons.arrow_drop_down, size: 20, color: AppColors.accent),
                            ],
                          ],
                        ),
                      ),
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.chevron_right),
                    onPressed: _isAllPeriods ? null : _nextMonth,
                    tooltip: 'Bulan Berikutnya',
                    visualDensity: VisualDensity.compact,
                  ),
                  const SizedBox(width: 4),
                  ChoiceChip(
                    label: Text(_isAllPeriods ? 'Per Bulan' : 'Semua', style: const TextStyle(fontSize: 11)),
                    selected: _isAllPeriods,
                    visualDensity: VisualDensity.compact,
                    onSelected: (val) {
                      setState(() {
                        _isAllPeriods = !_isAllPeriods;
                      });
                      _loadData();
                    },
                  ),
                ],
              ),
            ),

            // ── Arus Kas & Metode Filter Row ──
            const SizedBox(height: AppSpacing.xs),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
              child: Row(
                children: [
                  Expanded(
                    child: Container(
                      height: 38,
                      padding: const EdgeInsets.symmetric(horizontal: 10),
                      decoration: BoxDecoration(
                        color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                        borderRadius: BorderRadius.circular(AppRadius.base),
                        border: Border.all(
                          color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
                        ),
                      ),
                      child: DropdownButtonHideUnderline(
                        child: DropdownButton<String>(
                          value: _filterArus,
                          isExpanded: true,
                          style: theme.textTheme.bodySmall?.copyWith(fontWeight: FontWeight.w600),
                          dropdownColor: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                          items: const [
                            DropdownMenuItem(value: 'all', child: Text('Semua Arus Kas')),
                            DropdownMenuItem(value: 'masuk', child: Text('Pemasukan (Masuk)')),
                            DropdownMenuItem(value: 'keluar', child: Text('Pengeluaran (Keluar)')),
                          ],
                          onChanged: (val) {
                            if (val != null) {
                              setState(() => _filterArus = val);
                              _refreshFiltered();
                            }
                          },
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Container(
                      height: 38,
                      padding: const EdgeInsets.symmetric(horizontal: 10),
                      decoration: BoxDecoration(
                        color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                        borderRadius: BorderRadius.circular(AppRadius.base),
                        border: Border.all(
                          color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
                        ),
                      ),
                      child: DropdownButtonHideUnderline(
                        child: DropdownButton<String>(
                          value: _filterMetode,
                          isExpanded: true,
                          style: theme.textTheme.bodySmall?.copyWith(fontWeight: FontWeight.w600),
                          dropdownColor: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                          items: const [
                            DropdownMenuItem(value: 'all', child: Text('Semua Metode')),
                            DropdownMenuItem(value: 'CASH', child: Text('Tunai (Cash)')),
                            DropdownMenuItem(value: 'TRANSFER', child: Text('Transfer Bank')),
                          ],
                          onChanged: (val) {
                            if (val != null) {
                              setState(() => _filterMetode = val);
                              _refreshFiltered();
                            }
                          },
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // ── Search ──
            const SizedBox(height: AppSpacing.xs),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base),
              child: AppTextField(
                controller: _searchController,
                hintText: 'Cari transaksi...',
                prefixIcon: const Icon(Icons.search, size: 20),
                suffixIcon: _searchController.text.isNotEmpty
                    ? IconButton(
                        icon: const Icon(Icons.clear, size: 20),
                        onPressed: _clearSearch,
                      )
                    : null,
                onChanged: _onSearchChanged,
              ),
            ),
            const SizedBox(height: AppSpacing.sm),

            // ── Content (Transaction List directly below Summary Cards, no divider) ──
            Expanded(
              child: _filteredTransactions.isEmpty
                  ? Center(
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(AppSpacing.lg),
                        child: _searchController.text.isNotEmpty
                            ? EmptyState(
                                icon: Icons.search_off,
                                title: 'Transaksi Tidak Ditemukan',
                                description:
                                    'Tidak ada transaksi yang sesuai dengan pencarian.',
                                actionText: 'Hapus Pencarian',
                                onAction: _clearSearch,
                              )
                            : const EmptyState(
                                icon: Icons.receipt_long_outlined,
                                title: 'Belum Ada Transaksi',
                                description:
                                    'Transaksi KAS akan muncul di sini.',
                              ),
                      ),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.fromLTRB(
                        AppSpacing.base,
                        AppSpacing.xs,
                        AppSpacing.base,
                        AppSpacing.xl,
                      ),
                      itemCount: _filteredTransactions.length,
                      separatorBuilder: (context, index) =>
                          const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) {
                        final tx = _filteredTransactions[index];
                        return KasTransactionCard(
                          transaction: tx,
                          isDark: isDark,
                          onTap: () => context.push('/home/kas/detail/${tx.id}'),
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

}

String cleanKasDescription(String desc) {
  if (desc.isEmpty) return '';
  return desc
      .replaceAll(RegExp(r'\s*\(Bill:\s*[a-f0-9-]+\)', caseSensitive: false), '')
      .replaceAll(RegExp(r'\s*\(Period:\s*[^)]+\)', caseSensitive: false), '')
      .replaceAll(RegExp(r'\s*Payment for bill\s*[a-f0-9-]+', caseSensitive: false), 'Pembayaran Iuran')
      .replaceAll(RegExp(r'\s*Voluntary contribution for due\s+', caseSensitive: false), 'Iuran Sukarela: ')
      .trim();
}

/// Compact summary chip for the top summary row.
class SummaryChip extends StatelessWidget {
  final String label;
  final String value;
  final Color color;
  final bool isEditable;
  final VoidCallback? onEdit;
  final IconData? icon;

  const SummaryChip({
    super.key,
    required this.label,
    required this.value,
    required this.color,
    this.isEditable = false,
    this.onEdit,
    this.icon,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SizedBox(
      height: 58,
      child: AppCard(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Row(
              children: [
                if (icon != null) ...[
                  Icon(icon, size: 10, color: color),
                  const SizedBox(width: 3),
                ],
                Expanded(
                  child: Text(
                    label,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                if (isEditable)
                  InkWell(
                    onTap: onEdit,
                    child: const Padding(
                      padding: EdgeInsets.only(left: 2),
                      child: Icon(Icons.edit, size: 12, color: AppColors.accent),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 2),
            FittedBox(
              fit: BoxFit.scaleDown,
              alignment: Alignment.centerLeft,
              child: Text(
                value,
                style: theme.textTheme.titleSmall?.copyWith(
                  color: color,
                  fontWeight: FontWeight.bold,
                  fontSize: 12,
                ),
                maxLines: 1,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Transaction card for the KAS list.
class KasTransactionCard extends StatelessWidget {
  final KasTransaction transaction;
  final bool isDark;
  final VoidCallback? onTap;

  const KasTransactionCard({
    super.key,
    required this.transaction,
    required this.isDark,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isIncoming = transaction.jenis == KasJenis.masuk;
    final accentColor = isIncoming ? AppColors.success : AppColors.danger;
    final icon = isIncoming ? Icons.arrow_downward : Icons.arrow_upward;
    final rawNominal = RupiahFormatter.formatMoneyString(transaction.nominal);
    final nominalDisplay = isIncoming ? '+$rawNominal' : '-$rawNominal';

    // Build subtitle / description text
    String descText;
    if (transaction.dueName != null && transaction.dueName!.isNotEmpty) {
      final citizen = transaction.citizenName ?? 'Warga';
      final house = (transaction.houseNumber != null && transaction.houseNumber!.isNotEmpty)
          ? ' · ${transaction.houseNumber}'
          : '';
      descText = '${transaction.dueName} · $citizen$house';
    } else {
      descText = cleanKasDescription(transaction.keterangan);
    }

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppRadius.sm),
      child: AppCard(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.base, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Row 1: Direction Icon + Category + Nominal (Right aligned) ──
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: accentColor.withValues(alpha: isDark ? 0.20 : 0.12),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Icon(icon, size: 14, color: accentColor),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    transaction.kategori.label,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                      fontSize: 13,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  nominalDisplay,
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: accentColor,
                    fontWeight: FontWeight.bold,
                    fontSize: 13,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),

            // ── Row 2: Type Badge [↑ Keluar] / [↓ Masuk] + Description + Date (Right aligned) ──
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                  decoration: BoxDecoration(
                    color: accentColor.withValues(alpha: isDark ? 0.20 : 0.12),
                    borderRadius: BorderRadius.circular(4),
                    border: Border.all(color: accentColor.withValues(alpha: 0.35)),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(icon, size: 9, color: accentColor),
                      const SizedBox(width: 2),
                      Text(
                        isIncoming ? 'Masuk' : 'Keluar',
                        style: TextStyle(
                          color: accentColor,
                          fontWeight: FontWeight.bold,
                          fontSize: 10,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    descText,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                      fontSize: 11,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  formatTanggal(transaction.tanggal),
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontSize: 11,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
