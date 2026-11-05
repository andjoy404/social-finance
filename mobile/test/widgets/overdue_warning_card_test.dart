import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/widgets/overdue_warning_card.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

class _FakeAuthRepository extends AuthRepository {
  _FakeAuthRepository(Map<String, dynamic>? initialUser)
      : super(
          authService: AuthService(ApiClient(baseUrl: 'http://localhost')),
          apiClient: ApiClient(baseUrl: 'http://localhost'),
        ) {
    state = AsyncData(initialUser);
  }
}

class _FakeIuranRepository extends IuranRepository {
  OverdueWarningSummary summaryResponse;
  int fetchCalls = 0;

  _FakeIuranRepository({
    required this.summaryResponse,
  }) : super(apiClient: IuranApiClient(dio: Dio()));

  @override
  Future<OverdueWarningSummary> fetchOverdueWarningSummary() async {
    fetchCalls++;
    return summaryResponse;
  }
}

Widget _buildTestApp({
  required Map<String, dynamic>? user,
  required _FakeIuranRepository fakeRepo,
  String initialLocation = '/home',
}) {
  final router = GoRouter(
    initialLocation: initialLocation,
    routes: [
      GoRoute(
        path: '/home',
        builder: (context, state) => const Scaffold(
          body: Column(
            children: [
              OverdueWarningCard(),
              Expanded(child: Center(child: Text('Beranda Screen'))),
            ],
          ),
        ),
      ),
      GoRoute(
        path: '/home/iuran',
        builder: (context, state) => const Scaffold(
          body: Column(
            children: [
              OverdueWarningCard(),
              Expanded(child: Center(child: Text('Iuran Saya Screen'))),
            ],
          ),
        ),
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((ref) => _FakeAuthRepository(user)),
      iuranRepositoryProvider.overrideWithValue(fakeRepo),
    ],
    child: MaterialApp.router(
      routerConfig: router,
    ),
  );
}

void main() {
  const baseWargaUser = <String, dynamic>{
    'id': 'user-warga-1',
    'name': 'Budi Warga',
    'role': AppRole.warga,
    'system_role': null,
    'jabatan': null,
    'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
    'rt': '002',
    'rw_memberships': <Map<String, dynamic>>[],
  };

  group('OverdueWarningCard (Android Global Warning Card)', () {
    testWidgets(
      '1 & 5. renders warning card with title, badge, message, Rupiah total, bill count, and button for plain warga with >= 3m overdue',
      (tester) async {
        final fakeRepo = _FakeIuranRepository(
          summaryResponse: const OverdueWarningSummary(
            hasOverdue3Months: true,
            totalOutstanding: 450000,
            overdueBillCount: 3,
          ),
        );

        await tester.pumpWidget(
          _buildTestApp(user: baseWargaUser, fakeRepo: fakeRepo),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('global-overdue-warning-card')), findsOneWidget);
        expect(find.text('Peringatan Tunggakan Iuran'), findsOneWidget);
        expect(find.text('Tunggakan 3 Bulan+'), findsOneWidget);
        expect(
          find.textContaining('Anda memiliki tagihan iuran yang belum lunas selama 3 bulan atau lebih'),
          findsOneWidget,
        );
        expect(find.text('Rp 450.000'), findsOneWidget);
        expect(find.text('• 3 tagihan'), findsOneWidget);
        expect(find.text('Lihat Tagihan Saya'), findsOneWidget);
        expect(fakeRepo.fetchCalls, 1);
      },
    );

    testWidgets(
      '2 & 3. does not render card for warga with no overdue or < 3 months overdue',
      (tester) async {
        final fakeRepo = _FakeIuranRepository(
          summaryResponse: OverdueWarningSummary.empty,
        );

        await tester.pumpWidget(
          _buildTestApp(user: baseWargaUser, fakeRepo: fakeRepo),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('global-overdue-warning-card')), findsNothing);
        expect(fakeRepo.fetchCalls, 1);
      },
    );

    testWidgets(
      '4. does not render and does not fetch for pengurus, Ketua RT, Bendahara, RW pengurus, or Super Admin',
      (tester) async {
        final nonWargaUsers = <Map<String, dynamic>>[
          {...baseWargaUser, 'role': AppRole.pengurus},
          {...baseWargaUser, 'role': AppRole.bendahara},
          {...baseWargaUser, 'role': AppRole.superAdmin, 'system_role': 'super_admin'},
          {...baseWargaUser, 'role': AppRole.warga, 'jabatan': 'ketua'},
          {...baseWargaUser, 'role': AppRole.warga, 'jabatan': 'bendahara'},
          {
            ...baseWargaUser,
            'role': AppRole.warga,
            'rw_memberships': [
              {'rw_id': 'rw-1', 'rw_name': 'RW 01', 'jabatan': 'ketua'},
            ],
          },
        ];

        for (final u in nonWargaUsers) {
          expect(isPlainWargaUser(u), isFalse);
          final fakeRepo = _FakeIuranRepository(
            summaryResponse: const OverdueWarningSummary(
              hasOverdue3Months: true,
              totalOutstanding: 500000,
              overdueBillCount: 4,
            ),
          );

          await tester.pumpWidget(
            _buildTestApp(user: u, fakeRepo: fakeRepo),
          );
          await tester.pumpAndSettle();

          expect(find.byKey(const Key('global-overdue-warning-card')), findsNothing);
          expect(fakeRepo.fetchCalls, 0);
        }
      },
    );

    testWidgets(
      '6. tapping Lihat Tagihan Saya navigates to /home/iuran',
      (tester) async {
        final fakeRepo = _FakeIuranRepository(
          summaryResponse: const OverdueWarningSummary(
            hasOverdue3Months: true,
            totalOutstanding: 150000,
            overdueBillCount: 1,
          ),
        );

        await tester.pumpWidget(
          _buildTestApp(user: baseWargaUser, fakeRepo: fakeRepo),
        );
        await tester.pumpAndSettle();

        expect(find.text('Beranda Screen'), findsOneWidget);

        await tester.tap(find.byKey(const Key('overdue-warning-action-btn')));
        await tester.pumpAndSettle();

        expect(find.text('Iuran Saya Screen'), findsOneWidget);
      },
    );

    testWidgets(
      '7. hides card after overdue bills are paid and provider refreshes',
      (tester) async {
        final fakeRepo = _FakeIuranRepository(
          summaryResponse: const OverdueWarningSummary(
            hasOverdue3Months: true,
            totalOutstanding: 300000,
            overdueBillCount: 2,
          ),
        );

        late WidgetRef capturedRef;
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _FakeAuthRepository(baseWargaUser),
              ),
              iuranRepositoryProvider.overrideWithValue(fakeRepo),
            ],
            child: MaterialApp(
              home: Scaffold(
                body: Consumer(
                  builder: (context, ref, _) {
                    capturedRef = ref;
                    return const Column(
                      children: [
                        OverdueWarningCard(),
                      ],
                    );
                  },
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('global-overdue-warning-card')), findsOneWidget);

        // Simulate payment completion resolving the overdue balance
        fakeRepo.summaryResponse = OverdueWarningSummary.empty;
        await capturedRef.read(overdueWarningSummaryProvider.notifier).refresh();
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('global-overdue-warning-card')), findsNothing);
      },
    );

    testWidgets(
      '8. renders cleanly without overflow on narrow 320px screen and normalizes reminder deep links',
      (tester) async {
        tester.view.physicalSize = const Size(320, 640);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final fakeRepo = _FakeIuranRepository(
          summaryResponse: const OverdueWarningSummary(
            hasOverdue3Months: true,
            totalOutstanding: 1250000,
            overdueBillCount: 5,
          ),
        );

        await tester.pumpWidget(
          _buildTestApp(user: baseWargaUser, fakeRepo: fakeRepo),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byKey(const Key('global-overdue-warning-card')), findsOneWidget);

        // Also verify deep link normalization for the two new scheduler events
        expect(
          normalizeNotificationTarget(
            '/home/iuran/detail/bill-123',
            payload: {'type': 'iuran_upcoming_due_3_months', 'bill_id': 'bill-123'},
          ),
          '/home/iuran/detail/bill-123',
        );
        expect(
          normalizeNotificationTarget(
            '/home/iuran',
            payload: {'type': 'iuran_overdue_3_months'},
          ),
          '/home/iuran',
        );
      },
    );
  });
}
