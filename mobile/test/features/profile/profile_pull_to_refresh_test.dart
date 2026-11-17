import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/profile/presentation/screens/profile_screen.dart';

class _RefreshTestAuthRepo extends AuthRepository {
  int reloadCount = 0;

  _RefreshTestAuthRepo(ApiClient client, Map<String, dynamic> initialUser)
      : super(
          authService: AuthService(client),
          apiClient: client,
        ) {
    state = AsyncData(initialUser);
  }

  @override
  Future<void> reloadProfile() async {
    reloadCount++;
    state = AsyncData({
      ...state?.valueOrNull ?? {},
      'name': 'Budi Santoso (Updated)',
    });
  }
}

void main() {
  testWidgets('pull to refresh invokes reloadProfile and updates user profile data',
      (tester) async {
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
    final repo = _RefreshTestAuthRepo(client, user);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith((ref) => repo),
        ],
        child: const MaterialApp(
          home: ProfileScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RefreshIndicator), findsOneWidget);
    final initialReloads = repo.reloadCount;

    // Perform pull-to-refresh gesture
    await tester.fling(
      find.byType(SingleChildScrollView),
      const Offset(0, 300),
      1000,
    );
    await tester.pumpAndSettle();

    expect(repo.reloadCount, greaterThan(initialReloads));
    expect(find.text('Budi Santoso (Updated)'), findsOneWidget);
  });
}

