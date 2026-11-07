import 'dart:io';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:path_provider/path_provider.dart';
import 'package:printing/printing.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/error_state.dart';
import 'package:social_finance/core/widgets/loading_indicator.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/monitoring/data/monitoring_models.dart';
import 'package:social_finance/features/monitoring/data/monitoring_repository.dart';

/// Layar monitoring untuk Petugas Kebersihan.
///
/// Menampilkan daftar rumah yang memiliki tunggakan iuran wajib minimal
/// 3 bulan kalender dan masih belum lunas. Data bersumber dari backend,
/// tidak dihitung ulang di Flutter.
class MonitoringScreen extends ConsumerStatefulWidget {
  const MonitoringScreen({super.key});

  @override
  ConsumerState<MonitoringScreen> createState() => _MonitoringScreenState();
}

class _MonitoringScreenState extends ConsumerState<MonitoringScreen> {
  String? _rtId;
  String? _rtName;
  String? _rwNumber;

  MonitoringListResponse? _monitoringData;
  bool _isLoading = false;
  String? _activeExport;
  String? _error;

  String? _selectedOccupancyId;
  String? _selectedHouseNumber;
  bool _noteSaving = false;

  @override
  void initState() {
    super.initState();
    _loadTenantInfo();
  }

  Future<void> _loadTenantInfo() async {
    final authState = ref.read(authRepositoryProvider);
    final user = authState?.valueOrNull;
    if (user == null) return;

    final rtId = user['rt_id'] as String?;
    final rtName = user['rt_name'] as String?;
    final rwNumber = user['rt_rw'] as int?;

    if (rtId == null || rtId.isEmpty) {
      if (!mounted) return;
      setState(() {
        _error = 'Tidak dapat menentukan RT milik Anda.';
      });
      return;
    }

    if (!mounted) return;
    setState(() {
      _rtId = rtId;
      _rtName = rtName;
      _rwNumber = rwNumber != null ? 'RW ${rwNumber.toString().padLeft(3, '0')}' : '';
    });

    await _fetchMonitoring();
  }

