import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
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
import '../../data/warga_repository.dart';
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
  'operasional',
  'kebersihan',
};

/// Jabatan pengurus RT yang memiliki akses tulis warga (position_permissions).
/// Must not include AppRole.petugas or kPetugasKhususJabatan entries.
const Set<String> kWargaWriteJabatans = {
  'ketua',
  'wakil_ketua',
  'sekretaris',
  'bendahara',
};

/// Whether the current user has write access to warga data.
///
/// Evaluation order:
/// 1. If system_role == 'super_admin' → always allowed.
/// 2. If jabatan is non-empty → only allowed when jabatan is in
///    [kWargaWriteJabatans].
/// 3. Otherwise fall through to the existing role-based check
///    ([kWargaWriteRoles]).
bool _hasWargaWriteAccess(WidgetRef ref) {
  final authState = ref.read(authRepositoryProvider);
  final user = authState?.valueOrNull;
  final systemRole = user?['system_role']?.toString();

  // 1. SUPER_ADMIN always has write access.
  if (systemRole == 'super_admin') return true;

  // 2. Check jabatan-based authorization first.
  final jabatan = (user?['jabatan'] as String? ?? '').trim().toLowerCase();
  if (jabatan.isNotEmpty) {
    return kWargaWriteJabatans.contains(jabatan);
  }

  // 3. Fallback: role-based authorization (empty/null jabatan).
  final role = user?['role'] is AppRole
      ? user!['role'] as AppRole
      : (user?['role'] is String
          ? mapStringToAppRole(user!['role'] as String, systemRoleStr: systemRole)
          : null);
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
          WidgetsBinding.instance.addPostFrameCallback((_) async {
            if (choice == 'warga') {
              await context.push('/home/warga/baru');
            } else {
              await context.push('/home/warga/baru/petugas');
            }
            ref.read(wargaListProvider.notifier).refresh();
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
    final systemRole = user?['system_role']?.toString();
    final isSuperAdmin = systemRole == 'super_admin';
    final userRole = user?['role'] is AppRole
        ? user!['role'] as AppRole
        : (user?['role'] is String
            ? mapStringToAppRole(user!['role'] as String, systemRoleStr: systemRole)
            : null);
    final isWarga = userRole == AppRole.warga;

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
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Warga'),
        ),
        floatingActionButton: canWrite
            ? isWeb
                  ? _buildWebFab(
                      context,
                      fabIconSize,
                      fabLabelSize,
                      accentSoft,
                      onFabPressed!,
                    )
                  : _buildAndroidFab(
                      context,
                      fabSize,
                      fabIconSize,
                      fabLabelSize,
                      accentSoft,
                      onFabPressed!,
                    )
            : null,
        body: RefreshIndicator(
          onRefresh: () => ref.read(wargaListProvider.notifier).refresh(),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── Search Section ──
              Padding(
                padding: const EdgeInsets.only(
                  top: AppSpacing.sm,
                  left: AppSpacing.base,
                  right: AppSpacing.base,
                  bottom: AppSpacing.sm,
                ),
                child: SizedBox(
                  height: 38,
                  child: AppTextField(
                    controller: _searchController,
                    hintText: 'Cari nama, alamat, atau nomor rumah...',
                    isDense: true,
                    fillColor: theme.scaffoldBackgroundColor,
                    textAlignVertical: TextAlignVertical.center,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 0,
                    ),
                    prefixIconConstraints: const BoxConstraints(
                      minWidth: 36,
                      minHeight: 38,
                    ),
                    suffixIconConstraints: const BoxConstraints(
                      minWidth: 36,
                      minHeight: 38,
                    ),
                    prefixIcon: const Icon(Icons.search, size: 18),
                    suffixIcon: _searchController.text.isNotEmpty
                        ? IconButton(
                            padding: EdgeInsets.zero,
                            icon: const Icon(Icons.clear, size: 18),
                            onPressed: _clearSearch,
                          )
                        : null,
                    onChanged: (value) {
                      ref.read(wargaSearchQueryProvider.notifier).state = value;
                      setState(() {});
                    },
                  ),
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
                child: Builder(
                  builder: (context) {
                    if (wargaState.isLoading && !wargaState.hasValue) {
                      return const Center(child: CircularProgressIndicator());
                    }
                    if (wargaState.hasError && !wargaState.hasValue) {
                      return Center(
                        child: SingleChildScrollView(
                          physics: const AlwaysScrollableScrollPhysics(),
                          padding: const EdgeInsets.all(AppSpacing.lg),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(
                                Icons.error_outline,
                                size: 64,
                                color: theme.colorScheme.error.withValues(
                                  alpha: 0.5,
                                ),
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
                                    backgroundColor: theme.colorScheme.primary
                                        .withValues(alpha: 0.12),
                                    foregroundColor: theme.colorScheme.primary,
                                    elevation: 0,
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 16,
                                    ),
                                    shape: RoundedRectangleBorder(
                                      borderRadius: BorderRadius.circular(6),
                                      side: BorderSide(
                                        color: theme.colorScheme.primary
                                            .withValues(alpha: 0.35),
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
                      );
                    }
                    if (residents.isEmpty) {
                      return Center(
                        child: SingleChildScrollView(
                          physics: const AlwaysScrollableScrollPhysics(),
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
                      );
                    }
                    return ListView.separated(
                      physics: const AlwaysScrollableScrollPhysics(),
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
                    );
                  },
                ),
              ),
            ],
          ),
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
    final button = Container(
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
        clipBehavior: Clip.none,
        child: InkWell(
          onTap: onFabPressed,
          borderRadius: BorderRadius.circular(24),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Icon(Icons.person_add, size: iconSize, color: AppColors.accent),
          ),
        ),
      ),
    );
    return Tooltip(message: 'Tambah Warga', child: button);
  }

  Widget _buildAndroidFab(
    BuildContext context,
    double fabSize,
    double iconSize,
    double labelSize,
    Color accentSoft,
    VoidCallback onFabPressed,
  ) {
    final button = Container(
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
        clipBehavior: Clip.none,
        child: InkWell(
          onTap: onFabPressed,
          borderRadius: BorderRadius.circular(24),
          child: const SizedBox(
            height: 44,
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: 16),
              child: Icon(Icons.person_add, size: 20, color: AppColors.accent),
            ),
          ),
        ),
      ),
    );
    return Tooltip(message: 'Tambah Warga', child: button);
  }
}

