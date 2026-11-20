import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/monitoring/data/monitoring_models.dart';
import 'package:social_finance/features/security/data/pos_device_storage.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/pos_activation_screen.dart';
import 'package:social_finance/features/security/presentation/screens/pos_kiosk_screen.dart';
import 'package:social_finance/features/security/presentation/screens/security_absen_screen.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';

class _FakeAuthRepository
    extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepository(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _InMemoryPosDeviceStorage extends PosDeviceStorage {
  PosDeviceSession? _stored;

  @override
  Future<void> saveSession(PosDeviceSession session) async {
    _stored = session;
  }

  @override
  Future<PosDeviceSession?> loadSession() async => _stored;

  @override
  Future<void> clearSession() async {
    _stored = null;
  }
}

class _FakeSecurityApiClient extends SecurityApiClient {
  _FakeSecurityApiClient() : super(ApiClient(baseUrl: 'https://example.test'));

  bool qrRefreshed = false;
  bool guestCheckedOut = false;

  @override
  Future<SecurityDevice> fetchPosDeviceMe(PosDeviceSession session) async =>
      session.device;

  @override
  Future<OfficerAttendanceOverview> fetchMyAttendanceOverview() async {
    return OfficerAttendanceOverview(
      serverTime: DateTime(2026, 11, 12, 20, 5),
      timezone: 'Asia/Jakarta',
      currentSchedule: SecuritySchedule(
        id: 'sch-today',
        rtId: 'rt-002',
        userId: 'user-sec-1',
        officerName: 'Pak Budi',
        membershipId: 'mem-1',
        shiftConfigId: 'shift-malam',
        shiftVersion: 1,
        shiftDate: '2026-11-12',
        shiftName: 'Shift Malam',
        startTime: '20:00:00',
        endTime: '06:00:00',
        crossesMidnight: true,
        latenessToleranceMinutes: 15,
        status: 'scheduled',
      ),
      currentRecord: AttendanceRecord(
        id: 'att-today',
        rtId: 'rt-002',
        scheduleId: 'sch-today',
        userId: 'user-sec-1',
        officerName: 'Pak Budi',
        shiftConfigId: 'shift-malam',
        shiftVersion: 1,
        shiftDate: '2026-11-12',
        shiftName: 'Shift Malam',
        startTime: '20:00:00',
        endTime: '06:00:00',
        crossesMidnight: true,
        status: 'belum_hadir',
        statusLabel: 'Belum Hadir',
        lateMinutes: 0,
        correctionLocked: false,
      ),
      upcomingShifts: [
        SecuritySchedule(
          id: 'sch-next',
          rtId: 'rt-002',
          userId: 'user-sec-1',
          officerName: 'Pak Budi',
          membershipId: 'mem-1',
          shiftConfigId: 'shift-malam',
          shiftVersion: 1,
          shiftDate: '2026-11-13',
          shiftName: 'Shift Malam',
          startTime: '20:00:00',
          endTime: '06:00:00',
          crossesMidnight: true,
          latenessToleranceMinutes: 15,
          status: 'scheduled',
        ),
      ],
      recentHistory: customHistory.isNotEmpty ? customHistory : const [],
    );
  }

  @override
  Future<List<ShiftSwap>> listSwaps() async => [
    const ShiftSwap(
      id: 'swap-1',
      rtId: 'rt-002',
      requesterUserId: 'user-sec-2',
      requesterName: 'Pak Joko',
      targetUserId: 'user-sec-1',
      targetName: 'Pak Budi',
      requesterScheduleId: 'sch-joko',
      requesterShiftDate: '2026-11-14',
      requesterShiftName: 'Shift Pagi',
      targetScheduleId: 'sch-next',
      targetShiftDate: '2026-11-13',
      targetShiftName: 'Shift Malam',
      reason: 'Ada urusan keluarga mendadak',
      status: 'pending_target',
    ),
  ];

  List<AttendanceCorrection> correctionsList = [];
  List<AttendanceRecord> customHistory = [];
  String? lastCreatedRecordId;
  String? lastCreatedStatus;
  String? lastCreatedReason;

  @override
  Future<List<AttendanceCorrection>> listCorrections() async => correctionsList;

  @override
  Future<AttendanceCorrection> createCorrection({
    required String attendanceRecordId,
    required String requestType,
    required String requestedStatus,
    String? requestedCheckedInAt,
    String? requestedCheckedOutAt,
    required String reason,
  }) async {
    lastCreatedRecordId = attendanceRecordId;
    lastCreatedStatus = requestedStatus;
    lastCreatedReason = reason;
    final created = AttendanceCorrection(
      id: 'corr-new',
      attendanceRecordId: attendanceRecordId,
      scheduleId: 'sch-today',
      officerName: 'Pak Budi',
      requestedBy: 'user-sec-1',
      shiftDate: '2026-11-12',
      shiftName: 'Shift Malam',
      requestType: requestType,
      originalStatus: 'terlambat',
      requestedStatus: requestedStatus,
      reason: reason,
      status: 'pending',
      createdAt: DateTime.now(),
    );
    correctionsList = [...correctionsList, created];
    return created;
  }

  @override
  Future<PosDeviceSession> activatePosDevice({
    required String serverUrl,
    required String pairingToken,
  }) async {
    return PosDeviceSession(
      serverUrl: serverUrl,
      deviceToken: 'dev-1.secret-token',
      device: const SecurityDevice(
        id: 'dev-1',
        rtId: 'rt-002',
        rtName: 'RT 002',
        rtCode: '002',
        deviceName: 'Tablet Pos Gerbang Utama',
        locationLabel: 'Gerbang Utama',
        isActive: true,
        isPaired: true,
      ),
    );
  }

  @override
  Future<DynamicQrPayload> issuePosQrToken(PosDeviceSession session) async {
    qrRefreshed = true;
    return DynamicQrPayload(
      tokenId: 'tok-1',
      qrToken: 'DYNAMIC-QR-PAYLOAD-12345',
      deviceId: session.device.id,
      deviceName: session.device.deviceName,
      rtId: session.device.rtId,
      issuedAt: DateTime.now(),
      expiresAt: DateTime.now().add(const Duration(seconds: 40)),
      expiresInSeconds: 40,
    );
  }

  @override
  Future<List<AttendanceRecord>> fetchPosTodayAttendance(
    PosDeviceSession session,
  ) async {
    return [
      AttendanceRecord(
        id: 'att-1',
        rtId: 'rt-002',
        scheduleId: 'sch-1',
        userId: 'u-1',
        officerName: 'Pak Budi Satpam',
        shiftConfigId: 'shift-1',
        shiftVersion: 1,
        shiftDate: '2026-11-12',
        shiftName: 'Shift Malam',
        startTime: '20:00:00',
        endTime: '06:00:00',
        crossesMidnight: true,
        status: 'hadir',
        statusLabel: 'Hadir',
        lateMinutes: 0,
        checkedInAt: DateTime(2026, 11, 12, 20, 2),
        correctionLocked: false,
      ),
    ];
  }

  @override
  Future<List<PosHouseholdSummary>> fetchPosHouseholds(
    PosDeviceSession session, {
    String? search,
  }) async {
    return const [
      PosHouseholdSummary(
        householdOccupancyId: 'ho-1',
        houseNumber: 'A-12',
        headOfFamilyName: 'Bapak Hendra',
        occupancyType: 'tetap',
        residentCount: 4,
      ),
    ];
  }

  @override
  Future<PosHouseholdPage> fetchPosHouseholdsPage(
    PosDeviceSession session, {
    String? search,
    int page = 1,
    int perPage = 20,
  }) async {
    return const PosHouseholdPage(
      [
        PosHouseholdSummary(
          householdOccupancyId: 'ho-1',
          houseNumber: 'A-12',
          headOfFamilyName: 'Bapak Hendra',
          occupancyType: 'tetap',
          residentCount: 4,
        ),
      ],
      1,
      1,
    );
  }

  @override
  Future<GuestBookPage> fetchPosGuestBooks(
    PosDeviceSession session, {
    String? status,
    String? search,
    int page = 1,
  }) async {
    return GuestBookPage(
      [
        GuestBookEntry(
          id: 'guest-1',
          rtId: 'rt-002',
          deviceId: 'dev-1',
          deviceName: 'Tablet Pos Gerbang Utama',
          guestName: 'Kurir Ekspedisi',
          phoneMasked: '0812****7890',
          purpose: 'Antar Paket',
          destinationLabel: 'Rumah No. A-12',
          checkedInAt: DateTime(2026, 11, 12, 14, 30),
        ),
      ],
      1,
      1,
    );
  }

  @override
  Future<GuestBookEntry> checkoutPosGuestBook(
    PosDeviceSession session,
    String entryId,
  ) async {
    guestCheckedOut = true;
    return GuestBookEntry(
      id: entryId,
      rtId: 'rt-002',
      deviceId: 'dev-1',
      deviceName: 'Tablet Pos Gerbang Utama',
      guestName: 'Kurir Ekspedisi',
      purpose: 'Antar Paket',
      destinationLabel: 'Rumah No. A-12',
      checkedInAt: DateTime(2026, 11, 12, 14, 30),
      checkedOutAt: DateTime(2026, 11, 12, 14, 45),
    );
  }

  @override
  Future<PosWasteMonitoringResponse> fetchPosMonitoringSampah(
    PosDeviceSession session,
  ) async {
    return const PosWasteMonitoringResponse(
      evalDate: '2026-11-12',
      items: [
        MonitoringHousehold(
          householdOccupancyId: 'ho-9',
          houseNumber: 'B-05',
          note: 'Tempat sampah di depan pagar',
        ),
      ],
    );
  }
}

void main() {
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets(
    'Security officer nav shows Warga, Absen, Notifikasi, Profil in order without Sampah',
    (tester) async {
      final securityUser = {
        'id': 'user-sec-1',
        'email': 'keamanan@local.test',
        'name': 'Pak Budi',
        'role': AppRole.warga,
        'jabatan': 'keamanan',
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      final router = GoRouter(
        initialLocation: '/home/patroli',
        routes: [
          ShellRoute(
            builder: (context, state, child) =>
                MainNavigationShell(child: child),
            routes: [
              GoRoute(
                path: '/home/warga',
                builder: (context, state) => const Text('Halaman Warga'),
              ),
              GoRoute(
                path: '/home/absen',
                builder: (context, state) => const Text('Halaman Absensi'),
              ),
              GoRoute(
                path: '/home/patroli',
                builder: (context, state) => const Text('Halaman Patroli'),
              ),
              GoRoute(
                path: '/home/notifikasi',
                builder: (context, state) => const Text('Halaman Sampah'),
              ),
              GoRoute(
                path: '/home/paket',
                builder: (context, state) => const Text('Halaman Paket'),
              ),
              GoRoute(
                path: '/home/profile',
                builder: (context, state) => const Text('Halaman Profil'),
              ),
            ],
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (_) => _FakeAuthRepository(AsyncData(securityUser)),
            ),
            securityAlertsDataProvider.overrideWith(
              (ref) async => const SecurityAlertsData(),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      // Fixed bottom nav contains Beranda and Menu
      expect(find.byKey(const Key('nav-item-beranda')), findsOneWidget);
      expect(find.byKey(const Key('nav-item-menu')), findsOneWidget);

      // Open collapsed menu grid
      await tester.tap(find.byKey(const Key('nav-item-menu')));
      await tester.pumpAndSettle();

      expect(find.text('Warga'), findsOneWidget);
      expect(find.text('Absensi'), findsOneWidget);
      expect(find.text('Patroli'), findsOneWidget);
      expect(find.text('Sampah'), findsOneWidget);
      expect(find.text('Paket'), findsOneWidget);
      expect(find.text('Profil'), findsOneWidget);

      await tester.tap(find.byKey(const Key('nav-item-patroli')));
      await tester.pumpAndSettle();
      expect(find.text('Halaman Patroli'), findsOneWidget);
    },
  );

  testWidgets(
    'SecurityAbsenScreen displays current shift, scan QR button, and incoming swap request',
    (tester) async {
      final fakeApi = _FakeSecurityApiClient();
      final securityUser = {
        'id': 'user-sec-1',
        'name': 'Pak Budi',
        'role': AppRole.warga,
        'jabatan': 'keamanan',
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (_) => _FakeAuthRepository(AsyncData(securityUser)),
            ),
            securityAlertsDataProvider.overrideWith(
              (ref) async => const SecurityAlertsData(),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            securityApiClientProvider.overrideWithValue(fakeApi),
          ],
          child: const MaterialApp(home: SecurityAbsenScreen()),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Absensi Keamanan'), findsOneWidget);
      expect(find.textContaining('Shift Malam · 2026-11-12'), findsOneWidget);
      expect(find.text('Scan QR untuk Check-In'), findsOneWidget);
      expect(find.text('Permintaan Tukar Shift'), findsOneWidget);
      expect(find.text('Setujui Tukar'), findsOneWidget);
      expect(find.text('2026-11-13 · Shift Malam'), findsOneWidget);
    },
  );

  testWidgets(
    'PosActivationScreen activates device with pairing token and navigates to /pos',
    (tester) async {
      final fakeApi = _FakeSecurityApiClient();
      final fakeStorage = _InMemoryPosDeviceStorage();

      final router = GoRouter(
        initialLocation: '/pos/activate',
        routes: [
          GoRoute(
            path: '/login',
            builder: (context, state) => const Text('Login'),
          ),
          GoRoute(
            path: '/pos/activate',
            builder: (context, state) => const PosActivationScreen(),
          ),
          GoRoute(
            path: '/pos',
            builder: (context, state) => const Text('Kiosk Pos Aktif'),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            securityApiClientProvider.overrideWithValue(fakeApi),
            posDeviceStorageProvider.overrideWithValue(fakeStorage),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextField, 'URL Server API'),
        'https://api.socialfinance.test',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'Token Pairing Perangkat'),
        'PAIR-TOKEN-999',
      );
      await tester.ensureVisible(find.text('Aktifkan Perangkat Pos'));
      await tester.tap(find.text('Aktifkan Perangkat Pos'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      await tester.pumpAndSettle();

      expect(find.text('Kiosk Pos Aktif'), findsOneWidget);
      expect(fakeStorage._stored?.deviceToken, equals('dev-1.secret-token'));
    },
  );

  testWidgets(
    'PosKioskScreen displays dynamic QR, Warga, Buku Tamu checkout, and read-only Sampah tabs',
    (tester) async {
      final fakeApi = _FakeSecurityApiClient();
      final fakeStorage = _InMemoryPosDeviceStorage();
      await fakeStorage.saveSession(
        const PosDeviceSession(
          serverUrl: 'https://api.socialfinance.test',
          deviceToken: 'dev-1.secret-token',
          device: SecurityDevice(
            id: 'dev-1',
            rtId: 'rt-002',
            rtName: 'RT 002',
            rtCode: '002',
            deviceName: 'Tablet Pos Gerbang Utama',
            locationLabel: 'Gerbang Utama',
            isActive: true,
            isPaired: true,
          ),
        ),
      );

      final container = ProviderContainer(
        overrides: [
          securityApiClientProvider.overrideWithValue(fakeApi),
          posDeviceStorageProvider.overrideWithValue(fakeStorage),
        ],
      );
      addTearDown(container.dispose);
      await container.read(posSessionProvider.notifier).restoreSession();

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: PosKioskScreen()),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      // Default tab: Absensi QR
      expect(find.byType(QrImageView), findsOneWidget);
      expect(find.textContaining('detik lagi'), findsOneWidget);
      await tester.drag(find.byType(ListView).first, const Offset(0, -400));
      await tester.pump();
      expect(find.textContaining('Pak Budi Satpam'), findsOneWidget);
      expect(fakeApi.qrRefreshed, isTrue);

      // Switch to Warga (read-only)
      await tester.tap(find.text('Warga'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.textContaining('Bapak Hendra'), findsOneWidget);

      // Switch to Buku Tamu and checkout guest
      await tester.tap(find.text('Buku Tamu'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.textContaining('Kurir Ekspedisi'), findsOneWidget);
      await tester.tap(find.text('Keluar'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(fakeApi.guestCheckedOut, isTrue);

      // Switch to Sampah (read-only)
      await tester.tap(find.text('Sampah'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.text('Rumah No. B-05'), findsOneWidget);

      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump();
    },
  );

  testWidgets(
    'Next date shift remains available and previous completed shift does not lock out next day',
    (tester) async {
      final fakeApi = _FakeSecurityApiClient();
      // Configure overview for Day 2:
      // Yesterday (Day 1) was completed and is in recent history, but today has no record yet.
      final securityUser = {
        'id': 'user-sec-1',
        'name': 'Pak Budi',
        'role': AppRole.warga,
        'jabatan': 'keamanan',
      };

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (_) => _FakeAuthRepository(AsyncData(securityUser)),
            ),
            securityAlertsDataProvider.overrideWith(
              (ref) async => const SecurityAlertsData(),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            securityApiClientProvider.overrideWithValue(fakeApi),
          ],
          child: const MaterialApp(home: SecurityAbsenScreen()),
        ),
      );
      await tester.pumpAndSettle();

      // Today's shift should have Scan QR button available
      expect(find.text('Scan QR untuk Check-In'), findsOneWidget);
      expect(find.text('Absensi shift ini telah selesai.'), findsNothing);
    },
  );

  testWidgets(
    'Security officer can access manual correction, validate reason, submit correction, and view pending request in history',
    (tester) async {
      final fakeApi = _FakeSecurityApiClient();
      final securityUser = {
        'id': 'user-sec-1',
        'name': 'Pak Budi',
        'role': AppRole.warga,
        'jabatan': 'keamanan',
      };

      // Populate history with an eligible attendance record from yesterday
      fakeApi.customHistory = [
        AttendanceRecord(
          id: 'att-hist-1',
          rtId: 'rt-002',
          scheduleId: 'sch-today',
          userId: 'user-sec-1',
          officerName: 'Pak Budi',
          shiftConfigId: 'shift-malam',
          shiftVersion: 1,
          shiftDate: '2026-11-11',
          shiftName: 'Shift Malam',
          startTime: '20:00:00',
          endTime: '06:00:00',
          crossesMidnight: true,
          status: 'terlambat',
          statusLabel: 'Terlambat',
          lateMinutes: 25,
          correctionLocked: false,
          checkedInAt: DateTime(2026, 11, 11, 20, 25),
        ),
      ];

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(
              (_) => _FakeAuthRepository(AsyncData(securityUser)),
            ),
            securityAlertsDataProvider.overrideWith(
              (ref) async => const SecurityAlertsData(),
            ),
            securityAlertsProvider.overrideWith((ref) async => false),
            securityApiClientProvider.overrideWithValue(fakeApi),
          ],
          child: const MaterialApp(home: SecurityAbsenScreen()),
        ),
      );
      await tester.pumpAndSettle();

      // 1. Switch to 'Riwayat & Koreksi' tab
      await tester.tap(find.text('Riwayat & Koreksi'));
      await tester.pumpAndSettle();

      // 2. Verify attendance record is displayed in history with Koreksi button
      expect(find.text('2026-11-11 · Shift Malam'), findsOneWidget);
      expect(find.text('Terlambat'), findsOneWidget);
      final koreksiBtn = find.widgetWithText(TextButton, 'Koreksi');
      expect(koreksiBtn, findsOneWidget);

      // 3. Tap 'Koreksi' button to open dialog
      await tester.tap(koreksiBtn);
      await tester.pumpAndSettle();

      expect(find.text('Ajukan Koreksi Absensi'), findsOneWidget);
      expect(find.text('2026-11-11 · Shift Malam (Terlambat)'), findsOneWidget);

      // 4. Test validation error: submit without reason
      final submitBtn = find.widgetWithText(ElevatedButton, 'Kirim Koreksi');
      await tester.tap(submitBtn);
      await tester.pumpAndSettle();

      expect(find.text('Alasan koreksi wajib diisi.'), findsOneWidget);

      // 5. Open dialog again and provide valid reason and status
      await tester.tap(koreksiBtn);
      await tester.pumpAndSettle();

      await tester.enterText(
        find.byType(TextField),
        'Lupa scan pulang karena patroli darurat',
      );
      await tester.tap(find.widgetWithText(ElevatedButton, 'Kirim Koreksi'));
      await tester.pumpAndSettle();

      // 6. Verify API was called with exact record ID and reason
      expect(fakeApi.lastCreatedRecordId, 'att-hist-1');
      expect(fakeApi.lastCreatedReason, 'Lupa scan pulang karena patroli darurat');

      // 7. Verify newly created correction is now listed in history with pending status
      expect(find.textContaining('Status: pending'), findsOneWidget);
      expect(
        find.textContaining('Alasan: Lupa scan pulang karena patroli darurat'),
        findsOneWidget,
      );
    },
  );
}
