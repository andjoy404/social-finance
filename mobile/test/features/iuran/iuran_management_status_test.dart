import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_management_screen.dart';

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
    : super(
        authService: AuthService(ApiClient(baseUrl: 'http://test.local')),
        apiClient: ApiClient(baseUrl: 'http://test.local'),
      ) {
    state = AsyncData(user);
  }
}

class _FakeDueRepository extends IuranRepository {
  _FakeDueRepository(List<BackendDue> dues)
    : _dues = List.of(dues),
      super(apiClient: IuranApiClient(dio: Dio()));

  final List<BackendDue> _dues;
  final List<bool?> activeUpdates = [];

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => List.of(_dues);

  @override
  Future<BackendDue> updateDue(
    String id, {
    String? name,
    int? amount,
    DuePeriodType? periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    bool? isActive,
    String? validUntil,
  }) async {
    activeUpdates.add(isActive);
    final index = _dues.indexWhere((due) => due.id == id);
    final old = _dues[index];
    final updated = BackendDue(
      id: old.id,
      rtId: old.rtId,
      name: name ?? old.name,
      amount: amount != null ? '$amount.00' : old.amount,
      periodType: periodType ?? old.periodType,
      paymentRule: paymentRule ?? old.paymentRule,
      isVoluntary: isVoluntary ?? old.isVoluntary,
      isActive: isActive ?? old.isActive,
    );
    _dues[index] = updated;
    return updated;
  }
}

final _managerUser = <String, dynamic>{
  'id': 'u-bendahara',
  'email': 'bendahara@test.com',
  'role': 'bendahara',
  'system_role': null,
  'jabatan': 'bendahara',
  'profile_status': 'ready',
  'effective_permissions': <String>[
    'finance.read',
    'finance.create',
    'finance.update',
    'finance.approve',
  ],
};

BackendDue _due({required String id, required String name, required bool isActive}) {
  return BackendDue(
    id: id,
    rtId: 'rt-1',
    name: name,
    amount: '50000.00',
    periodType: DuePeriodType.monthly,
    paymentRule: PaymentRule.full,
    isVoluntary: false,
    isActive: isActive,
  );
}

Widget _buildScreen(_FakeDueRepository repository) {
  return ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(_managerUser)),
      iuranRepositoryProvider.overrideWithValue(repository),
    ],
    child: const MaterialApp(home: IuranManagementScreen()),
  );
}

Future<void> _openEditDialog(WidgetTester tester) async {
  await tester.tap(find.byIcon(Icons.edit_outlined).first);
  await tester.pumpAndSettle();
  expect(find.text('Edit Iuran'), findsOneWidget);
}

void main() {
  testWidgets('Edit Iuran shows the saved active status and saves inactive', (
    tester,
  ) async {
    final repository = _FakeDueRepository([
      _due(id: 'due-active', name: 'Iuran Keamanan', isActive: true),
    ]);

    await tester.pumpWidget(_buildScreen(repository));
    await tester.pumpAndSettle();

    await _openEditDialog(tester);

    expect(find.byKey(const ValueKey('edit-due-status')), findsOneWidget);
    expect(find.text('Aktif'), findsOneWidget);

    final statusField = find.byKey(const ValueKey('edit-due-status'));
    await tester.ensureVisible(statusField);
    await tester.pumpAndSettle();
    await tester.tap(statusField);
    await tester.pumpAndSettle();
    expect(find.text('Tidak Aktif'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('iuran-save-button')));
    await tester.pumpAndSettle();

    expect(repository.activeUpdates, contains(false));
    expect(find.textContaining('Tidak Aktif'), findsWidgets);
  });

  testWidgets('Edit Iuran shows inactive status and can reactivate a due', (
    tester,
  ) async {
    final repository = _FakeDueRepository([
      _due(id: 'due-inactive', name: 'Iuran Lama', isActive: false),
    ]);

    await tester.pumpWidget(_buildScreen(repository));
    await tester.pumpAndSettle();

    expect(find.textContaining('Tidak Aktif'), findsOneWidget);

    await _openEditDialog(tester);

    expect(find.text('Tidak Aktif'), findsWidgets);

    final statusField = find.byKey(const ValueKey('edit-due-status'));
    await tester.ensureVisible(statusField);
    await tester.pumpAndSettle();
    await tester.tap(statusField);
    await tester.pumpAndSettle();
    expect(find.text('Aktif'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('iuran-save-button')));
    await tester.pumpAndSettle();

    expect(repository.activeUpdates, contains(true));
    expect(find.textContaining('Aktif'), findsWidgets);
  });
}
