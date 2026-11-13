/// Laporan KAS RW (Android) — dokumen keuangan formal, IDENTIK dengan
/// Laporan KAS RT (`KasReportScreen`).
///
/// Perbedaan terhadap Laporan KAS RT — dan hanya perbedaan ini:
///   1. Sumber data: endpoint RW (`/api/v1/rw/{rwId}/kas/report` dan
///      `/api/v1/rw/{rwId}/kas/transactions`), BUKAN `/api/v1/transactions`
///      atau `/api/v1/reports/balance` milik RT.
///   2. Saldo awal: `rw_opening_balances` + net transaksi posted sebelum
///      periode (carry-forward), dihitung server oleh endpoint RW.
///   3. Identitas laporan: RW (RW number/name dari membership), bukan RT.
///   4. Hak akses: permission KAS RW (membership RW untuk rwId tsb), bukan
///      jabatan RT.
///
/// Komponen yang digunakan kembali secara langsung (tanpa perubahan):
///   - KasReportDocument (widget dokumen putih)
///   - buildKasReport / KasReportModel / KasReportIdentity / KasStatementRow
///   - kas_export_utils (generateKasReportPdf/Xlsx, file naming)
///   - BackendTransaction sebagai kontrak data bersama

library;

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show rootBundle;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_saver/file_saver.dart';
import 'package:printing/printing.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/export/kas_export_utils.dart';
import 'package:social_finance/features/kas/export/kas_report_model.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_report_screen.dart'
    show KasReportDocument;
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/data/rw_models.dart' as rw_models show RwTransaction;

// ---------------------------------------------------------------------------
// Akses (permission KAS RW — sama dengan Web RW)
// ---------------------------------------------------------------------------

class RwKasReportAccess {
  final bool canView;
  final bool canExportPdf;
  final bool canExportXlsx;
  final bool canPrint;

  bool get canExport => canExportPdf || canExportXlsx;

  const RwKasReportAccess({
    required this.canView,
    required this.canExportPdf,
    required this.canExportXlsx,
    required this.canPrint,
  });

  /// canView  : super_admin atau membership RW untuk rwId ini (read-only
  ///            member termasuk anggota biasa).
  /// privileged: super_admin atau jabatan ketua / wakil_ketua / sekretaris /
  ///            bendahara (paritas dengan daftar privileged RT).
  factory RwKasReportAccess.from({
    String? systemRole,
    List<Map<String, dynamic>>? rwMemberships,
    required String rwId,
  }) {
    final isSuperAdmin = systemRole == 'super_admin';
    final memberships = (rwMemberships ?? const <Map<String, dynamic>>[])
        .whereType<Map<String, dynamic>>()
        .where((m) => (m['rw_id'] ?? m['rwId']) == rwId)
        .toList();
    final canView = isSuperAdmin || memberships.isNotEmpty;

    final privileged = isSuperAdmin ||
        memberships.any((m) {
          final jabatan = (m['jabatan'] ?? '').toString().toLowerCase().trim();
          return jabatan == 'ketua' ||
              jabatan == 'wakil_ketua' ||
              jabatan == 'sekretaris' ||
              jabatan == 'bendahara';
        });

    return RwKasReportAccess(
      canView: canView,
      canExportPdf: canView,
      canExportXlsx: privileged,
      canPrint: privileged,
    );
  }
}

Map<String, dynamic>? resolveRwMembership(Map<String, dynamic>? user, String rwId) {
  final raw =
      (user?['rw_memberships'] as List?)?.whereType<Map<String, dynamic>>().toList() ??
          const <Map<String, dynamic>>[];
  for (final m in raw) {
    if ((m['rw_id'] ?? m['rwId']) == rwId) return m;
  }
  return null;
}

String rwNumberOf(Map<String, dynamic>? membership, String rwId, Map<String, dynamic>? user) {
  final raw = membership?['rw_number'] ?? membership?['rwNumber'] ?? user?['rw'];
  if (raw == null) return resolveRwNumber(rwId, user?['rw_memberships']) ?? rwId;
  final s = raw.toString().trim();
  final n = int.tryParse(s);
  return n == null ? s : n.toString().padLeft(3, '0');
}

