import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/utils/jakarta_time_utils.dart';

void main() {
  group('Jakarta Time Utils (WIB, UTC+07:00)', () {
    test('toJakartaDateTime parses UTC ISO 8601 timestamp with +7 hours offset', () {
      // 01:05:45 UTC -> 08:05:45 WIB
      final result = toJakartaDateTime('2026-11-07T01:05:45.000Z');
      expect(result, isNotNull);
      expect(result!.year, 2026);
      expect(result.month, 11);
      expect(result.day, 7);
      expect(result.hour, 8);
      expect(result.minute, 5);
      expect(result.second, 45);
    });

    test('toJakartaDateTime crosses midnight date boundary correctly', () {
      // 2026-11-06 18:30:00 UTC -> 2026-11-07 01:30:00 WIB
      final result = toJakartaDateTime('2026-11-06T18:30:00Z');
      expect(result, isNotNull);
      expect(result!.year, 2026);
      expect(result.month, 11);
      expect(result.day, 7);
      expect(result.hour, 1);
      expect(result.minute, 30);
    });

    test('toJakartaDateTime parses explicit +07:00 string without double shifting', () {
      final result = toJakartaDateTime('2026-11-07T08:05:45+07:00');
      expect(result, isNotNull);
      expect(result!.year, 2026);
      expect(result.month, 11);
      expect(result.day, 7);
      expect(result.hour, 8);
      expect(result.minute, 5);
    });

    test('toJakartaDateTime handles DateTime object in UTC', () {
      final utc = DateTime.utc(2026, 11, 7, 1, 5, 45);
      final result = toJakartaDateTime(utc);
      expect(result, isNotNull);
      expect(result!.hour, 8);
      expect(result.minute, 5);
    });

    test('toJakartaDateTime handles DateTime object in local timezone', () {
      // Regardless of local timezone, converting to UTC then +7 gives exact instant in WIB
      final local = DateTime(2026, 11, 7, 8, 5, 45);
      final result = toJakartaDateTime(local);
      expect(result, isNotNull);
      // result instant matches local instant
      expect(
        result!.millisecondsSinceEpoch - const Duration(hours: 7).inMilliseconds,
        local.toUtc().millisecondsSinceEpoch,
      );
    });

    test('toJakartaDateTime returns null for invalid or empty inputs', () {
      expect(toJakartaDateTime(null), isNull);
      expect(toJakartaDateTime(''), isNull);
      expect(toJakartaDateTime('   '), isNull);
      expect(toJakartaDateTime('invalid-date'), isNull);
      expect(toJakartaDateTime(12345), isNull);
    });

    test('formatJakartaDateTime formats to DD/MM/YYYY HH:mm', () {
      final str = formatJakartaDateTime('2026-11-07T01:05:45Z');
      expect(str, '07/11/2026 08:05');
    });

    test('formatJakartaDetailDateTime formats to DD/MM/YYYY • HH:mm', () {
      final str = formatJakartaDetailDateTime('2026-11-07T01:05:45Z');
      expect(str, '07/11/2026 • 08:05');
    });

    test('formatJakartaIsoDateHour formats to YYYY-MM-DD HH:mm', () {
      final str = formatJakartaIsoDateHour('2026-11-07T01:05:45Z');
      expect(str, '2026-11-07 08:05');
    });

    test('formatting functions return fallback when input is null or invalid', () {
      expect(formatJakartaDateTime(null), '-');
      expect(formatJakartaDetailDateTime(null), '-');
      expect(formatJakartaIsoDateHour(null), '-');
      expect(formatJakartaDateTime('bad', fallback: 'N/A'), 'N/A');
    });
  });
}

