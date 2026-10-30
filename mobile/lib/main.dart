import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:firebase_core/firebase_core.dart';

import 'app/app.dart';
import 'core/api/client.dart';
import 'core/api/device_id_service.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  try {
    await Firebase.initializeApp();
    if (kDebugMode) {
      debugPrint('Firebase initialized successfully');
    }
  } catch (e) {
    if (kDebugMode) {
      debugPrint('Firebase initialization warning: $e');
    }
  }

  // Ensure device ID is generated and set before app starts.
  final container = ProviderContainer();
  final deviceIdService = container.read(deviceIdServiceProvider);
  try {
    final deviceId = await deviceIdService.getDeviceId();
    container.read(apiClientProvider).setDeviceID(deviceId);
  } catch (e) {
    if (kDebugMode) {
      debugPrint('Warning: failed to set device ID: $e');
    }
  }
  container.dispose();

  runApp(const ProviderScope(child: App()));
}
