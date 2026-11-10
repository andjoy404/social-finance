import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

const _sectionGap = AppSpacing.lg;
const _titleGap = AppSpacing.sm;

class DashboardScreen extends ConsumerStatefulWidget {
  final Future<void> Function()? onRefreshOverride;

  const DashboardScreen({super.key, this.onRefreshOverride});

  @override
  ConsumerState<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends ConsumerState<DashboardScreen> {
  final PageController _bannerController = PageController();
  int _activeBannerIndex = 0;
  Timer? _autoScrollTimer;

  @override
  void initState() {
    super.initState();
    _startAutoScroll();
  }

  void _startAutoScroll() {
    _autoScrollTimer?.cancel();
    _autoScrollTimer = Timer.periodic(const Duration(seconds: 6), (_) {
      if (!mounted || !_bannerController.hasClients) return;
      // With a single banner there is no scroll extent; nothing to autoplay.
      if (_bannerController.position.maxScrollExtent <= 0) return;
      final page = _bannerController.page?.round() ?? 0;
      _bannerController.animateToPage(
        page + 1,
        duration: const Duration(milliseconds: 600),
        curve: Curves.easeInOut,
      );
    });
  }

  @override
  void dispose() {
    _autoScrollTimer?.cancel();
    _bannerController.dispose();
    super.dispose();
  }

  Future<void> _handleRefresh() async {
    ref.invalidate(homeDashboardFutureProvider);
    if (widget.onRefreshOverride != null) {
      await widget.onRefreshOverride!();
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final userName = user?['name'] ?? 'Pengguna';
    final roleUser = user?['role'] as AppRole? ?? AppRole.warga;
    final rt = user?['rt'] ?? '';
    final rawRw = user?['rw']?.toString() ?? '';
    final rw = looksLikeUuid(rawRw)
        ? (resolveRwNumber(rawRw, user?['rw_memberships']) ?? '')
        : rawRw;
    final rtRwDisplay = [
      if (rt.isNotEmpty) 'RT $rt',
      if (rw.isNotEmpty) 'RW $rw',
    ].join(' / ');

    final dashboardAsync = ref.watch(homeDashboardFutureProvider);

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Beranda'),
      ),
      body: RefreshIndicator(
        key: const Key('dashboard-refresh-indicator'),
        onRefresh: _handleRefresh,
        child: SingleChildScrollView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.fromLTRB(
            AppSpacing.base,
            AppSpacing.base,
            AppSpacing.base,
            DashboardMedia.bottomPadding(context),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── 1. Greeting & User identity ──
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          crossAxisAlignment: CrossAxisAlignment.center,
                          children: [
                            Text(
                              'Selamat datang,',
                              style: theme.textTheme.bodyMedium,
                            ),
                            const Spacer(),
                            AppBadge(
                              label: appRoleDisplayName(roleUser),
                              isNeonStyle: isDark,
                            ),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text(userName, style: theme.textTheme.headlineMedium),
                        Text(
                          rtRwDisplay.isNotEmpty ? rtRwDisplay : 'RT / RW',
                          style: theme.textTheme.bodySmall,
                        ),
                      ],
                    ),
                  ),
                ],
              ),

              const SizedBox(height: AppSpacing.md),

              // ── 2. Active Banners (Carousel) ──
              dashboardAsync.when(
                data: (data) =>
                    _buildBannersCarousel(data.banners, theme, isDark),
                loading: () => _buildBannersSkeleton(isDark),
                error: (e, s) => _buildDefaultBanner(theme, isDark),
              ),

              const SizedBox(height: _sectionGap),

              // ── 3. Security Officer Status (Empty state waiting for Security Module) ──
              dashboardAsync.when(
                data: (data) =>
                    _buildSecurityCard(data.security, theme, isDark),
                loading: () => _buildSecurityCard(null, theme, isDark),
                error: (e, s) => _buildSecurityCard(null, theme, isDark),
              ),

              const SizedBox(height: _sectionGap),

