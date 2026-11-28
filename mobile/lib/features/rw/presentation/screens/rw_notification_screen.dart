import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/notification/notification_url_helper.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/jakarta_time_utils.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/utils/url_launcher_utils.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_button.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/authenticated_network_image.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

typedef RwPickedImage = ({Uint8List bytes, String filename});
typedef RwImagePickCallback = Future<RwPickedImage?> Function(ImageSource source);

/// RW Notification screen (Android). Contracts:
/// - Routes behind backend RequireRWScope; service enforces rw.notification.read/write.
/// - Explicit RW membership: read for pengurus RW, write via rw.notification.write / notification.manage aliases.
/// - rt_inferred / no RW membership: must not access (tab hidden; backend denies).
/// - Super Admin bypass handled by backend.
/// State is keyed by rwId/page via autoDispose family providers; RW/account change disposes cache.
class RwNotificationScreen extends ConsumerStatefulWidget {
  final String rwId;
  final RwImagePickCallback? imagePickerOverride;

  const RwNotificationScreen({
    super.key,
    required this.rwId,
    this.imagePickerOverride,
  });

  @override
  ConsumerState<RwNotificationScreen> createState() => _RwNotificationScreenState();
}

class _RwNotificationScreenState extends ConsumerState<RwNotificationScreen>
    with SingleTickerProviderStateMixin {
  static const int _maxImageBytes = 5 * 1024 * 1024;
  static const Set<String> _imageSupportedEvents = {
    'manual_push',
    'rw_cash_in',
    'rw_cash_out',
  };

  static const Map<String, String> _eventLabels = {
    'manual_push': 'Notifikasi Manual RW',
    'rw_cash_in': 'Kas Masuk RW',
    'rw_cash_out': 'Kas Keluar RW',
    'iuran_payment_reminder': 'Pengingat Iuran',
  };

  late final TabController _tabController;
  int _page = 1;
  final int _minePage = 1;
  bool _sending = false;
  bool _templateBusy = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  String _eventLabel(String eventType) => _eventLabels[eventType] ?? eventType;

  bool _supportsImageForEvent(String eventType) =>
      _imageSupportedEvents.contains(eventType);

  Widget _buildBodyWithLinks(String body) {
    return NotificationBodyText(
      text: body,
      eventType: null,
      style: const TextStyle(color: Color(0xFFCBD5E1), fontSize: 12, height: 1.4),
      linkStyle: const TextStyle(
        color: Color(0xFFA970FF),
        decoration: TextDecoration.underline,
      ),
      onOpenUrl: (url) => UrlDetector.openUrl(url),
    );
  }

  Widget _buildBodyWithLinksDialog(String body) {
    final textStyle = TextStyle(fontSize: 12, color: Theme.of(context).colorScheme.onSurface);
    return NotificationBodyText(
      text: body,
      eventType: null,
      style: textStyle,
      linkStyle: const TextStyle(
        color: AppColors.accent,
        decoration: TextDecoration.underline,
      ),
      onOpenUrl: (url) => UrlDetector.openUrl(url),
    );
  }

  bool _canWrite(Map<String, dynamic>? user) =>
      canManageRwNotifications(user, widget.rwId);

  String _friendlyError(Object e) {
    final s = e.toString();
    final lower = s.toLowerCase();
    if (s.contains('403') ||
        lower.contains('forbidden') ||
        lower.contains('access denied') ||
        lower.contains('permission denied')) {
      return 'Anda tidak memiliki izin untuk aksi ini.';
    }
    if (s.contains('401') || lower.contains('unauthorized')) {
      return 'Sesi tidak valid. Silakan login ulang.';
    }
    if (lower.contains('camera_access_denied')) {
      return 'Izin kamera ditolak. Aktifkan izin kamera di pengaturan perangkat.';
    }
    if (lower.contains('photo_access_denied')) {
      return 'Izin galeri ditolak. Aktifkan izin foto/galeri di pengaturan perangkat.';
    }
    if (lower.contains('upload')) {
      return 'Gagal mengunggah gambar. Periksa format/ukuran file dan coba lagi.';
    }
    return 'Gagal memuat atau menyimpan data notifikasi.';
  }

  Future<RwPickedImage?> _pickImageFromSource(ImageSource source) async {
    if (widget.imagePickerOverride != null) {
      return widget.imagePickerOverride!(source);
    }
    final picker = ImagePicker();
    final picked = await picker.pickImage(
      source: source,
      maxWidth: 1920,
      maxHeight: 1920,
      imageQuality: 85,
    );
    if (picked == null) return null;
    final bytes = await picked.readAsBytes();
    return (bytes: bytes, filename: picked.name);
  }

  Future<void> _refresh() async {
    ref.invalidate(rwNotificationLogsProvider((rwId: widget.rwId, page: _page, eventType: null)));
    ref.invalidate(rwNotificationLogsMineProvider((rwId: widget.rwId, page: _minePage, eventType: null)));
    ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    ref.invalidate(rwUnreadCountProvider(widget.rwId));
  }

  Future<void> _markRwLogReadAndSync(RwNotificationLog n) async {
    final id = n.id.trim();
    if (id.isEmpty || n.isRead) return;
    try {
      await ref.read(notificationRepositoryProvider).markAsRead(id);
      if (!mounted) return;
      ref.invalidate(rwUnreadCountProvider(widget.rwId));
      ref.read(notificationUnreadCountProvider.notifier).refresh();
    } catch (_) {
      // Follow existing rule: if markAsRead fails, do not falsely clear badge.
    }
  }

  Future<void> _sendManualPush() async {
    final titleC = TextEditingController();
    final bodyC = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Kirim Notifikasi Manual'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: titleC, decoration: const InputDecoration(labelText: 'Judul')),
            const SizedBox(height: 8),
            TextField(controller: bodyC, decoration: const InputDecoration(labelText: 'Isi'), maxLines: 3),
            const SizedBox(height: 8),
            Text(
              'Notifikasi akan dikirim ke pengurus RW dan RT di bawah RW ini.',
              style: TextStyle(fontSize: 12, color: Theme.of(ctx).colorScheme.onSurfaceVariant),
            ),
          ],
        ),
        actions: [
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.danger,
              foregroundColor: Colors.white,
              elevation: 0,
            ),
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Kirim')),
        ],
      ),
    );
    if (ok != true) return;
    final title = titleC.text.trim();
    final body = bodyC.text.trim();
    if (title.isEmpty || body.isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Judul dan isi wajib diisi')));
      return;
    }
    setState(() => _sending = true);
    try {
      final count = await ref.read(rwApiClientProvider).sendManualPush(widget.rwId, title: title, body: body);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Notifikasi terkirim ($count penerima)')));
      await _refresh();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  Future<void> _createTemplate() async {
    await _showTemplateFormDialog();
  }

  Future<void> _showTemplateFormDialog({RwNotificationTemplate? template}) async {
    final isEditing = template != null;
    final nameC = TextEditingController(text: template?.name ?? '');
    final titleC = TextEditingController(text: template?.title ?? '');
    final bodyC = TextEditingController(text: template?.body ?? '');
    String eventType = template?.eventType ?? 'manual_push';
    String? currentImageUrl = template?.imageUrl;
    Uint8List? pickedBytes;
    String? pickedFilename;
    bool imageChanged = false;
    bool isSubmitting = false;
    String? formError;

    final eventOptions = <Map<String, String>>[
      {'value': 'manual_push', 'label': 'Notifikasi Manual RW'},
      {'value': 'rw_cash_in', 'label': 'Kas Masuk RW'},
      {'value': 'rw_cash_out', 'label': 'Kas Keluar RW'},
      {'value': 'iuran_payment_reminder', 'label': 'Pengingat Iuran'},
    ];

    if (!eventOptions.any((e) => e['value'] == eventType)) {
      eventOptions.add({'value': eventType, 'label': eventType});
    }

    final savedMessage = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) {
          final theme = Theme.of(ctx);
          final colorScheme = theme.colorScheme;
          final isDark = theme.brightness == Brightness.dark;
          final supportsImage = _supportsImageForEvent(eventType);
          final hasPreviewImage = pickedBytes != null ||
              (currentImageUrl != null && currentImageUrl!.isNotEmpty);

          Future<void> handlePick(ImageSource source) async {
            if (isSubmitting) return;
            setS(() => formError = null);
            try {
              final picked = await _pickImageFromSource(source);
              if (picked == null) {
                // Cancelled: keep existing draft/saved image untouched.
                return;
              }
              if (picked.bytes.lengthInBytes > _maxImageBytes) {
                setS(() => formError = 'Ukuran gambar maksimal 5 MB.');
                return;
              }
              setS(() {
                pickedBytes = picked.bytes;
                pickedFilename = picked.filename;
                currentImageUrl = null;
                imageChanged = true;
                formError = null;
              });
            } on PlatformException catch (e) {
              setS(() => formError = _friendlyError(e.code.isNotEmpty ? e.code : (e.message ?? e)));
            } catch (e) {
              setS(() => formError = _friendlyError(e));
            }
          }

          Future<void> handleSave() async {
            if (isSubmitting) return;
            final name = nameC.text.trim();
            final title = titleC.text.trim();
            final body = bodyC.text.trim();
            if (name.isEmpty || title.isEmpty || body.isEmpty) {
              setS(() => formError = 'Nama, judul, dan isi notifikasi wajib diisi.');
              return;
            }

            setS(() {
              isSubmitting = true;
              formError = null;
            });
            if (mounted) setState(() => _templateBusy = true);

            try {
              String? finalImageUrl;
              if (supportsImage && pickedBytes != null) {
                final uploaded = await ref.read(rwApiClientProvider).uploadNotificationImage(
                  widget.rwId,
                  eventType: eventType,
                  bytes: pickedBytes!,
                  filename: pickedFilename ?? 'attachment.png',
                );
                if (uploaded.isEmpty) {
                  throw Exception('upload failed: empty image url');
                }
                finalImageUrl = uploaded;
              } else if (isEditing) {
                if (imageChanged) {
                  // Explicit removal -> empty string tells backend to clear image_url.
                  finalImageUrl = (currentImageUrl != null && currentImageUrl!.isNotEmpty)
                      ? currentImageUrl
                      : '';
                } else {
                  // Unchanged -> omit image_url so backend preserves existing image.
                  finalImageUrl = null;
                }
              } else if (supportsImage && currentImageUrl != null && currentImageUrl!.isNotEmpty) {
                finalImageUrl = currentImageUrl;
              }

              if (isEditing) {
                await ref.read(rwApiClientProvider).updateNotificationTemplate(
                  widget.rwId,
                  template.id,
                  name: name,
                  title: title,
                  body: body,
                  imageUrl: finalImageUrl,
                );
              } else {
                await ref.read(rwApiClientProvider).createNotificationTemplate(
                  widget.rwId,
                  name: name,
                  eventType: eventType,
                  title: title,
                  body: body,
                  imageUrl: finalImageUrl,
                );
              }

              if (ctx.mounted) {
                Navigator.pop(
                  ctx,
                  isEditing ? 'Template berhasil diperbarui' : 'Template berhasil dibuat',
                );
              }
            } catch (e) {
              if (ctx.mounted) {
                setS(() {
                  isSubmitting = false;
                  formError = _friendlyError(e);
                });
              }
            } finally {
              if (mounted) setState(() => _templateBusy = false);
            }
          }

          return AlertDialog(
            title: Text(isEditing ? 'Ubah Template' : 'Tambah Template'),
            content: SizedBox(
              width: 420,
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (formError != null) ...[
                      Container(
                        key: const Key('rwTemplateFormError'),
                        width: double.infinity,
                        padding: const EdgeInsets.all(10),
                        margin: const EdgeInsets.only(bottom: 12),
                        decoration: BoxDecoration(
                          color: AppColors.danger.withValues(alpha: isDark ? 0.18 : 0.1),
                          borderRadius: BorderRadius.circular(8),
                          border: Border.all(
                            color: AppColors.danger.withValues(alpha: 0.4),
                          ),
                        ),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Icon(Icons.error_outline, size: 16, color: AppColors.danger),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                formError!,
                                style: const TextStyle(fontSize: 12, color: AppColors.danger),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                    TextField(
                      key: const Key('rwTemplateNameField'),
                      controller: nameC,
                      enabled: !isSubmitting,
                      decoration: const InputDecoration(labelText: 'Nama Template *'),
                    ),
                    const SizedBox(height: 12),
                    DropdownButtonFormField<String>(
                      key: const Key('rwTemplateEventTypeField'),
                      initialValue: eventType,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Jenis Event *'),
                      items: eventOptions
                          .map((e) => DropdownMenuItem(
                                value: e['value'],
                                child: Text(
                                  e['label']!,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ))
                          .toList(),
                      onChanged: (isEditing || isSubmitting)
                          ? null
                          : (v) {
                              if (v != null) {
                                setS(() {
                                  eventType = v;
                                  if (!_supportsImageForEvent(v)) {
                                    pickedBytes = null;
                                    pickedFilename = null;
                                    currentImageUrl = null;
                                  }
                                });
                              }
                            },
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      key: const Key('rwTemplateTitleField'),
                      controller: titleC,
                      enabled: !isSubmitting,
                      decoration: const InputDecoration(
                        labelText: 'Judul Notifikasi *',
                        hintText: 'Contoh: Kas Masuk {{amount}}',
                      ),
                    ),
                    const SizedBox(height: 12),
                    TextField(
                      key: const Key('rwTemplateBodyField'),
                      controller: bodyC,
                      enabled: !isSubmitting,
                      decoration: const InputDecoration(
                        labelText: 'Isi Notifikasi *',
                        hintText: 'Mendukung variabel {{amount}}, {{location}}',
                      ),
                      maxLines: 3,
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Lampiran Gambar (Opsional)',
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: colorScheme.onSurface,
                      ),
                    ),
                    const SizedBox(height: 6),
                    if (!supportsImage) ...[
                      Text(
                        'Jenis event ini tidak mendukung lampiran gambar.',
                        style: TextStyle(fontSize: 11, color: colorScheme.onSurfaceVariant),
                      ),
                    ] else ...[
                      if (pickedBytes != null) ...[
                        Container(
                          key: const Key('rwTemplateDraftImagePreview'),
                          decoration: BoxDecoration(
                            color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
                            borderRadius: BorderRadius.circular(8),
                            border: Border.all(
                              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
                            ),
                          ),
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: Image.memory(
                              pickedBytes!,
                              height: 140,
                              width: double.infinity,
                              fit: BoxFit.cover,
                            ),
                          ),
                        ),
                        const SizedBox(height: 8),
                      ] else if (currentImageUrl != null && currentImageUrl!.isNotEmpty) ...[
                        Container(
                          key: const Key('rwTemplateSavedImagePreview'),
                          decoration: BoxDecoration(
                            color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
                            borderRadius: BorderRadius.circular(8),
                            border: Border.all(
                              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
                            ),
                          ),
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: AuthenticatedNetworkImage(
                              imageUrl: ApiConfig.resolveImageUrl(currentImageUrl!) ?? currentImageUrl!,
                              height: 140,
                              width: double.infinity,
                              fit: BoxFit.cover,
                            ),
                          ),
                        ),
                        const SizedBox(height: 8),
                      ],
                      Row(
                        children: [
                          Expanded(
                            child: AppButton.accentSoft(
                              buttonKey: const Key('rwTemplatePickCameraBtn'),
                              text: 'Kamera',
                              icon: Icons.camera_alt_outlined,
                              size: AppButtonSize.medium,
                              fullWidth: true,
                              onPressed: isSubmitting
                                  ? null
                                  : () => handlePick(ImageSource.camera),
                            ),
                          ),
                          const SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: AppButton.accentSoft(
                              buttonKey: const Key('rwTemplatePickGalleryBtn'),
                              text: 'Galeri',
                              icon: Icons.photo_library_outlined,
                              size: AppButtonSize.medium,
                              fullWidth: true,
                              onPressed: isSubmitting
                                  ? null
                                  : () => handlePick(ImageSource.gallery),
                            ),
                          ),
                        ],
                      ),
                      if (hasPreviewImage) ...[
                        const SizedBox(height: AppSpacing.sm),
                        Align(
                          alignment: Alignment.centerLeft,
                          child: OutlinedButton.icon(
                            key: const Key('rwTemplateRemoveImageBtn'),
                            style: OutlinedButton.styleFrom(
                              foregroundColor: AppColors.danger,
                              side: BorderSide(
                                color: AppColors.danger.withValues(alpha: 0.5),
                              ),
                            ),
                            onPressed: isSubmitting
                                ? null
                                : () => setS(() {
                                      pickedBytes = null;
                                      pickedFilename = null;
                                      currentImageUrl = null;
                                      imageChanged = true;
                                    }),
                            icon: const Icon(Icons.delete_outline, size: 16),
                            label: const Text('Hapus Gambar', style: TextStyle(fontSize: 12)),
                          ),
                        ),
                      ],
                    ],
                  ],
                ),
              ),
            ),
            actions: [
              AppButton.danger(
                text: 'Batal',
                size: AppButtonSize.medium,
                onPressed: isSubmitting ? null : () => Navigator.pop(ctx),
              ),
              AppButton.primary(
                buttonKey: const Key('rwTemplateSaveBtn'),
                text: 'Simpan',
                size: AppButtonSize.medium,
                isLoading: isSubmitting,
                onPressed: isSubmitting ? null : handleSave,
              ),
            ],
          );
        },
      ),
    );

    if (savedMessage != null && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(savedMessage)));
      ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    }
  }

  Future<void> _toggleTemplate(RwNotificationTemplate t) async {
    setState(() => _templateBusy = true);
    try {
      await ref.read(rwApiClientProvider).toggleNotificationTemplate(widget.rwId, t.id);
      ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _templateBusy = false);
    }
  }

  Future<void> _deleteTemplate(RwNotificationTemplate t) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hapus Template?'),
        content: Text('Apakah Anda yakin ingin menghapus template "${t.name}"? Tindakan ini tidak dapat dibatalkan.'),
        actions: [
          ElevatedButton(
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.danger,
              foregroundColor: Colors.white,
              elevation: 0,
            ),
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            key: const Key('rwTemplateConfirmDeleteBtn'),
            style: FilledButton.styleFrom(backgroundColor: AppColors.danger),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    setState(() => _templateBusy = true);
    try {
      await ref.read(rwApiClientProvider).deleteNotificationTemplate(widget.rwId, t.id);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Template berhasil dihapus')));
      ref.invalidate(rwNotificationTemplatesProvider(widget.rwId));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _templateBusy = false);
    }
  }

  Future<void> _previewTemplate(RwNotificationTemplate t, bool canWrite) async {
    showDialog(
      context: context,
      builder: (ctx) => FutureBuilder<Map<String, dynamic>>(
        future: ref.read(rwApiClientProvider).previewNotificationTemplate(widget.rwId, t.id),
        builder: (ctx, snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const AlertDialog(
              content: SizedBox(
                height: 100,
                child: Center(child: CircularProgressIndicator()),
              ),
            );
          }
          final theme = Theme.of(ctx);
          final colorScheme = theme.colorScheme;
          final isDark = theme.brightness == Brightness.dark;
          final data = snap.data;
          final renderedTitle = data?['rendered_title'] as String? ?? t.title;
          final renderedBody = data?['rendered_body'] as String? ?? t.body;
          final sampleData = (data?['sample_data'] as Map?)?.cast<String, dynamic>() ?? const {};
          final placeholders = (data?['placeholders'] as List?)?.map((e) => '$e').toList() ?? const [];

          return AlertDialog(
            title: Row(
              children: [
                const Icon(Icons.preview_rounded, size: 20),
                const SizedBox(width: 8),
                Expanded(child: Text('Preview: ${t.name}', style: const TextStyle(fontSize: 16))),
              ],
            ),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Wrap(
                    spacing: 8,
                    runSpacing: 4,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      Chip(
                        label: Text(t.eventType, style: const TextStyle(fontSize: 11)),
                        padding: EdgeInsets.zero,
                        materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      ),
                      Text(
                        placeholders.isNotEmpty
                            ? '${placeholders.length} variabel'
                            : 'Tanpa variabel dinamis',
                        style: TextStyle(fontSize: 11, color: colorScheme.onSurfaceVariant),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: const Color(0xFF0F172A),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: const Color(0xFF334155)),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              width: 20,
                              height: 20,
                              decoration: BoxDecoration(
                                color: Colors.blue.shade600,
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: const Icon(Icons.notifications_active, size: 12, color: Colors.white),
                            ),
                            const SizedBox(width: 6),
                            const Expanded(
                              child: Text(
                                'SOCIAL FINANCE URENA • RW',
                                style: TextStyle(color: Color(0xFF94A3B8), fontSize: 10, fontWeight: FontWeight.bold),
                              ),
                            ),
                            const Text('Baru saja', style: TextStyle(color: Color(0xFF64748B), fontSize: 10)),
                          ],
                        ),
                        const SizedBox(height: 8),
                        Text(
                          renderedTitle,
                          style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.bold),
                        ),
                        const SizedBox(height: 4),
                        _buildBodyWithLinks(renderedBody),
                        if (t.imageUrl != null && t.imageUrl!.isNotEmpty) ...[
                          const SizedBox(height: 10),
                          ClipRRect(
                            borderRadius: BorderRadius.circular(8),
                            child: ConstrainedBox(
                              constraints: const BoxConstraints(maxHeight: 180),
                              child: AuthenticatedNetworkImage(
                                imageUrl: ApiConfig.resolveImageUrl(
                                      t.imageUrl!,
                                      fallbackBaseUrl: ref.read(apiClientProvider).baseUrl,
                                    ) ??
                                    t.imageUrl!,
                                width: double.infinity,
                                fit: BoxFit.contain,
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  if (sampleData.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    Text(
                      'Data Contoh Render:',
                      style: TextStyle(fontSize: 11, fontWeight: FontWeight.bold, color: colorScheme.onSurface),
                    ),
                    const SizedBox(height: 4),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: sampleData.entries.map((e) => Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          '{{${e.key}}}: ${e.value}',
                          style: TextStyle(fontSize: 10, color: colorScheme.onSurfaceVariant),
                        ),
                      )).toList(),
                    ),
                  ],
                ],
              ),
            ),
            actions: [
              AppButton.danger(
                text: 'Tutup',
                onPressed: () => Navigator.pop(ctx),
              ),
              if (canWrite && t.isActive)
                FilledButton.icon(
                  onPressed: () {
                    Navigator.pop(ctx);
                    _sendTemplate(t);
                  },
                  icon: const Icon(Icons.send_rounded, size: 16),
                  label: const Text('Lanjut Kirim'),
                ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _sendTemplate(RwNotificationTemplate t) async {
    String target = 'all_pengurus';
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setS) {
          final theme = Theme.of(ctx);
          final colorScheme = theme.colorScheme;
          final isDark = theme.brightness == Brightness.dark;
          return AlertDialog(
            title: Text('Kirim: ${t.name}'),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('Target Penerima:', style: TextStyle(fontSize: 12, fontWeight: FontWeight.bold)),
                const SizedBox(height: 6),
                DropdownButtonFormField<String>(
                  initialValue: target,
                  decoration: const InputDecoration(border: OutlineInputBorder(), isDense: true),
                  items: const [
                    DropdownMenuItem(value: 'all_pengurus', child: Text('Semua Pengurus (RW & RT)')),
                    DropdownMenuItem(value: 'rw_only', child: Text('Pengurus RW Saja')),
                  ],
                  onChanged: (v) => setS(() => target = v ?? 'all_pengurus'),
                ),
                const SizedBox(height: 12),
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle,
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(color: isDark ? AppColors.darkBorder : AppColors.lightBorder),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(t.title, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13)),
                      const SizedBox(height: 4),
                      _buildBodyWithLinksDialog(t.body),
                    ],
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'Notifikasi akan dikirim ke target terpilih dalam cakupan RW ini.',
                  style: TextStyle(fontSize: 11, color: colorScheme.onSurfaceVariant),
                ),
              ],
            ),
            actions: [
              ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.danger,
                  foregroundColor: Colors.white,
                  elevation: 0,
                ),
                onPressed: () => Navigator.pop(ctx, false),
                child: const Text('Batal'),
              ),
              FilledButton.icon(
                onPressed: () => Navigator.pop(ctx, true),
                icon: const Icon(Icons.send_rounded, size: 16),
                label: const Text('Kirim Sekarang'),
              ),
            ],
          );
        },
      ),
    );
    if (ok != true) return;
    setState(() => _sending = true);
    try {
      final count = await ref.read(rwApiClientProvider).sendTemplateNotification(widget.rwId, t.id, target: target);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Notifikasi terkirim ($count penerima)')));
      await _refresh();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(_friendlyError(e))));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final canWrite = _canWrite(user);
    final logsAsync = ref.watch(rwNotificationLogsProvider((rwId: widget.rwId, page: _page, eventType: null)));
    final templatesAsync = ref.watch(rwNotificationTemplatesProvider(widget.rwId));
    final templateCount = templatesAsync.valueOrNull?.length;
    final rwMemberships = (user?['rw_memberships'] as List?)
            ?.whereType<Map<String, dynamic>>()
            .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
            .toList() ??
        const <RWMembership>[];
    final isRwBadgeEligible = isRwUnreadBadgeEligible(
      user,
      rwMemberships: rwMemberships,
      rwId: widget.rwId,
    );
    final hasRiwayatUnread = isRwBadgeEligible &&
        widget.rwId.isNotEmpty &&
        ref.watch(rwTabUnreadStateProvider(widget.rwId)).hasNotifikasiUnread;
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Material(
      type: MaterialType.transparency,
      child: Column(
        children: [
          // TabBar directly below parent Data RW tabs (consistent with Notifikasi RT)
          TabBar(
            key: const Key('rwNotifSubTabBar'),
            controller: _tabController,
            tabAlignment: TabAlignment.fill,
            labelPadding: EdgeInsets.zero,
            labelStyle: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
            unselectedLabelStyle: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500),
            labelColor: AppColors.accent,
            unselectedLabelColor: isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
            indicatorColor: AppColors.accent,
            indicatorWeight: 3,
            tabs: [
              Tab(
                icon: const Icon(Icons.bookmark_border_rounded, size: 18),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Flexible(
                      child: Text(
                        'Template',
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    if (templateCount != null) ...[
                      const SizedBox(width: 6),
                      Container(
                        key: const Key('rwNotifTemplateCountBadge'),
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                        decoration: BoxDecoration(
                          color: AppColors.accent.withValues(alpha: isDark ? 0.22 : 0.12),
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: Text(
                          '($templateCount)',
                          style: const TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              Tab(
                icon: hasRiwayatUnread
                    ? Stack(
                        clipBehavior: Clip.none,
                        children: [
                          const Icon(Icons.history_outlined, size: 18),
                          Positioned(
                            top: -2,
                            right: -4,
                            child: Container(
                              key: const Key('rw-subtab-riwayat-unread-dot'),
                              width: 8,
                              height: 8,
                              decoration: BoxDecoration(
                                color: AppColors.danger,
                                shape: BoxShape.circle,
                                border: Border.all(
                                  color: theme.scaffoldBackgroundColor,
                                  width: 1.5,
                                ),
                              ),
                            ),
                          ),
                        ],
                      )
                    : const Icon(Icons.history_outlined, size: 18),
                text: 'Riwayat Notifikasi Sistem',
              ),
            ],
          ),
          Divider(
            height: 1,
            thickness: 1,
            color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
          ),
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                // TAB 0: TEMPLATE
                RefreshIndicator(
                  onRefresh: _refresh,
                  child: templatesAsync.when(
                    loading: () => ListView(
                      padding: const EdgeInsets.all(24),
                      children: const [Center(child: CircularProgressIndicator())],
                    ),
                    error: (e, _) => ListView(
                      padding: const EdgeInsets.all(16),
                      children: [_errorBox(e)],
                    ),
                    data: (items) {
                      return ListView(
                        padding: const EdgeInsets.all(16),
                        children: [
                          if (canWrite)
                            Padding(
                              padding: const EdgeInsets.only(bottom: 12),
                              child: Wrap(
                                spacing: 8,
                                runSpacing: 8,
                                children: [
                                  ElevatedButton.icon(
                                    key: const Key('rwNotifCreateTemplateBtn'),
                                    onPressed: _templateBusy ? null : _createTemplate,
                                    icon: const Icon(Icons.add, size: 16),
                                    label: const Text(
                                      'Tambah Template',
                                      style: TextStyle(fontSize: 12),
                                    ),
                                    style: ElevatedButton.styleFrom(
                                      backgroundColor: AppColors.accent,
                                      foregroundColor: Colors.white,
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 12,
                                        vertical: 8,
                                      ),
                                    ),
                                  ),
                                  OutlinedButton.icon(
                                    key: const Key('rwNotifManualPushBtn'),
                                    onPressed: _sending ? null : _sendManualPush,
                                    icon: const Icon(Icons.send, size: 15),
                                    label: const Text(
                                      'Kirim Manual',
                                      style: TextStyle(fontSize: 12),
                                    ),
                                    style: OutlinedButton.styleFrom(
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 12,
                                        vertical: 8,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          if (items.isEmpty)
                            _EmptyBox(
                              message: canWrite
                                  ? 'Belum ada template. Buat template baru.'
                                  : 'Belum ada template yang tersedia.',
                              action: canWrite
                                  ? ElevatedButton.icon(
                                      onPressed: _templateBusy ? null : _createTemplate,
                                      icon: const Icon(Icons.add, size: 16),
                                      label: const Text('Tambah Template'),
                                      style: ElevatedButton.styleFrom(
                                        backgroundColor: AppColors.accent,
                                        foregroundColor: Colors.white,
                                      ),
                                    )
                                  : null,
                            )
                          else
                            ...items.map((t) => _templateCard(t, canWrite, theme, isDark)),
                        ],
                      );
                    },
                  ),
                ),

                // TAB 1: RIWAYAT NOTIFIKASI SISTEM
                RefreshIndicator(
                  onRefresh: _refresh,
                  child: ListView(
                    padding: const EdgeInsets.all(16),
                    children: [
                      _sectionTitle('Riwayat Notifikasi Sistem'),
                      logsAsync.when(
                        loading: () => const Padding(
                          padding: EdgeInsets.all(24),
                          child: Center(child: CircularProgressIndicator()),
                        ),
                        error: (e, _) => _errorBox(e),
                        data: (pageData) {
                          if (pageData.items.isEmpty) {
                            return const _EmptyBox(message: 'Belum ada notifikasi untuk RW ini.');
                          }
                          return Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              ...pageData.items.map((n) => SizedBox(width: double.infinity, child: _logTile(n))),
                              if (pageData.totalPages > 1)
                                Row(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    IconButton(
                                      onPressed: _page > 1 ? () => setState(() => _page--) : null,
                                      icon: const Icon(Icons.chevron_left),
                                    ),
                                    Text('$_page / ${pageData.totalPages}'),
                                    IconButton(
                                      onPressed: _page < pageData.totalPages ? () => setState(() => _page++) : null,
                                      icon: const Icon(Icons.chevron_right),
                                    ),
                                  ],
                                ),
                            ],
                          );
                        },
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _sectionTitle(String t) => Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Text(t, style: const TextStyle(fontWeight: FontWeight.w700)),
      );

  Widget _errorBox(Object e) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.danger.withValues(alpha: isDark ? 0.16 : 0.08),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.danger.withValues(alpha: 0.35)),
      ),
      child: Row(
        children: [
          const Icon(Icons.error_outline, color: AppColors.danger),
          const SizedBox(width: 8),
          Expanded(child: Text(_friendlyError(e))),
          TextButton(onPressed: _refresh, child: const Text('Coba lagi')),
        ],
      ),
    );
  }

  Widget _buildDeliveryStatusBadge(String? status) {
    final s = (status ?? '').trim().toUpperCase();
    if (s.isEmpty) return const SizedBox.shrink();

    final label = switch (s) {
      'SENT' => 'TERKIRIM',
      'APPROVED' => 'DISETUJUI',
      'RECORDED' => 'TERCATAT',
      'FAILED' => 'GAGAL',
      'REJECTED' => 'DITOLAK',
      'PARTIAL' => 'SEBAGIAN',
      'NO_DEVICE' => 'TANPA PERANGKAT',
      'PENDING' => 'TERTUNDA',
      _ => s,
    };

    final color = switch (s) {
      'SENT' || 'APPROVED' || 'RECORDED' => AppColors.success,
      'FAILED' || 'REJECTED' => AppColors.danger,
      'PARTIAL' || 'NO_DEVICE' || 'PENDING' => AppColors.warning,
      _ => AppColors.lightTextMuted,
    };

    return AppBadge(
      label: label,
      backgroundColor: color,
      textColor: Colors.white,
    );
  }

  Widget _previewMetaRow(String label, String? textValue, {Widget? widgetValue}) {
    final muted = Theme.of(context).colorScheme.onSurfaceVariant;
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          SizedBox(
            width: 120,
            child: Text(
              label,
              style: TextStyle(fontSize: 12, color: muted),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: widgetValue ??
                Text(
                  textValue ?? '-',
                  style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500),
                ),
          ),
        ],
      ),
    );
  }

  void _openFullScreenImage(BuildContext context, String imageUrl) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) {
        return Dialog(
          backgroundColor: Colors.black.withValues(alpha: 0.92),
          insetPadding: const EdgeInsets.all(12),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.base),
          ),
          child: Stack(
            children: [
              InteractiveViewer(
                minScale: 0.8,
                maxScale: 4.0,
                child: Center(
                  child: AuthenticatedNetworkImage(
                    imageUrl: imageUrl,
                    fit: BoxFit.contain,
                    errorWidget: const Padding(
                      padding: EdgeInsets.all(24),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.broken_image_outlined,
                            color: Colors.white70,
                            size: 48,
                          ),
                          SizedBox(height: 12),
                          Text(
                            'Gambar lampiran tidak dapat dimuat.',
                            style: TextStyle(color: Colors.white70),
                            textAlign: TextAlign.center,
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
              Positioned(
                top: 8,
                right: 8,
                child: IconButton(
                  key: const Key('close_fullscreen_image_button'),
                  icon: const Icon(Icons.close, color: Colors.white),
                  tooltip: 'Tutup Gambar',
                  onPressed: () => Navigator.of(dialogContext).pop(),
                ),
              ),
            ],
          ),
        );
      },
    );
  }

  void _showNotificationPreview(RwNotificationLog n) {
    _markRwLogReadAndSync(n);
    final title = n.title?.isNotEmpty == true ? n.title! : n.eventType;
    final body = n.body ?? '';
    final dateStr = n.createdAt != null ? formatJakartaDetailDateTime(n.createdAt) : '-';
    final isRw = n.source?.toLowerCase() != 'rt';
    final rawImage = (n.imageUrl?.trim().isNotEmpty == true)
        ? n.imageUrl!.trim()
        : ((n.payload?['image_url'] as String?)?.trim().isNotEmpty == true
            ? (n.payload!['image_url'] as String).trim()
            : null);
    final resolvedImageUrl = rawImage != null
        ? ApiConfig.resolveImageUrl(
            rawImage,
            fallbackBaseUrl: ref.read(apiClientProvider).baseUrl,
          )
        : null;
    final linkUrl = n.url ?? (n.payload?['deep_link'] as String?);

    showDialog<void>(
      context: context,
      builder: (ctx) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
          titlePadding: const EdgeInsets.fromLTRB(20, 20, 20, 12),
          contentPadding: const EdgeInsets.symmetric(horizontal: 20),
          title: Row(
            children: [
              const Icon(Icons.preview_rounded, size: 22, color: AppColors.accent),
              const SizedBox(width: 8),
              const Expanded(
                child: Text(
                  'Preview Notifikasi',
                  style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
                ),
              ),
              IconButton(
                icon: const Icon(Icons.close, size: 20),
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
                onPressed: () => Navigator.pop(ctx),
              ),
            ],
          ),
          content: SizedBox(
            width: double.maxFinite,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: const Color(0xFF0F172A),
                      borderRadius: BorderRadius.circular(12),
                      border: Border.all(color: const Color(0xFF334155)),
                      boxShadow: [
                        BoxShadow(
                          color: Colors.black.withValues(alpha: 0.3),
                          blurRadius: 8,
                          offset: const Offset(0, 3),
                        ),
                      ],
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Container(
                              width: 22,
                              height: 22,
                              decoration: BoxDecoration(
                                color: isRw ? const Color(0xFF7C3AED) : AppColors.accent,
                                borderRadius: BorderRadius.circular(6),
                              ),
                              child: const Icon(Icons.notifications_active, size: 13, color: Colors.white),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                isRw ? 'SOCIAL FINANCE URENA • RW' : 'SOCIAL FINANCE URENA • RT',
                                style: const TextStyle(
                                  color: Color(0xFF94A3B8),
                                  fontSize: 10,
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                            ),
                            const Text('Baru saja', style: TextStyle(color: Color(0xFF64748B), fontSize: 10)),
                          ],
                        ),
                        const SizedBox(height: 10),
                        Text(
                          title,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 14,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 4),
                        _buildBodyWithLinks(body),
                        if (resolvedImageUrl != null && resolvedImageUrl.isNotEmpty) ...[
                          const SizedBox(height: 10),
                          GestureDetector(
                            key: const Key('preview_notification_image_thumbnail'),
                            onTap: () => _openFullScreenImage(context, resolvedImageUrl),
                            child: ClipRRect(
                              borderRadius: BorderRadius.circular(8),
                              child: ConstrainedBox(
                                constraints: const BoxConstraints(maxHeight: 180),
                                child: Stack(
                                  alignment: Alignment.bottomRight,
                                  children: [
                                    AuthenticatedNetworkImage(
                                      imageUrl: resolvedImageUrl,
                                      width: double.infinity,
                                      fit: BoxFit.contain,
                                    ),
                                    Container(
                                      margin: const EdgeInsets.all(6),
                                      padding: const EdgeInsets.all(4),
                                      decoration: BoxDecoration(
                                        color: Colors.black54,
                                        borderRadius: BorderRadius.circular(4),
                                      ),
                                      child: const Icon(
                                        Icons.fullscreen,
                                        size: 16,
                                        color: Colors.white,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  const SizedBox(height: 16),
                  const Text('Detail Informasi', style: TextStyle(fontSize: 13, fontWeight: FontWeight.bold)),
                  const SizedBox(height: 8),
                  _previewMetaRow('Waktu / Jadwal', dateStr),
                  _previewMetaRow(
                    'Sumber',
                    null,
                    widgetValue: AppBadge(
                      label: isRw ? 'RW' : 'RT',
                      backgroundColor: isRw ? const Color(0xFF7C3AED) : AppColors.accent,
                      textColor: Colors.white,
                    ),
                  ),
                  if (n.status != null && n.status!.isNotEmpty)
                    _previewMetaRow(
                      'Status Notifikasi',
                      null,
                      widgetValue: _buildDeliveryStatusBadge(n.status),
                    ),
                  if (n.deliveryCountsAvailable)
                    _previewMetaRow(
                      'Penerima',
                      '${n.sentCount} / ${n.recipientsCount} penerima terkirim'
                          '${n.failedCount > 0 ? ' (${n.failedCount} gagal)' : ''}',
                    ),
                  if (linkUrl != null && linkUrl.isNotEmpty) ...[
                    const SizedBox(height: 6),
                    InkWell(
                      onTap: () => UrlDetector.openUrl(linkUrl),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 4),
                        child: Row(
                          children: [
                            const Icon(Icons.link, size: 16, color: AppColors.accent),
                            const SizedBox(width: 6),
                            Expanded(
                              child: Text(
                                linkUrl,
                                style: const TextStyle(
                                  color: AppColors.accent,
                                  decoration: TextDecoration.underline,
                                  fontSize: 12,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
          actions: [
            AppButton.danger(
              text: 'Tutup',
              onPressed: () => Navigator.pop(ctx),
            ),
          ],
        );
      },
    );
  }

  Widget _logTile(RwNotificationLog n) {
    final dateStr = n.createdAt != null ? formatJakartaDateTime(n.createdAt) : '-';
    final isRw = n.source?.toLowerCase() != 'rt';
    final muted = Theme.of(context).colorScheme.onSurfaceVariant;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: InkWell(
        key: Key('rwNotifLog_${n.id}'),
        borderRadius: BorderRadius.circular(AppRadius.base),
        onTap: () => _showNotificationPreview(n),
        child: SizedBox(
          width: double.infinity,
          child: AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Wrap(
                  alignment: WrapAlignment.spaceBetween,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  runSpacing: 4,
                  children: [
                    Text(
                      dateStr,
                      style: TextStyle(fontSize: 12, color: muted),
                    ),
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(right: 6),
                          child: AppBadge(
                            label: isRw ? 'RW' : 'RT',
                            backgroundColor: isRw ? const Color(0xFF7C3AED) : AppColors.accent,
                            textColor: Colors.white,
                          ),
                        ),
                        _buildDeliveryStatusBadge(n.status),
                      ],
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                Text(
                  n.title?.isNotEmpty == true ? n.title! : n.eventType,
                  style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                ),
                if (n.body != null && n.body!.isNotEmpty) ...[
                  const SizedBox(height: 4),
                  Text(
                    n.body!,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 13, color: Theme.of(context).colorScheme.onSurface),
                  ),
                ],
                if (n.deliveryCountsAvailable) ...[
                  const SizedBox(height: 6),
                  Text(
                    '${n.sentCount} / ${n.recipientsCount} penerima terkirim',
                    style: TextStyle(fontSize: 11, color: muted),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _templateCard(
    RwNotificationTemplate t,
    bool canWrite,
    ThemeData theme,
    bool isDark,
  ) {
    final muted = isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted;
    final hasImage = t.imageUrl != null && t.imageUrl!.isNotEmpty;

    return Padding(
      key: Key('rwNotifTemplate_${t.id}'),
      padding: const EdgeInsets.only(bottom: 12),
      child: AppCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                Expanded(
                  child: Text(
                    t.name,
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                if (canWrite)
                  Switch(
                    key: Key('rwNotifTplToggle_${t.id}'),
                    value: t.isActive,
                    activeThumbColor: AppColors.accent,
                    onChanged: _templateBusy ? null : (_) => _toggleTemplate(t),
                  )
                else
                  AppBadge(
                    label: t.isActive ? 'Aktif' : 'Nonaktif',
                    backgroundColor: t.isActive
                        ? AppColors.success.withValues(alpha: isDark ? 0.2 : 0.14)
                        : (isDark ? AppColors.darkSurfaceSubtle : AppColors.lightSurfaceSubtle),
                    textColor: t.isActive ? AppColors.success : muted,
                  ),
              ],
            ),
            const SizedBox(height: 4),
            Wrap(
              spacing: 6,
              runSpacing: 4,
              children: [
                AppBadge(
                  label: _eventLabel(t.eventType),
                  backgroundColor: AppColors.accent.withValues(alpha: isDark ? 0.18 : 0.12),
                  textColor: AppColors.accent,
                ),
                if (hasImage)
                  AppBadge(
                    label: 'Gambar Terlampir',
                    backgroundColor: AppColors.infoBlue.withValues(alpha: isDark ? 0.2 : 0.14),
                    textColor: AppColors.infoBlue,
                  ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              t.title,
              style: theme.textTheme.bodyMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 2),
            Text(
              t.body,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodySmall?.copyWith(color: muted),
            ),
            if (hasImage) ...[
              const SizedBox(height: 10),
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: AuthenticatedNetworkImage(
                  imageUrl: ApiConfig.resolveImageUrl(t.imageUrl!) ?? t.imageUrl!,
                  height: 120,
                  width: double.infinity,
                  fit: BoxFit.cover,
                ),
              ),
            ],
            const SizedBox(height: 8),
            // Responsive action row (consistent with NotificationManagementScreen, safe on small screens)
            Align(
              alignment: Alignment.centerRight,
              child: Wrap(
                spacing: 4,
                runSpacing: 4,
                alignment: WrapAlignment.end,
                children: [
                  IconButton(
                    key: Key('rwNotifTplPreview_${t.id}'),
                    icon: const Icon(Icons.preview_rounded, size: 20, color: AppColors.infoBlue),
                    tooltip: 'Preview',
                    onPressed: () => _previewTemplate(t, canWrite),
                  ),
                  if (canWrite) ...[
                    IconButton(
                      key: Key('rwNotifTplSend_${t.id}'),
                      icon: Icon(
                        Icons.send_rounded,
                        size: 20,
                        color: (!t.isActive || _sending) ? muted : AppColors.infoBlue,
                      ),
                      tooltip: 'Kirim',
                      onPressed: (!t.isActive || _sending) ? null : () => _sendTemplate(t),
                    ),
                    IconButton(
                      key: Key('rwNotifTplEdit_${t.id}'),
                      icon: const Icon(Icons.edit_outlined, size: 20, color: AppColors.accent),
                      tooltip: 'Ubah',
                      onPressed: _templateBusy ? null : () => _showTemplateFormDialog(template: t),
                    ),
                    IconButton(
                      key: Key('rwNotifTplDelete_${t.id}'),
                      icon: const Icon(Icons.delete_outline, size: 20, color: AppColors.danger),
                      tooltip: 'Hapus',
                      onPressed: _templateBusy ? null : () => _deleteTemplate(t),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _EmptyBox extends StatelessWidget {
  final String message;
  final Widget? action;
  const _EmptyBox({required this.message, this.action});
  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(24),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        border: Border.all(color: colorScheme.outlineVariant),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            message,
            textAlign: TextAlign.center,
            style: TextStyle(color: colorScheme.onSurfaceVariant),
          ),
          if (action != null) ...[
            const SizedBox(height: 12),
            action!,
          ],
        ],
      ),
    );
  }
}
