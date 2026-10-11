import 'dart:typed_data';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

/// Provider caching the image bytes of a payment proof.
final paymentProofBytesProvider =
    FutureProvider.family.autoDispose<Uint8List, String>((ref, paymentId) {
  final repository = ref.read(iuranRepositoryProvider);
  return repository.getPaymentProofBytes(paymentId);
});

/// Reusable widget for displaying payment proof thumbnails and expanding them.
///
/// Images are fetched via authenticated Dio calls and rendered from memory bytes,
/// ensuring tokens are NEVER exposed in URL query parameters and tenant boundaries
/// are properly enforced.
class PaymentProofThumbnail extends ConsumerWidget {
  final String paymentId;
  final double size;
  final String? label;

  const PaymentProofThumbnail({
    super.key,
    required this.paymentId,
    this.size = 56.0,
    this.label,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final proofAsync = ref.watch(paymentProofBytesProvider(paymentId));
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return proofAsync.when(
      data: (bytes) {
        return InkWell(
          onTap: () => _showFullImageDialog(context, bytes, label),
          borderRadius: BorderRadius.circular(AppRadius.base),
          child: Container(
            width: size,
            height: size,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppRadius.base),
              border: Border.all(
                color: AppColors.accent.withValues(alpha: 0.3),
                width: 1.5,
              ),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.05),
                  blurRadius: 4,
                  offset: const Offset(0, 2),
                ),
              ],
            ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(AppRadius.base - 1.5),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  Image.memory(
                    bytes,
                    fit: BoxFit.cover,
                  ),
                  Positioned(
                    bottom: 2,
                    right: 2,
                    child: Container(
                      padding: const EdgeInsets.all(2),
                      decoration: BoxDecoration(
                        color: Colors.black.withValues(alpha: 0.6),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: const Icon(
                        Icons.fullscreen,
                        size: 14,
                        color: Colors.white,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
      loading: () => SizedBox(
        width: size,
        height: size,
        child: Center(
          child: SizedBox(
            width: size * 0.4,
            height: size * 0.4,
            child: const CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      ),
      error: (e, stack) => Container(
        width: size,
        height: size,
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkSurface : AppColors.lightSurfaceSubtle,
          borderRadius: BorderRadius.circular(AppRadius.base),
          border: Border.all(
            color: isDark ? AppColors.darkBorder : AppColors.lightBorder,
          ),
        ),
        child: const Center(
          child: Icon(
            Icons.broken_image_outlined,
            size: 20,
            color: AppColors.lightTextMuted,
          ),
        ),
      ),
    );
  }

  void _showFullImageDialog(
    BuildContext context,
    Uint8List bytes,
    String? title,
  ) {
    showDialog(
      context: context,
      builder: (dialogCtx) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.all(AppSpacing.md),
        child: Stack(
          alignment: Alignment.topRight,
          children: [
            Container(
              decoration: BoxDecoration(
                color: Theme.of(dialogCtx).colorScheme.surface,
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Padding(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          title ?? 'Bukti Pembayaran',
                          style: Theme.of(dialogCtx)
                              .textTheme
                              .titleMedium
                              ?.copyWith(fontWeight: FontWeight.w600),
                        ),
                        IconButton(
                          icon: const Icon(Icons.close),
                          onPressed: () => Navigator.of(dialogCtx).pop(),
                        ),
                      ],
                    ),
                  ),
                  Flexible(
                    child: InteractiveViewer(
                      panEnabled: true,
                      minScale: 0.5,
                      maxScale: 4.0,
                      child: Image.memory(
                        bytes,
                        fit: BoxFit.contain,
                      ),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.md),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
