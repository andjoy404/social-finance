import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Generic form navigation guard for pages with unsaved changes.
///
/// Usage:
///   1. Add `onExit` to the `GoRoute` definition.
///   2. Wrap the page body with `PopScope` and delegate to `requestFormCancel`.
///   3. After a successful save, call `allowExitFor(routePath)` before navigating away.
class FormNavigationGuard {
  FormNavigationGuard._();

  static final Set<String> _approvedExitPaths = {};
  static final Map<String, bool Function(BuildContext)> _routeExitHandlers = {};
  static Future<bool?>? _activeDialog;

  /// Register a handler for a specific route path. The handler receives the
  /// current BuildContext and returns true if navigation should proceed,
  /// false otherwise.
  static void register(String path, bool Function(BuildContext) handler) {
    _routeExitHandlers[path] = handler;
  }

  /// Remove the handler for a specific route path. Call this from the
  /// [State] object's [State.dispose] method.
  static void unregister(String path) {
    _routeExitHandlers.remove(path);
  }

  /// Callback for [GoRouter.onExit].
  ///
  /// When the user triggers navigation away from a guarded route (back button,
  /// drawer, bottom nav, system back), this checks if the target path has been
  /// approved (e.g. after a successful save) or if a route-specific handler
  /// should be consulted.
  static Future<bool> onRouteExit(GoRouterState state, {BuildContext? context}) {
    final currentPath = state.matchedLocation;
    if (_approvedExitPaths.contains(currentPath)) {
      _approvedExitPaths.remove(currentPath);
      return Future.value(true);
    }
    final handler = _routeExitHandlers[currentPath];
    if (handler != null && context != null) {
      return Future.value(handler(context));
    }
    // No handler registered — allow exit by default
    return Future.value(true);
  }

  /// Called from a [PopScope.onPopInvokedWithResult] callback to request a
  /// "leave without saving" confirmation.
  ///
  /// Returns [true] if the user confirmed navigation, [false] if they cancelled.
  static Future<bool> requestFormCancel(BuildContext context) async {
    if (_activeDialog != null) {
      final result = await _activeDialog!;
      return result == true;
    }
    _activeDialog = showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Apakah Anda yakin ingin membatalkan?'),
        content: const Text('Perubahan yang belum disimpan akan hilang.'),
        actions: [
          Align(
            alignment: Alignment.center,
            child: SizedBox(
              height: 48,
              child: ElevatedButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF8B5CF6),
                  foregroundColor: Colors.white,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
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
                onPressed: () => Navigator.of(dialogContext).pop(true),
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFEF4444),
                  foregroundColor: Colors.white,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                child: const Text('Ya, Batalkan'),
              ),
            ),
          ),
        ],
      ),
    );
    try {
      final result = await _activeDialog!;
      return result == true;
    } finally {
      _activeDialog = null;
    }
  }

  /// Temporarily allow exit from the given route path.
  ///
  /// Call this **before** navigating away after a successful save, so that
  /// the confirmation dialog is bypassed.
  static void allowExitFor(String routePath) {
    _approvedExitPaths.add(routePath);
  }

  /// Remove the current page from the approved exit set (e.g. if save failed
  /// or navigation was cancelled).
  static void revokeExitFor() {
    _approvedExitPaths.clear();
  }
}
