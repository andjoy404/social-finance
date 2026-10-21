import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_management_screen.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_menu_screen.dart';

class _TestBillsNotifier extends BillsNotifier {
  _TestBillsNotifier([this._bills = const []]);

  final List<IuranBill> _bills;

  @override
  Future<List<IuranBill>> build() async => _bills;
}

class _TestMyBillsNotifier extends BillsNotifier {
  _TestMyBillsNotifier([this._bills = const []]);

  final List<IuranBill> _bills;

  @override
  Future<List<IuranBill>> build() async => _bills;
}

class _TestDuesNotifier extends DuesNotifier {
  @override
  Future<List<BackendDue>> build() async => [
    BackendDue(
      id: 'due-1',
      rtId: 'rt-1',
      name: 'IPL',
      amount: '85000',
      periodType: DuePeriodType.monthly,
      paymentRule: PaymentRule.full,
      isVoluntary: false,
      isActive: true,
      createdAt: DateTime(2026, 10, 1),
    ),
    const BackendDue(
      id: 'due-2',
      rtId: 'rt-1',
      name: 'Donasi Sosial',
      amount: '0',
      periodType: DuePeriodType.oneTime,
      paymentRule: PaymentRule.partial,
      isVoluntary: true,
      isActive: true,
    ),
  ];
}

