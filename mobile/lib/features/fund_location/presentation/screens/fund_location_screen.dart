import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import '../../data/fund_location_repository.dart';
import '../../data/models/fund_location_models.dart';

String formatRupiah(int amount) => RupiahFormatter.format(amount);

String _generateUuid() {
  final r = Random();
  String hex(int count) =>
      List.generate(count, (_) => '0123456789abcdef'[r.nextInt(16)]).join();
  return '${hex(8)}-${hex(4)}-4${hex(3)}-'
      '${'89ab'[r.nextInt(4)]}${hex(3)}-'
      '${hex(12)}';
}

class FundLocationScreen extends ConsumerStatefulWidget {
  const FundLocationScreen({super.key});

  @override
  ConsumerState<FundLocationScreen> createState() => _FundLocationScreenState();
}

class _FundLocationScreenState extends ConsumerState<FundLocationScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  final TextEditingController _searchCtrl = TextEditingController();
  String _selectedCategory = 'all';

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    _searchCtrl.dispose();
    super.dispose();
  }

  bool _isWarga(WidgetRef ref) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final role = user?['role'] as AppRole?;
    return role == AppRole.warga;
  }

  bool _canManage(WidgetRef ref) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final systemRole = user?['system_role'] as String?;
    if (systemRole == 'super_admin') return false; // Super admin is read-only
    final role = user?['role'] as AppRole?;
    final jabatan = (user?['jabatan'] ?? '').toString().toLowerCase();
    return role == AppRole.bendahara ||
        role == AppRole.pengurus ||
        jabatan == 'ketua' ||
        jabatan == 'bendahara';
  }

  bool _isBendahara(WidgetRef ref) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final role = user?['role'] as AppRole?;
    final jabatan = (user?['jabatan'] ?? '').toString().toLowerCase();
    return role == AppRole.bendahara || jabatan == 'bendahara';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final accentSoft = AppColors.accentSoftColor(context);

    if (_isWarga(ref)) {
      return Scaffold(
        appBar: AppBar(title: const Text('Lokasi Dana')),
        body: const Center(
          child: Text(
            'Akses Dibatasi. Fitur Lokasi Dana hanya dapat diakses oleh Pengurus dan Bendahara.',
            textAlign: TextAlign.center,
          ),
        ),
      );
    }

    final locationsAsync = ref.watch(fundLocationsProvider);
    final transfersAsync = ref.watch(fundTransfersProvider);
    final reconAsync = ref.watch(fundReconciliationProvider);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Lokasi Dana'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: () {
              ref.invalidate(fundLocationsProvider);
              ref.invalidate(fundTransfersProvider);
              ref.invalidate(fundReconciliationProvider);
            },
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Daftar Lokasi', icon: Icon(Icons.account_balance)),
            Tab(text: 'Riwayat Transfer', icon: Icon(Icons.swap_horiz)),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          // Tab 1: Locations
          _buildLocationsTab(context, locationsAsync, reconAsync, isDark),
          // Tab 2: Transfers
          _buildTransfersTab(context, transfersAsync, isDark),
        ],
      ),
      floatingActionButton: _canManage(ref)
          ? Tooltip(
              message: 'Tambah Lokasi',
              child: Container(
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
                    onTap: () => _showAddLocationDialog(context),
                    borderRadius: BorderRadius.circular(24),
                    child: const Padding(
                      padding: EdgeInsets.symmetric(
                        horizontal: 16,
                        vertical: 8,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.add,
                            size: 20,
                            color: AppColors.accent,
                          ),
                          SizedBox(width: 4),
                          Icon(
                            Icons.account_balance,
                            size: 18,
                            color: AppColors.accent,
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            )
          : null,
    );
  }

  Widget _buildLocationsTab(
    BuildContext context,
    AsyncValue<List<FundLocation>> locationsAsync,
    AsyncValue<FundLocationReconciliationReport> reconAsync,
    bool isDark,
  ) {
    return locationsAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, stack) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text('Gagal memuat data: $err'),
            const SizedBox(height: 12),
            ElevatedButton(
              onPressed: () => ref.invalidate(fundLocationsProvider),
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
      data: (locations) {
        // Compute summary amounts
        int rekeningTotal = 0;
        int brankasTotal = 0;
        int cohTotal = 0;
        for (final loc in locations) {
          if (!loc.isActive) continue;
          switch (loc.category) {
            case FundLocationCategory.rekening:
              rekeningTotal += loc.balanceRupiah;
              break;
            case FundLocationCategory.brankas:
              brankasTotal += loc.balanceRupiah;
              break;
            case FundLocationCategory.coh:
              cohTotal += loc.balanceRupiah;
              break;
          }
        }

        // Filter locations
        final query = _searchCtrl.text.toLowerCase().trim();
        final filtered = locations.where((loc) {
          if (_selectedCategory != 'all' &&
              loc.category.value != _selectedCategory) {
            return false;
          }
          if (query.isNotEmpty) {
            final matchesName = loc.name.toLowerCase().contains(query);
            final matchesBank = (loc.bankName ?? '').toLowerCase().contains(
              query,
            );
            final matchesAcc = (loc.accountNumber ?? '').toLowerCase().contains(
              query,
            );
            return matchesName || matchesBank || matchesAcc;
          }
          return true;
        }).toList();

        return RefreshIndicator(
          onRefresh: () async {
            ref.invalidate(fundLocationsProvider);
            ref.invalidate(fundReconciliationProvider);
          },
          child: ListView(
            padding: const EdgeInsets.all(AppSpacing.base),
            children: [
              // Summary Cards
              Row(
                children: [
                  Expanded(
                    child: _buildSummaryCard(
                      'Rekening Bank',
                      formatRupiah(rekeningTotal),
                      Icons.account_balance,
                      AppColors.accent,
                      isDark,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: _buildSummaryCard(
                      'Brankas Fisik',
                      formatRupiah(brankasTotal),
                      Icons.lock_outline,
                      AppColors.success,
                      isDark,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: _buildSummaryCard(
                      'Kas Di Tangan',
                      formatRupiah(cohTotal),
                      Icons.payments_outlined,
                      AppColors.warning,
                      isDark,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.md),

              // Reconciliation Card
              reconAsync.when(
                loading: () => const SizedBox.shrink(),
                error: (err, stack) => const SizedBox.shrink(),
                data: (report) => _buildReconciliationCard(
                  context,
                  report,
                  isDark,
                  locations,
                ),
              ),
              const SizedBox(height: AppSpacing.md),

              // Action Toolbar: Transfer button (subtle neon badge style)
              if (_canManage(ref)) ...[
                Row(
                  children: [
                    Expanded(
                      child: Container(
                        decoration: BoxDecoration(
                          color: AppColors.accent
                              .withValues(alpha: isDark ? 0.08 : 0.06),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                          border: Border.all(
                            color: AppColors.accent.withValues(alpha: 0.35),
                            width: 0.5,
                          ),
                          boxShadow: [
                            BoxShadow(
                              color: AppColors.accent
                                  .withValues(alpha: isDark ? 0.15 : 0.10),
                              blurRadius: 4,
                              spreadRadius: 0,
                              offset: const Offset(0, 0),
                            ),
                          ],
                        ),
                        child: OutlinedButton.icon(
                          style: OutlinedButton.styleFrom(
                            foregroundColor: isDark
                                ? Colors.white
                                : AppColors.accent,
                            side: BorderSide.none,
                            padding:
                                const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                            shape: RoundedRectangleBorder(
                              borderRadius:
                                  BorderRadius.circular(AppRadius.sm),
                            ),
                          ),
                          onPressed: () =>
                              _showTransferDialog(context, locations),
                          icon: Icon(Icons.swap_horiz,
                              color: isDark ? Colors.white : AppColors.accent,
                              size: 16),
                          label: Text(
                            'Transfer Antar Lokasi',
                            style: TextStyle(
                              color: isDark ? Colors.white : AppColors.accent,
                              fontWeight: FontWeight.w600,
                              fontSize: 12,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.md),
              ],

              // Filter Chips
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    _buildCategoryChip('Semua', 'all'),
                    const SizedBox(width: 8),
                    _buildCategoryChip('Rekening Bank', 'rekening'),
                    const SizedBox(width: 8),
                    _buildCategoryChip('Brankas Fisik', 'brankas'),
                    const SizedBox(width: 8),
                    _buildCategoryChip('Kas Di Tangan', 'coh'),
                  ],
                ),
              ),
              const SizedBox(height: AppSpacing.sm),

              // Search field
              TextField(
                controller: _searchCtrl,
                decoration: InputDecoration(
                  hintText: 'Cari nama lokasi, bank, no. rekening...',
                  prefixIcon: const Icon(Icons.search),
                  suffixIcon: _searchCtrl.text.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear),
                          onPressed: () {
                            setState(() => _searchCtrl.clear());
                          },
                        )
                      : null,
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: AppSpacing.md,
                    vertical: 10,
                  ),
                ),
                onChanged: (_) => setState(() {}),
              ),
              const SizedBox(height: AppSpacing.md),

              // Locations List
              if (filtered.isEmpty)
                const EmptyState(
                  icon: Icons.account_balance_wallet_outlined,
                  title: 'Tidak ada lokasi dana',
                  description:
                      'Belum ada lokasi dana yang sesuai dengan filter.',
                )
              else
                ...filtered.map(
                  (loc) => _buildLocationItem(context, loc, isDark),
                ),
              const SizedBox(height: 80),
            ],
          ),
        );
      },
    );
  }

  Widget _buildSummaryCard(
    String title,
    String amount,
    IconData icon,
    Color color,
    bool isDark,
  ) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.sm),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
        borderRadius: BorderRadius.circular(AppRadius.base),
        border: Border.all(
          color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 16, color: color),
              const SizedBox(width: 4),
              Expanded(
                child: Text(
                  title,
                  style: const TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w500,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            amount,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.bold,
              color: color,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }

  Widget _buildReconciliationCard(
    BuildContext context,
    FundLocationReconciliationReport report,
    bool isDark,
    List<FundLocation> locations,
  ) {
    return _ReconciliationCard(
      report: report,
      isDark: isDark,
      onAllocate: () => _showOpeningBalanceDialog(
        context,
        locations,
        report.unallocatedBalanceRupiah,
      ),
    );
  }

  Widget _buildCategoryChip(String label, String value) {
    final isSelected = _selectedCategory == value;
    return ChoiceChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (selected) {
        if (selected) {
          setState(() => _selectedCategory = value);
        }
      },
    );
  }

  Widget _buildLocationItem(
    BuildContext context,
    FundLocation loc,
    bool isDark,
  ) {
    final icon = switch (loc.category) {
      FundLocationCategory.rekening => Icons.account_balance,
      FundLocationCategory.brankas => Icons.lock_outline,
      FundLocationCategory.coh => Icons.payments_outlined,
    };
    final iconColor = switch (loc.category) {
      FundLocationCategory.rekening => AppColors.accent,
      FundLocationCategory.brankas => AppColors.success,
      FundLocationCategory.coh => AppColors.warning,
    };

    return Card(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.base),
        side: BorderSide(
          color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
        ),
      ),
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: iconColor.withValues(alpha: 0.15),
          child: Icon(icon, color: iconColor),
        ),
        title: Row(
          children: [
            Expanded(
              child: Text(
                loc.name,
                style: TextStyle(
                  fontWeight: FontWeight.bold,
                  decoration: loc.isActive ? null : TextDecoration.lineThrough,
                ),
              ),
            ),
            if (!loc.isActive)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: Colors.grey.withValues(alpha: 0.2),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: const Text(
                  'Nonaktif',
                  style: TextStyle(fontSize: 10, color: Colors.grey),
                ),
              ),
          ],
        ),
        subtitle: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (loc.category == FundLocationCategory.rekening &&
                loc.bankName != null) ...[
              Text(
                '${loc.bankName} • ${loc.accountNumber ?? "-"} (${loc.accountHolder ?? "-"})',
                style: const TextStyle(fontSize: 12),
              ),
            ] else ...[
              Text(loc.category.label, style: const TextStyle(fontSize: 12)),
            ],
            const SizedBox(height: 4),
            Text(
              'Saldo: ${formatRupiah(loc.balanceRupiah)}',
              style: TextStyle(
                fontWeight: FontWeight.w600,
                color: loc.balanceRupiah > 0 ? AppColors.success : null,
              ),
            ),
          ],
        ),
        trailing: PopupMenuButton<String>(
          icon: const Icon(Icons.more_vert),
          onSelected: (val) {
            switch (val) {
              case 'history':
                _showLocationHistory(context, loc);
                break;
              case 'edit':
                _showEditLocationDialog(context, loc);
                break;
              case 'deactivate':
                _confirmDeactivate(context, loc);
                break;
            }
          },
          itemBuilder: (context) => [
            const PopupMenuItem(
              value: 'history',
              child: Row(
                children: [
                  Icon(Icons.history, size: 18),
                  SizedBox(width: 8),
                  Text('Riwayat Mutasi'),
                ],
              ),
            ),
            if (_canManage(ref)) ...[
              const PopupMenuItem(
                value: 'edit',
                child: Row(
                  children: [
                    Icon(Icons.edit_outlined, size: 18),
                    SizedBox(width: 8),
                    Text('Edit Lokasi'),
                  ],
                ),
              ),
              if (loc.isActive && loc.balanceRupiah == 0)
                const PopupMenuItem(
                  value: 'deactivate',
                  child: Row(
                    children: [
                      Icon(Icons.block, size: 18, color: AppColors.danger),
                      SizedBox(width: 8),
                      Text(
                        'Nonaktifkan',
                        style: TextStyle(color: AppColors.danger),
                      ),
                    ],
                  ),
                ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTransfersTab(
    BuildContext context,
    AsyncValue<PaginatedResponse<FundTransfer>> transfersAsync,
    bool isDark,
  ) {
    return transfersAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, stack) => Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text('Gagal memuat transfer: $err'),
            const SizedBox(height: 12),
            ElevatedButton(
              onPressed: () => ref.invalidate(fundTransfersProvider),
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
      data: (response) {
        final transfers = response.data;
        if (transfers.isEmpty) {
          return const EmptyState(
            icon: Icons.swap_horiz,
            title: 'Belum ada riwayat transfer',
            description:
                'Transfer dana antar rekening/brankas/COH akan muncul di sini.',
          );
        }

        return RefreshIndicator(
          onRefresh: () async {
            ref.invalidate(fundTransfersProvider);
          },
          child: ListView.builder(
            padding: const EdgeInsets.all(AppSpacing.base),
            itemCount: transfers.length,
            itemBuilder: (context, index) {
              final item = transfers[index];
              return Card(
                margin: const EdgeInsets.only(bottom: AppSpacing.sm),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppRadius.base),
                  side: BorderSide(
                    color: isDark
                        ? AppColors.darkBorder
                        : AppColors.lightBorder,
                  ),
                ),
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Row(
                            children: [
                              Text(
                                item.fromLocationName ?? 'Asal',
                                style: const TextStyle(
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                              const Padding(
                                padding: EdgeInsets.symmetric(horizontal: 6),
                                child: Icon(Icons.arrow_forward, size: 14),
                              ),
                              Text(
                                item.toLocationName ?? 'Tujuan',
                                style: const TextStyle(
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                            ],
                          ),
                          Text(
                            formatRupiah(item.amountRupiah),
                            style: const TextStyle(
                              fontWeight: FontWeight.bold,
                              fontSize: 14,
                              color: AppColors.accent,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            '${item.transferDate.year}-${item.transferDate.month.toString().padLeft(2, '0')}-${item.transferDate.day.toString().padLeft(2, '0')}',
                            style: const TextStyle(
                              fontSize: 12,
                              color: Colors.grey,
                            ),
                          ),
                          if (item.isReversal)
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 6,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: AppColors.danger.withValues(alpha: 0.15),
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: const Text(
                                'Reversal',
                                style: TextStyle(
                                  fontSize: 10,
                                  fontWeight: FontWeight.bold,
                                  color: AppColors.danger,
                                ),
                              ),
                            ),
                        ],
                      ),
                      if (item.notes != null && item.notes!.isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(
                          'Catatan: ${item.notes}',
                          style: const TextStyle(fontSize: 12),
                        ),
                      ],
                      if (!item.isReversal && _isBendahara(ref)) ...[
                        const Divider(height: 16),
                        Align(
                          alignment: Alignment.centerRight,
                          child: TextButton.icon(
                            style: TextButton.styleFrom(
                              foregroundColor: AppColors.danger,
                              visualDensity: VisualDensity.compact,
                            ),
                            onPressed: () =>
                                _confirmReverseTransfer(context, item),
                            icon: const Icon(Icons.undo, size: 14),
                            label: const Text('Batalkan Transfer (Reversal)'),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
              );
            },
          ),
        );
      },
    );
  }

  // --- Dialogs & Operations ---

  void _showAddLocationDialog(BuildContext context) {
    final nameCtrl = TextEditingController();
    final bankNameCtrl = TextEditingController();
    final accNoCtrl = TextEditingController();
    final accHolderCtrl = TextEditingController();
    String category = 'rekening';
    bool saving = false;
    String? error;

    showDialog(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text('Tambah Lokasi Dana'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (error != null) ...[
                      Text(
                        error!,
                        style: const TextStyle(color: AppColors.danger),
                      ),
                      const SizedBox(height: 8),
                    ],
                    DropdownButtonFormField<String>(
                      value: category,
                      decoration: const InputDecoration(
                        labelText: 'Kategori Lokasi',
                      ),
                      items: const [
                        DropdownMenuItem(
                          value: 'rekening',
                          child: Text('Rekening Bank'),
                        ),
                        DropdownMenuItem(
                          value: 'brankas',
                          child: Text('Brankas Fisik'),
                        ),
                        DropdownMenuItem(
                          value: 'coh',
                          child: Text('Kas Di Tangan'),
                        ),
                      ],
                      onChanged: (val) {
                        if (val != null) setDialogState(() => category = val);
                      },
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(
                        labelText: 'Nama Lokasi (Wajib)',
                      ),
                    ),
                    if (category == 'rekening') ...[
                      const SizedBox(height: 12),
                      TextField(
                        controller: bankNameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Nama Bank (misal BCA, BRI)',
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: accNoCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Nomor Rekening',
                        ),
                        keyboardType: TextInputType.number,
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: accHolderCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Atas Nama Rekening',
                        ),
                      ),
                    ],
                    const SizedBox(height: AppSpacing.lg),

                    // Horizontal buttons: Simpan + Batal
                    Row(
                      children: [
                        Expanded(
                          child: SizedBox(
                            height: 48,
                            child: OutlinedButton(
                              onPressed: saving ? null : () async {
                                final name = nameCtrl.text.trim();
                                if (name.isEmpty) {
                                  setDialogState(
                                    () => error = 'Nama lokasi wajib diisi.',
                                  );
                                  return;
                                }
                                setDialogState(() {
                                  saving = true;
                                  error = null;
                                });

                                try {
                                  final repo = ref.read(
                                    fundLocationRepositoryProvider,
                                  );
                                  await repo.createLocation(
                                    name: name,
                                    category: category,
                                    bankName: category == 'rekening'
                                        ? bankNameCtrl.text.trim()
                                        : null,
                                    accountNumber: category == 'rekening'
                                        ? accNoCtrl.text.trim()
                                        : null,
                                    accountHolder: category == 'rekening'
                                        ? accHolderCtrl.text.trim()
                                        : null,
                                  );

                                  if (context.mounted) {
                                    Navigator.pop(dialogCtx);
                                    ref.invalidate(fundLocationsProvider);
                                    ref.invalidate(fundReconciliationProvider);
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      const SnackBar(
                                        content: Text(
                                          'Lokasi dana berhasil ditambahkan',
                                        ),
                                        backgroundColor: AppColors.success,
                                      ),
                                    );
                                  }
                                } catch (e) {
                                  final repo = ref.read(
                                    fundLocationRepositoryProvider,
                                  );
                                  setDialogState(() {
                                    saving = false;
                                    error = repo.mapDioError(e).message;
                                  });
                                }
                              },
                              style: OutlinedButton.styleFrom(
                                backgroundColor: AppColors.accentSoftColor(context),
                                side: const BorderSide(color: AppColors.accent),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(AppRadius.base),
                                ),
                              ),
                              child: saving
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
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: SizedBox(
                            height: 48,
                            child: OutlinedButton(
                              onPressed: saving ? null : () => Navigator.pop(dialogCtx),
                              style: OutlinedButton.styleFrom(
                                backgroundColor: Theme.of(context).brightness == Brightness.dark
                                    ? AppColors.darkDangerSoft
                                    : AppColors.danger.withValues(alpha: 0.20),
                                side: const BorderSide(color: AppColors.danger),
                                foregroundColor: AppColors.danger,
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(AppRadius.base),
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
            );
          },
        );
      },
    );
  }

  void _showEditLocationDialog(BuildContext context, FundLocation loc) {
    final nameCtrl = TextEditingController(text: loc.name);
    final bankNameCtrl = TextEditingController(text: loc.bankName ?? '');
    final accNoCtrl = TextEditingController(text: loc.accountNumber ?? '');
    final accHolderCtrl = TextEditingController(text: loc.accountHolder ?? '');
    bool saving = false;
    String? error;

    showDialog(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text('Edit Lokasi Dana'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (error != null) ...[
                      Text(
                        error!,
                        style: const TextStyle(color: AppColors.danger),
                      ),
                      const SizedBox(height: 8),
                    ],
                    TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(
                        labelText: 'Nama Lokasi',
                      ),
                    ),
                    if (loc.category == FundLocationCategory.rekening) ...[
                      const SizedBox(height: 12),
                      TextField(
                        controller: bankNameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Nama Bank',
                        ),
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: accNoCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Nomor Rekening',
                        ),
                        keyboardType: TextInputType.number,
                      ),
                      const SizedBox(height: 12),
                      TextField(
                        controller: accHolderCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Atas Nama',
                        ),
                      ),
                    ],
                    const SizedBox(height: AppSpacing.lg),

                    // Horizontal buttons: Simpan + Batal
                    Row(
                      children: [
                        Expanded(
                          child: SizedBox(
                            height: 48,
                            child: OutlinedButton(
                              onPressed: saving ? null : () async {
                                final name = nameCtrl.text.trim();
                                if (name.isEmpty) {
                                  setDialogState(
                                    () => error = 'Nama lokasi tidak boleh kosong.',
                                  );
                                  return;
                                }
                                setDialogState(() {
                                  saving = true;
                                  error = null;
                                });

                                try {
                                  final repo = ref.read(
                                    fundLocationRepositoryProvider,
                                  );
                                  await repo.updateLocation(
                                    id: loc.id,
                                    name: name,
                                    bankName:
                                        loc.category == FundLocationCategory.rekening
                                        ? bankNameCtrl.text.trim()
                                        : null,
                                    accountNumber:
                                        loc.category == FundLocationCategory.rekening
                                        ? accNoCtrl.text.trim()
                                        : null,
                                    accountHolder:
                                        loc.category == FundLocationCategory.rekening
                                        ? accHolderCtrl.text.trim()
                                        : null,
                                  );

                                  if (context.mounted) {
                                    Navigator.pop(dialogCtx);
                                    ref.invalidate(fundLocationsProvider);
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      const SnackBar(
                                        content: Text(
                                          'Lokasi dana berhasil diperbarui',
                                        ),
                                        backgroundColor: AppColors.success,
                                      ),
                                    );
                                  }
                                } catch (e) {
                                  final repo = ref.read(
                                    fundLocationRepositoryProvider,
                                  );
                                  setDialogState(() {
                                    saving = false;
                                    error = repo.mapDioError(e).message;
                                  });
                                }
                              },
                              style: OutlinedButton.styleFrom(
                                backgroundColor: AppColors.accentSoftColor(context),
                                side: const BorderSide(color: AppColors.accent),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(AppRadius.base),
                                ),
                              ),
                              child: saving
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
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: SizedBox(
                            height: 48,
                            child: OutlinedButton(
                              onPressed: saving ? null : () => Navigator.pop(dialogCtx),
                              style: OutlinedButton.styleFrom(
                                backgroundColor: Theme.of(context).brightness == Brightness.dark
                                    ? AppColors.darkDangerSoft
                                    : AppColors.danger.withValues(alpha: 0.20),
                                side: const BorderSide(color: AppColors.danger),
                                foregroundColor: AppColors.danger,
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(AppRadius.base),
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
            );
          },
        );
      },
    );
  }

  void _confirmDeactivate(BuildContext context, FundLocation loc) {
    showDialog(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: const Text('Nonaktifkan Lokasi'),
        content: Text(
          'Apakah Anda yakin ingin menonaktifkan lokasi "${loc.name}"?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogCtx),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.danger),
            onPressed: () async {
              Navigator.pop(dialogCtx);
              try {
                final repo = ref.read(fundLocationRepositoryProvider);
                await repo.deactivateLocation(loc.id);
                if (context.mounted) {
                  ref.invalidate(fundLocationsProvider);
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(
                      content: Text('Lokasi berhasil dinonaktifkan'),
                    ),
                  );
                }
              } catch (e) {
                if (context.mounted) {
                  final repo = ref.read(fundLocationRepositoryProvider);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(repo.mapDioError(e).message),
                      backgroundColor: AppColors.danger,
                    ),
                  );
                }
              }
            },
            child: const Text(
              'Nonaktifkan',
              style: TextStyle(color: Colors.white),
            ),
          ),
        ],
      ),
    );
  }

  void _showTransferDialog(BuildContext context, List<FundLocation> locations) {
    final activeLocs = locations.where((l) => l.isActive).toList();
    if (activeLocs.length < 2) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Dibutuhkan minimal 2 lokasi dana aktif untuk melakukan transfer.',
          ),
          backgroundColor: AppColors.warning,
        ),
      );
      return;
    }

    String? fromId = activeLocs.first.id;
    String? toId = activeLocs.length > 1 ? activeLocs[1].id : null;
    final amountCtrl = TextEditingController();
    final notesCtrl = TextEditingController();
    bool saving = false;
    String? error;

    showDialog(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            final fromLoc = activeLocs.firstWhere(
              (l) => l.id == fromId,
              orElse: () => activeLocs.first,
            );

            return AlertDialog(
              title: const Text('Transfer Antar Lokasi'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (error != null) ...[
                      Text(
                        error!,
                        style: const TextStyle(color: AppColors.danger),
                      ),
                      const SizedBox(height: 8),
                    ],
                    DropdownButtonFormField<String>(
                      value: fromId,
                      decoration: const InputDecoration(
                        labelText: 'Dari Lokasi',
                      ),
                      items: activeLocs.map((l) {
                        return DropdownMenuItem(
                          value: l.id,
                          child: Text(
                            '${l.name} (${formatRupiah(l.balanceRupiah)})',
                          ),
                        );
                      }).toList(),
                      onChanged: (val) {
                        if (val != null) {
                          setDialogState(() {
                            fromId = val;
                            if (toId == val) {
                              toId = activeLocs
                                  .firstWhere((l) => l.id != val)
                                  .id;
                            }
                          });
                        }
                      },
                    ),
                    const SizedBox(height: 12),
                    DropdownButtonFormField<String>(
                      value: toId,
                      decoration: const InputDecoration(
                        labelText: 'Ke Lokasi Tujuan',
                      ),
                      items: activeLocs.where((l) => l.id != fromId).map((l) {
                        return DropdownMenuItem(
                          value: l.id,
                          child: Text(l.name),
                        );
                      }).toList(),
                      onChanged: (val) {
                        if (val != null) setDialogState(() => toId = val);
                      },
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: amountCtrl,
                      decoration: InputDecoration(
                        labelText: 'Nominal Transfer (Rp)',
                        helperText:
                            'Maksimal: ${formatRupiah(fromLoc.balanceRupiah)}',
                      ),
                      keyboardType: TextInputType.number,
                      onChanged: (v) {
                        final parsed = parseRupiahInput(v);
                        if (parsed != null && parsed > fromLoc.balanceRupiah) {
                          setDialogState(
                            () => error = 'Nominal melebihi saldo lokasi asal.',
                          );
                        } else {
                          setDialogState(() => error = null);
                        }
                      },
                    ),
                    const SizedBox(height: AppSpacing.lg),

                    // Horizontal buttons: Transfer + Batal
                    Row(
                      children: [
                        Expanded(
                          child: SizedBox(
                            height: 48,
                            child: OutlinedButton(
                              onPressed: saving
                                  ? null
                                  : () async {
                                      final amount =
                                          parseRupiahInput(amountCtrl.text.trim()) ?? 0;
                                      if (amount <= 0) {
                                        setDialogState(
                                          () => error =
                                              'Nominal transfer harus lebih dari 0.',
                                        );
                                        return;
                                      }
                                      if (amount > fromLoc.balanceRupiah) {
                                        setDialogState(
                                          () => error = 'Saldo tidak mencukupi.',
                                        );
                                        return;
                                      }
                                      if (fromId == toId) {
                                        setDialogState(
                                          () => error =
                                              'Lokasi asal dan tujuan tidak boleh sama.',
                                        );
                                        return;
                                      }

                                      setDialogState(() {
                                        saving = true;
                                        error = null;
                                      });

                                      try {
                                        final repo = ref.read(
                                          fundLocationRepositoryProvider,
                                        );
                                        await repo.createTransfer(
                                          fromLocationId: fromId!,
                                          toLocationId: toId!,
                                          amountRupiah: amount,
                                          notes: notesCtrl.text.trim().isNotEmpty
                                              ? notesCtrl.text.trim()
                                              : null,
                                          idempotencyKey: _generateUuid(),
                                        );

                                        if (context.mounted) {
                                          Navigator.pop(dialogCtx);
                                          ref.invalidate(fundLocationsProvider);
                                          ref.invalidate(fundTransfersProvider);
                                          ref.invalidate(fundReconciliationProvider);
                                          ScaffoldMessenger.of(context).showSnackBar(
                                            const SnackBar(
                                              content: Text(
                                                'Transfer dana berhasil dilakukan.',
                                              ),
                                              backgroundColor: AppColors.success,
                                            ),
                                          );
                                        }
                                      } catch (e) {
                                        final repo = ref.read(
                                          fundLocationRepositoryProvider,
                                        );
                                        setDialogState(() {
                                          saving = false;
                                          error = repo.mapDioError(e).message;
                                        });
                                      }
                                    },
                              style: OutlinedButton.styleFrom(
                                backgroundColor: AppColors.accentSoftColor(context),
                                side: const BorderSide(color: AppColors.accent),
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(AppRadius.base),
                                ),
                              ),
                              child: saving
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
                                        Icon(Icons.swap_horiz, size: 18),
                                        SizedBox(width: 6),
                                        Text('Transfer'),
                                      ],
                                    ),
                            ),
                          ),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: SizedBox(
                            height: 48,
                            child: OutlinedButton(
                              onPressed: saving ? null : () => Navigator.pop(dialogCtx),
                              style: OutlinedButton.styleFrom(
                                backgroundColor: Theme.of(context).brightness == Brightness.dark
                                    ? AppColors.darkDangerSoft
                                    : AppColors.danger.withValues(alpha: 0.20),
                                side: const BorderSide(color: AppColors.danger),
                                foregroundColor: AppColors.danger,
                                shape: RoundedRectangleBorder(
                                  borderRadius: BorderRadius.circular(AppRadius.base),
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
            );
          },
        );
      },
    );
  }

  void _showOpeningBalanceDialog(
    BuildContext context,
    List<FundLocation> locations,
    int maxAmount,
  ) {
    final activeLocs = locations.where((l) => l.isActive).toList();
    if (activeLocs.isEmpty) return;

    String? selectedId = activeLocs.first.id;
    final amountCtrl = TextEditingController(text: maxAmount.toString());
    final notesCtrl = TextEditingController(text: 'Alokasi Saldo Awal');
    bool saving = false;
    String? error;

    showDialog(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text('Alokasi Saldo Awal'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (error != null) ...[
                      Text(
                        error!,
                        style: const TextStyle(color: AppColors.danger),
                      ),
                      const SizedBox(height: 8),
                    ],
                    Text(
                      'Pindahkan saldo kas yang belum dialokasikan (maks. ${formatRupiah(maxAmount)}) ke lokasi dana tujuan.',
                      style: const TextStyle(fontSize: 12),
                    ),
                    const SizedBox(height: 12),
                    DropdownButtonFormField<String>(
                      value: selectedId,
                      decoration: const InputDecoration(
                        labelText: 'Lokasi Tujuan',
                      ),
                      items: activeLocs.map((l) {
                        return DropdownMenuItem(
                          value: l.id,
                          child: Text(l.name),
                        );
                      }).toList(),
                      onChanged: (val) {
                        if (val != null) setDialogState(() => selectedId = val);
                      },
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: amountCtrl,
                      decoration: InputDecoration(
                        labelText: 'Nominal Alokasi (Rp)',
                        helperText: 'Maksimal: ${formatRupiah(maxAmount)}',
                      ),
                      keyboardType: TextInputType.number,
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: notesCtrl,
                      decoration: const InputDecoration(labelText: 'Catatan'),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogCtx),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  onPressed: saving
                      ? null
                      : () async {
                          final amount =
                              parseRupiahInput(amountCtrl.text.trim()) ?? 0;
                          if (amount <= 0 || amount > maxAmount) {
                            setDialogState(
                              () => error =
                                  'Nominal harus antara 1 dan ${formatRupiah(maxAmount)}.',
                            );
                            return;
                          }

                          setDialogState(() {
                            saving = true;
                            error = null;
                          });

                          try {
                            final repo = ref.read(
                              fundLocationRepositoryProvider,
                            );
                            await repo.setOpeningBalance(
                              id: selectedId!,
                              amountRupiah: amount,
                              notes: notesCtrl.text.trim(),
                            );

                            if (context.mounted) {
                              Navigator.pop(dialogCtx);
                              ref.invalidate(fundLocationsProvider);
                              ref.invalidate(fundReconciliationProvider);
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text(
                                    'Saldo awal berhasil dialokasikan',
                                  ),
                                  backgroundColor: AppColors.success,
                                ),
                              );
                            }
                          } catch (e) {
                            final repo = ref.read(
                              fundLocationRepositoryProvider,
                            );
                            setDialogState(() {
                              saving = false;
                              error = repo.mapDioError(e).message;
                            });
                          }
                        },
                  child: saving
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('Alokasikan'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  void _confirmReverseTransfer(BuildContext context, FundTransfer transfer) {
    final reasonCtrl = TextEditingController();
    bool submitting = false;
    String? error;

    showDialog(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              title: const Text('Batalkan Transfer (Reversal)'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (error != null) ...[
                      Text(
                        error!,
                        style: const TextStyle(color: AppColors.danger),
                      ),
                      const SizedBox(height: 8),
                    ],
                    Text(
                      'Transfer sebesar ${formatRupiah(transfer.amountRupiah)} dari "${transfer.fromLocationName}" ke "${transfer.toLocationName}" akan dibalikkan.',
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      controller: reasonCtrl,
                      decoration: const InputDecoration(
                        labelText: 'Alasan Pembatalan (Wajib, min. 5 karakter)',
                        border: OutlineInputBorder(),
                      ),
                      maxLines: 2,
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogCtx),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.danger,
                  ),
                  onPressed: submitting
                      ? null
                      : () async {
                          final reason = reasonCtrl.text.trim();
                          if (reason.length < 5) {
                            setDialogState(
                              () => error =
                                  'Alasan pembatalan minimal 5 karakter.',
                            );
                            return;
                          }

                          setDialogState(() {
                            submitting = true;
                            error = null;
                          });

                          try {
                            final repo = ref.read(
                              fundLocationRepositoryProvider,
                            );
                            await repo.reverseTransfer(
                              id: transfer.id,
                              reason: reason,
                            );

                            if (context.mounted) {
                              Navigator.pop(dialogCtx);
                              ref.invalidate(fundLocationsProvider);
                              ref.invalidate(fundTransfersProvider);
                              ref.invalidate(fundReconciliationProvider);
                              ScaffoldMessenger.of(context).showSnackBar(
                                const SnackBar(
                                  content: Text(
                                    'Transfer berhasil dibatalkan (reversal).',
                                  ),
                                  backgroundColor: AppColors.success,
                                ),
                              );
                            }
                          } catch (e) {
                            final repo = ref.read(
                              fundLocationRepositoryProvider,
                            );
                            setDialogState(() {
                              submitting = false;
                              error = repo.mapDioError(e).message;
                            });
                          }
                        },
                  child: submitting
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text(
                          'Batalkan Transfer',
                          style: TextStyle(color: Colors.white),
                        ),
                ),
              ],
            );
          },
        );
      },
    );
  }

  void _showLocationHistory(BuildContext context, FundLocation loc) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.lg)),
      ),
      builder: (sheetCtx) {
        return DraggableScrollableSheet(
          initialChildSize: 0.75,
          minChildSize: 0.4,
          maxChildSize: 0.95,
          expand: false,
          builder: (context, scrollCtrl) {
            return FutureBuilder<PaginatedResponse<LocationBalanceChange>>(
              future: ref
                  .read(fundLocationRepositoryProvider)
                  .fetchLocationHistory(id: loc.id, page: 1, pageSize: 50),
              builder: (context, snapshot) {
                return Column(
                  children: [
                    Padding(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                'Mutasi: ${loc.name}',
                                style: const TextStyle(
                                  fontWeight: FontWeight.bold,
                                  fontSize: 16,
                                ),
                              ),
                              Text(
                                'Saldo: ${formatRupiah(loc.balanceRupiah)}',
                                style: const TextStyle(
                                  fontSize: 12,
                                  color: Colors.grey,
                                ),
                              ),
                            ],
                          ),
                          IconButton(
                            icon: const Icon(Icons.close),
                            onPressed: () => Navigator.pop(sheetCtx),
                          ),
                        ],
                      ),
                    ),
                    const Divider(height: 1),
                    Expanded(
                      child: () {
                        if (snapshot.connectionState ==
                            ConnectionState.waiting) {
                          return const Center(
                            child: CircularProgressIndicator(),
                          );
                        }
                        if (snapshot.hasError) {
                          return Center(
                            child: Text(
                              'Gagal memuat mutasi: ${snapshot.error}',
                            ),
                          );
                        }
                        final changes = snapshot.data?.data ?? [];
                        if (changes.isEmpty) {
                          return const Center(
                            child: Text(
                              'Belum ada riwayat mutasi untuk lokasi ini.',
                            ),
                          );
                        }

                        return ListView.separated(
                          controller: scrollCtrl,
                          padding: const EdgeInsets.all(AppSpacing.md),
                          itemCount: changes.length,
                          separatorBuilder: (_, index) =>
                              const Divider(height: 16),
                          itemBuilder: (context, index) {
                            final c = changes[index];
                            final isPositive = c.deltaRupiah >= 0;
                            return Row(
                              children: [
                                CircleAvatar(
                                  radius: 16,
                                  backgroundColor: isPositive
                                      ? AppColors.success.withValues(
                                          alpha: 0.15,
                                        )
                                      : AppColors.danger.withValues(
                                          alpha: 0.15,
                                        ),
                                  child: Icon(
                                    isPositive
                                        ? Icons.arrow_downward
                                        : Icons.arrow_upward,
                                    size: 16,
                                    color: isPositive
                                        ? AppColors.success
                                        : AppColors.danger,
                                  ),
                                ),
                                const SizedBox(width: AppSpacing.md),
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        c.changeTypeLabel,
                                        style: const TextStyle(
                                          fontWeight: FontWeight.bold,
                                          fontSize: 13,
                                        ),
                                      ),
                                      if (c.notes != null &&
                                          c.notes!.isNotEmpty)
                                        Text(
                                          c.notes!,
                                          style: const TextStyle(
                                            fontSize: 11,
                                            color: Colors.grey,
                                          ),
                                        ),
                                      Text(
                                        '${c.createdAt.year}-${c.createdAt.month.toString().padLeft(2, '0')}-${c.createdAt.day.toString().padLeft(2, '0')}',
                                        style: const TextStyle(
                                          fontSize: 10,
                                          color: Colors.grey,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                                Column(
                                  crossAxisAlignment: CrossAxisAlignment.end,
                                  children: [
                                    Text(
                                      '${isPositive ? '+' : ''}${formatRupiah(c.deltaRupiah)}',
                                      style: TextStyle(
                                        fontWeight: FontWeight.bold,
                                        color: isPositive
                                            ? AppColors.success
                                            : AppColors.danger,
                                      ),
                                    ),
                                    Text(
                                      'Saldo: ${formatRupiah(c.balanceAfterRupiah)}',
                                      style: const TextStyle(
                                        fontSize: 10,
                                        color: Colors.grey,
                                      ),
                                    ),
                                  ],
                                ),
                              ],
                            );
                          },
                        );
                      }(),
                    ),
                  ],
                );
              },
            );
          },
        );
      },
    );
  }
}

