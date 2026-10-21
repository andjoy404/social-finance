import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/features/kas/data/jakarta_time.dart';

void main() {
  group('Jakarta timezone utility', () {
    test('toJakartaUtc preserves the selected Jakarta calendar date', () {
      final utc = toJakartaUtc('2026-10-22');
      // Convert back to Jakarta wall clock by adding 7 hours
      final jakarta = utc.add(const Duration(hours: 7));
      expect(jakarta.year, 2026);
      expect(jakarta.month, 10);
      expect(jakarta.day, 22);
    });

    test('toJakartaUtc does not shift date for first-of-month', () {
      final utc = toJakartaUtc('2026-10-01');
      final jakarta = utc.add(const Duration(hours: 7));
      expect(jakarta.year, 2026);
      expect(jakarta.month, 10);
      expect(jakarta.day, 1);
    });

    test('toJakartaUtc does not shift date for last-of-month', () {
      final utc = toJakartaUtc('2026-01-31');
      final jakarta = utc.add(const Duration(hours: 7));
      expect(jakarta.year, 2026);
      expect(jakarta.month, 1);
      expect(jakarta.day, 31);
    });

    test('toJakartaUtc produces correctly ordered UTC instants for different dates', () {
      final a = toJakartaUtc('2026-10-01');
      final b = toJakartaUtc('2026-10-02');
      // Same Jakarta wall-clock time (approximately), later date = later instant
      expect(b.isAfter(a), isTrue);
    });

    test('toJakartaUtc earlier date produces earlier instant even with later wall-clock time', () {
      // Even if "now" advances between calls, a much earlier date should still be earlier
      final a = toJakartaUtc('2026-01-01');
      final b = toJakartaUtc('2026-12-31');
      expect(b.isAfter(a), isTrue);
    });

    test('formatJakartaIso8601 includes explicit +07:00 offset', () {
      final utc = DateTime.utc(2026, 10, 22, 3, 30, 0, 0); // 03:30 UTC = 10:30 WIB
      final result = formatJakartaIso8601(utc);
      expect(result, '2026-10-22T10:30:00.000+07:00');
    });

    test('formatJakartaIso8601 handles midnight boundary', () {
      // 2026-10-22T17:00:00Z = 2026-10-23T00:00:00+07:00
      final utc = DateTime.utc(2026, 10, 22, 17, 0, 0, 0);
      final result = formatJakartaIso8601(utc);
      expect(result, '2026-10-23T00:00:00.000+07:00');
    });

    test('toJakartaUtc and formatJakartaIso8601 are consistent', () {
      final utc = toJakartaUtc('2026-10-22');
      final formatted = formatJakartaIso8601(utc);
      // The formatted Jakarta wall clock should have the selected date
      expect(formatted.startsWith('2026-10-22T'), isTrue);
      expect(formatted.endsWith('+07:00'), isTrue);
    });
  });
}
