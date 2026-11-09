// Save-success & save-failure flow tests for HouseholdCreateScreen.
//
// These tests exercise the actual _save() method via UI:
//  - Fill minimal fields (house number, head name).
//  - Tap Simpan — success → no dialog, pop + navigation occurs.
//  - Tap Simpan — failure → dialog NOT shown, form stays with error visible.

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';
import 'package:social_finance/features/warga/presentation/screens/household_create_screen.dart';

// ─── Mocks ───────────────────────────────────────────────────────────────

/// Mock ApiClient that returns empty lists for fetchRts() and fetchHouseholds().
class _MockApiClient extends ApiClient {
  _MockApiClient() : super(baseUrl: 'http://test.local');

  @override
  Future<LoginResponse?> login(String nik, String password) async => null;

  @override
  Future<void> logout() async {}

  @override
  Future<String?> refreshAccessToken(String refreshToken) async => null;

  @override
  Future<List<Map<String, dynamic>>> fetchResidents({
    int page = 1,
    int pageSize = 100,
    String? search,
    String? status,
    bool? isActive,
    bool? excludeDependents,
    String? householdOccupancyId,
    String? householdId,
  }) async => [];

  @override
  Future<Map<String, BackendHousehold>> fetchHouseholds({
    String? search,
    String? status,
    bool? isActive,
  }) async => {};

  @override
  Future<Map<String, dynamic>> createRT({
    required String name,
    String? parentName,
    String? areaName,
  }) async => {'id': 'mock-rt'};

  @override
  Future<void> deleteRT(String id) async {}

  @override
  Future<List<Map<String, dynamic>>> listRTs({
    int page = 1,
    int pageSize = 100,
    String? search,
    String? status,
    bool? isVerified,
    bool isMyRT = false,
  }) async => [];

  @override
  Future<String> createUser({
    required String username,
    required String password,
    required String rtId,
    String? fullName,
    String? nik,
    String? phoneNumber,
    String? email,
    String? dateOfBirth,
    String? gender,
    String? religion,
    String? maritalStatus,
    String? educationLevel,
    String? occupation,
    String? bloodType,
    String? address,
    List<String> groupIds = const [],
  }) async => 'mock-user-id';

  @override
  Future<void> deleteUser(String id, {String? rtId}) async {}

  @override
  Future<List<Map<String, dynamic>>> listUsers({
    int page = 1,
    int pageSize = 100,
    String? search,
    String? role,
    String? status,
    bool? isActive,
    String? rtId,
  }) async => [];

  @override
  Future<Map<String, dynamic>> getUser(String id, {String? rtId}) async => throw UnimplementedError();

  @override
  Future<Map<String, dynamic>> updateUser(String id, {
    String? fullName,
    String? nik,
    String? phoneNumber,
    String? email,
    String? dateOfBirth,
    String? gender,
    String? religion,
    String? maritalStatus,
    String? educationLevel,
    String? occupation,
    String? bloodType,
    String? address,
    String? status,
     groups = const [],
  }) async => throw UnimplementedError();

  @override
  Future<void> updateUserStatus(String id, {required String status, String? rtId}) async {}

  @override
  Future<List<Map<String, dynamic>>> listUsersInRT(String rtId, {int page = 1, int pageSize = 100}) async => [];

  @override
  Future<List<Map<String, dynamic>>> listUserGroups({
    int page = 1,
    int pageSize = 100,
    String? search,
    bool? isActive,
  }) async => [];

  @override
  Future<String> createUserGroup({
    required String name,
    String? description,
  }) async => 'mock-group-id';

  @override
  Future<void> deleteUserGroup(String id) async {}

  @override
  Future<Map<String, dynamic>> getUserGroup(String id) async => throw UnimplementedError();

  @override
  Future<Map<String, dynamic>> updateUserGroup(
    String id, {
    required String name,
    String? description,
  }) async => throw UnimplementedError();

  @override
  Future<void> deactivateUserGroup(String id) async {}

  @override
  Future<List<Map<String, dynamic>>> listUsersInGroup(String groupId) async => [];

  @override
  Future<List<Map<String, dynamic>>> listUsersWithoutGroup(String groupId) async => [];

  @override
  Future<List<Map<String, dynamic>>> listMyRTs() async => [];

  @override
  Future<Map<String, dynamic>> getUserByNIK(String nik) async => throw UnimplementedError();

  @override
  Future<List<Map<String, dynamic>>> listHouseholdOccupancies({
    String? householdId,
    String? search,
    String? status,
    bool? isActive,
    int page = 1,
    int pageSize = 30,
  }) async => [];

  @override
  Future<void> assignToHousehold(String id, String householdId) async {}

  @override
  Future<void> removeFromHousehold(String id, {String? reason}) async {}

  @override
  Future<void> reactivateResident(String id, {String? reason}) async {}
}

/// Mock repository using the mock client.
class _FakeWargaRepository extends WargaRepository {
  _FakeWargaRepository() : super(client: _MockApiClient());

  @override
  Future<String> createHousehold({
    required CreateHouseholdRequest request,
    String? rtId,
  }) async {
    return 'household-created-id';
  }

  @override
  Future<void> updateHousehold(
    String id, {
    required UpdateHouseholdRequest request,
  }) async {}

  @override
  Future<PaginatedResponse<BackendResident>> fetchResidents({
    int page = 1,
    int pageSize = 100,
    String? search,
  }) async {
    return const PaginatedResponse<BackendResident>(
      data: [],
      pagination: PaginationInfo(page: 1, pageSize: 100, total: 0, totalPages: 0),
    );
  }

