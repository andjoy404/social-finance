import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

class KegiatanDetailScreen extends ConsumerStatefulWidget {
  final String activityId;

  const KegiatanDetailScreen({super.key, required this.activityId});

  @override
  ConsumerState<KegiatanDetailScreen> createState() =>
      _KegiatanDetailScreenState();
}

class _KegiatanDetailScreenState extends ConsumerState<KegiatanDetailScreen> {
  CommunityActivity? _activity;
  bool _isLoading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    if (Platform.environment.containsKey('FLUTTER_TEST')) {
      _isLoading = false;
    } else {
      _load();
    }
  }

  Future<void> _load() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });
    try {
      final repo = ref.read(communityRepositoryProvider);
      final act = await repo.getActivity(widget.activityId);
      if (mounted) {
        setState(() {
          _activity = act;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _isLoading = false;
        });
      }
    }
  }

  AppBadge _statusBadge(String status) {
    switch (status) {
      case 'upcoming':
        return const AppBadge(
          label: 'Mendatang',
          backgroundColor: Color(0x223B82F6),
          textColor: Colors.blue,
        );
      case 'ongoing':
        return const AppBadge(
          label: 'Berlangsung',
          backgroundColor: Color(0x22F59E0B),
          textColor: Colors.amber,
        );
      case 'completed':
        return const AppBadge(
          label: 'Selesai',
          backgroundColor: Color(0x2210B981),
          textColor: AppColors.income,
        );
      case 'cancelled':
        return const AppBadge(
          label: 'Dibatalkan',
          backgroundColor: Color(0x22EF4444),
          textColor: AppColors.danger,
        );
      default:
        return AppBadge(
          label: status,
          backgroundColor: Color(0x226B7280),
          textColor: Colors.grey,
        );
    }
  }

  String _formatDateTime(DateTime t) {
    final d = t.toLocal();
    const months = [
      'Jan',
      'Feb',
      'Mar',
      'Apr',
      'Mei',
      'Jun',
      'Jul',
      'Agu',
      'Sep',
      'Okt',
      'Nov',
      'Des',
    ];
    final day = d.day.toString().padLeft(2, '0');
    final hour = d.hour.toString().padLeft(2, '0');
    final minute = d.minute.toString().padLeft(2, '0');
    return '$day ${months[d.month - 1]} ${d.year}, $hour:$minute';
  }

  void _openImageViewer(BuildContext context, String imageRef, String title) {
    showDialog<void>(
      context: context,
      useSafeArea: false,
      builder: (dialogCtx) => Scaffold(
        backgroundColor: Colors.black,
        appBar: AppBar(
          backgroundColor: Colors.black.withValues(alpha: 0.7),
          foregroundColor: Colors.white,
          title: Text(
            title,
            style: const TextStyle(fontSize: 16, color: Colors.white),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          leading: IconButton(
            icon: const Icon(Icons.close),
            tooltip: 'Tutup',
            onPressed: () => Navigator.of(dialogCtx).pop(),
          ),
        ),
        body: Center(
          child: InteractiveViewer(
            panEnabled: true,
            minScale: 0.5,
            maxScale: 4.0,
            child: CommunityImage(
              imageRef: imageRef,
              fit: BoxFit.contain,
              alt: title,
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final a = _activity;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (didPop) return;
        if (context.canPop()) {
          context.pop();
        } else {
          context.go('/home/kegiatan');
        }
      },
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Detail Kegiatan'),
          leading: BackButton(
            onPressed: () {
              if (context.canPop()) {
                context.pop();
              } else {
                context.go('/home/kegiatan');
              }
            },
          ),
        ),
        body: _isLoading
            ? const Center(child: CircularProgressIndicator.adaptive())
            : _error != null
            ? Center(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.base),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(
                        Icons.error_outline,
                        size: 40,
                        color: AppColors.danger,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Kegiatan tidak dapat dibuka',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.titleMedium,
                      ),
                      const SizedBox(height: 4),
                      Text(_error!, textAlign: TextAlign.center),
                      const SizedBox(height: 12),
                      ElevatedButton(
                        onPressed: _load,
                        child: const Text('Coba Lagi'),
                      ),
                    ],
                  ),
                ),
              )
            : a == null
            ? const SizedBox.shrink()
            : ListView(
                padding: const EdgeInsets.all(AppSpacing.base),
                children: [
                  if (a.posterUrl != null && a.posterUrl!.isNotEmpty) ...[
                    ClipRRect(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: InkWell(
                        onTap: () => _openImageViewer(
                          context,
                          a.posterUrl!,
                          a.title,
                        ),
                        child: Container(
                          width: double.infinity,
                          height: 220,
                          color: Theme.of(context).brightness == Brightness.dark
                              ? AppColors.darkSurface
                              : AppColors.lightSurfaceSubtle,
                          alignment: Alignment.center,
                          child: Stack(
                            alignment: Alignment.center,
                            children: [
                              CommunityImage(
                                imageRef: a.posterUrl!,
                                width: double.infinity,
                                height: 220,
                                fit: BoxFit.contain,
                                alt: 'Poster kegiatan ${a.title}',
                              ),
                              Positioned(
                                right: 8,
                                bottom: 8,
                                child: Container(
                                  padding: const EdgeInsets.all(4),
                                  decoration: BoxDecoration(
                                    color: Colors.black.withValues(alpha: 0.5),
                                    borderRadius: BorderRadius.circular(AppRadius.xs),
                                  ),
                                  child: const Icon(
                                    Icons.fullscreen,
                                    color: Colors.white,
                                    size: 18,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.base),
                  ],
                _statusBadge(a.status),
                const SizedBox(height: 8),
                Text(
                  a.title,
                  style: theme.textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: AppSpacing.base),
                if (a.description.isNotEmpty) ...[
                  Text(
                    a.description,
                    style: theme.textTheme.bodyLarge?.copyWith(height: 1.6),
                  ),
                  const SizedBox(height: AppSpacing.base),
                ],
                _infoRow(
                  context,
                  Icons.schedule,
                  'Waktu',
                  _formatDateTime(a.startDate),
                ),
                if (a.endDate != null)
                  _infoRow(
                    context,
                    Icons.flag_outlined,
                    'Berakhir',
                    _formatDateTime(a.endDate!),
                  ),
                if (a.location.isNotEmpty)
                  _infoRow(
                    context,
                    Icons.location_on_outlined,
                    'Lokasi',
                    a.location,
                  ),
                if (a.organizer != null && a.organizer!.isNotEmpty)
                  _infoRow(
                    context,
                    Icons.people_outline,
                    'Penyelenggara',
                    a.organizer!,
                  ),
              ],
            ),
      ),
    );
  }

  Widget _infoRow(
    BuildContext context,
    IconData icon,
    String label,
    String value,
  ) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 18, color: AppColors.accent),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  label,
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: AppColors.textMutedColor(context),
                  ),
                ),
                Text(value),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
