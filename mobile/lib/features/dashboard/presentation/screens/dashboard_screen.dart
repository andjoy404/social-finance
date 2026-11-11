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
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';

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
  int _orgChartPointers = 0;

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
          physics: _orgChartPointers > 0
              ? const NeverScrollableScrollPhysics()
              : const AlwaysScrollableScrollPhysics(),
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
    final wargaAsync = ref.watch(wargaListProvider);
    final wargaResidents = switch (wargaAsync) {
      AsyncData(:final value) => value,
      _ => const <MappedResident>[],
    };

    bool extraActiveFilter(OrgMember member) {
      if (!member.isActiveMember) return false;
      if (wargaResidents.isEmpty) return true;

      final uid = member.userId.trim();
      if (uid.isNotEmpty) {
        final byUser = wargaResidents.where(
          (r) => (r.userId ?? '').trim() == uid,
        );
        if (byUser.isNotEmpty) {
          return byUser.any((r) => r.isActive);
        }
      }

      final normName = member.name.trim().toLowerCase().replaceAll(
        RegExp(r'\s+'),
        ' ',
      );
      final normPos = normalizeOrgPosition(member.jabatan);
      final byNameAndPos = wargaResidents.where((r) {
        final rName = r.name.trim().toLowerCase().replaceAll(
          RegExp(r'\s+'),
          ' ',
        );
        if (rName != normName) return false;
        final rPos = normalizeOrgPosition(r.jabatan ?? '');
        return normPos.isEmpty || rPos.isEmpty || rPos == normPos;
      });
      if (byNameAndPos.isNotEmpty) {
        return byNameAndPos.any((r) => r.isActive);
      }

      return true;
    }

    final activeTiers = organization.buildActiveDiagramTiers(
      extraActiveFilter: extraActiveFilter,
    );

    if (activeTiers.isEmpty) {
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

    final viewportHeight =
        (activeTiers.length * 40.0 + (activeTiers.length - 1) * 48.0 + 72.0)
            .clamp(220.0, 380.0);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SectionHeader(title: 'Struktur Organisasi RT'),
        const SizedBox(height: _titleGap),
        AppCard(
          child: ClipRRect(
            borderRadius: BorderRadius.circular(AppRadius.base),
            child: Listener(
              behavior: HitTestBehavior.translucent,
              onPointerDown: (_) {
                setState(() => _orgChartPointers++);
              },
              onPointerUp: (_) {
                setState(
                  () => _orgChartPointers = (_orgChartPointers - 1).clamp(
                    0,
                    99,
                  ),
                );
              },
              onPointerCancel: (_) {
                setState(() => _orgChartPointers = 0);
              },
              child: SizedBox(
                height: viewportHeight,
                width: double.infinity,
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final viewportWidth = constraints.maxWidth.isFinite
                        ? constraints.maxWidth
                        : MediaQuery.sizeOf(context).width;
                    return _OrgChartViewport(
                      viewportWidth: viewportWidth,
                      tiersSignature: activeTiers
                          .map((t) => t.map((n) => n.position).join(','))
                          .join('|'),
                      child: _OrgChartWithConnectors(
                        tiers: activeTiers,
                        isDark: isDark,
                        buildChip: (node) => _buildOrgMemberChip(
                          node: node,
                          theme: theme,
                          isDark: isDark,
                        ),
                      ),
                    );
                  },
                ),
              ),
            ),
          ),
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
    required List<OrgMember> members,
    required String jabatan,
    required Color color,
    required bool isDark,
  }) {
    final activeMembers = members.where((m) => m.isActiveMember).toList();
    if (activeMembers.isEmpty) return;

    final jNorm = jabatan.trim().toLowerCase();
    final isKetua =
        jNorm == 'ketua' || jNorm == 'ketua rt' || jNorm == 'ketua_rt';

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
                for (var i = 0; i < activeMembers.length; i++) ...[
                  if (i > 0) ...[
                    const SizedBox(height: 12),
                    const Divider(),
                    const SizedBox(height: 12),
                  ],
                  Builder(
                    builder: (context) {
                      final member = activeMembers[i];
                      final name = member.name.trim();
                      final safePhone = isKetua ? member.phone : null;
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
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
                                    if (i == 0) ...[
                                      Container(
                                        padding: const EdgeInsets.symmetric(
                                          horizontal: 8,
                                          vertical: 3,
                                        ),
                                        decoration: BoxDecoration(
                                          color: color.withValues(alpha: 0.15),
                                          borderRadius: BorderRadius.circular(
                                            AppRadius.sm,
                                          ),
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
                                    ],
                                    Text(
                                      name,
                                      style: theme.textTheme.titleMedium
                                          ?.copyWith(
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
                        ],
                      );
                    },
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
    required OrgNode node,
    required ThemeData theme,
    required bool isDark,
  }) {
    final jabatan = node.positionLabel;
    final color = _getJabatanColor(jabatan);
    return Material(
      color: Colors.transparent,
      child: InkWell(
        key: ValueKey('org-badge-${node.position}'),
        borderRadius: BorderRadius.circular(AppRadius.sm),
        onTap: () => _showOrgMemberDetail(
          context: context,
          members: node.members,
          jabatan: jabatan,
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



class _OrgChartViewport extends StatefulWidget {
  final double viewportWidth;
  final String tiersSignature;
  final Widget child;

  const _OrgChartViewport({
    required this.viewportWidth,
    required this.tiersSignature,
    required this.child,
  });

  @override
  State<_OrgChartViewport> createState() => _OrgChartViewportState();
}

class _OrgChartViewportState extends State<_OrgChartViewport> {
  final TransformationController _controller = TransformationController();
  final GlobalKey _contentKey = GlobalKey();
  bool _userInteracted = false;
  bool _fitScheduled = false;

  @override
  void initState() {
    super.initState();
    _scheduleFit();
  }

  @override
  void didUpdateWidget(covariant _OrgChartViewport oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.tiersSignature != widget.tiersSignature ||
        (oldWidget.viewportWidth - widget.viewportWidth).abs() > 1.0) {
      _userInteracted = false;
      _scheduleFit();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _scheduleFit() {
    if (_fitScheduled) return;
    _fitScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _fitScheduled = false;
      _applyInitialCenterFit();
    });
  }

  void _applyInitialCenterFit() {
    if (!mounted || _userInteracted) return;
    final box = _contentKey.currentContext?.findRenderObject() as RenderBox?;
    if (box == null || !box.hasSize) return;
    final contentWidth = box.size.width;
    if (contentWidth <= 0 || widget.viewportWidth <= 0) return;

    if (contentWidth > widget.viewportWidth + 1.0) {
      final fitScale = (widget.viewportWidth / contentWidth).clamp(0.5, 1.0);
      final tx = (widget.viewportWidth - contentWidth * fitScale) / 2.0;
      _controller.value = Matrix4.identity()
        ..translate(tx, 0.0)
        ..scale(fitScale, fitScale);
    } else {
      _controller.value = Matrix4.identity();
    }
  }

  @override
  Widget build(BuildContext context) {
    return InteractiveViewer(
      key: const ValueKey('org-chart-interactive-viewer'),
      transformationController: _controller,
      constrained: false,
      boundaryMargin: const EdgeInsets.all(320),
      minScale: 0.15,
      maxScale: 3.0,
      panEnabled: true,
      scaleEnabled: true,
      onInteractionStart: (_) {
        _userInteracted = true;
      },
      child: ConstrainedBox(
        key: _contentKey,
        constraints: BoxConstraints(minWidth: widget.viewportWidth),
        child: Center(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 24),
            child: widget.child,
          ),
        ),
      ),
    );
  }
}

class _ChipMeta {
  final String position;
  final GlobalKey key;
  _ChipMeta({required this.position, required this.key});
}

/// Represents a single orthogonal connector polyline segment on the RT diagram.
class OrgConnectorLine {
  final List<Offset> points;
  final double stroke;
  const OrgConnectorLine({required this.points, this.stroke = 2.0});
}

/// Computes strictly orthogonal (90-degree right-angle) connector lines
/// between adjacent active tiers in [tierPositions] using badge [rects].
///
/// Rules:
/// - Vertical stem exits from bottom-center `(p.center.dx, p.bottom)` of each parent.
/// - Single aligned parent -> single child uses a direct vertical line.
/// - Multiple children (or multiple parents) share a single horizontal branching line
///   at `busY` (midpoint between parent bottom and child top), with vertical stems
///   from parents to `busY` and vertical drops from `busY` to top-center
///   `(c.center.dx, c.top)` of each child.
List<OrgConnectorLine> computeOrgConnectorLines(
  List<List<String>> tierPositions,
  Map<String, Rect> rects, {
  double stroke = 2.0,
}) {
  final activeTiers = <List<String>>[];
  for (final tier in tierPositions) {
    final present = <String>[];
    for (final pos in tier) {
      if (rects.containsKey(pos) && !present.contains(pos)) {
        present.add(pos);
      }
    }
    if (present.isNotEmpty) {
      activeTiers.add(present);
    }
  }

  final lines = <OrgConnectorLine>[];
  for (var i = 1; i < activeTiers.length; i++) {
    final parentPositions = activeTiers[i - 1];
    final childPositions = activeTiers[i];

    final parentRects = <Rect>[for (final p in parentPositions) rects[p]!];
    final childRects = <Rect>[for (final c in childPositions) rects[c]!];
    if (parentRects.isEmpty || childRects.isEmpty) continue;

    double maxParentBottom = parentRects.first.bottom;
    for (final pr in parentRects) {
      if (pr.bottom > maxParentBottom) maxParentBottom = pr.bottom;
    }

    double minChildTop = childRects.first.top;
    for (final cr in childRects) {
      if (cr.top < minChildTop) minChildTop = cr.top;
    }

    final busY = maxParentBottom + (minChildTop - maxParentBottom) / 2.0;

    if (parentRects.length == 1 && childRects.length == 1) {
      final pr = parentRects.first;
      final cr = childRects.first;
      final px = pr.center.dx;
      final cx = cr.center.dx;
      if ((px - cx).abs() <= 1.0) {
        final midX = (px + cx) / 2.0;
        lines.add(
          OrgConnectorLine(
            points: [Offset(midX, pr.bottom), Offset(midX, cr.top)],
            stroke: stroke,
          ),
        );
      } else {
        final minX = px < cx ? px : cx;
        final maxX = px > cx ? px : cx;
        lines.add(
          OrgConnectorLine(
            points: [Offset(px, pr.bottom), Offset(px, busY)],
            stroke: stroke,
          ),
        );
        lines.add(
          OrgConnectorLine(
            points: [Offset(minX, busY), Offset(maxX, busY)],
            stroke: stroke,
          ),
        );
        lines.add(
          OrgConnectorLine(
            points: [Offset(cx, busY), Offset(cx, cr.top)],
            stroke: stroke,
          ),
        );
      }
    } else {
      // 1. Vertical stem down from bottom-center of each parent badge to busY
      for (final pr in parentRects) {
        lines.add(
          OrgConnectorLine(
            points: [Offset(pr.center.dx, pr.bottom), Offset(pr.center.dx, busY)],
            stroke: stroke,
          ),
        );
      }

      // 2. Single horizontal branching line at busY spanning all parent stems & child drops
      final allXs = <double>[
        ...parentRects.map((r) => r.center.dx),
        ...childRects.map((r) => r.center.dx),
      ];
      final minX = allXs.reduce((a, b) => a < b ? a : b);
      final maxX = allXs.reduce((a, b) => a > b ? a : b);
      if ((maxX - minX).abs() > 0.5) {
        lines.add(
          OrgConnectorLine(
            points: [Offset(minX, busY), Offset(maxX, busY)],
            stroke: stroke,
          ),
        );
      }

      // 3. Vertical drop from busY to top-center of each child badge
      for (final cr in childRects) {
        lines.add(
          OrgConnectorLine(
            points: [Offset(cr.center.dx, busY), Offset(cr.center.dx, cr.top)],
            stroke: stroke,
          ),
        );
      }
    }
  }

  return lines;
}

class _OrgChartWithConnectors extends StatefulWidget {
  final List<List<OrgNode>> tiers;
  final bool isDark;
  final Widget Function(OrgNode node) buildChip;

  const _OrgChartWithConnectors({
    required this.tiers,
    required this.isDark,
    required this.buildChip,
  });

  @override
  State<_OrgChartWithConnectors> createState() =>
      _OrgChartWithConnectorsState();
}

class _OrgChartWithConnectorsState extends State<_OrgChartWithConnectors> {
  final GlobalKey _rootKey = GlobalKey();
  final List<_ChipMeta> _chips = [];
  final Map<String, GlobalKey> _chipKeys = {};
  List<OrgConnectorLine> _lines = [];
  bool _recomputeScheduled = false;

  GlobalKey _keyFor(String position) {
    return _chipKeys.putIfAbsent(position, () => GlobalKey());
  }

  @override
  void initState() {
    super.initState();
    _scheduleRecompute();
  }

  @override
  void didUpdateWidget(covariant _OrgChartWithConnectors oldWidget) {
    super.didUpdateWidget(oldWidget);
    _scheduleRecompute();
  }

  void _scheduleRecompute() {
    if (_recomputeScheduled) return;
    _recomputeScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _recomputeScheduled = false;
      _recompute();
    });
  }

  bool _sameLines(List<OrgConnectorLine> a, List<OrgConnectorLine> b) {
    if (identical(a, b)) return true;
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      final pa = a[i].points;
      final pb = b[i].points;
      if (pa.length != pb.length) return false;
      for (var j = 0; j < pa.length; j++) {
        if ((pa[j].dx - pb[j].dx).abs() > 0.01 ||
            (pa[j].dy - pb[j].dy).abs() > 0.01) {
          return false;
        }
      }
    }
    return true;
  }

  Map<String, Rect> _measureChipRects() {
    final rootBox = _rootKey.currentContext?.findRenderObject() as RenderBox?;
    if (rootBox == null || !rootBox.hasSize) return const <String, Rect>{};
    final rects = <String, Rect>{};
    for (final c in _chips) {
      final ctx = c.key.currentContext;
      if (ctx == null) continue;
      final box = ctx.findRenderObject() as RenderBox?;
      if (box == null || !box.hasSize) continue;
      final topLeft = box.localToGlobal(Offset.zero, ancestor: rootBox);
      rects[c.position] = topLeft & box.size;
    }
    return rects;
  }

  List<List<String>> _tierPositionMatrix() {
    return [
      for (final tier in widget.tiers)
        [for (final node in tier) node.position],
    ];
  }

  void _recompute() {
    if (!mounted) return;
    final rects = _measureChipRects();
    if (rects.isEmpty && _chips.isNotEmpty) {
      _scheduleRecompute();
      return;
    }
    final lines = computeOrgConnectorLines(_tierPositionMatrix(), rects);
    if (_sameLines(_lines, lines)) return;
    setState(() => _lines = lines);
  }

  @override
  Widget build(BuildContext context) {
    _chips.clear();
    _scheduleRecompute();

    final lineColor = AppColors.accent.withValues(
      alpha: widget.isDark ? 0.65 : 0.5,
    );

    Widget buildTierRow(List<OrgNode> nodesAtTier) {
      final children = <Widget>[];
      for (final node in nodesAtTier) {
        if (!node.hasActiveMembers) continue;
        final key = _keyFor(node.position);
        _chips.add(_ChipMeta(position: node.position, key: key));
        children.add(KeyedSubtree(key: key, child: widget.buildChip(node)));
      }
      return Row(
        mainAxisSize: MainAxisSize.min,
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          for (var i = 0; i < children.length; i++) ...[
            if (i > 0) const SizedBox(width: 16),
            children[i],
          ],
        ],
      );
    }

    final tierPositions = _tierPositionMatrix();

    return Stack(
      key: _rootKey,
      alignment: Alignment.topCenter,
      children: [
        Positioned.fill(
          child: IgnorePointer(
            child: CustomPaint(
              key: const ValueKey('org-connectors-custom-paint'),
              painter: OrgConnectorsPainter(
                fallbackLines: _lines,
                color: lineColor,
                measureRects: _measureChipRects,
                tierPositions: tierPositions,
              ),
            ),
          ),
        ),
        Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            for (var i = 0; i < widget.tiers.length; i++) ...[
              if (i > 0) const SizedBox(height: 48),
              buildTierRow(widget.tiers[i]),
            ],
          ],
        ),
      ],
    );
  }
}

