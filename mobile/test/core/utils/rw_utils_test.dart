import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/utils/rw_utils.dart';

void main() {
  group('looksLikeUuid', () {
    test('returns true for standard canonical UUID strings', () {
      expect(looksLikeUuid('123e4567-e89b-12d3-a456-426614174000'), isTrue);
      expect(looksLikeUuid('A0EEBC99-9C0B-4EF8-BB6D-6BB9BD380A11'), isTrue);
      expect(looksLikeUuid('00000000-0000-0000-0000-000000000000'), isTrue);
    });

    test('returns false for non-UUID strings', () {
      expect(looksLikeUuid('016'), isFalse);
      expect(looksLikeUuid('RW 016'), isFalse);
      expect(looksLikeUuid(''), isFalse);
      expect(looksLikeUuid(null), isFalse);
      expect(looksLikeUuid('123e4567-e89b-12d3-a456-42661417400'), isFalse); // 35 chars
      expect(looksLikeUuid('123e4567-e89b-12d3-a456-4266141740000'), isFalse); // 37 chars
      expect(looksLikeUuid('not-a-uuid-string-with-36-chars-xx'), isFalse);
    });
  });

  group('rwIdToDisplayString', () {
    test('returns null for null, empty, or UUID', () {
      expect(rwIdToDisplayString(null), isNull);
      expect(rwIdToDisplayString(''), isNull);
      expect(rwIdToDisplayString('123e4567-e89b-12d3-a456-426614174000'), isNull);
    });

    test('returns clean string if not a UUID', () {
      expect(rwIdToDisplayString('016'), equals('016'));
      expect(rwIdToDisplayString('RW 016'), equals('RW 016'));
    });
  });

  group('resolveRwNumber with List<RWMembership>', () {
    final memberships = [
      const RWMembership(
        rwId: 'uuid-rw-16-aaaa-bbbb-cccc-000000000016',
        rwNumber: 16,
        rwName: 'RW 016 Cilandak',
        jabatan: 'ketua',
      ),
      const RWMembership(
        rwId: 'uuid-rw-02-aaaa-bbbb-cccc-000000000002',
        rwNumber: 2,
        rwName: 'RW 002',
        jabatan: 'bendahara',
      ),
      const RWMembership(
        rwId: 'uuid-rw-no-num-aaaa-bbbb-cccc-000000',
        rwNumber: null,
        rwName: 'RW Tanpa Nomor',
      ),
    ];

    test('resolves rwNumber correctly from matching membership', () {
      expect(
        resolveRwNumber('uuid-rw-16-aaaa-bbbb-cccc-000000000016', memberships),
        equals('016'),
      );
      expect(
        resolveRwNumber('uuid-rw-02-aaaa-bbbb-cccc-000000000002', memberships),
        equals('002'),
      );
    });

    test('returns null when matching membership has null rwNumber', () {
      expect(
        resolveRwNumber('uuid-rw-no-num-aaaa-bbbb-cccc-000000', memberships),
        isNull,
      );
    });

    test('returns null when UUID is not in memberships (never picks wrong membership)', () {
      expect(
        resolveRwNumber('123e4567-e89b-12d3-a456-426614174000', memberships),
        isNull,
      );
    });

    test('resolves non-UUID rwId directly to 3-digit padded number', () {
      expect(resolveRwNumber('16', memberships), equals('016'));
      expect(resolveRwNumber('002', memberships), equals('002'));
      expect(resolveRwNumber('RW 016', memberships), equals('016'));
    });

    test('handles empty or null inputs safely', () {
      expect(resolveRwNumber(null, memberships), isNull);
      expect(resolveRwNumber('', memberships), isNull);
      expect(resolveRwNumber('uuid-rw-16-aaaa-bbbb-cccc-000000000016', null), isNull);
      expect(resolveRwNumber('uuid-rw-16-aaaa-bbbb-cccc-000000000016', <RWMembership>[]), isNull);
    });
  });

  group('resolveRwNumber with List<Map<String, dynamic>>', () {
    final mapMemberships = [
      {
        'rw_id': 'uuid-rw-16-aaaa-bbbb-cccc-000000000016',
        'rw_number': 16,
        'rw_name': 'RW 016',
      },
      {
        'rw_id': 'uuid-rw-05-aaaa-bbbb-cccc-000000000005',
        'rw_number': '5',
        'rw_name': 'RW 005',
      },
    ];

    test('resolves int and string rw_number from maps', () {
      expect(
        resolveRwNumber('uuid-rw-16-aaaa-bbbb-cccc-000000000016', mapMemberships),
        equals('016'),
      );
      expect(
        resolveRwNumber('uuid-rw-05-aaaa-bbbb-cccc-000000000005', mapMemberships),
        equals('005'),
      );
    });
  });

  group('rwIdToReadable', () {
    final memberships = [
      const RWMembership(
        rwId: 'uuid-rw-16-aaaa-bbbb-cccc-000000000016',
        rwNumber: 16,
        rwName: 'RW 016 Cilandak',
      ),
      const RWMembership(
        rwId: 'uuid-rw-named-only-bbbb-cccc-00000000',
        rwNumber: null,
        rwName: 'RW Khusus',
      ),
      const RWMembership(
        rwId: 'uuid-rw-empty-info-cccc-dddd-00000000',
        rwNumber: null,
        rwName: null,
      ),
    ];

    test('returns "RW 016" when rwNumber is available', () {
      expect(
        rwIdToReadable('uuid-rw-16-aaaa-bbbb-cccc-000000000016', memberships),
        equals('RW 016'),
      );
    });

    test('returns rwName if rwNumber is null', () {
      expect(
        rwIdToReadable('uuid-rw-named-only-bbbb-cccc-00000000', memberships),
        equals('RW Khusus'),
      );
    });

    test('returns safe "RW" fallback for unresolvable or empty membership (never raw UUID)', () {
      expect(
        rwIdToReadable('uuid-rw-empty-info-cccc-dddd-00000000', memberships),
        equals('RW'),
      );
      expect(
        rwIdToReadable('123e4567-e89b-12d3-a456-426614174000', memberships),
        equals('RW'),
      );
      expect(rwIdToReadable(null, memberships), equals('RW'));
      expect(rwIdToReadable('', memberships), equals('RW'));
    });

    test('formats readable string correctly when input is already a number', () {
      expect(rwIdToReadable('16', memberships), equals('RW 016'));
      expect(rwIdToReadable('016', memberships), equals('RW 016'));
      expect(rwIdToReadable('RW 016', memberships), equals('RW 016'));
    });
  });

  group('RWMembership.displayName', () {
    test('returns "RW 016" when rwNumber is present', () {
      const m = RWMembership(rwId: 'uuid-1234', rwNumber: 16);
      expect(m.displayName, equals('RW 016'));
    });

    test('returns rwName when rwNumber is null and rwName is present', () {
      const m = RWMembership(rwId: 'uuid-1234', rwName: 'RW Mandiri');
      expect(m.displayName, equals('RW Mandiri'));
    });

    test('returns "RW" when both are null (never returns raw UUID)', () {
      const m = RWMembership(rwId: 'uuid-1234');
      expect(m.displayName, equals('RW'));
      expect(m.displayName.contains('uuid'), isFalse);
    });
  });

  group('RwBadgeStyle Contrast & Theming', () {
    double contrastRatio(Color c1, Color c2) {
      final l1 = c1.computeLuminance();
      final l2 = c2.computeLuminance();
      final lighter = l1 > l2 ? l1 : l2;
      final darker = l1 > l2 ? l2 : l1;
      return (lighter + 0.05) / (darker + 0.05);
    }

    test('all styles provide high contrast (>4.5:1 WCAG AA) for light mode', () {
      final styles = [
        RwBadgeStyle.success(isDark: false),
        RwBadgeStyle.danger(isDark: false),
        RwBadgeStyle.warning(isDark: false),
        RwBadgeStyle.info(isDark: false),
        RwBadgeStyle.accent(isDark: false),
        RwBadgeStyle.neutral(isDark: false),
      ];

      for (final s in styles) {
        final cr = contrastRatio(s.background, s.text);
        expect(cr, greaterThanOrEqualTo(4.5),
            reason: 'Contrast ratio for light mode badge must meet WCAG AA');
      }
    });

    test('all styles provide high contrast (>4.5:1 WCAG AA) for dark mode', () {
      final styles = [
        RwBadgeStyle.success(isDark: true),
        RwBadgeStyle.danger(isDark: true),
        RwBadgeStyle.warning(isDark: true),
        RwBadgeStyle.info(isDark: true),
        RwBadgeStyle.accent(isDark: true),
        RwBadgeStyle.neutral(isDark: true),
      ];

      for (final s in styles) {
        final cr = contrastRatio(s.background, s.text);
        expect(cr, greaterThanOrEqualTo(4.5),
            reason: 'Contrast ratio for dark mode badge must meet WCAG AA');
      }
    });
  });
}
