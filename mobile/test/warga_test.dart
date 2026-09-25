import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/warga/data/mock_warga_data.dart';
import 'package:social_finance/features/warga/presentation/screens/warga_screen.dart';

Widget _createWargaTestApp({
  List<ResidentItem>? residentsOverride,
  String initialQuery = '',
}) {
  return ProviderScope(
    overrides: [
      if (residentsOverride != null)
        wargaListProvider.overrideWithValue(residentsOverride),
      wargaSearchQueryProvider.overrideWith((ref) => initialQuery),
    ],
    child: const MaterialApp(home: WargaScreen()),
  );
}

Widget _createAuthenticatedApp() {
  final client = ApiClient(baseUrl: 'http://test.local');
  final repo = AuthRepository(
    authService: AuthService(client),
    apiClient: client,
  );
  repo.state = const AsyncData({
    'name': 'Heri Prastyo',
    'role': AppRole.bendahara,
    'rt': '002',
    'rw': '016',
  });

  return ProviderScope(
    overrides: [authRepositoryProvider.overrideWith((ref) => repo)],
    child: const App(),
  );
}

void main() {
  group('ResidentItem Model', () {
    test(
      'maskedNik masks middle digits correctly for standard 16-digit NIK',
      () {
        const resident = ResidentItem(
          id: 'res-1',
          name: 'Test Resident',
          nik: '3201012304750001',
          houseNumber: 'A1',
          isHeadOfHousehold: true,
          relationship: 'Kepala Keluarga',
        );

        expect(resident.maskedNik, equals('320101******0001'));
      },
    );

    test('maskedNik handles short NIK safely', () {
      const resident = ResidentItem(
        id: 'res-2',
        name: 'Test Short',
        nik: '12345',
        houseNumber: 'A2',
        isHeadOfHousehold: false,
        relationship: 'Anak',
      );

      expect(resident.maskedNik, equals('***'));
    });
  });

  group('Warga Screen Rendering & Search', () {
    testWidgets(
      'Warga screen renders with title, description, and search field',
      (tester) async {
        await tester.pumpWidget(_createWargaTestApp());
        await tester.pumpAndSettle();

        expect(find.text('Warga'), findsOneWidget);
        expect(
          find.text('Daftar warga dan kepala keluarga di lingkungan RT.'),
          findsOneWidget,
        );
        expect(find.byType(TextField), findsOneWidget);
        expect(
          find.textContaining(
            'Menampilkan ${WargaMockData.residents.length} warga',
          ),
          findsOneWidget,
        );
      },
    );

    testWidgets('Warga screen displays mock residents with details', (
      tester,
    ) async {
      await tester.pumpWidget(_createWargaTestApp());
      await tester.pumpAndSettle();

      expect(find.text('Bambang Sutrisno'), findsOneWidget);
      expect(find.text('Blok A1 No. 12'), findsWidgets);
      expect(find.text('NIK: 320101******0001'), findsOneWidget);
      expect(find.text('Kepala Keluarga'), findsWidgets);
      expect(find.text('081234567890'), findsOneWidget);
    });

    testWidgets('Search filters residents by name', (tester) async {
      await tester.pumpWidget(_createWargaTestApp());
      await tester.pumpAndSettle();

      // Enter search text
      await tester.enterText(find.byType(TextField), 'Bambang');
      await tester.pumpAndSettle();

      expect(find.text('Bambang Sutrisno'), findsOneWidget);
      expect(find.text('Siti Aminah'), findsNothing);
      expect(find.text('Menampilkan 1 warga'), findsOneWidget);
    });

    testWidgets('Search filters residents by NIK', (tester) async {
      await tester.pumpWidget(_createWargaTestApp());
      await tester.pumpAndSettle();

      // Search by NIK suffix '0006' (Ahmad Fauzi)
      await tester.enterText(find.byType(TextField), '0006');
      await tester.pumpAndSettle();

      expect(find.text('Ahmad Fauzi'), findsOneWidget);
      expect(find.text('Bambang Sutrisno'), findsNothing);
    });

    testWidgets('Search filters residents by house number', (tester) async {
      await tester.pumpWidget(_createWargaTestApp());
      await tester.pumpAndSettle();

      // Search by house number 'Blok B2'
      await tester.enterText(find.byType(TextField), 'Blok B2');
      await tester.pumpAndSettle();

      expect(find.text('Heri Prastyo'), findsOneWidget);
      expect(find.text('Ratna Dewi'), findsOneWidget);
      expect(find.text('Bambang Sutrisno'), findsNothing);
      expect(find.text('Menampilkan 2 warga'), findsOneWidget);
    });

    testWidgets('Empty search result displays empty state with reset button', (
      tester,
    ) async {
      await tester.pumpWidget(_createWargaTestApp());
      await tester.pumpAndSettle();

      // Enter query matching no one
      await tester.enterText(find.byType(TextField), 'NonExistentPerson');
      await tester.pumpAndSettle();

      expect(find.text('Warga Tidak Ditemukan'), findsOneWidget);
      expect(
        find.text(
          'Tidak ada data warga yang sesuai dengan "NonExistentPerson".',
        ),
        findsOneWidget,
      );
      expect(find.text('Hapus Pencarian'), findsOneWidget);

      // Tap reset button
      await tester.tap(find.text('Hapus Pencarian'));
      await tester.pumpAndSettle();

      // All mock residents should be displayed again
      expect(find.text('Bambang Sutrisno'), findsOneWidget);
      expect(
        find.textContaining(
          'Menampilkan ${WargaMockData.residents.length} warga',
        ),
        findsOneWidget,
      );
    });

    testWidgets('Empty resident list displays empty state', (tester) async {
      await tester.pumpWidget(_createWargaTestApp(residentsOverride: []));
      await tester.pumpAndSettle();

      expect(find.text('Belum Ada Warga'), findsOneWidget);
      expect(
        find.text('Data kependudukan warga RT belum tersedia.'),
        findsOneWidget,
      );
    });
  });

  group('Warga Navigation Flow', () {
    testWidgets('Tapping Warga bottom navigation tab opens Warga screen', (
      tester,
    ) async {
      await tester.pumpWidget(_createAuthenticatedApp());
      await tester.pump(const Duration(milliseconds: 100));
      await tester.pump();

      // Initial tab is Beranda
      expect(find.text('Selamat datang,'), findsOneWidget);

      // Verify Warga tab item exists
      final wargaTab = find.text('Warga');
      expect(wargaTab, findsOneWidget);

      // Tap Warga tab
      await tester.tap(wargaTab);
      await tester.pumpAndSettle();

      // Warga screen is now active
      expect(
        find.text('Daftar warga dan kepala keluarga di lingkungan RT.'),
        findsOneWidget,
      );
      expect(find.text('Bambang Sutrisno'), findsOneWidget);
    });
  });
}
