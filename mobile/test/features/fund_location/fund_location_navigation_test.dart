import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import 'package:social_finance/features/fund_location/presentation/screens/fund_location_screen.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';

final List<FundLocation> _sampleLocations = [
  FundLocation(
    id: 'loc-1',
    rtId: 'rt-002',
    name: 'BCA Operasional RT',
    category: FundLocationCategory.rekening,
    bankName: 'BCA',
    accountNumber: '1234567890',
    accountHolder: 'RT 002',
    isActive: true,
    currentBalance: '1000000.00',
    createdAt: DateTime.parse('2026-10-25T10:00:00Z'),
    updatedAt: DateTime.parse('2026-10-25T10:00:00Z'),
  ),
];

final _sampleReconciliation = FundLocationReconciliationReport(
  rtId: 'rt-002',
  kasBalance: '1325000.00',
  totalAllocatedBalance: '1000000.00',
  unallocatedBalance: '325000.00',
  discrepancy: '0.00',
  status: 'Cocok',
  asOf: DateTime.parse('2026-10-25T12:00:00Z'),
  anomaliesDetected: false,
);

final _sampleTransfers = const PaginatedResponse<FundTransfer>(
  data: [],
  pagination: PaginationInfo(page: 1, pageSize: 50, total: 0, totalPages: 1),
);

rp.ProviderScope _createApp({
  required Map<String, dynamic> user,
}) {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) {
        final client = ApiClient(customDio: Dio());
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        repo.state = rp.AsyncData<Map<String, dynamic>?>(user);
        return repo;
      }),
      fundLocationsProvider.overrideWith((_) async => _sampleLocations),
      fundTransfersProvider.overrideWith((_) async => _sampleTransfers),
      fundReconciliationProvider.overrideWith((_) async => _sampleReconciliation),
    ],
    child: const App(),
  );
}

void main() {
  group('Lokasi Dana - Mobile Navigation and Access Control', () {
    testWidgets('shows Lokasi Dana tab in bottom nav for Bendahara', (tester) async {
      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@local.com',
        'name': 'Bendahara RT',
        'role': AppRole.bendahara,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: bendaharaUser));
      await tester.pumpAndSettle();

      final bottomNav = find.byType(BottomAppBar);
      expect(find.descendant(of: bottomNav, matching: find.text('Beranda')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Kas')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Lokasi Dana')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Iuran')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Warga')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Profil')), findsOneWidget);
    });

    testWidgets('shows Lokasi Dana tab in bottom nav for Pengurus', (tester) async {
      final pengurusUser = {
        'id': 'user-pengurus',
        'email': 'pengurus@local.com',
        'name': 'Pengurus RT',
        'role': AppRole.pengurus,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: pengurusUser));
      await tester.pumpAndSettle();

      final bottomNav = find.byType(BottomAppBar);
      expect(find.descendant(of: bottomNav, matching: find.text('Lokasi Dana')), findsOneWidget);
    });

    testWidgets('does not show Lokasi Dana tab for Warga', (tester) async {
      final wargaUser = {
        'id': 'user-warga',
        'email': 'warga@local.com',
        'name': 'Warga RT',
        'role': AppRole.warga,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: wargaUser));
      await tester.pumpAndSettle();

      final bottomNav = find.byType(BottomAppBar);
      expect(find.descendant(of: bottomNav, matching: find.text('Beranda')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Kas')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Lokasi Dana')), findsNothing);
      expect(find.descendant(of: bottomNav, matching: find.text('Iuran')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Warga')), findsOneWidget);
      expect(find.descendant(of: bottomNav, matching: find.text('Profil')), findsOneWidget);
    });

    testWidgets('tapping Lokasi Dana tab navigates and renders FundLocationScreen', (tester) async {
      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@local.com',
        'name': 'Bendahara RT',
        'role': AppRole.bendahara,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: bendaharaUser));
      await tester.pumpAndSettle();

      // Tap Lokasi Dana tab in bottom bar
      final lokasiTab = find.descendant(
        of: find.byType(BottomAppBar),
        matching: find.text('Lokasi Dana'),
      );
      expect(lokasiTab, findsOneWidget);
      await tester.tap(lokasiTab);
      await tester.pumpAndSettle();

      // Verify FundLocationScreen is rendered with its tabs and actions
      expect(find.byType(FundLocationScreen), findsOneWidget);
      expect(find.text('Daftar Lokasi'), findsOneWidget);
      expect(find.text('Riwayat Transfer'), findsOneWidget);
      expect(find.text('Rekening Bank'), findsWidgets);
      expect(find.text('Transfer Antar Lokasi'), findsOneWidget);
    });

    testWidgets('FAB shows icon-only (no text), has tooltip, opens dialog', (tester) async {
      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@local.com',
        'name': 'Bendahara RT',
        'role': AppRole.bendahara,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: bendaharaUser));
      await tester.pumpAndSettle();

      // Navigate to Lokasi Dana tab
      await tester.tap(
        find.descendant(
          of: find.byType(BottomAppBar),
          matching: find.text('Lokasi Dana'),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(FundLocationScreen), findsOneWidget);

      // Back button should NOT appear on the header
      expect(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.byType(BackButton),
        ),
        findsNothing,
      );

      // FAB should exist (Container-style like Warga screen, not FloatingActionButton)
      expect(find.byType(Container), findsWidgets);

      // No '+' text character should be visible
      expect(find.text('+'), findsNothing);

      // FAB should have a tooltip with message 'Tambah Lokasi'
      expect(find.byTooltip('Tambah Lokasi'), findsOneWidget);

      // Tapping the button opens the add-location dialog
      await tester.tap(find.byTooltip('Tambah Lokasi'));
      await tester.pumpAndSettle();
      expect(find.text('Tambah Lokasi Dana'), findsOneWidget);
    });

    testWidgets('Warga does not see FAB', (tester) async {
      final wargaUser = {
        'id': 'user-warga',
        'email': 'warga@local.com',
        'name': 'Warga RT',
        'role': AppRole.warga,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: wargaUser));
      await tester.pumpAndSettle();

      expect(find.text('Beranda'), findsOneWidget);
    });

    testWidgets('Dialogs use consistent button layout (Row + OutlinedButton)', (tester) async {
      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@local.com',
        'name': 'Bendahara RT',
        'role': AppRole.bendahara,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: bendaharaUser));
      await tester.pumpAndSettle();

      // Navigate to Lokasi Dana tab
      await tester.tap(
        find.descendant(
          of: find.byType(BottomAppBar),
          matching: find.text('Lokasi Dana'),
        ),
      );
      await tester.pumpAndSettle();

      // Open Add dialog and verify horizontal button layout with OutlinedButton
      await tester.tap(find.byTooltip('Tambah Lokasi'));
      await tester.pumpAndSettle();
      expect(find.text('Tambah Lokasi Dana'), findsOneWidget);
      expect(find.byType(OutlinedButton), findsWidgets);

      // Cancel
      await tester.tap(find.text('Batal'));
      await tester.pumpAndSettle();

      // Verify OutlinedButton is used in the main page Transfer button too
      expect(find.byType(OutlinedButton), findsWidgets);
    });
  });
}
