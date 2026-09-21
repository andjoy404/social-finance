import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

class ProfileScreen extends ConsumerWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final authState = ref.watch(authRepositoryProvider);
    final authRepo = ref.read(authRepositoryProvider.notifier);

    final user = authState!.value;
    final userName = user?['name'] ?? 'Pengguna';
    final role = user?['role'] as AppRole? ?? AppRole.warga;
    final rt = user?['rt'] ?? '';
    final rw = user?['rw'] ?? '';

    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBar(title: const Text('Profil')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(20.0),
        child: Column(
          children: [
            const SizedBox(height: 24),
            // Avatar
            CircleAvatar(
              radius: 40,
              backgroundColor: theme.colorScheme.primary.withValues(
                alpha: isDark ? 0.25 : 0.15,
              ),
              child: Icon(
                Icons.person,
                size: 40,
                color: theme.colorScheme.primary,
              ),
            ),
            const SizedBox(height: 16),
            // Identity
            Text(userName, style: theme.textTheme.headlineSmall),
            const SizedBox(height: 8),
            AppBadge(
              label: appRoleDisplayName(role),
              isNeonStyle: isDark,
              backgroundColor: isDark
                  ? null
                  : theme.colorScheme.primary.withValues(alpha: 0.12),
            ),
            const SizedBox(height: 4),
            Text('RT $rt / RW $rw', style: theme.textTheme.bodySmall),
            const SizedBox(height: 32),

            // Profile items
            _buildProfileSection(theme, [
              _ProfileItemData(
                icon: Icons.home,
                title: 'RT / RW',
                subtitle: 'RT $rt / RW $rw',
              ),
              _ProfileItemData(
                icon: Icons.security,
                title: 'Keamanan',
                subtitle: 'Ubah kata sandi',
              ),
              _ProfileItemData(
                icon: Icons.info_outline,
                title: 'Tentang',
                subtitle: 'Social Finance v1.0.0',
              ),
            ], isDark),
            const SizedBox(height: 40),
            // Logout
            SizedBox(
              width: double.infinity,
              height: 48,
              child: OutlinedButton(
                onPressed: () => authRepo.logout(),
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: AppColors.expense),
                  foregroundColor: AppColors.expense,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                ),
                child: const Text('Keluar'),
              ),
            ),
            const SizedBox(height: 16),
          ],
        ),
      ),
    );
  }

  Widget _buildProfileSection(
    ThemeData theme,
    List<_ProfileItemData> items,
    bool isDark,
  ) {
    return Column(
      children: items.map((item) {
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: AppCard(
            padding: EdgeInsets.zero,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
              child: Row(
                children: [
                  Icon(item.icon, size: 20, color: theme.colorScheme.primary),
                  const SizedBox(width: 14),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(item.title, style: theme.textTheme.titleMedium),
                        Text(item.subtitle, style: theme.textTheme.bodySmall),
                      ],
                    ),
                  ),
                  Icon(
                    Icons.chevron_right,
                    size: 20,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ],
              ),
            ),
          ),
        );
      }).toList(),
    );
  }
}

class _ProfileItemData {
  final IconData icon;
  final String title;
  final String subtitle;
  const _ProfileItemData({
    required this.icon,
    required this.title,
    required this.subtitle,
  });
}
