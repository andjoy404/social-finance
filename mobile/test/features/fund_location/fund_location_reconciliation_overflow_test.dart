import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';

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

const _sampleTransfers = PaginatedResponse<FundTransfer>(
  data: [],
  pagination: PaginationInfo(page: 1, pageSize: 50, total: 0, totalPages: 1),
);

final Map<String, dynamic> _bendaharaUser = {
  'id': 'user-bendahara',
  'email': 'bendahara@local.com',
  'name': 'Bendahara RT',
  'role': AppRole.bendahara,
  'rt': '002',
  'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
};

Widget _buildTestApp({
  required FundLocationReconciliationReport report,
  required double width,
}) {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) {
        final client = ApiClient(customDio: Dio());
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        repo.state = rp.AsyncData<Map<String, dynamic>?>(_bendaharaUser);
        return repo;
      }),
      fundLocationsProvider.overrideWith((_) async => _sampleLocations),
      fundTransfersProvider.overrideWith((_) async => _sampleTransfers),
      fundReconciliationProvider.overrideWith((_) async => report),
    ],
    child: MaterialApp(
      home: MediaQuery(
        data: MediaQueryData(size: Size(width, 874)),
        child: Center(
          child: SizedBox(
            width: width,
            height: 874,
            child: const FundLocationScreen(),
          ),
        ),
      ),
    ),
  );
}

FundLocationReconciliationReport _makeReport(String status) {
  return FundLocationReconciliationReport(
    rtId: 'rt-002',
    kasBalance: '1000000.00',
    totalAllocatedBalance: status == 'MATCH' ? '1000000.00' : '750000.00',
    unallocatedBalance: status == 'MATCH' ? '0.00' : '250000.00',
    discrepancy: status == 'DISCREPANCY_ANOMALY' ? '50000.00' : '0.00',
    status: status,
    asOf: DateTime(2026, 11, 25, 12, 0, 0),
    anomaliesDetected: status == 'DISCREPANCY_ANOMALY',
  );
}

void main() {
  group('FundLocationScreen — _ReconciliationCard header overflow', () {
    for (final entry in const [
      ('MATCH', 'Cocok Sempurna'),
      ('MATCH_WITH_NOTES', 'Sesuai Catatan'),
      ('DISCREPANCY_ANOMALY', 'Selisih / Anomali'),
    ]) {
      final status = entry.$1;
      final expectedLabel = entry.$2;

      testWidgets(
        'does not overflow at 402 px screen width (343 px header constraint) for $status',
        (tester) async {
          await tester.binding.setSurfaceSize(const Size(402, 874));
          addTearDown(() => tester.binding.setSurfaceSize(null));

          await tester.pumpWidget(
            _buildTestApp(report: _makeReport(status), width: 402),
          );
          await tester.pumpAndSettle();

          expect(tester.takeException(), isNull);

          final headingFinder = find.text('Status Rekonsiliasi Kas');
          final statusFinder = find.text(expectedLabel);
          final chevronFinder = find.byIcon(Icons.expand_more);

          expect(headingFinder, findsOneWidget);
          expect(statusFinder, findsOneWidget);
          expect(chevronFinder, findsOneWidget);

          // Confirm the outer header Row matches the 343 px width constraint
          // (402 - 32 ListView padding - 24 Container padding - 3 border = 343).
          final headerRowFinder = find
              .ancestor(of: headingFinder, matching: find.byType(Row))
              .last;
          final headerRowSize = tester.getSize(headerRowFinder);
          expect(headerRowSize.width, closeTo(343.0, 0.5));

          // Verify heading, status badge, and chevron are all within the header bounds
          final headerRect = tester.getRect(headerRowFinder);
          expect(
            tester.getRect(headingFinder).right,
            lessThanOrEqualTo(headerRect.right + 0.1),
          );
          expect(
            tester.getRect(statusFinder).right,
            lessThanOrEqualTo(headerRect.right + 0.1),
          );
          expect(
            tester.getRect(chevronFinder).right,
            lessThanOrEqualTo(headerRect.right + 0.1),
          );
        },
      );
    }

    testWidgets(
      'tapping reconciliation header toggles chevron behavior',
      (tester) async {
        // Use a wider surface so the unrelated 4-column metrics row inside
        // _buildExpandedContent does not overflow under flutter_test's
        // wide square-glyph Ahem font when expanded.
        await tester.binding.setSurfaceSize(const Size(600, 874));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        await tester.pumpWidget(
          _buildTestApp(report: _makeReport('MATCH'), width: 600),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byIcon(Icons.expand_more), findsOneWidget);
        expect(find.byIcon(Icons.expand_less), findsNothing);

        await tester.tap(find.text('Status Rekonsiliasi Kas'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byIcon(Icons.expand_less), findsOneWidget);
      },
    );

    testWidgets(
      'does not overflow at narrower 360 px screen width (301 px header constraint)',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(360, 800));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        await tester.pumpWidget(
          _buildTestApp(
            report: _makeReport('DISCREPANCY_ANOMALY'),
            width: 360,
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.text('Status Rekonsiliasi Kas'), findsOneWidget);
        expect(find.text('Selisih / Anomali'), findsOneWidget);
        expect(find.byIcon(Icons.expand_more), findsOneWidget);
      },
    );
  });
}
