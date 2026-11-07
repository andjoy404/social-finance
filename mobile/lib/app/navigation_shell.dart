import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/overdue_warning_card.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';

bool isDirectlyOnTopLevelTab(
  String currentLocation,
  Iterable<String> tabPaths,
) {
  if (currentLocation == '/home/notifikasi/waste-truck' &&
      tabPaths.contains('/home/notifikasi')) {
    return true;
  }
  if (currentLocation.startsWith('/rw/')) return true;
  return tabPaths.contains(currentLocation);
}

class MainNavigationShell extends ConsumerStatefulWidget {
  final Widget child;

  const MainNavigationShell({super.key, required this.child});

  @override
  ConsumerState<MainNavigationShell> createState() =>
      _MainNavigationShellState();
}

class _MainNavigationShellState extends ConsumerState<MainNavigationShell> {
  int _currentIndex = 0;
  int _currentMatch = 0;

  static const _homeTab = _NavRoute(
    path: '/home',
    label: 'Beranda',
    icon: Icons.home_outlined,
    activeIcon: Icons.home,
  );
  static const _kasTab = _NavRoute(
    path: '/home/kas',
    label: 'Kas',
    icon: Icons.account_balance_wallet_outlined,
    activeIcon: Icons.account_balance_wallet,
  );
  static const _lokasiTab = _NavRoute(
    path: '/home/lokasi-dana',
    label: 'Lokasi Dana',
    icon: Icons.account_balance_outlined,
    activeIcon: Icons.account_balance,
  );
  static const _iuranTab = _NavRoute(
    path: '/home/iuran',
    label: 'Iuran',
    icon: Icons.receipt_long_outlined,
    activeIcon: Icons.receipt_long,
    isProminent: true,
  );
  static const _wargaTab = _NavRoute(
    path: '/home/warga',
    label: 'Warga',
    icon: Icons.people_outline,
    activeIcon: Icons.people,
  );
  static const _rwDataTab = _NavRoute(
    path: '/home/rw-data',
    label: 'Data RW',
    icon: Icons.area_chart_outlined,
    activeIcon: Icons.area_chart,
  );
  static const _profileTab = _NavRoute(
    path: '/home/profile',
    label: 'Profil',
    icon: Icons.person_outline,
    activeIcon: Icons.person,
  );
  static const _notifikasiTab = _NavRoute(
    path: '/home/notifikasi',
    label: 'Notifikasi',
    icon: Icons.notifications_outlined,
    activeIcon: Icons.notifications,
  );
  static const _monitoringTab = _NavRoute(
    path: '/home/monitoring',
    label: 'Monitoring',
    icon: Icons.checklist_outlined,
    activeIcon: Icons.checklist,
  );

  bool _canAccessLokasiDana(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (isSuperAdminUser(user)) return true;
    final role = user['role'] as AppRole?;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    return role == AppRole.bendahara ||
        role == AppRole.pengurus ||
        isKetuaRTUser(user) ||
        jabatan == 'bendahara' ||
        jabatan == 'ketua' ||
        jabatan == 'wakil_ketua' ||
        jabatan == 'sekretaris';
  }

  bool _canAccessNotifikasi(Map<String, dynamic>? user) {
    return canUserAccessNotificationMenu(user);
  }

  List<RWMembership> _getValidRWMemberships(Map<String, dynamic>? user) {
    if (user == null) return [];
    final systemRole = (user['system_role'] ?? '').toString().toLowerCase();
    final role = (user['role'] ?? '').toString().toLowerCase();
    final isSuperAdmin =
        systemRole == 'super_admin' ||
        role == 'super_admin' ||
        role.contains('superadmin');
    final rawList =
        (user['rw_memberships'] as List?)
            ?.whereType<Map<String, dynamic>>()
            .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
            .toList() ??
        <RWMembership>[];
    return rawList
        .where(
          (m) =>
              isSuperAdmin ||
              (m.jabatan != null &&
                  [
                    'ketua',
                    'wakil_ketua',
                    'sekretaris',
                    'bendahara',
                  ].contains(m.jabatan!.toLowerCase())),
        )
        .toList();
  }

