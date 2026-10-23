/// Laporan KAS (Android) — dokumen keuangan formal mengikuti gambar referensi.
///
/// Data: GET /api/v1/reports/balance + SEMUA halaman GET /api/v1/transactions
/// (status=posted) lewat kasRepositoryProvider. Tanpa data mock.

library;

import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show rootBundle;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:printing/printing.dart';
import 'package:share_plus/share_plus.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/export/kas_export_utils.dart';
import 'package:social_finance/features/kas/export/kas_report_model.dart';

// ---------------------------------------------------------------------------
// Akses (sama dengan Web)
// ---------------------------------------------------------------------------

class KasReportAccess {
  final bool canView;
  final bool canExport;
  const KasReportAccess({required this.canView, required this.canExport});

  factory KasReportAccess.from({
    String? systemRole,
    String? jabatan,
    String? role,
  }) {
    final j = (jabatan ?? '').toLowerCase().trim();
    final r = (role ?? '').toLowerCase().trim();
    final blocked = j == 'keamanan' || j == 'operasional';
    return KasReportAccess(
      canView: systemRole == 'super_admin' || !blocked,
      canExport: j == 'ketua' || j == 'wakil_ketua' || r == 'bendahara',
    );
  }
}

// ---------------------------------------------------------------------------
// Layar Laporan
// ---------------------------------------------------------------------------

class KasReportScreen extends ConsumerStatefulWidget {
  const KasReportScreen({super.key, this.period});

  /// Periode YYYY-MM; null = semua periode.
  final String? period;

  @override
  ConsumerState<KasReportScreen> createState() => _KasReportScreenState();
}