Widget _menuApp(
  Map<String, dynamic>? user, {
  List<IuranBill> myBills = const [],
  List<IuranBill> bills = const [],
}) {
  final router = GoRouter(
    initialLocation: '/home/iuran',
    routes: [
      GoRoute(
        path: '/home/iuran',
        builder: (context, state) => const IuranMenuScreen(),
        routes: [
          GoRoute(
            path: 'warga',
            builder: (context, state) => const IuranNavigationShell(
              child: Scaffold(body: Text('Iuran Warga destination')),
            ),
          ),
          GoRoute(
            path: 'list',
            builder: (context, state) => const IuranNavigationShell(
              child: Scaffold(body: Text('List destination')),
            ),
          ),
          GoRoute(
            path: 'tagihan',
            builder: (context, state) => const IuranNavigationShell(
              child: Scaffold(body: Text('Tagihan destination')),
            ),
          ),
          GoRoute(
            path: 'kelola',
            builder: (context, state) => const IuranManagementScreen(),
          ),
          GoRoute(
            path: 'form',
            builder: (context, state) =>
                const Scaffold(body: Text('Form destination')),
          ),
        ],
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((ref) {
        final client = ApiClient(customDio: Dio());
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        repo.state = AsyncData<Map<String, dynamic>?>(user);
        return repo;
      }),
      billsProvider.overrideWith(() => _TestBillsNotifier(bills)),
      myBillsProvider.overrideWith(() => _TestMyBillsNotifier(myBills)),
      duesProvider.overrideWith(() => _TestDuesNotifier()),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets('WARGA directly sees bills list without tabs or kelola', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.warga,
        'effective_permissions': ['finance.read'],
      }),
    );
    await tester.pumpAndSettle();

    // Warga directly sees bills list
    expect(find.text('Belum Ada Tagihan'), findsOneWidget);
    // Warga does not see any tabs
    expect(find.text('Iuran Saya'), findsNothing);
    expect(find.text('Iuran Warga'), findsNothing);
    expect(find.text('Kelola Iuran'), findsNothing);
    // D1: No + Buat Tagihan button on IuranScreen
    expect(find.text('Buat Tagihan'), findsNothing);
  });

  testWidgets(
    'authorized bendahara/pengurus sees Iuran Saya, Iuran Warga, Kelola Iuran in order and lands on Iuran Saya (D1)',
    (tester) async {
      final ownBill = IuranBill(
        id: 'own-bill',
        householdName: 'Tagihan Pribadi',
        iuranType: 'IPL',
        periode: 'Oktober 2026',
        nominal: 50000,
      );
      final residentBill = IuranBill(
        id: 'resident-bill',
        householdName: 'Tagihan Warga Lain',
        iuranType: 'IPL',
        periode: 'Oktober 2026',
        nominal: 50000,
      );

      await tester.pumpWidget(
        _menuApp(
          {
            'role': AppRole.bendahara,
            'effective_permissions': [
              'finance.read',
              'finance.create',
              'finance.update',
              'finance.approve',
            ],
          },
          myBills: [ownBill],
          bills: [residentBill],
        ),
      );
      await tester.pumpAndSettle();

      // Directly shows Iuran title and tabs in the required order
      expect(find.text('Iuran Saya'), findsOneWidget);
      expect(find.text('Iuran Warga'), findsOneWidget);
      expect(find.text('Kelola Iuran'), findsOneWidget);

      final sayaX = tester.getTopLeft(find.text('Iuran Saya')).dx;
      final wargaX = tester.getTopLeft(find.text('Iuran Warga')).dx;
      final kelolaX = tester.getTopLeft(find.text('Kelola Iuran')).dx;
      expect(sayaX, lessThan(wargaX));
      expect(wargaX, lessThan(kelolaX));

      // Default landing page is Iuran Saya: only the personal bill is listed
      expect(find.text('Tagihan Pribadi'), findsOneWidget);
      expect(find.text('Tagihan Warga Lain'), findsNothing);

      // D1: + Buat Tagihan FAB must not exist on main Iuran screen
      expect(find.text('Buat Tagihan'), findsNothing);

      // Tapping Iuran Warga navigates to the resident-wide list
      await tester.tap(find.text('Iuran Warga'));
      await tester.pumpAndSettle();
      expect(find.text('Iuran Warga destination'), findsOneWidget);
    },
  );

  testWidgets(
    'Kelola Iuran renders + Buat Iuran pill button (A4), no Switch (D2), KAS-style filters (D3), and List Iuran badges (D4)',
    (tester) async {
      await tester.pumpWidget(
        _menuApp({
          'role': AppRole.bendahara,
          'effective_permissions': [
            'finance.read',
            'finance.create',
            'finance.update',
            'finance.approve',
          ],
        }),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.text('Kelola Iuran'));
      await tester.pumpAndSettle();

      // A4: + pill button exists with tooltip
      expect(find.byKey(const Key('kelola-iuran-create-btn')), findsOneWidget);
      expect(find.byTooltip('Buat Iuran'), findsOneWidget);

      // D2: Edit button exists, Switch does NOT exist
      expect(find.byIcon(Icons.edit_outlined), findsWidgets);
      expect(find.byType(Switch), findsNothing);

      // D4: Badges matching List Iuran (IPL, Reguler, Sukarela, OKTOBER 2026)
      expect(find.text('IPL'), findsWidgets);
      expect(find.text('Reguler'), findsWidgets);
      expect(find.text('Sukarela'), findsWidgets);
      expect(find.text('OKTOBER 2026'), findsOneWidget);

      // D3: Filter by Jenis Iuran (Reguler vs Sukarela)
      await tester.tap(find.byKey(const Key('kelola-iuran-jenis-filter')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Sukarela').last);
      await tester.pumpAndSettle();

      expect(find.text('Donasi Sosial'), findsWidgets);
      expect(find.text('OKTOBER 2026'), findsNothing);
    },
  );

  testWidgets('profile loading shows loading indicator', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.pengurus,
        'profile_status': 'loading',
        'effective_permissions': <String>[],
      }),
    );

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(
      find.text('Akses Iuran tidak tersedia untuk akun ini.'),
      findsNothing,
    );
  });

  testWidgets('profile errors provide retry instead of access denial', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.pengurus,
        'profile_status': 'error',
        'effective_permissions': <String>[],
      }),
    );

    expect(
      find.text(
        'Profil akses gagal dimuat. Hak akses Iuran belum dapat diverifikasi.',
      ),
      findsOneWidget,
    );
    expect(find.text('Coba Lagi'), findsOneWidget);
    expect(
      find.text('Akses Iuran tidak tersedia untuk akun ini.'),
      findsNothing,
    );
  });

  testWidgets('missing permission contract is reported as unavailable', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.warga,
        'profile_status': 'permissions_unavailable',
        'effective_permissions': <String>[],
      }),
    );

    expect(
      find.text('Server belum mengirimkan data permission untuk akun ini.'),
      findsOneWidget,
    );
    expect(
      find.text('Akses Iuran tidak tersedia untuk akun ini.'),
      findsNothing,
    );
  });
}
