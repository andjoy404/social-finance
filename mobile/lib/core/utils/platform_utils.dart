import 'package:flutter/foundation.dart';

/// Returns the platform identifier string used for backend device registration.
///
/// Matches backend migration 022 CHECK constraint:
///   `platform TEXT NOT NULL CHECK (platform IN ('android', 'ios', 'web'))`
///
/// Returns:
/// - 'android' on Android devices
/// - 'ios' on iOS devices
/// - 'web' when running on web platform
///
/// This function is platform-safe and does not throw on any platform.
String getPlatformIdentifier() {
  if (kIsWeb) return 'web';
  switch (defaultTargetPlatform) {
    case TargetPlatform.android:
      return 'android';
    case TargetPlatform.iOS:
      return 'ios';
    case TargetPlatform.macOS:
    case TargetPlatform.windows:
    case TargetPlatform.linux:
    case TargetPlatform.fuchsia:
      return 'web';
  }
}
