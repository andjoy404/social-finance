import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_access.dart';

class IuranMenuScreen extends ConsumerWidget {
  const IuranMenuScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authRepositoryProvider);
    if (authState == null || authState.isLoading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (authState.hasError) {
      return const Scaffold(
        body: Center(
          child: Text('Gagal memuat akses Iuran. Silakan coba lagi.'),
        ),
      );
    }

    final user = authState.valueOrNull;
    if (user == null) {
      return const Scaffold(
        body: Center(child: Text('Profil akun belum tersedia.')),
      );
    }

    final profileStatus = user['profile_status'] as String? ?? 'ready';
    if (profileStatus != 'ready') {
      return IuranProfileStatusScreen(status: profileStatus);
    }

    final canRead = canReadIuran(user);
    final canManage = canManageIuran(user);

    return Scaffold(
      appBar: AppBar(title: const Text('Iuran')),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.base),
        children: [
          if (canRead) ...[
            _IuranMenuItem(
              title: 'List Iuran',
              subtitle: 'Lihat daftar tagihan iuran',
              icon: Icons.receipt_long_outlined,
              onTap: () => context.go('/home/iuran/list'),
            ),
            _IuranMenuItem(
              title: 'Tagihan',
              subtitle: 'Lihat tagihan yang belum lunas',
              icon: Icons.pending_actions_outlined,
              onTap: () => context.go('/home/iuran/tagihan'),
            ),
          ],
          if (canManage)
            _IuranMenuItem(
              title: 'Kelola Iuran',
              subtitle: 'Kelola jenis iuran dan statusnya',
              icon: Icons.settings_outlined,
              onTap: () => context.go('/home/iuran/kelola'),
            ),
          if (!canRead && !canManage)
            const Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Text('Akses Iuran tidak tersedia untuk akun ini.'),
            ),
        ],
      ),
    );
  }
}

class IuranProfileStatusScreen extends ConsumerWidget {
  const IuranProfileStatusScreen({super.key, required this.status});

  final String status;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isLoading = status == 'loading';
    final permissionUnavailable = status == 'permissions_unavailable';
    final message = permissionUnavailable
        ? 'Server belum mengirimkan data permission untuk akun ini.'
        : 'Profil akses gagal dimuat. Hak akses Iuran belum dapat diverifikasi.';

    return Scaffold(
      appBar: AppBar(title: const Text('Iuran')),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.lg),
          child: isLoading
              ? const CircularProgressIndicator()
              : Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(message, textAlign: TextAlign.center),
                    const SizedBox(height: AppSpacing.md),
                    FilledButton(
                      onPressed: () => ref
                          .read(authRepositoryProvider.notifier)
                          .reloadProfile(),
                      child: const Text('Coba Lagi'),
                    ),
                  ],
                ),
        ),
      ),
    );
  }
}

class IuranNavigationShell extends ConsumerWidget {
  const IuranNavigationShell({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final canRead = canReadIuran(user);
    final canManage = canManageIuran(user);
    final location = GoRouterState.of(context).matchedLocation;
    final isTagihan =
        location.startsWith('/home/iuran/tagihan') ||
        location.startsWith('/home/iuran/tunggakan');
    final isKelola =
        location.startsWith('/home/iuran/kelola') ||
        location.startsWith('/home/iuran/form');

    return Scaffold(
      body: SafeArea(
        child: Column(
          children: [
            if (canRead || canManage)
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(
                  horizontal: AppSpacing.sm,
                  vertical: AppSpacing.xs,
                ),
                child: Row(
                  children: [
                    if (canRead)
                      _IuranRouteChip(
                        label: 'List Iuran',
                        icon: Icons.receipt_long_outlined,
                        selected: !isTagihan && !isKelola,
                        onTap: () => context.go('/home/iuran/list'),
                      ),
                    if (canRead)
                      _IuranRouteChip(
                        label: 'Tagihan',
                        icon: Icons.pending_actions_outlined,
                        selected: isTagihan,
                        onTap: () => context.go('/home/iuran/tagihan'),
                      ),
                    if (canManage)
                      _IuranRouteChip(
                        label: 'Kelola Iuran',
                        icon: Icons.settings_outlined,
                        selected: isKelola,
                        onTap: () => context.go('/home/iuran/kelola'),
                      ),
                  ],
                ),
              ),
            Expanded(child: child),
          ],
        ),
      ),
    );
  }
}

class _IuranRouteChip extends StatelessWidget {
  const _IuranRouteChip({
    required this.label,
    required this.icon,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final IconData icon;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.xs),
      child: selected
          ? FilledButton.tonalIcon(
              onPressed: onTap,
              icon: Icon(icon),
              label: Text(label),
            )
          : TextButton.icon(
              onPressed: onTap,
              icon: Icon(icon),
              label: Text(label),
            ),
    );
  }
}

class _IuranMenuItem extends StatelessWidget {
  const _IuranMenuItem({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.onTap,
  });

  final String title;
  final String subtitle;
  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: ListTile(
        leading: Icon(icon),
        title: Text(title),
        subtitle: Text(subtitle),
        trailing: const Icon(Icons.chevron_right),
        onTap: onTap,
      ),
    );
  }
}
