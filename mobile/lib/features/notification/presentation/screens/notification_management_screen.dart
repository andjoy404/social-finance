import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/notification/notification_url_helper.dart';
import 'package:social_finance/core/notification/push_notification_service.dart';
import 'package:social_finance/core/utils/jakarta_time_utils.dart';
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
    with TickerProviderStateMixin {
  late final TabController _tabController;
  late final TabController _historyTabController;
  late final TabController _configTabController;

  static const Map<String, String> _knownEventNames = {
    'manual_push': 'Pengumuman Pengurus',
    'waste_truck_arrival': 'Kedatangan Mobil Sampah',
    'iuran_bill_created': 'Penerbitan Tagihan Iuran',
    'iuran_payment_reminder': 'Pengingat Pembayaran Iuran',
    'iuran_upcoming_due_3_months':
        'Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)',
    'iuran_overdue_3_months': 'Pengingat Tunggakan Iuran (3 Bulan+)',
    'payment_verified': 'Pembayaran Diverifikasi',
    'payment_rejected': 'Pembayaran Ditolak',
  };

  // Logs state
  String? _currentUserId;
  List<NotificationLogEntry> _logs = [];
  bool _isLoadingLogs = false;
  String? _logsError;
  String _selectedLogEventType = '';
  String _selectedLogStatus = 'all';
  String? _historyCategory;

  // Schedules state
  List<NotificationSchedule> _schedules = [];
  bool _isLoadingSchedules = false;
  String? _schedulesError;
  bool _isSavingSchedule = false;

  /// Tracks whether the current data corresponds to the given user.
  bool _dataBelongsTo(String? userId) {
    return _currentUserId == userId;
  }

  /// Clears all cached data and resets filter state.
  void _clearState() {
    _logs = [];
    _isLoadingLogs = false;
    _logsError = null;
    _selectedLogEventType = '';
    _selectedLogStatus = 'all';
    _historyCategory = null;
    _schedules = [];
    _isLoadingSchedules = false;
    _schedulesError = null;
    _isSavingSchedule = false;
  }

  /// Loads data for the given user, clearing stale data if the user changed.
  void _loadInitialData({required bool isAdmin, String? userId}) {
    if (!_dataBelongsTo(userId)) {
      _clearState();
      _currentUserId = userId;
    }
    if (isAdmin) {
      _loadHistoryForTab(_historyTabController.index);
      _loadSchedules();
    } else {
      _loadLogs();
    }
  }

  late final _authSubscription =
      ref.listenManual<AsyncValue<Map<String, dynamic>?>?>(
    authRepositoryProvider,
    (previous, next) {
      final user = next?.valueOrNull;
      if (user == null) {
        // Logged out – clear everything.
        if (mounted) {
          setState(() {
            _currentUserId = null;
            _logs = [];
            _schedules = [];
            _isLoadingLogs = false;
            _isLoadingSchedules = false;
            _logsError = null;
            _schedulesError = null;
          });
        }
        return;
      }
      final userId = user['id'] as String?;
      // Only load data when the user identity actually changes.
      if (_dataBelongsTo(userId)) {
        return;
      }
      final newAdmin = canUserAccessNotificationAdmin(user);
      if (mounted) {
        setState(() {
          _clearState();
          _currentUserId = userId;
        });
        if (newAdmin) {
          _loadHistoryForTab(_historyTabController.index);
          _loadSchedules();
        } else {
          _loadLogs();
        }
      }
    },
  );

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    _historyTabController = TabController(length: 2, vsync: this);
    _configTabController = TabController(length: 3, vsync: this);
    _historyTabController.addListener(_handleHistoryTabChange);

    // Read initial auth state and load data for the current user.
    final authState = ref.read(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final isAdmin = canUserAccessNotificationAdmin(user);
    _loadInitialData(isAdmin: isAdmin, userId: user?['id'] as String?);
  }

  @override
  void dispose() {
    // ProviderSubscription from ref.listenManual doesn't have a public dispose.
    // The subscription is automatically cleaned up when the ConsumerState is disposed.
    _historyTabController.removeListener(_handleHistoryTabChange);
    _tabController.dispose();
    _historyTabController.dispose();
    _configTabController.dispose();
    super.dispose();
  }

  void _handleHistoryTabChange() {
    _loadHistoryForTab(_historyTabController.index);
  }

  void _loadHistoryForTab(int index) {
    final category = switch (index) {
      0 => 'system',
      1 => 'mine',
      _ => null,
    };
    if (category != null && category != _historyCategory) {
      _historyCategory = category;
      _loadLogs(category: category);
    }
  }

  Future<void> _loadLogs({String? category}) async {
    setState(() {
      _isLoadingLogs = true;
      _logsError = null;
    });
    // Capture current user ID so stale responses can be discarded.
    final snapshotUserId = _currentUserId;
    try {
      final repo = ref.read(notificationRepositoryProvider);
      final resp = category == null
          ? await repo.fetchLogs(
              page: 1,
              perPage: 50,
              eventType: _selectedLogEventType.isNotEmpty
                  ? _selectedLogEventType
                  : null,
              status: _selectedLogStatus != 'all' ? _selectedLogStatus : null,
            )
          : await repo.fetchManagementHistory(
              category: category,
              page: 1,
              perPage: 50,
            );
      // Only apply the result if the user hasn't changed.
      if (mounted && _currentUserId == snapshotUserId) {
        // Client-side security filter (defense-in-depth):
        // - For admin categories ('system'/'mine'): no additional filtering.
        //   The server already scopes by role and category.
        // - For non-admin users (raw logs, category == null): ONLY show logs
        //   where user_id matches the current user. No null, no broadcast,
        //   no dispatch records leak through.
        final isAdminViewing = category != null;
        final filtered = isAdminViewing
            ? resp.data
            : resp.data.where(
                (log) => log.userId == snapshotUserId,
              ).toList();
        setState(() {
          _logs = filtered;
          _isLoadingLogs = false;
        });
      }
    } catch (e) {
      // Only show the error if the user hasn't changed.
      if (mounted && _currentUserId == snapshotUserId) {
        setState(() {
          _logsError = e.toString();
          _isLoadingLogs = false;
        });
      }
    }
  }

  Future<void> _loadSchedules() async {
    setState(() {
      _isLoadingSchedules = true;
      _schedulesError = null;
    });
    final snapshotUserId = _currentUserId;
    try {
      final repo = ref.read(notificationRepositoryProvider);
      final list = await repo.fetchSchedules();
      if (mounted && _currentUserId == snapshotUserId) {
        setState(() {
          _schedules = list;
          _isLoadingSchedules = false;
        });
      }
    } catch (e) {
      if (mounted && _currentUserId == snapshotUserId) {
        setState(() {
          _schedulesError = e.toString();
          _isLoadingSchedules = false;
        });
      }
    }
  }

  Future<void> _markAllAsRead() async {
    try {
      final repo = ref.read(notificationRepositoryProvider);
      await repo.markAllAsRead();
      if (mounted) {
        setState(() {
          _logs = _logs
              .map(
                (l) => l.copyWith(
                  isRead: true,
                  readAt: l.readAt ?? DateTime.now(),
                ),
              )
              .toList();
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Semua notifikasi ditandai sebagai sudah dibaca'),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal menandai notifikasi: $e')),
        );
      }
    }
  }

  void _openLogDetail(NotificationLogEntry log) {
    if (_historyCategory == null && !log.isRead) {
      ref
          .read(notificationRepositoryProvider)
          .markAsRead(log.id)
          .then((_) {
            if (mounted) {
              setState(() {
                _logs = _logs
                    .map(
                      (l) => l.id == log.id
                          ? l.copyWith(isRead: true, readAt: DateTime.now())
                          : l,
                    )
                    .toList();
              });
            }
          })
          .catchError((_) {});
    }

    final combinedPayload = <String, dynamic>{
      if (log.payload != null) ...log.payload!,
      if (log.variables.isNotEmpty) ...log.variables,
      'type': log.eventType,
      'title': log.title,
      'body': log.body,
      if (log.imageUrl != null) 'image_url': log.imageUrl,
      if (log.createdAt != null) 'sent_at': log.createdAt!.toIso8601String(),
      'notification_id': log.id,
    };

    final deepLink = log.payload?['deep_link'] as String?;
    var target = normalizeNotificationTarget(
      deepLink,
      payload: combinedPayload,
    );
    if (target.contains('/notifikasi/detail') && !target.contains('from=')) {
      final sep = target.contains('?') ? '&' : '?';
      target = '$target${sep}from=notifikasi';
    }

    final router = GoRouter.maybeOf(context);
    if (router != null) {
      router.push(target);
      return;
    }

    final id = log.id.trim();
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
                icon: const Icon(
                  Icons.arrow_back_ios_new,
                  color: Colors.white,
                  size: 20,
                ),
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
                                errorBuilder: (_, _, _) =>
                                    const SizedBox.shrink(),
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                    const SizedBox(height: 24),
                    // Label info
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 8,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.accent.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            Icons.info_outline,
                            size: 16,
                            color: AppColors.accent,
                          ),
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
                    Text(
                      title,
                      style: const TextStyle(
                        fontWeight: FontWeight.bold,
                        fontSize: 13,
                      ),
                    ),
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
                        'Notifikasi berhasil disiarkan ke seluruh warga RT!',
                      ),
                    ),
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
            'Uji notifikasi berhasil dikirim ($count perangkat aktif Anda).',
          ),
        ),
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
    final imageController = TextEditingController(
      text: template?.imageUrl ?? '',
    );

    Uint8List? pickedImageBytes;
    bool isUploadingImage = false;
    String? imageUploadError;

    String selectedEvent =
        template?.eventType ??
        defaultEventType ??
        (systemTypes?.isNotEmpty == true
            ? systemTypes!.first.eventType
            : 'iuran_bill_created');

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
                  color: isDark
                      ? AppColors.darkSurface
                      : AppColors.lightSurface,
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
                      if (!isEditing &&
                          systemTypes != null &&
                          systemTypes.isNotEmpty) ...[
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
                              color: isDark
                                  ? AppColors.darkBorder
                                  : AppColors.lightBorder,
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
                                  child: Text(
                                    st.name,
                                    style: theme.textTheme.bodyMedium,
                                  ),
                                );
                              }).toList(),
                              onChanged: (val) {
                                if (val != null) {
                                  setSheetState(() {
                                    selectedEvent = val;
                                    final match = systemTypes.firstWhere(
                                      (s) => s.eventType == val,
                                    );
                                    nameController.text =
                                        'Template ${match.name}';
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

                      // Image input section for events supporting images
                      if (selectedEvent == 'manual_push') ...[
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text(
                              'Gambar Lampiran Pengumuman (Opsional)',
                              style: theme.textTheme.labelMedium?.copyWith(
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                            if (pickedImageBytes != null ||
                                imageController.text.isNotEmpty)
                              TextButton(
                                onPressed: () {
                                  setSheetState(() {
                                    pickedImageBytes = null;
                                    imageController.clear();
                                    imageUploadError = null;
                                  });
                                },
                                style: TextButton.styleFrom(
                                  visualDensity: VisualDensity.compact,
                                  foregroundColor: AppColors.danger,
                                ),
                                child: const Text(
                                  'Hapus Gambar',
                                  style: TextStyle(fontSize: 12),
                                ),
                              ),
                          ],
                        ),
                        const SizedBox(height: 6),
                        if (imageUploadError != null)
                          Container(
                            padding: const EdgeInsets.all(8),
                            margin: const EdgeInsets.only(bottom: 8),
                            decoration: BoxDecoration(
                              color: AppColors.danger.withValues(alpha: 0.1),
                              borderRadius: BorderRadius.circular(AppRadius.sm),
                            ),
                            child: Text(
                              imageUploadError!,
                              style: const TextStyle(
                                color: AppColors.danger,
                                fontSize: 12,
                              ),
                            ),
                          ),
                        if (isUploadingImage)
                          Container(
                            padding: const EdgeInsets.symmetric(vertical: 24),
                            alignment: Alignment.center,
                            child: const Column(
                              children: [
                                CircularProgressIndicator(),
                                SizedBox(height: 8),
                                Text(
                                  'Mengunggah gambar ke server...',
                                  style: TextStyle(fontSize: 12),
                                ),
                              ],
                            ),
                          )
                        else if (pickedImageBytes != null) ...[
                          ClipRRect(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            child: AspectRatio(
                              aspectRatio: 16 / 9,
                              child: Image.memory(
                                pickedImageBytes!,
                                fit: BoxFit.cover,
                              ),
                            ),
                          ),
                          const SizedBox(height: 8),
                          Row(
                            children: [
                              OutlinedButton.icon(
                                onPressed: () async {
                                  setSheetState(() {
                                    imageUploadError = null;
                                    isUploadingImage = true;
                                  });
                                  try {
                                    final picker = ImagePicker();
                                    final picked = await picker.pickImage(
                                      source: ImageSource.gallery,
                                      maxWidth: 1920,
                                      maxHeight: 1920,
                                      imageQuality: 85,
                                    );
                                    if (picked == null) {
                                      setSheetState(
                                        () => isUploadingImage = false,
                                      );
                                      return;
                                    }
                                    final bytes = await picked.readAsBytes();
                                    final repo = ref.read(
                                      notificationRepositoryProvider,
                                    );
                                    final url = await repo.uploadImage(
                                      eventType: selectedEvent,
                                      bytes: bytes,
                                      filename: picked.name,
                                    );
                                    setSheetState(() {
                                      pickedImageBytes = bytes;
                                      imageController.text = url;
                                      isUploadingImage = false;
                                      imageUploadError = null;
                                    });
                                  } catch (e) {
                                    setSheetState(() {
                                      isUploadingImage = false;
                                      imageUploadError =
                                          'Gagal mengunggah gambar: $e';
                                    });
                                  }
                                },
                                icon: const Icon(Icons.refresh, size: 16),
                                label: const Text(
                                  'Ganti Gambar',
                                  style: TextStyle(fontSize: 12),
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 10),
                        ] else if (imageController.text.isNotEmpty) ...[
                          ClipRRect(
                            borderRadius: BorderRadius.circular(AppRadius.base),
                            child: AspectRatio(
                              aspectRatio: 16 / 9,
                              child: Image.network(
                                ApiConfig.resolveImageUrl(
                                      imageController.text,
                                    ) ??
                                    imageController.text,
                                fit: BoxFit.cover,
                                errorBuilder: (_, _, _) => Container(
                                  color: Colors.grey.withValues(alpha: 0.1),
                                  alignment: Alignment.center,
                                  child: const Text(
                                    'Gagal menampilkan gambar pratinjau',
                                    style: TextStyle(fontSize: 12),
                                  ),
                                ),
                              ),
                            ),
                          ),
                          const SizedBox(height: 8),
                          Row(
                            children: [
                              OutlinedButton.icon(
                                onPressed: () async {
                                  setSheetState(() {
                                    imageUploadError = null;
                                    isUploadingImage = true;
                                  });
                                  try {
                                    final picker = ImagePicker();
                                    final picked = await picker.pickImage(
                                      source: ImageSource.gallery,
                                      maxWidth: 1920,
                                      maxHeight: 1920,
                                      imageQuality: 85,
                                    );
                                    if (picked == null) {
                                      setSheetState(
                                        () => isUploadingImage = false,
                                      );
                                      return;
                                    }
                                    final bytes = await picked.readAsBytes();
                                    final repo = ref.read(
                                      notificationRepositoryProvider,
                                    );
                                    final url = await repo.uploadImage(
                                      eventType: selectedEvent,
                                      bytes: bytes,
                                      filename: picked.name,
                                    );
                                    setSheetState(() {
                                      pickedImageBytes = bytes;
                                      imageController.text = url;
                                      isUploadingImage = false;
                                      imageUploadError = null;
                                    });
                                  } catch (e) {
                                    setSheetState(() {
                                      isUploadingImage = false;
                                      imageUploadError =
                                          'Gagal mengunggah gambar: $e';
                                    });
                                  }
                                },
                                icon: const Icon(Icons.refresh, size: 16),
                                label: const Text(
                                  'Ganti Gambar',
                                  style: TextStyle(fontSize: 12),
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 10),
                        ] else ...[
                          Row(
                            children: [
                              Expanded(
                                child: OutlinedButton.icon(
                                  onPressed: () async {
                                    setSheetState(() {
                                      imageUploadError = null;
                                      isUploadingImage = true;
                                    });
                                    try {
                                      final picker = ImagePicker();
                                      final picked = await picker.pickImage(
                                        source: ImageSource.camera,
                                        maxWidth: 1920,
                                        maxHeight: 1920,
                                        imageQuality: 85,
                                      );
                                      if (picked == null) {
                                        setSheetState(
                                          () => isUploadingImage = false,
                                        );
                                        return;
                                      }
                                      final bytes = await picked.readAsBytes();
                                      final repo = ref.read(
                                        notificationRepositoryProvider,
                                      );
                                      final url = await repo.uploadImage(
                                        eventType: selectedEvent,
                                        bytes: bytes,
                                        filename: picked.name,
                                      );
                                      setSheetState(() {
                                        pickedImageBytes = bytes;
                                        imageController.text = url;
                                        isUploadingImage = false;
                                        imageUploadError = null;
                                      });
                                    } catch (e) {
                                      setSheetState(() {
                                        isUploadingImage = false;
                                        imageUploadError =
                                            'Gagal mengunggah gambar: $e';
                                      });
                                    }
                                  },
                                  icon: const Icon(Icons.camera_alt, size: 16),
                                  label: const Text(
                                    'Kamera',
                                    style: TextStyle(fontSize: 12),
                                  ),
                                ),
                              ),
                              const SizedBox(width: 8),
                              Expanded(
                                child: OutlinedButton.icon(
                                  onPressed: () async {
                                    setSheetState(() {
                                      imageUploadError = null;
                                      isUploadingImage = true;
                                    });
                                    try {
                                      final picker = ImagePicker();
                                      final picked = await picker.pickImage(
                                        source: ImageSource.gallery,
                                        maxWidth: 1920,
                                        maxHeight: 1920,
                                        imageQuality: 85,
                                      );
                                      if (picked == null) {
                                        setSheetState(
                                          () => isUploadingImage = false,
                                        );
                                        return;
                                      }
                                      final bytes = await picked.readAsBytes();
                                      final repo = ref.read(
                                        notificationRepositoryProvider,
                                      );
                                      final url = await repo.uploadImage(
                                        eventType: selectedEvent,
                                        bytes: bytes,
                                        filename: picked.name,
                                      );
                                      setSheetState(() {
                                        pickedImageBytes = bytes;
                                        imageController.text = url;
                                        isUploadingImage = false;
                                        imageUploadError = null;
                                      });
                                    } catch (e) {
                                      setSheetState(() {
                                        isUploadingImage = false;
                                        imageUploadError =
                                            'Gagal mengunggah gambar: $e';
                                      });
                                    }
                                  },
                                  icon: const Icon(
                                    Icons.photo_library,
                                    size: 16,
                                  ),
                                  label: const Text(
                                    'Galeri',
                                    style: TextStyle(fontSize: 12),
                                  ),
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 10),
                        ],
                      ] else if (selectedEvent == 'waste_truck_arrival') ...[
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
                            hintText:
                                'https://... atau /api/v1/notifications/images/...',
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
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
                              label: Text(
                                tag,
                                style: const TextStyle(fontSize: 12),
                              ),
                              backgroundColor: AppColors.accent.withValues(
                                alpha: 0.12,
                              ),
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
                            child: ElevatedButton(
                              onPressed: () => Navigator.of(ctx).pop(),
                              style: ElevatedButton.styleFrom(
                                backgroundColor: AppColors.danger,
                                foregroundColor: Colors.white,
                              ),
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
                                if (name.isEmpty ||
                                    title.isEmpty ||
                                    body.isEmpty) {
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    const SnackBar(
                                      content: Text('Semua kolom wajib diisi'),
                                    ),
                                  );
                                  return;
                                }

                                final rawImg = imageController.text.trim();
                                final supportsImg =
                                    selectedEvent == 'waste_truck_arrival' ||
                                    selectedEvent == 'manual_push';
                                final imgUrl =
                                    (supportsImg && rawImg.isNotEmpty)
                                    ? rawImg
                                    : null;

                                final messenger = ScaffoldMessenger.of(context);
                                try {
                                  final repo = ref.read(
                                    notificationRepositoryProvider,
                                  );
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
                  messenger.showSnackBar(SnackBar(content: Text(e.toString())));
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
    final isAdmin = canUserAccessNotificationAdmin(user);

    if (!isAdmin) {
      return Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: Navigator.of(context).canPop(),
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Notifikasi'),
          actions: [
            IconButton(
              icon: const Icon(Icons.done_all_rounded),
              tooltip: 'Tandai Semua Dibaca',
              onPressed: _markAllAsRead,
            ),
          ],
        ),
        body: _buildLogsTab(
          theme: theme,
          isDark: isDark,
          isAdmin: false,
          systemTypes: const [],
        ),
      );
    }

    final systemTypesAsync = ref.watch(notificationSystemTypesProvider);
    final templatesAsync = ref.watch(notificationTemplatesProvider);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: Navigator.of(context).canPop(),
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Kelola Notifikasi'),
        actions: [],
        bottom: TabBar(
          controller: _tabController,
          tabAlignment: TabAlignment.fill,
          labelPadding: EdgeInsets.zero,
          labelStyle: const TextStyle(fontSize: 12),
          labelColor: AppColors.accent,
          unselectedLabelColor: isDark
              ? AppColors.darkTextMuted
              : AppColors.lightTextMuted,
          indicatorColor: AppColors.accent,
          indicatorWeight: 3,
          tabs: const [
            Tab(icon: Icon(Icons.notifications_outlined, size: 20), text: 'NOTIFIKASI'),
            Tab(icon: Icon(Icons.bookmark_border_rounded, size: 20), text: 'TEMPLATE'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          Column(
            children: [
              TabBar(
                controller: _historyTabController,
                onTap: _loadHistoryForTab,
                tabAlignment: TabAlignment.fill,
                labelPadding: EdgeInsets.zero,
                labelStyle: const TextStyle(fontSize: 11),
                labelColor: AppColors.accent,
                unselectedLabelColor: isDark
                    ? AppColors.darkTextMuted
                    : AppColors.lightTextMuted,
                indicatorColor: AppColors.accent,
                tabs: const [
                  Tab(text: 'SISTEM'),
                  Tab(text: 'RIWAYAT SAYA'),
                ],
              ),
              Expanded(
                child: TabBarView(
                  controller: _historyTabController,
                  children: [
                    _buildLogsTab(
                      theme: theme,
                      isDark: isDark,
                      isAdmin: true,
                      systemTypes: systemTypesAsync.valueOrNull ?? [],
                      category: 'system',
                    ),
                    _buildLogsTab(
                      theme: theme,
                      isDark: isDark,
                      isAdmin: true,
                      systemTypes: systemTypesAsync.valueOrNull ?? [],
                      category: 'mine',
                    ),
                  ],
                ),
              ),
            ],
          ),
          Column(
            children: [
              TabBar(
                controller: _configTabController,
                tabAlignment: TabAlignment.fill,
                labelPadding: EdgeInsets.zero,
                labelStyle: const TextStyle(fontSize: 9),
                labelColor: AppColors.accent,
                unselectedLabelColor: isDark
                    ? AppColors.darkTextMuted
                    : AppColors.lightTextMuted,
                indicatorColor: AppColors.accent,
                tabs: const [
                  Tab(icon: Icon(Icons.apps_rounded, size: 14), text: 'SISTEM'),
                  Tab(
                    icon: Icon(Icons.bookmark_border_rounded, size: 14),
                    text: 'KUSTOM',
                  ),
                  Tab(
                    icon: Icon(Icons.schedule_rounded, size: 14),
                    text: 'PENJADWALAN',
                  ),
                ],
              ),
              Expanded(
                child: TabBarView(
                  controller: _configTabController,
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
                          onRetry: () =>
                              ref.invalidate(notificationSystemTypesProvider),
                        ),
                        data: (types) {
                          if (types.isEmpty) {
                            return const EmptyState(
                              icon: Icons.notifications_off_outlined,
                              title: 'Tidak Ada Jenis Notifikasi',
                              description:
                                  'Belum ada notifikasi sistem yang terdaftar.',
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
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Row(
                                        mainAxisAlignment:
                                            MainAxisAlignment.spaceBetween,
                                        children: [
                                          Expanded(
                                            child: Text(
                                              st.name,
                                              style: theme.textTheme.titleMedium
                                                  ?.copyWith(
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
                                        style: theme.textTheme.bodySmall
                                            ?.copyWith(
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
                                                ? Colors.white.withValues(
                                                    alpha: 0.04,
                                                  )
                                                : Colors.black.withValues(
                                                    alpha: 0.03,
                                                  ),
                                            borderRadius: BorderRadius.circular(
                                              8,
                                            ),
                                          ),
                                          child: Column(
                                            crossAxisAlignment:
                                                CrossAxisAlignment.stretch,
                                            children: [
                                              Text(
                                                'Penerima:',
                                                style: TextStyle(
                                                  fontSize: 11,
                                                  fontWeight: FontWeight.bold,
                                                  color: isDark
                                                      ? AppColors.darkTextMuted
                                                      : AppColors
                                                            .lightTextMuted,
                                                ),
                                              ),
                                              const SizedBox(height: 2),
                                              Text(
                                                st.recipientsRule,
                                                style:
                                                    theme.textTheme.bodySmall,
                                              ),
                                            ],
                                          ),
                                        ),
                                      ),
                                      const SizedBox(height: 10),
                                      Container(
                                        padding: const EdgeInsets.only(
                                          left: 10,
                                        ),
                                        decoration: const BoxDecoration(
                                          border: Border(
                                            left: BorderSide(
                                              color: AppColors.accent,
                                              width: 3,
                                            ),
                                          ),
                                        ),
                                        child: Column(
                                          crossAxisAlignment:
                                              CrossAxisAlignment.start,
                                          children: [
                                            Text(
                                              st.defaultTitle,
                                              style: theme.textTheme.bodyMedium
                                                  ?.copyWith(
                                                    fontWeight: FontWeight.bold,
                                                  ),
                                            ),
                                            const SizedBox(height: 2),
                                            Text(
                                              st.defaultBody,
                                              style: theme.textTheme.bodySmall
                                                  ?.copyWith(
                                                    color: isDark
                                                        ? AppColors
                                                              .darkTextMuted
                                                        : AppColors
                                                              .lightTextMuted,
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
                                          children: st.allowedVariables.map((
                                            v,
                                          ) {
                                            return Container(
                                              padding:
                                                  const EdgeInsets.symmetric(
                                                    horizontal: 6,
                                                    vertical: 2,
                                                  ),
                                              decoration: BoxDecoration(
                                                color: isDark
                                                    ? Colors.white.withValues(
                                                        alpha: 0.08,
                                                      )
                                                    : Colors.black.withValues(
                                                        alpha: 0.06,
                                                      ),
                                                borderRadius:
                                                    BorderRadius.circular(4),
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
                                      Wrap(
                                        spacing: 8,
                                        runSpacing: 8,
                                        children: [
                                          SizedBox(
                                            width: 135,
                                            child: ElevatedButton(
                                              onPressed: () =>
                                                  _showPreviewBanner(
                                                    title: st.defaultTitle,
                                                    body: st.defaultBody,
                                                    eventName: st.name,
                                                  ),
                                              style: ElevatedButton.styleFrom(
                                                backgroundColor: AppColors.info,
                                                foregroundColor: Colors.white,
                                                padding:
                                                    const EdgeInsets.symmetric(
                                                      horizontal: 12,
                                                      vertical: 10,
                                                    ),
                                              ),
                                              child: const Text(
                                                'Preview Banner',
                                              ),
                                            ),
                                          ),
                                          SizedBox(
                                            width: 135,
                                            child: ElevatedButton(
                                              onPressed: () =>
                                                  _showAddEditTemplateDialog(
                                                    defaultEventType:
                                                        st.eventType,
                                                    systemTypes: types,
                                                  ),
                                              style: ElevatedButton.styleFrom(
                                                backgroundColor:
                                                    AppColors.accent,
                                                foregroundColor: Colors.white,
                                                padding:
                                                    const EdgeInsets.symmetric(
                                                      horizontal: 12,
                                                      vertical: 10,
                                                    ),
                                              ),
                                              child: const Text(
                                                'Kustomisasi',
                                                softWrap: true,
                                                textAlign: TextAlign.center,
                                              ),
                                            ),
                                          ),
                                        ],
                                      ),
                                      if (st.eventType == 'manual_push' ||
                                          (isSuperAdmin &&
                                              st.supportsTest)) ...[
                                        const SizedBox(height: 8),
                                        Wrap(
                                          spacing: 8,
                                          runSpacing: 8,
                                          children: [
                                            if (isSuperAdmin && st.supportsTest)
                                              SizedBox(
                                                width: 135,
                                                child: OutlinedButton.icon(
                                                  onPressed: () =>
                                                      _handleTestPush(
                                                        eventType: st.eventType,
                                                        title: st.defaultTitle,
                                                        body: st.defaultBody,
                                                      ),
                                                  icon: const Icon(
                                                    Icons.send_outlined,
                                                    size: 16,
                                                  ),
                                                  label: const Text(
                                                    'Test Notifikasi',
                                                  ),
                                                  style:
                                                      OutlinedButton.styleFrom(
                                                        foregroundColor:
                                                            AppColors.info,
                                                        side: const BorderSide(
                                                          color: AppColors.info,
                                                        ),
                                                      ),
                                                ),
                                              ),
                                            if (st.eventType == 'manual_push')
                                              SizedBox(
                                                width: 135,
                                                child: ElevatedButton.icon(
                                                  onPressed: () =>
                                                      _confirmSendBroadcastDialog(
                                                        eventType: st.eventType,
                                                        title: st.defaultTitle,
                                                        body: st.defaultBody,
                                                      ),
                                                  icon: const Icon(
                                                    Icons.campaign_outlined,
                                                    size: 16,
                                                  ),
                                                  label: const Text(
                                                    'Kirim Notifikasi',
                                                    softWrap: true,
                                                    textAlign: TextAlign.center,
                                                  ),
                                                  style:
                                                      ElevatedButton.styleFrom(
                                                        backgroundColor:
                                                            AppColors.success,
                                                        foregroundColor:
                                                            Colors.white,
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
                          onRetry: () =>
                              ref.invalidate(notificationTemplatesProvider),
                        ),
                        data: (templates) {
                          final systemTypes =
                              systemTypesAsync.valueOrNull ?? [];
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
                                      style: theme.textTheme.titleMedium
                                          ?.copyWith(
                                            fontWeight: FontWeight.bold,
                                          ),
                                    ),
                                    const SizedBox(height: 6),
                                    Text(
                                      'Sistem menggunakan pesan default standar Urena. Anda dapat menambahkan template kustom untuk mengubah format pesan.',
                                      textAlign: TextAlign.center,
                                      style: theme.textTheme.bodySmall
                                          ?.copyWith(
                                            color: isDark
                                                ? AppColors.darkTextMuted
                                                : AppColors.lightTextMuted,
                                          ),
                                    ),
                                    const SizedBox(height: 16),
                                    ElevatedButton.icon(
                                      onPressed: () =>
                                          _showAddEditTemplateDialog(
                                            systemTypes: systemTypes,
                                          ),
                                      icon: const Icon(Icons.add),
                                      label: const Text(
                                        'Buat Template Pertama',
                                      ),
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
                                  padding: const EdgeInsets.symmetric(
                                    vertical: 8,
                                  ),
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
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Row(
                                        mainAxisAlignment:
                                            MainAxisAlignment.spaceBetween,
                                        children: [
                                          Expanded(
                                            child: Text(
                                              tmpl.name,
                                              style: theme.textTheme.titleMedium
                                                  ?.copyWith(
                                                    fontWeight: FontWeight.bold,
                                                  ),
                                            ),
                                          ),
                                          Switch(
                                            value: tmpl.isActive,
                                            activeColor: AppColors.accent,
                                            onChanged: (val) async {
                                              final messenger =
                                                  ScaffoldMessenger.of(context);
                                              try {
                                                final repo = ref.read(
                                                  notificationRepositoryProvider,
                                                );
                                                await repo.toggleTemplateActive(
                                                  tmpl.id,
                                                );
                                                if (!mounted) return;
                                                ref.invalidate(
                                                  notificationTemplatesProvider,
                                                );
                                              } catch (e) {
                                                if (!mounted) return;
                                                messenger.showSnackBar(
                                                  SnackBar(
                                                    content: Text(e.toString()),
                                                  ),
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
                                            backgroundColor: AppColors.accent
                                                .withValues(alpha: 0.15),
                                            textColor: AppColors.accent,
                                          ),
                                          if (tmpl.imageUrl != null &&
                                              tmpl.imageUrl!.isNotEmpty)
                                            AppBadge(
                                              label: 'Gambar Terlampir',
                                              backgroundColor: AppColors.info
                                                  .withValues(alpha: 0.15),
                                              textColor: AppColors.info,
                                            ),
                                        ],
                                      ),
                                      const SizedBox(height: 8),
                                      Text(
                                        tmpl.title,
                                        style: theme.textTheme.bodyMedium
                                            ?.copyWith(
                                              fontWeight: FontWeight.bold,
                                            ),
                                      ),
                                      const SizedBox(height: 2),
                                      NotificationBodyText(
                                        text: tmpl.body,
                                        eventType: tmpl.eventType,
                                        style: theme.textTheme.bodySmall
                                            ?.copyWith(
                                              color: isDark
                                                  ? AppColors.darkTextMuted
                                                  : AppColors.lightTextMuted,
                                            ),
                                        onOpenUrl: (url) {
                                          ref
                                              .read(
                                                pushNotificationServiceProvider,
                                              )
                                              .openExternalUrl(url);
                                        },
                                      ),
                                      const SizedBox(height: 12),
                                      Row(
                                        mainAxisAlignment:
                                            MainAxisAlignment.end,
                                        children: [
                                          if (isSuperAdmin &&
                                              (tmpl.eventType ==
                                                      'manual_push' ||
                                                  tmpl.eventType ==
                                                      'waste_truck_arrival'))
                                            IconButton(
                                              icon: const Icon(
                                                Icons.send_outlined,
                                                size: 20,
                                                color: AppColors.info,
                                              ),
                                              tooltip:
                                                  'Test Notifikasi (Super Admin)',
                                              onPressed: () => _handleTestPush(
                                                eventType: tmpl.eventType,
                                                title: tmpl.title,
                                                body: tmpl.body,
                                                imageUrl: tmpl.imageUrl,
                                              ),
                                            ),
                                          if (tmpl.eventType == 'manual_push')
                                            IconButton(
                                              icon: const Icon(
                                                Icons.campaign_outlined,
                                                size: 20,
                                                color: AppColors.accent,
                                              ),
                                              tooltip:
                                                  'Kirim Notifikasi Siaran',
                                              onPressed: () =>
                                                  _confirmSendBroadcastDialog(
                                                    eventType: tmpl.eventType,
                                                    title: tmpl.title,
                                                    body: tmpl.body,
                                                    imageUrl: tmpl.imageUrl,
                                                  ),
                                            ),
                                          IconButton(
                                            icon: const Icon(
                                              Icons.preview_rounded,
                                              size: 20,
                                              color: AppColors.info,
                                            ),
                                            tooltip: 'Preview Banner',
                                            onPressed: () => _showPreviewBanner(
                                              title: tmpl.title,
                                              body: tmpl.body,
                                              eventName: tmpl.name,
                                              imageUrl: tmpl.imageUrl,
                                            ),
                                          ),
                                          IconButton(
                                            icon: const Icon(
                                              Icons.edit_outlined,
                                              size: 20,
                                              color: AppColors.accent,
                                            ),
                                            tooltip: 'Ubah Template',
                                            onPressed: () =>
                                                _showAddEditTemplateDialog(
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
                                            onPressed: () =>
                                                _confirmDeleteTemplate(tmpl),
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

                    // TAB 3: PENJADWALAN NOTIFIKASI
                    RefreshIndicator(
                      onRefresh: () async {
                        ref.invalidate(notificationSchedulesProvider);
                        _loadSchedules();
                      },
                      child: _buildSchedulesTab(
                        theme: theme,
                        isDark: isDark,
                        templatesAsync: templatesAsync,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),

        ],
      ),
    );
  }

  // ========================================================================
  // TAB 4: PENJADWALAN NOTIFIKASI
  // ========================================================================

  Widget _buildSchedulesTab({
    required ThemeData theme,
    required bool isDark,
    required AsyncValue<List<NotificationTemplate>> templatesAsync,
  }) {
    final textColor = isDark ? AppColors.darkText : AppColors.lightText;
    final muted = isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted;

    if (_isLoadingSchedules) {
      return const Center(child: LoadingIndicator());
    }
    if (_schedulesError != null) {
      return ErrorState(message: _schedulesError!, onRetry: _loadSchedules);
    }
    if (_schedules.isEmpty) {
      return EmptyState(
        icon: Icons.schedule_outlined,
        title: 'Belum Ada Jadwal Notifikasi',
        description:
            'Buat jadwal untuk mengirim notifikasi otomatis (misal pengingat pembayaran iuran) sesuai tanggal dan jam yang ditentukan.',
        actionText: 'Buat Jadwal',
        onAction: () => _showAddEditScheduleDialog(),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: _schedules.length,
      itemBuilder: (context, index) {
        final s = _schedules[index];
        final eventName = _knownEventNames[s.eventType] ?? s.eventType;
        final days = s.sendDays.isNotEmpty ? s.sendDays.join(', ') : '-';
        return AppCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Top row: name + switch toggle
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Text(
                      s.name,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: textColor,
                      ),
                    ),
                  ),
                  Switch(
                    value: s.isActive,
                    activeColor: AppColors.accent,
                    onChanged: (val) async {
                      final messenger = ScaffoldMessenger.of(context);
                      try {
                        final repo = ref.read(notificationRepositoryProvider);
                        await repo.toggleScheduleActive(s.id);
                        if (!mounted) return;
                        ref.invalidate(notificationSchedulesProvider);
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
              const SizedBox(height: 8),
              // Info section
              Row(
                children: [
                  Text(
                    eventName,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: textColor,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              Text(
                'Tanggal: $days | Jam: ${s.sendHour.toString().padLeft(2, '0')}:${s.sendMinute.toString().padLeft(2, '0')} WIB',
                style: theme.textTheme.bodySmall?.copyWith(color: muted),
              ),
              if (s.timezone.isNotEmpty && s.timezone != 'Asia/Jakarta') ...[
                const SizedBox(height: 2),
                Text(
                  'Timezone: ${s.timezone}',
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: muted,
                    fontSize: 11,
                  ),
                ),
              ],
              if (s.lastExecutedAt != null) ...[
                const SizedBox(height: 4),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Icon(
                      Icons.access_time,
                      size: 14,
                      color: AppColors.accent,
                    ),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Text(
                                'Terakhir: ${_formatDateTime(s.lastExecutedAt!)}',
                                style: theme.textTheme.bodySmall?.copyWith(color: muted),
                              ),
                              if (s.lastExecutedStatus != null) ...[
                                const SizedBox(width: 6),
                                Container(
                                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                                  decoration: BoxDecoration(
                                    color: s.lastExecutedStatus == 'success'
                                        ? AppColors.success.withValues(alpha: 0.12)
                                        : s.lastExecutedStatus == 'partial'
                                        ? AppColors.warning.withValues(alpha: 0.15)
                                        : s.lastExecutedStatus == 'no_device'
                                        ? Colors.blue.withValues(alpha: 0.12)
                                        : s.lastExecutedStatus == 'no_recipients'
                                        ? Colors.grey.withValues(alpha: 0.12)
                                        : AppColors.danger.withValues(alpha: 0.12),
                                    borderRadius: BorderRadius.circular(4),
                                    border: Border.all(
                                      color: s.lastExecutedStatus == 'success'
                                          ? AppColors.success.withValues(alpha: 0.3)
                                          : s.lastExecutedStatus == 'partial'
                                          ? AppColors.warning.withValues(alpha: 0.3)
                                          : s.lastExecutedStatus == 'no_device'
                                          ? Colors.blue.withValues(alpha: 0.3)
                                          : s.lastExecutedStatus == 'no_recipients'
                                          ? Colors.grey.withValues(alpha: 0.3)
                                          : AppColors.danger.withValues(alpha: 0.3),
                                    ),
                                  ),
                                  child: Text(
                                    s.lastExecutedStatus == 'success'
                                        ? 'BERHASIL'
                                        : s.lastExecutedStatus == 'partial'
                                        ? 'SEBAGIAN'
                                        : s.lastExecutedStatus == 'no_device'
                                        ? 'IN-APP'
                                        : s.lastExecutedStatus == 'no_recipients'
                                        ? 'NIHIL'
                                        : 'GAGAL',
                                    style: TextStyle(
                                      fontSize: 10,
                                      fontWeight: FontWeight.bold,
                                      color: s.lastExecutedStatus == 'success'
                                          ? AppColors.success
                                          : s.lastExecutedStatus == 'partial'
                                          ? AppColors.warning
                                          : s.lastExecutedStatus == 'no_device'
                                          ? Colors.blue.shade700
                                          : s.lastExecutedStatus == 'no_recipients'
                                          ? Colors.grey.shade700
                                          : AppColors.danger,
                                    ),
                                  ),
                                ),
                              ],
                            ],
                          ),
                          if (s.lastExecutedSummary != null) ...[
                            const SizedBox(height: 2),
                            Text(
                              s.lastExecutedStatus == 'no_recipients'
                                  ? 'Tidak ada penerima yang memenuhi kriteria'
                                  : s.lastExecutedStatus == 'no_device'
                                  ? '${s.lastExecutedSummary!.noDeviceCount} warga (tanpa perangkat)'
                                  : '${s.lastExecutedSummary!.sentCount} terkirim'
                                      '${s.lastExecutedSummary!.failedCount > 0 ? ', ${s.lastExecutedSummary!.failedCount} gagal' : ''}'
                                      '${s.lastExecutedSummary!.noDeviceCount > 0 ? ', ${s.lastExecutedSummary!.noDeviceCount} tanpa perangkat' : ''}',
                              style: theme.textTheme.bodySmall?.copyWith(color: muted, fontSize: 11),
                            ),
                          ],
                          if (s.lastExecutedError != null && s.lastExecutedError!.isNotEmpty && s.lastExecutedSummary == null) ...[
                            const SizedBox(height: 2),
                            Text(
                              s.lastExecutedError!,
                              style: theme.textTheme.bodySmall?.copyWith(color: AppColors.danger, fontSize: 11),
                            ),
                          ],
                        ],
                      ),
                    ),
                  ],
                ),
              ],
              const SizedBox(height: 12),
              // Action buttons - horizontal below info
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  IconButton(
                    icon: const Icon(
                      Icons.preview_rounded,
                      size: 20,
                      color: AppColors.info,
                    ),
                    tooltip: 'Preview Notifikasi',
                    onPressed: () {
                      final tmpl = templatesAsync.value
                          ?.where((t) => t.id == s.templateId)
                          .firstOrNull;
                      _showPreviewBanner(
                        title: tmpl?.title ?? '',
                        body: tmpl?.body ?? '',
                        eventName: eventName,
                        imageUrl: tmpl?.imageUrl,
                      );
                    },
                  ),
                  IconButton(
                    icon: const Icon(
                      Icons.edit_outlined,
                      size: 20,
                      color: AppColors.accent,
                    ),
                    tooltip: 'Ubah Jadwal',
                    onPressed: () => _showAddEditScheduleDialog(schedule: s),
                  ),
                  IconButton(
                    icon: const Icon(
                      Icons.delete_outline,
                      size: 20,
                      color: AppColors.danger,
                    ),
                    tooltip: 'Hapus Jadwal',
                    onPressed: () => _confirmDeleteSchedule(s),
                  ),
                ],
              ),
            ],
          ),
        );
      },
    );
  }

  Future<void> _toggleSchedule(NotificationSchedule schedule) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(
          schedule.isActive ? 'Nonaktifkan Jadwal' : 'Aktifkan Jadwal',
        ),
        content: Text(
          schedule.isActive
              ? 'Apakah Anda yakin ingin menonaktifkan jadwal "${schedule.name}"? Notifikasi tidak akan dikirim sesuai jadwal.'
              : 'Apakah Anda yakin ingin mengaktifkan jadwal "${schedule.name}"? Notifikasi akan dikirim sesuai jadwal yang ditetapkan.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            style: TextButton.styleFrom(foregroundColor: AppColors.accent),
            child: Text(schedule.isActive ? 'Nonaktifkan' : 'Aktifkan'),
          ),
        ],
      ),
    );
    if (confirm != true || !mounted) return;

    try {
      final repo = ref.read(notificationRepositoryProvider);
      await repo.toggleScheduleActive(schedule.id);
      if (!mounted) return;
      setState(() {
        _schedules = _schedules.map((s) {
          if (s.id == schedule.id) {
            return s.copyWith(isActive: !s.isActive);
          }
          return s;
        }).toList();
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'Jadwal ${schedule.isActive ? 'dinonaktifkan' : 'diaktifkan'}',
          ),
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal mengubah status jadwal: $e')),
        );
      }
    }
  }

  Future<void> _confirmDeleteSchedule(NotificationSchedule schedule) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Hapus Jadwal'),
        content: Text(
          'Hapus jadwal "${schedule.name}"? Tindakan tidak dapat dibatalkan.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Batal'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            style: TextButton.styleFrom(foregroundColor: AppColors.danger),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirm == true && mounted) {
      try {
        final repo = ref.read(notificationRepositoryProvider);
        await repo.deleteSchedule(schedule.id);
        if (mounted) {
          setState(() {
            _schedules.removeWhere((s) => s.id == schedule.id);
          });
          ref.invalidate(notificationSchedulesProvider);
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(const SnackBar(content: Text('Jadwal dihapus')));
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('Gagal menghapus jadwal: $e')));
        }
      }
    }
  }

  Future<void> _showAddEditScheduleDialog({
    NotificationSchedule? schedule,
  }) async {
    final isEdit = schedule != null;
    final repo = ref.read(notificationRepositoryProvider);

    final nameCtrl = TextEditingController(text: schedule?.name ?? '');
    final timezoneCtrl = TextEditingController(
      text: schedule?.timezone ?? 'Asia/Jakarta',
    );
    final selectedDays = <int>{}..addAll(schedule?.sendDays ?? []);
    var sendHour = schedule?.sendHour ?? 8;
    var sendMinute = schedule?.sendMinute ?? 0;
    var isActive = schedule?.isActive ?? true;

    // Fetch system types and templates upfront.
    List<SystemNotificationType> systemTypes = [];
    List<NotificationTemplate> templates = [];
    try {
      systemTypes = await ref.read(notificationSystemTypesProvider.future);
      templates = await ref.read(notificationTemplatesProvider.future);
    } catch (_) {}

    String? selectedEventType = schedule?.eventType;
    if (selectedEventType == null && systemTypes.isNotEmpty) {
      selectedEventType = systemTypes.first.eventType;
    }
    String? selectedTemplateId = schedule?.templateId;
    if (selectedTemplateId == null && templates.isNotEmpty) {
      selectedTemplateId = templates.first.id;
    }

    if (!mounted) return;

    final result = await showDialog<bool>(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (dialogContext, setSheetState) {
            final activeTemplates = templates.where((t) => t.isActive).toList();
            if (selectedTemplateId == null && activeTemplates.isNotEmpty) {
              selectedTemplateId = activeTemplates.first.id;
            } else if (selectedTemplateId != null &&
                !activeTemplates.any((t) => t.id == selectedTemplateId)) {
              selectedTemplateId = activeTemplates.isNotEmpty
                  ? activeTemplates.first.id
                  : null;
            }

            return AlertDialog(
              title: Text(isEdit ? 'Ubah Jadwal' : 'Buat Jadwal Baru'),
              content: SizedBox(
                width: 440,
                child: SingleChildScrollView(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      TextField(
                        controller: nameCtrl,
                        decoration: const InputDecoration(
                          labelText: 'Nama Jadwal *',
                          hintText: 'Contoh: Pengingat Iuran Bulanan',
                        ),
                        maxLength: 128,
                      ),
                      const SizedBox(height: 12),
                      // Event type
                      if (systemTypes.isNotEmpty)
                        DropdownButtonFormField<String>(
                          value: selectedEventType,
                          items: systemTypes
                              .map(
                                (t) => DropdownMenuItem(
                                  value: t.eventType,
                                  child: Text(t.name),
                                ),
                              )
                              .toList(),
                          decoration: const InputDecoration(
                            labelText: 'Jenis Notifikasi *',
                          ),
                          onChanged: (v) {
                            setSheetState(() {
                              selectedEventType = v;
                              selectedTemplateId = null;
                            });
                          },
                        ),
                      if (systemTypes.isEmpty)
                        const Text(
                          'Tidak ada jenis notifikasi tersedia.',
                          style: TextStyle(color: AppColors.danger),
                        ),
                      const SizedBox(height: 12),
                      // Template
                      if (activeTemplates.isNotEmpty)
                        DropdownButtonFormField<String>(
                          value: selectedTemplateId,
                          items: activeTemplates
                              .map(
                                (t) => DropdownMenuItem(
                                  value: t.id,
                                  child: Text('${t.name} (${t.eventType})'),
                                ),
                              )
                              .toList(),
                          decoration: const InputDecoration(
                            labelText: 'Template Pesan *',
                          ),
                          onChanged: (v) => selectedTemplateId = v,
                        ),
                      if (activeTemplates.isEmpty)
                        const Text(
                          'Tidak ada template aktif tersedia.',
                          style: TextStyle(color: AppColors.danger),
                        ),
                      const SizedBox(height: 12),
                      // Send days
                      const Text(
                        'Tanggal Pengiriman (1-31)',
                        style: TextStyle(
                          fontWeight: FontWeight.bold,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(height: 6),
                      Wrap(
                        spacing: 6,
                        runSpacing: 6,
                        children: List.generate(31, (i) {
                          final day = i + 1;
                          final selected = selectedDays.contains(day);
                          return ChoiceChip(
                            label: Text(day.toString()),
                            selected: selected,
                            onSelected: (sel) {
                              setSheetState(() {
                                if (sel) {
                                  selectedDays.add(day);
                                } else {
                                  selectedDays.remove(day);
                                }
                              });
                            },
                            selectedColor: AppColors.accent.withValues(
                              alpha: 0.2,
                            ),
                          );
                        }),
                      ),
                      const SizedBox(height: 12),
                      // Time + timezone
                      Row(
                        children: [
                          Expanded(
                            child: DropdownButtonFormField<int>(
                              value: sendHour,
                              items: List.generate(24, (h) => h)
                                  .map(
                                    (h) => DropdownMenuItem(
                                      value: h,
                                      child: Text(
                                        '${h.toString().padLeft(2, '0')}:00',
                                      ),
                                    ),
                                  )
                                  .toList(),
                              decoration: const InputDecoration(
                                labelText: 'Jam',
                              ),
                              onChanged: (v) => sendHour = v ?? sendHour,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: DropdownButtonFormField<int>(
                              value: sendMinute,
                              items: List.generate(60, (m) => m)
                                  .map(
                                    (m) => DropdownMenuItem(
                                      value: m,
                                      child: Text(
                                        ':${m.toString().padLeft(2, '0')}',
                                      ),
                                    ),
                                  )
                                  .toList(),
                              decoration: const InputDecoration(
                                labelText: 'Menit',
                              ),
                              onChanged: (v) => sendMinute = v ?? sendMinute,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: TextField(
                              controller: timezoneCtrl,
                              decoration: const InputDecoration(
                                labelText: 'Timezone',
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      SwitchListTile(
                        title: const Text('Aktif'),
                        subtitle: const Text(
                          'Jadwal akan dijalankan sesuai tanggal dan jam yang ditetapkan.',
                        ),
                        value: isActive,
                        onChanged: (v) => setSheetState(() => isActive = v),
                        activeColor: AppColors.accent,
                      ),
                    ],
                  ),
                ),
              ),
              actions: [
                ElevatedButton(
                  onPressed: () => Navigator.of(dialogContext).pop(false),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.danger,
                    foregroundColor: Colors.white,
                  ),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  onPressed: _isSavingSchedule
                      ? null
                      : () async {
                          if (nameCtrl.text.trim().isEmpty) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(
                                content: Text('Nama jadwal wajib diisi'),
                              ),
                            );
                            return;
                          }
                          if (selectedDays.isEmpty) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(
                                content: Text(
                                  'Pilih minimal satu tanggal pengiriman',
                                ),
                              ),
                            );
                            return;
                          }
                          if (selectedEventType == null ||
                              selectedTemplateId == null) {
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(
                                content: Text(
                                  'Jenis notifikasi dan template wajib dipilih',
                                ),
                              ),
                            );
                            return;
                          }
                          final tz = timezoneCtrl.text.trim().isNotEmpty
                              ? timezoneCtrl.text.trim()
                              : 'Asia/Jakarta';
                          try {
                            setSheetState(() => _isSavingSchedule = true);
                            final input = CreateNotificationScheduleInput(
                              eventType: selectedEventType!,
                              templateId: selectedTemplateId!,
                              name: nameCtrl.text.trim(),
                              sendDays: selectedDays.toList()..sort(),
                              sendHour: sendHour,
                              sendMinute: sendMinute,
                              timezone: tz,
                            );
                            if (isEdit) {
                              await repo.updateSchedule(
                                schedule.id,
                                UpdateNotificationScheduleInput(
                                  name: nameCtrl.text.trim(),
                                  isActive: isActive,
                                  sendDays: selectedDays.toList()..sort(),
                                  sendHour: sendHour,
                                  sendMinute: sendMinute,
                                  timezone: tz,
                                ),
                              );
                            } else {
                              await repo.createSchedule(input);
                            }
                            if (dialogContext.mounted) {
                              Navigator.of(dialogContext).pop(true);
                            }
                            if (mounted) {
                              ref.invalidate(notificationSchedulesProvider);
                              _loadSchedules();
                              if (context.mounted) {
                                ScaffoldMessenger.of(context).showSnackBar(
                                  SnackBar(
                                    content: Text(
                                      'Jadwal ${isEdit ? 'diperbarui' : 'dibuat'}',
                                    ),
                                  ),
                                );
                              }
                            }
                          } catch (e) {
                            if (mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text('Gagal menyimpan jadwal: $e'),
                                ),
                              );
                            }
                          } finally {
                            if (mounted) {
                              setState(() => _isSavingSchedule = false);
                            }
                          }
                        },
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.accent,
                    foregroundColor: Colors.white,
                  ),
                  child: _isSavingSchedule
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            valueColor: AlwaysStoppedAnimation<Color>(
                              Colors.white,
                            ),
                          ),
                        )
                      : Text(isEdit ? 'Simpan Perubahan' : 'Buat Jadwal'),
                ),
              ],
            );
          },
        );
      },
    );
    if (result == true && mounted) {
      _loadSchedules();
    }
  }

  String _formatDateTime(DateTime dt) {
    return formatJakartaIsoDateHour(dt);
  }

  Widget _buildLogsTab({
    required ThemeData theme,
    required bool isDark,
    required bool isAdmin,
    required List<SystemNotificationType> systemTypes,
    String? category,
  }) {
    return RefreshIndicator(
      onRefresh: () => _loadLogs(category: category),
      child: Column(
        children: [
          if (!isAdmin)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: Row(
                children: [
                  Expanded(
                    flex: 3,
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        isExpanded: true,
                        value: _selectedLogEventType,
                        items: [
                          const DropdownMenuItem(
                            value: '',
                            child: Text('Semua Jenis'),
                          ),
                          if (systemTypes.isNotEmpty)
                            ...systemTypes.map((st) {
                              return DropdownMenuItem(
                                value: st.eventType,
                                child: Text(st.name),
                              );
                            })
                          else
                            ..._knownEventNames.entries.map((e) {
                              return DropdownMenuItem(
                                value: e.key,
                                child: Text(e.value),
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
                  const SizedBox(width: 8),
                  Expanded(
                    flex: 2,
                    child: DropdownButtonHideUnderline(
                      child: DropdownButton<String>(
                        isExpanded: true,
                        value: _selectedLogStatus,
                        items: const [
                          DropdownMenuItem(value: 'all', child: Text('Semua')),
                          DropdownMenuItem(
                            value: 'unread',
                            child: Text('Belum Dibaca'),
                          ),
                          DropdownMenuItem(
                            value: 'read',
                            child: Text('Sudah Dibaca'),
                          ),
                        ],
                        onChanged: (val) {
                          if (val != null) {
                            setState(() {
                              _selectedLogStatus = val;
                            });
                            _loadLogs();
                          }
                        },
                      ),
                    ),
                  ),
                  if (!isAdmin)
                    IconButton(
                      icon: const Icon(Icons.done_all_rounded, size: 20),
                      tooltip: 'Tandai Semua Dibaca',
                      onPressed: _markAllAsRead,
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
                ? ErrorState(message: _logsError!, onRetry: _loadLogs)
                : _logs.isEmpty
                ? EmptyState(
                    icon: Icons.history_toggle_off_rounded,
                    title: category == null
                        ? 'Belum Ada Riwayat Notifikasi'
                        : category == 'mine'
                        ? 'Belum Ada Riwayat Saya'
                        : 'Belum Ada Riwayat Sistem',
                    description: category == null
                        ? 'Riwayat tercatat secara otomatis setiap kali notifikasi push dikirimkan ke warga.'
                        : category == 'mine'
                        ? 'Aktivitas notifikasi yang Anda lakukan akan tercatat di sini.'
                        : 'Notifikasi terjadwal yang dikirim sistem akan tercatat di sini.',
                  )
                : ListView.builder(
                    key: PageStorageKey<String>(
                      'notification_logs_${category ?? 'personal'}',
                    ),
                    padding: const EdgeInsets.all(16),
                    itemCount: _logs.length,
                    itemBuilder: (context, index) {
                      final log = _logs[index];
                      // Warga (non-admin): always hide internal delivery status, show TERKIRIM for everything.
                      // Admin/Ketua RT: show detailed status from the ledger.
                      final deliveryStatus = log.status.toUpperCase();
                      String deliveryStatusLabel;
                      Color deliveryStatusColor;
                      if (!isAdmin) {
                        deliveryStatusLabel = 'TERKIRIM';
                        deliveryStatusColor = AppColors.success;
                      } else {
                        deliveryStatusLabel = switch (deliveryStatus) {
                          'SENT' => 'TERKIRIM',
                          'FAILED' => 'GAGAL',
                          'PARTIAL' => 'SEBAGIAN GAGAL',
                          'UNKNOWN' => 'TIDAK DIKETAHUI',
                          'NO_DEVICE' => 'PERANGKAT TIDAK TERSEDIA',
                          'APPROVED' => 'DISETUJUI',
                          'REJECTED' => 'DITOLAK',
                          'RECORDED' => 'TERCATAT',
                          _ => deliveryStatus,
                        };
                        deliveryStatusColor = switch (deliveryStatus) {
                          'SENT' || 'APPROVED' || 'RECORDED' => AppColors.success,
                          'FAILED' || 'REJECTED' => AppColors.danger,
                          'PARTIAL' => AppColors.warning,
                          'NO_DEVICE' => AppColors.warning,
                          _ => AppColors.lightTextMuted,
                        };
                      }
                      final showReadState = category == null;
                      final dateStr = formatJakartaDateTime(log.createdAt);
                      final activityRelationLabel =
                          switch (log.activityRelation) {
                            'sent_by_me' => 'Dikirim oleh Saya',
                            'received_by_me' => 'Diterima oleh Saya',
                            'performed_by_me' => 'Tindakan Saya',
                            _ => null,
                          };

                      return Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: InkWell(
                          key: Key('notification_log_item_${log.id}'),
                          borderRadius: BorderRadius.circular(AppRadius.base),
                          onTap: () => _openLogDetail(log),
                          child: AppCard(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Wrap(
                                  alignment: WrapAlignment.spaceBetween,
                                  crossAxisAlignment: WrapCrossAlignment.center,
                                  spacing: 8,
                                  runSpacing: 4,
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
                                    Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        if (showReadState && !log.isRead) ...[
                                          const Padding(
                                            padding: EdgeInsets.only(right: 6),
                                            child: AppBadge(
                                              label: 'BARU',
                                              backgroundColor: AppColors.accent,
                                              textColor: Colors.white,
                                            ),
                                          ),
                                        ],
                                        AppBadge(
                                          label: deliveryStatusLabel,
                                          backgroundColor: deliveryStatusColor,
                                          textColor: Colors.white,
                                        ),
                                      ],
                                    ),
                                  ],
                                ),
                                const SizedBox(height: 6),
                                Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        if (showReadState && !log.isRead)
                                          Container(
                                            width: 8,
                                            height: 8,
                                            margin: const EdgeInsets.only(right: 6),
                                            decoration: const BoxDecoration(
                                              color: AppColors.accent,
                                              shape: BoxShape.circle,
                                            ),
                                          ),
                                        Expanded(
                                          child: Text(
                                            log.title,
                                            maxLines: 2,
                                            overflow: TextOverflow.ellipsis,
                                            style: theme.textTheme.bodyMedium
                                                ?.copyWith(
                                                  fontWeight:
                                                      !showReadState || log.isRead
                                                      ? FontWeight.w600
                                                      : FontWeight.bold,
                                                ),
                                          ),
                                        ),
                                      ],
                                    ),
                                    if (activityRelationLabel != null)
                                      Padding(
                                        padding: const EdgeInsets.only(top: 2),
                                        child: Text(
                                          activityRelationLabel,
                                          style: theme.textTheme.labelSmall
                                              ?.copyWith(
                                                color: AppColors.accent,
                                                fontWeight: FontWeight.w600,
                                              ),
                                        ),
                                      ),
                                  ],
                                ),
                                const SizedBox(height: 2),
                                NotificationBodyText(
                                  text: log.body,
                                  eventType: log.eventType,
                                  dedupKey: log.dedupKey,
                                  style: theme.textTheme.bodySmall?.copyWith(
                                    color: isDark
                                        ? AppColors.darkTextMuted
                                        : AppColors.lightTextMuted,
                                  ),
                                  onOpenUrl: (url) {
                                    ref
                                        .read(pushNotificationServiceProvider)
                                        .openExternalUrl(url);
                                  },
                                ),
                                const SizedBox(height: 6),
                                Row(
                                  mainAxisAlignment:
                                      MainAxisAlignment.spaceBetween,
                                  children: [
                                    Text(
                                      _knownEventNames[log.eventType] ??
                                          log.eventType,
                                      style: TextStyle(
                                        fontSize: 11,
                                        color: isDark
                                            ? AppColors.darkTextMuted
                                            : AppColors.lightTextMuted,
                                      ),
                                    ),
                                    if (isAdmin)
                                      Flexible(
                                        child: Text(
                                          log.deliveryCountsAvailable
                                              ? 'Terkirim: ${log.sentCount} / ${log.recipientsCount} penerima'
                                                    '${log.failedCount > 0 ? ' - Gagal: ${log.failedCount}' : ''}'
                                              : 'Hasil pengiriman tidak dicatat',
                                          textAlign: TextAlign.end,
                                          maxLines: 2,
                                          overflow: TextOverflow.ellipsis,
                                          style: TextStyle(
                                            fontSize: 11,
                                            color: isDark
                                                ? AppColors.darkTextMuted
                                                : AppColors.lightTextMuted,
                                          ),
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
                  ),
          ),
        ],
      ),
    );
  }
}
