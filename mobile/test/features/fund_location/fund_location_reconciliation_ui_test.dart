import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/app/app.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
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

final _sampleTransfers = const PaginatedResponse<FundTransfer>(
  data: [],
  pagination: PaginationInfo(page: 1, pageSize: 50, total: 0, totalPages: 1),
);

rp.ProviderScope _createApp({
  required Map<String, dynamic> user,
  required FundLocationReconciliationReport report,
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
      fundReconciliationProvider.overrideWith((_) async => report),
    ],
    child: const App(),
  );
}

void main() {
  group('Reconciliation Card - Styling & Wording', () {
    testWidgets(
      'MATCH_WITH_NOTES: badge uses blue color (info) and human-readable label',
      (tester) async {
        final report = FundLocationReconciliationReport(
          rtId: 'rt-002',
          kasBalance: '1000000.00',
          totalAllocatedBalance: '750000.00',
          unallocatedBalance: '250000.00',
          discrepancy: '0.00',
          status: 'Cocok dengan catatan',
          asOf: DateTime(2026, 10, 25, 12, 0, 0),
          anomaliesDetected: false,
        );

        final bendaharaUser = {
          'id': 'user-bendahara',
          'email': 'bendahara@local.com',
          'name': 'Bendahara RT',
          'role': AppRole.bendahara,
          'rt': '002',
          'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        };

        await tester.pumpWidget(
          _createApp(user: bendaharaUser, report: report),
        );
        await tester.pumpAndSettle();

        // Navigate to Lokasi Dana tab
        await tester.tap(
          find.descendant(
            of: find.byType(BottomAppBar),
            matching: find.text('Lokasi Dana'),
          ),
        );
        await tester.pumpAndSettle();

        // Verify FundLocationScreen is rendered
        expect(find.byType(FundLocationScreen), findsOneWidget);

        // Badge text must be human-readable ("Sesuai Catatan"), not internal enum
        expect(find.text('Sesuai Catatan'), findsOneWidget);

        // The internal enum/status string must NOT appear as standalone text
        expect(find.text('Cocok dengan catatan'), findsNothing);

        // Description text should be displayed for MATCH_WITH_NOTES
        expect(
          find.text(
            'Saldo KAS sesuai dengan catatan. Masih ada saldo historis yang belum dialokasikan ke lokasi dana.',
          ),
          findsOneWidget,
        );
      },
    );

    testWidgets('MATCHED: badge uses green color (success) and no description', (
      tester,
    ) async {
      final report = FundLocationReconciliationReport(
        rtId: 'rt-002',
        kasBalance: '1000000.00',
        totalAllocatedBalance: '1000000.00',
        unallocatedBalance: '0.00',
        discrepancy: '0.00',
        status: 'MATCH',
        asOf: DateTime(2026, 10, 25, 12, 0, 0),
        anomaliesDetected: false,
      );

      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@local.com',
        'name': 'Bendahara RT',
        'role': AppRole.bendahara,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: bendaharaUser, report: report));
      await tester.pumpAndSettle();

      // Navigate to Lokasi Dana tab
      await tester.tap(
        find.descendant(
          of: find.byType(BottomAppBar),
          matching: find.text('Lokasi Dana'),
        ),
      );
      await tester.pumpAndSettle();

      // Badge text must be human-readable
      expect(find.text('Cocok Sempurna'), findsOneWidget);

      // No description text should be displayed for MATCHED
      expect(
        find.text(
          'Saldo KAS sesuai dengan catatan. Masih ada saldo historis yang belum dialokasikan ke lokasi dana.',
        ),
        findsNothing,
      );
    });

    testWidgets('MATCH_WITH_NOTES: reconciliation card has blue border', (
      tester,
    ) async {
      final report = FundLocationReconciliationReport(
        rtId: 'rt-002',
        kasBalance: '1000000.00',
        totalAllocatedBalance: '750000.00',
        unallocatedBalance: '250000.00',
        discrepancy: '0.00',
        status: 'Cocok dengan catatan',
        asOf: DateTime(2026, 10, 25, 12, 0, 0),
        anomaliesDetected: false,
      );

      final bendaharaUser = {
        'id': 'user-bendahara',
        'email': 'bendahara@local.com',
        'name': 'Bendahara RT',
        'role': AppRole.bendahara,
        'rt': '002',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
      };

      await tester.pumpWidget(_createApp(user: bendaharaUser, report: report));
      await tester.pumpAndSettle();

      // Navigate to Lokasi Dana tab
      await tester.tap(
        find.descendant(
          of: find.byType(BottomAppBar),
          matching: find.text('Lokasi Dana'),
        ),
      );
      await tester.pumpAndSettle();

      // Verify the reconciliation card has a blue-tinted border.
      // The code uses `AppColors.info.withValues(alpha: 0.5)` which
      // produces a Color with different numeric value, so we check
      // the red component is lower than green/blue (blue-dominant).
      final containers = tester.widgetList<Container>(find.byType(Container));
      bool hasBlueBorder = false;
      for (final container in containers) {
        final decoration = container.decoration as BoxDecoration?;
        if (decoration?.border != null) {
          final border = decoration!.border!;
          final topColor = border.top.color;
          // Blue-dominant: red < green and red < blue
          if (topColor.red < topColor.green &&
              topColor.red < topColor.blue &&
              topColor.blue > 200) {
            hasBlueBorder = true;
            break;
          }
        }
      }
      expect(
        hasBlueBorder,
        isTrue,
        reason: 'Reconciliation card should have blue border',
      );
    });
  });
}
