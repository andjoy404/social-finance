import 'package:flutter/material.dart';

/// NavigatorObserver yang men-track route changes pada form KAS Masuk/Keluar.
///
/// Ketika user membuka /home/kas/masuk atau /home/kas/keluar (via context.push
/// dari dalam shell), observer ini akan set isOnKasForm ke true. Saat route
/// di-pop atau di-remove, nilai kembali ke false.
class KasFormNavigatorObserver extends NavigatorObserver {
  final ValueNotifier<bool> isOnKasForm;

  KasFormNavigatorObserver() : isOnKasForm = ValueNotifier<bool>(false);

  /// Cek apakah route name adalah salah satu form KAS.
  bool _isKasFormRoute(String? name) {
    return name == '/home/kas/masuk' || name == '/home/kas/keluar';
  }

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (_isKasFormRoute(route.settings.name)) {
      isOnKasForm.value = true;
    }
  }

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (_isKasFormRoute(route.settings.name)) {
      isOnKasForm.value = false;
    }
  }

  @override
  void didRemove(Route<dynamic> route, Route<dynamic>? previousRoute) {
    if (_isKasFormRoute(route.settings.name)) {
      isOnKasForm.value = false;
    }
  }

  @override
  void didReplace({Route<dynamic>? newRoute, Route<dynamic>? oldRoute}) {
    if (newRoute != null && _isKasFormRoute(newRoute.settings.name)) {
      isOnKasForm.value = true;
    }
  }
}
