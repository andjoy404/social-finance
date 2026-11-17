import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/presentation/screens/warga_screen.dart';

class _TestWargaListNotifier extends WargaListNotifier {
  final List<MappedResident> residents;
  _TestWargaListNotifier(this.residents);

  @override
  Future<List<MappedResident>> build() async {
    state = AsyncData(residents);
    return residents;
  }
}

class _MockAuthRepositoryForRoleTest extends AuthRepository {
  _MockAuthRepositoryForRoleTest(Map<String, dynamic>? user)
      : super(
          authService: AuthService(ApiClient(baseUrl: 'http://test.local')),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

Widget _buildRoleTestApp({
  required List<MappedResident> residents,
  required Map<String, dynamic>? currentUser,
}) {
  return ProviderScope(
    overrides: [
      wargaListProvider.overrideWith(() => _TestWargaListNotifier(residents)),
      wargaSearchQueryProvider.overrideWith((_) => ''),
      authRepositoryProvider.overrideWith(
        (ref) => _MockAuthRepositoryForRoleTest(currentUser),
      ),
    ],
    child: const MaterialApp(home: WargaScreen()),
  );
}

void main() {
  const regularResident = MappedResident(
    id: 'res-reg-1',
    name: 'Siti Rahma',
    houseNumber: 'B-05',
    phoneNumber: '081298765432',
    email: 'siti@example.com',
    occupancyStatus: 'Pemilik',
    jabatan: 'Anggota',
  );

  const securityResident = MappedResident(
    id: 'res-sec-1',
    name: 'Pak Satpam',
    houseNumber: 'Pos-01',
    phoneNumber: '08111222333',
    email: 'keamanan@example.com',
    occupancyStatus: 'Pemilik',
    jabatan: 'keamanan',
  );

  group('WargaScreen AppRole type-safety and contact redaction', () {
    testWidgets('1. AppRole.warga opens WargaScreen without any TypeError or exception', (tester) async {
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident, securityResident],
          currentUser: {
            'id': 'warga-user-1',
            'email': 'warga@example.com',
            'role': AppRole.warga,
            'system_role': null,
            'jabatan': null,
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('Siti Rahma'), findsOneWidget);
      expect(find.text('Pak Satpam'), findsOneWidget);
    });

    testWidgets('2. AppRole.warga does NOT see phone or email of regular residents, but sees phone for keamanan', (tester) async {
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident, securityResident],
          currentUser: {
            'id': 'warga-user-1',
            'email': 'warga@example.com',
            'role': AppRole.warga,
            'system_role': null,
            'jabatan': null,
          },
        ),
      );
      await tester.pumpAndSettle();

      // Regular resident: phone and email hidden
      expect(find.text('081298765432'), findsNothing);
      expect(find.text('siti@example.com'), findsNothing);

      // Keamanan resident: phone is visible, email is hidden
      expect(find.text('08111222333'), findsOneWidget);
      expect(find.text('keamanan@example.com'), findsNothing);
    });

    testWidgets('3. AppRole.pengurus sees phone and email for all residents', (tester) async {
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident],
          currentUser: {
            'id': 'pengurus-user-1',
            'email': 'pengurus@example.com',
            'role': AppRole.pengurus,
            'system_role': null,
            'jabatan': 'sekretaris',
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('081298765432'), findsOneWidget);
      expect(find.text('siti@example.com'), findsOneWidget);
    });

    testWidgets('4. Ketua RT with role AppRole.warga and jabatan ketua sees phone and email', (tester) async {
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident],
          currentUser: {
            'id': 'ketua-user-1',
            'email': 'ketua@example.com',
            'role': AppRole.warga,
            'system_role': null,
            'jabatan': 'ketua',
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('081298765432'), findsOneWidget);
      expect(find.text('siti@example.com'), findsOneWidget);
    });

    testWidgets('5. Super Admin sees phone, email, and FAB without regression', (tester) async {
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident],
          currentUser: {
            'id': 'sa-user-1',
            'email': 'admin@example.com',
            'role': AppRole.superAdmin,
            'system_role': 'super_admin',
            'jabatan': null,
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.text('081298765432'), findsOneWidget);
      expect(find.text('siti@example.com'), findsOneWidget);
      expect(find.byIcon(Icons.person_add), findsOneWidget);
    });

    testWidgets('6. Null role, null system_role, and null user are handled safely without crash', (tester) async {
      // 6a: null role and null system_role
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident],
          currentUser: {
            'id': 'empty-user-1',
            'email': 'empty@example.com',
            'role': null,
            'system_role': null,
            'jabatan': null,
          },
        ),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);

      // 6b: null user
      await tester.pumpWidget(
        _buildRoleTestApp(
          residents: [regularResident],
          currentUser: null,
        ),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  });
}

