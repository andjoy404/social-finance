import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';

/// Registration record for an active form route's dirty and saving state.
class FormGuardRegistration {
  final bool Function() isDirty;
  final bool Function() isSaving;

  const FormGuardRegistration({
    required this.isDirty,
    required this.isSaving,
  });
}

/// Snapshot comparator for accurate form dirty-state detection.
///
/// Compares the current values of all form inputs (text, dropdown, checkbox,
/// switch, date/time, files/images, nested configuration maps/lists) against
/// the captured initial snapshot. Returning any field to its initial value
/// brings the form back to the clean state.
class FormSnapshot {
  FormSnapshot(Map<String, Object?> initialValues)
      : _initial = _normalizeMap(initialValues);

  Map<String, Object?> _initial;

  /// Replaces the baseline snapshot (e.g. after async edit data finishes
  /// loading or after a form resets).
  void reset(Map<String, Object?> values) {
    _initial = _normalizeMap(values);
  }

  /// Returns `true` when [currentValues] differ from the baseline snapshot.
  bool isDirty(Map<String, Object?> currentValues) {
    return !_deepEquals(_initial, _normalizeMap(currentValues));
  }

  static Map<String, Object?> _normalizeMap(Map<String, Object?> input) {
    return input.map((key, value) => MapEntry(key, _normalizeValue(value)));
  }

  static Object? _normalizeValue(Object? value) {
    if (value == null || value is num || value is bool || value is String) {
      return value;
    }
    if (value is DateTime) {
      return value.millisecondsSinceEpoch;
    }
    if (value is Uint8List) {
      return _BytesFingerprint.fromBytes(value);
    }
    if (value is Enum) {
      return '${value.runtimeType}.${value.name}';
    }
    if (value is List) {
      return value.map(_normalizeValue).toList(growable: false);
    }
    if (value is Set) {
      final list = value.map(_normalizeValue).toList(growable: false);
      list.sort((a, b) => a.toString().compareTo(b.toString()));
      return list;
    }
    if (value is Map) {
      final entries = value.entries.toList()
        ..sort((a, b) => a.key.toString().compareTo(b.key.toString()));
      return <String, Object?>{
        for (final entry in entries)
          entry.key.toString(): _normalizeValue(entry.value),
      };
    }
    return value;
  }

  static bool _deepEquals(Object? a, Object? b) {
    if (identical(a, b)) return true;
    if (a == null || b == null) return false;
    if (a is List && b is List) {
      if (a.length != b.length) return false;
      for (var i = 0; i < a.length; i++) {
        if (!_deepEquals(a[i], b[i])) return false;
      }
      return true;
    }
    if (a is Map && b is Map) {
      if (a.length != b.length) return false;
      for (final key in a.keys) {
        if (!b.containsKey(key)) return false;
        if (!_deepEquals(a[key], b[key])) return false;
      }
      return true;
    }
    return a == b;
  }
}

class _BytesFingerprint {
  final int length;
  final int hash;

  const _BytesFingerprint(this.length, this.hash);

  factory _BytesFingerprint.fromBytes(Uint8List bytes) {
    var h = 0x811c9dc5;
    for (var i = 0; i < bytes.length; i++) {
      h ^= bytes[i];
      h = (h * 0x01000193) & 0x7fffffff;
    }
    return _BytesFingerprint(bytes.length, h);
  }

  @override
  bool operator ==(Object other) =>
      other is _BytesFingerprint &&
      other.length == length &&
      other.hash == hash;

  @override
  int get hashCode => Object.hash(length, hash);
}

/// Centralized unsaved-changes navigation guard shared across GoRouter `onExit`,
/// `PopScope`, AppBar Back, Batal button, Shell Navigation (Bottom Nav, Drawer,
/// Menu Grid), Tab switches, and Modal/Dialog forms.
class FormNavigationGuard {
  FormNavigationGuard._();

  /// Standardized button keys (kept backward compatible with existing tests).
  static const Key stayButtonKey = Key('kas-cancel-stay');
  static const Key confirmButtonKey = Key('kas-cancel-confirm');

  static Future<bool>? _activeDialog;
  static final Set<String> _approvedExitPaths = <String>{};
  static final Map<String, FormGuardRegistration> _registrations =
      <String, FormGuardRegistration>{};

  /// Temporary compatibility prefixes for routes that have not registered a
  /// dirty-state callback yet. Every migrated form registers an explicit
  /// snapshot-based callback via [register].
  static final Set<String> _legacyFormPrefixes = <String>{
    ..._defaultLegacyFormPrefixes,
  };

