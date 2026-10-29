import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/models/role.dart';

bool canUserAccessNotificationMenu(Map<String, dynamic>? user) {
  if (user == null) return false;
  final systemRole = user['system_role'] as String?;
  final jabatan = (user['jabatan'] ?? '').toString().toLowerCase();
  return systemRole == 'super_admin' || jabatan == 'ketua';
}

void main() {
  group('Notification Menu Access Control', () {
    test('super_admin has full access', () {
      final user = {
        'id': 'u-admin',
        'system_role': 'super_admin',
        'role': AppRole.pengurus,
        'jabatan': null,
      };
      expect(canUserAccessNotificationMenu(user), isTrue);
    });

    test('Ketua RT has access', () {
      final user = {
        'id': 'u-ketua',
        'system_role': null,
        'role': AppRole.pengurus,
        'jabatan': 'ketua',
      };
      expect(canUserAccessNotificationMenu(user), isTrue);
    });

    test('Bendahara RT does NOT have access', () {
      final user = {
        'id': 'u-bendahara',
        'system_role': null,
        'role': AppRole.bendahara,
        'jabatan': 'bendahara',
      };
      expect(canUserAccessNotificationMenu(user), isFalse);
    });

    test('Warga does NOT have access', () {
      final user = {
        'id': 'u-warga',
        'system_role': null,
        'role': AppRole.warga,
        'jabatan': null,
      };
      expect(canUserAccessNotificationMenu(user), isFalse);
    });

    test('Pengurus non-ketua (sekretaris) does NOT have access', () {
      final user = {
        'id': 'u-sekre',
        'system_role': null,
        'role': AppRole.pengurus,
        'jabatan': 'sekretaris',
      };
      expect(canUserAccessNotificationMenu(user), isFalse);
    });
  });
}

