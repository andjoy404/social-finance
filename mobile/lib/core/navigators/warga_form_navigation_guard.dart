import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'form_navigation_guard.dart';

/// Backward-compatible wrapper delegating to [FormNavigationGuard.onRouteExit].
Future<bool> wargaFormRouteExit(BuildContext context, GoRouterState state) {
  return FormNavigationGuard.onRouteExit(context, state);
}
