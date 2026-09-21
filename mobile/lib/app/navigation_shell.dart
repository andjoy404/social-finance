import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/features/cash/presentation/screens/cash_screen.dart';
import 'package:social_finance/features/dues/presentation/screens/dues_screen.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/profile/presentation/screens/profile_screen.dart';
import 'package:social_finance/features/reports/presentation/screens/reports_screen.dart';

class MainNavigationShell extends StatefulWidget {
  final Widget child;

  const MainNavigationShell({super.key, required this.child});

  @override
  State<MainNavigationShell> createState() => _MainNavigationShellState();
}

class _MainNavigationShellState extends State<MainNavigationShell> {
  int _currentIndex = 0;

  static const _tabs = [
    _NavRoute(
      path: '/home',
      label: 'Beranda',
      icon: Icons.home_outlined,
      activeIcon: Icons.home,
    ),
    _NavRoute(
      path: '/home/cash',
      label: 'Kas',
      icon: Icons.account_balance_wallet_outlined,
      activeIcon: Icons.account_balance_wallet,
    ),
    _NavRoute(
      path: '/home/dues',
      label: 'Iuran',
      icon: Icons.receipt_long_outlined,
      activeIcon: Icons.receipt_long,
    ),
    _NavRoute(
      path: '/home/reports',
      label: 'Laporan',
      icon: Icons.bar_chart_outlined,
      activeIcon: Icons.bar_chart,
    ),
    _NavRoute(
      path: '/home/profile',
      label: 'Profil',
      icon: Icons.person_outline,
      activeIcon: Icons.person,
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        bottom: false,
        child: IndexedStack(
          index: _currentIndex,
          children: const [
            DashboardScreen(),
            CashScreen(),
            DuesScreen(),
            ReportsScreen(),
            ProfileScreen(),
          ],
        ),
      ),
      bottomNavigationBar: BottomAppBar(
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceAround,
          children: List.generate(
            _tabs.length,
            (index) => _NavRailItem(
              tab: _tabs[index],
              isSelected: index == _currentIndex,
              onTap: () => _onTabTapped(index),
            ),
          ),
        ),
      ),
    );
  }

  void _onTabTapped(int index) {
    final tab = _tabs[index];
    if (GoRouterState.of(context).matchedLocation != tab.path) {
      context.go(tab.path);
    }
    setState(() => _currentIndex = index);
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
          color: isSelected ? const Color(0x1A7C5FCE) : null,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isSelected ? tab.activeIcon : tab.icon,
              size: 22,
              color: isSelected
                  ? const Color(0xFF7C5FCE)
                  : theme.colorScheme.onSurfaceVariant,
            ),
            const SizedBox(height: 2),
            Text(
              tab.label,
              style: theme.textTheme.bodySmall?.copyWith(
                color: isSelected
                    ? const Color(0xFF7C5FCE)
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