              // ── 4. Dynamic Organizational Hierarchy Diagram ──
              dashboardAsync.when(
                data: (data) => _buildOrganizationHierarchy(
                  data.organization,
                  theme,
                  isDark,
                ),
                loading: () =>
                    const Center(child: CircularProgressIndicator.adaptive()),
                error: (e, s) => const SizedBox.shrink(),
              ),

              const SizedBox(height: _sectionGap),

              // ── 5. Upcoming Activities (3-5 items) ──
              dashboardAsync.when(
                data: (data) => _buildUpcomingActivities(
                  data.upcomingActivities,
                  theme,
                  isDark,
                ),
                loading: () => const SizedBox.shrink(),
                error: (e, s) => const SizedBox.shrink(),
              ),

              const SizedBox(height: AppSpacing.base),
            ],
          ),
        ),
      ),
    );
  }

  // ── Banners Carousel ──
  Widget _buildBannersCarousel(
    List<CommunityBanner> banners,
    ThemeData theme,
    bool isDark,
  ) {
    if (banners.isEmpty) {
      return _buildDefaultBanner(theme, isDark);
    }

    return Column(
      children: [
        SizedBox(
          height: 160,
          child: PageView.builder(
            controller: _bannerController,
            // Infinite virtual loop when more than one banner; a clamped
            // single page otherwise (no out-of-range animateToPage).
            itemCount: banners.length > 1 ? null : 1,
            onPageChanged: (idx) {
              setState(() {
                _activeBannerIndex = idx % banners.length;
              });
            },
            itemBuilder: (context, index) {
              final banner = banners[index % banners.length];
              final isIuran = banner.sourceType == 'iuran_reminder';
              final bannerLabel = switch (banner.sourceType) {
                'activity' => 'Kegiatan',
                'announcement' => 'Pengumuman',
                'iuran_reminder' => 'Pengingat Iuran',
                _ => 'Pengumuman RT',
              };

              return InkWell(
                onTap: () {
                  final title = banner.title.trim();
                  if (title == 'Pengingat Iuran Warga' || isIuran) {
                    context.push('/home/iuran');
                    return;
                  }
                  if (banner.sourceType == 'announcement' &&
                      banner.sourceId != null &&
                      banner.sourceId!.isNotEmpty) {
                    context.push('/home/pengumuman/${banner.sourceId}');
                    return;
                  }
                  if (banner.sourceType == 'activity' &&
                      banner.sourceId != null &&
                      banner.sourceId!.isNotEmpty) {
                    context.push('/home/kegiatan/${banner.sourceId}');
                    return;
                  }
                  final action = banner.actionUrl;
                  if (action != null && action.isNotEmpty) {
                    final target = action.startsWith('/home')
                        ? action
                        : '/home$action';
                    context.push(target);
                  }
                },
                borderRadius: BorderRadius.circular(AppRadius.base),
                child: Container(
                  margin: const EdgeInsets.symmetric(horizontal: 4),
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      colors: isIuran
                          ? [
                              isDark
                                  ? const Color(0xFF6A1B9A)
                                  : const Color(0xFF8E24AA),
                              isDark
                                  ? const Color(0xFF4A148C)
                                  : const Color(0xFF6A1B9A),
                            ]
                          : [
                              isDark
                                  ? const Color(0xFF1E3A8A)
                                  : const Color(0xFF2563EB),
                              isDark
                                  ? const Color(0xFF1E1B4B)
                                  : const Color(0xFF1D4ED8),
                            ],
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: 0.12),
                        blurRadius: 8,
                        offset: const Offset(0, 4),
                      ),
                    ],
                  ),
                  padding: const EdgeInsets.all(16),
                  child: Stack(
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    Container(
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 8,
                                        vertical: 4,
                                      ),
                                      decoration: BoxDecoration(
                                        color: Colors.white.withValues(
                                          alpha: 0.2,
                                        ),
                                        borderRadius: BorderRadius.circular(
                                          AppRadius.sm,
                                        ),
                                      ),
                                      child: Text(
                                        bannerLabel,
                                        style: const TextStyle(
                                          color: Colors.white,
                                          fontSize: 10,
                                          fontWeight: FontWeight.bold,
                                        ),
                                      ),
                                    ),
                                    if (banner.priority > 0) ...[
                                      const SizedBox(width: 6),
                                      Container(
                                        padding: const EdgeInsets.symmetric(
                                          horizontal: 6,
                                          vertical: 2,
                                        ),
                                        decoration: BoxDecoration(
                                          color: Colors.amber.shade700,
                                          borderRadius: BorderRadius.circular(
                                            AppRadius.xs,
                                          ),
                                        ),
                                        child: const Text(
                                          'Penting',
                                          style: TextStyle(
                                            color: Colors.white,
                                            fontSize: 9,
                                            fontWeight: FontWeight.bold,
                                          ),
                                        ),
                                      ),
                                    ],
                                  ],
                                ),
                                const Spacer(),
                                Text(
                                  banner.title,
                                  style: const TextStyle(
                                    color: Colors.white,
                                    fontSize: 16,
                                    fontWeight: FontWeight.bold,
                                  ),
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  banner.description,
                                  style: TextStyle(
                                    color: Colors.white.withValues(alpha: 0.85),
                                    fontSize: 12,
                                  ),
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ],
                            ),
                          ),
                          if (banner.imageUrl != null &&
                              banner.imageUrl!.isNotEmpty) ...[
                            const SizedBox(width: 12),
                            ClipRRect(
                              borderRadius: BorderRadius.circular(AppRadius.sm),
                              child: Container(
                                width: 92,
                                height: 92,
                                color: isDark
                                    ? AppColors.darkSurface
                                    : AppColors.lightSurfaceSubtle,
                                alignment: Alignment.center,
                                child: CommunityImage(
                                  imageRef: banner.imageUrl!,
                                  width: 92,
                                  height: 92,
                                  fit: BoxFit.contain,
                                  alt: banner.sourceType == 'activity'
                                      ? 'Banner kegiatan'
                                      : 'Banner pengumuman',
                                ),
                              ),
                            ),
                          ],
                        ],
                      ),
                      if (banner.actionUrl != null || isIuran)
                        const Positioned(
                          right: 0,
                          bottom: 0,
                          child: Icon(
                            Icons.arrow_forward_rounded,
                            color: Colors.white,
                            size: 20,
                          ),
                        ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        if (banners.length > 1) ...[
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: List.generate(
              banners.length,
              (i) => Container(
                margin: const EdgeInsets.symmetric(horizontal: 3),
                width: i == _activeBannerIndex ? 16 : 6,
                height: 6,
                decoration: BoxDecoration(
                  color: i == _activeBannerIndex
                      ? AppColors.accent
                      : (isDark ? Colors.grey.shade700 : Colors.grey.shade300),
                  borderRadius: BorderRadius.circular(3),
                ),
              ),
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildDefaultBanner(ThemeData theme, bool isDark) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            isDark ? const Color(0xFF1E293B) : const Color(0xFF3B82F6),
            isDark ? const Color(0xFF0F172A) : const Color(0xFF1D4ED8),
          ],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(AppRadius.base),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.2),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                ),
                child: const Text(
                  'Komunitas Warga',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 10,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          const Text(
            'Selamat Datang di Portal Warga',
            style: TextStyle(
              color: Colors.white,
              fontSize: 16,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            'Informasi kegiatan, pengumuman, dan kepengurusan RT hadir secara transparan.',
            style: TextStyle(
              color: Colors.white.withValues(alpha: 0.85),
              fontSize: 12,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBannersSkeleton(bool isDark) {
    return Container(
      height: 150,
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkSurface : AppColors.lightSurface,
        borderRadius: BorderRadius.circular(AppRadius.base),
      ),
      child: const Center(child: CircularProgressIndicator.adaptive()),
    );
  }

  // ── 3. Security Officer Status Card (Empty State Graceful) ──
  Widget _buildSecurityCard(
    SecurityOnDutyInfo? security,
    ThemeData theme,
    bool isDark,
  ) {
    final isAvailable = security?.isAvailable ?? false;
    final message =
        security?.message ?? 'Belum ada petugas aktif / jadwal shift saat ini';
    final subtext =
        security?.subtext ??
        'Data shift terintegrasi langsung saat Modul Keamanan berjalan';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionHeader(title: 'Petugas Keamanan Bertugas'),
        const SizedBox(height: _titleGap),
        AppCard(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: isAvailable
                        ? AppColors.accent.withValues(alpha: 0.12)
                        : (isDark
                              ? Colors.grey.shade800
                              : Colors.grey.shade200),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                  child: Icon(
                    Icons.shield_outlined,
                    color: isAvailable ? AppColors.accent : Colors.grey,
                    size: 28,
                  ),
                ),
                const SizedBox(width: 14),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 3,
                            ),
                            decoration: BoxDecoration(
                              color: isAvailable
                                  ? AppColors.income.withValues(alpha: 0.15)
                                  : (isDark
                                        ? Colors.grey.shade800
                                        : Colors.grey.shade200),
                              borderRadius: BorderRadius.circular(AppRadius.xs),
                            ),
                            child: Text(
                              isAvailable ? 'Shift Aktif' : 'Belum Ada Jadwal',
                              style: TextStyle(
                                fontSize: 10,
                                fontWeight: FontWeight.bold,
                                color: isAvailable
                                    ? AppColors.income
                                    : (isDark
                                          ? Colors.grey.shade400
                                          : Colors.grey.shade700),
                              ),
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 6),
                      Text(
                        message,
                        style: theme.textTheme.titleSmall?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        subtext,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: AppColors.textMutedColor(context),
                          fontSize: 11,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  // ── 4. Dynamic Organizational Hierarchy Diagram ──
  Widget _buildOrganizationHierarchy(
    OrganizationStructure organization,
    ThemeData theme,
    bool isDark,
  ) {
    if (organization.levels.isEmpty) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SectionHeader(title: 'Struktur Organisasi RT'),
          const SizedBox(height: _titleGap),
          AppCard(
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.base),
              child: Row(
                children: [
                  Icon(
                    Icons.group_outlined,
                    size: 32,
                    color: AppColors.textMutedColor(context),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text(
                          'Belum ada struktur organisasi',
                          style: TextStyle(fontWeight: FontWeight.w600),
                        ),
                        Text(
                          'Data perangkat RT belum tersedia dari server.',
                          style: TextStyle(
                            fontSize: 12,
                            color: AppColors.textMutedColor(context),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      );
    }

    final sortedLevels = organization.levels.keys.toList()..sort();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionHeader(title: 'Struktur Organisasi RT'),
        const SizedBox(height: _titleGap),
        AppCard(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                for (final lvl in sortedLevels) ...[
                  if (lvl > sortedLevels.first) ...[
                    Center(
                      child: Padding(
                        padding: const EdgeInsets.symmetric(vertical: 4),
                        child: Column(
                          children: [
                            Container(
                              width: 2,
                              height: 12,
                              color: AppColors.accent.withValues(alpha: 0.6),
                            ),
                            Container(
                              width: 6,
                              height: 6,
                              decoration: const BoxDecoration(
                                color: AppColors.accent,
                                shape: BoxShape.circle,
                              ),
                            ),
                            Container(
                              width: 2,
                              height: 12,
                              color: AppColors.accent.withValues(alpha: 0.6),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                  _buildLevelNodeGroup(
                    lvl,
                    organization.levels[lvl]!,
                    theme,
                    isDark,
                  ),
                ],
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildLevelNodeGroup(
    int level,
    List<OrgNode> nodes,
    ThemeData theme,
    bool isDark,
  ) {
    final levelLabels = {
      1: 'Pimpinan RT (Level 1)',
      2: 'Sekretariat & Perwakilan (Level 2)',
      3: 'Keuangan (Level 3)',
      4: 'Seksi & Operasional (Level 4)',
    };

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          levelLabels[level] ?? 'Tingkat $level',
          style: theme.textTheme.bodySmall?.copyWith(
            fontWeight: FontWeight.bold,
            color: AppColors.accent,
            fontSize: 11,
          ),
        ),
        const SizedBox(height: 6),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: nodes.expand((node) {
            if (node.members.isEmpty) {
              return [
                _buildOrgMemberChip(
                  name: 'Belum terisi',
                  jabatan: node.positionLabel,
                  phone: null,
                  theme: theme,
                  isDark: isDark,
                ),
              ];
            }
            return node.members.map(
              (m) => _buildOrgMemberChip(
                name: m.name,
                jabatan: node.positionLabel,
                phone: m.phone,
                theme: theme,
                isDark: isDark,
              ),
            );
          }).toList(),
        ),
      ],
    );
  }

  Color _getJabatanColor(String jabatan) {
    final normalized = jabatan
        .trim()
        .toLowerCase()
        .replaceAll(RegExp(r'\s+'), '_');
    switch (normalized) {
      case 'ketua':
      case 'ketua_rt':
        return AppColors.accent;
      case 'wakil_ketua':
      case 'wakil_ketua_rt':
        return AppColors.info;
      case 'sekretaris':
      case 'sekretaris_rt':
        return AppColors.success;
      case 'bendahara':
      case 'bendahara_rt':
        return AppColors.warning;
      case 'keamanan':
      case 'keamanan_rt':
      case 'seksi_keamanan':
      case 'seksi_keamanan_rt':
        return AppColors.danger;
      case 'operasional':
      case 'operasional_rt':
        return AppColors.info;
      case 'kebersihan':
      case 'kebersihan_rt':
        return AppColors.success;
      case 'sosial':
      case 'sosial_rt':
      case 'seksi_sosial':
        return AppColors.warning;
      default:
        return AppColors.accent;
    }
  }

  void _showOrgMemberDetail({
    required BuildContext context,
    required String name,
    required String jabatan,
    required String? phone,
    required Color color,
    required bool isDark,
  }) {
    final jNorm = jabatan.trim().toLowerCase();
    final isKetua = jNorm == 'ketua' || jNorm == 'ketua rt' || jNorm == 'ketua_rt';
    final safePhone = isKetua ? phone : null;

    showModalBottomSheet(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.lg)),
      ),
      builder: (ctx) {
        final theme = Theme.of(ctx);
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 36,
                    height: 4,
                    decoration: BoxDecoration(
                      color: Colors.grey.shade400,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                Row(
                  children: [
                    CircleAvatar(
                      radius: 24,
                      backgroundColor: color.withValues(alpha: 0.15),
                      child: Text(
                        name.isNotEmpty ? name[0].toUpperCase() : '?',
                        style: TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.bold,
                          color: color,
                        ),
                      ),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 3,
                            ),
                            decoration: BoxDecoration(
                              color: color.withValues(alpha: 0.15),
                              borderRadius: BorderRadius.circular(AppRadius.sm),
                            ),
                            child: Text(
                              jabatan.toUpperCase(),
                              style: TextStyle(
                                fontSize: 11,
                                fontWeight: FontWeight.bold,
                                color: color,
                              ),
                            ),
                          ),
                          const SizedBox(height: 6),
                          Text(
                            name,
                            style: theme.textTheme.titleMedium?.copyWith(
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                if (safePhone != null && safePhone.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  const Divider(),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      const Icon(
                        Icons.phone_outlined,
                        size: 20,
                        color: AppColors.accent,
                      ),
                      const SizedBox(width: 10),
                      Text(
                        safePhone,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                ],
                const SizedBox(height: 20),
                SizedBox(
                  width: double.infinity,
                  child: OutlinedButton(
                    onPressed: () => Navigator.pop(ctx),
                    child: const Text('Tutup'),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildOrgMemberChip({
    required String name,
    required String jabatan,
    required String? phone,
    required ThemeData theme,
    required bool isDark,
  }) {
    final color = _getJabatanColor(jabatan);
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(AppRadius.sm),
        onTap: () => _showOrgMemberDetail(
          context: context,
          name: name,
          jabatan: jabatan,
          phone: phone,
          color: color,
          isDark: isDark,
        ),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: color.withValues(alpha: isDark ? 0.18 : 0.12),
            borderRadius: BorderRadius.circular(AppRadius.sm),
            border: Border.all(
              color: color.withValues(alpha: isDark ? 0.45 : 0.35),
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 6,
                height: 6,
                decoration: BoxDecoration(
                  color: color,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 6),
              Text(
                jabatan.toUpperCase(),
                style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.bold,
                  color: color,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  // ── 5. Upcoming Activities ──
  Widget _buildUpcomingActivities(
    List<CommunityActivity> activities,
    ThemeData theme,
    bool isDark,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            const SectionHeader(title: 'Kegiatan Mendatang'),
            TextButton(
              onPressed: () => context.push('/home/kegiatan'),
              style: TextButton.styleFrom(
                visualDensity: VisualDensity.compact,
                padding: const EdgeInsets.symmetric(horizontal: 8),
              ),
              child: const Text('Lihat Semua'),
            ),
          ],
        ),
        const SizedBox(height: _titleGap),
        if (activities.isEmpty)
          AppCard(
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Center(
                child: Column(
                  children: [
                    Icon(
                      Icons.event_available_outlined,
                      size: 32,
                      color: AppColors.textMutedColor(context),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Belum ada kegiatan yang dijadwalkan.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: AppColors.textMutedColor(context),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          )
        else
          ...activities
              .take(4)
              .map(
                (act) => Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: AppCard(
                    child: InkWell(
                      onTap: () => context.push('/home/kegiatan'),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      child: Padding(
                        padding: const EdgeInsets.all(12),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.center,
                          children: [
                            Container(
                              width: 48,
                              height: 48,
                              decoration: BoxDecoration(
                                color: AppColors.accent.withValues(alpha: 0.12),
                                borderRadius: BorderRadius.circular(
                                  AppRadius.sm,
                                ),
                              ),
                              child: Column(
                                mainAxisAlignment: MainAxisAlignment.center,
                                children: [
                                  Text(
                                    act.startDate.day.toString(),
                                    style: const TextStyle(
                                      fontWeight: FontWeight.bold,
                                      fontSize: 16,
                                      color: AppColors.accent,
                                    ),
                                  ),
                                  Text(
                                    _monthShort(act.startDate.month),
                                    style: const TextStyle(
                                      fontSize: 9,
                                      fontWeight: FontWeight.bold,
                                      color: AppColors.accent,
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    act.title,
                                    style: theme.textTheme.titleSmall?.copyWith(
                                      fontWeight: FontWeight.w600,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                  const SizedBox(height: 3),
                                  Row(
                                    children: [
                                      Icon(
                                        Icons.location_on_outlined,
                                        size: 13,
                                        color: AppColors.textMutedColor(
                                          context,
                                        ),
                                      ),
                                      const SizedBox(width: 3),
                                      Expanded(
                                        child: Text(
                                          act.location,
                                          style: theme.textTheme.bodySmall
                                              ?.copyWith(
                                                fontSize: 11,
                                                color: AppColors.textMutedColor(
                                                  context,
                                                ),
                                              ),
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                        ),
                                      ),
                                    ],
                                  ),
                                ],
                              ),
                            ),
                            const Icon(
                              Icons.chevron_right,
                              size: 18,
                              color: Colors.grey,
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
      ],
    );
  }

  String _monthShort(int m) {
    const months = [
      '',
      'JAN',
      'FEB',
      'MAR',
      'APR',
      'MEI',
      'JUN',
      'JUL',
      'AGU',
      'SEP',
      'OKT',
      'NOV',
      'DES',
    ];
    return m >= 1 && m <= 12 ? months[m] : '';
  }
}

class DashboardMedia {
  DashboardMedia._();

  static double bottomPadding(BuildContext context) {
    final mediaQuery = MediaQuery.of(context);
    return mediaQuery.padding.bottom + AppSpacing.base;
  }
}