  List<_NavRoute> _getActiveTabs() {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final jabatan = (user?['jabatan'] ?? '').toString().trim().toLowerCase();

    // Security officers retain their read-only Warga and dispatch menus,
    // with Profile always available for account and logout actions.
    if (jabatan == 'keamanan') {
      return [_wargaTab, _notifikasiTab, _profileTab];
    }

    // Kebersihan keeps Monitoring and also has Profile/logout access.
    if (jabatan == 'kebersihan') {
      return [_monitoringTab, _profileTab];
    }

    final canNotifikasi = _canAccessNotifikasi(user);
    final rwMemberships = _getValidRWMemberships(user);
    if (isAllowedRT(user)) {
      final canLokasiDana = _canAccessLokasiDana(user);
      return [
        _homeTab,
        _kasTab,
        if (canLokasiDana) _lokasiTab,
        _iuranTab,
        _wargaTab,
        if (rwMemberships.isNotEmpty) _rwDataTab,
        if (canNotifikasi) _notifikasiTab,
        _profileTab,
      ];
    }
    return [
      _homeTab,
      if (rwMemberships.isNotEmpty) _rwDataTab,
      if (canNotifikasi) _notifikasiTab,
      _profileTab,
    ];
  }

  int _resolveCurrentMatch(List<_NavRoute> tabs, String location) {
    var bestMatch = 0;
    var found = false;
    for (var i = 0; i < tabs.length; i++) {
      final tabPath = tabs[i].path;
      if (location == tabPath || location.startsWith('$tabPath/')) {
        if (!found || tabPath.length > tabs[bestMatch].path.length) {
          bestMatch = i;
          found = true;
        }
      }
    }
    // Also check RW routes — if location starts with /rw/, match the RW data tab.
    if (!found || tabs[bestMatch].path != '/home/rw-data') {
      final rwDataIndex = tabs.indexWhere((t) => t.path == '/home/rw-data');
      if (rwDataIndex >= 0 && location.startsWith('/rw/')) {
        // Prefer RW data tab when on an RW route, but only if not already
        // on a more specific tab (e.g. /home, /home/kas, etc.).
        final rwDataPreferred = tabs[rwDataIndex].path;
        final isRWScope =
            location.startsWith('/rw/') ||
            (location == rwDataPreferred ||
                location.startsWith('/home/rw-data/'));
        // Only pick RW data if we're not already on a /home/* tab.
        if (isRWScope && !location.startsWith('/home')) {
          return rwDataIndex;
        }
      }
    }
    return bestMatch;
  }

  String _currentLocation(BuildContext context) {
    return GoRouter.of(context).routeInformationProvider.value.uri.path;
  }

  /// Used by DoubleBackExitScope to determine whether system back should
  /// exit the app or navigate to the parent route.
  bool _isDirectlyOnTopLevelTab(String currentLocation, List<_NavRoute> tabs) {
    return isDirectlyOnTopLevelTab(
      currentLocation,
      tabs.map((tab) => tab.path),
    );
  }

  final GlobalKey<ScaffoldState> _scaffoldKey = GlobalKey<ScaffoldState>();

