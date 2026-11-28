import 'dart:async';
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

final List<FundLocation> _mockLocations = [
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

const _emptyTransfers = PaginatedResponse<FundTransfer>(
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

class _TrackingFundLocationRepository extends FundLocationRepository {
  int createLocationCalls = 0;
  int updateLocationCalls = 0;
  int createTransferCalls = 0;

  Completer<FundLocation>? createLocationCompleter;
  Completer<FundLocation>? updateLocationCompleter;
  Completer<FundTransfer>? createTransferCompleter;

  _TrackingFundLocationRepository()
      : super(apiClient: FundLocationApiClient(dio: Dio()));

  @override
  Future<FundLocation> createLocation({
    required String name,
    required String category,
    String? bankName,
    String? accountNumber,
    String? accountHolder,
  }) async {
    createLocationCalls++;
    if (createLocationCompleter != null) {
      return createLocationCompleter!.future;
    }
    return FundLocation(
      id: 'loc-created',
      rtId: 'rt-002',
      name: name,
      category: category == 'rekening'
          ? FundLocationCategory.rekening
          : FundLocationCategory.coh,
      bankName: bankName,
      accountNumber: accountNumber,
      accountHolder: accountHolder,
      isActive: true,
      currentBalance: '0.00',
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }

  @override
  Future<FundLocation> updateLocation({
    required String id,
    String? name,
    String? bankName,
    String? accountNumber,
    String? accountHolder,
    bool? isActive,
  }) async {
    updateLocationCalls++;
    if (updateLocationCompleter != null) {
      return updateLocationCompleter!.future;
    }
    return FundLocation(
      id: id,
      rtId: 'rt-002',
      name: name ?? 'Location',
      category: FundLocationCategory.rekening,
      bankName: bankName,
      accountNumber: accountNumber,
      accountHolder: accountHolder,
      isActive: isActive ?? true,
      currentBalance: '2500000.00',
      createdAt: DateTime.now(),
      updatedAt: DateTime.now(),
    );
  }

  @override
  Future<FundTransfer> createTransfer({
    required String fromLocationId,
    required String toLocationId,
    required int amountRupiah,
    String? notes,
    String? idempotencyKey,
  }) async {
    createTransferCalls++;
    if (createTransferCompleter != null) {
      return createTransferCompleter!.future;
    }
    return FundTransfer(
      id: 'tr-new',
      rtId: 'rt-002',
      fromLocationId: fromLocationId,
      fromLocationName: 'BCA Operasional RT 002',
      toLocationId: toLocationId,
      toLocationName: 'Kas Di Tangan Bendahara',
      amount: '$amountRupiah.00',
      transferDate: DateTime.now(),
      notes: notes,
      createdAt: DateTime.now(),
    );
  }
}

Widget _buildScreen({
  required _TrackingFundLocationRepository repo,
  List<FundLocation>? locations,
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
      fundLocationsProvider.overrideWith((_) async => locations ?? _mockLocations),
      fundTransfersProvider.overrideWith((_) async => _emptyTransfers),
      fundReconciliationProvider.overrideWith(
        (_) => Future<FundLocationReconciliationReport>.error('omitted-in-test'),
      ),
    ],
    child: const MaterialApp(
      home: FundLocationScreen(),
    ),
  );
}

void main() {
  group('FundLocationScreen — Dirty Navigation Guard', () {
    testWidgets('Tambah Lokasi Dana: Clean state exits directly with Batal', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Add Location dialog
      await tester.tap(find.byIcon(Icons.add));
      await tester.pumpAndSettle();

      expect(find.text('Tambah Lokasi Dana'), findsOneWidget);

      // Clean state: tap Batal
      await tester.tap(find.byKey(const Key('fund-add-cancel-btn')));
      await tester.pumpAndSettle();

      // Directly closes without confirmation dialog
      expect(find.text('Tambah Lokasi Dana'), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });

    testWidgets('Tambah Lokasi Dana: Clean state exits directly on backdrop tap', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Add Location dialog
      await tester.tap(find.byIcon(Icons.add));
      await tester.pumpAndSettle();

      expect(find.text('Tambah Lokasi Dana'), findsOneWidget);

      // Tap backdrop outside the dialog
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      expect(find.text('Tambah Lokasi Dana'), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });

    testWidgets('Tambah Lokasi Dana: Dirty state shows confirmation and Tetap di Halaman preserves input', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Add Location dialog
      await tester.tap(find.byIcon(Icons.add));
      await tester.pumpAndSettle();

      // Enter name (makes it dirty)
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'Bank Baru RT');
      await tester.pumpAndSettle();

      // Tap Batal
      await tester.tap(find.byKey(const Key('fund-add-cancel-btn')));
      await tester.pumpAndSettle();

      // Confirmation dialog must appear
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);

      // Tap Tetap di Halaman
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();

      // Confirmation closes, form remains with input preserved
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.text('Tambah Lokasi Dana'), findsOneWidget);
      expect(find.text('Bank Baru RT'), findsOneWidget);
      expect(repo.createLocationCalls, equals(0));
    });

    testWidgets('Tambah Lokasi Dana: Dirty state Ya Batalkan exits without saving', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Add Location dialog
      await tester.tap(find.byIcon(Icons.add));
      await tester.pumpAndSettle();

      // Enter name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'Bank Baru RT');
      await tester.pumpAndSettle();

      // Tap Batal
      await tester.tap(find.byKey(const Key('fund-add-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap Ya, Batalkan
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();

      // Both dialogs closed, no repo call
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.text('Tambah Lokasi Dana'), findsNothing);
      expect(repo.createLocationCalls, equals(0));
    });

    testWidgets('Tambah Lokasi Dana: Dirty state backdrop tap triggers confirmation', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Add Location dialog
      await tester.tap(find.byIcon(Icons.add));
      await tester.pumpAndSettle();

      // Enter name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'Bank Baru RT');
      await tester.pumpAndSettle();

      // Tap backdrop
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      // Confirmation dialog must appear
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Confirm cancel to clean up
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();
      expect(find.text('Tambah Lokasi Dana'), findsNothing);
    });

    testWidgets('Tambah Lokasi Dana: Saving in flight blocks dismissal and double submit', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      repo.createLocationCompleter = Completer<FundLocation>();

      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Add Location dialog
      await tester.tap(find.byIcon(Icons.add));
      await tester.pumpAndSettle();

      // Enter name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'Bank Baru RT');
      await tester.pumpAndSettle();

      // Tap Simpan
      await tester.tap(find.text('Simpan'));
      await tester.pump(); // don't settle because completer is pending

      expect(repo.createLocationCalls, equals(1));

      // Attempt to tap Batal while saving
      final cancelBtn = tester.widget<ElevatedButton>(find.byKey(const Key('fund-add-cancel-btn')));
      expect(cancelBtn.onPressed, isNull);

      // Attempt backdrop tap while saving
      await tester.tapAt(const Offset(10, 10));
      await tester.pump();

      // Dialog should still be open
      expect(find.text('Tambah Lokasi Dana'), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);

      // Complete save
      repo.createLocationCompleter!.complete(FundLocation(
        id: 'loc-new',
        rtId: 'rt-002',
        name: 'Bank Baru RT',
        category: FundLocationCategory.rekening,
        isActive: true,
        currentBalance: '0.00',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ));
      await tester.pumpAndSettle();

      // After successful save, dialog closes
      expect(find.text('Tambah Lokasi Dana'), findsNothing);
    });

    testWidgets('Ubah Lokasi Dana: Clean state exits directly with Batal', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open popup menu for first location
      await tester.tap(find.byIcon(Icons.more_vert).first);
      await tester.pumpAndSettle();

      // Tap Ubah Lokasi
      await tester.tap(find.text('Ubah Lokasi'));
      await tester.pumpAndSettle();

      expect(find.text('Ubah Lokasi Dana'), findsOneWidget);

      // Clean state: tap Batal
      await tester.tap(find.byKey(const Key('fund-edit-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Ubah Lokasi Dana'), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });

    testWidgets('Ubah Lokasi Dana: Clean state exits directly on backdrop tap', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open popup menu for first location
      await tester.tap(find.byIcon(Icons.more_vert).first);
      await tester.pumpAndSettle();

      // Tap Ubah Lokasi
      await tester.tap(find.text('Ubah Lokasi'));
      await tester.pumpAndSettle();

      expect(find.text('Ubah Lokasi Dana'), findsOneWidget);

      // Tap backdrop
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      expect(find.text('Ubah Lokasi Dana'), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });

    testWidgets('Ubah Lokasi Dana: Dirty state shows confirmation and Tetap di Halaman preserves changes', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Ubah dialog
      await tester.tap(find.byIcon(Icons.more_vert).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ubah Lokasi'));
      await tester.pumpAndSettle();

      expect(find.text('Ubah Lokasi Dana'), findsOneWidget);

      // Modify name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'BCA Operasional RT 002 Edited');
      await tester.pumpAndSettle();

      // Tap Batal
      await tester.tap(find.byKey(const Key('fund-edit-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap Tetap di Halaman
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.text('Ubah Lokasi Dana'), findsOneWidget);
      expect(find.text('BCA Operasional RT 002 Edited'), findsOneWidget);
      expect(repo.updateLocationCalls, equals(0));
    });

    testWidgets('Ubah Lokasi Dana: Dirty state Ya Batalkan exits without saving', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Ubah dialog
      await tester.tap(find.byIcon(Icons.more_vert).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ubah Lokasi'));
      await tester.pumpAndSettle();

      // Modify name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'BCA Operasional RT 002 Edited');
      await tester.pumpAndSettle();

      // Tap Batal
      await tester.tap(find.byKey(const Key('fund-edit-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap Ya, Batalkan
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.text('Ubah Lokasi Dana'), findsNothing);
      expect(repo.updateLocationCalls, equals(0));
    });

    testWidgets('Ubah Lokasi Dana: Dirty state backdrop tap triggers confirmation', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Ubah dialog
      await tester.tap(find.byIcon(Icons.more_vert).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ubah Lokasi'));
      await tester.pumpAndSettle();

      // Modify name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'BCA Operasional RT 002 Edited');
      await tester.pumpAndSettle();

      // Tap backdrop
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Confirm cancel to clean up
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();
      expect(find.text('Ubah Lokasi Dana'), findsNothing);
    });

    testWidgets('Ubah Lokasi Dana: Saving in flight blocks dismissal and double submit', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      repo.updateLocationCompleter = Completer<FundLocation>();

      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Ubah dialog
      await tester.tap(find.byIcon(Icons.more_vert).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ubah Lokasi'));
      await tester.pumpAndSettle();

      // Modify name
      final nameField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(nameField, 'BCA Operasional RT 002 Edited');
      await tester.pumpAndSettle();

      // Tap Simpan
      await tester.tap(find.text('Simpan'));
      await tester.pump();

      expect(repo.updateLocationCalls, equals(1));

      // Attempt to tap Batal while saving
      final cancelBtn = tester.widget<ElevatedButton>(find.byKey(const Key('fund-edit-cancel-btn')));
      expect(cancelBtn.onPressed, isNull);

      // Attempt backdrop tap while saving
      await tester.tapAt(const Offset(10, 10));
      await tester.pump();

      // Dialog should still be open
      expect(find.text('Ubah Lokasi Dana'), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);

      // Complete save
      repo.updateLocationCompleter!.complete(FundLocation(
        id: 'loc-1',
        rtId: 'rt-002',
        name: 'BCA Operasional RT 002 Edited',
        category: FundLocationCategory.rekening,
        isActive: true,
        currentBalance: '2500000.00',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ));
      await tester.pumpAndSettle();

      // After successful save, dialog closes
      expect(find.text('Ubah Lokasi Dana'), findsNothing);
    });

    testWidgets('Transfer Antar Lokasi: Clean state exits directly with Batal', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Transfer dialog
      await tester.tap(find.text('Transfer Antar Lokasi'));
      await tester.pumpAndSettle();

      expect(find.text('Transfer Antar Lokasi'), findsWidgets);

      // Clean state: tap Batal
      await tester.tap(find.byKey(const Key('fund-transfer-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });

    testWidgets('Transfer Antar Lokasi: Clean state exits directly on backdrop tap', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Transfer dialog
      await tester.tap(find.text('Transfer Antar Lokasi'));
      await tester.pumpAndSettle();

      expect(find.byType(AlertDialog), findsOneWidget);

      // Tap backdrop
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
    });

    testWidgets('Transfer Antar Lokasi: Dirty state shows confirmation and Tetap di Halaman preserves input', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Transfer dialog
      await tester.tap(find.text('Transfer Antar Lokasi'));
      await tester.pumpAndSettle();

      // Enter amount
      final amountField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(amountField, '150000');
      await tester.pumpAndSettle();

      // Tap Batal
      await tester.tap(find.byKey(const Key('fund-transfer-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);

      // Tap Tetap di Halaman
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(
        tester.widget<TextField>(amountField).controller?.text.isNotEmpty,
        isTrue,
      );
      expect(repo.createTransferCalls, equals(0));
    });

    testWidgets('Transfer Antar Lokasi: Dirty state Ya Batalkan exits without saving', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Transfer dialog
      await tester.tap(find.text('Transfer Antar Lokasi'));
      await tester.pumpAndSettle();

      // Enter amount
      final amountField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(amountField, '150000');
      await tester.pumpAndSettle();

      // Tap Batal
      await tester.tap(find.byKey(const Key('fund-transfer-cancel-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Tap Ya, Batalkan
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);
      expect(find.byType(AlertDialog), findsNothing);
      expect(repo.createTransferCalls, equals(0));
    });

    testWidgets('Transfer Antar Lokasi: Dirty state backdrop tap triggers confirmation', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Transfer dialog
      await tester.tap(find.text('Transfer Antar Lokasi'));
      await tester.pumpAndSettle();

      // Enter amount
      final amountField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(amountField, '150000');
      await tester.pumpAndSettle();

      // Tap backdrop
      await tester.tapAt(const Offset(10, 10));
      await tester.pumpAndSettle();

      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      // Confirm cancel to clean up
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets('Transfer Antar Lokasi: Saving in flight blocks dismissal and double submit', (tester) async {
      await tester.binding.setSurfaceSize(const Size(400, 900));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repo = _TrackingFundLocationRepository();
      repo.createTransferCompleter = Completer<FundTransfer>();

      await tester.pumpWidget(_buildScreen(repo: repo));
      await tester.pumpAndSettle();

      // Open Transfer dialog
      await tester.tap(find.text('Transfer Antar Lokasi'));
      await tester.pumpAndSettle();

      // Enter amount
      final amountField = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.byType(TextField),
      ).first;
      await tester.enterText(amountField, '150000');
      await tester.pumpAndSettle();

      // Tap Transfer
      await tester.tap(find.text('Transfer'));
      await tester.pump();

      expect(repo.createTransferCalls, equals(1));

      // Attempt Batal while saving
      final cancelBtn = tester.widget<ElevatedButton>(find.byKey(const Key('fund-transfer-cancel-btn')));
      expect(cancelBtn.onPressed, isNull);

      // Attempt backdrop tap while saving
      await tester.tapAt(const Offset(10, 10));
      await tester.pump();

      // Dialog should still be open
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);

      // Complete transfer
      repo.createTransferCompleter!.complete(FundTransfer(
        id: 'tr-done',
        rtId: 'rt-002',
        fromLocationId: 'loc-1',
        fromLocationName: 'BCA Operasional RT 002',
        toLocationId: 'loc-2',
        toLocationName: 'Kas Di Tangan Bendahara',
        amount: '150000.00',
        transferDate: DateTime.now(),
        createdAt: DateTime.now(),
      ));
      await tester.pumpAndSettle();

      // Closes on completion
      expect(find.byType(AlertDialog), findsNothing);
    });
  });
}
