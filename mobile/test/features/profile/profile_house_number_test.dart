import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/profile/presentation/screens/profile_screen.dart';

class _TestAuthRepo extends AuthRepository {
  _TestAuthRepo(ApiClient client, Map<String, dynamic> initialUser)
      : super(
          authService: AuthService(client),
          apiClient: client,
        ) {
    state = AsyncData(initialUser);
  }

  @override
  Future<void> reloadProfile() async {
    // No-op for test to keep initial state
  }
}

void main() {
  group('ProfileScreen house number display', () {
    testWidgets('displays house number when present in user profile', (tester) async {
      final client = ApiClient();
      final user = {
        'id': 'user-1',
        'name': 'Budi Santoso',
        'email': 'budi@example.com',
        'phone': '081234567890',
        'role': AppRole.warga,
        'rt': '002',
        'rw': '003',
        'house_number': 'Blok A No. 10',
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _TestAuthRepo(client, user),
            ),
          ],
          child: const MaterialApp(
            home: ProfileScreen(),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Nomor Rumah'), findsOneWidget);
      expect(find.text('Blok A No. 10'), findsOneWidget);
    });

    testWidgets('displays "-" when house number is null or empty', (tester) async {
      final client = ApiClient();
      final user = {
        'id': 'user-2',
        'name': 'Siti Rahayu',
        'email': 'siti@example.com',
        'phone': '081234567891',
        'role': AppRole.warga,
        'rt': '002',
        'rw': '003',
        'house_number': null,
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (ref) => _TestAuthRepo(client, user),
            ),
          ],
          child: const MaterialApp(
            home: ProfileScreen(),
          ),
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Nomor Rumah'), findsOneWidget);
      expect(find.text('-'), findsWidgets);
    });
  });
}

