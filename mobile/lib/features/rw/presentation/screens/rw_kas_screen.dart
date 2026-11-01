import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

/// KAS screen untuk konteks RW — catatan transaksi KAS RW dan riwayat.
class RwKasScreen extends ConsumerStatefulWidget {
  final String rwId;

  const RwKasScreen({super.key, required this.rwId});

  @override
  ConsumerState<RwKasScreen> createState() => _RwKasScreenState();
}

class _RwKasScreenState extends ConsumerState<RwKasScreen> {
  RwKasReport? _kasReport;
  List<RwTransaction> _transactions = [];
  bool _isLoading = true;
  String? _error;
  int _selectedMonth = DateTime.now().month;
  int _selectedYear = DateTime.now().year;
  bool _isAllPeriods = false;
  String _searchQuery = '';
  String _filterArus = 'all';

  String get _periodKey {
    if (_isAllPeriods) return 'all';
    return '$_selectedYear-${_selectedMonth.toString().padLeft(2, '0')}';
  }

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
      final period = _isAllPeriods ? 'all' : _periodKey;
      final results = await Future.wait([
        api.getKas(widget.rwId, period: period),
        api.getTransactions(
          rwId: widget.rwId,
          period: period,
          page: 1,
          pageSize: 100,
        ),
      ]);
      if (mounted) {
        setState(() {
          _kasReport = results[0] as RwKasReport;
          _transactions = (results[1] as RwTransactionsResponse).data;
        });
      }
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _isLoading = false);
    }
  }

  List<RwTransaction> get _filteredTransactions {
    var txs = _transactions;
    if (_filterArus != 'all') {
      txs = txs.where((t) => t.type == _filterArus).toList();
    }
    if (_searchQuery.isNotEmpty) {
      final q = _searchQuery.toLowerCase();
      txs = txs.where((t) =>
          t.description.toLowerCase().contains(q) ||
          t.rtName.toLowerCase().contains(q) ||
          (t.locationName ?? '').toLowerCase().contains(q)).toList();
    }
    return txs;
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canCreate = _canCreateRw(user);

    final rwMemberships = (user?['rw_memberships'] as List?)
        ?.whereType<Map<String, dynamic>>()
        .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
        .toList() ?? <RWMembership>[];
    final currentMembership = rwMemberships.where((m) => m.rwId == widget.rwId).firstOrNull;
    final rwNumberText = currentMembership?.rwNumber != null
        ? currentMembership!.rwNumber!.toString().padLeft(3, '0')
        : resolveRwNumber(widget.rwId, rwMemberships);
    final rwDisplayName = currentMembership?.rwName ?? (rwNumberText != null ? 'RW $rwNumberText' : 'RW');

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: true,
        centerTitle: false,
        title: MenuAppBarTitle(title: 'KAS RW', rw: rwNumberText),
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
                          borderRadius: BorderRadius.circular(AppRadius.sm),
                        ),
                        child: const Icon(Icons.account_balance_wallet,
                            color: Colors.white, size: 18),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text('KAS RW',
                                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                            Text(
                              canCreate ? 'Kelola Transaksi KAS $rwDisplayName' : '$rwDisplayName — Read Only',
                              style: TextStyle(fontSize: 12, color: AppColors.textMutedColor(context)),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  if (canCreate)
                    _buildActionToolbar(context)
                  else
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: AppColors.warning.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(AppRadius.sm),
                        border: Border.all(color: AppColors.warning.withValues(alpha: 0.3)),
                      ),
                      child: Row(
                        children: [
                          const Icon(Icons.info_outline, color: AppColors.warning, size: 18),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(
                              'Mode Read-Only: Anda hanya memiliki akses melihat transaksi KAS RW.',
                              style: TextStyle(fontSize: 12, color: AppColors.textColor(context)),
                            ),
                          ),
                        ],
                      ),
                    ),
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
                const SizedBox(height: 12),
                _buildKasSummary(),
                const SizedBox(height: 12),
                _buildMainContent(),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildActionToolbar(BuildContext context) {
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        ElevatedButton.icon(
          onPressed: () => _showCreateTransactionDialog(context, 'income'),
          icon: const Icon(Icons.arrow_downward, size: 16),
          label: const Text('Catat Kas Masuk', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.success,
            foregroundColor: Colors.white,
            visualDensity: VisualDensity.compact,
          ),
        ),
        ElevatedButton.icon(
          onPressed: () => _showCreateTransactionDialog(context, 'expense'),
          icon: const Icon(Icons.arrow_upward, size: 16),
          label: const Text('Catat Kas Keluar', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.danger,
            foregroundColor: Colors.white,
            visualDensity: VisualDensity.compact,
          ),
        ),
      ],
    );
  }

  Widget _buildKasSummary() {
    if (_isLoading) {
      return const Center(child: Padding(
        padding: EdgeInsets.all(32), child: CircularProgressIndicator()));
    }
    if (_kasReport == null) return const SizedBox.shrink();

    final openingBal = RupiahFormatter.parseMoneyString(_kasReport!.openingBalance);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 20),
      child: Column(
        children: [
          if (openingBal > 0) ...[
            _buildSummaryCard(
              'Saldo Awal KAS RW',
              RupiahFormatter.format(openingBal),
              AppColors.info,
              Icons.flag_outlined,
            ),
            const SizedBox(height: 12),
          ],
          _buildSummaryCard('Total Pemasukan', RupiahFormatter.format(RupiahFormatter.parseMoneyString(_kasReport!.totalIncome)), AppColors.success, Icons.arrow_downward),
          const SizedBox(height: 12),
          _buildSummaryCard('Total Pengeluaran', RupiahFormatter.format(RupiahFormatter.parseMoneyString(_kasReport!.totalExpense)), AppColors.danger, Icons.arrow_upward),
          const SizedBox(height: 12),
          _buildSummaryCard('Saldo Bersih', RupiahFormatter.format(RupiahFormatter.parseMoneyString(_kasReport!.netBalance)), AppColors.accent, Icons.account_balance),
          const SizedBox(height: 12),
          _buildSummaryCard('Transaksi', _kasReport!.transactionCount.toString(), AppColors.lightTextMuted, Icons.receipt_long),
        ],
      ),
    );
  }

  Widget _buildSummaryCard(String title, String value, Color color, IconData icon) {
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
            decoration: BoxDecoration(color: color.withValues(alpha: 0.1), borderRadius: BorderRadius.circular(AppRadius.sm)),
            child: Icon(icon, size: 16, color: color),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: const TextStyle(fontSize: 10, fontWeight: FontWeight.w600, color: AppColors.lightTextMuted)),
                const SizedBox(height: 2),
                Text(value, style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: color)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMainContent() {
    return Card(
      margin: const EdgeInsets.fromLTRB(20, 12, 20, 0),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildFilterToolbar(),
            const SizedBox(height: 12),
            if (_error != null) _buildErrorBanner(),
            if (_isLoading) const _LoadingBanner(),
            if (!_isLoading && _transactions.isEmpty) const _EmptyBanner(),
            if (!_isLoading && _filteredTransactions.isNotEmpty) _buildTransactionsTable(),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterToolbar() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          borderRadius: BorderRadius.circular(AppRadius.sm),
          onTap: _showMonthPicker,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            decoration: BoxDecoration(
              color: AppColors.info.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(AppRadius.sm),
              border: Border.all(color: AppColors.info.withValues(alpha: 0.25)),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.calendar_today, size: 14, color: AppColors.accent),
                const SizedBox(width: 6),
                Text(
                  _isAllPeriods ? 'Semua Periode' : '${_monthName(_selectedMonth)} $_selectedYear',
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: AppColors.accent),
                ),
                const SizedBox(width: 4),
                const Icon(Icons.arrow_drop_down, size: 18, color: AppColors.accent),
              ],
            ),
          ),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              flex: 2,
              child: DropdownButtonFormField<String>(
                value: _filterArus,
                decoration: const InputDecoration(labelText: 'Arus Kas', isDense: true,
                    contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 8), border: OutlineInputBorder()),
                items: const [
                  DropdownMenuItem(value: 'all', child: Text('Semua Arus Kas')),
                  DropdownMenuItem(value: 'income', child: Text('Pemasukan (+)')),
                  DropdownMenuItem(value: 'expense', child: Text('Pengeluaran (-)')),
                ],
                onChanged: (v) { if (v != null) setState(() => _filterArus = v); },
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: ElevatedButton(
                onPressed: () { setState(() => _isAllPeriods = !_isAllPeriods); _fetchData(); },
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.accent.withValues(alpha: 0.1), elevation: 0,
                  side: BorderSide(color: AppColors.accent.withValues(alpha: 0.5))),
                child: Text(_isAllPeriods ? 'Tutup Semua' : 'Semua Periode',
                    style: const TextStyle(color: AppColors.accent, fontSize: 12)),
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        TextField(
          decoration: const InputDecoration(
            hintText: 'Cari keterangan, RT, lokasi...', prefixIcon: Icon(Icons.search, size: 18),
            isDense: true, contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 8), border: OutlineInputBorder()),
          onChanged: (v) => setState(() => _searchQuery = v),
        ),
      ],
    );
  }

  Future<void> _showMonthPicker() async {
    final now = DateTime.now();
    final pickedYear = await showDialog<int>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Pilih Tahun'),
        content: SizedBox(
          width: double.maxFinite,
          child: GridView.builder(
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              crossAxisSpacing: 8,
              mainAxisSpacing: 8,
            ),
            itemCount: 5,
            itemBuilder: (ctx, index) {
              final year = now.year - 2 + index;
              final isSelected = year == _selectedYear;
              return ElevatedButton(
                onPressed: () => Navigator.pop(ctx, year),
                style: ElevatedButton.styleFrom(
                  backgroundColor: isSelected ? AppColors.accent : Colors.grey.shade200,
                  foregroundColor: isSelected ? Colors.white : Colors.black87,
                  elevation: 0,
                  side: BorderSide(color: isSelected ? AppColors.accent : Colors.grey.shade300),
                ),
                child: Text('$year'),
              );
            },
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, null),
            child: const Text('Batal'),
          ),
        ],
      ),
    );
    if (pickedYear == null || !mounted) return;

    final pickedMonth = await showDialog<int>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Pilih Bulan — $pickedYear'),
        content: SizedBox(
          width: double.maxFinite,
          child: GridView.builder(
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 3,
              crossAxisSpacing: 8,
              mainAxisSpacing: 8,
            ),
            itemCount: 12,
            itemBuilder: (ctx, index) {
              final month = index + 1;
              final isSelected = month == _selectedMonth;
              final shortName = _monthName(month).length > 4
                  ? _monthName(month).substring(0, 3)
                  : _monthName(month);
              return ElevatedButton(
                onPressed: () => Navigator.pop(ctx, month),
                style: ElevatedButton.styleFrom(
                  backgroundColor: isSelected ? AppColors.accent : Colors.grey.shade200,
                  foregroundColor: isSelected ? Colors.white : Colors.black87,
                  elevation: 0,
                  side: BorderSide(color: isSelected ? AppColors.accent : Colors.grey.shade300),
                ),
                child: Text(shortName),
              );
            },
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, null),
            child: const Text('Batal'),
          ),
        ],
      ),
    );
    if (pickedMonth == null || !mounted) return;

    setState(() {
      _selectedYear = pickedYear;
      _selectedMonth = pickedMonth;
      _isAllPeriods = false;
    });
    _fetchData();
  }

  Widget _buildErrorBanner() {
    return Container(
      margin: const EdgeInsets.only(bottom: 12), padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: AppColors.danger.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(AppRadius.sm),
          border: Border.all(color: AppColors.danger.withValues(alpha: 0.3))),
      child: Text('Error: $_error', style: const TextStyle(color: AppColors.danger, fontSize: 13)),
    );
  }

  Widget _buildTransactionsTable() {
    final txs = _filteredTransactions;
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minWidth: 750),
        child: DataTable(
          headingRowColor: WidgetStateProperty.all(AppColors.lightTextMuted.withValues(alpha: 0.05)),
          columns: [
            DataColumn(label: Text('TANGGAL', style: TextStyle(fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(label: Text('SUMBER / TUJUAN', style: TextStyle(fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(label: Text('LOKASI DANA', style: TextStyle(fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(label: Text('KETERANGAN', style: TextStyle(fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(label: Text('ARUS', style: TextStyle(fontSize: 10, color: AppColors.textMutedColor(context)))),
            DataColumn(label: Text('PEMASUKAN (+)', style: const TextStyle(fontSize: 10, color: AppColors.success))),
            DataColumn(label: Text('PENGELUARAN (-)', style: const TextStyle(fontSize: 10, color: AppColors.danger))),
            DataColumn(label: Text('AKSI', style: TextStyle(fontSize: 10, color: AppColors.textMutedColor(context)))),
          ],
          rows: txs.map((t) {
            final isIncome = t.type == 'income';
            final sourceDisplay = t.sourceRtName?.isNotEmpty == true
                ? 'RT ${t.sourceRtName}'
                : (t.rtName.isNotEmpty ? t.rtName : 'Kas RW');

            return DataRow(cells: [
              DataCell(Text(_formatDate(t.occurredAt), style: TextStyle(fontSize: 12, color: AppColors.textMutedColor(context)))),
              DataCell(Text(
                sourceDisplay,
                style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500),
              )),
              DataCell(Text(t.locationName ?? '—', style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500))),
              DataCell(
                InkWell(
                  onTap: () => _showTransactionDetailDialog(context, t),
                  child: Text(
                    t.description,
                    style: const TextStyle(fontSize: 13, color: AppColors.accent, fontWeight: FontWeight.w500),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
              DataCell(AppBadge(label: isIncome ? '↓ Masuk' : '↑ Keluar', backgroundColor: isIncome ? AppColors.success : AppColors.danger)),
              DataCell(Text(isIncome ? RupiahFormatter.format(RupiahFormatter.parseMoneyString(t.amount)) : '—',
                  style: TextStyle(fontSize: 13, color: isIncome ? AppColors.success : AppColors.lightTextMuted,
                      fontWeight: isIncome ? FontWeight.w600 : FontWeight.normal))),
              DataCell(Text(!isIncome ? RupiahFormatter.format(RupiahFormatter.parseMoneyString(t.amount)) : '—',
                  style: TextStyle(fontSize: 13, color: !isIncome ? AppColors.danger : AppColors.lightTextMuted,
                      fontWeight: !isIncome ? FontWeight.w600 : FontWeight.normal))),
              DataCell(
                IconButton(
                  icon: const Icon(Icons.info_outline, size: 18),
                  tooltip: 'Detail Transaksi',
                  onPressed: () => _showTransactionDetailDialog(context, t),
                ),
              ),
            ]);
          }).toList(),
        ),
      ),
    );
  }

  void _showTransactionDetailDialog(BuildContext context, RwTransaction t) {
    final isIncome = t.type == 'income';
    final amt = RupiahFormatter.parseMoneyString(t.amount);
    final sourceDisplay = t.sourceRtName?.isNotEmpty == true
        ? 'RT ${t.sourceRtName}'
        : (t.rtName.isNotEmpty ? t.rtName : 'Kas RW');

    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(isIncome ? 'Detail KAS Masuk RW' : 'Detail KAS Keluar RW'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildDetailRow('Tanggal:', _formatDate(t.occurredAt)),
            _buildDetailRow('Jenis Transaksi:', isIncome ? 'Pemasukan (KAS Masuk)' : 'Pengeluaran (KAS Keluar)'),
            _buildDetailRow('Sumber / Tujuan:', sourceDisplay),
            _buildDetailRow('Lokasi Dana:', t.locationName ?? '—'),
            _buildDetailRow('Metode Pembayaran:', (t.paymentMethod ?? '—').toUpperCase()),
            _buildDetailRow('Nominal:', RupiahFormatter.format(amt),
                color: isIncome ? AppColors.success : AppColors.danger),
            const SizedBox(height: 8),
            const Text('Keterangan:', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600)),
            const SizedBox(height: 2),
            Text(t.description, style: const TextStyle(fontSize: 13)),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Tutup'),
          ),
        ],
      ),
    );
  }

  Widget _buildDetailRow(String label, String value, {Color? color}) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: const TextStyle(fontSize: 12, color: AppColors.lightTextMuted)),
          Text(
            value,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: color,
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _showCreateTransactionDialog(BuildContext context, String type) async {
    final isIncome = type == 'income';
    final amountCtrl = TextEditingController();
    final descCtrl = TextEditingController();
    String? selectedLocationId;
    String paymentMethod = 'transfer';
    bool saving = false;
    String? error;
    List<RwFundLocation> locations = [];
    bool loadingLocations = true;

    try {
      locations = await ref.read(rwApiClientProvider).getLocations(widget.rwId, isActive: true);
      loadingLocations = false;
      if (locations.isNotEmpty) {
        selectedLocationId = locations.first.id;
        paymentMethod = locations.first.category == 'rekening' ? 'transfer' : 'cash';
      }
    } catch (_) {
      loadingLocations = false;
    }

    if (!context.mounted) return;

    await showDialog<void>(
      context: context,
      builder: (dialogCtx) {
        return StatefulBuilder(
          builder: (statefulCtx, setDialogState) {
            final selectedLoc = locations.where((l) => l.id == selectedLocationId).firstOrNull;
            final locBal = selectedLoc != null ? (double.tryParse(selectedLoc.currentBalance) ?? 0) : 0;
            final isBank = selectedLoc?.category == 'rekening';

            return AlertDialog(
              title: Text(isIncome ? 'Catat KAS Masuk RW' : 'Catat KAS Keluar RW'),
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
                    if (loadingLocations)
                      const Padding(
                        padding: EdgeInsets.all(16),
                        child: CircularProgressIndicator(),
                      )
                    else if (locations.isEmpty)
                      const Text(
                        'Tidak ada lokasi dana aktif. Tambahkan lokasi dana terlebih dahulu.',
                        style: TextStyle(color: AppColors.danger, fontSize: 12),
                      )
                    else ...[
                      DropdownButtonFormField<String>(
                        value: selectedLocationId,
                        decoration: InputDecoration(
                          labelText: isIncome ? 'Lokasi Dana Tujuan *' : 'Lokasi Dana Sumber *',
                        ),
                        items: locations.map((loc) {
                          final bal = double.tryParse(loc.currentBalance) ?? 0;
                          return DropdownMenuItem(
                            value: loc.id,
                            child: Text('${loc.name} (${RupiahFormatter.format(bal.toInt())})'),
                          );
                        }).toList(),
                        onChanged: (val) {
                          setDialogState(() {
                            selectedLocationId = val;
                            final loc = locations.where((l) => l.id == val).firstOrNull;
                            if (loc != null) {
                              paymentMethod = loc.category == 'rekening' ? 'transfer' : 'cash';
                            }
                          });
                        },
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: amountCtrl,
                        keyboardType: TextInputType.number,
                        decoration: InputDecoration(
                          labelText: 'Nominal (Rp) *',
                          helperText: !isIncome && selectedLoc != null
                              ? 'Saldo tersedia: ${RupiahFormatter.format(locBal.toInt())}'
                              : null,
                        ),
                      ),
                      const SizedBox(height: 8),
                      DropdownButtonFormField<String>(
                        value: paymentMethod,
                        decoration: InputDecoration(
                          labelText: 'Metode Pembayaran *',
                          helperText: isBank
                              ? 'Otomatis Transfer Bank untuk kategori Rekening'
                              : 'Otomatis Tunai (Cash) untuk kategori Brankas/Kas',
                        ),
                        items: isBank
                            ? const [
                                DropdownMenuItem(value: 'transfer', child: Text('Transfer Bank')),
                              ]
                            : const [
                                DropdownMenuItem(value: 'cash', child: Text('Tunai (Cash)')),
                              ],
                        onChanged: (val) {
                          if (val != null) setDialogState(() => paymentMethod = val);
                        },
                      ),
                      const SizedBox(height: 8),
                      TextField(
                        controller: descCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Keterangan *',
                          hintText: 'Contoh: Iuran RW dari RT 01, Pembelian perlengkapan...',
                        ),
                        maxLines: 2,
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
                  onPressed: saving || locations.isEmpty ? null : () async {
                    if (selectedLocationId == null) {
                      setDialogState(() => error = 'Pilih lokasi dana');
                      return;
                    }
                    final amt = double.tryParse(amountCtrl.text.replaceAll(RegExp(r'[^0-9]'), '')) ?? 0;
                    if (amt <= 0) {
                      setDialogState(() => error = 'Nominal harus lebih dari 0');
                      return;
                    }
                    if (!isIncome && amt > locBal) {
                      setDialogState(() => error = 'Saldo lokasi sumber tidak mencukupi');
                      return;
                    }
                    if (descCtrl.text.trim().isEmpty) {
                      setDialogState(() => error = 'Keterangan transaksi wajib diisi');
                      return;
                    }

                    setDialogState(() {
                      saving = true;
                      error = null;
                    });

                    try {
                      final api = ref.read(rwApiClientProvider);
                      await api.createTransaction(widget.rwId, {
                        'type': type,
                        'amount': '${amt.toInt()}.00',
                        'location_id': selectedLocationId,
                        'description': descCtrl.text.trim(),
                        'payment_method': paymentMethod,
                      });
                      if (context.mounted) {
                        Navigator.pop(dialogCtx);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('Transaksi KAS ${isIncome ? 'Masuk' : 'Keluar'} RW berhasil disimpan')),
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
                  style: ElevatedButton.styleFrom(
                    backgroundColor: isIncome ? AppColors.success : AppColors.danger,
                    foregroundColor: Colors.white,
                  ),
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

  String _monthName(int m) {
    const names = ['', 'Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni',
        'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'];
    return names[m];
  }

  String _formatDate(DateTime dt) {
    return '${dt.day.toString().padLeft(2, '0')} ${_monthName(dt.month)} ${dt.year}';
  }
}

class _EmptyBanner extends StatelessWidget {
  const _EmptyBanner();
  @override
  Widget build(BuildContext context) {
    return const Center(child: Padding(
      padding: EdgeInsets.symmetric(vertical: 32),
      child: Text('Belum ada transaksi pada periode ini.',
          style: TextStyle(color: AppColors.lightTextMuted, fontSize: 13))));
  }
}

class _LoadingBanner extends StatelessWidget {
  const _LoadingBanner();
  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: 12), padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(color: AppColors.lightTextMuted.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(AppRadius.sm)),
      child: const Text('Memuat data KAS RW...',
          style: TextStyle(color: AppColors.lightTextMuted, fontSize: 13)));
  }
}
