import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import '../../data/api_warga_models.dart';
import '../../data/warga_providers.dart';
import '../dialogs/data_choice_dialog.dart';
import 'package:social_finance/core/models/role.dart';
import '../../../auth/data/mock_auth_repository.dart';

/// Screen displaying the list of RT residents with search capabilities.
class WargaScreen extends ConsumerStatefulWidget {
  const WargaScreen({super.key});

  @override
  ConsumerState<WargaScreen> createState() => _WargaScreenState();
}

/// Jabatan yang hanya muncul dengan tombol edit petugas, bukan edit warga biasa.
const Set<String> kPetugasKhususJabatan = {
  'keamanan',
  'kebersihan_pembangunan',
};

/// Whether the current user has write access to warga data.
bool _hasWargaWriteAccess(WidgetRef ref) {
  final authState = ref.read(authRepositoryProvider);
  final role = authState?.valueOrNull?['role'] as AppRole?;
  return (role != null && kWargaWriteRoles.contains(role));
}

/// Launch DataChoiceDialog and navigate to the selected form.
void _handleFabPressed(BuildContext context, WidgetRef ref, bool isSuperAdmin) {
  showDialog<void>(
    context: context,
    builder: (dialogCtx) {
      return DataChoiceDialog(
        onChoice: (choice) {
          Navigator.of(dialogCtx).pop();
          // Defer navigation until after the dialog dismiss animation completes.
          // Pushing immediately races with the dialog exit animation,
          // causing GoRouter to silently drop the route change.
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (choice == 'warga') {
              context.push('/home/warga/baru');
            } else {
              context.push('/home/warga/baru/petugas');
            }
          });
        },
      );
    },
  );
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
    final isWeb = kIsWeb;
    final wargaState = ref.watch(wargaListProvider);
    final currentQuery = ref.watch(wargaSearchQueryProvider);
    final residents = ref.watch(filteredWargaListProvider);
    final canWrite = _hasWargaWriteAccess(ref);
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final systemRole = user?['system_role'] as String?;
    final isSuperAdmin = systemRole == 'super_admin';

    final errorMessage = switch (wargaState) {
      AsyncError(:final error) =>
        error is ServerException
            ? error.message
            : 'Terjadi kesalahan yang tidak diketahui.',
      _ => '',
    };

    final accentSoft = AppColors.accentSoftColor(context);
    final fabSize = isWeb ? 48.0 : 36.0;
    final fabIconSize = isWeb ? 20.0 : 16.0;
    final fabLabelSize = isWeb ? 13.0 : 11.0;

    final void Function()? onFabPressed = canWrite
        ? () => _handleFabPressed(context, ref, isSuperAdmin)
        : null;

    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Warga'),
        ),
        floatingActionButton: canWrite
            ? isWeb
                ? _buildWebFab(context, fabIconSize, fabLabelSize, accentSoft, onFabPressed!)
                : _buildAndroidFab(context, fabSize, fabIconSize, fabLabelSize, accentSoft, onFabPressed!)
            : null,
        body: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Search Section ──
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.base,
                AppSpacing.sm,
                AppSpacing.base,
                AppSpacing.sm,
              ),
              child: AppTextField(
                controller: _searchController,
                hintText: 'Cari nama, alamat, atau nomor rumah...',
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
              AsyncLoading() => const Center(
                child: CircularProgressIndicator(),
              ),
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
                      SizedBox(
                        height: 36,
                        child: ElevatedButton(
                          onPressed: _retry,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: theme.colorScheme.primary.withValues(alpha: 0.12),
                            foregroundColor: theme.colorScheme.primary,
                            elevation: 0,
                            padding: const EdgeInsets.symmetric(horizontal: 16),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(6),
                              side: BorderSide(
                                color: theme.colorScheme.primary.withValues(alpha: 0.35),
                              ),
                            ),
                            textStyle: const TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                              letterSpacing: 0.01,
                            ),
                          ),
                          child: const Text('Coba Lagi'),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              _ =>
                residents.isEmpty
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
                          return _ResidentCard(
                            resident: resident,
                            isDark: isDark,
                            canWrite: canWrite,
                          );
                        },
                      ),
            },
          ),
        ],
      ),
    ),
  );
}
  Widget _buildWebFab(
    BuildContext context,
    double iconSize,
    double labelSize,
    Color accentSoft,
    VoidCallback onFabPressed,
  ) {
    return Container(
      decoration: BoxDecoration(
        color: accentSoft,
        borderRadius: BorderRadius.circular(24),
        border: Border.all(color: AppColors.accent, width: 1),
        boxShadow: [
          BoxShadow(
            color: AppColors.accent.withValues(alpha: 0.15),
            blurRadius: 8,
            offset: const Offset(0, 0),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onFabPressed,
          borderRadius: BorderRadius.circular(24),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.person_add, size: iconSize, color: AppColors.accent),
                const SizedBox(width: 6),
                Text(
                  'Tambah',
                  style: TextStyle(
                    fontSize: labelSize,
                    fontWeight: FontWeight.w600,
                    color: AppColors.accent,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildAndroidFab(
    BuildContext context,
    double fabSize,
    double iconSize,
    double labelSize,
    Color accentSoft,
    VoidCallback onFabPressed,
  ) {
    return Container(
      decoration: BoxDecoration(
        color: accentSoft,
        borderRadius: BorderRadius.circular(fabSize / 2),
        border: Border.all(color: AppColors.accent, width: 1),
        boxShadow: [
          BoxShadow(
            color: AppColors.accent.withValues(alpha: 0.15),
            blurRadius: 8,
            offset: const Offset(0, 0),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onFabPressed,
          borderRadius: BorderRadius.circular(fabSize / 2),
          child: SizedBox(
            width: fabSize,
            height: fabSize,
            child: Icon(Icons.person_add, size: iconSize, color: AppColors.accent),
          ),
        ),
      ),
    );
  }
}

class _ResidentCard extends StatelessWidget {
  final MappedResident resident;
  final bool isDark;
  final bool canWrite;

  const _ResidentCard({
    required this.resident,
    required this.isDark,
    required this.canWrite,
  });

  String _getInitials(String name) {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.isEmpty) return '';
    if (parts.length == 1) return parts[0].substring(0, 1).toUpperCase();
    return (parts[0][0] + parts[1][0]).toUpperCase();
  }

  Widget _buildJabatanBadge({
    required MappedResident resident,
    required bool isDark,
    required ThemeData theme,
  }) {
    // Guard: if jabatan is null, return null to let caller decide
    final jabatan = resident.jabatan;
    if (jabatan == null || jabatan.isEmpty) return Text('');

    final text = resident.displayJabatan ?? jabatan;
    if (resident.isNeutralJabatan) {
      final neutralText = isDark
          ? AppColors.darkBadgeNeutral
          : AppColors.lightBadgeNeutral;
      final neutralBg = isDark
          ? AppColors.darkBadgeNeutralBg
          : AppColors.lightBadgeNeutralBg;
      final neutralBorder = isDark
          ? AppColors.darkBadgeNeutralBorder
          : AppColors.lightBadgeNeutralBorder;
      return Container(
        padding: const EdgeInsets.symmetric(
          horizontal: 8,
          vertical: 2,
        ),
        decoration: BoxDecoration(
          color: neutralBg,
          borderRadius: BorderRadius.circular(AppRadius.xs),
          border: Border.all(
            color: neutralBorder,
            width: 0.5,
          ),
        ),
        child: Text(
          text,
          style: theme.textTheme.bodySmall?.copyWith(
            fontSize: 10,
            fontWeight: FontWeight.w500,
            color: neutralText,
          ),
        ),
      );
    }
    final color = resident.jabatanColor;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: 8,
        vertical: 2,
      ),
      decoration: BoxDecoration(
        color: color.withValues(alpha: isDark ? 0.18 : 0.10),
        borderRadius: BorderRadius.circular(AppRadius.xs),
        border: Border.all(
          color: color.withValues(alpha: isDark ? 0.35 : 0.25),
          width: 0.5,
        ),
      ),
      child: Text(
        text,
        style: theme.textTheme.bodySmall?.copyWith(
          fontSize: 10,
          fontWeight: FontWeight.w500,
          color: color,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final iconColor = theme.brightness == Brightness.dark
        ? AppColors.darkTextMuted
        : AppColors.lightTextMuted;

    // Occupancy styling matching web Badge variants:
    // OWNER ('Pemilik'): violet accent
    // TENANT ('Penyewa'): blue info
    final isOwner = resident.occupancyStatus == 'Pemilik';
    final occupancyColor = isOwner ? AppColors.accent : AppColors.info;

    // Determine if this resident qualifies as Petugas Khusus for edit routing.
    // Only 'keamanan' and 'kebersihan_pembangunan' get the edit-petugas button;
    // Pengurus RT and warga biasa get edit-warga (household) instead.
    final isPetugasKhusus =
        resident.jabatan != null &&
            kPetugasKhususJabatan.contains(resident.jabatan!);

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

              // Name and Occupancy / Jabatan Badges
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
                        if (resident.jabatan != null && resident.jabatan!.isNotEmpty)
                          _buildJabatanBadge(
                            resident: resident,
                            isDark: isDark,
                            theme: theme,
                          ),
                      ],
                    ),
                  ],
                ),
              ),
              if (canWrite &&
                  !kIsWeb &&
                  (isPetugasKhusus || resident.householdId != null))
                IconButton(
                  icon: const Icon(Icons.edit_outlined, size: 20),
                  tooltip: 'Ubah',
                  color: AppColors.accent,
                  onPressed: () {
                    if (isPetugasKhusus) {
                      context.push('/home/warga/edit/petugas/${resident.id}');
                    } else if (resident.householdId != null) {
                      context.push(
                        '/home/warga/edit/household/${resident.householdId}',
                      );
                    }
                  },
                ),
            ],
          ),
          const Divider(height: 20),

          // Details in exact order:
          // 1. Nomor Rumah
          // 2. Nomor Telepon
          // 3. Email
          if (resident.houseNumber.isNotEmpty) ...[
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
            if ((resident.phoneNumber != null &&
                    resident.phoneNumber!.isNotEmpty) ||
                (resident.email != null && resident.email!.isNotEmpty))
              const SizedBox(height: 6),
          ],
          if (resident.phoneNumber != null &&
              resident.phoneNumber!.isNotEmpty) ...[
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
            if (resident.email != null && resident.email!.isNotEmpty)
              const SizedBox(height: 6),
          ],
          if (resident.email != null && resident.email!.isNotEmpty) ...[
            Row(
              children: [
                Icon(Icons.email_outlined, size: 16, color: iconColor),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    resident.email!,
                    style: theme.textTheme.bodyMedium,
                  ),
                ),
              ],
            ),
          ],

          // ── Ubah Action (Web only, mobile uses header IconButton) ──
          if (canWrite && kIsWeb) ...[
            if (isPetugasKhusus)
              Container(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(AppRadius.xs),
                  border: Border.all(color: AppColors.accent, width: 1.5),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.accent.withValues(alpha: 0.15),
                      blurRadius: 8,
                      offset: const Offset(0, 0),
                    ),
                  ],
                ),
                child: Material(
                  color: Colors.transparent,
                  child: InkWell(
                    onTap: () {
                      context.push('/home/warga/edit/petugas/${resident.id}');
                    },
                    borderRadius: BorderRadius.circular(AppRadius.xs),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        vertical: AppSpacing.xxs,
                        horizontal: AppSpacing.sm,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(Icons.edit_outlined, size: 16),
                          const SizedBox(width: 4),
                          Text(
                            'Ubah',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                              color: AppColors.accent,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
            if (resident.householdId != null && !isPetugasKhusus)
              Container(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(AppRadius.xs),
                  border: Border.all(color: AppColors.accent, width: 1.5),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.accent.withValues(alpha: 0.15),
                      blurRadius: 8,
                      offset: const Offset(0, 0),
                    ),
                  ],
                ),
                child: Material(
                  color: Colors.transparent,
                  child: InkWell(
                    onTap: () {
                      context.push(
                        '/home/warga/edit/household/${resident.householdId}',
                      );
                    },
                    borderRadius: BorderRadius.circular(AppRadius.xs),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        vertical: AppSpacing.xxs,
                        horizontal: AppSpacing.sm,
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Icon(Icons.edit_outlined, size: 16),
                          const SizedBox(width: 4),
                          Text(
                            'Ubah',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                              color: AppColors.accent,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
          ],
        ],
      ),
    );
  }
}