  static const Set<String> _defaultLegacyFormPrefixes = <String>{
    '/home/kas/masuk',
    '/home/kas/keluar',
    '/home/iuran/form',
    '/home/iuran/pembayaran',
    '/home/warga/baru',
    '/home/warga/baru/petugas',
    '/home/warga/edit',
    '/home/warga/edit/household',
    '/home/warga/edit/petugas',
    '/home/pengumuman/create',
    '/home/kegiatan/create',
  };

  /// Normalizes a route string by stripping query strings and fragments.
  static String normalizePath(String rawPath) {
    final trimmed = rawPath.trim();
    if (trimmed.isEmpty) return trimmed;
    final parsed = Uri.tryParse(trimmed);
    if (parsed != null && parsed.path.isNotEmpty) {
      return parsed.path;
    }
    return trimmed.split('?').first.split('#').first;
  }

  /// Registers a form route with explicit snapshot-based [isDirty] and
  /// [isSaving] callbacks.
  ///
  /// Returns the registration token to pass back to [unregister] in `dispose()`
  /// so a disposing screen never clobbers a newly mounted screen on the same
  /// route.
  static FormGuardRegistration register(
    String path, {
    required bool Function() isDirty,
    bool Function()? isSaving,
  }) {
    final normalized = normalizePath(path);
    final registration = FormGuardRegistration(
      isDirty: isDirty,
      isSaving: isSaving ?? () => false,
    );
    _registrations[normalized] = registration;
    // Clear any stale one-time bypass left over for this path.
    _approvedExitPaths.remove(normalized);
    return registration;
  }

  /// Unregisters a form route callback on screen disposal.
  static void unregister(
    String path, {
    FormGuardRegistration? registration,
  }) {
    final normalized = normalizePath(path);
    if (registration == null ||
        identical(_registrations[normalized], registration)) {
      _registrations.remove(normalized);
    }
    _approvedExitPaths.remove(normalized);
  }

  /// Temporary compatibility helper to mark a path prefix as a form route
  /// without a dirty callback. Do NOT use as standard for new or migrated forms;
  /// use [register] with snapshot-based [isDirty] and [isSaving] instead.
  @Deprecated(
    'Temporary compatibility only. Use FormNavigationGuard.register with snapshot-based isDirty/isSaving.',
  )
  static void registerFormPath(String prefix) {
    final normalized = normalizePath(prefix);
    if (normalized.isNotEmpty) {
      _legacyFormPrefixes.add(normalized);
    }
  }

  static bool _matchesPrefix(String path, String prefix) {
    return path == prefix || path.startsWith('$prefix/');
  }

  static bool _matchesEditDynamicRoute(String path) {
    return (path.startsWith('/home/pengumuman/') && path.endsWith('/edit')) ||
        (path.startsWith('/home/kegiatan/') && path.endsWith('/edit'));
  }

  /// Returns true when [path] corresponds to a guarded form route (either
  /// explicitly registered via [register] or matching a legacy form prefix).
  static bool isFormPath(String path) {
    final normalized = normalizePath(path);
    if (_registrations.containsKey(normalized)) return true;
    if (_matchesEditDynamicRoute(normalized)) return true;
    return _legacyFormPrefixes.any(
      (prefix) => _matchesPrefix(normalized, prefix),
    );
  }

  /// Backward-compatible helper retained for legacy callers.
  static bool isKasPath(String path) {
    final normalized = normalizePath(path);
    return normalized == '/home/kas' || normalized.startsWith('/home/kas/');
  }

  /// Returns true if [path] is currently in a saving state.
  static bool isSaving(String path) {
    final normalized = normalizePath(path);
    final reg = _registrations[normalized];
    if (reg == null) return false;
    return reg.isSaving();
  }

  /// Returns true if any registered form is currently saving.
  static bool isAnySaving() {
    for (final reg in _registrations.values) {
      if (reg.isSaving()) return true;
    }
    return false;
  }

  /// Returns true if [path] currently has unsaved user changes.
  ///
  /// When a form has registered a snapshot callback via [register], that
  /// callback is the sole source of truth (clean state returns `false` and
  /// exits immediately without dialog). Only unregistered legacy prefixes
  /// fall back to `true` for temporary compatibility.
  static bool isDirty(String path) {
    final normalized = normalizePath(path);
    final reg = _registrations[normalized];
    if (reg != null) {
      return reg.isDirty();
    }
    return isFormPath(normalized);
  }

