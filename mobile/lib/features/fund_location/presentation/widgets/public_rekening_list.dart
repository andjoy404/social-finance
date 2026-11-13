import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';

/// Daftar nomor rekening aktif (Lokasi Dana Rekening) yang public untuk seluruh
/// warga RT — dipakai di BERANDA dan halaman pembayaran Iuran (Transfer).
///
/// Setiap item menampilkan: nama lokasi dana, bank + nomor rekening (monospace,
/// bisa dicopy), dan A/N pemegang rekening.
class PublicRekeningList extends ConsumerWidget {
  final bool compact;
  /// Ultra-compact mode for bottom navbar: single line, no icon, no header.
  final bool ultraCompact;

  const PublicRekeningList({super.key, this.compact = false, this.ultraCompact = false});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final async = ref.watch(publicRekeningProvider);
    return async.when(
      data: (list) => ultraCompact
          ? _buildUltraCompactList(context, theme, list)
          : _buildList(context, theme, list),
      loading: () => compact
          ? const Padding(
              padding: EdgeInsets.symmetric(vertical: 6),
              child: Center(
                child: SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            )
          : const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Center(
                child: SizedBox(
                  width: 24,
                  height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2.5),
                ),
              ),
            ),
      error: (e, s) => _buildError(context, ref, theme, e),
    );
  }

  Widget _buildError(BuildContext context, WidgetRef ref, ThemeData theme, Object error) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Icon(Icons.error_outline, size: 18, color: theme.colorScheme.error),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              'Gagal memuat nomor rekening — tarik untuk refresh.',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
          ),
          TextButton(
            onPressed: () => ref.invalidate(publicRekeningProvider),
            child: const Text('Coba Lagi'),
          ),
        ],
      ),
    );
  }

  Widget _buildList(BuildContext context, ThemeData theme, List<PublicRekening> list) {
    if (list.isEmpty) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          children: [
            Icon(Icons.account_balance_outlined,
                size: 18, color: theme.colorScheme.onSurfaceVariant),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Belum ada nomor rekening terdaftar.',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
          ],
        ),
      );
    }
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ...list.map((r) {
          final bank = (r.bankName ?? '').trim();
          final number = (r.accountNumber ?? '').trim();
          final holder = (r.accountHolder ?? '').trim();
          return Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: GestureDetector(
              onTap: number.isNotEmpty
                  ? () {
                      Clipboard.setData(ClipboardData(text: number));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text('Nomor rekening $number disalin'),
                          behavior: SnackBarBehavior.floating,
                          duration: const Duration(seconds: 2),
                        ),
                      );
                    }
                  : null,
              child: Row(
                children: [
                  Container(
                    width: 38,
                    height: 38,
                    decoration: BoxDecoration(
                      color: AppColors.accent.withValues(alpha: 0.08),
                      border: Border.all(
                        color: AppColors.accent.withValues(alpha: 0.3),
                      ),
                      borderRadius: BorderRadius.circular(AppRadius.sm),
                    ),
                    child: const Icon(
                      Icons.account_balance,
                      size: 18,
                      color: AppColors.accent,
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          bank.isNotEmpty ? bank : r.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.bodySmall?.copyWith(
                            fontWeight: FontWeight.w700,
                            color: AppColors.darkText,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          number,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontFamily: 'monospace',
                            letterSpacing: 0.6,
                            fontWeight: FontWeight.w700,
                            fontSize: 11,
                            color: AppColors.warning,
                          ),
                        ),
                        if (holder.isNotEmpty) ...[
                          const SizedBox(height: 2),
                          Text(
                            'A/N $holder',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style:
                                theme.textTheme.bodySmall?.copyWith(
                                  color: const Color(0xFF8B8B9E),
                                  fontStyle: FontStyle.italic,
                                ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  if (number.isNotEmpty)
                    Icon(
                      Icons.copy_rounded,
                      size: 16,
                      color: AppColors.accent,
                    ),
                ],
              ),
            ),
          );
        }),
      ],
    );
  }

  Widget _buildUltraCompactList(BuildContext context, ThemeData theme, List<PublicRekening> list) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        const Text(
          'Rekening RT:',
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w600,
            color: Color(0xFF8B8B9E),
          ),
        ),
        const SizedBox(height: 4),
        ...list.map((r) {
          final bank = (r.bankName ?? '').trim();
          final number = (r.accountNumber ?? '').trim();
          final holder = (r.accountHolder ?? '').trim();
          final info = [bank, number].where((s) => s.isNotEmpty).join('  ');
          final displayInfo = info.isEmpty ? r.name : info;
          return Padding(
            padding: const EdgeInsets.only(bottom: 2),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Flexible(
                  child: Text(
                    displayInfo,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 10,
                      fontFamily: 'monospace',
                      color: Color(0xFFA970FF),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                if (holder.isNotEmpty) ...[
                  const SizedBox(width: 4),
                  Text(
                    'A/N $holder',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 9, color: Color(0xFF8B8B9E)),
                  ),
                ],
                if (number.isNotEmpty) ...[
                  const SizedBox(width: 4),
                  GestureDetector(
                    onTap: () {
                      Clipboard.setData(ClipboardData(text: number));
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text('Nomor rekening $number disalin'),
                          behavior: SnackBarBehavior.floating,
                          duration: const Duration(seconds: 2),
                        ),
                      );
                    },
                    child: const Icon(
                      Icons.copy,
                      size: 14,
                      color: Color(0xFF8B8B9E),
                    ),
                  ),
                ],
              ],
            ),
          );
        }),
      ],
    );
  }
}
