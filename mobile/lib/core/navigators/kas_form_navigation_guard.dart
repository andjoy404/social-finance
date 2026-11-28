import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/navigators/form_navigation_guard.dart';

/// Backward-compatible alias delegating all operations to [FormNavigationGuard].
///
/// Retained temporarily so existing imports and tests keep working while all
/// forms standardize on [FormNavigationGuard].
class KasFormNavigationGuard {
  KasFormNavigationGuard._();

  static bool isFormPath(String path) => FormNavigationGuard.isFormPath(path);

  static bool isKasPath(String path) => FormNavigationGuard.isKasPath(path);

  static void allowExitFor(String path) =>
      FormNavigationGuard.allowExitFor(path);

  static void revokeExitFor(String path) =>
      FormNavigationGuard.revokeExitFor(path);

  @visibleForTesting
  static void resetForTesting() =>
      // ignore: invalid_use_of_visible_for_testing_member
      FormNavigationGuard.resetForTesting();

  static Future<bool> onRouteExit(
    BuildContext context,
    GoRouterState state,
  ) =>
      FormNavigationGuard.onRouteExit(context, state);

  static Future<bool> confirmShellNavigation(
    BuildContext context, {
    required String source,
    required String destination,
  }) =>
      FormNavigationGuard.confirmShellNavigation(
        context,
        source: source,
        destination: destination,
      );

  static Future<void> requestFormCancel(BuildContext context) =>
      FormNavigationGuard.requestFormCancel(context);

  static Future<bool> confirm(BuildContext context) =>
      FormNavigationGuard.confirm(context);
}
