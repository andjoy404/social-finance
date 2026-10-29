import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/error_state.dart';
import 'package:social_finance/core/widgets/loading_indicator.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import '../../data/models/notification_models.dart';
import '../../data/notification_repository.dart';
import 'notification_detail_screen.dart';

class NotificationManagementScreen extends ConsumerStatefulWidget {
  const NotificationManagementScreen({super.key});

  @override
  ConsumerState<NotificationManagementScreen> createState() =>
      _NotificationManagementScreenState();
}

class _NotificationManagementScreenState
    extends ConsumerState<NotificationManagementScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  // Logs state
  List<NotificationLogEntry> _logs = [];
  bool _isLoadingLogs = false;
  String? _logsError;
  String _selectedLogEventType = '';

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _loadLogs();
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadLogs() async {
    setState(() {
      _isLoadingLogs = true;
      _logsError = null;
    });
    try {
      final repo = ref.read(notificationRepositoryProvider);
      final resp = await repo.fetchLogs(
        page: 1,
        perPage: 50,
        eventType: _selectedLogEventType.isNotEmpty ? _selectedLogEventType : null,
      );
      if (mounted) {
        setState(() {
          _logs = resp.data;
          _isLoadingLogs = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _logsError = e.toString();
          _isLoadingLogs = false;
        });
      }
    }
  }

  void _openLogDetail(NotificationLogEntry log) {
    final id = log.id.trim();
    final router = GoRouter.maybeOf(context);
    if (router != null) {
      final path = id.isNotEmpty
          ? '/home/notifikasi/detail/$id'
          : '/home/notifikasi/detail';
      final queryParams = <String, String>{
        'from': 'notifikasi',
        if (log.title.trim().isNotEmpty) 'title': log.title.trim(),
        if (log.body.trim().isNotEmpty) 'body': log.body.trim(),
        if (log.eventType.trim().isNotEmpty) 'type': log.eventType.trim(),
        if (log.imageUrl != null && log.imageUrl!.trim().isNotEmpty)
          'image_url': log.imageUrl!.trim(),
        if (log.createdAt != null)
          'sent_at': log.createdAt!.toIso8601String(),
      };
      final uri = Uri(path: path, queryParameters: queryParams);
      router.push(uri.toString());
      return;
    }

    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => NotificationDetailScreen(
          notificationId: id.isNotEmpty ? id : null,
          initialTitle: log.title,
          initialBody: log.body,
          initialImageUrl: log.imageUrl,
          initialEventType: log.eventType,
          initialSentAt: log.createdAt?.toIso8601String(),
          openedFromNotificationList: true,
        ),
      ),
    );
  }

  void _showPreviewBanner({
    required String title,
    required String body,
    required String eventName,
    String? imageUrl,
  }) {
    Navigator.of(context).push(
      MaterialPageRoute(
        fullscreenDialog: true,
        builder: (ctx) {
          return Scaffold(
            backgroundColor: const Color(0xFF0F172A),
            appBar: AppBar(
              backgroundColor: const Color(0xFF0F172A),
              elevation: 0,
              leading: IconButton(
                icon: const Icon(Icons.arrow_back_ios_new, color: Colors.white, size: 20),
                onPressed: () => Navigator.of(ctx).pop(),
              ),
              title: Text(
                'Preview Notifikasi',
                style: const TextStyle(color: Colors.white, fontSize: 16),
              ),
              centerTitle: true,
            ),
            body: SingleChildScrollView(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Banner container
                    Container(
                      padding: const EdgeInsets.all(16),
                      decoration: BoxDecoration(
                        color: const Color(0xFF1E293B),
                        borderRadius: BorderRadius.circular(16),
                        border: Border.all(
                          color: const Color(0xFF334155),
                          width: 1,
                        ),
                        boxShadow: [
                          BoxShadow(
                            color: Colors.black.withValues(alpha: 0.5),
                            blurRadius: 16,
                            offset: const Offset(0, 6),
                          ),
                        ],
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          // Banner header
                          Row(
                            children: [
                              Container(
                                width: 28,
                                height: 28,
                                decoration: BoxDecoration(
                                  color: AppColors.accent,
                                  borderRadius: BorderRadius.circular(8),
                                ),
                                child: const Icon(
                                  Icons.notifications_active,
                                  size: 16,
                                  color: Colors.white,
                                ),
                              ),
                              const SizedBox(width: 10),
                              const Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      'Social Finance Urena',
                                      style: TextStyle(
                                        color: Colors.white,
                                        fontSize: 12,
                                        fontWeight: FontWeight.bold,
                                      ),
                                    ),
                                    Text(
                                      'Push Notification',
                                      style: TextStyle(
                                        color: Color(0xFF94A3B8),
                                        fontSize: 10,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              const Text(
                                'baru saja',
                                style: TextStyle(
                                  color: Color(0xFF64748B),
                                  fontSize: 11,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 12),
                          // Content
                          Text(
                            title,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 14,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            body,
                            style: const TextStyle(
                              color: Color(0xFFCBD5E1),
                              fontSize: 13,
                              height: 1.35,
                            ),
                          ),
                          if (imageUrl != null && imageUrl.isNotEmpty) ...[
                            const SizedBox(height: 10),
                            ClipRRect(
                              borderRadius: BorderRadius.circular(8),
                              child: Image.network(
                                ApiConfig.resolveImageUrl(imageUrl) ?? imageUrl,
                                width: double.infinity,
                                fit: BoxFit.cover,
                                errorBuilder: (_, _, _) => const SizedBox.shrink(),
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                    const SizedBox(height: 24),
                    // Label info
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                      decoration: BoxDecoration(
                        color: AppColors.accent.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(Icons.info_outline, size: 16, color: AppColors.accent),
                          const SizedBox(width: 8),
                          Text(
                            eventName,
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                              color: AppColors.accent,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  void _confirmSendBroadcastDialog({
    required String eventType,
    required String title,
    required String body,
    String? imageUrl,
  }) {
    showDialog(
      context: context,
      builder: (ctx) {
        return AlertDialog(
          title: const Text('Konfirmasi Kirim Notifikasi Siaran'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Apakah Anda yakin ingin menyiarkan notifikasi ini ke SELURUH warga di RT Anda?',
                style: TextStyle(fontSize: 14),
              ),
              const SizedBox(height: 12),
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: Colors.black.withValues(alpha: 0.05),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title,
                        style: const TextStyle(
                            fontWeight: FontWeight.bold, fontSize: 13)),
                    const SizedBox(height: 4),
                    Text(body, style: const TextStyle(fontSize: 12)),
                  ],
                ),
              ),
              const SizedBox(height: 10),
              const Text(
                'Peringatan: Pesan ini akan langsung dikirimkan ke perangkat warga.',
                style: TextStyle(fontSize: 11, color: AppColors.warning),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Batal'),
            ),
            ElevatedButton(
              onPressed: () async {
                Navigator.of(ctx).pop();
                final messenger = ScaffoldMessenger.of(context);
                try {
                  final repo = ref.read(notificationRepositoryProvider);
                  await repo.sendManualPush(
                    title: title,
                    body: body,
                    imageUrl: imageUrl,
                  );
                  messenger.showSnackBar(
                    const SnackBar(
                        content: Text(
                            'Notifikasi berhasil disiarkan ke seluruh warga RT!')),
                  );
                } catch (e) {
                  messenger.showSnackBar(
                    SnackBar(content: Text('Gagal menyiarkan: $e')),
                  );
                }
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.success,
                foregroundColor: Colors.white,
              ),
              child: const Text('Ya, Kirim Sekarang'),
            ),
          ],
        );
      },
    );
  }

  Future<void> _handleTestPush({
    required String eventType,
    required String title,
    required String body,
    String? imageUrl,
  }) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      final repo = ref.read(notificationRepositoryProvider);
      final count = await repo.sendTestPush(
        eventType: eventType,
        title: title,
        body: body,
        imageUrl: imageUrl,
      );
      messenger.showSnackBar(
        SnackBar(
            content: Text(
                'Uji notifikasi berhasil dikirim ($count perangkat aktif Anda).')),
      );
    } catch (e) {
      messenger.showSnackBar(
        SnackBar(content: Text('Gagal uji notifikasi: $e')),
      );
    }
  }

  void _showAddEditTemplateDialog({
    NotificationTemplate? template,
    String? defaultEventType,
    List<SystemNotificationType>? systemTypes,
  }) {
    final isEditing = template != null;
    final nameController = TextEditingController(text: template?.name ?? '');
    final titleController = TextEditingController(text: template?.title ?? '');
    final bodyController = TextEditingController(text: template?.body ?? '');
    final imageController = TextEditingController(text: template?.imageUrl ?? '');

    String selectedEvent = template?.eventType ??
        defaultEventType ??
        (systemTypes?.isNotEmpty == true ? systemTypes!.first.eventType : 'iuran_bill_created');

    if (!isEditing && (template?.title == null || template!.title.isEmpty)) {
      final match = systemTypes?.firstWhere(
        (st) => st.eventType == selectedEvent,
        orElse: () => const SystemNotificationType(
          eventType: '',
          name: '',
          trigger: '',
          recipientsRule: '',
          defaultTitle: '',
          defaultBody: '',
          allowedVariables: [],
        ),
      );
      if (match != null && match.eventType.isNotEmpty) {
        titleController.text = match.defaultTitle;
        bodyController.text = match.defaultBody;
        if (nameController.text.isEmpty) {
          nameController.text = 'Template ${match.name}';
        }
      }
    }

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (context, setSheetState) {
            final theme = Theme.of(context);
            final isDark = theme.brightness == Brightness.dark;

            final currentSystemType = systemTypes?.firstWhere(
              (st) => st.eventType == selectedEvent,
              orElse: () => const SystemNotificationType(
                eventType: '',
                name: '',
                trigger: '',
                recipientsRule: '',
                defaultTitle: '',
                defaultBody: '',
                allowedVariables: [],
              ),
            );

            return Padding(
              padding: EdgeInsets.only(
                bottom: MediaQuery.of(ctx).viewInsets.bottom,
              ),
              child: Container(
                constraints: BoxConstraints(
                  maxHeight: MediaQuery.of(ctx).size.height * 0.85,
                ),
                decoration: BoxDecoration(
                  color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
                  borderRadius: const BorderRadius.vertical(
                    top: Radius.circular(20),
                  ),
                ),
                padding: const EdgeInsets.all(20),
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Header
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            isEditing ? 'Ubah Template' : 'Buat Template Baru',
                            style: theme.textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          IconButton(
                            icon: const Icon(Icons.close),
                            onPressed: () => Navigator.of(ctx).pop(),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),

                      // Event type selector (only if creating)
                      if (!isEditing && systemTypes != null && systemTypes.isNotEmpty) ...[
                        Text(
                          'Jenis Notifikasi Sistem',
                          style: theme.textTheme.labelMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 12),
                          decoration: BoxDecoration(
                            border: Border.all(
                              color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
                            ),
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                          child: DropdownButtonHideUnderline(
                            child: DropdownButton<String>(
                              isExpanded: true,
                              value: selectedEvent,
                              items: systemTypes.map((st) {
                                return DropdownMenuItem<String>(
                                  value: st.eventType,
                                  child: Text(st.name, style: theme.textTheme.bodyMedium),
                                );
                              }).toList(),
                              onChanged: (val) {
                                if (val != null) {
                                  setSheetState(() {
                                    selectedEvent = val;
                                    final match = systemTypes.firstWhere((s) => s.eventType == val);
                                    nameController.text = 'Template ${match.name}';
                                    titleController.text = match.defaultTitle;
                                    bodyController.text = match.defaultBody;
                                  });
                                }
                              },
                            ),
                          ),
                        ),
                        const SizedBox(height: 14),
                      ],

                      // Name input
                      Text(
                        'Nama Template',
                        style: theme.textTheme.labelMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 6),
                      TextField(
                        controller: nameController,
                        decoration: InputDecoration(
                          hintText: 'Misal: Notifikasi Resmi Bulanan',
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                          contentPadding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 10,
                          ),
                        ),
                      ),
                      const SizedBox(height: 14),

                      // Title input
                      Text(
                        'Judul Pesan',
                        style: theme.textTheme.labelMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 6),
                      TextField(
                        controller: titleController,
                        decoration: InputDecoration(
                          hintText: 'Judul yang tampil di banner notifikasi',
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                          contentPadding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 10,
                          ),
                        ),
                      ),
                      const SizedBox(height: 14),

                      // Body input
                      Text(
                        'Isi Pesan',
                        style: theme.textTheme.labelMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 6),
                      TextField(
                        controller: bodyController,
                        maxLines: 3,
                        decoration: InputDecoration(
                          hintText: 'Gunakan variabel untuk data dinamis',
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                          ),
                          contentPadding: const EdgeInsets.all(12),
                        ),
                      ),
                      const SizedBox(height: 10),

                      // Image URL input (only for events supporting images)
                      if (selectedEvent == 'waste_truck_arrival' ||
                          selectedEvent == 'manual_push') ...[
                        Text(
                          'URL Gambar Lampiran (Opsional)',
                          style: theme.textTheme.labelMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: 6),
                        TextField(
                          controller: imageController,
                          decoration: InputDecoration(
                            hintText: 'https://... atau /api/v1/notifications/images/...',
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(AppRadius.base),
                            ),
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 10,
                            ),
                          ),
                        ),
                        const SizedBox(height: 10),
                      ],

                      // Allowed variable tags
                      if (currentSystemType != null &&
                          currentSystemType.allowedVariables.isNotEmpty) ...[
                        Text(
                          'Variabel Dinamis Tersedia (ketuk untuk menyisipkan):',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: 6),
                        Wrap(
                          spacing: 8,
                          runSpacing: 4,
                          children: currentSystemType.allowedVariables.map((v) {
                            final tag = '{{$v}}';
                            return ActionChip(
                              label: Text(tag, style: const TextStyle(fontSize: 12)),
                              backgroundColor: AppColors.accent.withValues(alpha: 0.12),
                              side: BorderSide.none,
                              onPressed: () {
                                final current = bodyController.text;
                                bodyController.text = '$current $tag'.trim();
                              },
                            );
                          }).toList(),
                        ),
                        const SizedBox(height: 16),
                      ],

                      // Actions
                      Row(
                        children: [
                          Expanded(
                            child: OutlinedButton(
                              onPressed: () => Navigator.of(ctx).pop(),
                              child: const Text('Batal'),
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: ElevatedButton(
                              onPressed: () async {
                                final name = nameController.text.trim();
                                final title = titleController.text.trim();
                                final body = bodyController.text.trim();
                                if (name.isEmpty || title.isEmpty || body.isEmpty) {
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(
                                      content: Text('Semua kolom wajib diisi'),
                                    ),
                                  );
                                  return;
                                }

                                final rawImg = imageController.text.trim();
                                final supportsImg = selectedEvent == 'waste_truck_arrival' || selectedEvent == 'manual_push';
                                final imgUrl = (supportsImg && rawImg.isNotEmpty) ? rawImg : null;

                                final messenger = ScaffoldMessenger.of(context);
                                try {
                                  final repo = ref.read(notificationRepositoryProvider);
                                  if (isEditing) {
                                    await repo.updateTemplate(
                                      id: template.id,
                                      name: name,
                                      title: title,
                                      body: body,
                                      imageUrl: imgUrl,
                                    );
                                  } else {
                                    await repo.createTemplate(
                                      eventType: selectedEvent,
                                      name: name,
                                      title: title,
                                      body: body,
                                      imageUrl: imgUrl,
                                    );
                                  }
                                  if (!mounted) return;
                                  ref.invalidate(notificationTemplatesProvider);
                                  if (ctx.mounted) {
                                    Navigator.of(ctx).pop();
                                  }
                                  messenger.showSnackBar(
                                    SnackBar(
                                      content: Text(
                                        isEditing
                                            ? 'Template berhasil diperbarui'
                                            : 'Template berhasil dibuat',
                                      ),
                                    ),
                                  );
                                } catch (e) {
                                  if (!mounted) return;
                                  messenger.showSnackBar(
                                    SnackBar(content: Text(e.toString())),
                                  );
                                }
                              },
                              style: ElevatedButton.styleFrom(
                                backgroundColor: AppColors.accent,
                                foregroundColor: Colors.white,
                              ),
                              child: Text(isEditing ? 'Simpan' : 'Buat'),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        );
      },
    );
  }

  void _confirmDeleteTemplate(NotificationTemplate template) {
    showDialog(
      context: context,
      builder: (ctx) {
        return AlertDialog(
          title: const Text('Hapus Template?'),
          content: Text(
            'Apakah Anda yakin ingin menghapus template "${template.name}"? '
            'Setelah dihapus, sistem akan otomatis menggunakan pesan default bawaan.',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Batal'),
            ),
            TextButton(
              onPressed: () async {
                Navigator.of(ctx).pop();
                final messenger = ScaffoldMessenger.of(context);
                try {
                  final repo = ref.read(notificationRepositoryProvider);
                  await repo.deleteTemplate(template.id);
                  if (!mounted) return;
                  ref.invalidate(notificationTemplatesProvider);
                  messenger.showSnackBar(
                    const SnackBar(content: Text('Template berhasil dihapus')),
                  );
                } catch (e) {
                  if (!mounted) return;
                  messenger.showSnackBar(
                    SnackBar(content: Text(e.toString())),
                  );
                }
              },
              style: TextButton.styleFrom(foregroundColor: AppColors.danger),
              child: const Text('Hapus'),
            ),
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final isSuperAdmin = isSuperAdminUser(user);

    final systemTypesAsync = ref.watch(notificationSystemTypesProvider);
    final templatesAsync = ref.watch(notificationTemplatesProvider);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: Navigator.of(context).canPop(),
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Kelola Notifikasi'),
        bottom: TabBar(
          controller: _tabController,
          labelColor: AppColors.accent,
          unselectedLabelColor: isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
          indicatorColor: AppColors.accent,
          indicatorWeight: 3,
          tabs: const [
            Tab(
              icon: Icon(Icons.apps_rounded, size: 20),
              text: 'Sistem',
            ),
            Tab(
              icon: Icon(Icons.bookmark_border_rounded, size: 20),
              text: 'Template',
            ),
            Tab(
              icon: Icon(Icons.history_rounded, size: 20),
              text: 'Riwayat',
            ),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          // TAB 1: NOTIFIKASI SISTEM
          RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(notificationSystemTypesProvider);
              await ref.read(notificationSystemTypesProvider.future);
            },
            child: systemTypesAsync.when(
              loading: () => const Center(child: LoadingIndicator()),
              error: (err, _) => ErrorState(
                message: err.toString(),
                onRetry: () => ref.invalidate(notificationSystemTypesProvider),
              ),
              data: (types) {
                if (types.isEmpty) {
                  return const EmptyState(
                    icon: Icons.notifications_off_outlined,
                    title: 'Tidak Ada Jenis Notifikasi',
                    description: 'Belum ada notifikasi sistem yang terdaftar.',
                  );
                }
                return ListView.builder(
                  padding: const EdgeInsets.all(16),
                  itemCount: types.length,
                  itemBuilder: (context, index) {
                    final st = types[index];
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: AppCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                Expanded(
                                  child: Text(
                                    st.name,
                                    style: theme.textTheme.titleMedium?.copyWith(
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                ),
                                const AppBadge(
                                  label: 'FCM PUSH',
                                  backgroundColor: AppColors.accent,
                                  textColor: Colors.white,
                                ),
                              ],
                            ),
                            const SizedBox(height: 6),
                            Text(
                              st.trigger,
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: isDark
                                    ? AppColors.darkTextMuted
                                    : AppColors.lightTextMuted,
                              ),
                            ),
                            const SizedBox(height: 10),
                            SizedBox(
                              width: double.infinity,
                              child: Container(
                                padding: const EdgeInsets.all(10),
                                decoration: BoxDecoration(
                                  color: isDark
                                      ? Colors.white.withValues(alpha: 0.04)
                                      : Colors.black.withValues(alpha: 0.03),
                                  borderRadius: BorderRadius.circular(8),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.stretch,
                                  children: [
                                    Text(
                                      'Penerima:',
                                      style: TextStyle(
                                        fontSize: 11,
                                        fontWeight: FontWeight.bold,
                                        color: isDark
                                            ? AppColors.darkTextMuted
                                            : AppColors.lightTextMuted,
                                      ),
                                    ),
                                    const SizedBox(height: 2),
                                    Text(
                                      st.recipientsRule,
                                      style: theme.textTheme.bodySmall,
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            const SizedBox(height: 10),
                            Container(
                              padding: const EdgeInsets.only(left: 10),
                              decoration: const BoxDecoration(
                                border: Border(
                                  left: BorderSide(
                                    color: AppColors.accent,
                                    width: 3,
                                  ),
                                ),
                              ),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    st.defaultTitle,
                                    style: theme.textTheme.bodyMedium?.copyWith(
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                  const SizedBox(height: 2),
                                  Text(
                                    st.defaultBody,
                                    style: theme.textTheme.bodySmall?.copyWith(
                                      color: isDark
                                          ? AppColors.darkTextMuted
                                          : AppColors.lightTextMuted,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            if (st.allowedVariables.isNotEmpty) ...[
                              const SizedBox(height: 10),
                              Wrap(
                                spacing: 6,
                                runSpacing: 4,
                                children: st.allowedVariables.map((v) {
                                  return Container(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 6,
                                      vertical: 2,
                                    ),
                                    decoration: BoxDecoration(
                                      color: isDark
                                          ? Colors.white.withValues(alpha: 0.08)
                                          : Colors.black.withValues(alpha: 0.06),
                                      borderRadius: BorderRadius.circular(4),
                                    ),
                                    child: Text(
                                      '{{$v}}',
                                      style: const TextStyle(
                                        fontSize: 11,
                                        fontFamily: 'monospace',
                                      ),
                                    ),
                                  );
                                }).toList(),
                              ),
                            ],
                            const SizedBox(height: 12),
                            Row(
                              children: [
                                Expanded(
                                  child: OutlinedButton(
                                    onPressed: () => _showPreviewBanner(
                                      title: st.defaultTitle,
                                      body: st.defaultBody,
                                      eventName: st.name,
                                    ),
                                    child: const Text('Preview Banner'),
                                  ),
                                ),
                                const SizedBox(width: 8),
                                Expanded(
                                  child: ElevatedButton(
                                    onPressed: () => _showAddEditTemplateDialog(
                                      defaultEventType: st.eventType,
                                      systemTypes: types,
                                    ),
                                    style: ElevatedButton.styleFrom(
                                      backgroundColor: AppColors.accent,
                                      foregroundColor: Colors.white,
                                    ),
                                    child: const Text('Kustomisasi'),
                                  ),
                                ),
                              ],
                            ),
                            if (st.eventType == 'manual_push' || (isSuperAdmin && st.supportsTest)) ...[
                              const SizedBox(height: 8),
                              Row(
                                children: [
                                  if (isSuperAdmin && st.supportsTest)
                                    Expanded(
                                      child: OutlinedButton.icon(
                                        onPressed: () => _handleTestPush(
                                          eventType: st.eventType,
                                          title: st.defaultTitle,
                                          body: st.defaultBody,
                                        ),
                                        icon: const Icon(Icons.send_outlined, size: 16),
                                        label: const Text('Test Notifikasi'),
                                        style: OutlinedButton.styleFrom(
                                          foregroundColor: AppColors.info,
                                          side: const BorderSide(color: AppColors.info),
                                        ),
                                      ),
                                    ),
                                  if (isSuperAdmin && st.supportsTest && st.eventType == 'manual_push')
                                    const SizedBox(width: 8),
                                  if (st.eventType == 'manual_push')
                                    Expanded(
                                      child: ElevatedButton.icon(
                                        onPressed: () => _confirmSendBroadcastDialog(
                                          eventType: st.eventType,
                                          title: st.defaultTitle,
                                          body: st.defaultBody,
                                        ),
                                        icon: const Icon(Icons.campaign_outlined, size: 16),
                                        label: const Text('Kirim Notifikasi'),
                                        style: ElevatedButton.styleFrom(
                                          backgroundColor: AppColors.success,
                                          foregroundColor: Colors.white,
                                        ),
                                      ),
                                    ),
                                ],
                              ),
                            ],
                          ],
                        ),
                      ),
                    );
                  },
                );
              },
            ),
          ),

          // TAB 2: TEMPLATE KUSTOM
          RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(notificationTemplatesProvider);
              await ref.read(notificationTemplatesProvider.future);
            },
            child: templatesAsync.when(
              loading: () => const Center(child: LoadingIndicator()),
              error: (err, _) => ErrorState(
                message: err.toString(),
                onRetry: () => ref.invalidate(notificationTemplatesProvider),
              ),
              data: (templates) {
                final systemTypes = systemTypesAsync.valueOrNull ?? [];
                if (templates.isEmpty) {
                  return Center(
                    child: Padding(
                      padding: const EdgeInsets.all(24.0),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.bookmark_border_rounded,
                            size: 48,
                            color: isDark
                                ? AppColors.darkTextMuted
                                : AppColors.lightTextMuted,
                          ),
                          const SizedBox(height: 12),
                          Text(
                            'Belum Ada Template Kustom',
                            style: theme.textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 6),
                          Text(
                            'Sistem menggunakan pesan default standar Urena. Anda dapat menambahkan template kustom untuk mengubah format pesan.',
                            textAlign: TextAlign.center,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: isDark
                                  ? AppColors.darkTextMuted
                                  : AppColors.lightTextMuted,
                            ),
                          ),
                          const SizedBox(height: 16),
                          ElevatedButton.icon(
                            onPressed: () => _showAddEditTemplateDialog(
                              systemTypes: systemTypes,
                            ),
                            icon: const Icon(Icons.add),
                            label: const Text('Buat Template Pertama'),
                            style: ElevatedButton.styleFrom(
                              backgroundColor: AppColors.accent,
                              foregroundColor: Colors.white,
                            ),
                          ),
                        ],
                      ),
                    ),
                  );
                }

                return ListView.builder(
                  padding: const EdgeInsets.all(16),
                  itemCount: templates.length + 1,
                  itemBuilder: (context, index) {
                    if (index == templates.length) {
                      return Padding(
                        padding: const EdgeInsets.symmetric(vertical: 8),
                        child: OutlinedButton.icon(
                          onPressed: () => _showAddEditTemplateDialog(
                            systemTypes: systemTypes,
                          ),
                          icon: const Icon(Icons.add),
                          label: const Text('Tambah Template Baru'),
                        ),
                      );
                    }

                    final tmpl = templates[index];
                    final stName = systemTypes
                            .firstWhere(
                              (s) => s.eventType == tmpl.eventType,
                              orElse: () => SystemNotificationType(
                                eventType: tmpl.eventType,
                                name: tmpl.eventType,
                                trigger: '',
                                recipientsRule: '',
                                defaultTitle: '',
                                defaultBody: '',
                                allowedVariables: const [],
                              ),
                            )
                            .name;

                    return Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: AppCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                Expanded(
                                  child: Text(
                                    tmpl.name,
                                    style: theme.textTheme.titleMedium?.copyWith(
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                ),
                                Switch(
                                  value: tmpl.isActive,
                                  activeColor: AppColors.accent,
                                  onChanged: (val) async {
                                    final messenger = ScaffoldMessenger.of(context);
                                    try {
                                      final repo = ref.read(
                                        notificationRepositoryProvider,
                                      );
                                      await repo.toggleTemplateActive(tmpl.id);
                                      if (!mounted) return;
                                      ref.invalidate(
                                        notificationTemplatesProvider,
                                      );
                                    } catch (e) {
                                      if (!mounted) return;
                                      messenger.showSnackBar(
                                        SnackBar(content: Text(e.toString())),
                                      );
                                    }
                                  },
                                ),
                              ],
                            ),
                            Wrap(
                              spacing: 6,
                              children: [
                                AppBadge(
                                  label: stName,
                                  backgroundColor:
                                      AppColors.accent.withValues(alpha: 0.15),
                                  textColor: AppColors.accent,
                                ),
                                if (tmpl.imageUrl != null &&
                                    tmpl.imageUrl!.isNotEmpty)
                                  AppBadge(
                                    label: 'Gambar Terlampir',
                                    backgroundColor:
                                        AppColors.info.withValues(alpha: 0.15),
                                    textColor: AppColors.info,
                                  ),
                              ],
                            ),
                            const SizedBox(height: 8),
                            Text(
                              tmpl.title,
                              style: theme.textTheme.bodyMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              tmpl.body,
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: isDark
                                    ? AppColors.darkTextMuted
                                    : AppColors.lightTextMuted,
                              ),
                            ),
                            const SizedBox(height: 12),
                            Row(
                              mainAxisAlignment: MainAxisAlignment.end,
                              children: [
                                if (isSuperAdmin &&
                                    (tmpl.eventType == 'manual_push' ||
                                        tmpl.eventType == 'waste_truck_arrival'))
                                  IconButton(
                                    icon: const Icon(Icons.send_outlined,
                                        size: 20, color: AppColors.info),
                                    tooltip: 'Test Notifikasi (Super Admin)',
                                    onPressed: () => _handleTestPush(
                                      eventType: tmpl.eventType,
                                      title: tmpl.title,
                                      body: tmpl.body,
                                      imageUrl: tmpl.imageUrl,
                                    ),
                                  ),
                                if (tmpl.eventType == 'manual_push')
                                  IconButton(
                                    icon: const Icon(Icons.campaign_outlined,
                                        size: 20, color: AppColors.accent),
                                    tooltip: 'Kirim Notifikasi Siaran',
                                    onPressed: () => _confirmSendBroadcastDialog(
                                      eventType: tmpl.eventType,
                                      title: tmpl.title,
                                      body: tmpl.body,
                                      imageUrl: tmpl.imageUrl,
                                    ),
                                  ),
                                IconButton(
                                  icon: const Icon(Icons.preview_rounded, size: 20),
                                  tooltip: 'Preview Banner',
                                  onPressed: () => _showPreviewBanner(
                                    title: tmpl.title,
                                    body: tmpl.body,
                                    eventName: tmpl.name,
                                    imageUrl: tmpl.imageUrl,
                                  ),
                                ),
                                IconButton(
                                  icon: const Icon(Icons.edit_outlined, size: 20),
                                  tooltip: 'Ubah Template',
                                  onPressed: () => _showAddEditTemplateDialog(
                                    template: tmpl,
                                    systemTypes: systemTypes,
                                  ),
                                ),
                                IconButton(
                                  icon: const Icon(
                                    Icons.delete_outline,
                                    size: 20,
                                    color: AppColors.danger,
                                  ),
                                  tooltip: 'Hapus Template',
                                  onPressed: () => _confirmDeleteTemplate(tmpl),
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
            ),
          ),

          // TAB 3: RIWAYAT PENGIRIMAN
          RefreshIndicator(
            onRefresh: _loadLogs,
            child: Column(
              children: [
                // Filter bar
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  child: Row(
                    children: [
                      Text(
                        'Filter:',
                        style: theme.textTheme.bodySmall?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: DropdownButtonHideUnderline(
                          child: DropdownButton<String>(
                            isExpanded: true,
                            value: _selectedLogEventType,
                            items: [
                              const DropdownMenuItem(
                                value: '',
                                child: Text('Semua Jenis Notifikasi'),
                              ),
                              ...(systemTypesAsync.valueOrNull ?? []).map((st) {
                                return DropdownMenuItem(
                                  value: st.eventType,
                                  child: Text(st.name),
                                );
                              }),
                            ],
                            onChanged: (val) {
                              if (val != null) {
                                setState(() {
                                  _selectedLogEventType = val;
                                });
                                _loadLogs();
                              }
                            },
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                const Divider(height: 1),

                // List
                Expanded(
                  child: _isLoadingLogs
                      ? const Center(child: LoadingIndicator())
                      : _logsError != null
                          ? ErrorState(
                              message: _logsError!,
                              onRetry: _loadLogs,
                            )
                          : _logs.isEmpty
                              ? const EmptyState(
                                  icon: Icons.history_toggle_off_rounded,
                                  title: 'Belum Ada Riwayat Notifikasi',
                                  description:
                                      'Riwayat tercatat secara otomatis setiap kali notifikasi push dikirimkan ke warga.',
                                )
                              : ListView.builder(
                                  key: const PageStorageKey<String>(
                                    'notification_logs_list',
                                  ),
                                  padding: const EdgeInsets.all(16),
                                  itemCount: _logs.length,
                                  itemBuilder: (context, index) {
                                    final log = _logs[index];
                                    final dateStr = log.createdAt != null
                                        ? '${log.createdAt!.day.toString().padLeft(2, '0')}/${log.createdAt!.month.toString().padLeft(2, '0')}/${log.createdAt!.year} ${log.createdAt!.hour.toString().padLeft(2, '0')}:${log.createdAt!.minute.toString().padLeft(2, '0')}'
                                        : '-';

                                    return Padding(
                                      padding: const EdgeInsets.only(bottom: 10),
                                      child: InkWell(
                                        key: Key(
                                          'notification_log_item_${log.id}',
                                        ),
                                        borderRadius: BorderRadius.circular(
                                          AppRadius.base,
                                        ),
                                        onTap: () => _openLogDetail(log),
                                        child: AppCard(
                                          child: Column(
                                            crossAxisAlignment:
                                                CrossAxisAlignment.start,
                                            children: [
                                              Row(
                                                mainAxisAlignment:
                                                    MainAxisAlignment.spaceBetween,
                                                children: [
                                                  Text(
                                                    dateStr,
                                                    style: theme.textTheme.bodySmall
                                                        ?.copyWith(
                                                      color: isDark
                                                          ? AppColors.darkTextMuted
                                                          : AppColors.lightTextMuted,
                                                    ),
                                                  ),
                                                  const AppBadge(
                                                    label: 'TERKIRIM',
                                                    backgroundColor: AppColors.success,
                                                    textColor: Colors.white,
                                                  ),
                                                ],
                                              ),
                                              const SizedBox(height: 6),
                                              Text(
                                                log.title,
                                                style: theme.textTheme.bodyMedium
                                                    ?.copyWith(
                                                  fontWeight: FontWeight.bold,
                                                ),
                                              ),
                                              const SizedBox(height: 2),
                                              Text(
                                                log.body,
                                                style: theme.textTheme.bodySmall
                                                    ?.copyWith(
                                                  color: isDark
                                                      ? AppColors.darkTextMuted
                                                      : AppColors.lightTextMuted,
                                                ),
                                              ),
                                              const SizedBox(height: 6),
                                              Text(
                                                'Penerima: ${log.recipientsCount} perangkat',
                                                style: TextStyle(
                                                  fontSize: 11,
                                                  color: isDark
                                                      ? AppColors.darkTextMuted
                                                      : AppColors.lightTextMuted,
                                                ),
                                              ),
                                            ],
                                          ),
                                        ),
                                      ),
                                    );
                                  },
                                ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
