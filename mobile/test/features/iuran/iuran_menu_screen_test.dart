import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_menu_screen.dart';

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
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  testWidgets('warga sees List Iuran and Tagihan, and routes correctly', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.warga,
        'effective_permissions': ['finance.read'],
      }),
    );

    expect(find.text('List Iuran'), findsOneWidget);
    expect(find.text('Tagihan'), findsOneWidget);
    expect(find.text('Kelola Iuran'), findsNothing);

    await tester.tap(find.text('List Iuran'));
    await tester.pumpAndSettle();
    expect(find.text('List destination'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'List Iuran'), findsOneWidget);
  });

  testWidgets('authorized bendahara can open Kelola Iuran', (tester) async {
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

    expect(find.text('List Iuran'), findsOneWidget);
    expect(find.text('Tagihan'), findsOneWidget);
    expect(find.text('Kelola Iuran'), findsOneWidget);

    await tester.tap(find.text('Kelola Iuran'));
    await tester.pumpAndSettle();
    expect(find.text('Kelola destination'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Kelola Iuran'), findsOneWidget);
  });

  testWidgets('partial permissions expose read-only Iuran menu items', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.pengurus,
        'jabatan': 'sekretaris',
        'effective_permissions': ['finance.read', 'finance.create'],
      }),
    );

    expect(find.text('List Iuran'), findsOneWidget);
    expect(find.text('Tagihan'), findsOneWidget);
    expect(find.text('Kelola Iuran'), findsNothing);
  });

  testWidgets('profile loading does not show an access-denied message', (
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

  testWidgets('read-only pengurus cannot see Kelola Iuran', (tester) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.pengurus,
        'jabatan': 'wakil_ketua',
        'effective_permissions': ['finance.read'],
      }),
    );

    expect(find.text('List Iuran'), findsOneWidget);
    expect(find.text('Tagihan'), findsOneWidget);
    expect(find.text('Kelola Iuran'), findsNothing);
  });

  testWidgets('tagihan item opens its corresponding destination', (
    tester,
  ) async {
    await tester.pumpWidget(
      _menuApp({
        'role': AppRole.warga,
        'effective_permissions': ['finance.read'],
      }),
    );

    await tester.tap(find.text('Tagihan'));
    await tester.pumpAndSettle();
    expect(find.text('Tagihan destination'), findsOneWidget);
    expect(find.widgetWithText(FilledButton, 'Tagihan'), findsOneWidget);
  });
}
