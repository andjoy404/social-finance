import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

class RwListScreen extends ConsumerStatefulWidget {
  const RwListScreen({super.key});

  @override
  ConsumerState<RwListScreen> createState() => _RwListScreenState();
}

class _RwListScreenState extends ConsumerState<RwListScreen> {
  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final rwState = ref.watch(rwListProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Kelola RW'),
      ),
      body: switch (rwState) {
        AsyncLoading() => const Center(child: CircularProgressIndicator()),
        AsyncError(:final error) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(error.toString()),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: () => ref.read(rwListProvider.notifier).refresh(),
                child: const Text('Coba Lagi'),
              ),
            ],
          ),
        ),
        AsyncData(:final value) =>
          value.isEmpty
              ? const Center(
                  child: EmptyState(
                    icon: Icons.location_city_outlined,
                    title: 'Belum Ada RW',
                    description: 'Belum ada data RW yang terdaftar.',
                  ),
                )
              : ListView.separated(
                  padding: const EdgeInsets.all(AppSpacing.base),
                  itemCount: value.length,
                  separatorBuilder: (context, index) =>
                      const SizedBox(height: AppSpacing.sm),
                  itemBuilder: (context, index) {
                    final rw = value[index];
                    return _RwCard(rw: rw, isDark: isDark);
                  },
                ),
        _ => const SizedBox.shrink(),
      },
    );
  }
}

class _RwCard extends StatelessWidget {
  final RwModel rw;
  final bool isDark;

  const _RwCard({required this.rw, required this.isDark});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return AppCard(
      padding: const EdgeInsets.all(AppSpacing.base),
      child: Row(
        children: [
          CircleAvatar(
            radius: 20,
            backgroundColor: AppColors.accent.withValues(
              alpha: isDark ? 0.25 : 0.12,
            ),
            child: Text(
              rw.rwNumber.toString().padLeft(2, '0'),
              style: TextStyle(
                color: AppColors.accent,
                fontWeight: FontWeight.bold,
                fontSize: 14,
              ),
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  rw.displayName,
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (rw.headName != null && rw.headName!.isNotEmpty)
                  Text(
                    'Ketua: ${rw.headName}',
                    style: theme.textTheme.bodySmall,
                  ),
                if (rw.address != null && rw.address!.isNotEmpty)
                  Text(
                    rw.address!,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
              ],
            ),
          ),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: (rw.isActive ? AppColors.success : AppColors.danger)
                  .withValues(alpha: isDark ? 0.25 : 0.12),
              borderRadius: BorderRadius.circular(AppRadius.xs),
            ),
            child: Text(
              rw.isActive ? 'Aktif' : 'Nonaktif',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: rw.isActive ? AppColors.success : AppColors.danger,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
