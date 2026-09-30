import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../theme/app_colors.dart';
import '../../features/auth/data/mock_auth_repository.dart';
import '../../features/warga/data/api_warga_models.dart';
import '../../features/warga/data/warga_providers.dart';

/// Reusable AppBar title widget for all main menu screens.
///
/// Follows the format:
/// ```text
///           Lingkungan RT [RT 000] RW [000]
/// ⬅ [TITLE] |
///           Nama Lokasi RT
/// ```
///
/// The menu [title] (e.g. WARGA, BERANDA, KAS, IURAN, PROFIL) is vertically
/// centered with respect to the two-line subtitle/location block, with a subtle
/// vertical divider between them.
class MenuAppBarTitle extends ConsumerWidget {
  final String title;
  final String? rt;
  final String? rw;
  final String? location;

  const MenuAppBarTitle({
    super.key,
    required this.title,
    this.rt,
    this.rw,
    this.location,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;

    final wargaAsync = ref.watch(wargaListProvider);
    final residents = switch (wargaAsync) {
      AsyncData(:final value) => value,
      _ => const <MappedResident>[],
    };
    final firstResident = residents.isNotEmpty ? residents.first : null;

    final userRt = user?['rt']?.toString();
    final userRw = user?['rw']?.toString();

    final effectiveRt = (rt != null && rt!.isNotEmpty)
        ? rt!
        : ((userRt != null && userRt.isNotEmpty)
            ? userRt
            : (firstResident?.rtNumber != null && firstResident!.rtNumber!.isNotEmpty
                ? firstResident.rtNumber!
                : '001'));

    final effectiveRw = (rw != null && rw!.isNotEmpty)
        ? rw!
        : ((userRw != null && userRw.isNotEmpty)
            ? userRw
            : (firstResident?.rw != null
                ? '${firstResident!.rw}'
                : '002'));

    final effectiveLocation = (location != null && location!.isNotEmpty)
        ? location!
        : (user?['rt_address'] as String? ??
            user?['alamat'] as String? ??
            user?['rt_name'] as String? ??
            (firstResident?.address != null && firstResident!.address!.isNotEmpty
                ? firstResident.address!
                : (firstResident?.rtName != null && firstResident!.rtName!.isNotEmpty
                    ? firstResident.rtName!
                    : 'Wisma Rukun Tunggal')));

    final rtText = effectiveRt.isNotEmpty ? 'RT $effectiveRt' : 'RT';
    final rwText = effectiveRw.isNotEmpty ? 'RW $effectiveRw' : '';
    final line1 = rwText.isNotEmpty
        ? 'Lingkungan $rtText $rwText'
        : 'Lingkungan $rtText';
    final line2 = effectiveLocation;

    return Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Text(
          title.toUpperCase(),
          style: TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w700,
            letterSpacing: 0.5,
            color: isDark ? AppColors.darkText : AppColors.lightText,
          ),
        ),
        const SizedBox(width: 8),
        Container(
          width: 1.5,
          height: 28,
          decoration: BoxDecoration(
            color: isDark
                ? AppColors.darkBorder.withValues(alpha: 0.8)
                : AppColors.lightBorder,
            borderRadius: BorderRadius.circular(1),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(
                line1,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: isDark ? AppColors.darkText : AppColors.lightText,
                  height: 1.2,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                line2,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.normal,
                  color: isDark ? AppColors.darkTextMuted : AppColors.lightTextMuted,
                  height: 1.2,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