class _ResidentCard extends ConsumerWidget {
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
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        decoration: BoxDecoration(
          color: neutralBg,
          borderRadius: BorderRadius.circular(AppRadius.xs),
          border: Border.all(color: neutralBorder, width: 0.5),
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
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
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
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final homeIconColor = isDark ? const Color(0xFFFBBF24) : const Color(0xFFD97706);
    final phoneIconColor = isDark ? AppColors.success : const Color(0xFF16A34A);
    final emailIconColor = isDark ? AppColors.info : const Color(0xFF2563EB);

    // Warga can see phone only from residents whose jabatan is 'keamanan'.
    // Warga can NEVER see email — only pengurus/bendahara/super_admin.
    final authData = ref.read(authRepositoryProvider);
    final user = authData?.valueOrNull;
    final userRole = user?['role'] is AppRole
        ? user!['role'] as AppRole
        : (user?['role'] is String
            ? mapStringToAppRole(user!['role'] as String, systemRoleStr: user['system_role']?.toString())
            : null);
    final userJabatan = (user?['jabatan'] as String? ?? '').trim().toLowerCase();
    final systemRole = user?['system_role']?.toString();
    final isSuperAdmin = systemRole == 'super_admin';
    final isPengurus = userRole == AppRole.pengurus ||
        userRole == AppRole.bendahara ||
        kWargaWriteJabatans.contains(userJabatan);

    final showPhone = (() {
      if (user == null) return false;
      if (isSuperAdmin || isPengurus) return true;
      final jabatan = (resident.jabatan ?? '').trim().toLowerCase();
      return jabatan == 'keamanan';
    })();
    // Warga can NEVER see email — only pengurus/bendahara/super_admin.
    final showEmail = (() {
      if (user == null) return false;
      if (isSuperAdmin || isPengurus) return true;
      return false;
    })();

    // Occupancy styling matching web Badge variants:
    // OWNER ('Pemilik'): violet accent
    // TENANT ('Penyewa'): blue info
    final isOwner = resident.occupancyStatus == 'Pemilik';
    final occupancyColor = isOwner ? AppColors.accent : AppColors.info;

    // Determine if this resident qualifies as Petugas Khusus for edit routing.
    // Only 'keamanan', 'operasional', and 'kebersihan' get the edit-petugas button;
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
                        if (resident.jabatan != null &&
                            resident.jabatan!.isNotEmpty)
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
              if (canWrite && !kIsWeb) ...[
                IconButton(
                  icon: const Icon(Icons.vpn_key_outlined, size: 20),
                  tooltip: 'Reset Password',
                  color: AppColors.accent,
                  onPressed: () => _handleResetPassword(context, ref, resident),
                ),
                IconButton(
                  icon: const Icon(Icons.edit_outlined, size: 20),
                  tooltip: 'Ubah',
                  color: AppColors.accent,
                  onPressed: () async {
                    if (resident.householdId != null) {
                      await context.push(
                        '/home/warga/edit/household/${resident.householdId}',
                      );
                    } else {
                      await context.push(
                        '/home/warga/edit/petugas/${resident.id}',
                      );
                    }
                    ref.read(wargaListProvider.notifier).refresh();
                  },
                ),
              ],
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
                Icon(Icons.home_outlined, size: 16, color: homeIconColor),
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
          if (showPhone && resident.phoneNumber != null && resident.phoneNumber!.isNotEmpty) ...[
            Row(
              children: [
                Icon(Icons.phone_outlined, size: 16, color: phoneIconColor),
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
          if (showEmail && resident.email != null && resident.email!.isNotEmpty) ...[
            Row(
              children: [
                Icon(Icons.email_outlined, size: 16, color: emailIconColor),
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
                  clipBehavior: Clip.none,
                  child: InkWell(
                    onTap: () async {
                      await context.push(
                        '/home/warga/edit/petugas/${resident.id}',
                      );
                      ref.read(wargaListProvider.notifier).refresh();
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
                  clipBehavior: Clip.none,
                  child: InkWell(
                    onTap: () async {
                      await context.push(
                        '/home/warga/edit/household/${resident.householdId}',
                      );
                      ref.read(wargaListProvider.notifier).refresh();
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

  Future<void> _handleResetPassword(
    BuildContext context,
    WidgetRef ref,
    MappedResident resident,
  ) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Reset Password Warga'),
        content: Text(
          'Buat 4 digit kode reset kata sandi untuk ${resident.name}?\n\nKode ini berlaku selama 24 jam.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Buat Kode'),
          ),
        ],
      ),
    );

    if (confirm != true || !context.mounted) return;

    try {
      final res = await ref
          .read(wargaRepositoryProvider)
          .generateResetPasswordCode(resident.id);
      if (!context.mounted) return;

      final code = res['code']?.toString() ?? '';
      showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('Kode Reset Kata Sandi'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                resident.name,
                style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 16),
              ),
              const SizedBox(height: 12),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
                decoration: BoxDecoration(
                  color: AppColors.accent.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: AppColors.accent.withValues(alpha: 0.4)),
                ),
                child: Text(
                  code,
                  style: const TextStyle(
                    fontSize: 32,
                    fontWeight: FontWeight.bold,
                    letterSpacing: 8,
                    fontFamily: 'monospace',
                    color: AppColors.accent,
                  ),
                ),
              ),
              const SizedBox(height: 12),
              const Text(
                'Berikan 4 digit kode ini kepada warga untuk mereset kata sandi melalui menu "Lupa Kata Sandi?" pada halaman Login.\n\nKode berlaku 24 jam dan hanya dapat digunakan 1 kali.',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 12, color: Colors.grey),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Tutup'),
            ),
            FilledButton.icon(
              icon: const Icon(Icons.copy, size: 16),
              label: const Text('Salin Kode'),
              onPressed: () {
                Clipboard.setData(ClipboardData(text: code));
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('Kode reset berhasil disalin')),
                );
                Navigator.of(ctx).pop();
              },
            ),
          ],
        ),
      );
    } catch (e) {
      if (!context.mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Gagal membuat kode: $e'),
          backgroundColor: AppColors.danger,
        ),
      );
    }
  }
}
