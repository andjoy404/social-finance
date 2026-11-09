import 'package:flutter/material.dart';
import '../../../../core/theme/app_colors.dart';

/// Permanent system events that cannot be created as custom templates or scheduled.
const List<String> kFixedSystemEvents = [
  'iuran_bill_created',
  'iuran_payment_submitted',
  'iuran_payment_verified',
];

/// Checks whether an event type is a fixed system notification.
bool isFixedSystemEvent(String eventType) {
  return kFixedSystemEvents.contains(eventType);
}

/// Urgency / Severity levels for notifications.
enum NotificationSeverity {
  informational,
  normal,
  warning,
  urgent,
}

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

