import 'package:flutter/material.dart';
import '../../../../core/theme/app_colors.dart';

/// Permanent system events that cannot be created as custom templates or scheduled.
///
/// Kept for backwards compatibility: all `by_system` trigger-category events
/// are permanent system events. Use [isBySystemTriggerEvent] for new code.
const List<String> kFixedSystemEvents = [
  'iuran_bill_created',
  'iuran_payment_submitted',
  'iuran_payment_verified',
  'activity_created',
  'activity_updated',
  'activity_cancelled',
  'activity_completed',
];

/// Checks whether an event type is a fixed system notification
/// (any `by_system` trigger-category event).
bool isFixedSystemEvent(String eventType) {
  return isBySystemTriggerEvent(eventType);
}

/// Urgency / Severity levels for notifications.
enum NotificationSeverity { informational, normal, warning, urgent }

/// Resolves the severity level of a notification based on its event type.
NotificationSeverity getNotificationSeverity(String eventType) {
  switch (eventType) {
    case 'payment_rejected':
    case 'iuran_overdue_3_months':
      return NotificationSeverity.urgent;
    case 'iuran_payment_reminder':
    case 'iuran_upcoming_due_3_months':
    case 'iuran_payment_submitted':
    case 'rw_cash_out':
      return NotificationSeverity.warning;
    case 'payment_verified':
    case 'iuran_payment_verified':
    case 'rw_cash_in':
      return NotificationSeverity.normal;
    case 'iuran_bill_created':
    case 'manual_push':
    case 'waste_truck_arrival':
    default:
      return NotificationSeverity.informational;
  }
}

/// Resolves the primary foreground color for notification severity.
Color getNotificationSeverityColor(String eventType) {
  final severity = getNotificationSeverity(eventType);
  switch (severity) {
    case NotificationSeverity.urgent:
      return AppColors.danger;
    case NotificationSeverity.warning:
      return AppColors.warning;
    case NotificationSeverity.normal:
      return AppColors.success;
    case NotificationSeverity.informational:
      return AppColors.info;
  }
}

/// Resolves the badge background color with appropriate opacity.
Color getNotificationSeverityBgColor(String eventType, bool isDark) {
  final color = getNotificationSeverityColor(eventType);
  return color.withValues(alpha: isDark ? 0.2 : 0.12);
}

// --- Trigger categories (dispatch mechanism) ---
enum NotificationTriggerCategory { bySystem, terjadwal, manualTrigger }

const List<String> kBySystemTriggerEvents = [
  'iuran_bill_created',
  'iuran_payment_submitted',
  'iuran_payment_verified',
  'activity_created',
  'activity_updated',
  'activity_cancelled',
  'activity_completed',
];

const List<String> kTerjadwalTriggerEvents = [
  'iuran_payment_reminder',
  'iuran_upcoming_due_3_months',
  'iuran_overdue_3_months',
  'activity_reminder',
];

const List<String> kManualTriggerEvents = [
  'waste_truck_arrival',
  'manual_push',
  'rw_cash_in',
  'rw_cash_out',
];

/// Classifies the dispatch mechanism of an event (automatic handler /
/// scheduler / explicit officer action). Prefers the trigger_category
/// emitted by the backend when available (passed as `apiTriggerCategory`).
NotificationTriggerCategory getNotificationTriggerCategory(
  String eventType, {
  String? apiTriggerCategory,
}) {
  final api = (apiTriggerCategory ?? '').trim().toLowerCase();
  switch (api) {
    case 'by_system':
      return NotificationTriggerCategory.bySystem;
    case 'terjadwal':
      return NotificationTriggerCategory.terjadwal;
    case 'manual_trigger':
      return NotificationTriggerCategory.manualTrigger;
    default:
      break;
  }
  if (kBySystemTriggerEvents.contains(eventType)) {
    return NotificationTriggerCategory.bySystem;
  }
  if (kTerjadwalTriggerEvents.contains(eventType)) {
    return NotificationTriggerCategory.terjadwal;
  }
  if (kManualTriggerEvents.contains(eventType)) {
    return NotificationTriggerCategory.manualTrigger;
  }
  // Unknown event: fall back to terjadwal (never classify unknowns as
  // by_system). Known by_system events are all listed in
  // kBySystemTriggerEvents.
  return NotificationTriggerCategory.terjadwal;
}

String getNotificationTriggerCategoryLabel(
  NotificationTriggerCategory category,
) {
  switch (category) {
    case NotificationTriggerCategory.bySystem:
      return 'By System';
    case NotificationTriggerCategory.terjadwal:
      return 'Terjadwal';
    case NotificationTriggerCategory.manualTrigger:
      return 'Manual Trigger';
  }
}

String getNotificationTriggerCategoryApiValue(
  NotificationTriggerCategory category,
) {
  switch (category) {
    case NotificationTriggerCategory.bySystem:
      return 'by_system';
    case NotificationTriggerCategory.terjadwal:
      return 'terjadwal';
    case NotificationTriggerCategory.manualTrigger:
      return 'manual_trigger';
  }
}

/// Badge colors consistent with the Web implementation:
/// by_system -> blue (AppColors.info), terjadwal -> amber (AppColors.warning),
/// manual_trigger -> violet (AppColors.accent).
Color getNotificationTriggerCategoryColor(
  NotificationTriggerCategory category,
) {
  switch (category) {
    case NotificationTriggerCategory.bySystem:
      return AppColors.info;
    case NotificationTriggerCategory.terjadwal:
      return AppColors.warning;
    case NotificationTriggerCategory.manualTrigger:
      return AppColors.accent;
  }
}

Color getNotificationTriggerCategoryBgColor(
  NotificationTriggerCategory category,
  bool isDark,
) {
  return getNotificationTriggerCategoryColor(
    category,
  ).withValues(alpha: isDark ? 0.2 : 0.12);
}

bool isBySystemTriggerEvent(String eventType, {String? apiTriggerCategory}) {
  return getNotificationTriggerCategory(
        eventType,
        apiTriggerCategory: apiTriggerCategory,
      ) ==
      NotificationTriggerCategory.bySystem;
}

/// Place By-System events at the very bottom while preserving the relative
/// order of the rest (previous ordering requirement for the system list).
List<T> sortSystemTypesLastBy<T>(
  List<T> types,
  String Function(T) eventTypeOf, {
  String? Function(T)? triggerCategoryOf,
}) {
  final rest = <T>[];
  final bySystem = <T>[];
  for (final item in types) {
    final category = getNotificationTriggerCategory(
      eventTypeOf(item),
      apiTriggerCategory: triggerCategoryOf?.call(item),
    );
    if (category == NotificationTriggerCategory.bySystem) {
      bySystem.add(item);
    } else {
      rest.add(item);
    }
  }
  return [...rest, ...bySystem];
}
