import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_access.dart';
import 'iuran_screen.dart';

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

    if (canRead || canManage) {
      return const IuranScreen();
    }

    return const Scaffold(
      body: Center(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Text('Akses Iuran tidak tersedia untuk akun ini.'),
        ),
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
    return child;
  }
}