class OrgConnectorsPainter extends CustomPainter {
  final List<OrgConnectorLine> fallbackLines;
  final Color color;
  final Map<String, Rect> Function()? measureRects;
  final List<List<String>> tierPositions;

  OrgConnectorsPainter({
    required this.fallbackLines,
    required this.color,
    this.measureRects,
    this.tierPositions = const <List<String>>[],
  });

  Map<String, Rect> computeCurrentRects() {
    return measureRects?.call() ?? const <String, Rect>{};
  }

  List<OrgConnectorLine> computeCurrentLines() {
    final rects = computeCurrentRects();
    if (rects.isNotEmpty && tierPositions.isNotEmpty) {
      return computeOrgConnectorLines(tierPositions, rects);
    }
    return fallbackLines;
  }

  @override
  void paint(Canvas canvas, Size size) {
    final activeLines = computeCurrentLines();
    if (activeLines.isEmpty) return;

    final verticalPaint = Paint()
      ..color = color
      ..strokeWidth = 2.0
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.butt
      ..strokeJoin = StrokeJoin.miter
      ..isAntiAlias = true;

    final horizontalPaint = Paint()
      ..color = color
      ..strokeWidth = 2.0
      ..style = PaintingStyle.stroke
      ..strokeCap = StrokeCap.square
      ..strokeJoin = StrokeJoin.miter
      ..isAntiAlias = true;

    for (final line in activeLines) {
      if (line.points.length < 2) continue;
      for (var i = 0; i < line.points.length - 1; i++) {
        final p0 = line.points[i];
        final p1 = line.points[i + 1];
        final isHorizontal = (p0.dy - p1.dy).abs() < 0.05;
        canvas.drawLine(
          p0,
          p1,
          isHorizontal ? horizontalPaint : verticalPaint,
        );
      }
    }
  }

  @override
  bool shouldRepaint(covariant OrgConnectorsPainter old) =>
      old.fallbackLines != fallbackLines ||
      old.color != color ||
      old.tierPositions != tierPositions;
}

class DashboardMedia {
  DashboardMedia._();

  static double bottomPadding(BuildContext context) {
    final mediaQuery = MediaQuery.of(context);
    return mediaQuery.padding.bottom + AppSpacing.base;
  }
}
