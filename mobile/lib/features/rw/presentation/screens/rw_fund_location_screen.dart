import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

class RwFundLocationScreen extends ConsumerStatefulWidget {
  final String rwId;

  const RwFundLocationScreen({super.key, required this.rwId});

  @override
  ConsumerState<RwFundLocationScreen> createState() =>
      _RwFundLocationScreenState();
}

class _RwFundLocationScreenState
    extends ConsumerState<RwFundLocationScreen> {
  List<RwFundLocation> _locations = [];
  bool _isLoading = true;
  String? _error;

  String _selectedCategory = 'all';
  String _selectedStatus = 'all';
  String _searchQuery = '';

  bool _canCreateRw(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (user['role'] == AppRole.superAdmin || user['system_role'] == 'super_admin') return true;
    final rwMemberships = (user['rw_memberships'] as List?)
        ?.whereType<Map<String, dynamic>>()
        .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
        .toList() ?? <RWMembership>[];
    final currentMembership = rwMemberships.where((m) => m.rwId == widget.rwId).firstOrNull;
    final jabatan = (currentMembership?.jabatan ?? '').toLowerCase();
    return jabatan == 'bendahara' || jabatan == 'ketua' || jabatan == 'wakil_ketua';
  }

  bool _canUpdateRw(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (user['role'] == AppRole.superAdmin || user['system_role'] == 'super_admin') return true;
    final rwMemberships = (user['rw_memberships'] as List?)
        ?.whereType<Map<String, dynamic>>()
        .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
        .toList() ?? <RWMembership>[];
    final currentMembership = rwMemberships.where((m) => m.rwId == widget.rwId).firstOrNull;
    final jabatan = (currentMembership?.jabatan ?? '').toLowerCase();
    return jabatan == 'bendahara' || jabatan == 'ketua';
  }

  @override
  void initState() {
    super.initState();
    _fetchData();
  }

  Future<void> _fetchData() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final api = ref.read(rwApiClientProvider);
      final locations = await api.getLocations(widget.rwId);

      if (mounted) {
        setState(() => _locations = locations);
      }
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  List<RwFundLocation> get _filteredLocations {
    var locs = _locations;

    if (_selectedCategory != 'all') {
      locs = locs.where((l) => l.category == _selectedCategory).toList();
    }
    if (_selectedStatus == 'active') {
      locs = locs.where((l) => l.isActive).toList();
    } else if (_selectedStatus == 'inactive') {
      locs = locs.where((l) => !l.isActive).toList();
    }
    if (_searchQuery.isNotEmpty) {
      final q = _searchQuery.toLowerCase();
      locs = locs.where((l) =>
          l.name.toLowerCase().contains(q) ||
          (l.bankName ?? '').toLowerCase().contains(q) ||
          (l.accountNumber ?? '').toLowerCase().contains(q)).toList();
    }

    return locs;
  }

  // Summary calculations
  double get _totalBalance {
    return _locations.fold(0, (sum, loc) {
      final bal = double.tryParse(loc.currentBalance) ?? 0;
      return sum + bal;
    });
  }

  Map<String, double> get _categoryTotals {
    return {
      'rekening': _locations
          .where((l) => l.category == 'rekening')
          .fold(0, (sum, loc) => sum + (double.tryParse(loc.currentBalance) ?? 0)),
      'brankas': _locations
          .where((l) => l.category == 'brankas')
          .fold(0, (sum, loc) => sum + (double.tryParse(loc.currentBalance) ?? 0)),
      'coh': _locations
          .where((l) => l.category == 'coh')
          .fold(0, (sum, loc) => sum + (double.tryParse(loc.currentBalance) ?? 0)),
    };
  }

  int get _activeLocationCount {
    return _locations.where((l) => l.isActive).length;
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canCreate = _canCreateRw(user);
    final canUpdate = _canUpdateRw(user);
    final isReadOnly = !canCreate && !canUpdate;

    final rwMemberships = (user?['rw_memberships'] as List?)
        ?.whereType<Map<String, dynamic>>()
        .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
        .toList() ?? <RWMembership>[];
    final currentMembership = rwMemberships.where((m) => m.rwId == widget.rwId).firstOrNull;
    final rwDisplayName = currentMembership?.rwName ?? 'RW ${widget.rwId}';

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: true,
        centerTitle: false,
        title: MenuAppBarTitle(title: 'Lokasi Dana RW', rw: widget.rwId),
      ),
      body: NestedScrollView(
        headerSliverBuilder: (ctx, innerBoxIsScrolled) => [
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 32,
                        height: 32,
                        decoration: BoxDecoration(
                          color: AppColors.accent,
                          borderRadius:
                              BorderRadius.circular(AppRadius.sm),
                        ),
                        child: const Icon(Icons.business,
                            color: Colors.white, size: 18),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text(
                              'Lokasi Dana RW',
                              style: TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700),
                            ),
                            Text(
                              !isReadOnly ? 'Kelola Lokasi Dana $rwDisplayName' : '$rwDisplayName — Read Only',
                              style: TextStyle(
                                fontSize: 12,
                                color: AppColors.textMutedColor(context),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  if (isReadOnly) ...[
                    Container(
                      padding: const EdgeInsets.all(12),
                      margin: const EdgeInsets.only(bottom: 12),
                      decoration: BoxDecoration(
                        color: AppColors.warning.withValues(alpha: 0.1),
                        borderRadius:
                            BorderRadius.circular(AppRadius.sm),
                        border: Border.all(
                            color: AppColors.warning.withValues(alpha: 0.3)),
                      ),
                      child: Row(
                        children: [
                          const Icon(Icons.info_outline,
                              color: AppColors.warning, size: 18),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              'Mode Read-Only: Anda hanya memiliki akses melihat lokasi dana RW.',
                              style: TextStyle(
                                fontSize: 12,
                                color: AppColors.textColor(context),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                  _buildActionToolbar(context, canCreate: canCreate, canUpdate: canUpdate),
                ],
              ),
            ),
          ),
        ],
        body: RefreshIndicator(
          onRefresh: _fetchData,
          child: SingleChildScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.only(bottom: 32),
            child: Column(
              children: [
                // Category Summary
                _buildCategorySummary(context),
                const SizedBox(height: 12),

                // Main content
                _buildMainContent(context, canUpdate: canUpdate),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildActionToolbar(BuildContext context, {required bool canCreate, required bool canUpdate}) {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        if (canCreate)
          ElevatedButton.icon(
            onPressed: () => _showCreateLocationDialog(context),
            icon: const Icon(Icons.add, size: 16),
            label: const Text('Tambah Lokasi', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.accent,
              foregroundColor: Colors.white,
              visualDensity: VisualDensity.compact,
            ),
          ),
        if (canUpdate)
          OutlinedButton.icon(
            onPressed: () => _showTransferDialog(context),
            icon: const Icon(Icons.swap_horiz, size: 16),
            label: const Text('Transfer Antar-Lokasi', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
            style: OutlinedButton.styleFrom(
              visualDensity: VisualDensity.compact,
            ),
          ),
        OutlinedButton.icon(
          onPressed: () => _showTransfersHistoryDialog(context, canUpdate),
          icon: const Icon(Icons.history, size: 16),
          label: const Text('Riwayat Transfer', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
          style: OutlinedButton.styleFrom(
            visualDensity: VisualDensity.compact,
          ),
        ),
        OutlinedButton.icon(
          onPressed: () => _showReconcileDialog(context),
          icon: const Icon(Icons.account_balance, size: 16),
          label: const Text('Rekonsiliasi', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
          style: OutlinedButton.styleFrom(
            visualDensity: VisualDensity.compact,
          ),
        ),
      ],
    );
  }

  Widget _buildCategorySummary(BuildContext context) {
    final totals = _categoryTotals;

    return Padding(
      padding: const EdgeInsets.symmetric(
          horizontal: 20),
      child: Column(
        children: [
          // Total Card
          _buildSummaryCard(
            context,
            'Total Saldo',
            RupiahFormatter.format(_totalBalance.toInt()),
            AppColors.accent,
            Icons.business,
            '$_activeLocationCount lokasi aktif',
          ),
          const SizedBox(height: 12),

          Row(
            children: [
              Expanded(
                child: _buildSummaryCard(
                  context,
                  'Rekening Bank',
                  RupiahFormatter.format(
                      totals['rekening']?.toInt() ?? 0),
                  AppColors.info,
                  Icons.account_balance,
                  '${_locations.where((l) => l.category == 'rekening' && l.isActive).length} lokasi',
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildSummaryCard(
                  context,
                  'Brankas Fisik',
                  RupiahFormatter.format(
                      totals['brankas']?.toInt() ?? 0),
                  AppColors.warning,
                  Icons.security,
                  '${_locations.where((l) => l.category == 'brankas' && l.isActive).length} lokasi',
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),

          _buildSummaryCard(
            context,
            'Kas Di Tangan (COH)',
            RupiahFormatter.format(
                totals['coh']?.toInt() ?? 0),
            AppColors.success,
            Icons.wallet,
            '${_locations.where((l) => l.category == 'coh' && l.isActive).length} lokasi',
          ),
        ],
      ),
    );
  }

  Widget _buildSummaryCard(BuildContext context, String title, String value,
      Color color, IconData icon, String subtitle) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(AppRadius.base),
        border: Border.all(color: color.withValues(alpha: 0.1)),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            child: Icon(icon, size: 16, color: color),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontSize: 10,
                    fontWeight: FontWeight.w600,
                    color: AppColors.lightTextMuted,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  value,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: color,
                  ),
                ),
                Text(
                  subtitle,
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.textMutedColor(context),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMainContent(BuildContext context, {required bool canUpdate}) {
    return Card(
      margin: const EdgeInsets.fromLTRB(
          20, 12, 20, 0),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Filter toolbar
            _buildFilterToolbar(),
            const SizedBox(height: 12),

            // Error
            if (_error != null)
              _buildErrorBanner(),

            // Loading
            if (_isLoading) const _LoadingBanner(),

            // Empty
            if (!_isLoading && _locations.isEmpty)
              const _EmptyBanner(),

            // Locations table
            if (!_isLoading && _filteredLocations.isNotEmpty)
              _buildLocationsTable(context, canUpdate: canUpdate),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterToolbar() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            // Category filter
            Expanded(
              flex: 2,
              child: DropdownButtonFormField<String>(
                value: _selectedCategory,
                decoration: const InputDecoration(
                  labelText: 'Kategori',
                  isDense: true,
                  contentPadding:
                      EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  border: OutlineInputBorder(),
                ),
                items: const [
                  DropdownMenuItem(
                      value: 'all', child: Text('Semua Kategori')),
                  DropdownMenuItem(
                      value: 'rekening',
                      child: Text('Rekening Bank')),
                  DropdownMenuItem(
                      value: 'brankas', child: Text('Brankas Fisik')),
                  DropdownMenuItem(
                      value: 'coh',
                      child: Text('Kas Di Tangan (COH)')),
                ],
                onChanged: (v) {
                  if (v != null) setState(() => _selectedCategory = v);
                },
              ),
            ),
            const SizedBox(width: 8),
            // Status filter
            Expanded(
              flex: 2,
              child: DropdownButtonFormField<String>(
                value: _selectedStatus,
                decoration: const InputDecoration(
                  labelText: 'Status',
                  isDense: true,
                  contentPadding:
                      EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  border: OutlineInputBorder(),
                ),
                items: const [
                  DropdownMenuItem(
                      value: 'all', child: Text('Semua Status')),
                  DropdownMenuItem(
                      value: 'active', child: Text('Aktif')),
                  DropdownMenuItem(
                      value: 'inactive', child: Text('Nonaktif')),
                ],
                onChanged: (v) {
                  if (v != null) setState(() => _selectedStatus = v);
                },
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),

        // Search
        TextField(
          decoration: const InputDecoration(
            hintText: 'Cari nama lokasi, RT, bank...',
            prefixIcon: Icon(Icons.search, size: 18),
            isDense: true,
            contentPadding:
                EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            border: OutlineInputBorder(),
          ),
          onChanged: (v) => setState(() => _searchQuery = v),
        ),
      ],
    );
  }

  Widget _buildErrorBanner() {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.danger.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppRadius.sm),
        border:
            Border.all(color: AppColors.danger.withValues(alpha: 0.3)),
      ),
      child: Text(
        'Error: $_error',
        style: const TextStyle(color: AppColors.danger, fontSize: 13),
      ),
    );
  }

  Widget _buildLocationsTable(BuildContext context, {required bool canUpdate}) {
    final locs = _filteredLocations;

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minWidth: 700),
        child: DataTable(
          headingRowColor: WidgetStateProperty.all(
              AppColors.lightTextMuted.withValues(alpha: 0.05)),
          columns: [
            DataColumn(
                label: Text('NAMA LOKASI',
                    style: TextStyle(
                        fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(
                label: Text('KATEGORI',
                    style: TextStyle(
                        fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(
                label: Text('DETAIL / BANK',
                    style: TextStyle(
                        fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(
                label: Text('SALDO',
                    style: TextStyle(
                        fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(
                label: Text('STATUS',
                    style: TextStyle(
                        fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(
                label: Text('AKSI',
                    style: TextStyle(
                        fontSize: 10, color: AppColors.textMutedColor(context)))),
          ],
          rows: locs.map((loc) {
            final bal = double.tryParse(loc.currentBalance) ?? 0;
            final detailText = loc.category == 'rekening'
                ? [loc.bankName, loc.accountNumber].where((s) => s != null && s.isNotEmpty).join(' - ')
                : '—';

            return DataRow(cells: [
              DataCell(
                InkWell(
                  onTap: () => _showLocationDetailDialog(context, loc),
                  child: Text(
                    loc.name,
                    style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600, color: AppColors.accent),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
              DataCell(AppBadge(
                label: loc.category == 'rekening'
                    ? 'Rekening Bank'
                    : loc.category == 'brankas'
                        ? 'Brankas Fisik'
                        : 'Kas Di Tangan (COH)',
                backgroundColor: loc.category == 'rekening'
                    ? AppColors.info
                    : loc.category == 'brankas'
                        ? AppColors.warning
                        : AppColors.success,
              )),
              DataCell(Text(
                detailText.isNotEmpty ? detailText : '—',
                style: const TextStyle(fontSize: 12),
              )),
              DataCell(Text(
                RupiahFormatter.format(bal.toInt()),
                style: const TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                ),
              )),
              DataCell(AppBadge(
                label: loc.isActive ? 'Aktif' : 'Nonaktif',
                backgroundColor: loc.isActive ? AppColors.success : AppColors.danger,
              )),
              DataCell(
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    IconButton(
                      icon: const Icon(Icons.info_outline, size: 18),
                      tooltip: 'Detail Lokasi',
                      onPressed: () => _showLocationDetailDialog(context, loc),
                    ),
                    IconButton(
                      icon: const Icon(Icons.history, size: 18),
                      tooltip: 'Riwayat Mutasi',
                      onPressed: () => _showMutationsDialog(context, loc),
                    ),
                    if (canUpdate) ...[
                      IconButton(
                        icon: const Icon(Icons.edit, size: 18),
                        tooltip: 'Edit',
                        onPressed: () => _showEditLocationDialog(context, loc),
                      ),
                      IconButton(
                        icon: Icon(
                          loc.isActive ? Icons.power_settings_new : Icons.check_circle_outline,
                          size: 18,
                          color: loc.isActive ? AppColors.danger : AppColors.success,
                        ),
                        tooltip: loc.isActive ? 'Nonaktifkan' : 'Aktifkan',
                        onPressed: () => _toggleLocationStatus(context, loc),
                      ),
                    ],
                  ],
                ),
              ),
            ]);
          }).toList(),
        ),
      ),
    );
  }

  void _showLocationDetailDialog(BuildContext context, RwFundLocation loc) {
    final bal = double.tryParse(loc.currentBalance) ?? 0;
    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(loc.name),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildReconcileRow('Kategori:', loc.category == 'rekening'
                ? 'Rekening Bank'
                : loc.category == 'brankas'
                    ? 'Brankas Fisik'
                    : 'Kas Di Tangan (COH)'),
            if (loc.category == 'rekening') ...[
              _buildReconcileRow('Nama Bank:', loc.bankName ?? '—'),
              _buildReconcileRow('No. Rekening:', loc.accountNumber ?? '—'),
              _buildReconcileRow('Pemilik Rekening:', loc.accountHolder ?? '—'),
            ],
            _buildReconcileRow('Saldo Saat Ini:', RupiahFormatter.format(bal.toInt())),
            _buildReconcileRow('Status:', loc.isActive ? 'Aktif' : 'Nonaktif'),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _showMutationsDialog(context, loc);
            },
            child: const Text('Lihat Mutasi'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Tutup'),
          ),
        ],
      ),
    );
  }

  Future<void> _showCreateLocationDialog(BuildContext context) async {
    final nameCtrl = TextEditingController();
    final bankCtrl = TextEditingController();
    final noCtrl = TextEditingController();
    final holderCtrl = TextEditingController();
    String category = 'rekening';
    bool saving = false;
    String? error;

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (statefulCtx, setDialogState) {
            return AlertDialog(
              title: const Text('Tambah Lokasi Dana RW'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (error != null)
                      Container(
                        padding: const EdgeInsets.all(8),
                        margin: const EdgeInsets.only(bottom: 8),
                        decoration: BoxDecoration(
                          color: AppColors.danger.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: Text(error!, style: const TextStyle(color: AppColors.danger, fontSize: 12)),
                      ),
                    TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(labelText: 'Nama Lokasi *'),
                    ),
                    const SizedBox(height: 8),
                    DropdownButtonFormField<String>(
                      value: category,
                      decoration: const InputDecoration(labelText: 'Kategori *'),
                      items: const [
                        DropdownMenuItem(value: 'rekening', child: Text('Rekening Bank')),
                        DropdownMenuItem(value: 'brankas', child: Text('Brankas Fisik')),
                        DropdownMenuItem(value: 'coh', child: Text('Kas Di Tangan (COH)')),
                      ],
                      onChanged: (val) {
                        if (val != null) setDialogState(() => category = val);
                      },
                    ),
                    if (category == 'rekening') ...[
                      const SizedBox(height: 8),
                      TextField(
                        controller: bankCtrl,
                        decoration: const InputDecoration(labelText: 'Nama Bank * (mis. BCA, Mandiri)'),
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: noCtrl,
                        decoration: const InputDecoration(labelText: 'Nomor Rekening'),
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: holderCtrl,
                        decoration: const InputDecoration(labelText: 'Nama Pemilik Rekening'),
                      ),
                    ],
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: saving ? null : () => Navigator.pop(dialogCtx),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  onPressed: saving ? null : () async {
                    if (nameCtrl.text.trim().isEmpty) {
                      setDialogState(() => error = 'Nama lokasi wajib diisi');
                      return;
                    }
                    if (category == 'rekening' && bankCtrl.text.trim().isEmpty) {
                      setDialogState(() => error = 'Nama bank wajib diisi untuk rekening');
                      return;
                    }
                    setDialogState(() {
                      saving = true;
                      error = null;
                    });
                    try {
                      final api = ref.read(rwApiClientProvider);
                      await api.createLocation(widget.rwId, {
                        'name': nameCtrl.text.trim(),
                        'category': category,
                        if (category == 'rekening') 'bank_name': bankCtrl.text.trim(),
                        if (category == 'rekening' && noCtrl.text.trim().isNotEmpty)
                          'account_number': noCtrl.text.trim(),
                        if (category == 'rekening' && holderCtrl.text.trim().isNotEmpty)
                          'account_holder': holderCtrl.text.trim(),
                      });
                      if (context.mounted) {
                        Navigator.pop(dialogCtx);
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Lokasi dana RW berhasil ditambahkan')),
                        );
                        _fetchData();
                      }
                    } catch (e) {
                      setDialogState(() {
                        saving = false;
                        error = e.toString();
                      });
                    }
                  },
                  child: saving
                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Simpan'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _showEditLocationDialog(BuildContext context, RwFundLocation loc) async {
    final nameCtrl = TextEditingController(text: loc.name);
    final bankCtrl = TextEditingController(text: loc.bankName ?? '');
    final noCtrl = TextEditingController(text: loc.accountNumber ?? '');
    final holderCtrl = TextEditingController(text: loc.accountHolder ?? '');
    bool saving = false;
    String? error;

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (statefulCtx, setDialogState) {
            return AlertDialog(
              title: Text('Edit ${loc.name}'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (error != null)
                      Container(
                        padding: const EdgeInsets.all(8),
                        margin: const EdgeInsets.only(bottom: 8),
                        decoration: BoxDecoration(
                          color: AppColors.danger.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: Text(error!, style: const TextStyle(color: AppColors.danger, fontSize: 12)),
                      ),
                    TextField(
                      controller: nameCtrl,
                      decoration: const InputDecoration(labelText: 'Nama Lokasi *'),
                    ),
                    if (loc.category == 'rekening') ...[
                      const SizedBox(height: 8),
                      TextField(
                        controller: bankCtrl,
                        decoration: const InputDecoration(labelText: 'Nama Bank'),
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: noCtrl,
                        decoration: const InputDecoration(labelText: 'Nomor Rekening'),
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: holderCtrl,
                        decoration: const InputDecoration(labelText: 'Nama Pemilik Rekening'),
                      ),
                    ],
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: saving ? null : () => Navigator.pop(dialogCtx),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  onPressed: saving ? null : () async {
                    if (nameCtrl.text.trim().isEmpty) {
                      setDialogState(() => error = 'Nama lokasi wajib diisi');
                      return;
                    }
                    setDialogState(() {
                      saving = true;
                      error = null;
                    });
                    try {
                      final api = ref.read(rwApiClientProvider);
                      await api.updateLocation(widget.rwId, loc.id, {
                        'name': nameCtrl.text.trim(),
                        if (loc.category == 'rekening') 'bank_name': bankCtrl.text.trim(),
                        if (loc.category == 'rekening') 'account_number': noCtrl.text.trim(),
                        if (loc.category == 'rekening') 'account_holder': holderCtrl.text.trim(),
                      });
                      if (context.mounted) {
                        Navigator.pop(dialogCtx);
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Lokasi dana berhasil diperbarui')),
                        );
                        _fetchData();
                      }
                    } catch (e) {
                      setDialogState(() {
                        saving = false;
                        error = e.toString();
                      });
                    }
                  },
                  child: saving
                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Simpan'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _toggleLocationStatus(BuildContext context, RwFundLocation loc) async {
    final messenger = ScaffoldMessenger.of(context);
    final bal = double.tryParse(loc.currentBalance) ?? 0;
    if (loc.isActive && bal != 0) {
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Tidak Dapat Dinonaktifkan'),
          content: Text(
            'Lokasi "${loc.name}" masih memiliki saldo sebesar ${RupiahFormatter.format(bal.toInt())}. '
            'Saldo harus Rp 0 terlebih dahulu sebelum dapat dinonaktifkan.',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Tutup'),
            ),
          ],
        ),
      );
      return;
    }

    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(loc.isActive ? 'Nonaktifkan Lokasi?' : 'Aktifkan Lokasi?'),
        content: Text(
          loc.isActive
              ? 'Lokasi "${loc.name}" tidak dapat digunakan untuk mutasi baru setelah dinonaktifkan.'
              : 'Aktifkan kembali lokasi "${loc.name}"?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
              backgroundColor: loc.isActive ? AppColors.danger : AppColors.success,
              foregroundColor: Colors.white,
            ),
            child: Text(loc.isActive ? 'Nonaktifkan' : 'Aktifkan'),
          ),
        ],
      ),
    );

    if (confirm != true || !mounted) return;

    try {
      final api = ref.read(rwApiClientProvider);
      await api.toggleLocationStatus(widget.rwId, loc.id, !loc.isActive);
      if (mounted) {
        messenger.showSnackBar(
          SnackBar(content: Text('Status "${loc.name}" berhasil diubah')),
        );
        _fetchData();
      }
    } catch (e) {
      if (mounted) {
        messenger.showSnackBar(
          SnackBar(content: Text('Gagal mengubah status: $e')),
        );
      }
    }
  }

  Future<void> _showTransferDialog(BuildContext context) async {
    final activeLocs = _locations.where((l) => l.isActive).toList();
    if (activeLocs.length < 2) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Dibutuhkan minimal 2 lokasi aktif untuk melakukan transfer')),
      );
      return;
    }

    String? fromId;
    String? toId;
    final amountCtrl = TextEditingController();
    final notesCtrl = TextEditingController();
    bool saving = false;
    String? error;

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (statefulCtx, setDialogState) {
            final fromLoc = activeLocs.where((l) => l.id == fromId).firstOrNull;
            final fromBal = fromLoc != null ? (double.tryParse(fromLoc.currentBalance) ?? 0) : 0;

            return AlertDialog(
              title: const Text('Transfer Antar-Lokasi RW'),
              content: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (error != null)
                      Container(
                        padding: const EdgeInsets.all(8),
                        margin: const EdgeInsets.only(bottom: 8),
                        decoration: BoxDecoration(
                          color: AppColors.danger.withValues(alpha: 0.1),
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: Text(error!, style: const TextStyle(color: AppColors.danger, fontSize: 12)),
                      ),
                    DropdownButtonFormField<String>(
                      value: fromId,
                      decoration: const InputDecoration(labelText: 'Dari Lokasi (Sumber) *'),
                      items: activeLocs.map((l) {
                        final b = double.tryParse(l.currentBalance) ?? 0;
                        return DropdownMenuItem(
                          value: l.id,
                          child: Text('${l.name} (${RupiahFormatter.format(b.toInt())})'),
                        );
                      }).toList(),
                      onChanged: (val) {
                        setDialogState(() {
                          fromId = val;
                          if (toId == val) toId = null;
                        });
                      },
                    ),
                    const SizedBox(height: 8),
                    DropdownButtonFormField<String>(
                      value: toId,
                      decoration: const InputDecoration(labelText: 'Ke Lokasi (Tujuan) *'),
                      items: activeLocs.where((l) => l.id != fromId).map((l) {
                        return DropdownMenuItem(
                          value: l.id,
                          child: Text(l.name),
                        );
                      }).toList(),
                      onChanged: (val) {
                        setDialogState(() => toId = val);
                      },
                    ),
                    const SizedBox(height: 8),
                    TextField(
                      controller: amountCtrl,
                      keyboardType: TextInputType.number,
                      decoration: InputDecoration(
                        labelText: 'Nominal Transfer (Rp) *',
                        helperText: fromLoc != null
                            ? 'Saldo tersedia: ${RupiahFormatter.format(fromBal.toInt())}'
                            : null,
                      ),
                    ),
                    const SizedBox(height: 8),
                    TextField(
                      controller: notesCtrl,
                      decoration: const InputDecoration(labelText: 'Catatan (Opsional)'),
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
                  onPressed: saving ? null : () async {
                    if (fromId == null || toId == null) {
                      setDialogState(() => error = 'Pilih lokasi sumber dan tujuan');
                      return;
                    }
                    if (fromId == toId) {
                      setDialogState(() => error = 'Lokasi sumber dan tujuan tidak boleh sama');
                      return;
                    }
                    final amt = double.tryParse(amountCtrl.text.replaceAll(RegExp(r'[^0-9]'), '')) ?? 0;
                    if (amt <= 0) {
                      setDialogState(() => error = 'Nominal transfer harus lebih dari 0');
                      return;
                    }
                    if (amt > fromBal) {
                      setDialogState(() => error = 'Saldo tidak mencukupi untuk transfer');
                      return;
                    }
                    setDialogState(() {
                      saving = true;
                      error = null;
                    });
                    try {
                      final api = ref.read(rwApiClientProvider);
                      final now = DateTime.now();
                      final dateStr = '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
                      await api.createTransfer(widget.rwId, {
                        'from_location_id': fromId,
                        'to_location_id': toId,
                        'amount': '${amt.toInt()}.00',
                        'transfer_date': dateStr,
                        if (notesCtrl.text.trim().isNotEmpty) 'notes': notesCtrl.text.trim(),
                      });
                      if (context.mounted) {
                        Navigator.pop(dialogCtx);
                        ScaffoldMessenger.of(context).showSnackBar(
                          const SnackBar(content: Text('Transfer dana RW berhasil dilakukan')),
                        );
                        _fetchData();
                      }
                    } catch (e) {
                      setDialogState(() {
                        saving = false;
                        error = e.toString();
                      });
                    }
                  },
                  child: saving
                      ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                      : const Text('Transfer'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _showTransfersHistoryDialog(BuildContext context, bool canUpdate) async {
    showDialog<void>(
      context: context,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (dCtx, setDialogState) {
            return AlertDialog(
              title: const Text('Riwayat Transfer Antar-Lokasi RW'),
              content: SizedBox(
                width: double.maxFinite,
                height: 450,
                child: FutureBuilder<List<RwFundTransfer>>(
                  future: ref.read(rwApiClientProvider).getTransfers(widget.rwId),
                  builder: (fCtx, snapshot) {
                    if (snapshot.connectionState == ConnectionState.waiting) {
                      return const Center(child: CircularProgressIndicator());
                    }
                    if (snapshot.hasError) {
                      return Center(child: Text('Gagal memuat riwayat transfer: ${snapshot.error}'));
                    }
                    final transfers = snapshot.data ?? [];
                    if (transfers.isEmpty) {
                      return const Center(child: Text('Belum ada riwayat transfer dana antar-lokasi.'));
                    }
                    return ListView.separated(
                      itemCount: transfers.length,
                      separatorBuilder: (context, index) => const Divider(),
                      itemBuilder: (_, idx) {
                        final t = transfers[idx];
                        final amt = double.tryParse(t.amount) ?? 0;
                        final isReversed = t.isReversal || t.reversesTransferId != null;

                        return Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                children: [
                                  Text(
                                    t.transferDate,
                                    style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
                                  ),
                                  AppBadge(
                                    label: isReversed ? 'Dibatalkan (Reversal)' : 'Berhasil',
                                    backgroundColor: isReversed ? AppColors.danger : AppColors.success,
                                  ),
                                ],
                              ),
                              const SizedBox(height: 4),
                              Row(
                                children: [
                                  Expanded(
                                    child: Text(
                                      '${t.fromLocationName ?? 'Lokasi Asal'}  ➔  ${t.toLocationName ?? 'Lokasi Tujuan'}',
                                      style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w500),
                                    ),
                                  ),
                                  Text(
                                    RupiahFormatter.format(amt.toInt()),
                                    style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: AppColors.accent),
                                  ),
                                ],
                              ),
                              if (t.notes?.isNotEmpty == true) ...[
                                const SizedBox(height: 2),
                                Text(
                                  'Catatan: ${t.notes}',
                                  style: TextStyle(fontSize: 11, color: AppColors.textMutedColor(context)),
                                ),
                              ],
                              if (canUpdate && !isReversed) ...[
                                const SizedBox(height: 6),
                                Align(
                                  alignment: Alignment.centerRight,
                                  child: OutlinedButton.icon(
                                    onPressed: () async {
                                      final reasonCtrl = TextEditingController();
                                      final confirm = await showDialog<bool>(
                                        context: context,
                                        builder: (cCtx) => AlertDialog(
                                          title: const Text('Batalkan Transfer (Reversal)?'),
                                          content: Column(
                                            mainAxisSize: MainAxisSize.min,
                                            crossAxisAlignment: CrossAxisAlignment.start,
                                            children: [
                                              Text(
                                                'Transfer sebesar ${RupiahFormatter.format(amt.toInt())} dari '
                                                '${t.fromLocationName ?? 'Asal'} ke ${t.toLocationName ?? 'Tujuan'} '
                                                'akan dikembalikan (reversal).',
                                                style: const TextStyle(fontSize: 13),
                                              ),
                                              const SizedBox(height: 12),
                                              TextField(
                                                controller: reasonCtrl,
                                                decoration: const InputDecoration(
                                                  labelText: 'Alasan Pembatalan / Reversal *',
                                                  hintText: 'Contoh: Salah pilih lokasi tujuan...',
                                                  isDense: true,
                                                  border: OutlineInputBorder(),
                                                ),
                                              ),
                                            ],
                                          ),
                                          actions: [
                                            TextButton(
                                              onPressed: () => Navigator.pop(cCtx, false),
                                              child: const Text('Batal'),
                                            ),
                                            ElevatedButton(
                                              onPressed: () {
                                                if (reasonCtrl.text.trim().isEmpty) return;
                                                Navigator.pop(cCtx, true);
                                              },
                                              style: ElevatedButton.styleFrom(
                                                backgroundColor: AppColors.danger,
                                                foregroundColor: Colors.white,
                                              ),
                                              child: const Text('Batalkan Transfer'),
                                            ),
                                          ],
                                        ),
                                      );

                                      if (confirm == true && context.mounted) {
                                        try {
                                          await ref.read(rwApiClientProvider).reverseTransfer(
                                            widget.rwId,
                                            t.id,
                                            reasonCtrl.text.trim(),
                                          );
                                          if (context.mounted) {
                                            ScaffoldMessenger.of(context).showSnackBar(
                                              const SnackBar(content: Text('Transfer berhasil dibatalkan (reversal)')),
                                            );
                                            setDialogState(() {});
                                            _fetchData();
                                          }
                                        } catch (e) {
                                          if (context.mounted) {
                                            ScaffoldMessenger.of(context).showSnackBar(
                                              SnackBar(content: Text('Gagal membatalkan transfer: $e')),
                                            );
                                          }
                                        }
                                      }
                                    },
                                    icon: const Icon(Icons.undo, size: 14, color: AppColors.danger),
                                    label: const Text(
                                      'Batalkan Transfer',
                                      style: TextStyle(fontSize: 11, color: AppColors.danger),
                                    ),
                                    style: OutlinedButton.styleFrom(
                                      visualDensity: VisualDensity.compact,
                                      side: const BorderSide(color: AppColors.danger),
                                    ),
                                  ),
                                ),
                              ],
                            ],
                          ),
                        );
                      },
                    );
                  },
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(ctx),
                  child: const Text('Tutup'),
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _showMutationsDialog(BuildContext context, RwFundLocation loc) async {
    showDialog<void>(
      context: context,
      builder: (ctx) {
        return AlertDialog(
          title: Text('Mutasi: ${loc.name}'),
          content: SizedBox(
            width: double.maxFinite,
            height: 400,
            child: FutureBuilder<List<RwLocationMutation>>(
              future: ref.read(rwApiClientProvider).getLocationMutations(widget.rwId, loc.id),
              builder: (fCtx, snapshot) {
                if (snapshot.connectionState == ConnectionState.waiting) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (snapshot.hasError) {
                  return Center(child: Text('Gagal memuat mutasi: ${snapshot.error}'));
                }
                final mutations = snapshot.data ?? [];
                if (mutations.isEmpty) {
                  return const Center(child: Text('Belum ada mutasi untuk lokasi ini.'));
                }
                return ListView.separated(
                  itemCount: mutations.length,
                  separatorBuilder: (context, index) => const Divider(),
                  itemBuilder: (_, idx) {
                    final m = mutations[idx];
                    final delta = double.tryParse(m.amountDelta) ?? 0;
                    final isPositive = delta > 0;
                    return ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      title: Text(
                        m.notes?.isNotEmpty == true ? m.notes! : m.changeType,
                        style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                      ),
                      subtitle: Text(
                        '${m.changeType} • ${m.createdAt.day}/${m.createdAt.month}/${m.createdAt.year}',
                        style: const TextStyle(fontSize: 11),
                      ),
                      trailing: Text(
                        '${isPositive ? '+' : ''}${RupiahFormatter.format(delta.toInt())}',
                        style: TextStyle(
                          fontWeight: FontWeight.w700,
                          color: isPositive ? AppColors.success : AppColors.danger,
                        ),
                      ),
                    );
                  },
                );
              },
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Tutup'),
            ),
          ],
        );
      },
    );
  }

  Future<void> _showReconcileDialog(BuildContext context) async {
    showDialog<void>(
      context: context,
      builder: (ctx) {
        return AlertDialog(
          title: const Text('Rekonsiliasi Kas RW'),
          content: SizedBox(
            width: double.maxFinite,
            child: FutureBuilder<RwReconciliationReport>(
              future: ref.read(rwApiClientProvider).getReconciliation(widget.rwId),
              builder: (fCtx, snapshot) {
                if (snapshot.connectionState == ConnectionState.waiting) {
                  return const Center(child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()));
                }
                if (snapshot.hasError) {
                  return Center(child: Text('Gagal memuat rekonsiliasi: ${snapshot.error}'));
                }
                final r = snapshot.data!;
                final calc = double.tryParse(r.calculatedBalance) ?? 0;
                final totalLoc = double.tryParse(r.totalLocationBalances) ?? 0;
                final disc = double.tryParse(r.discrepancy) ?? 0;

                return Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: r.isReconciled
                            ? AppColors.success.withValues(alpha: 0.1)
                            : AppColors.danger.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        border: Border.all(
                          color: r.isReconciled
                              ? AppColors.success.withValues(alpha: 0.3)
                              : AppColors.danger.withValues(alpha: 0.3),
                        ),
                      ),
                      child: Row(
                        children: [
                          Icon(
                            r.isReconciled ? Icons.check_circle : Icons.warning_amber_rounded,
                            color: r.isReconciled ? AppColors.success : AppColors.danger,
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              r.isReconciled
                                  ? 'TEREKONSILIASI: Total saldo lokasi dana cocok dengan saldo ledger KAS RW.'
                                  : 'SELISIH DITEMUKAN: Terdapat selisih sebesar ${RupiahFormatter.format(disc.toInt())}.',
                              style: TextStyle(
                                fontSize: 12,
                                fontWeight: FontWeight.w600,
                                color: r.isReconciled ? AppColors.success : AppColors.danger,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 16),
                    _buildReconcileRow('Saldo Ledger KAS RW:', RupiahFormatter.format(calc.toInt())),
                    _buildReconcileRow('Total Saldo Lokasi Dana:', RupiahFormatter.format(totalLoc.toInt())),
                    _buildReconcileRow('Selisih (Discrepancy):', RupiahFormatter.format(disc.toInt()),
                        isDanger: !r.isReconciled),
                  ],
                );
              },
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Tutup'),
            ),
          ],
        );
      },
    );
  }

  Widget _buildReconcileRow(String label, String val, {bool isDanger = false}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(fontSize: 13)),
          Text(
            val,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w700,
              color: isDanger ? AppColors.danger : null,
            ),
          ),
        ],
      ),
    );
  }
}

class _LoadingBanner extends StatelessWidget {
  const _LoadingBanner();

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.lightTextMuted.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppRadius.sm),
      ),
      child: const Text(
        'Memuat data lokasi dana RW...',
        style: TextStyle(color: AppColors.lightTextMuted, fontSize: 13),
      ),
    );
  }
}

class _EmptyBanner extends StatelessWidget {
  const _EmptyBanner();

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 32),
        child: Text(
          'Belum ada lokasi dana pada RW ini.',
          style: TextStyle(
            color: AppColors.lightTextMuted,
            fontSize: 13,
          ),
        ),
      ),
    );
  }
}
