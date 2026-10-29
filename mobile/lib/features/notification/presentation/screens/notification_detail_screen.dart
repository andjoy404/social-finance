import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/error_state.dart';
import 'package:social_finance/core/widgets/loading_indicator.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';

import '../../data/models/notification_models.dart';
import '../../data/notification_repository.dart';

class NotificationDetailScreen extends ConsumerStatefulWidget {
  final String? notificationId;
  final String? initialTitle;
  final String? initialBody;
  final String? initialImageUrl;
  final String? initialEventType;
  final String? initialSentAt;
  final bool openedFromNotificationList;

  const NotificationDetailScreen({
    super.key,
    this.notificationId,
    this.initialTitle,
    this.initialBody,
    this.initialImageUrl,
    this.initialEventType,
    this.initialSentAt,
    this.openedFromNotificationList = false,
  });

  @override
  ConsumerState<NotificationDetailScreen> createState() =>
      _NotificationDetailScreenState();
}

class _NotificationDetailScreenState
    extends ConsumerState<NotificationDetailScreen> {
  NotificationLogEntry? _fetchedEntry;
  bool _isLoading = false;
  String? _fetchError;
  bool _hasFetched = false;

  @override
  void initState() {
    super.initState();
    _maybeFetchDetail();
  }

  @override
  void didUpdateWidget(covariant NotificationDetailScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.notificationId != widget.notificationId) {
      _hasFetched = false;
      _fetchedEntry = null;
      _fetchError = null;
      _maybeFetchDetail();
    }
  }

  Future<void> _maybeFetchDetail({bool force = false}) async {
    final id = widget.notificationId?.trim();
    if (id == null || id.isEmpty) return;
    if (_hasFetched && !force) return;
    _hasFetched = true;

    setState(() {
      _isLoading = true;
      _fetchError = null;
    });

    try {
      final repo = ref.read(notificationRepositoryProvider);
      final detail = await repo.fetchNotificationDetail(id);
      if (!mounted) return;
      setState(() {
        _fetchedEntry = detail;
        _isLoading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _fetchError = _mapErrorToUserMessage(e);
        _isLoading = false;
      });
    }
  }

  static String _mapErrorToUserMessage(dynamic error) {
    // Helper to classify HTTP status into categories
    int? classifyStatusCode(int? code) {
      if (code == null) return null;
      if (code == 401) return 401;
      if (code == 403) return 403;
      if (code == 404) return 404;
      if (code == 408 || code == 504) return 408;
      if (code >= 500) return 500;
      return null;
    }

    // Handle ServerException properly — use its .message property, not .toString()
    if (error is ServerException) {
      final classified = classifyStatusCode(error.statusCode);
      switch (classified) {
        case 401:
          return 'Sesi Anda telah berakhir. Silakan login kembali.';
        case 403:
          return 'Anda tidak memiliki akses untuk melihat notifikasi ini.';
        case 404:
          return 'Detail notifikasi tidak ditemukan.';
        case 408:
          return 'Permintaan timeout. Periksa koneksi Anda dan coba lagi.';
        case 500:
          return 'Server sedang mengalami gangguan. Coba lagi nanti.';
        default:
          final cleanMsg = error.message.trim();
          return cleanMsg.isNotEmpty
              ? cleanMsg
              : 'Terjadi kesalahan. Coba lagi nanti.';
      }
    }

    // Handle DioException with HTTP status codes
    if (error is DioException) {
      final status = error.response?.statusCode;
      final body = error.response?.data;
      // Try to extract user-friendly message from backend error response
      if (body is Map && body['error'] is Map) {
        final msg = body['error']['message'] as String?;
        if (msg != null && msg.isNotEmpty) {
          return msg;
        }
      }
      final classified = classifyStatusCode(status);
      switch (classified) {
        case 401:
          return 'Sesi Anda telah berakhir. Silakan login kembali.';
        case 403:
          return 'Anda tidak memiliki akses untuk melihat notifikasi ini.';
        case 404:
          return 'Detail notifikasi tidak ditemukan.';
        case 408:
          return 'Permintaan timeout. Periksa koneksi Anda dan coba lagi.';
        case 500:
          return 'Server sedang mengalami gangguan. Coba lagi nanti.';
        default:
          return 'Gagal memuat detail notifikasi. Periksa koneksi Anda.';
      }
    }

    // Generic fallback for unknown error types
    if (error is AppWithMessageError) return error.message;
    return 'Terjadi kesalahan tidak terduga. Silakan coba lagi.';
  }

  void _handleBack(BuildContext context) {
    final router = GoRouter.maybeOf(context);

    if (widget.openedFromNotificationList) {
      // Case B: Detail opened from in-app notification list
      if (router != null) {
        if (_hasNotificationListInStack(router) && router.canPop()) {
          router.pop();
        } else {
          router.go('/home/notifikasi');
        }
      } else if (Navigator.of(context).canPop()) {
        Navigator.of(context).pop();
      }
      return;
    }

    // Case A/C: Detail opened from push notification or other external source
    if (router != null) {
      router.go('/home');
    } else if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    }
  }

  bool _hasNotificationListInStack(GoRouter router) {
    return router.routerDelegate.currentConfiguration.routes.any(
      (route) => route is GoRoute && route.name == 'notifikasi',
    );
  }

  String _formatCategoryLabel(String? eventType) {
    switch ((eventType ?? '').trim()) {
      case 'manual_push':
        return 'Pengumuman RT';
      case 'waste_truck_arrival':
        return 'Kedatangan Truk Sampah';
      case 'iuran_bill_created':
        return 'Tagihan Iuran';
      case 'iuran_payment_submitted':
      case 'iuran_payment_verified':
        return 'Pembayaran Iuran';
      case 'test_self':
        return 'Uji Notifikasi';
      default:
        return '';
    }
  }

  String? _formatTimestamp(DateTime? dt, String? rawSentAt) {
    final effective =
        dt ?? (rawSentAt != null ? DateTime.tryParse(rawSentAt) : null);
    if (effective == null) return null;
    final local = effective.toLocal();
    final day = local.day.toString().padLeft(2, '0');
    final month = local.month.toString().padLeft(2, '0');
    final year = local.year;
    final hour = local.hour.toString().padLeft(2, '0');
    final minute = local.minute.toString().padLeft(2, '0');
    return '$day/$month/$year • $hour:$minute';
  }

  void _openFullScreenImage(BuildContext context, String resolvedImageUrl) {
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
                  child: Image.network(
                    resolvedImageUrl,
                    fit: BoxFit.contain,
                    errorBuilder: (_, _, _) => const Padding(
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

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final activeBaseUrl = ref.watch(apiClientProvider).baseUrl;

    final effectiveTitle = (_fetchedEntry?.title.trim().isNotEmpty == true)
        ? _fetchedEntry!.title.trim()
        : (widget.initialTitle?.trim() ?? '');
    final effectiveBody = (_fetchedEntry?.body.trim().isNotEmpty == true)
        ? _fetchedEntry!.body.trim()
        : (widget.initialBody?.trim() ?? '');
    final rawImageUrl = (_fetchedEntry?.imageUrl?.trim().isNotEmpty == true)
        ? _fetchedEntry!.imageUrl!.trim()
        : (widget.initialImageUrl?.trim());
    final resolvedImageUrl = ApiConfig.resolveImageUrl(
      rawImageUrl,
      fallbackBaseUrl: activeBaseUrl,
    );
    final effectiveEventType =
        (_fetchedEntry?.eventType.trim().isNotEmpty == true)
            ? _fetchedEntry!.eventType.trim()
            : (widget.initialEventType?.trim());
    final categoryLabel = _formatCategoryLabel(effectiveEventType);
    final formattedTime = _formatTimestamp(
      _fetchedEntry?.createdAt,
      widget.initialSentAt,
    );

    final hasDisplayableContent =
        effectiveTitle.isNotEmpty || effectiveBody.isNotEmpty;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        _handleBack(context);
      },
      child: Scaffold(
        appBar: AppBar(
          leading: IconButton(
            key: const Key('notification_detail_back_button'),
            icon: const Icon(Icons.arrow_back),
            tooltip: 'Kembali',
            onPressed: () => _handleBack(context),
          ),
          title: const MenuAppBarTitle(title: 'Detail Notifikasi'),
        ),
        body: Builder(
        builder: (context) {
          if (_isLoading && !hasDisplayableContent) {
            return const Center(child: LoadingIndicator());
          }

          if (_fetchError != null && !hasDisplayableContent) {
            return ErrorState(
              message: _fetchError!,
              onRetry: () => _maybeFetchDetail(force: true),
              actionText: 'Coba Lagi',
            );
          }

          if (!hasDisplayableContent) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const EmptyState(
                      icon: Icons.notifications_none_rounded,
                      title: 'Detail Notifikasi Tidak Tersedia',
                      description:
                          'Rincian notifikasi ini tidak ditemukan atau sudah tidak tersedia.',
                    ),
                    const SizedBox(height: 16),
                    ElevatedButton.icon(
                      key: const Key('notification_detail_home_button'),
                      onPressed: () => _handleBack(context),
                      icon: const Icon(Icons.home_outlined, size: 18),
                      label: const Text('Kembali ke Beranda'),
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

          return SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (categoryLabel.isNotEmpty || formattedTime != null) ...[
                        Wrap(
                          spacing: 8,
                          runSpacing: 6,
                          crossAxisAlignment: WrapCrossAlignment.center,
                          children: [
                            if (categoryLabel.isNotEmpty)
                              AppBadge(
                                label: categoryLabel,
                                backgroundColor:
                                    AppColors.accent.withValues(alpha: 0.15),
                                textColor: AppColors.accent,
                              ),
                            if (formattedTime != null)
                              Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(
                                    Icons.schedule_rounded,
                                    size: 14,
                                    color: isDark
                                        ? AppColors.darkTextMuted
                                        : AppColors.lightTextMuted,
                                  ),
                                  const SizedBox(width: 4),
                                  Text(
                                    formattedTime,
                                    style: theme.textTheme.bodySmall?.copyWith(
                                      color: isDark
                                          ? AppColors.darkTextMuted
                                          : AppColors.lightTextMuted,
                                    ),
                                  ),
                                ],
                              ),
                          ],
                        ),
                        const SizedBox(height: 12),
                      ],
                      if (effectiveTitle.isNotEmpty)
                        Text(
                          effectiveTitle,
                          style: theme.textTheme.titleLarge?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                      if (effectiveTitle.isNotEmpty &&
                          effectiveBody.isNotEmpty)
                        const SizedBox(height: 12),
                      if (effectiveBody.isNotEmpty)
                        Text(
                          effectiveBody,
                          style: theme.textTheme.bodyMedium?.copyWith(
                            height: 1.5,
                          ),
                        ),
                      if (rawImageUrl != null && rawImageUrl.isNotEmpty) ...[
                        const SizedBox(height: 16),
                        if (resolvedImageUrl != null)
                          Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              InkWell(
                                key: const Key('notification_detail_image_tap'),
                                borderRadius:
                                    BorderRadius.circular(AppRadius.base),
                                onTap: () => _openFullScreenImage(
                                  context,
                                  resolvedImageUrl,
                                ),
                                child: ClipRRect(
                                  borderRadius:
                                      BorderRadius.circular(AppRadius.base),
                                  child: Image.network(
                                    resolvedImageUrl,
                                    width: double.infinity,
                                    fit: BoxFit.cover,
                                    loadingBuilder:
                                        (context, child, loadingProgress) {
                                      if (loadingProgress == null) return child;
                                      return Container(
                                        height: 180,
                                        alignment: Alignment.center,
                                        decoration: BoxDecoration(
                                          color: isDark
                                              ? Colors.white
                                                  .withValues(alpha: 0.04)
                                              : Colors.black
                                                  .withValues(alpha: 0.04),
                                          borderRadius: BorderRadius.circular(
                                            AppRadius.base,
                                          ),
                                        ),
                                        child: const LoadingIndicator(),
                                      );
                                    },
                                    errorBuilder: (_, _, _) =>
                                        _buildImageFallback(theme, isDark),
                                  ),
                                ),
                              ),
                              const SizedBox(height: 6),
                              Row(
                                children: [
                                  Icon(
                                    Icons.zoom_out_map_rounded,
                                    size: 14,
                                    color: isDark
                                        ? AppColors.darkTextMuted
                                        : AppColors.lightTextMuted,
                                  ),
                                  const SizedBox(width: 4),
                                  Text(
                                    'Ketuk gambar untuk memperbesar',
                                    style: theme.textTheme.bodySmall?.copyWith(
                                      color: isDark
                                          ? AppColors.darkTextMuted
                                          : AppColors.lightTextMuted,
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          )
                        else
                          _buildImageFallback(theme, isDark),
                      ],
                    ],
                  ),
                ),
                const SizedBox(height: 16),
                OutlinedButton.icon(
                  key: const Key('notification_detail_close_button'),
                  onPressed: () => _handleBack(context),
                  icon: const Icon(Icons.arrow_back, size: 18),
                  label: const Text('Kembali'),
                ),
              ],
            ),
          );
        },
      ),
      ),
    );
  }

  Widget _buildImageFallback(ThemeData theme, bool isDark) {
    return Container(
      key: const Key('notification_detail_image_fallback'),
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? Colors.white.withValues(alpha: 0.04)
            : Colors.black.withValues(alpha: 0.04),
        borderRadius: BorderRadius.circular(AppRadius.base),
        border: Border.all(
          color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.broken_image_outlined,
            color: isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Gambar lampiran tidak dapat dimuat.',
              style: theme.textTheme.bodySmall?.copyWith(
                color:
                    isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
