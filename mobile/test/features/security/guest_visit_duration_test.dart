import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/security/presentation/utils/guest_visit_duration.dart';

void main() {
  group('guest visit duration', () {
    test('active guest: duration from check-in to now', () {
      final checkedIn = DateTime(2026, 11, 11, 10, 0);
      final now = DateTime(2026, 11, 11, 11, 25);
      expect(
        guestVisitDurationLabel(checkedIn, null, now: now),
        'Sedang berkunjung · 1 jam 25 menit',
      );
    });

    test('checked out guest: final duration checkout minus check-in', () {
      final checkedIn = DateTime(2026, 11, 11, 9, 0);
      final checkedOut = DateTime(2026, 11, 11, 11, 10);
      final laterNow = DateTime(2026, 11, 11, 15, 0);
      final label = guestVisitDurationLabel(checkedIn, checkedOut, now: laterNow);
      expect(label, 'Durasi kunjungan · 2 jam 10 menit');
      // final value does not change when time passes
      expect(
        guestVisitDurationLabel(
          checkedIn,
          checkedOut,
          now: DateTime(2026, 11, 11, 23, 59),
        ),
        label,
      );
    });

    test('checkout earlier than check-in is handled safely', () {
      final checkedIn = DateTime(2026, 11, 11, 10, 0);
      final checkedOut = DateTime(2026, 11, 11, 9, 30);
      expect(
        formatGuestVisitDuration(checkedIn, checkedOut),
        '0 menit',
      );
    });

    test('sub-minute active duration is safe', () {
      final checkedIn = DateTime(2026, 11, 11, 10, 0, 0);
      final now = DateTime(2026, 11, 11, 10, 0, 20);
      expect(
        formatGuestVisitDuration(checkedIn, null, now: now),
        '0 menit',
      );
    });
  });
}