  Future<void> _fetchMonitoring() async {
    if (_rtId == null) return;

    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final repo = ref.read(monitoringRepositoryProvider);
      final data = await repo.fetchMonitoringHouseholds(_rtId!);
      if (!mounted) return;
      setState(() {
        _monitoringData = data;
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        if (e is ServerException) {
          _error = e.message;
        } else {
          _error = 'Gagal memuat daftar monitoring: $e';
        }
      });
    }
  }

  void _openNoteModal(MonitoringHousehold household) {
    _selectedOccupancyId = household.householdOccupancyId;
    _selectedHouseNumber = household.houseNumber;
    _showNoteDialog();
  }

  Future<void> _showNoteDialog() async {
    if (_selectedOccupancyId == null) return;

    final result = await showDialog<String>(
      context: context,
      builder: (dialogContext) => _NoteDialog(
        houseNumber: _selectedHouseNumber ?? '',
      ),
    );

    if (result != null && result.isNotEmpty && _selectedOccupancyId != null) {
      await _saveNote(result);
    }
  }

  Future<void> _saveNote(String note) async {
    if (_selectedOccupancyId == null || _rtId == null) return;

    setState(() {
      _noteSaving = true;
    });

    try {
      final repo = ref.read(monitoringRepositoryProvider);
      await repo.createMonitoringNote(
        rtId: _rtId!,
        householdOccupancyId: _selectedOccupancyId!,
        note: note,
      );
      if (!mounted) return;
      setState(() {
        _noteSaving = false;
      });
      await _fetchMonitoring();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Catatan tersimpan')),
        );
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _noteSaving = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Gagal menyimpan catatan: $e'),
          backgroundColor: AppColors.danger,
        ),
      );
    }
  }

  Future<void> _exportExcel() async {
    await _saveMonitoringExportLocally(
      format: 'xlsx',
      generate: (repo, rtId) => repo.exportMonitoringExcel(rtId),
    );
  }

  Future<void> _exportPDF() async {
    await _saveMonitoringExportLocally(
      format: 'pdf',
      generate: (repo, rtId) => repo.exportMonitoringPDF(rtId),
    );
  }

  Future<void> _saveMonitoringExportLocally({
    required String format,
    required Future<List<int>> Function(MonitoringRepository, String) generate,
  }) async {
    final rtId = _rtId;
    if (rtId == null || _isLoading || _activeExport != null) return;

    setState(() {
      _activeExport = format;
      _error = null;
    });

    try {
      final repo = ref.read(monitoringRepositoryProvider);
      final bytes = await generate(repo, rtId);
      if (bytes.isEmpty) {
        throw const FormatException('File export kosong.');
      }

      final now = DateTime.now();
      final date =
          '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
      final rtLabel = (_rtName ?? rtId)
          .replaceAll(RegExp(r'[^A-Za-z0-9_-]'), '_');
      final fileName = 'Monitoring_Kebersihan_${rtLabel}_$date.$format';

      Directory? targetDir;
      if (Platform.isAndroid) {
        final androidDownload = Directory('/storage/emulated/0/Download');
        if (await androidDownload.exists()) {
          targetDir = androidDownload;
        } else {
          try {
            targetDir = await getDownloadsDirectory();
          } catch (_) {}
          targetDir ??= await getExternalStorageDirectory() ??
              await getApplicationDocumentsDirectory();
        }
      } else {
        targetDir = await getDownloadsDirectory() ??
            await getApplicationDocumentsDirectory();
      }

      if (!await targetDir.exists()) {
        await targetDir.create(recursive: true);
      }

      // Handle duplicate filenames gracefully
      var counter = 1;
      var savePath = '${targetDir.path}/$fileName';
      var finalName = fileName;
      final dotIdx = fileName.lastIndexOf('.');
      final baseName = dotIdx != -1 ? fileName.substring(0, dotIdx) : fileName;
      final ext = dotIdx != -1 ? fileName.substring(dotIdx) : '';

      while (await File(savePath).exists()) {
        finalName = '${baseName}_$counter$ext';
        savePath = '${targetDir.path}/$finalName';
        counter++;
      }

      final file = File(savePath);
      await file.writeAsBytes(bytes, flush: true);
      if (await file.length() != bytes.length) {
        throw const FileSystemException('File export tidak tersimpan utuh.');
      }

      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'File berhasil disimpan:\n$finalName\nLokasi: ${targetDir.path}',
          ),
          backgroundColor: AppColors.success,
          duration: const Duration(seconds: 4),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      final message = e is ServerException
          ? e.message
          : 'Gagal menyimpan file export: $e';
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(message),
          backgroundColor: AppColors.danger,
        ),
      );
    } finally {
      if (mounted) {
        setState(() {
          _activeExport = null;
        });
      }
    }
  }

  Future<void> _printPDF() async {
    if (_rtId == null) return;
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final repo = ref.read(monitoringRepositoryProvider);
      final bytes = await repo.exportMonitoringPDF(_rtId!);
      final pdfBytes = Uint8List.fromList(bytes);

      if (!mounted) return;
      await Printing.layoutPdf(
        onLayout: (format) async => pdfBytes,
        name: 'Laporan Monitoring Kebersihan',
      );
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        if (e is ServerException) {
          _error = e.message;
        } else {
          _error = 'Gagal mencetak PDF: $e';
        }
      });
    } finally {
      if (mounted) {
        setState(() {
          _isLoading = false;
        });
      }
    }
  }

  String _formatDateTime(String? raw) {
    if (raw == null || raw.isEmpty) return '-';
    try {
      final dt = DateTime.tryParse(raw);
      if (dt == null) return raw;
      return '${dt.day.toString().padLeft(2, '0')}/${dt.month.toString().padLeft(2, '0')}/${dt.year} ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return raw;
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Monitoring'),
      ),
      body: RefreshIndicator(
        onRefresh: _fetchMonitoring,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // ── Header Info ──
            Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          'RT ${_rtName ?? _rtId ?? '-'}',
                          style: theme.textTheme.titleLarge?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      ),
                      if (_rwNumber != null)
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.accent.withValues(alpha: 0.10),
                            borderRadius: BorderRadius.circular(AppRadius.xs),
                          ),
                          child: Text(
                            _rwNumber!,
                            style: theme.textTheme.bodySmall?.copyWith(
                              fontWeight: FontWeight.w600,
                              color: AppColors.accent,
                            ),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Daftar rumah dengan tunggakan iuran wajib 3+ bulan',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  if (_monitoringData != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        'Total: ${_monitoringData!.total} rumah (per ${_monitoringData!.evalDate})',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontWeight: FontWeight.w600,
                          color: AppColors.accent,
                        ),
                      ),
                    ),
                ],
              ),
            ),

            // ── Error State ──
            if (_error != null && _monitoringData == null)
              Expanded(
                child: Center(
                  child: ErrorState(
                    message: _error!,
                    onRetry: _fetchMonitoring,
                    actionText: 'Coba Lagi',
                  ),
                ),
              ),

            // ── Content ──
            if (_error == null)
              Expanded(
                child: Builder(
                  builder: (context) {
                    if (_isLoading && _monitoringData == null) {
                      return const Center(child: LoadingIndicator());
                    }

                    final households = _monitoringData?.households ?? [];
                    if (households.isEmpty) {
                      return const Center(
                        child: EmptyState(
                          icon: Icons.home_outlined,
                          title: 'Tidak Ada Rumah dalam Monitoring',
                          description:
                              'Belum ada rumah yang memenuhi kriteria tunggakan iuran wajib 3+ bulan.',
                        ),
                      );
                    }

                    return ListView.builder(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      itemCount: households.length,
                      itemBuilder: (context, index) {
                        final h = households[index];
                        return _HouseholdCard(
                          key: ValueKey(h.householdOccupancyId),
                          household: h,
                          isDark: isDark,
                          onNoteTap: _openNoteModal,
                        );
                      },
                    );
                  },
                ),
              ),
          ],
        ),
      ),
      bottomNavigationBar: _monitoringData == null
          ? null
          : SafeArea(
              top: false,
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (_activeExport != null) ...[
                      const LinearProgressIndicator(),
                      const SizedBox(height: 8),
                      Text(
                        'Menyiapkan file ${_activeExport!.toUpperCase()}...',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                      const SizedBox(height: 8),
                    ],
                    Row(
                      children: [
                        Expanded(
                          child: _actionButton(
                            label: 'Export Excel',
                            icon: Icons.table_view_outlined,
                            color: AppColors.success,
                            foregroundColor: Colors.white,
                            onPressed: _canStartAction ? _exportExcel : null,
                          ),
                        ),
                        const SizedBox(width: 10),
                        Expanded(
                          child: _actionButton(
                            label: 'Export PDF',
                            icon: Icons.picture_as_pdf_outlined,
                            color: AppColors.danger,
                            foregroundColor: Colors.white,
                            onPressed: _canStartAction ? _exportPDF : null,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    _actionButton(
                      label: 'Cetak',
                      icon: Icons.print_outlined,
                      color: AppColors.accent,
                      foregroundColor: Colors.white,
                      onPressed: _canStartAction ? _printPDF : null,
                    ),
                  ],
                ),
              ),
            ),
    );
  }

  bool get _canStartAction => !_isLoading && _activeExport == null;

  Widget _actionButton({
    required String label,
    required IconData icon,
    required Color color,
    required Color foregroundColor,
    required VoidCallback? onPressed,
  }) {
    return SizedBox(
      width: double.infinity,
      height: 48,
      child: FilledButton.icon(
        onPressed: onPressed,
        icon: Icon(icon, size: 18),
        label: FittedBox(
          fit: BoxFit.scaleDown,
          child: Text(label, maxLines: 1),
        ),
        style: FilledButton.styleFrom(
          backgroundColor: color,
          foregroundColor: foregroundColor,
          disabledBackgroundColor: color.withValues(alpha: 0.38),
          disabledForegroundColor: foregroundColor.withValues(alpha: 0.38),
          padding: const EdgeInsets.symmetric(horizontal: 12),
          textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.sm),
          ),
        ),
      ),
    );
  }
}

