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

  group('NotificationTriggerCategory mapping & tokens', () {
    const bySystem = [
      'iuran_bill_created',
      'iuran_payment_submitted',
      'iuran_payment_verified',
      'activity_created',
      'activity_updated',
      'activity_cancelled',
      'activity_completed',
    ];
    const terjadwal = [
      'iuran_payment_reminder',
      'iuran_upcoming_due_3_months',
      'iuran_overdue_3_months',
      'activity_reminder',
    ];
    const manualTrigger = [
      'waste_truck_arrival',
      'manual_push',
      'rw_cash_in',
      'rw_cash_out',
    ];

    test('every registered event maps to exactly one category', () {
      final all = [...bySystem, ...terjadwal, ...manualTrigger];
      expect(all.toSet().length, equals(all.length), reason: 'no event key may belong to two categories');
      for (final ev in all) {
        final cat = getNotificationTriggerCategory(ev);
        expect(
          [NotificationTriggerCategory.bySystem, NotificationTriggerCategory.terjadwal, NotificationTriggerCategory.manualTrigger],
          contains(cat),
          reason: 'event $ev must map to a known trigger category',
        );
      }
    });

    test('classification matches the actual dispatch mechanism', () {
      for (final ev in bySystem) {
        expect(getNotificationTriggerCategory(ev), NotificationTriggerCategory.bySystem, reason: '$ev is dispatched automatically on a business event');
        expect(isFixedSystemEvent(ev), isTrue, reason: '$ev must be protected from custom template/schedule actions');
      }
      for (final ev in terjadwal) {
        expect(getNotificationTriggerCategory(ev), NotificationTriggerCategory.terjadwal, reason: '$ev is dispatched by the scheduler');
        expect(isFixedSystemEvent(ev), isFalse);
      }
      for (final ev in manualTrigger) {
        expect(getNotificationTriggerCategory(ev), NotificationTriggerCategory.manualTrigger, reason: '$ev requires an explicit officer/user action');
        expect(isFixedSystemEvent(ev), isFalse);
      }
    });

    test('API-provided trigger_category takes precedence over the local classifier', () {
      expect(
        getNotificationTriggerCategory('iuran_bill_created', apiTriggerCategory: 'terjadwal'),
        NotificationTriggerCategory.terjadwal,
      );
      expect(
        getNotificationTriggerCategory('waste_truck_arrival', apiTriggerCategory: 'by_system'),
        NotificationTriggerCategory.bySystem,
      );
      expect(
        getNotificationTriggerCategory('activity_reminder', apiTriggerCategory: 'manual_trigger'),
        NotificationTriggerCategory.manualTrigger,
      );
      expect(
        getNotificationTriggerCategory('unknown_event', apiTriggerCategory: 'by_system'),
        NotificationTriggerCategory.bySystem,
      );
      expect(
        getNotificationTriggerCategory('unknown_event', apiTriggerCategory: 'not-a-category'),
        NotificationTriggerCategory.terjadwal,
      );
    });

    test('labels and API values are stable and lowercase', () {
      expect(getNotificationTriggerCategoryLabel(NotificationTriggerCategory.bySystem), 'By System');
      expect(getNotificationTriggerCategoryLabel(NotificationTriggerCategory.terjadwal), 'Terjadwal');
      expect(getNotificationTriggerCategoryLabel(NotificationTriggerCategory.manualTrigger), 'Manual Trigger');
      expect(getNotificationTriggerCategoryApiValue(NotificationTriggerCategory.bySystem), 'by_system');
      expect(getNotificationTriggerCategoryApiValue(NotificationTriggerCategory.terjadwal), 'terjadwal');
      expect(getNotificationTriggerCategoryApiValue(NotificationTriggerCategory.manualTrigger), 'manual_trigger');
    });

    test('by_system events are placed at the very bottom preserving the relative order of the rest', () {
      const input = [
        ('iuran_bill_created', 'by'),
        ('iuran_payment_reminder', 't'),
        ('activity_updated', 'by'),
        ('waste_truck_arrival', 'm'),
        ('activity_completed', 'by'),
        ('activity_reminder', 't'),
      ];
      final sorted = sortSystemTypesLastBy(
        input,
        (e) => e.$1,
      );
      expect(
        sorted.map((e) => e.$1).toList(),
        equals([
          'iuran_payment_reminder',
          'waste_truck_arrival',
          'activity_reminder',
          'iuran_bill_created',
          'activity_updated',
          'activity_completed',
        ]),
      );
    });

    test('colors map to blue/amber/violet consistent with the Web variants', () {
      expect(getNotificationTriggerCategoryColor(NotificationTriggerCategory.bySystem), AppColors.info);
      expect(getNotificationTriggerCategoryColor(NotificationTriggerCategory.terjadwal), AppColors.warning);
      expect(getNotificationTriggerCategoryColor(NotificationTriggerCategory.manualTrigger), AppColors.accent);
    });
  });
}

