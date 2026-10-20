import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/iuran/data/iuran_access.dart';

void main() {
  group('Iuran access', () {
    test('role-based access follows the backend route role requirements', () {
      expect(
        canReadIuran({
          'role': AppRole.warga,
          'effective_permissions': ['finance.read'],
        }),
        isTrue,
      );
      expect(
        canReadIuran({
          'role': AppRole.pengurus,
          'effective_permissions': ['finance.read'],
        }),
        isTrue,
      );
      expect(
        canReadIuran({
          'role': AppRole.bendahara,
          'effective_permissions': ['finance.read'],
        }),
        isTrue,
      );
      expect(
        canReadIuran({
          'role': AppRole.perangkat,
          'effective_permissions': ['finance.read'],
        }),
        isFalse,
      );

      const managementPermissions = [
        'finance.create',
        'finance.update',
        'finance.approve',
      ];
      expect(
        canManageIuran({
          'role': AppRole.pengurus,
          'effective_permissions': managementPermissions,
        }),
        isTrue,
      );
      expect(
        canManageIuran({
          'role': AppRole.bendahara,
          'effective_permissions': managementPermissions,
        }),
        isTrue,
      );
      expect(
        canManageIuran({
          'role': AppRole.warga,
          'effective_permissions': managementPermissions,
        }),
        isFalse,
      );
      expect(
        canManageIuran({
          'role': AppRole.perangkat,
          'effective_permissions': managementPermissions,
        }),
        isFalse,
      );
    });

    test('uses only permissions returned by /me, never jabatan alone', () {
      final user = {
        'role': AppRole.pengurus,
        'jabatan': 'sekretaris',
        'effective_permissions': ['finance.read', 'finance.create'],
      };

      expect(canReadIuran(user), isTrue);
      expect(canManageIuran(user), isFalse);
      expect(
        canManageIuran({
          'role': AppRole.pengurus,
          'jabatan': 'ketua',
          'effective_permissions': <String>[],
        }),
        isFalse,
      );
    });

    test('jabatan ketua or bendahara with warga role can manage when permissions granted', () {
      const managementPermissions = [
        'finance.read',
        'finance.create',
        'finance.update',
        'finance.approve',
      ];
      final ketuaUser = {
        'role': AppRole.warga,
        'jabatan': 'ketua',
        'effective_permissions': managementPermissions,
      };
      expect(canReadIuran(ketuaUser), isTrue);
      expect(canManageIuran(ketuaUser), isTrue);

      final bendaharaUser = {
        'role': 'warga',
        'jabatan': 'bendahara',
        'effective_permissions': managementPermissions,
      };
      expect(canReadIuran(bendaharaUser), isTrue);
      expect(canManageIuran(bendaharaUser), isTrue);
    });

    test('route access applies read and manage permissions to deep links', () {
      final warga = {
        'role': AppRole.warga,
        'effective_permissions': ['finance.read'],
      };
      final pengurus = {
        'role': AppRole.pengurus,
        'effective_permissions': [
          'finance.read',
          'finance.create',
          'finance.update',
          'finance.approve',
        ],
      };
      final readOnlyPengurus = {
        'role': AppRole.pengurus,
        'effective_permissions': ['finance.read'],
      };

      expect(canAccessIuranPath('/home/iuran/list', warga), isTrue);
      expect(canAccessIuranPath('/home/iuran/tagihan', warga), isTrue);
      expect(canAccessIuranPath('/home/iuran/kelola', warga), isFalse);
      expect(canAccessIuranPath('/home/iuran/form', warga), isFalse);
      expect(canAccessIuranPath('/home/iuran/kelola', pengurus), isTrue);
      expect(
        canAccessIuranPath('/home/iuran/kelola', readOnlyPengurus),
        isFalse,
      );
      expect(canAccessIuranPath('/home/iuran/detail/id-1', pengurus), isTrue);
    });

    test('super admin follows the backend permission bypass', () {
      const user = {
        'role': AppRole.superAdmin,
        'system_role': 'super_admin',
        'effective_permissions': [
          'finance.read',
          'finance.create',
          'finance.update',
          'finance.approve',
        ],
      };

      expect(canReadIuran(user), isTrue);
      expect(canManageIuran(user), isTrue);
    });

    test('missing or unexpected auth data does not grant access', () {
      expect(canReadIuran(null), isFalse);
      expect(canManageIuran(null), isFalse);
      expect(canManageIuran({'role': 'unknown'}), isFalse);
      expect(
        canManageIuran({
          'role': AppRole.pengurus,
          'effective_permissions': ['finance.create', 'finance.update'],
        }),
        isFalse,
      );
      expect(
        canManageIuran({
          'role': AppRole.pengurus,
          'effective_permissions': [
            'finance.create',
            'finance.update',
            'finance.approve',
          ],
          'jabatan': '',
        }),
        isTrue,
      );
      expect(
        canManageIuran({
          'role': AppRole.pengurus,
          'effective_permissions': [
            'finance.create',
            'finance.update',
            'finance.approve',
            123,
          ],
        }),
        isFalse,
      );
    });
  });
}