/// Kartu satu rumah dalam daftar monitoring.
class _HouseholdCard extends ConsumerWidget {
  final MonitoringHousehold household;
  final bool isDark;
  final void Function(MonitoringHousehold) onNoteTap;

  const _HouseholdCard({
    super.key,
    required this.household,
    required this.isDark,
    required this.onNoteTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: AppCard(
        child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              CircleAvatar(
                radius: 18,
                backgroundColor: AppColors.accent.withValues(alpha: 0.12),
                child: Text(
                  household.houseNumber.isNotEmpty
                      ? household.houseNumber[0].toUpperCase()
                      : '?',
                  style: TextStyle(
                    color: AppColors.accent,
                    fontWeight: FontWeight.bold,
                    fontSize: 16,
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'No. Rumah ${household.houseNumber}',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    const SizedBox(height: 4),
                  ],
                ),
              ),
              IconButton(
                icon: const Icon(Icons.edit_outlined, size: 20),
                color: AppColors.accent,
                tooltip: 'Edit Catatan',
                onPressed: () => onNoteTap(household),
              ),
            ],
          ),
          const Divider(height: 20),
          if (household.note != null && household.note!.isNotEmpty) ...[
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(
                  Icons.note_outlined,
                  size: 16,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Catatan Operasional',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontWeight: FontWeight.w600,
                          color: AppColors.accent,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        household.note!,
                        style: theme.textTheme.bodyMedium,
                      ),
                      if (household.noteCreatedAt != null ||
                          household.noteUpdatedAt != null) ...[
                        const SizedBox(height: 4),
                        Text(
                          'Diperbarui: ${_formatDateTime(household.noteUpdatedAt ?? household.noteCreatedAt)}',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                            fontStyle: FontStyle.italic,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ] else
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text(
                'Belum ada catatan lapangan.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
        ],
      ),
    ),
  );
  }

  String _formatDateTime(String? raw) {
    if (raw == null || raw.isEmpty) return '-';
    try {
      final dt = DateTime.tryParse(raw);
      if (dt == null) return raw;
      return '${dt.day.toString().padLeft(2, '0')}/${dt.month.toString().padLeft(2, '0')}/${dt.year} ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return raw;
    }
  }
}