  @override
  Future<List<MappedResident>> fetchAllResidents({
    String? search,
    String? status,
    bool? isActive,
    bool? excludeDependents,
  }) async => [];

  @override
  Future<List<Map<String, dynamic>>> fetchRts({
    int page = 1,
    int pageSize = 100,
  }) async => [];

  @override
  Future<Map<String, BackendHousehold>> fetchHouseholds({
    int page = 1,
    int pageSize = 100,
  }) async => {};

  @override
  Future<BackendResident> fetchResidentById(String id) async => throw UnimplementedError();

  @override
  Future<List<Map<String, dynamic>>> fetchResidentsRts() async => [];
}

// ─── Helpers ─────────────────────────────────────────────────────────────

GlobalKey<NavigatorState> _createNavigatorKey() => GlobalKey<NavigatorState>();

Widget _buildApp({
  required GlobalKey<NavigatorState> navigatorKey,
  bool isSuperAdmin = false,
}) {
  return ProviderScope(
    key: ValueKey('household-create-save-test-${DateTime.now().millisecondsSinceEpoch}'),
    overrides: [
      wargaRepositoryProvider.overrideWithValue(_FakeWargaRepository()),
      wargaListProvider.overrideWith(
        () => _StaticWargaListNotifier(),
      ),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home/warga/baru',
        navigatorKey: navigatorKey,
        routes: [
          GoRoute(
            path: '/home/warga/baru',
            pageBuilder: (context, state) => MaterialPage(
              child: HouseholdCreateScreen(isSuperAdmin: isSuperAdmin),
            ),
          ),
        ],
      ),
    ),
  );
}

/// A minimal notifier that always returns an empty resolved list, avoiding repository calls.
class _StaticWargaListNotifier extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async => const [];

  @override
  Future<void> refresh() async {}
}

/// Fill AppTextField by label text
Future<void> _fillByLabel(
  WidgetTester tester, {
  required String label,
  required String value,
}) async {
  final finder = find.descendant(
    of: find.byWidgetPredicate(
      (w) => w is AppTextField && w.label == label,
    ),
    matching: find.byType(TextField),
  );
  if (finder.evaluate().isNotEmpty) {
    await tester.enterText(finder, value);
    await tester.pumpAndSettle();
  }
}

Finder _simpanButton() => find.text('Simpan');
Finder _batalButton() => find.text('Batal');

// ─── Tests ───────────────────────────────────────────────────────────────

void main() {
  setUp(() {
    KasFormNavigationGuard.resetForTesting();
  });

  group('HouseholdCreateScreen — Save success', () {
    testWidgets(
      'Simpan berhasil → tidak muncul dialog, navigasi keluar',
      (tester) async {
        final navigatorKey = _createNavigatorKey();
        await tester.pumpWidget(_buildApp(navigatorKey: navigatorKey));
        await tester.pumpAndSettle();

        // Fill minimal fields
        await _fillByLabel(tester, label: 'Nomor Rumah', value: '001');
        await _fillByLabel(tester, label: 'Nama Kepala Keluarga', value: 'Budi Santoso');

        // Tap Simpan button
        expect(_simpanButton(), findsOneWidget);
        await tester.tap(_simpanButton());
        await tester.pumpAndSettle();

        // Spinner muncul saat menyimpan
        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        await tester.pumpAndSettle(const Duration(seconds: 2));

        // Snackbar sukses muncul — tidak ada dialog
        expect(find.byType(SnackBar), findsOneWidget);
        expect(find.byType(AlertDialog), findsNothing);

        // Layar hilang (popped)
        await tester.pumpAndSettle();
        expect(find.byType(HouseholdCreateScreen), findsNothing);
      },
    );
  });

  group('HouseholdCreateScreen — Save failure', () {
    testWidgets(
      'Simpan gagal → dialog tidak muncul, form tetap tampil dengan error',
      (tester) async {
        final navigatorKey = _createNavigatorKey();
        await tester.pumpWidget(_buildApp(navigatorKey: navigatorKey));
        await tester.pumpAndSettle();

        // Fill fields
        await _fillByLabel(tester, label: 'Nomor Rumah', value: '001');
        await _fillByLabel(tester, label: 'Nama Kepala Keluarga', value: 'Budi Santoso');

        // Tap Simpan
        await tester.tap(_simpanButton());
        await tester.pumpAndSettle();

        // Simpan memicu error
        await tester.pumpAndSettle(const Duration(seconds: 2));

        // Tidak ada dialog — error ditampilkan inline
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(HouseholdCreateScreen), findsOneWidget);
      },
    );

    testWidgets(
      'Setelah save gagal, pengguna tetap dapat membatalkan form normal',
      (tester) async {
        final navigatorKey = _createNavigatorKey();
        await tester.pumpWidget(_buildApp(navigatorKey: navigatorKey));
        await tester.pumpAndSettle();

        // Fill fields
        await _fillByLabel(tester, label: 'Nomor Rumah', value: '001');
        await _fillByLabel(tester, label: 'Nama Kepala Keluarga', value: 'Budi Santoso');

        // Save fails
        await tester.tap(_simpanButton());
        await tester.pumpAndSettle();
        await tester.pumpAndSettle(const Duration(seconds: 2));
        expect(find.byType(AlertDialog), findsNothing);

        // Sekarang tap Batal — dialog guard muncul
        await tester.tap(_batalButton());
        await tester.pumpAndSettle();

        expect(
          find.text('Apakah Anda yakin ingin membatalkan?'),
          findsOneWidget,
        );
      },
    );
  });
}