class _KasReportScreenState extends ConsumerState<KasReportScreen> {
  bool _loading = true;
  String? _error;
  KasReportModel? _model;
  List<BackendTransaction> _transactions = const [];
  bool _exporting = false;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  KasReportIdentity _identity({BackendRTIdentity? rtDetail}) {
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    final rt = user?['rt']?.toString();
    final rw = user?['rw']?.toString();
    final rtName = (rtDetail?.name ?? user?['rt_name'])?.toString().trim();
    final rtAddress = (rtDetail?.address ?? user?['rt_address'])
        ?.toString()
        .trim();
    final rwName = (rtDetail?.rwName ?? user?['rw_name'])?.toString().trim();
    final rtCode = rtDetail?.rt ?? rt;
    final rwCode = rtDetail?.rw?.toString() ?? rw;
    return KasReportIdentity(
      orgName: rtName != null && rtName.isNotEmpty ? rtName : '',
      rtLabel: rtCode != null && rtCode.isNotEmpty ? 'RT $rtCode' : null,
      rwLabel: rwName != null && rwName.isNotEmpty
          ? rwName
          : (rwCode != null && rwCode.isNotEmpty ? 'RW $rwCode' : null),
      address: rtAddress != null && rtAddress.isNotEmpty ? rtAddress : null,
    );
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final repository = ref.read(kasRepositoryProvider);
    try {
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final rtId = user?['rt_id']?.toString();

      final txs = await repository.fetchAllBackendTransactions(
        status: 'posted',
        period: widget.period,
      );
      final balance = await repository.fetchBalance(period: widget.period);

      BackendRTIdentity? rtDetail;
      if (rtId != null && rtId.isNotEmpty) {
        try {
          rtDetail = await repository.fetchRTIdentity(rtId);
        } catch (_) {
          // Silently ignore; the report uses fallback identity from user.
        }
      }
      if (!mounted) return;
      final model = buildKasReport(
        transactions: txs,
        openingBalance: balance.openingBalance,
        apiClosingBalance: balance.closingBalance,
        period: widget.period,
        identity: _identity(rtDetail: rtDetail),
      );
      setState(() {
        _transactions = txs;
        _model = model;
        _loading = false;
      });
    } catch (e, st) {
      debugPrint('KasReportScreen load failed: $e\n$st');
      if (!mounted) return;
      setState(() {
        _model = null;
        _loading = false;
        _error = e is FormatException
            ? 'Data laporan tidak dapat dibaca: ${e.message}'
            : repository.mapDioError(e).message;
      });
    }
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
      await Printing.sharePdf(
        bytes: bytes,
        filename: getKasReportPdfFileName(model),
      );
    } catch (e, st) {
      debugPrint('Export PDF gagal: $e\n$st');
      _toast('Gagal mengekspor PDF: $e');
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
      final dir = await getTemporaryDirectory();
      final name = getKasReportXlsxFileName(model);
      final file = File('${dir.path}/$name');
      await file.writeAsBytes(bytes, flush: true);
      await Share.shareXFiles([XFile(file.path)], subject: name);
    } catch (e, st) {
      debugPrint('Export XLSX gagal: $e\n$st');
      _toast('Gagal mengekspor XLSX: $e');
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  void _toast(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: Colors.red),
    );
  }

  @override
  Widget build(BuildContext context) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final access = KasReportAccess.from(
      systemRole: user?['system_role'] as String?,
      jabatan: user?['jabatan'] as String?,
      role: user?['role']?.toString().split('.').last,
    );

    return Scaffold(
      appBar: AppBar(
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Laporan Kas'),
      ),
      body: SafeArea(
        child: RefreshIndicator(
          onRefresh: _load,
          child: _buildBody(context, access),
        ),
      ),
    );
  }

  Widget _buildBody(BuildContext context, KasReportAccess access) {
    final theme = Theme.of(context);
    if (!access.canView) {
      return _centerMessage(
        theme,
        icon: Icons.lock_outline,
        title: 'Akses Ditolak',
        message: 'Anda tidak memiliki izin untuk mengakses Laporan KAS.',
        action: ElevatedButton.icon(
          onPressed: () => Navigator.of(context).maybePop(),
          icon: const Icon(Icons.arrow_back),
          label: const Text('Kembali'),
        ),
      );
    }
    if (_loading) {
      return const Center(
        key: Key('kas-report-loading'),
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
          // ---- Toolbar ekspor ----
          if (access.canExport)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Row(
                children: [
                  Expanded(
                    child: _simpleExportButton(
                      key: const Key('kas-report-export-pdf'),
                      label: _exporting ? 'Membuat PDF…' : 'PDF',
                      icon: Icons.picture_as_pdf,
                      color: AppColors.danger,
                      onPressed: _exporting ? null : _exportPdf,
                      isLoading: _exporting,
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: _simpleExportButton(
                      key: const Key('kas-report-export-xlsx'),
                      label: _exporting ? 'Membuat XLSX…' : 'XLSX',
                      icon: Icons.table_chart,
                      color: AppColors.success,
                      onPressed: _exporting ? null : _exportXlsx,
                      isLoading: _exporting,
                    ),
                  ),
                ],
              ),
            ),
          const SizedBox(height: 12),
          // ---- Peringatan selisih saldo ----
          if (model.reconciles == false)
            Container(
              key: const Key('kas-report-mismatch'),
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

  /// Lembar dokumen Laporan KAS — selalu berlatar putih bersih,
  /// tidak mewarisi tema gelap dari Scaffold.  Dokumen dibungkus
  /// dalam SingleChildScrollView agar tidak terjadi conflict
  /// scroll antara halaman utama (vertikal) dan tabel (horizontal).
  Widget _buildDocumentPage(BuildContext context, KasReportModel model) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Container(
        decoration: const BoxDecoration(color: Color(0xFFFFFFFF)),
        constraints: const BoxConstraints(minWidth: 320, maxWidth: 900),
        margin: const EdgeInsets.symmetric(horizontal: 16),
        child: KasReportDocument(model: model),
      ),
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

  // ------------------------------------------------------------------
  // Simple export button (tanpa neon / shadow)
  // ------------------------------------------------------------------

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
      color: Colors.white,
      borderRadius: BorderRadius.circular(8),
      elevation: 1,
      child: InkWell(
        onTap: isLoading ? null : onPressed,
        borderRadius: BorderRadius.circular(8),
        child: Container(
          height: 40,
          decoration: BoxDecoration(
            border: Border.all(color: color.withValues(alpha: 0.35)),
            borderRadius: BorderRadius.circular(8),
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

// ---------------------------------------------------------------------------
// Dokumen laporan — formal, mengikuti gambar referensi
// Dokumen selalu putih, tabel dengan lebar minimum untuk proporsi benar
// ---------------------------------------------------------------------------

class KasReportDocument extends StatelessWidget {
  const KasReportDocument({super.key, required this.model});
  final KasReportModel model;

  @override
  Widget build(BuildContext context) {
    final rows = model.statementRows();
    final generatedAt = model.generatedAt;
    final hasOther = model.expenseHasOther;

    return Container(
      key: const Key('kas-report-document'),
      decoration: const BoxDecoration(color: Color(0xFFFFFFFF)),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ---- Header ----
          _buildHeader(),

          const Divider(height: 24, thickness: 0.5, color: Color(0xFFDDDDDD)),

          // ---- Tabel transaksi ----
          if (model.transactionCount == 0)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 24),
              child: Center(
                child: Text(
                  'Belum ada transaksi KAS pada ${model.periodLabel}.',
                  key: const Key('kas-report-empty'),
                  style: const TextStyle(
                    fontSize: 11,
                    color: Color(0xFF666666),
                  ),
                ),
              ),
            )
          else
            _buildStatementTable(rows),

          // ---- Rekap Pengeluaran per Kategori ----
          const SizedBox(height: 28),
          _buildRecapSection(hasOther),

          // ---- Footer ----
          const SizedBox(height: 16),
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Divider(thickness: 0.5, color: const Color(0xFFE0E0E0)),
          ),
          Padding(
            padding: const EdgeInsets.only(top: 12),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.start,
              children: [
                Text(
                  'Dicetak: ${_formatDate(generatedAt)}',
                  style: const TextStyle(fontSize: 9, color: Color(0xFF888888)),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // ------------------------------------------------------------------
  // Header: logo kiri, judul kiri-aligned mengikuti referensi
  // ------------------------------------------------------------------

  Widget _buildHeader() {
    final lines = kasTitleLines(model);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Image.asset(
          'assets/images/logo.png',
          width: 44,
          height: 44,
          errorBuilder: (context, error, stackTrace) => SizedBox(width: 44, height: 44),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              for (var i = 0; i < lines.length; i++)
                Padding(
                  padding: const EdgeInsets.only(bottom: 1),
                  child: Text(
                    lines[i],
                    style: TextStyle(
                      fontSize: i == 0 ? 14 : 11,
                      fontWeight: i == 0 ? FontWeight.bold : FontWeight.w600,
                      color: const Color(0xFF222222),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }

  // ------------------------------------------------------------------
  // Tabel transaksi utama — semua lebar kolom eksplisit.
  // ConstrainedBox(minWidth) menjamin FlexColumnWidth mendapat ruang
  // yang cukup sebelum SingleChildScrollView( Axis.horizontal )
  // mengizinkan scroll horizontal.
  // ------------------------------------------------------------------

  Widget _buildStatementTable(List<KasStatementRow> rows) {
    return ConstrainedBox(
      constraints: const BoxConstraints(minWidth: 700),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Table(
          key: const Key('kas-report-statement'),
          border: TableBorder.all(color: const Color(0xFFCCCCCC), width: 0.5),
          columnWidths: const {
            0: FixedColumnWidth(30), // No
            1: FlexColumnWidth(), // Uraian — dominan
            2: FixedColumnWidth(100), // Debet
            3: FixedColumnWidth(100), // Kredit
            4: FixedColumnWidth(110), // Saldo
          },
          defaultVerticalAlignment: TableCellVerticalAlignment.middle,
          children: [
            // Header kolom
            _tableRow(
              children: [
                _tc('No', right: true, bold: true, size: 10),
                _tc('Uraian', bold: true, size: 10),
                _tc('Debet', right: true, bold: true, size: 10),
                _tc('Kredit', right: true, bold: true, size: 10),
                _tc('Saldo', right: true, bold: true, size: 10),
              ],
              header: true,
            ),
            // Baris data
            for (final r in rows) _statementRow(r),
          ],
        ),
      ),
    );
  }

  TableRow _tableRow({
    required List<Widget> children,
    bool header = false,
    Color? bg,
  }) {
    return TableRow(
      decoration: BoxDecoration(
        color: header
            ? const Color(0xFFEFEFEF)
            : (bg ?? const Color(0xFFFFFFFF)),
      ),
      children: children,
    );
  }

  TableRow _statementRow(KasStatementRow r) {
    Color? bg;
    final isBold = r.kind != KasRowKind.row;

    if (r.kind == KasRowKind.grand) {
      bg = const Color(0xFFE8EEF7);
    } else if (r.kind == KasRowKind.total) {
      bg = const Color(0xFFF8F8F8);
    } else if (r.kind == KasRowKind.subtotal) {
      bg = const Color(0xFFFAFAFA);
    }

    return _tableRow(
      bg: bg,
      children: [
        _tc(r.no?.toString() ?? '', right: true, bold: isBold, size: 10),
        _tc(r.label, bold: isBold, size: 10),
        _tc(
          r.debit != null && r.debit! != 0 ? formatRp(r.debit!) : '',
          right: true,
          bold: isBold,
          size: 10,
        ),
        _tc(
          r.credit != null && r.credit! != 0 ? formatRp(r.credit!) : '',
          right: true,
          bold: isBold,
          size: 10,
        ),
        _tc(
          r.saldo != null ? formatRp(r.saldo!) : '',
          right: true,
          bold: isBold,
          size: r.kind == KasRowKind.grand ? 11 : 10,
        ),
      ],
    );
  }

  Widget _tc(
    String text, {
    bool right = false,
    bool bold = false,
    double size = 10,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 4),
      child: Text(
        text,
        textAlign: right ? TextAlign.right : TextAlign.left,
        style:
            const TextStyle(
              fontSize: 10,
              color: Color(0xFF222222),
              height: 1.3,
            ).copyWith(
              fontWeight: bold ? FontWeight.bold : FontWeight.normal,
              fontSize: size,
            ),
      ),
    );
  }

  // ------------------------------------------------------------------
  // Rekap pengeluaran per kategori
  // ------------------------------------------------------------------

  Widget _buildRecapSection(bool hasOther) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          'REKAP PENGELUARAN PER KATEGORI',
          textAlign: TextAlign.center,
          style: const TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: Color(0xFF222222),
          ),
        ),
        Text(
          model.period != null
              ? 'Periode ${model.periodLabel}'
              : 'Semua Periode',
          textAlign: TextAlign.center,
          style: const TextStyle(fontSize: 10, color: Color(0xFF666666)),
        ),
        const SizedBox(height: 8),
        _buildRecapTable(hasOther),
      ],
    );
  }

  Widget _buildRecapTable(bool hasOther) {
    return ConstrainedBox(
      constraints: const BoxConstraints(minWidth: 450),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Table(
          key: const Key('kas-report-recap'),
          border: TableBorder.all(color: const Color(0xFFCCCCCC), width: 0.5),
          columnWidths: {
            0: FlexColumnWidth(),
            1: FixedColumnWidth(100),
            2: FixedColumnWidth(100),
            if (hasOther) 3: FixedColumnWidth(100),
          },
          defaultVerticalAlignment: TableCellVerticalAlignment.middle,
          children: [
            // Header
            _tableRow(
              children: [
                _tc('Kategori', bold: true, size: 10),
                _tc('Tunai', right: true, bold: true, size: 10),
                _tc('Transfer', right: true, bold: true, size: 10),
                if (hasOther) _tc('Lainnya', right: true, bold: true, size: 10),
              ],
              header: true,
              bg: const Color(0xFFD6E8F7),
            ),
            // Baris kategori
            for (final c in model.expenseCategories)
              _tableRow(
                children: [
                  _tc(c.category, size: 10),
                  _tc(formatNum(c.cash), right: true, size: 10),
                  _tc(formatNum(c.transfer), right: true, size: 10),
                  if (hasOther) _tc(formatNum(c.other), right: true, size: 10),
                ],
              ),
            // Grand Total
            _tableRow(
              children: [
                _tc('Total', bold: true, size: 10),
                _tc(
                  formatNum(model.expense.cash),
                  right: true,
                  bold: true,
                  size: 10,
                ),
                _tc(
                  formatNum(model.expense.transfer),
                  right: true,
                  bold: true,
                  size: 10,
                ),
                if (hasOther)
                  _tc(
                    formatNum(model.expense.other),
                    right: true,
                    bold: true,
                    size: 10,
                  ),
              ],
              bg: const Color(0xFFD6E8F7),
            ),
          ],
        ),
      ),
    );
  }

  String _formatDate(DateTime dt) {
    const months = [
      'Januari',
      'Februari',
      'Maret',
      'April',
      'Mei',
      'Juni',
      'Juli',
      'Agustus',
      'September',
      'Oktober',
      'November',
      'Desember',
    ];
    return '${dt.day} ${months[dt.month - 1]} ${dt.year}';
  }
}
