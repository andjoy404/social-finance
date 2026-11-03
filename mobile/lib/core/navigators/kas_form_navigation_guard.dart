import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

/// One navigation guard shared by form back/pop and shell navigation.
class KasFormNavigationGuard {
  KasFormNavigationGuard._();

  static Future<bool>? _activeDialog;
  static final Set<String> _approvedExitPaths = <String>{};

  /// Returns true when [path] corresponds to one of the guarded form pages.
  /// Uses prefix matching so routes with dynamic segments (e.g.
  /// `/home/iuran/pembayaran/:billId`) are also covered.
  static bool isFormPath(String path) => _guardedPrefixes.any((prefix) {
        // Exact match or path starts with prefix + `/`
        return path == prefix || path.startsWith('$prefix/');
      });

  static const Set<String> _guardedPrefixes = {
    '/home/kas/masuk',
    '/home/kas/keluar',
    '/home/iuran/form',
    '/home/iuran/pembayaran',
    '/home/warga/baru',
    '/home/warga/baru/petugas',
    '/home/warga/edit/household',
    '/home/warga/edit/petugas',
  };

  static bool isKasPath(String path) =>
      path == '/home/kas' || path.startsWith('/home/kas/');

  /// Allows one route exit after an explicit confirmation or successful save.
  static void allowExitFor(String path) {
    if (isFormPath(path)) _approvedExitPaths.add(path);
  }

  /// Removes a single exit allowance.
  static void revokeExitFor(String path) {
    _approvedExitPaths.remove(path);
  }

  /// Resets static guard state for clean test isolation.
  @visibleForTesting
  static void resetForTesting() {
    _activeDialog = null;
    _approvedExitPaths.clear();
  }

  /// Guard GoRouter exits, covering pop, system back, redirects, and go calls.
  ///
  /// The only bypass is `_approvedExitPaths` which is populated by
  /// [allowExitFor] after explicit user confirmation (shell nav) or
  /// successful save (form pop).  No section-based bypass exists.
  static Future<bool> onRouteExit(
    BuildContext context,
    GoRouterState state,
  ) async {
    final source = state.uri.path;
    if (!isFormPath(source)) return true;
    if (_approvedExitPaths.remove(source)) return true;
    return confirm(context);
  }

  /// Show the shared confirmation before shell controls initiate navigation.
  /// Always asks for confirmation when leaving a guarded form; the caller is
  /// responsible for registering [allowExitFor] after explicit approval so
  /// that the synchronous [GoRouter] navigation does not trigger a second dialog.
  static Future<bool> confirmShellNavigation(
    BuildContext context, {
    required String source,
    required String destination,
  }) async {
    if (!isFormPath(source)) return true;
    return confirm(context);
  }

  /// Cancels the current form by popping after confirmation.
  ///
  /// Used by the form "Batal" button and PopScope; shows the confirmation
  /// dialog and navigates away only if the user confirms.
  ///
  /// Flow:
  ///  1. User chooses "Tidak" → form stays open, input preserved.
  ///  2. User chooses "Ya" → register one-time bypass via [allowExitFor],
  ///     then call [context.pop] (or [Navigator.maybePop] if canPop is false).
  ///     If pop fails, revoke the bypass so [onRouteExit] is not stuck.
  static Future<void> requestFormCancel(BuildContext context) async {
    if (!context.mounted) return;
    final state = GoRouterState.of(context);
    final currentPath = state.uri.path;
    // If already approved (e.g. save succeeded), skip dialog and navigate immediately.
    if (_approvedExitPaths.contains(currentPath)) {
      try {
        context.pop();
      } catch (_) {
        rethrow;
      }
      return;
    }
    final shouldCancel = await confirm(context);
    if (!context.mounted) return;
    if (!shouldCancel) return; // "Tidak" — form stays, no bypass registered.
    // Explicitly approved — register one-time exit bypass BEFORE navigating.
    allowExitFor(currentPath);
    try {
      context.pop();
    } catch (_) {
      // Pop failed (e.g. navigator stack underflow) — revoke bypass.
      revokeExitFor(currentPath);
      rethrow;
    }
  }

  static Future<bool> confirm(BuildContext context) async {
    if (!context.mounted) return false;
    final active = _activeDialog;
    if (active != null) return active;

    final dialogResult = showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => PopScope(
        canPop: false,
        child: AlertDialog(
          title: const Text('Apakah Anda yakin ingin membatalkan?'),
          content: const Text('Perubahan yang belum disimpan akan hilang.'),
          actions: [
            Align(
              alignment: Alignment.center,
              child: SizedBox(
                height: 48,
                child: ElevatedButton(
                  key: const Key('kas-cancel-stay'),
                  onPressed: () => Navigator.of(dialogContext).pop(false),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.accent,
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                  ),
                  child: const Text('Tidak, Tetap di Halaman'),
                ),
              ),
            ),
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.center,
              child: SizedBox(
                height: 48,
                child: ElevatedButton(
                  key: const Key('kas-cancel-confirm'),
                  onPressed: () => Navigator.of(dialogContext).pop(true),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.danger,
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                  ),
                  child: const Text('Ya, Batalkan'),
                ),
              ),
            ),
          ],
        ),
      ),
    ).then((result) => result ?? false);

    late final Future<bool> guardedResult;
    guardedResult = dialogResult.whenComplete(() {
      if (identical(_activeDialog, guardedResult)) _activeDialog = null;
    });
    _activeDialog = guardedResult;
    return guardedResult;
  }
}
