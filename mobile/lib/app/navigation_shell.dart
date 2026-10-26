import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

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
  static const _profileTab = _NavRoute(
    path: '/home/profile',
    label: 'Profil',
    icon: Icons.person_outline,
    activeIcon: Icons.person,
  );

  bool _canAccessLokasiDana(Map<String, dynamic>? user) {
    if (user == null) return false;
    final systemRole = user['system_role'] as String?;
    if (systemRole == 'super_admin') return true;
    final role = user['role'] as AppRole?;
    final jabatan = (user['jabatan'] ?? '').toString().toLowerCase();
    return role == AppRole.bendahara ||
        role == AppRole.pengurus ||
        jabatan == 'ketua' ||
        jabatan == 'bendahara';
  }

  List<_NavRoute> _getActiveTabs() {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    if (isAllowedRT(user)) {
      if (_canAccessLokasiDana(user)) {
        return const [
          _homeTab,
          _kasTab,
          _lokasiTab,
          _iuranTab,
          _wargaTab,
          _profileTab,
        ];
      }
      return const [_homeTab, _kasTab, _iuranTab, _wargaTab, _profileTab];
    }
    return const [_homeTab, _profileTab];
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
    return bestMatch;
  }

  String _currentLocation(BuildContext context) {
    return GoRouter.of(context).routeInformationProvider.value.uri.path;
  }

  /// Used by DoubleBackExitScope to determine whether system back should
  /// exit the app or navigate to the parent route.
  bool _isDirectlyOnTopLevelTab(String currentLocation, List<_NavRoute> tabs) {
    return tabs.any((t) => t.path == currentLocation);
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

    return DoubleBackExitScope(
      allowPop: !_isDirectlyOnTopLevelTab(location, tabs),
      child: Scaffold(
        key: _scaffoldKey,
        body: SafeArea(
          bottom: false,
          child: widget.child,
        ),
        bottomNavigationBar: BottomAppBar(
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceAround,
            children: List.generate(
              tabs.length,
              (index) => _NavRailItem(
                key: Key('nav-item-${tabs[index].label.toLowerCase()}'),
                tab: tabs[index],
                isSelected: index == _currentMatch,
                onTap: () => _onTabTapped(tabs, index, context),
              ),
            ),
          ),
        ),
      ),
    );
  }

  void _onTabTapped(List<_NavRoute> tabs, int index, BuildContext tapContext) {
    if (index >= 0 && index < tabs.length) {
      final tab = tabs[index];
      final currentLocation = _currentLocation(tapContext);
      if (currentLocation != tab.path) {
        context.go(tab.path);
      }
      setState(() => _currentMatch = index);
    }
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
