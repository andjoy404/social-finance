import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/overdue_warning_card.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';

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
  static const _keuanganTab = _NavRoute(
    path: '/home/keuangan',
    label: 'Keuangan',
    icon: Icons.analytics_outlined,
    activeIcon: Icons.analytics,
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
  );
  static const _kegiatanTab = _NavRoute(
    path: '/home/kegiatan',
    label: 'Kegiatan',
    icon: Icons.event_outlined,
    activeIcon: Icons.event,
  );
  static const _pengumumanTab = _NavRoute(
    path: '/home/pengumuman',
    label: 'Pengumuman',
    icon: Icons.campaign_outlined,
    activeIcon: Icons.campaign,
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
    label: 'Sampah',
    icon: Icons.checklist_outlined,
    activeIcon: Icons.checklist,
    highlightWhenSelected: true,
  );
  static const _absenTab = _NavRoute(
    path: '/home/absen',
    label: 'Absen',
    icon: Icons.qr_code_scanner_outlined,
    activeIcon: Icons.qr_code_scanner,
  );
  static const _keamananAbsenTab = _NavRoute(
    path: '/home/absen',
    label: 'Absensi',
    icon: Icons.qr_code_scanner_outlined,
    activeIcon: Icons.qr_code_scanner,
  );
  static const _securityManagementTab = _NavRoute(
    path: '/home/security-management',
    label: 'Keamanan',
    icon: Icons.security_outlined,
    activeIcon: Icons.security,
  );
  static const _securityGuestBookTab = _NavRoute(
    path: '/home/security-guestbook',
    label: 'Buku Tamu',
    icon: Icons.menu_book_outlined,
    activeIcon: Icons.menu_book,
  );
  static const _patroliTab = _NavRoute(
    path: '/home/patroli',
    label: 'Patroli',
    icon: Icons.local_police_outlined,
    activeIcon: Icons.local_police,
  );
  static const _paketTab = _NavRoute(
    path: '/home/paket',
    label: 'Paket',
    icon: Icons.inventory_2_outlined,
    activeIcon: Icons.inventory_2,
  );
  static const _keamananSampahTab = _NavRoute(
    path: '/home/notifikasi',
    label: 'Sampah',
    icon: Icons.checklist_outlined,
    activeIcon: Icons.checklist,
  );
  static const _menuTab = _NavRoute(
    path: '#menu',
    label: 'Menu',
    icon: Icons.grid_view_outlined,
    activeIcon: Icons.grid_view,
  );

  bool _canAccessPatrol(Map<String, dynamic>? user) {
    if (user == null) return false;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'sekretaris') return false;
    if (jabatan == 'keamanan') return true;
    if (isSuperAdminUser(user) || isKetuaRTUser(user)) return true;
    return userHasPermission(user, 'patrol.read') ||
        userHasPermission(user, 'patrol.write');
  }

  bool _canAccessPaket(Map<String, dynamic>? user) {
    if (user == null) return false;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'sekretaris') return false;
    if (jabatan == 'keamanan') return true;
    if (isSuperAdminUser(user) || isKetuaRTUser(user)) return true;
    return userHasPermission(user, 'paket.read') ||
        userHasPermission(user, 'paket.write');
  }

  bool _canAccessSecurityManage(Map<String, dynamic>? user) {
    if (user == null) return false;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'sekretaris' || jabatan == 'keamanan') return false;
    if (isSuperAdminUser(user) || isKetuaRTUser(user)) return true;
    return userHasPermission(user, 'security.schedule.read') ||
        userHasPermission(user, 'security.schedule.write') ||
        userHasPermission(user, 'security.device.manage') ||
        userHasPermission(user, 'security.correction');
  }

  bool _canAccessKeuangan(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (!isAllowedRT(user)) return false;
    final systemRole = (user['system_role'] ?? '').toString().toLowerCase();
    if (systemRole == 'super_admin') return true;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'keamanan' ||
        jabatan == 'kebersihan' ||
        jabatan == 'sosial' ||
        jabatan == 'operasional') {
      return false;
    }
    final role = (user['role'] ?? '')
        .toString()
        .split('.')
        .last
        .trim()
        .toLowerCase();
    final isPengurus = user['role'] == AppRole.pengurus || role == 'pengurus';
    final isAnggotaRT =
        jabatan == 'anggota_rt' || (jabatan == 'anggota' && isPengurus);
    final isAnggotaRW = jabatan == 'anggota_rw';
    if (isAnggotaRT || isAnggotaRW) return false;
    final perms = user['effective_permissions'];
    if (perms is List) {
      return perms.contains('finance.read');
    }
    return role == 'warga' ||
        role == 'pengurus' ||
        role == 'bendahara' ||
        role == 'perangkat';
  }

  bool _canAccessLokasiDana(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (isSuperAdminUser(user)) return true;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'sekretaris' ||
        jabatan == 'sosial' ||
        jabatan == 'operasional' ||
        jabatan == 'anggota' ||
        jabatan == 'anggota_rt' ||
        jabatan == 'anggota_rw') {
      return false;
    }
    final roleVal = user['role'];
    final role = roleVal is AppRole
        ? roleVal
        : (roleVal?.toString() == 'pengurus'
              ? AppRole.pengurus
              : (roleVal?.toString() == 'bendahara'
                    ? AppRole.bendahara
                    : (roleVal?.toString() == 'warga' ? AppRole.warga : null)));
    return role == AppRole.bendahara ||
        role == AppRole.pengurus ||
        isKetuaRTUser(user) ||
        jabatan == 'bendahara' ||
        jabatan == 'ketua' ||
        jabatan == 'wakil_ketua';
  }

  bool _canAccessNotifikasi(Map<String, dynamic>? user) {
    return canUserAccessNotificationMenu(user);
  }

  bool _canAccessIuran(Map<String, dynamic>? user) {
    if (user == null) return false;
    if (!isAllowedRT(user)) return false;
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'operasional') return false;
    if (jabatan == 'keamanan' || jabatan == 'kebersihan') return false;
    final role = (user['role'] ?? '')
        .toString()
        .split('.')
        .last
        .trim()
        .toLowerCase();
    final isPengurus = user['role'] == AppRole.pengurus || role == 'pengurus';
    final isAnggotaRT =
        jabatan == 'anggota_rt' || (jabatan == 'anggota' && isPengurus);
    final isAnggotaRW = jabatan == 'anggota_rw';
    if (isAnggotaRT || isAnggotaRW) return false;
    // Sekretaris RT can access Iuran personal view (own bills/payments).
    // Admin Iuran operations are blocked by backend authorization.
    return true;
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
    // Include anggota, anggota_rt, and anggota_rw so Anggota RT and Anggota RW can access RW Dashboard and RW Kas READ.
    final filtered = rawList
        .where(
          (m) =>
              isSuperAdmin ||
              (m.jabatan != null &&
                  [
                    'ketua',
                    'wakil_ketua',
                    'sekretaris',
                    'bendahara',
                    'anggota',
                    'anggota_rt',
                    'anggota_rw',
                  ].contains(m.jabatan!.toLowerCase())),
        )
        .toList();
    if (filtered.isNotEmpty) return filtered;

    // Fallback if rw_memberships was empty or omitted: derive from user's rw_id / rt_rw
    final rwId = (user['rw_id'] ?? '').toString().trim();
    final jab = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    final isEligible =
        isSuperAdmin ||
        [
          'ketua',
          'wakil_ketua',
          'sekretaris',
          'bendahara',
          'anggota_rt',
          'anggota_rw',
        ].contains(jab) ||
        (jab == 'anggota' && role == 'pengurus');
    if (rwId.isNotEmpty && isEligible) {
      final rwName = user['rw_name']?.toString();
      final rwNum = user['rt_rw'] is num
          ? (user['rt_rw'] as num).toInt()
          : int.tryParse(user['rw']?.toString() ?? '');
      return [
        RWMembership(
          rwId: rwId,
          rwNumber: rwNum,
          rwName: rwName,
          jabatan: jab.isNotEmpty ? jab : 'anggota_rt',
          source: 'rt_inferred',
        ),
      ];
    }
    return [];
  }

  List<_NavRoute> _getBottomNavTabs(Map<String, dynamic>? user) {
    final jabatan = (user?['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'anggota_rw') {
      return [_rwDataTab, _notifikasiTab, _profileTab];
    }

    final canKeuangan = _canAccessKeuangan(user);
    final canIuran = _canAccessIuran(user);

    return [
      _homeTab,
      if (canKeuangan) _keuanganTab,
      if (canIuran) _iuranTab,
      _menuTab,
    ];
  }

  List<_NavRoute> _getCollapsedTabs(Map<String, dynamic>? user) {
    if (user == null) return [];
    final jabatan = (user['jabatan'] ?? '').toString().trim().toLowerCase();
    if (jabatan == 'anggota_rw') {
      return [];
    }
    final role = (user['role'] ?? '')
        .toString()
        .split('.')
        .last
        .trim()
        .toLowerCase();
    final isPengurus = user['role'] == AppRole.pengurus || role == 'pengurus';
    final isAnggotaRT =
        jabatan == 'anggota_rt' || (jabatan == 'anggota' && isPengurus);

    // Petugas Keamanan: strictly 6 menus: Warga, Absensi, Patroli, Paket, Sampah, Profil.
    if (jabatan == 'keamanan') {
      return [
        _wargaTab,
        _keamananAbsenTab,
        _patroliTab,
        _paketTab,
        _keamananSampahTab,
        _profileTab,
      ];
    }

    if (jabatan == 'kebersihan') {
      return [_monitoringTab, _profileTab];
    }

    // Anggota RT: navigation — exactly 4 menus: Data RW, Warga, Notifikasi, and Profil.
    if (isAnggotaRT) {
      return [_rwDataTab, _wargaTab, _notifikasiTab, _profileTab];
    }

    final isSekretaris = jabatan == 'sekretaris';
    final canSecurityManage = _canAccessSecurityManage(user);
    final canGuestBook = userHasPermission(user, 'security.guest_book');
    final canPatrol = _canAccessPatrol(user);
    final canPaket = _canAccessPaket(user);
    final canNotifikasi = _canAccessNotifikasi(user);
    final rwMemberships = _getValidRWMemberships(user);
    final isSosialOrOperasional =
        jabatan == 'sosial' || jabatan == 'operasional';
    final canKas = !isSosialOrOperasional;

    if (isAllowedRT(user)) {
      final canLokasiDana = _canAccessLokasiDana(user);
      final canMonitoring =
          jabatan == 'kebersihan' ||
          jabatan == 'keamanan' ||
          jabatan == 'ketua';
      return [
        if (canKas) _kasTab,
        if (canLokasiDana) _lokasiTab,
        _wargaTab,
        if (rwMemberships.isNotEmpty) _rwDataTab,
        if (canMonitoring && jabatan != 'ketua') _monitoringTab,
        // Ketua RT: open editable Sampah when the user has notes.write,
        // otherwise fall back to read-only view.
        if (jabatan == 'ketua' && canMonitoring)
          userHasPermission(user, 'rw.monitoring.notes.write')
              ? _monitoringTab
              : const _NavRoute(
                  path: '/home/monitoring?mode=read_only',
                  label: 'Sampah',
                  icon: Icons.checklist_outlined,
                  activeIcon: Icons.checklist,
                  highlightWhenSelected: true,
                ),
        if (canSecurityManage) _securityManagementTab,
        if (canGuestBook) _securityGuestBookTab,
        if (canPatrol) _patroliTab,
        if (canPaket) _paketTab,
        // Guaranteed trailing 4 items in exact order:
        // 1. Pengumuman
        // 2. Kegiatan
        // 3. Notifikasi
        // 4. Profil
        _pengumumanTab,
        _kegiatanTab,
        if (canNotifikasi) _notifikasiTab,
        _profileTab,
      ];
    }

    return [
      if (rwMemberships.isNotEmpty) _rwDataTab,
      // Guaranteed trailing 4 items in exact order:
      // 1. Pengumuman
      // 2. Kegiatan
      // 3. Notifikasi
      // 4. Profil
      _pengumumanTab,
      _kegiatanTab,
      if (canNotifikasi) _notifikasiTab,
      _profileTab,
    ];
  }

  List<_NavRoute> _getAllAccessibleTabs(Map<String, dynamic>? user) {
    final bottomTabs = _getBottomNavTabs(user).where((t) => t != _menuTab);
    final collapsedTabs = _getCollapsedTabs(user);
    return [...bottomTabs, ...collapsedTabs];
  }

  List<_NavRoute> _getActiveTabs() {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    return _getAllAccessibleTabs(user);
  }

  List<_NavRoute> _getDrawerTabs() {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    return _getAllAccessibleTabs(user);
  }

  int _resolveCollapsedMatch(List<_NavRoute> collapsedTabs, String location) {
    var bestMatch = -1;
    for (var i = 0; i < collapsedTabs.length; i++) {
      final tabPath = collapsedTabs[i].path;
      final cleanTabPath = tabPath.split('?').first;
      if (cleanTabPath == '/home/rw-data' &&
          (location.startsWith('/rw/') || location == '/home/rw-data')) {
        return i;
      }
      if (location == cleanTabPath || location.startsWith('$cleanTabPath/')) {
        if (bestMatch == -1 ||
            cleanTabPath.length >
                collapsedTabs[bestMatch].path.split('?').first.length) {
          bestMatch = i;
        }
      }
    }
    return bestMatch;
  }

  int _resolveBottomNavMatch(
    List<_NavRoute> bottomTabs,
    List<_NavRoute> collapsedTabs,
    String location,
  ) {
    final collapsedMatch = _resolveCollapsedMatch(collapsedTabs, location);
    if (collapsedMatch != -1) {
      final menuIdx = bottomTabs.indexOf(_menuTab);
      if (menuIdx >= 0) return menuIdx;
    }
    // If Data RW is in bottomTabs and location is an RW route, match Data RW tab.
    final rwDataIdx = bottomTabs.indexWhere((t) => t.path == '/home/rw-data');
    if (rwDataIdx >= 0 &&
        (location.startsWith('/rw/') ||
            location == '/home/rw-data' ||
            location.startsWith('/home/rw-data/'))) {
      return rwDataIdx;
    }
    var bestMatch = -1;
    for (var i = 0; i < bottomTabs.length; i++) {
      final tab = bottomTabs[i];
      if (tab == _menuTab) continue;
      final cleanPath = tab.path.split('?').first;
      if (location == cleanPath || location.startsWith('$cleanPath/')) {
        if (bestMatch == -1 ||
            cleanPath.length >
                bottomTabs[bestMatch].path.split('?').first.length) {
          bestMatch = i;
        }
      }
    }
    if (bestMatch != -1) return bestMatch;
    if (location.startsWith('/rw/')) return -1;
    return 0;
  }

  int _resolveCurrentMatch(List<_NavRoute> tabs, String location) {
    var bestMatch = 0;
    var found = false;
    for (var i = 0; i < tabs.length; i++) {
      final tabPath = tabs[i].path;
      final cleanPath = tabPath.split('?').first;
      if (location == cleanPath || location.startsWith('$cleanPath/')) {
        if (!found ||
            cleanPath.length > tabs[bestMatch].path.split('?').first.length) {
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
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        ref.read(notificationUnreadCountProvider.notifier).refresh();
      }
    });
  }

  @override
  void didUpdateWidget(MainNavigationShell oldWidget) {
    super.didUpdateWidget(oldWidget);
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    final bottomTabs = _getBottomNavTabs(user);
    final collapsedTabs = _getCollapsedTabs(user);
    final location = _currentLocation(context);
    final bestMatch = _resolveBottomNavMatch(
      bottomTabs,
      collapsedTabs,
      location,
    );
    if (bestMatch != _currentIndex) {
      setState(() {
        _currentIndex = bestMatch;
        _currentMatch = bestMatch;
      });
    }
  }

  void _showCollapsedMenuGrid(
    BuildContext shellContext,
    List<_NavRoute> collapsedTabs,
    int activeCollapsedIndex,
    bool Function(String path) tabHasBadge,
    Key Function(String path) tabBadgeKey,
  ) {
    showModalBottomSheet<void>(
      context: shellContext,
      isScrollControlled: true,
      backgroundColor: Theme.of(shellContext).colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (sheetContext) {
        return _CollapsedMenuGridSheet(
          collapsedTabs: collapsedTabs,
          activeCollapsedIndex: activeCollapsedIndex,
          tabHasBadge: tabHasBadge,
          tabBadgeKey: tabBadgeKey,
          onSelectTab: (index) {
            Navigator.of(sheetContext).pop();
            _onTabTapped(collapsedTabs, index, shellContext);
          },
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final bottomNavTabs = _getBottomNavTabs(user);
    final collapsedTabs = _getCollapsedTabs(user);
    final allTabs = _getAllAccessibleTabs(user);
    final drawerTabs = _getDrawerTabs();
    final location = _currentLocation(context);

    final collapsedMatch = _resolveCollapsedMatch(collapsedTabs, location);
    final isCollapsedActive = collapsedMatch != -1;
    final bottomMatch = _resolveBottomNavMatch(
      bottomNavTabs,
      collapsedTabs,
      location,
    );
    final drawerMatch = _resolveCurrentMatch(drawerTabs, location);

    _currentMatch = bottomMatch;

    final jabatan = (user?['jabatan'] ?? '').toString().trim().toLowerCase();
    final unreadCount = ref.watch(notificationUnreadCountProvider);
    final hasUnreadNotification = unreadCount > 0;

    final hasSecurityAlert =
        ref.watch(securityAlertsProvider).valueOrNull ?? false;
    final alertsData =
        ref.watch(securityAlertsDataProvider).valueOrNull ??
        const SecurityAlertsData();
    final effectiveSecurityAlert = hasSecurityAlert || alertsData.hasAlert;

    final rwMemberships = _getValidRWMemberships(user);
    final rwId = rwMemberships.isNotEmpty
        ? rwMemberships.first.rwId
        : (user?['rw_id'] ?? '').toString().trim();
    final rwUnreadCount = rwId.isNotEmpty
        ? (ref.watch(rwUnreadCountProvider(rwId)).valueOrNull ?? 0)
        : 0;
    final isActualRwExec =
        rwMemberships.any(
          (m) =>
              m.source != 'rt_inferred' &&
              (m.jabatan == 'ketua' || m.jabatan == 'bendahara'),
        ) ||
        userHasPermission(user, 'rw.notification.write');
    final hasRwUnread = isActualRwExec && rwUnreadCount > 0;

    bool tabHasBadge(String path) {
      if (jabatan == 'keamanan') return false;
      final cleanPath = path.split('?').first;
      if (cleanPath == '/home/notifikasi') return hasUnreadNotification;
      if (cleanPath == '/home/rw-data') return hasRwUnread;
      if (cleanPath == '/home/security-management' ||
          cleanPath == '/home/absen') {
        return effectiveSecurityAlert;
      }
      if (cleanPath == '/home/patroli') {
        return alertsData.patrolUnread > 0;
      }
      if (cleanPath == '/home/paket') {
        return alertsData.paketUnread > 0;
      }
      return false;
    }

    Key tabBadgeKey(String path) {
      final cleanPath = path.split('?').first;
      if (cleanPath == '/home/rw-data') return const Key('rw-unread-dot');
      if (cleanPath == '/home/patroli') return const Key('patrol-unread-dot');
      if (cleanPath == '/home/paket') return const Key('paket-unread-dot');
      if (cleanPath == '/home/security-management' ||
          cleanPath == '/home/absen') {
        return const Key('security-unread-dot');
      }
      return const Key('notification-unread-dot');
    }

    final hasMenuBadge = collapsedTabs.any((tab) => tabHasBadge(tab.path));

    final profileStatus = user?['profile_status'] as String?;
    final showOfflineSyncBanner =
        profileStatus == 'stale' || profileStatus == 'error';

    return DoubleBackExitScope(
      allowPop: !_isDirectlyOnTopLevelTab(location, allTabs),
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
          tabs: drawerTabs,
          currentMatch: drawerMatch,
          user: user,
          unreadCount: unreadCount,
          hasRwUnread: hasRwUnread,
          hasSecurityAlert: alertsData.hasAlert,
          onSelectTab: (index) => _onTabTapped(drawerTabs, index, context),
        ),
        bottomNavigationBar: BottomAppBar(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 8),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceAround,
            children: List.generate(bottomNavTabs.length, (index) {
              final tab = bottomNavTabs[index];
              final isMenu = tab == _menuTab;
              final isSelected = isMenu
                  ? isCollapsedActive
                  : (bottomMatch == index);
              final showBadge = isMenu ? hasMenuBadge : tabHasBadge(tab.path);
              final badgeKey = isMenu
                  ? const Key('menu-aggregate-unread-dot')
                  : tabBadgeKey(tab.path);
              return Expanded(
                child: _NavRailItem(
                  key: Key('nav-item-${tab.label.toLowerCase()}'),
                  tab: tab,
                  isSelected: isSelected,
                  showBadge: showBadge,
                  badgeKey: badgeKey,
                  onTap: () {
                    if (isMenu) {
                      _showCollapsedMenuGrid(
                        context,
                        collapsedTabs,
                        collapsedMatch,
                        tabHasBadge,
                        tabBadgeKey,
                      );
                    } else {
                      _onTabTapped(bottomNavTabs, index, context);
                    }
                  },
                ),
              );
            }),
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
        final currentRwId = rwMemberships.isNotEmpty
            ? rwMemberships.first.rwId
            : (user?['rw_id'] ?? '').toString().trim();
        if (currentRwId.isNotEmpty) {
          ref.invalidate(rwUnreadCountProvider(currentRwId));
        }

        if (rwMemberships.isEmpty) {
          final rwId = (user?['rw_id'] ?? '').toString().trim();
          if (rwId.isNotEmpty) {
            final destination = '/rw/$rwId/keuangan?tab=beranda';
            await _confirmAndGo(tapContext, currentLocation, destination);
          }
          return;
        }

        if (rwMemberships.length == 1) {
          final destination =
              '/rw/${rwMemberships.first.rwId}/keuangan?tab=beranda';
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
          final destination = '/rw/$selectedRwId/keuangan?tab=beranda';
          await _confirmAndGo(tapContext, currentLocation, destination);
        }
        return;
      }

      if (tab.path == '/home/notifikasi') {
        ref.read(notificationUnreadCountProvider.notifier).refresh();
      }
      if (tab.path == '/home/security-management' ||
          tab.path == '/home/absen') {
        ref.invalidate(securityAlertsProvider);
        ref.invalidate(securityAlertsDataProvider);
      }
      if (tab.path == '/home/patroli') {
        ref.read(securityApiClientProvider).markReadReceipt(domain: 'patrol');
        ref.invalidate(securityAlertsProvider);
        ref.invalidate(securityAlertsDataProvider);
      }
      if (tab.path == '/home/paket') {
        ref.read(securityApiClientProvider).markReadReceipt(domain: 'paket');
        ref.invalidate(securityAlertsProvider);
        ref.invalidate(securityAlertsDataProvider);
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
  final bool highlightWhenSelected;

  const _NavRoute({
    required this.path,
    required this.label,
    required this.icon,
    required this.activeIcon,
    this.highlightWhenSelected = false,
  });
}

class _NavRailItem extends StatelessWidget {
  final _NavRoute tab;
  final bool isSelected;
  final bool showBadge;
  final Key? badgeKey;
  final VoidCallback onTap;

  const _NavRailItem({
    super.key,
    required this.tab,
    required this.isSelected,
    this.showBadge = false,
    this.badgeKey,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    final bool highlightWhenSelected = tab.highlightWhenSelected;
    final bool highlightNow = isSelected && highlightWhenSelected;

    final Color iconColor = highlightNow
        ? AppColors.accent
        : isSelected
        ? AppColors.seed
        : theme.colorScheme.onSurfaceVariant;

    final Color labelColor = highlightNow
        ? AppColors.accent
        : isSelected
        ? AppColors.seed
        : theme.colorScheme.onSurfaceVariant;

    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        alignment: Alignment.center,
        padding: const EdgeInsets.symmetric(
          horizontal: 5,
          vertical: 5,
        ),
        decoration: BoxDecoration(
          color: highlightNow
              ? AppColors.accent.withValues(alpha: 0.20)
              : (isSelected ? AppColors.hoverTint : null),
          borderRadius: BorderRadius.circular(
            highlightNow ? 12 : 8,
          ),
          border: highlightNow
              ? Border.all(color: AppColors.accent, width: 1.5)
              : null,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Stack(
              clipBehavior: Clip.none,
              children: [
                Icon(
                  isSelected ? tab.activeIcon : tab.icon,
                  size: 22,
                  color: iconColor,
                ),
                if (showBadge)
                  Positioned(
                    top: -1,
                    right: -1,
                    child: Container(
                      key: badgeKey ?? const Key('notification-unread-dot'),
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
            ),
            const SizedBox(height: 2),
            Text(
              tab.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodySmall?.copyWith(
                color: labelColor,
                fontWeight: isSelected ? FontWeight.w700 : FontWeight.normal,
                fontSize: 11,
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
  final int unreadCount;
  final bool hasRwUnread;
  final bool hasSecurityAlert;
  final void Function(int index) onSelectTab;

  const _AppDrawer({
    required this.tabs,
    required this.currentMatch,
    required this.user,
    this.unreadCount = 0,
    this.hasRwUnread = false,
    this.hasSecurityAlert = false,
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
                  final isNotif = tab.path == '/home/notifikasi';
                  final showDrawerBadge =
                      jabatan != 'keamanan' &&
                      ((isNotif && unreadCount > 0) ||
                          (tab.path == '/home/rw-data' && hasRwUnread) ||
                          ((tab.path == '/home/security-management' ||
                                  tab.path == '/home/absen') &&
                              hasSecurityAlert));
                  final drawerBadgeKey = tab.path == '/home/rw-data'
                      ? const Key('drawer-rw-unread-dot')
                      : (tab.path == '/home/security-management' ||
                                tab.path == '/home/absen'
                            ? const Key('drawer-security-unread-dot')
                            : const Key('drawer-notification-unread-dot'));
                  return ListTile(
                    leading: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        Icon(
                          isSelected ? tab.activeIcon : tab.icon,
                          color: isSelected
                              ? AppColors.seed
                              : theme.colorScheme.onSurfaceVariant,
                        ),
                        if (showDrawerBadge)
                          Positioned(
                            top: -1,
                            right: -1,
                            child: Container(
                              key: drawerBadgeKey,
                              width: 8,
                              height: 8,
                              decoration: BoxDecoration(
                                color: AppColors.danger,
                                shape: BoxShape.circle,
                                border: Border.all(
                                  color: theme.colorScheme.surface,
                                  width: 1.5,
                                ),
                              ),
                            ),
                          ),
                      ],
                    ),
                    title: Text(
                      tab.label,
                      style: TextStyle(
                        fontWeight: isSelected
                            ? FontWeight.w700
                            : FontWeight.w500,
                        color: isSelected ? AppColors.seed : null,
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

class _CollapsedMenuGridSheet extends StatelessWidget {
  final List<_NavRoute> collapsedTabs;
  final int activeCollapsedIndex;
  final bool Function(String path) tabHasBadge;
  final Key Function(String path) tabBadgeKey;
  final void Function(int index) onSelectTab;

  const _CollapsedMenuGridSheet({
    required this.collapsedTabs,
    required this.activeCollapsedIndex,
    required this.tabHasBadge,
    required this.tabBadgeKey,
    required this.onSelectTab,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final maxHeight = MediaQuery.of(context).size.height * 0.70;

    return SafeArea(
      child: ConstrainedBox(
        constraints: BoxConstraints(maxHeight: maxHeight),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 8),
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: theme.colorScheme.onSurfaceVariant.withValues(
                  alpha: 0.3,
                ),
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              child: Row(
                children: [
                  Text(
                    'Menu',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const Spacer(),
                  IconButton(
                    icon: const Icon(Icons.close, size: 20),
                    visualDensity: VisualDensity.compact,
                    onPressed: () => Navigator.of(context).pop(),
                  ),
                ],
              ),
            ),
            const Divider(height: 1),
            Flexible(
              child: GridView.builder(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 12,
                ),
                shrinkWrap: true,
                physics: const BouncingScrollPhysics(),
                gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                  crossAxisCount: 4,
                  mainAxisSpacing: 10,
                  crossAxisSpacing: 8,
                  childAspectRatio: 0.82,
                ),
                itemCount: collapsedTabs.length,
                itemBuilder: (context, index) {
                  final tab = collapsedTabs[index];
                  final isSelected = index == activeCollapsedIndex;
                  return _CollapsedGridItem(
                    tab: tab,
                    isSelected: isSelected,
                    showBadge: tabHasBadge(tab.path),
                    badgeKey: tabBadgeKey(tab.path),
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

class _CollapsedGridItem extends StatelessWidget {
  final _NavRoute tab;
  final bool isSelected;
  final bool showBadge;
  final Key? badgeKey;
  final VoidCallback onTap;

  const _CollapsedGridItem({
    required this.tab,
    required this.isSelected,
    this.showBadge = false,
    this.badgeKey,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final iconColor = isSelected
        ? AppColors.seed
        : theme.colorScheme.onSurfaceVariant;
    final textColor = isSelected ? AppColors.seed : theme.colorScheme.onSurface;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        key: Key('grid-item-${tab.label.toLowerCase()}'),
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          key: Key('nav-item-${tab.label.toLowerCase()}'),
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
          decoration: BoxDecoration(
            color: isSelected ? AppColors.hoverTint : Colors.transparent,
            borderRadius: BorderRadius.circular(12),
            border: isSelected
                ? Border.all(
                    color: AppColors.seed.withValues(alpha: 0.5),
                    width: 1.5,
                  )
                : null,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Stack(
                clipBehavior: Clip.none,
                children: [
                  Container(
                    width: 44,
                    height: 44,
                    decoration: BoxDecoration(
                      color: isSelected
                          ? AppColors.seed.withValues(alpha: 0.12)
                          : theme.colorScheme.surfaceContainerHighest
                                .withValues(alpha: 0.5),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      isSelected ? tab.activeIcon : tab.icon,
                      size: 24,
                      color: iconColor,
                    ),
                  ),
                  if (showBadge)
                    Positioned(
                      top: 0,
                      right: 0,
                      child: Container(
                        key: badgeKey ?? const Key('notification-unread-dot'),
                        width: 9,
                        height: 9,
                        decoration: BoxDecoration(
                          color: AppColors.danger,
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: theme.colorScheme.surface,
                            width: 1.5,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                tab.label,
                maxLines: 2,
                textAlign: TextAlign.center,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: textColor,
                  fontWeight: isSelected ? FontWeight.w700 : FontWeight.w500,
                  fontSize: 11,
                  height: 1.15,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