  /// Allows a single scoped route exit after explicit user confirmation or
  /// successful save.
  static void allowExitFor(String path) {
    final normalized = normalizePath(path);
    if (isFormPath(normalized)) {
      _approvedExitPaths.add(normalized);
    }
  }

  /// Removes a single exit allowance (e.g. if navigation failed or was aborted).
  static void revokeExitFor(String path) {
    _approvedExitPaths.remove(normalizePath(path));
  }

  /// Clears all approved exits except optionally the currently active [exceptPath].
  /// Called on route transitions so stale bypasses never linger.
  static void clearStaleApprovedExits({String? activePath}) {
    if (activePath == null) {
      _approvedExitPaths.clear();
      return;
    }
    final normalized = normalizePath(activePath);
    _approvedExitPaths.removeWhere((p) => p != normalized);
  }

  /// Returns true if a one-time bypass is currently pending for [path].
  @visibleForTesting
  static bool hasApprovedExitFor(String path) {
    return _approvedExitPaths.contains(normalizePath(path));
  }

  /// Resets all static guard state for clean test isolation.
  @visibleForTesting
  static void resetForTesting() {
    _activeDialog = null;
    _approvedExitPaths.clear();
    _registrations.clear();
    _legacyFormPrefixes
      ..clear()
      ..addAll(_defaultLegacyFormPrefixes);
  }

  static String? _resolveCurrentPath(BuildContext context) {
    try {
      final state = GoRouterState.of(context);
      return normalizePath(state.uri.path);
    } catch (_) {
      final router = GoRouter.maybeOf(context);
      if (router != null) {
        return normalizePath(router.routeInformationProvider.value.uri.path);
      }
      return null;
    }
  }

  /// Guard for `GoRoute.onExit`. Covers pop, system back, AppBar back,
  /// programmatic `go`, and redirects.
  ///
  /// Order of checks:
  /// 1. Non-form routes exit immediately.
  /// 2. Scoped one-time approved exit (save success or confirmed cancel) is
  ///    consumed once and allows exit.
  /// 3. Active saving state blocks exit immediately without dialog.
  /// 4. Clean state exits immediately without dialog.
  /// 5. Dirty state opens the single shared confirmation dialog.
  static Future<bool> onRouteExit(
    BuildContext context,
    GoRouterState state,
  ) async {
    final source = normalizePath(state.uri.path);
    if (!isFormPath(source)) return true;
    if (_approvedExitPaths.remove(source)) return true;
    if (isSaving(source)) return false;
    if (!isDirty(source)) return true;
    return confirm(context);
  }

  /// Shared check before shell controls (bottom nav, drawer, menu grid, RW
  /// switcher) initiate navigation away from [source].
  static Future<bool> confirmShellNavigation(
    BuildContext context, {
    required String source,
    required String destination,
  }) async {
    final normSource = normalizePath(source);
    final normDest = normalizePath(destination);
    if (source == destination ||
        (normSource == normDest && !destination.contains('?'))) {
      return true;
    }
    if (!isFormPath(normSource)) return true;
    if (isSaving(normSource)) return false;
    if (!isDirty(normSource)) return true;
    return confirm(context);
  }

  /// Shared check before switching tabs inside a screen containing an inline
  /// form (e.g. Patroli, Paket, Waste Truck).
  static Future<bool> confirmTabChange(
    BuildContext context, {
    required bool isDirty,
    required bool isSaving,
  }) async {
    if (isSaving) return false;
    if (!isDirty) return true;
    return confirm(context);
  }

  /// Official exit path after a form saves successfully.
  ///
  /// Registers a one-time scoped exit bypass and pops (or navigates to
  /// [fallbackRoute] when `canPop` is false) without showing the cancel dialog.
  /// Automatically revokes the bypass if navigation throws or cannot pop.
  static void exitAfterSave(
    BuildContext context, {
    String? routePath,
    String? fallbackRoute,
    Object? result,
  }) {
    if (!context.mounted) return;
    final resolvedPath = routePath != null
        ? normalizePath(routePath)
        : _resolveCurrentPath(context);
    if (resolvedPath != null && isFormPath(resolvedPath)) {
      allowExitFor(resolvedPath);
    }
    _performExit(
      context,
      currentPath: resolvedPath,
      fallbackRoute: fallbackRoute,
      result: result,
    );
  }

