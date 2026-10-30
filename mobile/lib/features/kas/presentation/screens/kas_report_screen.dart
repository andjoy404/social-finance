/// Laporan KAS (Android) — dokumen keuangan formal mengikuti gambar referensi.
///
/// Data: GET /api/v1/reports/balance + SEMUA halaman GET /api/v1/transactions
/// (status=posted) lewat kasRepositoryProvider. Tanpa data mock.

library;

import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show rootBundle;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_saver/file_saver.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/export/kas_export_utils.dart';
import 'package:social_finance/features/kas/export/kas_report_model.dart';

// --------------------------------------------i-------------------------------
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
      canExport: j == 'ketua' ||
          j == 'wakil_ketua' ||
          j == 'bendahara' ||
          r == 'bendahara',
    );
  }
}

// ---------------------------------------------------------------------------
// Layar Laporan
// ---------------------------------------------------------------------------

class KasReportScreen extends ConsumerStatefulWidget {
  const KasReportScreen({super.key, this.period, this.startDate, this.endDate});

  /// Periode YYYY-MM; null = semua periode.
  final String? period;

  /// Date range filter for year view.
  final String? startDate;

  /// Date range filter for year view.
  final String? endDate;

  @override
  ConsumerState<KasReportScreen> createState() => _KasReportScreenState();
}

