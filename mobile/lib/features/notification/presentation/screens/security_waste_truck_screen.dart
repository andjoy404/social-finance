import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/loading_indicator.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/primary_button.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'security_waste_truck_history_screen.dart';

/// Layar khusus Petugas Keamanan untuk mengirim notifikasi kedatangan truk sampah.
///
/// Hanya menggunakan template waste_truck_arrival yang sudah tersedia.
/// Konten template hanya ditampilkan, tidak dapat diedit.
/// Petugas mengambil foto truk sampah, mengunggahnya, lalu mengirim notifikasi.
class SecurityWasteTruckScreen extends ConsumerStatefulWidget {
  const SecurityWasteTruckScreen({super.key});

  @override
  ConsumerState<SecurityWasteTruckScreen> createState() =>
      _SecurityWasteTruckScreenState();
}

class _SecurityWasteTruckScreenState
    extends ConsumerState<SecurityWasteTruckScreen> {
  WasteTruckTemplate? _template;
  bool _isLoadingTemplate = true;
  String? _templateError;

  XFile? _photo;
  bool _isSending = false;
  String? _sendError;
  String _customMessage = '';

  @override
  void initState() {
    super.initState();
    _loadTemplate();
  }

  Future<void> _loadTemplate() async {
    setState(() {
      _isLoadingTemplate = true;
      _templateError = null;
    });

    try {
      final repo = ref.read(notificationRepositoryProvider);
      final template = await repo.fetchWasteTruckTemplate();
      if (!mounted) return;
      setState(() {
        _template = template;
        _isLoadingTemplate = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoadingTemplate = false;
        if (e is ServerException) {
          _templateError = e.message;
        } else {
          _templateError = 'Gagal memuat template: $e';
        }
      });
    }
  }

  Future<void> _takePhoto() async {
    if (_isSending) return;

    try {
      final picker = ImagePicker();
      final picked = await picker.pickImage(
        source: ImageSource.camera,
        maxWidth: 1024,
        maxHeight: 1024,
        imageQuality: 85,
      );

      if (!mounted) return;

      if (picked != null) {
        setState(() {
          _photo = picked;
        });
      }
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Gagal mengambil foto: $e'),
          backgroundColor: AppColors.danger,
        ),
      );
    }
  }

  void _removePhoto() {
    setState(() {
      _photo = null;
    });
  }

  Future<void> _sendNotification() async {
    if (_isSending) return;
    if (_photo == null) {
      setState(() {
        _sendError = 'Ambil foto truk sampah terlebih dahulu.';
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Ambil foto truk sampah terlebih dahulu.'),
        ),
      );
      return;
    }

    setState(() {
      _isSending = true;
      _sendError = null;
    });

    String? imageUrl;

    // Unggah foto terlebih dahulu jika ada
    if (_photo != null) {
      try {
        final bytes = await _photo!.readAsBytes();
        final repo = ref.read(notificationRepositoryProvider);
        imageUrl = await repo.uploadWasteTruckImage(
          bytes: bytes,
          filename: 'waste_truck_${DateTime.now().millisecondsSinceEpoch}.jpg',
          eventType: 'waste_truck_arrival',
        );
      } catch (e) {
        if (!mounted) return;
        setState(() {
          _isSending = false;
        });
        if (e is ServerException) {
          _sendError = e.message;
        } else {
          _sendError = 'Gagal mengunggah foto: $e';
        }
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(_sendError ?? 'Gagal mengunggah foto'),
            backgroundColor: AppColors.danger,
          ),
        );
        return;
      }
    }

    // Kirim notifikasi
    try {
      final repo = ref.read(notificationRepositoryProvider);
      await repo.sendWasteTruck(
        message: _customMessage.trim(),
        imageUrl: imageUrl,
      );
      if (!mounted) return;
      setState(() {
        _isSending = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Permintaan diterima. Periksa status pengiriman pada tab Riwayat.',
          ),
          backgroundColor: AppColors.success,
        ),
      );
      // Reset state setelah berhasil
      setState(() {
        _photo = null;
        _customMessage = '';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isSending = false;
      });
      if (e is ServerException) {
        _sendError = e.message;
      } else {
        _sendError = 'Gagal mengirim notifikasi: $e';
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(_sendError ?? 'Gagal mengirim notifikasi'),
          backgroundColor: AppColors.danger,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return DefaultTabController(
      length: 2,
      child: Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Notifikasi'),
          bottom: const TabBar(
            tabs: [
              Tab(text: 'Template'),
              Tab(text: 'Riwayat'),
            ],
          ),
        ),
        body: TabBarView(
          children: [
            _buildBody(theme, isDark),
            const SecurityWasteTruckHistoryScreen(),
          ],
        ),
      ),
    );
  }

  Widget _buildBody(ThemeData theme, bool isDark) {
    if (_isLoadingTemplate) {
      return const Center(child: LoadingIndicator());
    }

    if (_templateError != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 64, color: AppColors.danger),
            const SizedBox(height: 16),
            Text(
              'Template Tidak Tersedia',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            Text(
              _templateError!,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
            const SizedBox(height: 16),
            PrimaryButton(
              text: 'Coba Lagi',
              icon: Icons.refresh,
              onPressed: _loadTemplate,
            ),
          ],
        ),
      );
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ── Template Info (Read Only) ──
          AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            _template?.title ?? 'Kedatangan Truk Sampah',
                            style: theme.textTheme.titleLarge?.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 8),
                          Text(
                            _template?.body ??
                                'Truk sampah telah tiba di lokasi RT.',
                            style: theme.textTheme.bodyMedium,
                          ),
                        ],
                      ),
                    ),
                    if (_template?.imageUrl != null &&
                        _template!.imageUrl!.isNotEmpty)
                      ClipRRect(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                        child: Image.network(
                          _template!.imageUrl!,
                          width: 100,
                          height: 100,
                          fit: BoxFit.cover,
                          errorBuilder: (_, _, _) => Container(
                            width: 100,
                            height: 100,
                            color: isDark
                                ? Colors.grey[800]!
                                : Colors.grey[200]!,
                            child: const Icon(Icons.broken_image, size: 32),
                          ),
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.accent.withValues(alpha: 0.08),
                    borderRadius: BorderRadius.circular(AppRadius.sm),
                    border: Border.all(
                      color: AppColors.accent.withValues(alpha: 0.2),
                    ),
                  ),
                  child: Row(
                    children: [
                      const Icon(
                        Icons.info_outline,
                        size: 16,
                        color: AppColors.accent,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          'Notifikasi akan dikirim ke seluruh warga aktif dalam RT Anda. Konten template tidak dapat diubah.',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),

          const SizedBox(height: 20),

          // ── Foto Truk Sampah ──
          Text(
            'Foto Truk Sampah',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 12),
          _buildPhotoSection(theme, isDark),

          const SizedBox(height: 20),

          // ── Pesan Tambahan (Opsional) ──
          Text(
            'Pesan Tambahan (Opsional)',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          AppCard(
            child: TextField(
              maxLines: 3,
              decoration: InputDecoration(
                hintText: 'Tambahkan pesan tambahan jika diperlukan...',
                border: const OutlineInputBorder(),
                contentPadding: const EdgeInsets.all(12),
              ),
              onChanged: (value) {
                setState(() {
                  _customMessage = value;
                });
              },
            ),
          ),

          const SizedBox(height: 24),

          // ── Tombol Kirim ──
          PrimaryButton(
            text: 'Kirim Notifikasi',
            icon: Icons.send_outlined,
            onPressed: _isSending ? null : _sendNotification,
            isLoading: _isSending,
          ),
        ],
      ),
    );
  }

  Widget _buildPhotoSection(ThemeData theme, bool isDark) {
    if (_photo != null) {
      return AppCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Stack(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(AppRadius.base),
                  child: Image.file(
                    File(_photo!.path),
                    width: double.infinity,
                    height: 200,
                    fit: BoxFit.cover,
                  ),
                ),
                Positioned(
                  top: 8,
                  right: 8,
                  child: IconButton(
                    icon: const Icon(
                      Icons.delete_outline,
                      color: AppColors.danger,
                    ),
                    onPressed: _removePhoto,
                    tooltip: 'Hapus Foto',
                  ),
                ),
                Positioned(
                  bottom: 8,
                  right: 8,
                  child: Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: Colors.black.withValues(alpha: 0.6),
                      borderRadius: BorderRadius.circular(AppRadius.xs),
                    ),
                    child: const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.camera_alt, color: Colors.white, size: 16),
                        SizedBox(width: 4),
                        Text(
                          'Ulangi',
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 12,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              'Foto siap diunggah bersama notifikasi',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: double.infinity,
          child: PrimaryButton(
            text: 'Ambil Foto Truk Sampah',
            icon: Icons.camera_alt_outlined,
            onPressed: _isSending ? null : _takePhoto,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'Foto wajib diambil sebelum mengirim notifikasi.',
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontStyle: FontStyle.italic,
          ),
        ),
      ],
    );
  }
}
