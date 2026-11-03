import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'form_navigation_guard.dart';

/// Wrapper to satisfy GoRouter.onExit which expects [FutureOr<bool>].
Future<bool> wargaFormRouteExit(GoRouterState state) {
  return FormNavigationGuard.onRouteExit(state);
}