class _KasReportScreenState extends ConsumerState<KasReportScreen> {
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
        startDate: widget.startDate,
        endDate: widget.endDate,
      );
      final balance = await repository.fetchBalance(
        period: widget.period,
        startDate: widget.startDate,
        endDate: widget.endDate,
      );

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
      final fileName = getKasReportPdfFileName(model);
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
      final fileName = getKasReportXlsxFileName(model);
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

  void _showToast(String message) {
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
          const SizedBox(height: 24),
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
  /// tidak mewarisi tema gelap dari Scaffold.
  ///
  /// Layout responsif:
  ///   - Dokumen mengisi lebar area layar pada zoom 100%.
  ///   - Ketika konten melebihi viewport (zoom > 1.0), horizontal scrolling aktif.
  ///   - Pinch-to-zoom tersedia dengan batas 0.7 — 2.5.
  ///
  /// Rantai constraint:
  ///   LayoutBuilder → SingleChildScrollView(h) → Align(topCenter)
  ///     → Container → KasReportDocument → Column → _buildStatementTable
  ///       → ConstrainedBox(minWidth) → SingleChildScrollView(h) → Table
  ///
  /// [constraints.maxWidth] memberi lebar area yang tersedia tanpa batas atas
  /// agar dokumen memenuhi viewport. [Align(topCenter)] memusatkan dokumen
  /// secara horizontal ketika lebarnya lebih kecil daripada viewport.
  Widget _buildDocumentPage(BuildContext context, KasReportModel model) {
    return LayoutBuilder(
      builder: (context, constraints) {
        // Simpan lebar viewport sebelum masuk horizontal scroll.
        // constraints.maxWidth dari LayoutBuilder di luar horizontal scroll
        // adalah lebar viewport sebenarnya. Dari dalam horizontal scroll,
        // maxWidth akan infinity sehingga tidak bisa dipakai untuk menentukan
        // lebar dokumen.
        final viewportWidth = constraints.maxWidth;

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ---- Kontrol zoom ----
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Row(
                children: [
                  // Tombol minus
                  IconButton(
                    key: const Key('kas-report-zoom-out'),
                    icon: const Text('-'),
                    onPressed: () {
                      final newZoom = (_zoom - 0.1)
                          .clamp(_zoomMin, _zoomMax);
                      if ((newZoom - _zoom).abs() > 0.001) {
                        setState(() => _zoom = newZoom);
                      }
                    },
                  ),
                  const SizedBox(width: 8),
                  // Persentase zoom
                  Text(
                    '${(_zoom * 100).toInt()}%',
                    style: const TextStyle(fontSize: 12),
                  ),
                  const SizedBox(width: 8),
                  // Tombol plus
                  IconButton(
                    key: const Key('kas-report-zoom-in'),
                    icon: const Text('+'),
                    onPressed: () {
                      final newZoom = (_zoom + 0.1)
                          .clamp(_zoomMin, _zoomMax);
                      if ((newZoom - _zoom).abs() > 0.001) {
                        setState(() => _zoom = newZoom);
                      }
                    },
                  ),
                  const SizedBox(width: 16),
                  // Pinch-to-zoom hint
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
                  final newZoom = (_zoomAtScaleStart * details.scale)
                      .clamp(_zoomMin, _zoomMax);
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

// ---------------------------------------------------------------------------
// Dokumen laporan — formal, mengikuti gambar referensi
// Dokumen selalu putih, tabel dengan lebar minimum untuk proporsi benar
// ---------------------------------------------------------------------------

class KasReportDocument extends StatelessWidget {
  const KasReportDocument({
    super.key,
    required this.model,
    this.zoom = 1.0,
    this.viewportWidth = 900.0,
  });
  final KasReportModel model;
  final double zoom;
  final double viewportWidth;

  @override
  Widget build(BuildContext context) {
    final rows = model.statementRows();
    final generatedAt = model.generatedAt;
    final hasOther = model.expenseHasOther;

    // Lebar efektif diskalakan oleh zoom agar dokumen membesar/mengecil sesuai zoom level.
    // Dikalikan zoomMax (2.5) sebagai batas maksimal agar dokumen tidak terlalu lebar pada zoom > 1.
    final effectiveWidth =
        (viewportWidth.isFinite ? viewportWidth : 900.0) * zoom;
    final effectiveMaxWidth = effectiveWidth * 2.5;
    final effectiveMinWidth = effectiveWidth.clamp(300.0, effectiveMaxWidth);

    return ConstrainedBox(
      constraints: BoxConstraints(
        minWidth: effectiveMinWidth,
        maxWidth: effectiveMaxWidth,
      ),
      child: Container(
        key: const Key('kas-report-document'),
        decoration: const BoxDecoration(color: Color(0xFFFFFFFF)),
        padding: EdgeInsets.all(16 * zoom),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            // ---- Header ----
            _buildHeader(),

            Divider(
              height: 24 * zoom,
              thickness: 0.5 * zoom,
              color: const Color(0xFFDDDDDD),
            ),

            // ---- Tabel transaksi ----
            if (model.transactionCount == 0)
              Padding(
                padding: EdgeInsets.symmetric(vertical: 24 * zoom),
                child: Center(
                  child: Text(
                    'Belum ada transaksi KAS pada ${model.periodLabel}.',
                    key: const Key('kas-report-empty'),
                    style: TextStyle(
                      fontSize: 11 * zoom,
                      color: const Color(0xFF666666),
                    ),
                  ),
                ),
              )
            else
              _buildStatementTable(rows),

            // ---- Rekap Pengeluaran per Kategori ----
            SizedBox(height: 28 * zoom),
            _buildRecapSection(hasOther),

            // ---- Footer ----
            SizedBox(height: 16 * zoom),
            Padding(
              padding: EdgeInsets.only(top: 12 * zoom),
              child: Divider(
                thickness: 0.5 * zoom,
                color: const Color(0xFFE0E0E0),
              ),
            ),
            Padding(
              padding: EdgeInsets.only(top: 12 * zoom),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.start,
                children: [
                  Text(
                    'Dicetak: ${_formatDate(generatedAt)}',
                    style: TextStyle(
                      fontSize: 9 * zoom,
                      color: const Color(0xFF888888),
                    ),
                  ),
                ],
              ),
            ),
            ],
          ),
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
                      fontSize: (i == 0 ? 14 : 11) * zoom,
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
  // Tabel transaksi utama — lebar kolom eksplisit dan diskalakan oleh zoom.
  //
  // Kontrak data: setiap baris memiliki [r.label] = uraian/deskripsi
  // yang berasal dari field [description] pada model transaksi.
  // Header "Uraian" dan sel uraian (r.label) sudah ada di kode asli.
  //
  // Penyebab kolom Uraian hilang: sebelumnya menggunakan
  // [FlexColumnWidth()] tanpa [maxWidth] pada container luar, sehingga
  // kolom fleksibel mendapat ruang hampir nol.
  // Perbaikan: [FixedColumnWidth(120 * zoom)] menjamin lebar minimum
  // agar teks uraian selalu terbaca, bersama dengan [minWidth: 700 * zoom]
  // agar tabel tidak lebih kecil dari lebar tabelnya sendiri.
  // ------------------------------------------------------------------

  Widget _buildStatementTable(List<KasStatementRow> rows) {
    return LayoutBuilder(
      builder: (context, constraints) {
        // Tidak perlu SingleChildScrollView lagi — outer scroll di
        // _buildDocumentPage sudah menangani horizontal scroll ketika konten
        // melebihi viewport.
        return Table(
          key: const Key('kas-report-statement'),
          border: TableBorder.all(
            color: const Color(0xFFCCCCCC),
            width: 0.5 * zoom,
          ),
          columnWidths: {
            0: FixedColumnWidth(25 * zoom),       // No
            1: FlexColumnWidth(1.0),              // Uraian — ambil sisa ruang
            2: FixedColumnWidth(85 * zoom),       // Debet
            3: FixedColumnWidth(85 * zoom),       // Kredit
            4: FixedColumnWidth(95 * zoom),       // Saldo
          },
          defaultVerticalAlignment: TableCellVerticalAlignment.middle,
          children: [
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
            for (final r in rows) _statementRow(r),
          ],
        );
      },
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
      padding: EdgeInsets.symmetric(
        horizontal: 6 * zoom,
        vertical: 4 * zoom,
      ),
      child: Text(
        text,
        softWrap: false,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        textAlign: right ? TextAlign.right : TextAlign.left,
        style: TextStyle(
          fontSize: size * zoom,
          color: const Color(0xFF222222),
          height: 1.3,
        ).copyWith(
          fontWeight: bold ? FontWeight.bold : FontWeight.normal,
        ),
      ),
    );
  }

  // ------------------------------------------------------------------
  // Rekap pengeluaran per kategori
  // ------------------------------------------------------------------

  Widget _buildRecapSection(bool hasOther) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'REKAP PENGELUARAN PER KATEGORI',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 12 * zoom,
            fontWeight: FontWeight.bold,
            color: const Color(0xFF222222),
          ),
        ),
        Text(
          model.period != null
              ? 'Periode ${model.periodLabel}'
              : 'Semua Periode',
          textAlign: TextAlign.center,
          style: TextStyle(
            fontSize: 10 * zoom,
            color: const Color(0xFF666666),
          ),
        ),
        SizedBox(height: 8 * zoom),
        _buildRecapTable(hasOther),
      ],
    );
  }

  Widget _buildRecapTable(bool hasOther) {
    return ConstrainedBox(
      constraints: BoxConstraints(
        minWidth: 450 * zoom,
        maxWidth: 2000 * zoom,
      ),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Table(
          key: const Key('kas-report-recap'),
          border: TableBorder.all(
            color: const Color(0xFFCCCCCC),
            width: 0.5 * zoom,
          ),
          columnWidths: {
            0: FlexColumnWidth(),
            1: FixedColumnWidth(100 * zoom),
            2: FixedColumnWidth(100 * zoom),
            if (hasOther) 3: FixedColumnWidth(100 * zoom),
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
