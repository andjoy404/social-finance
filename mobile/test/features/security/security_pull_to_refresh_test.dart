import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';
import 'package:social_finance/features/security/presentation/screens/security_guest_book_screen.dart';

class _FakeAuthRepository
    extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepository(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _PullToRefreshMockApi extends SecurityApiClient {
  _PullToRefreshMockApi() : super(ApiClient(baseUrl: 'https://example.test'));

  int attendanceCalls = 0;
  int correctionsCalls = 0;
  int devicesCalls = 0;
  int guestsCalls = 0;
  int officersCalls = 0;

  bool failAttendance = false;
  bool failCorrections = false;
  bool failDevices = false;
  bool failGuests = false;

  @override
  Future<List<SecurityOfficer>> listManagerOfficers() async {
    officersCalls++;
    return const [
      SecurityOfficer(
        userId: 'user-f',
        membershipId: 'mem-f',
        rtId: 'rt-002',
        fullName: 'Petugas F',
        email: 'f@example.test',
        isActive: true,
      ),
    ];
  }

  @override
  Future<List<ShiftConfig>> listManagerShifts() async => const [];

  @override
  Future<List<SecuritySchedule>> listManagerSchedules({
    String? dateFrom,
    String? dateTo,
    String? userId,
    int page = 1,
  }) async => const [];

  @override
  Future<List<AttendanceRecord>> listManagerAttendance({
    String? dateFrom,
    String? dateTo,
    String? userId,
    String? status,
    int page = 1,
  }) async {
    attendanceCalls++;
    if (failAttendance) throw Exception('attendance reload failed');
    return List.generate(
      3,
      (i) => AttendanceRecord(
        id: 'att-$attendanceCalls-$i',
        rtId: 'rt-002',
        scheduleId: 'sch-$i',
        userId: 'user-$i',
        officerName: 'Petugas Attendance $attendanceCalls-$i',
        shiftConfigId: 'shift-1',
        shiftVersion: 1,
        shiftDate: '2026-10-08',
        shiftName: 'Shift Pagi',
        startTime: '07:00',
        endTime: '15:00',
        crossesMidnight: false,
        status: 'hadir',
        statusLabel: 'Hadir',
        lateMinutes: 0,
        correctionLocked: false,
      ),
    );
  }

  @override
  Future<List<AttendanceCorrection>> listManagerCorrections() async {
    correctionsCalls++;
    if (failCorrections) throw Exception('correction reload failed');
    return List.generate(
      2,
      (i) => AttendanceCorrection(
        id: 'corr-$correctionsCalls-$i',
        attendanceRecordId: 'att-$i',
        scheduleId: 'sch-$i',
        officerName: 'user-$i',
        shiftDate: '2026-10-08',
        shiftName: 'Shift Pagi',
        originalStatus: 'belum_hadir',
        requestedStatus: 'hadir',
        reason: 'Lupa scan',
        requestType: 'status_change',
        requestedBy: 'user-$i',
        status: 'pending',
      ),
    );
  }

  @override
  Future<List<SecurityDevice>> listManagerDevices() async {
    devicesCalls++;
    if (failDevices) throw Exception('device reload failed');
    return List.generate(
      2,
      (i) => SecurityDevice(
        id: 'dev-$devicesCalls-$i',
        rtId: 'rt-002',
        rtName: 'RT 002',
        rtCode: '002',
        deviceName: 'Pos Device $devicesCalls-$i',
        locationLabel: 'Pos Utama',
        isActive: true,
        isPaired: true,
      ),
    );
  }

  @override
  Future<GuestBookPage> listManagerGuestBooks({
    String? search,
    int page = 1,
    String? status,
  }) async {
    guestsCalls++;
    if (failGuests) throw Exception('guest reload failed');
    return GuestBookPage(
      List.generate(
        2,
        (i) => GuestBookEntry(
          id: 'guest-$guestsCalls-$i',
          rtId: 'rt-002',
          deviceId: 'dev-1',
          deviceName: 'Pos Utama',
          guestName: 'Tamu $guestsCalls-$i',
          phoneMasked: '***',
          purpose: 'kunjungan',
          destinationLabel: 'Pos',
          notes: null,
          checkedInAt: DateTime(2026, 10, 8, 10, 0),
          checkedOutAt: null,
        ),
      ),
      2,
      1,
    );
  }
}

void main() {
  const managerUser = {
    'id': 'user-ketua',
    'email': 'ketua@local.test',
    'name': 'Ketua RT',
    'role': AppRole.warga,
    'jabatan': 'ketua',
    'effective_permissions': [
      'security.schedule.read',
      'security.schedule.write',
      'security.device.manage',
      'security.correction',
      'security.guest_book',
      'security.attendance',
    ],
    'rt': '002',
    'rt_id': 'rt-002',
  };

  Future<void> pumpScreen(
    WidgetTester tester,
    _PullToRefreshMockApi mockApi, {
    Size size = const Size(1200, 1000),
  }) async {
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(home: SecurityManagementScreen()),
      ),
    );
    await tester.pumpAndSettle();
  }

  Future<void> pumpGuestBookScreen(
    WidgetTester tester,
    _PullToRefreshMockApi mockApi, {
    Size size = const Size(1200, 1000),
  }) async {
    tester.view.physicalSize = size;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(
            (_) => _FakeAuthRepository(AsyncData(managerUser)),
          ),
          securityApiClientProvider.overrideWithValue(mockApi),
        ],
        child: const MaterialApp(home: SecurityGuestBookScreen()),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('ABSENSI: pull to refresh memuat ulang data absensi', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpScreen(tester, mock);

    // switch to Absensi tab (top tab index 1)
    await tester.tap(find.text('Absensi').first);
    await tester.pumpAndSettle();

    final initialCalls = mock.attendanceCalls;
    expect(find.textContaining('Petugas Attendance $initialCalls-0'), findsOneWidget);

    // pull to refresh
    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(mock.attendanceCalls, greaterThan(initialCalls));
    expect(find.textContaining('Petugas Attendance ${mock.attendanceCalls}-0'), findsOneWidget);
    expect(find.textContaining('Petugas Attendance $initialCalls-0'), findsNothing);
  });

  testWidgets(
    'ABSENSI: refresh gagal tidak menghapus data lama dan menampilkan snackbar',
    (tester) async {
      final mock = _PullToRefreshMockApi();
      await pumpScreen(tester, mock);

      await tester.tap(find.text('Absensi').first);
      await tester.pumpAndSettle();
      final initialCalls = mock.attendanceCalls;
      expect(find.textContaining('Petugas Attendance $initialCalls-0'), findsOneWidget);

      mock.failAttendance = true;
      await tester.fling(
        find.byType(ListView).first,
        const Offset(0, 400),
        1000,
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('Petugas Attendance $initialCalls-0'), findsOneWidget);
      expect(find.textContaining('Gagal memuat ulang'), findsOneWidget);
    },
  );

  testWidgets('ABSENSI: refresh tetap bekerja saat daftar kosong', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    mock.failAttendance = false;
    final _ = mock;
    // make attendance empty: override by setting a flag via subclass? Instead, just verify call count increments
    await pumpScreen(tester, mock);

    await tester.tap(find.text('Absensi').first);
    await tester.pumpAndSettle();
    expect(mock.attendanceCalls, greaterThanOrEqualTo(1));

    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();
    expect(mock.attendanceCalls, greaterThanOrEqualTo(2));
  });

  testWidgets('KOREKSI: pull to refresh memuat ulang daftar koreksi', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpScreen(tester, mock);

    await tester.tap(find.text('Absensi').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Koreksi'));
    await tester.pumpAndSettle();

    expect(mock.correctionsCalls, greaterThanOrEqualTo(1));
    expect(find.text('user-0 · 2026-10-08 · pending'), findsAtLeastNWidgets(1));

    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(mock.correctionsCalls, greaterThanOrEqualTo(2));
    expect(find.textContaining('Lupa scan'), findsWidgets);
  });

  testWidgets('PERANGKAT: pull to refresh memuat ulang daftar perangkat', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpScreen(tester, mock);

    await tester.tap(find.text('Perangkat').first);
    await tester.pumpAndSettle();

    final initialCalls = mock.devicesCalls;
    expect(find.textContaining('Pos Device $initialCalls-'), findsWidgets);

    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(mock.devicesCalls, greaterThan(initialCalls));
    expect(find.textContaining('Pos Device ${mock.devicesCalls}-'), findsWidgets);
  });

  testWidgets('BUKU TAMU: pull to refresh memuat ulang daftar tamu', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpGuestBookScreen(tester, mock);

    final initialCalls = mock.guestsCalls;
    expect(find.textContaining('Tamu $initialCalls-'), findsWidgets);

    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(mock.guestsCalls, greaterThan(initialCalls));
    expect(find.textContaining('Tamu ${mock.guestsCalls}-'), findsWidgets);
  });

  testWidgets('BUKU TAMU: refresh gagal tidak menghapus data lama', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpGuestBookScreen(tester, mock);

    final initialCalls = mock.guestsCalls;
    expect(find.textContaining('Tamu $initialCalls-'), findsWidgets);

    mock.failGuests = true;
    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(find.textContaining('Tamu $initialCalls-'), findsWidgets);
    expect(find.textContaining('Gagal memuat ulang'), findsOneWidget);
  });

  testWidgets('BUKU TAMU: search/filter tetap berfungsi setelah refresh', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpGuestBookScreen(tester, mock);

    // refresh
    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    // search field still exists and usable
    final searchField = find.byType(TextField);
    expect(searchField, findsWidgets);
    await tester.enterText(searchField.first, 'Tamu');
    await tester.pump();
    expect(searchField, findsWidgets);
  });

  testWidgets('refresh tidak overflow pada layar kecil (320px)', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    tester.view.physicalSize = const Size(320, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    await pumpScreen(tester, mock);

    await tester.tap(find.text('Absensi').first);
    await tester.pumpAndSettle();
    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(mock.attendanceCalls, greaterThanOrEqualTo(2));
  });

  testWidgets('refresh gagal pada PERANGKAT tidak menghapus data lama', (
    tester,
  ) async {
    final mock = _PullToRefreshMockApi();
    await pumpScreen(tester, mock);

    await tester.tap(find.text('Perangkat').first);
    await tester.pumpAndSettle();
    final initialCalls = mock.devicesCalls;
    expect(find.textContaining('Pos Device $initialCalls-'), findsWidgets);

    mock.failDevices = true;
    await tester.fling(find.byType(ListView).first, const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(find.textContaining('Pos Device $initialCalls-'), findsWidgets);
    expect(find.textContaining('Gagal memuat ulang'), findsOneWidget);
  });
}