class _NoteDialog extends StatefulWidget {
  final String houseNumber;
  const _NoteDialog({required this.houseNumber});

  @override
  State<_NoteDialog> createState() => _NoteDialogState();
}

class _NoteDialogState extends State<_NoteDialog> {
  late final TextEditingController _controller;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return AlertDialog(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      title: Text(
        'Catatan Lapangan - ${widget.houseNumber}',
        style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 16),
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            controller: _controller,
            maxLines: 4,
            minLines: 3,
            decoration: InputDecoration(
              hintText: 'Masukkan catatan lapangan...',
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppRadius.sm),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppRadius.sm),
                borderSide: const BorderSide(
                  color: AppColors.accent,
                  width: 2,
                ),
              ),
            ),
          ),
        ],
      ),
      actionsPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      actions: [
        OutlinedButton(
          style: OutlinedButton.styleFrom(
            minimumSize: const Size(80, 42),
            foregroundColor:
                isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
            side: BorderSide(
              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
            ),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            textStyle: const TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: 14,
            ),
          ),
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        ElevatedButton(
          style: ElevatedButton.styleFrom(
            minimumSize: const Size(88, 42),
            backgroundColor: AppColors.accent,
            foregroundColor: Colors.white,
            elevation: 0,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppRadius.sm),
            ),
            textStyle: const TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: 14,
            ),
          ),
          onPressed: () {
            final note = _controller.text.trim();
            if (note.isNotEmpty) {
              Navigator.of(context).pop(note);
            }
          },
          child: const Text('Simpan'),
        ),
      ],
    );
  }
}