String rwNameOf(Map<String, dynamic>? membership, Map<String, dynamic>? user, String fallback) {
  final raw = membership?['rw_name'] ?? membership?['rwName'] ?? user?['rw_name'];
  final s = raw?.toString().trim() ?? '';
  return s.isNotEmpty ? s : fallback;
}

BackendTransaction rwToBackendTransaction(rw_models.RwTransaction t) {
  return BackendTransaction(
    id: t.id,
    rtId: t.rtId,
    categoryId: t.categoryId ?? '',
    amount: t.amount,
    type: t.type,
    status: t.status,
    paymentId: t.paymentId,
    paymentNo: t.referenceNo,
    description: t.description,
    occurredAt: t.occurredAt.toUtc().toIso8601String(),
    createdAt: t.createdAt.toUtc().toIso8601String(),
    categoryName: t.categoryName,
    officerName: null,
    dueName: null,
    citizenName: null,
    houseNumber: null,
    paymentMethod: t.paymentMethod,
    expenseCategory: t.expenseCategory,
  );
}

String? rwLabelPeriod({String? period, String? startDate, String? endDate}) {
  if (period != null && period.isNotEmpty) return period;
  if (startDate != null &&
      endDate != null &&
      startDate.length == 10 &&
      endDate.length == 10) {
    final sy = startDate.substring(0, 4);
    final ey = endDate.substring(0, 4);
    if (sy == ey && startDate.endsWith('-01-01') && endDate.endsWith('-12-31')) {
      return sy;
    }
  }
  return null;
}

// ---------------------------------------------------------------------------
// Layar Laporan KAS RW
// ---------------------------------------------------------------------------

class RwKasReportScreen extends ConsumerStatefulWidget {
  const RwKasReportScreen({
    super.key,
    required this.rwId,
    this.period,
    this.startDate,
    this.endDate,
  });

  final String rwId;

  /// Periode YYYY-MM; null berarti semua periode.
  final String? period;

  /// Date range filter (inclusive both ends). Mutually exclusive with period.
  final String? startDate;
  final String? endDate;

  @override
  ConsumerState<RwKasReportScreen> createState() => _RwKasReportScreenState();
}

class _RwKasReportScreenState extends ConsumerState<RwKasReportScreen> {
  bool _loading = true;
  String? _error;
  KasReportModel? _model;
  List<BackendTransaction> _transactions = const [];
  bool _exporting = false;

  // Pinch-to-zoom
  double _zoom = 1.0;
  double _zoomAtScaleStart = 1.0;
  static const double _zoomMin = 0.7;
  static const double _zoomMax = 2.5;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  KasReportIdentity _identity(String rwLabel, String rwName) {
    return KasReportIdentity(
      orgName: rwName,
      rtLabel: null,
      rwLabel: rwLabel,
      address: null,
    );
  }

  Future<List<rw_models.RwTransaction>> _fetchAllTransactionsPosted(RwApiClient api) async {
    final all = <rw_models.RwTransaction>[];
    var page = 1;
    var totalPages = 1;
    do {
      final resp = await api.getTransactions(
        rwId: widget.rwId,
        period: (widget.period != null && widget.period!.isNotEmpty) ? widget.period : null,
        startDate: (widget.period != null && widget.period!.isNotEmpty) ? null : widget.startDate,
        endDate: (widget.period != null && widget.period!.isNotEmpty) ? null : widget.endDate,
        page: page,
        pageSize: 100,
      );
      all.addAll(resp.data);
      totalPages = resp.pagination.totalPages;
      page++;
    } while (page <= totalPages);
    return all;
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final api = ref.read(rwApiClientProvider);
    try {
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final membership = resolveRwMembership(user, widget.rwId);
      final rwNum = rwNumberOf(membership, widget.rwId, user);
      final rwLabel = rwNum.startsWith('RW') ? rwNum : 'RW $rwNum';
      final rwName = rwNameOf(membership, user, rwLabel);

      final periodParam = (widget.period != null && widget.period!.isNotEmpty)
          ? widget.period
          : null;
      final detail = await api.getKasReportDetail(
        widget.rwId,
        period: periodParam,
        startDate: periodParam == null ? widget.startDate : null,
        endDate: periodParam == null ? widget.endDate : null,
      );

      final rwTxs = await _fetchAllTransactionsPosted(api);
      final backendTxs = rwTxs.map(rwToBackendTransaction).toList();

      List<KasFundLocationItem> reportLocations = [];
      try {
        final locs = await api.getLocations(widget.rwId, isActive: true);
        reportLocations = locs
            .map(
              (l) => KasFundLocationItem(
                name: l.name,
                category: l.category,
                balanceCents: _moneyToCents(l.currentBalance),
              ),
            )
            .toList();
      } catch (_) {}

      if (!mounted) return;
      final model = buildKasReport(
        transactions: backendTxs,
        openingBalance: detail.openingBalance,
        apiClosingBalance: detail.totalBalance,
        period: rwLabelPeriod(
          period: periodParam,
          startDate: widget.startDate,
          endDate: widget.endDate,
        ),
        identity: _identity(rwLabel, rwName),
        fundLocations: reportLocations,
      );
      setState(() {
        _transactions = backendTxs;
        _model = model;
        _loading = false;
      });
    } catch (e, st) {
      debugPrint('RwKasReportScreen load failed: $e\n$st');
      if (!mounted) return;
      setState(() {
        _model = null;
        _loading = false;
        _error = e is FormatException
            ? 'Data laporan tidak dapat dibaca: ${e.message}'
            : e.toString();
      });
    }
  }

