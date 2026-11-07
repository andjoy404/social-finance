import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/monitoring/data/monitoring_api_client.dart';
import 'package:social_finance/features/monitoring/data/monitoring_models.dart';
import 'package:social_finance/features/monitoring/data/monitoring_repository.dart';
import 'package:social_finance/features/monitoring/presentation/screens/monitoring_screen.dart';

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(
            ApiClient(baseUrl: 'http://test.local'),
            DeviceIdService(),
          ),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

class _FakeMonitoringRepository extends MonitoringRepository {
  _FakeMonitoringRepository()
      : super(apiClient: MonitoringApiClient(dio: Dio()));

  bool exportExcelCalled = false;
  bool exportPDFCalled = false;
  String? createdNote;

  @override
  Future<MonitoringListResponse> fetchMonitoringHouseholds(String rtId) async {
    return const MonitoringListResponse(
      evalDate: '2026-11-07',
      total: 1,
      households: [
        MonitoringHousehold(
          householdOccupancyId: 'occ-1',
          houseNumber: 'A-12',
          note: 'Catatan sebelumnya',
        ),
      ],
    );
  }

  @override
  Future<MonitoringNote> createMonitoringNote({
    required String rtId,
    required String householdOccupancyId,
    required String note,
  }) async {
    createdNote = note;
    return MonitoringNote(
      householdOccupancyId: householdOccupancyId,
      houseNumber: 'A-12',
      note: note,
    );
  }

  @override
  Future<List<int>> exportMonitoringExcel(String rtId) async {
    exportExcelCalled = true;
    return [1, 2, 3, 4];
  }

  @override
  Future<List<int>> exportMonitoringPDF(String rtId) async {
    exportPDFCalled = true;
    return [5, 6, 7, 8];
  }
}

void main() {
  testWidgets('renders monitoring screen and opens styled Catatan Lapangan dialog',
      (tester) async {
    final repo = _FakeMonitoringRepository();
    final cleaningUser = {
      'id': 'user-kebersihan',
      'email': 'kebersihan@example.com',
      'name': 'Petugas Kebersihan',
      'role': AppRole.warga,
      'jabatan': 'kebersihan',
      'rt': '002',
      'rt_id': 'rt-002',
      'rt_name': 'RT 002',
    };

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          monitoringRepositoryProvider.overrideWithValue(repo),
          authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(cleaningUser)),
        ],
        child: const MaterialApp(
          home: MonitoringScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Verify household is rendered
    expect(find.text('No. Rumah A-12'), findsOneWidget);
    expect(find.text('Catatan sebelumnya'), findsOneWidget);

    // Tap edit note icon
    await tester.tap(find.byIcon(Icons.edit_outlined));
    await tester.pumpAndSettle();

    // Dialog title
    expect(find.text('Catatan Lapangan - A-12'), findsOneWidget);

    // Verify Simpan and Batal buttons exist with proper styling
    final simpanFinder = find.widgetWithText(ElevatedButton, 'Simpan');
    final batalFinder = find.widgetWithText(OutlinedButton, 'Batal');
    expect(simpanFinder, findsOneWidget);
    expect(batalFinder, findsOneWidget);

    final simpanButton = tester.widget<ElevatedButton>(simpanFinder);
    expect(
      simpanButton.style?.backgroundColor?.resolve({}),
      AppColors.accent,
    );

    // Enter note and submit
    await tester.enterText(find.byType(TextField), 'Sampah sudah diangkut.');
    await tester.tap(simpanFinder);
    await tester.pumpAndSettle();

    expect(repo.createdNote, 'Sampah sudah diangkut.');
  });

  testWidgets('export action buttons are present and invoke local export without crash',
      (tester) async {
    final repo = _FakeMonitoringRepository();
    final cleaningUser = {
      'id': 'user-kebersihan',
      'email': 'kebersihan@example.com',
      'name': 'Petugas Kebersihan',
      'role': AppRole.warga,
      'jabatan': 'kebersihan',
      'rt': '002',
      'rt_id': 'rt-002',
      'rt_name': 'RT 002',
    };

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          monitoringRepositoryProvider.overrideWithValue(repo),
          authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(cleaningUser)),
        ],
        child: const MaterialApp(
          home: MonitoringScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Export Excel'), findsOneWidget);
    expect(find.text('Export PDF'), findsOneWidget);
    expect(find.text('Cetak'), findsOneWidget);
  });
}
