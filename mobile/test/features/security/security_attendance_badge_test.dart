import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/utils/attendance_order.dart';
import 'package:social_finance/features/security/presentation/widgets/attendance_status_badge.dart';
import 'package:social_finance/features/security/presentation/screens/security_management_screen.dart';

class _FakeAuthRepository
    extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepository(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

SecuritySchedule _schedule({
  required String id,
  required String officerName,
  required String attendanceStatus,
  String? attendanceStatusLabel,
}) {
  return SecuritySchedule(
    id: id,
    assignmentId: 'asg-$id',
    assignmentStartDate: '2026-10-01',
    assignmentEndDate: '2026-10-31',
    rtId: 'rt-002',
    userId: 'user-$id',
    officerName: officerName,
    membershipId: 'mem-$id',
    shiftConfigId: 'shift-1',
    shiftVersion: 1,
    shiftDate: '2026-10-08',
    shiftName: 'Shift Pagi',
    startTime: '07:00',
    endTime: '15:00',
    crossesMidnight: false,
    latenessToleranceMinutes: 15,
    status: 'scheduled',
    attendanceStatus: attendanceStatus,
    attendanceStatusLabel: attendanceStatusLabel,
  );
}

AttendanceRecord _attendance({
  required String id,
  required String officerName,
  required String shiftDate,
  required String startTime,
  required String status,
  required String statusLabel,
  DateTime? checkedInAt,
}) {
  return AttendanceRecord(
    id: id,
    rtId: 'rt-002',
    scheduleId: 'sch-$id',
    userId: 'user-$id',
    officerName: officerName,
    shiftConfigId: 'shift-1',
    shiftVersion: 1,
    shiftDate: shiftDate,
    shiftName: 'Shift Pagi',
    startTime: startTime,
    endTime: '15:00',
    crossesMidnight: false,
    status: status,
    statusLabel: statusLabel,
    checkedInAt: checkedInAt,
    lateMinutes: 0,
    correctionLocked: false,
  );
}

class _MockSecurityApiClient extends SecurityApiClient {
  _MockSecurityApiClient({required this.schedules, required this.attendance})
    : super(ApiClient(baseUrl: 'https://example.test'));

  final List<SecuritySchedule> schedules;
  final List<AttendanceRecord> attendance;

  @override
  Future<List<SecurityOfficer>> listManagerOfficers() async => const [
    SecurityOfficer(
      userId: 'user-f',
      membershipId: 'mem-f',
      rtId: 'rt-002',
      fullName: 'Petugas F',
      email: 'f@example.test',
      isActive: true,
    ),
  ];

  @override
  Future<List<ShiftConfig>> listManagerShifts() async => const [
    ShiftConfig(
      id: 'shift-1',
      rtId: 'rt-002',
      name: 'Shift Pagi',
      startTime: '07:00:00',
      endTime: '15:00:00',
      crossesMidnight: false,
      latenessToleranceMinutes: 15,
      isActive: true,
    ),
  ];

  @override
  Future<List<SecuritySchedule>> listManagerSchedules({
    String? dateFrom,
    String? dateTo,
    String? userId,
    int page = 1,
  }) async => List.of(schedules);

  @override
  Future<List<AttendanceRecord>> listManagerAttendance({
    String? dateFrom,
    String? dateTo,
    String? userId,
    String? status,
    int page = 1,
  }) async {
    if (status == null || status.isEmpty) return List.of(attendance);
    return attendance.where((a) => a.status == status).toList();
  }

  @override
  Future<List<AttendanceCorrection>> listManagerCorrections() async => const [];

  @override
  Future<List<SecurityDevice>> listManagerDevices() async => const [];

  @override
  Future<GuestBookPage> listManagerGuestBooks({
    String? search,
    int page = 1,
    String? status,
  }) async => const GuestBookPage([], 0, 1);
}

void main() {
  group('AttendanceStatusBadge tokens', () {
    test(
      'normalizeAttendanceStatusCode handles case, whitespace and dashes',
      () {
        expect(normalizeAttendanceStatusCode('Hadir'), 'hadir');
        expect(normalizeAttendanceStatusCode('  Belum Hadir '), 'belum_hadir');
        expect(
          normalizeAttendanceStatusCode('belum-absen-pulang'),
          'belum_absen_pulang',
        );
        expect(normalizeAttendanceStatusCode(null), '');
        expect(normalizeAttendanceStatusCode('   '), '');
      },
    );

    test('attendanceStatusLabelOf uses canonical Indonesian labels', () {
      expect(attendanceStatusLabelOf('hadir'), 'Hadir');
      expect(attendanceStatusLabelOf('belum_hadir'), 'Belum Hadir');
      expect(attendanceStatusLabelOf('terlambat'), 'Terlambat');
      expect(
        attendanceStatusLabelOf('sudah_absen_pulang'),
        'Sudah Absen Pulang',
      );
      expect(attendanceStatusLabelOf('tidak_hadir'), 'Tidak Hadir');
      expect(
        attendanceStatusLabelOf('belum_absen_pulang'),
        'Belum Absen Pulang',
      );
      // label form juga ikut terprioritas (dinormalkan sama)
      expect(
        attendanceStatusLabelOf('Belum Absen Pulang'),
        'Belum Absen Pulang',
      );
      expect(attendanceStatusLabelOf('Tidak Hadir'), 'Tidak Hadir');
      // unknown status: ditampilkan apa adanya, tidak crash
      expect(attendanceStatusLabelOf('status_baru'), 'status_baru');
      expect(attendanceStatusLabelOf(null), 'Tidak Diketahui');
      expect(attendanceStatusLabelOf('   '), 'Tidak Diketahui');
    });

    test('attendanceStatusColors maps statuses to intended hues', () {
      expect(
        attendanceStatusColors('hadir', isDark: false).background,
        isA<Color>(),
      );
      expect(
        attendanceStatusColors('belum_hadir', isDark: false).background,
        AppColors.danger.withValues(alpha: 0.13),
      );
      expect(
        attendanceStatusColors('tidak_hadir', isDark: false).background,
        AppColors.danger.withValues(alpha: 0.13),
      );
      expect(
        attendanceStatusColors('terlambat', isDark: false).background,
        AppColors.warning.withValues(alpha: 0.13),
      );
      expect(
        attendanceStatusColors('belum_absen_pulang', isDark: false).background,
        AppColors.warning.withValues(alpha: 0.13),
      );
      expect(
        attendanceStatusColors('hadir', isDark: false).background,
        AppColors.success.withValues(alpha: 0.13),
      );
      expect(
        attendanceStatusColors('sudah_absen_pulang', isDark: true).background,
        AppColors.success.withValues(alpha: 0.22),
      );
      // unknown / null -> abu-abu netral
      expect(
        attendanceStatusColors('status_baru', isDark: false).border,
        const Color(0xFF8A9AB0).withValues(alpha: 0.45),
      );
      expect(
        attendanceStatusColors(null, isDark: false).border,
        const Color(0xFF8A9AB0).withValues(alpha: 0.45),
      );
    });
  });

  group('compareAttendanceRecency', () {
    final f = _attendance(
      id: 'rec-f',
      officerName: 'Petugas F',
      shiftDate: '2026-10-08',
      startTime: '06:30',
      status: 'hadir',
      statusLabel: 'Hadir',
      checkedInAt: DateTime(2026, 10, 8, 10, 0),
    );
    final c = _attendance(
      id: 'rec-c',
      officerName: 'Petugas C',
      shiftDate: '2026-10-08',
      startTime: '08:00',
      status: 'hadir',
      statusLabel: 'Hadir',
    );
    final d = _attendance(
      id: 'rec-d',
      officerName: 'Petugas D',
      shiftDate: '2026-10-08',
      startTime: '08:00',
      status: 'hadir',
      statusLabel: 'Hadir',
    );
    final b = _attendance(
      id: 'rec-b',
      officerName: 'Petugas B',
      shiftDate: '2026-10-08',
      startTime: '07:30',
      status: 'tidak_hadir',
      statusLabel: 'Tidak Hadir',
    );
    final g = _attendance(
      id: 'rec-g',
      officerName: 'Petugas G',
      shiftDate: '2026-10-08',
      startTime: '07:00',
      status: 'belum_hadir',
      statusLabel: 'Belum Hadir',
    );
    final e = _attendance(
      id: 'rec-e',
      officerName: 'Petugas E',
      shiftDate: '2026-10-07',
      startTime: '08:00',
      status: 'belum_absen_pulang',
      statusLabel: 'Belum Absen Pulang',
    );
    final a = _attendance(
      id: 'rec-a',
      officerName: 'Petugas A',
      shiftDate: '2026-10-06',
      startTime: '19:00',
      status: 'sudah_absen_pulang',
      statusLabel: 'Sudah Absen Pulang',
    );

    test(
      'newest day first, then time, with id tie-break for identical timestamps',
      () {
        final shuffled = [a, c, e, d, b, f, g];
        final sorted = [...shuffled]..sort(compareAttendanceRecency);
        expect(sorted.map((r) => r.officerName).toList(), [
          'Petugas F',
          'Petugas C',
          'Petugas D',
          'Petugas B',
          'Petugas G',
          'Petugas E',
          'Petugas A',
        ]);
      },
    );

    test('same date & same time differing only by id is deterministic', () {
      expect(compareAttendanceRecency(c, d) < 0, isTrue);
      expect(compareAttendanceRecency(d, c) > 0, isTrue);
    });

    test('checked_in_at wins over scheduled start time', () {
      // F mencatat masuk 10:00 pada shift 06:30 — harus di atas record 08:00.
      expect(compareAttendanceRecency(f, c) < 0, isTrue);
    });
  });

  group('Keamanan screens - badge & sorting', () {
    final schedules = [
      _schedule(
        id: 's-1',
        officerName: 'Petugas Hadir',
        attendanceStatus: 'hadir',
        attendanceStatusLabel: 'Hadir',
      ),
      _schedule(
        id: 's-2',
        officerName: 'Petugas Belum',
        attendanceStatus: 'belum_hadir',
        attendanceStatusLabel: 'Belum Hadir',
      ),
      _schedule(
        id: 's-3',
        officerName: 'Petugas Terlambat',
        attendanceStatus: 'terlambat',
        attendanceStatusLabel: 'Terlambat',
      ),
      _schedule(
        id: 's-4',
        officerName: 'Petugas Sudah',
        attendanceStatus: 'sudah_absen_pulang',
        attendanceStatusLabel: 'Sudah Absen Pulang',
      ),
      _schedule(
        id: 's-5',
        officerName: 'Petugas Tidak',
        attendanceStatus: 'tidak_hadir',
        attendanceStatusLabel: 'Tidak Hadir',
      ),
      _schedule(
        id: 's-6',
        officerName: 'Petugas Pulang',
        attendanceStatus: 'belum_absen_pulang',
        attendanceStatusLabel: 'Belum Absen Pulang',
      ),
    ];

    // Sengaja di-acak: harus ditampilkan terbaru -> terlama setelah _loadData.
    final attendance = [
      _attendance(
        id: 'rec-a',
        officerName: 'Petugas A',
        shiftDate: '2026-10-06',
        startTime: '19:00',
        status: 'sudah_absen_pulang',
        statusLabel: 'Sudah Absen Pulang',
      ),
      _attendance(
        id: 'rec-b',
        officerName: 'Petugas B',
        shiftDate: '2026-10-08',
        startTime: '07:30',
        status: 'tidak_hadir',
        statusLabel: 'Tidak Hadir',
      ),
      _attendance(
        id: 'rec-c',
        officerName: 'Petugas C',
        shiftDate: '2026-10-08',
        startTime: '08:00',
        status: 'hadir',
        statusLabel: 'Hadir',
      ),
      _attendance(
        id: 'rec-f',
        officerName: 'Petugas F',
        shiftDate: '2026-10-08',
        startTime: '06:30',
        status: 'hadir',
        statusLabel: 'Hadir',
        checkedInAt: DateTime(2026, 10, 8, 10, 0),
      ),
      _attendance(
        id: 'rec-g',
        officerName: 'Petugas G',
        shiftDate: '2026-10-08',
        startTime: '07:00',
        status: 'belum_hadir',
        statusLabel: 'Belum Hadir',
      ),
      _attendance(
        id: 'rec-e',
        officerName: 'Petugas E',
        shiftDate: '2026-10-07',
        startTime: '08:00',
        status: 'terlambat',
        statusLabel: 'Terlambat',
      ),
      _attendance(
        id: 'rec-d',
        officerName: 'Petugas D',
        shiftDate: '2026-10-08',
        startTime: '08:00',
        status: 'hadir',
        statusLabel: 'Hadir',
      ),
    ];

    final managerUser = {
      'id': 'user-ketua',
      'email': 'ketua@local.test',
      'name': 'Ketua RT',
      'role': AppRole.warga,
      'jabatan': 'ketua',
      'effective_permissions': ['security.schedule.read'],
      'rt': '002',
      'rt_id': 'rt-002',
    };

    Future<void> pumpScreen(
      WidgetTester tester, {
      Size size = const Size(1200, 1500),
    }) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });
      final mockApi = _MockSecurityApiClient(
        schedules: schedules,
        attendance: attendance,
      );
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

    AttendanceStatusBadge badgeOf(WidgetTester tester, String label) {
      final finder = find.byWidgetPredicate(
        (w) => w is AttendanceStatusBadge && w.statusLabel == label,
      );
      expect(finder, findsAtLeastNWidgets(1));
      return tester.widget<AttendanceStatusBadge>(finder.first);
    }

    testWidgets(
      'A. Jadwal tab menampilkan badge status berwarna untuk 6 status',
      (tester) async {
        await pumpScreen(tester);
        await tester.tap(find.text('Jadwal'));
        await tester.pumpAndSettle();

        expect(find.byType(AttendanceStatusBadge), findsNWidgets(6));
        expect(badgeOf(tester, 'Hadir').statusCode, 'hadir');
        expect(badgeOf(tester, 'Belum Hadir').statusCode, 'belum_hadir');
        expect(badgeOf(tester, 'Terlambat').statusCode, 'terlambat');
        expect(
          badgeOf(tester, 'Sudah Absen Pulang').statusCode,
          'sudah_absen_pulang',
        );
        expect(badgeOf(tester, 'Tidak Hadir').statusCode, 'tidak_hadir');
        expect(
          badgeOf(tester, 'Belum Absen Pulang').statusCode,
          'belum_absen_pulang',
        );
      },
    );

    testWidgets('B. Absensi tab menampilkan badge & urutan terbaru-first', (
      tester,
    ) async {
      await pumpScreen(tester);
      // Ke top-level Absensi, lalu sub-tab Absensi (find.text('Absensi') .first = top tab)
      await tester.tap(find.text('Absensi').first);
      await tester.pumpAndSettle();

      expect(find.byType(AttendanceStatusBadge), findsNWidgets(7));

      double yOf(String text) =>
          tester.getTopLeft(find.textContaining(text)).dy;
      expect(yOf('Petugas F'), lessThan(yOf('Petugas C')));
      expect(yOf('Petugas C'), lessThan(yOf('Petugas D')));
      expect(yOf('Petugas D'), lessThan(yOf('Petugas B')));
      expect(yOf('Petugas B'), lessThan(yOf('Petugas G')));
      expect(yOf('Petugas G'), lessThan(yOf('Petugas E')));
      expect(yOf('Petugas E'), lessThan(yOf('Petugas A')));
    });

    testWidgets('B/C. Urutan tetap benar setelah refresh tab & filter status', (
      tester,
    ) async {
      await pumpScreen(tester);
      await tester.tap(find.text('Absensi').first);
      await tester.pumpAndSettle();

      // Refresh dengan pindah sub-tab bolak-balik.
      await tester.tap(find.text('Koreksi'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Absensi').last);
      await tester.pumpAndSettle();

      double yOf(String text) =>
          tester.getTopLeft(find.textContaining(text)).dy;
      expect(yOf('Petugas F'), lessThan(yOf('Petugas C')));
      expect(yOf('Petugas C'), lessThan(yOf('Petugas D')));
      expect(yOf('Petugas E'), lessThan(yOf('Petugas A')));

      // Filter status 'terlambat' → hanya record terlambat yang tersisa.
      await tester.tap(find.text('Semua Status'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Terlambat').last);
      await tester.pumpAndSettle();

      expect(find.textContaining('Petugas E'), findsOneWidget);
      expect(find.textContaining('Petugas C'), findsNothing);
      expect(find.byType(AttendanceStatusBadge), findsOneWidget);
    });

    testWidgets('badge tidak overflow pada layar kecil (320px)', (
      tester,
    ) async {
      await pumpScreen(tester, size: const Size(320, 1600));
      await tester.tap(find.text('Jadwal'));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(find.byType(AttendanceStatusBadge), findsNWidgets(6));
    });

    testWidgets(
      'navigasi & aksi lama tetap berfungsi (dialog detail Jadwal & Absensi)',
      (tester) async {
        await pumpScreen(tester);
        await tester.tap(find.text('Jadwal'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Petugas Hadir · Shift Pagi'));
        await tester.pumpAndSettle();
        expect(find.textContaining('Periode tugas'), findsOneWidget);
        await tester.tap(find.text('Tutup'));
        await tester.pumpAndSettle();

        await tester.tap(find.text('Absensi').first);
        await tester.pumpAndSettle();
        await tester.tap(find.text('Petugas F · Shift Pagi · 2026-10-08'));
        await tester.pumpAndSettle();
        expect(find.textContaining('Status:'), findsOneWidget);
        await tester.tap(find.text('Tutup'));
        await tester.pumpAndSettle();
      },
    );
  });
}
