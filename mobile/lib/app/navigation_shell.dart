import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

class MainNavigationShell extends ConsumerStatefulWidget {
  final Widget child;

  const MainNavigationShell({super.key, required this.child});

  @override
  ConsumerState<MainNavigationShell> createState() => _MainNavigationShellState();
}

class _MainNavigationShellState extends ConsumerState<MainNavigationShell> {
  int _currentIndex = 0;

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
  static const _iuranTab = _NavRoute(
    path: '/home/iuran',
    label: 'Iuran',
    icon: Icons.receipt_long_outlined,
    activeIcon: Icons.receipt_long,
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

  List<_NavRoute> _getActiveTabs() {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    if (isAllowedRT(user)) {
      return const [_homeTab, _kasTab, _iuranTab, _wargaTab, _profileTab];
    }
    return const [_homeTab, _profileTab];
  }

  @override
  void didUpdateWidget(MainNavigationShell oldWidget) {
    super.didUpdateWidget(oldWidget);
    final tabs = _getActiveTabs();
    final location = GoRouterState.of(context).matchedLocation;
    var bestMatch = -1;
    for (var i = 0; i < tabs.length; i++) {
      final tabPath = tabs[i].path;
      if (location == tabPath || location.startsWith('$tabPath/')) {
        if (bestMatch == -1 ||
            tabPath.length > tabs[bestMatch].path.length) {
          bestMatch = i;
        }
      }
    }
    if (bestMatch != -1 && bestMatch != _currentIndex) {
      setState(() => _currentIndex = bestMatch);
    }
  }

  @override
  Widget build(BuildContext context) {
    final tabs = _getActiveTabs();
    final location = GoRouterState.of(context).matchedLocation;
    var currentMatch = 0;
    for (var i = 0; i < tabs.length; i++) {
      final tabPath = tabs[i].path;
      if (location == tabPath || location.startsWith('$tabPath/')) {
        if (currentMatch == 0 ||
            tabPath.length > tabs[currentMatch].path.length) {
          currentMatch = i;
        }
      }
    }

    return DoubleBackExitScope(
      child: Scaffold(
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
                tab: tabs[index],
                isSelected: index == currentMatch,
                onTap: () => _onTabTapped(tabs, index),
              ),
            ),
          ),
        ),
      ),
    );
  }

  void _onTabTapped(List<_NavRoute> tabs, int index) {
    if (index >= 0 && index < tabs.length) {
      final tab = tabs[index];
      if (GoRouterState.of(context).matchedLocation != tab.path) {
        context.go(tab.path);
      }
      setState(() => _currentIndex = index);
    }
  }
}

class _NavRoute {
  final String path;
  final String label;
  final IconData icon;
  final IconData activeIcon;

  const _NavRoute({
    required this.path,
    required this.label,
    required this.icon,
    required this.activeIcon,
  });
}

class _NavRailItem extends StatelessWidget {
  final _NavRoute tab;
  final bool isSelected;
  final VoidCallback onTap;

  const _NavRailItem({
    required this.tab,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 200),
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: isSelected ? AppColors.hoverTint : null,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isSelected ? tab.activeIcon : tab.icon,
              size: 22,
              color: isSelected
                  ? AppColors.seed
                  : theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(height: 2),
            Text(
              tab.label,
              style: theme.textTheme.bodySmall?.copyWith(
                color: isSelected
                    ? AppColors.seed
                    : theme.colorScheme.onSurfaceVariant,
                fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
                fontSize: 11,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