  @override
  void didUpdateWidget(MainNavigationShell oldWidget) {
    super.didUpdateWidget(oldWidget);
    final tabs = _getActiveTabs();
    final location = _currentLocation(context);
    final bestMatch = _resolveCurrentMatch(tabs, location);
    if (bestMatch != _currentIndex) {
      setState(() {
        _currentIndex = bestMatch;
        _currentMatch = bestMatch;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final tabs = _getActiveTabs();
    final location = _currentLocation(context);
    final currentMatch = _resolveCurrentMatch(tabs, location);

    _currentMatch = currentMatch;

    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final profileStatus = user?['profile_status'] as String?;
    final showOfflineSyncBanner =
        profileStatus == 'stale' || profileStatus == 'error';

    return DoubleBackExitScope(
      allowPop: !_isDirectlyOnTopLevelTab(location, tabs),
      child: Scaffold(
        key: _scaffoldKey,
        body: SafeArea(
          bottom: false,
          child: Column(
            children: [
              if (showOfflineSyncBanner)
                Material(
                  color: AppColors.warning.withValues(alpha: 0.14),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 6,
                    ),
                    child: Row(
                      children: [
                        const Icon(
                          Icons.cloud_off_outlined,
                          size: 16,
                          color: AppColors.warning,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            profileStatus == 'stale'
                                ? 'Menampilkan profil tersimpan (koneksi belum stabil).'
                                : 'Sinkronisasi profil tertunda. Periksa koneksi Anda.',
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(
                                  fontSize: 11.5,
                                  fontWeight: FontWeight.w500,
                                ),
                          ),
                        ),
                        TextButton(
                          style: TextButton.styleFrom(
                            visualDensity: VisualDensity.compact,
                            padding: const EdgeInsets.symmetric(horizontal: 8),
                            minimumSize: const Size(0, 28),
                          ),
                          onPressed: () {
                            ref
                                .read(authRepositoryProvider.notifier)
                                .reloadProfile();
                          },
                          child: const Text(
                            'Segarkan',
                            style: TextStyle(
                              fontSize: 11.5,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              const OverdueWarningCard(),
              Expanded(child: widget.child),
            ],
          ),
        ),
        drawer: _AppDrawer(
          tabs: tabs,
          currentMatch: currentMatch,
          user: user,
          onSelectTab: (index) => _onTabTapped(tabs, index, context),
        ),
        bottomNavigationBar: BottomAppBar(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
          child: tabs.length <= 4
              ? Row(
                  mainAxisAlignment: MainAxisAlignment.spaceAround,
                  children: List.generate(
                    tabs.length,
                    (index) => Expanded(
                      child: _NavRailItem(
                        key: Key('nav-item-${tabs[index].label.toLowerCase()}'),
                        tab: tabs[index],
                        isSelected: index == _currentMatch,
                        onTap: () => _onTabTapped(tabs, index, context),
                      ),
                    ),
                  ),
                )
              : LayoutBuilder(
                  builder: (context, constraints) => SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    physics: const BouncingScrollPhysics(),
                    child: ConstrainedBox(
                      constraints: BoxConstraints(
                        minWidth: constraints.maxWidth,
                      ),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.spaceAround,
                        children: List.generate(
                          tabs.length,
                          (index) => Padding(
                            padding: const EdgeInsets.symmetric(horizontal: 4),
                            child: _NavRailItem(
                              key: Key(
                                'nav-item-${tabs[index].label.toLowerCase()}',
                              ),
                              tab: tabs[index],
                              isSelected: index == _currentMatch,
                              onTap: () => _onTabTapped(tabs, index, context),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
        ),
      ),
    );
  }

  Future<void> _onTabTapped(
    List<_NavRoute> tabs,
    int index,
    BuildContext tapContext,
  ) async {
    if (index >= 0 && index < tabs.length) {
      final tab = tabs[index];
      final currentLocation = _currentLocation(tapContext);
      // Special handling for RW data tab
      if (tab.path == '/home/rw-data') {
        final authState = ref.read(authRepositoryProvider);
        final user = authState?.valueOrNull;
        final rwMemberships = _getValidRWMemberships(user);

        if (rwMemberships.isEmpty) return;

        if (rwMemberships.length == 1) {
          final destination = '/rw/${rwMemberships.first.rwId}/dashboard';
          await _confirmAndGo(tapContext, currentLocation, destination);
        } else {
          // Multiple memberships: select the exact target before asking for
          // confirmation, then navigate only after explicit approval.
          final selectedRwId = await showDialog<String>(
            context: tapContext,
            builder: (dialogContext) => AlertDialog(
              title: const Text('Pilih RW'),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: rwMemberships.map((m) {
                  return ListTile(
                    title: Text(
                      ((m.rwName ?? '').isNotEmpty) ? m.rwName! : m.displayName,
                    ),
                    subtitle: ((m.jabatan ?? '').isNotEmpty)
                        ? Text('Jabatan: ${m.jabatan}')
                        : null,
                    onTap: () {
                      Navigator.pop(dialogContext, m.rwId);
                    },
                  );
                }).toList(),
              ),
            ),
          );
          if (selectedRwId == null ||
              selectedRwId.isEmpty ||
              !tapContext.mounted) {
            return;
          }
          final destination = '/rw/$selectedRwId/dashboard';
          await _confirmAndGo(tapContext, currentLocation, destination);
        }
        return;
      }

      // If currently on a guarded form page, always ask for confirmation
      // before navigating away (bottom nav or drawer).
      if (KasFormNavigationGuard.isFormPath(currentLocation)) {
        await _confirmNavigationToTab(tabs, index, currentLocation, tapContext);
        return;
      }

      if (!tapContext.mounted || !mounted) return;
      if (_scaffoldKey.currentState?.isDrawerOpen == true) {
        Navigator.of(tapContext).pop();
      }
      _navigateToTab(tabs, index, currentLocation);
    }
  }

  Future<void> _confirmNavigationToTab(
    List<_NavRoute> tabs,
    int index,
    String currentLocation,
    BuildContext tapContext,
  ) async {
    if (await _confirmAndGo(tapContext, currentLocation, tabs[index].path)) {
      if (mounted) setState(() => _currentMatch = index);
    }
  }

  Future<bool> _confirmAndGo(
    BuildContext tapContext,
    String source,
    String destination,
  ) async {
    final confirmed = await KasFormNavigationGuard.confirmShellNavigation(
      tapContext,
      source: source,
      destination: destination,
    );
    if (!confirmed || !mounted || !tapContext.mounted) return false;
    if (_scaffoldKey.currentState?.isDrawerOpen == true) {
      Navigator.of(tapContext).pop();
    }
    if (KasFormNavigationGuard.isFormPath(source)) {
      KasFormNavigationGuard.allowExitFor(source);
    }
    try {
      context.go(destination);
    } catch (_) {
      KasFormNavigationGuard.revokeExitFor(source);
      rethrow;
    }
    return true;
  }

  void _navigateToTab(List<_NavRoute> tabs, int index, String currentLocation) {
    final destination = tabs[index].path;
    if (currentLocation != destination) {
      context.go(destination);
    }
    if (mounted) setState(() => _currentMatch = index);
  }
}

class _NavRoute {
  final String path;
  final String label;
  final IconData icon;
  final IconData activeIcon;
  final bool isProminent;

  const _NavRoute({
    required this.path,
    required this.label,
    required this.icon,
    required this.activeIcon,
    this.isProminent = false,
  });
}

class _NavRailItem extends StatelessWidget {
  final _NavRoute tab;
  final bool isSelected;
  final VoidCallback onTap;

  const _NavRailItem({
    super.key,
    required this.tab,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isProminent = tab.isProminent;

    final Color iconColor = isSelected
        ? AppColors.seed
        : (isProminent ? AppColors.accent : theme.colorScheme.onSurfaceVariant);

    final Color labelColor = isSelected
        ? AppColors.seed
        : (isProminent ? AppColors.accent : theme.colorScheme.onSurfaceVariant);

    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        alignment: Alignment.center,
        padding: EdgeInsets.symmetric(
          horizontal: isProminent ? 10 : 5,
          vertical: isProminent ? 4 : 5,
        ),
        decoration: BoxDecoration(
          color: isProminent
              ? (isSelected
                    ? AppColors.accent.withValues(alpha: 0.20)
                    : AppColors.accent.withValues(alpha: 0.08))
              : (isSelected ? AppColors.hoverTint : null),
          borderRadius: BorderRadius.circular(isProminent ? 12 : 8),
          border: isProminent
              ? Border.all(
                  color: isSelected
                      ? AppColors.accent
                      : AppColors.accent.withValues(alpha: 0.35),
                  width: isSelected ? 1.5 : 1.0,
                )
              : null,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isSelected ? tab.activeIcon : tab.icon,
              size: isProminent ? 24 : 22,
              color: iconColor,
            ),
            const SizedBox(height: 2),
            Text(
              tab.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodySmall?.copyWith(
                color: labelColor,
                fontWeight: isSelected
                    ? FontWeight.w700
                    : (isProminent ? FontWeight.w600 : FontWeight.normal),
                fontSize: isProminent ? 11.5 : 11,
                height: 1.1,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AppDrawer extends StatelessWidget {
  final List<_NavRoute> tabs;
  final int currentMatch;
  final Map<String, dynamic>? user;
  final void Function(int index) onSelectTab;

  const _AppDrawer({
    required this.tabs,
    required this.currentMatch,
    required this.user,
    required this.onSelectTab,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final screenWidth = MediaQuery.of(context).size.width;
    final drawerWidth = (screenWidth * 0.75).clamp(240.0, 320.0);

    final name = (user?['name'] as String?)?.isNotEmpty == true
        ? user!['name'] as String
        : (user?['email'] as String? ?? 'Pengguna');
    final role = user?['role']?.toString().split('.').last ?? '';
    final jabatan = user?['jabatan'] as String? ?? '';
    final rt = user?['rt']?.toString() ?? '';
    final rawRw = user?['rw']?.toString() ?? '';
    final rw = looksLikeUuid(rawRw)
        ? (resolveRwNumber(rawRw, user?['rw_memberships']) ?? '')
        : rawRw;
    final rtRwText = [
      if (rt.isNotEmpty) 'RT $rt',
      if (rw.isNotEmpty) 'RW $rw',
    ].join(' / ');

    return SizedBox(
      width: drawerWidth,
      child: Drawer(
        child: Column(
          children: [
            DrawerHeader(
              decoration: BoxDecoration(
                color: theme.colorScheme.primaryContainer.withValues(
                  alpha: 0.3,
                ),
              ),
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  CircleAvatar(
                    radius: 24,
                    backgroundColor: AppColors.accent,
                    child: Text(
                      name.isNotEmpty ? name[0].toUpperCase() : 'U',
                      style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 20,
                      ),
                    ),
                  ),
                  const SizedBox(height: 10),
                  Text(
                    name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Row(
                    children: [
                      if (jabatan.isNotEmpty || role.isNotEmpty)
                        Flexible(
                          child: Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 6,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: AppColors.accent.withValues(alpha: 0.12),
                              borderRadius: BorderRadius.circular(4),
                            ),
                            child: Text(
                              jabatan.isNotEmpty
                                  ? jabatan.toUpperCase()
                                  : role.toUpperCase(),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: theme.textTheme.bodySmall?.copyWith(
                                fontSize: 10,
                                fontWeight: FontWeight.w700,
                                color: AppColors.accent,
                              ),
                            ),
                          ),
                        ),
                      if ((jabatan.isNotEmpty || role.isNotEmpty) &&
                          rtRwText.isNotEmpty)
                        const SizedBox(width: 6),
                      if (rtRwText.isNotEmpty)
                        Expanded(
                          child: Text(
                            rtRwText,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                          ),
                        ),
                    ],
                  ),
                ],
              ),
            ),
            Expanded(
              child: ListView.builder(
                padding: const EdgeInsets.symmetric(vertical: 8),
                itemCount: tabs.length,
                itemBuilder: (context, index) {
                  final tab = tabs[index];
                  final isSelected = index == currentMatch;
                  return ListTile(
                    leading: Icon(
                      isSelected ? tab.activeIcon : tab.icon,
                      color: isSelected
                          ? AppColors.seed
                          : (tab.isProminent
                                ? AppColors.accent
                                : theme.colorScheme.onSurfaceVariant),
                    ),
                    title: Text(
                      tab.label,
                      style: TextStyle(
                        fontWeight: isSelected
                            ? FontWeight.w700
                            : FontWeight.w500,
                        color: isSelected
                            ? AppColors.seed
                            : (tab.isProminent ? AppColors.accent : null),
                      ),
                    ),
                    selected: isSelected,
                    selectedTileColor: AppColors.hoverTint,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(8),
                    ),
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 0,
                    ),
                    dense: true,
                    onTap: () => onSelectTab(index),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