class _ReconciliationCard extends StatefulWidget {
  const _ReconciliationCard({
    required this.report,
    required this.isDark,
    required this.onAllocate,
  });

  final FundLocationReconciliationReport report;
  final bool isDark;
  final VoidCallback onAllocate;

  @override
  State<_ReconciliationCard> createState() => _ReconciliationCardState();
}

class _ReconciliationCardState extends State<_ReconciliationCard>
    with SingleTickerProviderStateMixin {
  bool _expanded = false;

  void _toggle() {
    setState(() {
      _expanded = !_expanded;
    });
  }

  Color get _statusColor {
    if (widget.report.status == 'MATCH' || widget.report.status == 'Cocok') {
      return AppColors.success;
    }
    if (widget.report.status == 'MATCH_WITH_NOTES' ||
        widget.report.status == 'Cocok dengan catatan') {
      return AppColors.info;
    }
    return AppColors.danger;
  }

  bool get _isMatched => widget.report.isBalanced;

  Widget _buildHeader() {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: _toggle,
        borderRadius: BorderRadius.circular(AppRadius.base),
        child: Container(
          padding: const EdgeInsets.all(AppSpacing.md),
          decoration: BoxDecoration(
            color: widget.isDark
                ? AppColors.darkSurface
                : AppColors.lightSurface,
            borderRadius: BorderRadius.circular(AppRadius.base),
            border: Border.all(
              color: _statusColor.withValues(alpha: 0.5),
              width: 1.5,
            ),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Row(
                children: [
                  Icon(
                    _isMatched
                        ? Icons.check_circle_outline
                        : Icons.error_outline,
                    color: _statusColor,
                    size: 20,
                  ),
                  const SizedBox(width: 6),
                  const Text(
                    'Status Rekonsiliasi Kas',
                    style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                  ),
                ],
              ),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    decoration: BoxDecoration(
                      color: _statusColor.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(12),
                    ),
                    child: Text(
                      widget.report.statusLabel,
                      style: TextStyle(
                        color: _statusColor,
                        fontWeight: FontWeight.bold,
                        fontSize: 12,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Icon(
                    _expanded ? Icons.expand_less : Icons.expand_more,
                    color: _statusColor,
                    size: 20,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildExpandedContent() {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: widget.isDark ? AppColors.darkSurface : AppColors.lightSurface,
        borderRadius: const BorderRadius.only(
          bottomLeft: Radius.circular(AppRadius.base),
          bottomRight: Radius.circular(AppRadius.base),
        ),
        border: Border(
          left: BorderSide(
            color: _statusColor.withValues(alpha: 0.5),
            width: 1.5,
          ),
          right: BorderSide(
            color: _statusColor.withValues(alpha: 0.5),
            width: 1.5,
          ),
          bottom: BorderSide(
            color: _statusColor.withValues(alpha: 0.5),
            width: 1.5,
          ),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (widget.report.statusDescription != null) ...[
            Text(
              widget.report.statusDescription!,
              style: TextStyle(
                fontSize: 11,
                color: _statusColor.withValues(alpha: 0.9),
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
          ],
          const Text(
            'Formula: K (Kas) = L (Terlokasi) + U (Belum Dialokasikan)',
            style: TextStyle(fontSize: 11, color: Colors.grey),
          ),
          const Divider(height: 16),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              _buildMetric(
                'Total Kas (K)',
                formatRupiah(widget.report.kasBalanceRupiah),
              ),
              _buildMetric(
                'Terlokasi (L)',
                formatRupiah(widget.report.allocatedBalanceRupiah),
              ),
              _buildMetric(
                'Sisa (U)',
                formatRupiah(widget.report.unallocatedBalanceRupiah),
              ),
              _buildMetric(
                'Selisih (D)',
                formatRupiah(widget.report.discrepancyRupiah),
                color: _statusColor,
              ),
            ],
          ),
          if (widget.report.anomaliesDetected) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: AppColors.danger.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(6),
              ),
              child: const Row(
                children: [
                  Icon(Icons.warning, color: AppColors.danger, size: 16),
                  SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      'Peringatan: Terdapat mutasi tanpa referensi transaksi kas valid.',
                      style: TextStyle(color: AppColors.danger, fontSize: 11),
                    ),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 10),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: widget.onAllocate,
              icon: const Icon(Icons.account_balance_wallet, size: 16),
              label: Text(
                'Alokasikan Saldo Awal (${formatRupiah(widget.report.unallocatedBalanceRupiah)})',
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMetric(String label, String value, {Color? color}) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: const TextStyle(fontSize: 10, color: Colors.grey)),
        const SizedBox(height: 2),
        Text(
          value,
          style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: color,
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildHeader(),
        AnimatedCrossFade(
          firstChild: const SizedBox.shrink(),
          secondChild: _buildExpandedContent(),
          firstCurve: const Interval(0.0, 1.0, curve: Curves.fastOutSlowIn),
          secondCurve: const Interval(0.0, 1.0, curve: Curves.fastOutSlowIn),
          sizeCurve: Curves.easeInOut,
          crossFadeState: _expanded
              ? CrossFadeState.showSecond
              : CrossFadeState.showFirst,
          duration: const Duration(milliseconds: 300),
        ),
      ],
    );
  }
}
