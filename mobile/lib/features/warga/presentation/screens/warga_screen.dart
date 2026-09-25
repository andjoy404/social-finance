import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import '../../data/warga_providers.dart';
import '../../data/api_warga_models.dart';

/// Screen displaying the list of RT residents with search capabilities.
class WargaScreen extends ConsumerStatefulWidget {
  const WargaScreen({super.key});

  @override
  ConsumerState<WargaScreen> createState() => _WargaScreenState();
}

class _WargaScreenState extends ConsumerState<WargaScreen> {
  late final TextEditingController _searchController;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _searchController.text = ref.read(wargaSearchQueryProvider);
    });
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _clearSearch() {
    _searchController.clear();
    ref.read(wargaSearchQueryProvider.notifier).state = '';
    setState(() {});
  }

  void _retry() {
    ref.read(wargaListProvider.notifier).refresh();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final wargaState = ref.watch(wargaListProvider);
    final currentQuery = ref.watch(wargaSearchQueryProvider);
    final residents = ref.watch(filteredWargaListProvider);

    final errorMessage = switch (wargaState) {
      AsyncError(:final error) =>
        error is ServerException ? error.message : 'Terjadi kesalahan yang tidak diketahui.',
      _ => '',
    };

    return Scaffold(
      appBar: AppBar(title: const Text('Warga')),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── Search & Header Section ──
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.base,
              AppSpacing.sm,
              AppSpacing.base,
              AppSpacing.sm,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Daftar warga dan kepala keluarga di lingkungan RT.',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                AppTextField(
                  controller: _searchController,
                  hintText: 'Cari nama, NIK, atau nomor rumah...',
                  prefixIcon: const Icon(Icons.search),
                  suffixIcon: _searchController.text.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.clear, size: 20),
                          onPressed: _clearSearch,
                        )
                      : null,
                  onChanged: (value) {
                    ref.read(wargaSearchQueryProvider.notifier).state = value;
                    setState(() {});
                  },
                ),
              ],
            ),
          ),

          // ── Summary Count ──
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.base,
              vertical: AppSpacing.xs,
            ),
            child: Text(
              'Menampilkan ${residents.length} warga',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),

          // ── Content: Loading / Error / Empty / List ──
          Expanded(
            child: switch (wargaState) {
              AsyncLoading() => const Center(child: CircularProgressIndicator()),
              AsyncError() => Center(
                  child: SingleChildScrollView(
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.error_outline,
                          size: 64,
                          color: theme.colorScheme.error.withValues(alpha: 0.5),
                        ),
                        const SizedBox(height: 16),
                        Text(
                          'Terjadi Kesalahan',
                          style: theme.textTheme.titleLarge,
                        ),
                        const SizedBox(height: 8),
                        Text(
                          errorMessage,
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodyMedium,
                        ),
                        const SizedBox(height: 16),
                        ElevatedButton(
                          onPressed: _retry,
                          child: const Text('Coba Lagi'),
                        ),
                      ],
                    ),
                  ),
                ),
              _ => residents.isEmpty
                  ? Center(
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(AppSpacing.lg),
                        child: currentQuery.isNotEmpty
                            ? EmptyState(
                                icon: Icons.search_off,
                                title: 'Warga Tidak Ditemukan',
                                description:
                                    'Tidak ada data warga yang sesuai dengan "$currentQuery".',
                                actionText: 'Hapus Pencarian',
                                onAction: _clearSearch,
                              )
                            : const EmptyState(
                                icon: Icons.people_outline,
                                title: 'Belum Ada Warga',
                                description:
                                    'Data kependudukan warga RT belum tersedia.',
                              ),
                      ),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.fromLTRB(
                        AppSpacing.base,
                        AppSpacing.xs,
                        AppSpacing.base,
                        AppSpacing.xl,
                      ),
                      itemCount: residents.length,
                      separatorBuilder: (context, index) =>
                          const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) {
                        final resident = residents[index];
                        return _ResidentCard(resident: resident, isDark: isDark);
                      },
                    ),
            },
          ),
        ],
      ),
    );
  }
}

class _ResidentCard extends StatelessWidget {
  final MappedResident resident;
  final bool isDark;

  const _ResidentCard({required this.resident, required this.isDark});

  String _getInitials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.isEmpty) return '';
    if (parts.length == 1) return parts[0].substring(0, 1).toUpperCase();
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final iconColor = AppColors.iconTint(context);

    // Occupancy styling matching web Badge variants:
    // OWNER ('Pemilik'): violet accent
    // TENANT ('Penyewa'): blue accent
    final isOwner = resident.occupancyStatus == 'Pemilik';
    final occupancyColor = isOwner
        ? (isDark ? const Color(0xFF9B8FD6) : AppColors.seed)
        : (isDark ? const Color(0xFF60A5FA) : const Color(0xFF1976D2));

    return AppCard(
      padding: const EdgeInsets.all(AppSpacing.base),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              // Avatar with Initials
              CircleAvatar(
                radius: 20,
                backgroundColor: theme.colorScheme.primary.withValues(
                  alpha: isDark ? 0.25 : 0.12,
                ),
                child: Text(
                  _getInitials(resident.name),
                  style: TextStyle(
                    color: theme.colorScheme.primary,
                    fontWeight: FontWeight.bold,
                    fontSize: 14,
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.md),

              // Name and Relationship / Occupancy Badges
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      resident.name,
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: [
                        AppBadge(
                          label: resident.isHeadOfHousehold
                              ? 'Kepala Keluarga'
                              : resident.relationship,
                          isNeonStyle: isDark && resident.isHeadOfHousehold,
                          backgroundColor: resident.isHeadOfHousehold
                              ? null
                              : theme.colorScheme.surfaceContainerHighest,
                          textColor: resident.isHeadOfHousehold
                              ? null
                              : theme.colorScheme.onSurfaceVariant,
                        ),
                        if (resident.occupancyStatus != null)
                          Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: occupancyColor.withValues(
                                alpha: isDark ? 0.18 : 0.10,
                              ),
                              borderRadius: BorderRadius.circular(AppRadius.xs),
                              border: Border.all(
                                color: occupancyColor.withValues(
                                  alpha: isDark ? 0.35 : 0.25,
                                ),
                                width: 0.5,
                              ),
                            ),
                            child: Text(
                              resident.occupancyStatus!,
                              style: theme.textTheme.bodySmall?.copyWith(
                                fontSize: 10,
                                fontWeight: FontWeight.w500,
                                color: occupancyColor,
                              ),
                            ),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
          const Divider(height: 20),

          // Details: House, NIK, Phone
          Row(
            children: [
              Icon(Icons.home_outlined, size: 16, color: iconColor),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  resident.houseNumber,
                  style: theme.textTheme.bodyMedium,
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              Icon(Icons.badge_outlined, size: 16, color: iconColor),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'NIK: ${resident.maskedNik}',
                  style: theme.textTheme.bodyMedium,
                ),
              ),
            ],
          ),
          if (resident.phoneNumber != null) ...[
            const SizedBox(height: 6),
            Row(
              children: [
                Icon(Icons.phone_outlined, size: 16, color: iconColor),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    resident.phoneNumber!,
                    style: theme.textTheme.bodyMedium,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