  /// Handles form cancellation from Batal button, AppBar Back, or `PopScope`.
  ///
  /// - If saving: blocks silently.
  /// - If already approved (e.g. save success): exits immediately.
  /// - If clean: exits immediately without dialog.
  /// - If dirty: opens the single confirmation dialog and exits only if the
  ///   user chooses `Ya, Batalkan`.
  static Future<void> requestFormCancel(
    BuildContext context, {
    String? routePath,
    bool Function()? isDirtyOverride,
    bool Function()? isSavingOverride,
    String? fallbackRoute,
    Object? result,
    VoidCallback? onPop,
  }) async {
    if (!context.mounted) return;
    final currentPath = routePath != null
        ? normalizePath(routePath)
        : _resolveCurrentPath(context);

    // Already approved (e.g. save succeeded) -> exit immediately without dialog.
    if (currentPath != null && _approvedExitPaths.contains(currentPath)) {
      _performExit(
        context,
        currentPath: currentPath,
        fallbackRoute: fallbackRoute,
        result: result,
        onPop: onPop,
      );
      return;
    }

    final saving = isSavingOverride?.call() ??
        (currentPath != null ? isSaving(currentPath) : false);
    if (saving) return;

    final dirty = isDirtyOverride?.call() ??
        (currentPath != null ? isDirty(currentPath) : true);
    if (dirty) {
      final shouldCancel = await confirm(context);
      if (!context.mounted) return;
      if (!shouldCancel) return;
    }

    if (currentPath != null && isFormPath(currentPath)) {
      allowExitFor(currentPath);
    }
    _performExit(
      context,
      currentPath: currentPath,
      fallbackRoute: fallbackRoute,
      result: result,
      onPop: onPop,
    );
  }

  static void _performExit(
    BuildContext context, {
    required String? currentPath,
    String? fallbackRoute,
    Object? result,
    VoidCallback? onPop,
  }) {
    try {
      if (onPop != null) {
        onPop();
        return;
      }
      final router = GoRouter.maybeOf(context);
      if (router != null) {
        if (router.canPop()) {
          context.pop(result);
          return;
        }
        if (fallbackRoute != null && fallbackRoute.isNotEmpty) {
          context.go(fallbackRoute);
          return;
        }
      }
      final navigator = Navigator.maybeOf(context);
      if (navigator != null && navigator.canPop()) {
        navigator.pop(result);
        return;
      }
      // Nothing popped (e.g. standalone test route at stack root) -> clear bypass.
      if (currentPath != null) {
        revokeExitFor(currentPath);
      }
    } catch (_) {
      if (currentPath != null) {
        revokeExitFor(currentPath);
      }
      rethrow;
    }
  }

  /// Guarded close helper for dialog and modal bottom sheet forms.
  ///
  /// - While [isSaving] is true: blocks dismissal and returns `false`.
  /// - While [isDirty] is false: closes immediately without dialog and returns `true`.
  /// - While [isDirty] is true: opens the single confirmation dialog and closes
  ///   only when the user chooses `Ya, Batalkan`.
  static Future<bool> requestModalClose(
    BuildContext context, {
    required bool isDirty,
    required bool isSaving,
    Object? result,
  }) async {
    if (!context.mounted) return false;
    if (isSaving) return false;
    if (isDirty) {
      final confirmed = await confirm(context);
      if (!context.mounted || !confirmed) return false;
    }
    final nav = Navigator.maybeOf(context);
    if (nav != null && nav.canPop()) {
      nav.pop(result);
      return true;
    }
    return false;
  }

  /// Displays the single standardized confirmation dialog.
  ///
  /// Concurrent calls share the same active [Future<bool>] so a double dialog
  /// can never appear.
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
                  key: stayButtonKey,
                  onPressed: () => Navigator.of(dialogContext).pop(false),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF8B5CF6),
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                  ),
                  child: const Text('Tetap di Halaman'),
                ),
              ),
            ),
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.center,
              child: SizedBox(
                height: 48,
                child: ElevatedButton(
                  key: confirmButtonKey,
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

/// Reusable `PopScope` wrapper for modal dialogs and bottom sheets that contain
/// user inputs.
///
/// Always uses `canPop: false` so text-only edits never suffer from stale
/// `canPop` evaluation, and intercepts system back, swipe/gesture back, and
/// barrier taps via [FormNavigationGuard.requestModalClose].
class GuardedFormModalScope extends StatelessWidget {
  final bool Function() isDirty;
  final bool Function() isSaving;
  final Object? popResult;
  final Widget child;

  const GuardedFormModalScope({
    super.key,
    required this.isDirty,
    required this.isSaving,
    required this.child,
    this.popResult,
  });

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        await FormNavigationGuard.requestModalClose(
          context,
          isDirty: isDirty(),
          isSaving: isSaving(),
          result: popResult,
        );
      },
      child: child,
    );
  }
}
