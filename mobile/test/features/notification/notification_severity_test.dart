import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/notification/data/notification_severity.dart';

void main() {
  group('NotificationSeverity mapping & tokens', () {
    test('Scope 3: permanent system events are detected accurately', () {
      expect(isFixedSystemEvent('iuran_bill_created'), isTrue);
      expect(isFixedSystemEvent('iuran_payment_submitted'), isTrue);
      expect(isFixedSystemEvent('iuran_payment_verified'), isTrue);

      expect(isFixedSystemEvent('iuran_payment_reminder'), isFalse);
      expect(isFixedSystemEvent('waste_truck_arrival'), isFalse);
      expect(isFixedSystemEvent('manual_push'), isFalse);
    });

    test('Scope 4: severity levels map to standard categories', () {
      expect(
        getNotificationSeverity('payment_rejected'),
        equals(NotificationSeverity.urgent),
      );
      expect(
        getNotificationSeverity('iuran_overdue_3_months'),
        equals(NotificationSeverity.urgent),
      );

      expect(
        getNotificationSeverity('iuran_payment_reminder'),
        equals(NotificationSeverity.warning),
      );
      expect(
        getNotificationSeverity('iuran_upcoming_due_3_months'),
        equals(NotificationSeverity.warning),
      );
      expect(
        getNotificationSeverity('rw_cash_out'),
        equals(NotificationSeverity.warning),
      );

      expect(
        getNotificationSeverity('payment_verified'),
        equals(NotificationSeverity.normal),
      );
      expect(
        getNotificationSeverity('iuran_payment_verified'),
        equals(NotificationSeverity.normal),
      );
      expect(
        getNotificationSeverity('rw_cash_in'),
        equals(NotificationSeverity.normal),
      );

      expect(
        getNotificationSeverity('manual_push'),
        equals(NotificationSeverity.informational),
      );
      expect(
        getNotificationSeverity('waste_truck_arrival'),
        equals(NotificationSeverity.informational),
      );
    });

    test('Severity colors map to standard AppColors tokens', () {
      expect(getNotificationSeverityColor('payment_rejected'), equals(AppColors.danger));
      expect(getNotificationSeverityColor('iuran_payment_reminder'), equals(AppColors.warning));
      expect(getNotificationSeverityColor('payment_verified'), equals(AppColors.success));
      expect(getNotificationSeverityColor('manual_push'), equals(AppColors.info));
    });
  });
}

