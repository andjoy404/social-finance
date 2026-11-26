import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/data/fund_location_api_client.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import 'package:social_finance/features/fund_location/presentation/screens/fund_location_screen.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';

final List<FundLocation> _twoActiveLocations = [
  FundLocation(
    id: 'loc-1',
    rtId: 'rt-002',
    name: 'BCA Operasional RT 002',
    category: FundLocationCategory.rekening,
    bankName: 'BCA',
    accountNumber: '1234567890',
    accountHolder: 'RT 002',
    isActive: true,
    currentBalance: '2500000.00',
    createdAt: DateTime.parse('2026-10-25T10:00:00Z'),
    updatedAt: DateTime.parse('2026-10-25T10:00:00Z'),
  ),
  FundLocation(
    id: 'loc-2',
    rtId: 'rt-002',
    name: 'Kas Di Tangan Bendahara',
    category: FundLocationCategory.coh,
    isActive: true,
    currentBalance: '500000.00',
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

class _FakeFundLocationRepository extends FundLocationRepository {
  int createTransferCalls = 0;
  String? lastFromId;
  String? lastToId;
  int? lastAmount;

  _FakeFundLocationRepository()
    : super(apiClient: FundLocationApiClient(dio: Dio()));

  @override
  Future<FundTransfer> createTransfer({
    required String fromLocationId,
    required String toLocationId,
    required int amountRupiah,
    String? notes,
    String? idempotencyKey,
  }) async {
    createTransferCalls++;
    lastFromId = fromLocationId;
    lastToId = toLocationId;
    lastAmount = amountRupiah;
    return FundTransfer(
      id: 'tr-1',
      rtId: 'rt-002',
      fromLocationId: fromLocationId,
      fromLocationName: 'BCA Operasional RT 002',
      toLocationId: toLocationId,
      toLocationName: 'Kas Di Tangan Bendahara',
      amount: '$amountRupiah.00',
      transferDate: DateTime.parse('2026-11-26T00:00:00Z'),
      notes: notes,
      createdAt: DateTime.parse('2026-11-26T00:00:00Z'),
    );
  }
}

Widget _buildScreen({
  required _FakeFundLocationRepository repo,
  required TargetPlatform platform,
}) {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith((_) {
        final client = ApiClient(customDio: Dio());
        final authRepo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
        );
        authRepo.state = rp.AsyncData<Map<String, dynamic>?>(_bendaharaUser);
        return authRepo;
      }),
      fundLocationRepositoryProvider.overrideWithValue(repo),
      fundLocationsProvider.overrideWith((_) async => _twoActiveLocations),
      fundTransfersProvider.overrideWith((_) async => _sampleTransfers),
      // Omit the unrelated reconciliation card so this test isolates Transfer
      // Antar Lokasi dialog layout & behavior across both HEAD and WIP trees.
      fundReconciliationProvider.overrideWith(
        (_) => Future<FundLocationReconciliationReport>.error(
          'omitted-in-transfer-dialog-test',
        ),
      ),
    ],
    child: MaterialApp(
      theme: ThemeData(platform: platform),
      home: const FundLocationScreen(),
    ),
  );
}

void main() {
  group('FundLocationScreen — Transfer Antar Lokasi Dialog (iOS)', () {
    testWidgets(
      'opens Transfer Antar Lokasi dialog cleanly on iOS, validates, and closes with Batal',
      variant: TargetPlatformVariant.only(TargetPlatform.iOS),
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(390, 844));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        final fakeRepo = _FakeFundLocationRepository();
        await tester.pumpWidget(
          _buildScreen(repo: fakeRepo, platform: TargetPlatform.iOS),
        );
        await tester.pumpAndSettle();

        expect(find.text('Transfer Antar Lokasi'), findsOneWidget);
        await tester.tap(find.text('Transfer Antar Lokasi'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(find.text('Dari Lokasi'), findsOneWidget);
        expect(find.text('Ke Lokasi Tujuan'), findsOneWidget);
        expect(find.text('Nominal Transfer'), findsOneWidget);
        expect(find.text('Batal'), findsOneWidget);

        // Validate empty amount shows error inside dialog
        await tester.tap(find.text('Transfer'));
        await tester.pumpAndSettle();
        expect(
          find.text('Nominal transfer harus lebih dari 0.'),
          findsOneWidget,
        );

        // Cancel closes dialog cleanly
        await tester.tap(find.text('Batal'));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsNothing);
      },
    );

    testWidgets(
      'submits valid transfer on iOS and calls repository createTransfer',
      variant: TargetPlatformVariant.only(TargetPlatform.iOS),
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(390, 844));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        final fakeRepo = _FakeFundLocationRepository();
        await tester.pumpWidget(
          _buildScreen(repo: fakeRepo, platform: TargetPlatform.iOS),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.text('Transfer Antar Lokasi'));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        // Enter valid amount
        final amountField = find.descendant(
          of: find.byType(AlertDialog),
          matching: find.byType(TextField),
        );
        await tester.enterText(amountField, '150000');
        await tester.pumpAndSettle();

        await tester.tap(find.text('Transfer'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(fakeRepo.createTransferCalls, equals(1));
        expect(fakeRepo.lastFromId, equals('loc-1'));
        expect(fakeRepo.lastToId, equals('loc-2'));
        expect(fakeRepo.lastAmount, equals(150000));
        expect(find.byType(AlertDialog), findsNothing);
      },
    );

    testWidgets(
      'preserves Transfer Antar Lokasi dialog behavior on Android without regression',
      variant: TargetPlatformVariant.only(TargetPlatform.android),
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(390, 844));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        final fakeRepo = _FakeFundLocationRepository();
        await tester.pumpWidget(
          _buildScreen(repo: fakeRepo, platform: TargetPlatform.android),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.text('Transfer Antar Lokasi'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byType(AlertDialog), findsOneWidget);
        await tester.tap(find.text('Batal'));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsNothing);
      },
    );
  });
}
