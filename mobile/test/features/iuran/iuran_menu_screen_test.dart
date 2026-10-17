import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_menu_screen.dart';

class _TestBillsNotifier extends BillsNotifier {
  @override
  Future<List<IuranBill>> build() async => [];
}

Widget _menuApp(Map<String, dynamic>? user) {
  final router = GoRouter(
    initialLocation: '/home/iuran',
    routes: [
      GoRoute(
        path: '/home/iuran',
        builder: (context, state) => const IuranMenuScreen(),
        routes: [
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
            builder: (context, state) => const IuranNavigationShell(
              child: Scaffold(body: Text('Kelola destination')),
            ),
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
      billsProvider.overrideWith(() => _TestBillsNotifier()),
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
    expect(find.text('List Iuran'), findsNothing);
    expect(find.text('Tagihan'), findsNothing);
    expect(find.text('Kelola Iuran'), findsNothing);
  });

  testWidgets('authorized bendahara/pengurus sees tabs below Iuran title', (
    tester,
  ) async {
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

    // Directly shows Iuran title and tabs
    expect(find.text('List Iuran'), findsOneWidget);
    expect(find.text('Tagihan'), findsOneWidget);
    expect(find.text('Kelola Iuran'), findsOneWidget);

    // Tapping Tagihan navigates to Tagihan destination
    await tester.tap(find.text('Tagihan'));
    await tester.pumpAndSettle();
    expect(find.text('Tagihan destination'), findsOneWidget);
  });

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
