import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Deep link normalization logic', () {
    String normalizeDeepLink(String? link) {
      if (link == null || link.trim().isEmpty) return '/home';
      final trimmed = link.trim();
      final billMatch =
          RegExp(r'^/(?:home/)?iuran/(?:detail/)?([a-zA-Z0-9_-]+)$')
              .firstMatch(trimmed);
      if (billMatch != null) {
        final billId = billMatch.group(1);
        if (billId != null &&
            billId.isNotEmpty &&
            billId != 'detail' &&
            billId != 'tunggakan' &&
            billId != 'laporan' &&
            billId != 'form') {
          return '/home/iuran/detail/$billId';
        }
      }
      if (trimmed == '/iuran' || trimmed == '/home/iuran') {
        return '/home/iuran';
      }
      if (trimmed.startsWith('/home')) {
        return trimmed;
      }
      if (trimmed.startsWith('/')) {
        return '/home$trimmed';
      }
      return '/home/$trimmed';
    }

    test('normalizes legacy FCM route /iuran/:id to /home/iuran/detail/:id', () {
      expect(
        normalizeDeepLink('/iuran/17363fb-c502-4fc9-b684-25035f606e10'),
        equals('/home/iuran/detail/17363fb-c502-4fc9-b684-25035f606e10'),
      );
    });

    test('normalizes /iuran/detail/:id to /home/iuran/detail/:id', () {
      expect(
        normalizeDeepLink('/iuran/detail/bill-12345'),
        equals('/home/iuran/detail/bill-12345'),
      );
    });

    test('normalizes base /iuran to /home/iuran', () {
      expect(normalizeDeepLink('/iuran'), equals('/home/iuran'));
    });

    test('preserves already canonical /home/iuran/detail/:id', () {
      expect(
        normalizeDeepLink('/home/iuran/detail/bill-xyz'),
        equals('/home/iuran/detail/bill-xyz'),
      );
    });

    test('handles empty or null deep link safely', () {
      expect(normalizeDeepLink(null), equals('/home'));
      expect(normalizeDeepLink(''), equals('/home'));
      expect(normalizeDeepLink('   '), equals('/home'));
    });
  });
}