  int _moneyToCents(String s) {
    final v = s.trim();
    final negative = v.startsWith('-');
    final body = negative ? v.substring(1) : v;
    final parts = body.split('.');
    final whole = parts.isEmpty ? '0' : parts[0];
    final frac = parts.length > 1 ? parts[1] : '0';
    final fracPadded = frac.padRight(2, '0');
    final wholeInt = int.tryParse(whole.isEmpty ? '0' : whole) ?? 0;
    final fracInt = int.tryParse(fracPadded.length >= 2 ? fracPadded.substring(0, 2) : fracPadded.padRight(2, '0')) ?? 0;
    final cents = wholeInt * 100 + fracInt;
    return negative ? -cents : cents;
  }

  Future<Uint8List?> _logoBytes() async {
    try {
      final data = await rootBundle.load('assets/images/logo.png');
      return data.buffer.asUint8List();
    } catch (e) {
      debugPrint('Logo laporan tidak dapat dimuat: $e');
      return null;
    }
  }

  String _fileName(KasReportModel model, String extension) {
    final period = model.period;
    final suffix = period == null || period.isEmpty ? 'semua-periode' : period;
    return 'Laporan-KAS-RW-$suffix.$extension';
  }

  Future<void> _exportPdf() async {
    final model = _model;
    if (model == null || _exporting) return;
    setState(() => _exporting = true);
    try {
      final bytes = await generateKasReportPdf(
        model: model,
        transactions: _transactions,
        logoBytes: await _logoBytes(),
      );
      final fileName = _fileName(model, 'pdf');
      final saved = await FileSaver.instance.saveAs(
        name: fileName.replaceAll('.pdf', ''),
        bytes: bytes,
        mimeType: MimeType.pdf,
        fileExtension: 'pdf',
      );
      if (!mounted) return;
      if (saved != null) {
        _showToast('PDF berhasil disimpan:\n$fileName');
      } else {
        _showToast('Penyimpanan PDF dibatalkan atau gagal.');
      }
    } catch (e, st) {
      debugPrint('Export PDF gagal: $e\n$st');
      _showToast('Gagal mengekspor PDF: $e');
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  Future<void> _exportXlsx() async {
    final model = _model;
    if (model == null || _exporting) return;
    setState(() => _exporting = true);
    try {
      final bytes = await generateKasReportXlsx(
        model: model,
        transactions: _transactions,
      );
      final fileName = _fileName(model, 'xlsx');
      final saved = await FileSaver.instance.saveAs(
        name: fileName.replaceAll('.xlsx', ''),
        bytes: bytes,
        mimeType: MimeType.microsoftExcel,
        fileExtension: 'xlsx',
      );
      if (!mounted) return;
      if (saved != null) {
        _showToast('XLSX berhasil disimpan:\n$fileName');
      } else {
        _showToast('Penyimpanan XLSX dibatalkan atau gagal.');
      }
    } catch (e, st) {
      debugPrint('Export XLSX gagal: $e\n$st');
      _showToast('Gagal mengekspor XLSX: $e');
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  Future<void> _printReport() async {
    final model = _model;
    if (model == null || _exporting) return;
    try {
      final logo = await _logoBytes();
      await Printing.layoutPdf(
        onLayout: (format) async => await generateKasReportPdf(
          model: model,
          transactions: _transactions,
          logoBytes: logo,
        ),
        name: _fileName(model, 'pdf'),
      );
    } catch (e, st) {
      debugPrint('Print laporan kas gagal: $e\n$st');
      _showToast('Gagal mencetak laporan: $e');
    }
  }

  void _showToast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: Colors.red),
    );
  }

  @override
  Widget build(BuildContext context) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final memberships =
        (user?['rw_memberships'] as List?)?.whereType<Map<String, dynamic>>().toList() ??
            const <Map<String, dynamic>>[];
    final access = RwKasReportAccess.from(
      systemRole: user?['system_role'] as String?,
      rwMemberships: memberships,
      rwId: widget.rwId,
    );

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Laporan KAS RW'),
      ),
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: _load,
          child: _buildBody(context, access),
        ),
      ),
    );
  }

  Widget _buildBody(BuildContext context, RwKasReportAccess access) {
    final theme = Theme.of(context);
    if (!access.canView) {
      return _centerMessage(
        theme,
        icon: Icons.lock_outline,
        title: 'Akses Ditolak',
        message: 'Anda tidak memiliki izin untuk mengakses Laporan KAS RW ini.',
        action: ElevatedButton.icon(
          onPressed: () => Navigator.of(context).maybePop(),
          icon: const Icon(Icons.arrow_back),
          label: const Text('Kembali'),
        ),
      );
    }
    if (_loading) {
      return const Center(
        key: Key('rw-kas-report-loading'),
        child: CircularProgressIndicator(),
      );
    }
    if (_error != null) {
      return _centerMessage(
        theme,
        icon: Icons.error_outline,
        title: 'Gagal memuat laporan',
        message: _error!,
        action: ElevatedButton.icon(
          onPressed: _load,
          icon: const Icon(Icons.refresh),
          label: const Text('Coba Lagi'),
        ),
      );
    }
    final model = _model!;
    return SingleChildScrollView(
      padding: const EdgeInsets.only(top: 12),
      child: Column(
        children: [
          // ---- Toolbar ekspor & cetak ----
          if (access.canExportPdf || access.canExportXlsx || access.canPrint)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Row(
                children: [
                  if (access.canExportPdf)
                    Expanded(
                      child: _simpleExportButton(
                        key: const Key('rw-kas-report-export-pdf'),
                        label: _exporting ? 'Membuat PDF…' : 'PDF',
                        icon: Icons.picture_as_pdf,
                        color: AppColors.danger,
                        onPressed: _exporting ? null : _exportPdf,
                        isLoading: _exporting,
                      ),
                    ),
                  if (access.canExportPdf && (access.canExportXlsx || access.canPrint))
                    const SizedBox(width: 8),
                  if (access.canExportXlsx)
                    Expanded(
                      child: _simpleExportButton(
                        key: const Key('rw-kas-report-export-xlsx'),
                        label: _exporting ? 'Membuat XLSX…' : 'XLSX',
                        icon: Icons.table_chart,
                        color: AppColors.success,
                        onPressed: _exporting ? null : _exportXlsx,
                        isLoading: _exporting,
                      ),
                    ),
                  if (access.canExportXlsx && access.canPrint)
                    const SizedBox(width: 8),
                  if (access.canPrint)
                    Expanded(
                      child: _simpleExportButton(
                        key: const Key('rw-kas-report-export-print'),
                        label: 'Cetak',
                        icon: Icons.print,
                        color: AppColors.accent,
                        onPressed: _exporting ? null : _printReport,
                        isLoading: false,
                      ),
                    ),
                ],
              ),
            ),
          const SizedBox(height: 24),
          // ---- Peringatan selisih saldo ----
          if (model.reconciles == false)
            Container(
              key: const Key('rw-kas-report-mismatch'),
              margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              padding: const EdgeInsets.all(12),
              color: Colors.red.withValues(alpha: 0.1),
              child: Text(
                'Saldo akhir hasil hitung (${formatRp(model.closingCents)}) '
                'berbeda dari saldo akhir server '
                '(${formatRp(model.apiClosingCents ?? 0)}).',
                style: const TextStyle(color: Colors.red, fontSize: 12),
              ),
            ),
          // ---- Lembar dokumen putih ----
          _buildDocumentPage(context, model),
          const SizedBox(height: 16),
        ],
      ),
    );
  }

  Widget _buildDocumentPage(BuildContext context, KasReportModel model) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final viewportWidth = constraints.maxWidth;

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ---- Kontrol zoom ----
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Row(
                children: [
                  IconButton(
                    key: const Key('rw-kas-report-zoom-out'),
                    icon: const Text('-'),
                    onPressed: () {
                      final newZoom = (_zoom - 0.1).clamp(_zoomMin, _zoomMax);
                      if ((newZoom - _zoom).abs() > 0.001) {
                        setState(() => _zoom = newZoom);
                      }
                    },
                  ),
                  const SizedBox(width: 8),
                  Text(
                    '${(_zoom * 100).toInt()}%',
                    style: const TextStyle(fontSize: 12),
                  ),
                  const SizedBox(width: 8),
                  IconButton(
                    key: const Key('rw-kas-report-zoom-in'),
                    icon: const Text('+'),
                    onPressed: () {
                      final newZoom = (_zoom + 0.1).clamp(_zoomMin, _zoomMax);
                      if ((newZoom - _zoom).abs() > 0.001) {
                        setState(() => _zoom = newZoom);
                      }
                    },
                  ),
                  const SizedBox(width: 16),
                  const Text(
                    'Cubit untuk zoom',
                    style: TextStyle(fontSize: 11, color: Color(0xFF888888)),
                  ),
                ],
              ),
            ),
            // ---- Dokumen laporan ----
            SingleChildScrollView(
              child: GestureDetector(
                onScaleStart: (_) {
                  _zoomAtScaleStart = _zoom;
                },
                onScaleUpdate: (details) {
                  final newZoom =
                      (_zoomAtScaleStart * details.scale).clamp(_zoomMin, _zoomMax);
                  if ((newZoom - _zoom).abs() > 0.001) {
                    setState(() => _zoom = newZoom);
                  }
                },
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  physics: const BouncingScrollPhysics(),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(minWidth: viewportWidth),
                    child: Align(
                      alignment: Alignment.topCenter,
                      child: Container(
                        decoration: const BoxDecoration(color: Color(0xFFFFFFFF)),
                        child: KasReportDocument(
                          model: model,
                          zoom: _zoom,
                          viewportWidth: viewportWidth,
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _centerMessage(
    ThemeData theme, {
    required IconData icon,
    required String title,
    required String message,
    required Widget action,
  }) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 48, color: Colors.red),
            const SizedBox(height: 16),
            Text(
              title,
              style: theme.textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(height: 24),
            action,
          ],
        ),
      ),
    );
  }

  Widget _simpleExportButton({
    Key? key,
    required String label,
    required IconData icon,
    required Color color,
    required VoidCallback? onPressed,
    bool isLoading = false,
  }) {
    return Material(
      key: key,
      color: Colors.transparent,
      borderRadius: BorderRadius.circular(8),
      elevation: 0,
      child: InkWell(
        onTap: isLoading ? null : onPressed,
        borderRadius: BorderRadius.circular(8),
        highlightColor: color.withValues(alpha: 0.12),
        splashColor: color.withValues(alpha: 0.18),
        child: Container(
          height: 40,
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [
                color.withValues(alpha: 0.22),
                color.withValues(alpha: 0.08),
              ],
            ),
            border: Border.all(color: color.withValues(alpha: 0.45)),
            borderRadius: BorderRadius.circular(8),
            boxShadow: [
              BoxShadow(
                color: color.withValues(alpha: 0.18),
                blurRadius: 8,
                offset: const Offset(0, 2),
              ),
            ],
          ),
          alignment: Alignment.center,
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (isLoading)
                SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(
                    strokeWidth: 1.5,
                    valueColor: AlwaysStoppedAnimation<Color>(color),
                  ),
                )
              else ...[
                Icon(icon, size: 16, color: color),
                const SizedBox(width: 6),
              ],
              Flexible(
                child: Text(
                  label,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: color,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
