import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_detail_screen.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';

class MockFundLocationRepository extends Fake implements FundLocationRepository {
  final List<FundLocation>? customLocations;

  MockFundLocationRepository([this.customLocations]);

  @override
  Future<List<FundLocation>> fetchLocations({String? category, bool? isActive}) async {
    if (customLocations != null) return customLocations!;
    return [
      FundLocation(
        id: 'loc-1',
        rtId: 'rt-1',
        name: 'Rekening BCA',
        category: FundLocationCategory.rekening,
        bankName: 'BCA',
        accountNumber: '1234567890',
        isActive: true,
        currentBalance: '1000000.00',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ),
      FundLocation(
        id: 'loc-2',
        rtId: 'rt-1',
        name: 'Brankas Sekretariat RT',
        category: FundLocationCategory.brankas,
        isActive: true,
        currentBalance: '500000.00',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ),
      FundLocation(
        id: 'loc-3',
        rtId: 'rt-1',
        name: 'Kas Di Tangan Bendahara',
        category: FundLocationCategory.coh,
        isActive: true,
        currentBalance: '200000.00',
        createdAt: DateTime.now(),
        updatedAt: DateTime.now(),
      ),
    ];
  }
}

class MockIuranRepository extends Fake implements IuranRepository {
  String? lastVerifiedId;
  String? lastAction;
  String? lastRejectionReason;
  String? lastOfficerNotes;
  String? lastLocationId;
  String? lastActualReceivedDate;

  @override
  Future<BackendPayment> verifyPayment(
    String paymentId, {
    required String action,
    String? rejectionReason,
    String? officerNotes,
    String? locationId,
    String? actualReceivedDate,
  }) async {
    lastVerifiedId = paymentId;
    lastAction = action;
    lastRejectionReason = rejectionReason;
    lastOfficerNotes = officerNotes;
    lastLocationId = locationId;
    lastActualReceivedDate = actualReceivedDate;

    return BackendPayment(
      id: paymentId,
      rtId: 'rt-1',
      billId: 'bill-1',
      amount: '50000.00',
      paidAt: DateTime(2026, 10, 20),
      method: PaymentMethod.transfer,
      origin: PaymentOrigin.selfSubmitted,
      status: action == 'approve' ? PaymentStatus.approved : PaymentStatus.rejected,
      notes: 'Catatan warga',
      rejectionReason: rejectionReason,
      officerNotes: officerNotes,
      verifiedBy: 'officer-1',
      verifiedAt: DateTime.now(),
    );
  }
}

void main() {
  final testBill = IuranBill(
    id: 'bill-1',
    householdName: 'Rumah 01 — Budi',
    iuranType: 'Kebersihan',
    periode: '2026-10',
    nominal: 50000,
    paidAmount: 0,
    status: IuranStatus.belumBayar,
  );

  final pendingPayment = BackendPayment(
    id: 'pay-pending-1',
    rtId: 'rt-1',
    billId: 'bill-1',
    amount: '50000.00',
    paidAt: DateTime(2026, 10, 20),
    method: PaymentMethod.transfer,
    origin: PaymentOrigin.selfSubmitted,
    status: PaymentStatus.pending,
    notes: 'Transfer via BCA',
  );

  final approvedPayment = BackendPayment(
    id: 'pay-approved-1',
    rtId: 'rt-1',
    billId: 'bill-1',
    amount: '50000.00',
    paidAt: DateTime(2026, 10, 19),
    method: PaymentMethod.transfer,
    origin: PaymentOrigin.selfSubmitted,
    status: PaymentStatus.approved,
    notes: 'Sudah lunas',
  );

  group('IuranDetailScreen — Verification UI and Neon Buttons Polish', () {
    testWidgets(
      'Detail Tagihan does NOT render "Menunggu Verifikasi" section',
      (tester) async {
        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Ensure "Menunggu Verifikasi" is nowhere on the screen
        expect(find.textContaining('Menunggu Verifikasi'), findsNothing);
        // "Riwayat Pembayaran" is present
        expect(find.text('Riwayat Pembayaran'), findsOneWidget);
      },
    );

    testWidgets(
      'Authorized user sees neon Terima and Tolak buttons in Detail Pembayaran dialog for pending payment',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final mockRepo = MockIuranRepository();

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
                iuranRepositoryProvider.overrideWithValue(mockRepo),
                fundLocationRepositoryProvider.overrideWithValue(MockFundLocationRepository()),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Tap the payment in Riwayat Pembayaran to open Detail Pembayaran dialog
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Verify Verifikasi Pembayaran dialog is displayed
        expect(find.byType(Dialog), findsOneWidget);
        expect(
          find.descendant(
            of: find.byType(Dialog),
            matching: find.text('Verifikasi Pembayaran'),
          ),
          findsOneWidget,
        );
        expect(find.text('Bukti Transfer'), findsOneWidget);

        // Verify neon buttons exist
        expect(find.text('Terima'), findsOneWidget);
        expect(find.text('Tolak'), findsOneWidget);

        // Verify neon green styling for Terima
        final terimaFinder = find.widgetWithText(InkWell, 'Terima');
        expect(terimaFinder, findsOneWidget);

        // Tap Terima to trigger approval dialog
        await tester.tap(terimaFinder);
        await tester.pumpAndSettle();

        expect(find.text('Konfirmasi Persetujuan'), findsOneWidget);
        expect(
          find.text('Apakah Anda yakin ingin menyetujui pembayaran ini?'),
          findsOneWidget,
        );

        // Verify Setuju button uses neon green styling and Batal uses neon red styling
        final setujuFinder = find.widgetWithText(InkWell, 'Setuju');
        expect(setujuFinder, findsOneWidget);

        final batalFinder = find.widgetWithText(InkWell, 'Batal');
        expect(batalFinder, findsOneWidget);

        // Verify Setuju is on the left and Batal is on the right horizontally
        final setujuOffset = tester.getTopLeft(setujuFinder);
        final batalOffset = tester.getTopLeft(batalFinder);
        expect(setujuOffset.dx < batalOffset.dx, isTrue);
        expect((setujuOffset.dy - batalOffset.dy).abs() < 2.0, isTrue);

        // Confirm approval
        await tester.tap(setujuFinder);
        await tester.pumpAndSettle();

        expect(mockRepo.lastVerifiedId, 'pay-pending-1');
        expect(mockRepo.lastAction, 'approve');
        expect(mockRepo.lastActualReceivedDate, isNotNull);
        expect(mockRepo.lastActualReceivedDate, matches(RegExp(r'^\d{4}-\d{2}-\d{2}$')));
      },
    );

    testWidgets(
      'Tapping Batal in Konfirmasi Persetujuan dialog dismisses popup without approving',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final mockRepo = MockIuranRepository();

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
                iuranRepositoryProvider.overrideWithValue(mockRepo),
                fundLocationRepositoryProvider.overrideWithValue(MockFundLocationRepository()),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Tap the payment to open Detail Pembayaran
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Tap Terima to open Konfirmasi Persetujuan
        await tester.tap(find.widgetWithText(InkWell, 'Terima'));
        await tester.pumpAndSettle();

        expect(find.text('Konfirmasi Persetujuan'), findsOneWidget);

        // Tap Batal
        final batalFinder = find.widgetWithText(InkWell, 'Batal');
        expect(batalFinder, findsOneWidget);
        await tester.tap(batalFinder);
        await tester.pumpAndSettle();

        // Confirmation dialog is closed, no verification called
        expect(find.text('Konfirmasi Persetujuan'), findsNothing);
        expect(mockRepo.lastVerifiedId, isNull);
      },
    );

    testWidgets(
      'Authorized user can reject with validation from Detail Pembayaran dialog',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final mockRepo = MockIuranRepository();

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
                iuranRepositoryProvider.overrideWithValue(mockRepo),
                fundLocationRepositoryProvider.overrideWithValue(MockFundLocationRepository()),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Open Detail Pembayaran dialog
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Tap Tolak button
        final tolakFinder = find.widgetWithText(InkWell, 'Tolak');
        expect(tolakFinder, findsOneWidget);
        await tester.tap(tolakFinder);
        await tester.pumpAndSettle();

        expect(find.text('Konfirmasi Penolakan'), findsOneWidget);

        // Try submitting without reason (should show validation message)
        await tester.tap(find.widgetWithText(ElevatedButton, 'Tolak'));
        await tester.pump();
        expect(find.text('Alasan penolakan harus diisi'), findsOneWidget);
        expect(mockRepo.lastVerifiedId, isNull);

        // Enter rejection reason and submit
        await tester.enterText(
          find.widgetWithText(TextField, 'Alasan Penolakan'),
          'Bukti transfer buram',
        );
        await tester.tap(find.widgetWithText(ElevatedButton, 'Tolak'));
        await tester.pumpAndSettle();

        expect(mockRepo.lastVerifiedId, 'pay-pending-1');
        expect(mockRepo.lastAction, 'reject');
        expect(mockRepo.lastRejectionReason, 'Bukti transfer buram');
      },
    );

    testWidgets(
      'Warga (unauthorized user) does NOT see Terima or Tolak buttons in Detail Pembayaran dialog',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => false), // Warga / unauthorized
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Open Detail Pembayaran dialog
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Verifikasi Pembayaran is visible for Warga
        expect(find.byType(Dialog), findsOneWidget);
        expect(
          find.descendant(
            of: find.byType(Dialog),
            matching: find.text('Verifikasi Pembayaran'),
          ),
          findsOneWidget,
        );
        expect(find.text('Bukti Transfer'), findsOneWidget);

        // BUT Terima and Tolak buttons are NOT present
        expect(find.text('Terima'), findsNothing);
        expect(find.text('Tolak'), findsNothing);
      },
    );

    testWidgets(
      'Already approved payment does NOT show Terima or Tolak buttons even for authorized users',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [approvedPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Open Detail Pembayaran dialog
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Verifikasi Pembayaran is visible
        expect(find.byType(Dialog), findsOneWidget);
        expect(
          find.descendant(
            of: find.byType(Dialog),
            matching: find.text('Verifikasi Pembayaran'),
          ),
          findsOneWidget,
        );

        // Buttons must not be present since payment is already approved
        expect(find.text('Terima'), findsNothing);
        expect(find.text('Tolak'), findsNothing);
      },
    );

    testWidgets(
      'Pembayaran TRANSFER hanya menampilkan rekening bank pada Konfirmasi Persetujuan',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final mockRepo = MockIuranRepository();
        final mockFundLocRepo = MockFundLocationRepository();

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
                iuranRepositoryProvider.overrideWithValue(mockRepo),
                fundLocationRepositoryProvider.overrideWithValue(mockFundLocRepo),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Tap the payment in Riwayat Pembayaran
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Tap Terima to trigger approval dialog
        await tester.tap(find.widgetWithText(InkWell, 'Terima'));
        await tester.pumpAndSettle();

        expect(find.text('Konfirmasi Persetujuan'), findsOneWidget);
        expect(find.text('Metode: Transfer Bank'), findsOneWidget);

        // Verify dropdown is present
        expect(find.byType(DropdownButtonFormField<String>), findsOneWidget);

        // Verify Rekening BCA is displayed
        expect(find.textContaining('Rekening BCA'), findsOneWidget);

        // Open the dropdown to inspect options
        await tester.tap(find.byType(DropdownButtonFormField<String>));
        await tester.pumpAndSettle();

        // Rekening bank MUST be present in options
        expect(find.textContaining('Rekening BCA'), findsWidgets);

        // Cash / COH / Brankas MUST NOT be present in options
        expect(find.textContaining('Brankas Sekretariat RT'), findsNothing);
        expect(find.textContaining('Kas Di Tangan Bendahara'), findsNothing);

        // Close dropdown by tapping outside or re-selecting
        await tester.tap(find.textContaining('Rekening BCA').last);
        await tester.pumpAndSettle();

        // Confirm approval
        await tester.tap(find.widgetWithText(InkWell, 'Setuju'));
        await tester.pumpAndSettle();

        // Verify repository received the bank account location_id
        expect(mockRepo.lastVerifiedId, 'pay-pending-1');
        expect(mockRepo.lastAction, 'approve');
        expect(mockRepo.lastLocationId, 'loc-1');
      },
    );

    testWidgets(
      'Pembayaran CASH hanya menampilkan Brankas Fisik dan COH pada Konfirmasi Persetujuan',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final mockRepo = MockIuranRepository();
        final mockFundLocRepo = MockFundLocationRepository();

        final pendingCashPayment = BackendPayment(
          id: 'pay-pending-cash',
          rtId: 'rt-1',
          billId: 'bill-1',
          amount: '50000.00',
          paidAt: DateTime(2026, 10, 21),
          method: PaymentMethod.cash,
          origin: PaymentOrigin.selfSubmitted,
          status: PaymentStatus.pending,
          notes: 'Bayar tunai di pos',
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingCashPayment]),
                canVerifyPaymentProvider.overrideWith((ref) => true),
                iuranRepositoryProvider.overrideWithValue(mockRepo),
                fundLocationRepositoryProvider.overrideWithValue(mockFundLocRepo),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Tap the payment in Riwayat Pembayaran
        await tester.tap(find.textContaining('Cash'));
        await tester.pumpAndSettle();

        // Tap Terima to trigger approval dialog
        await tester.tap(find.widgetWithText(InkWell, 'Terima'));
        await tester.pumpAndSettle();

        expect(find.text('Konfirmasi Persetujuan'), findsOneWidget);
        expect(find.text('Metode: Tunai (CASH)'), findsOneWidget);

        // Verify dropdown is present
        expect(find.byType(DropdownButtonFormField<String>), findsOneWidget);

        // Open the dropdown to inspect options
        await tester.tap(find.byType(DropdownButtonFormField<String>));
        await tester.pumpAndSettle();

        // Cash options MUST be present
        expect(find.textContaining('Brankas Sekretariat RT'), findsWidgets);
        expect(find.textContaining('Kas Di Tangan Bendahara'), findsWidgets);

        // Rekening bank MUST NOT be present in options
        expect(find.textContaining('Rekening BCA'), findsNothing);

        // Select Brankas option
        await tester.tap(find.textContaining('Brankas Sekretariat RT').last);
        await tester.pumpAndSettle();

        // Confirm approval
        await tester.tap(find.widgetWithText(InkWell, 'Setuju'));
        await tester.pumpAndSettle();

        // Verify repository received the cash location_id
        expect(mockRepo.lastVerifiedId, 'pay-pending-cash');
        expect(mockRepo.lastAction, 'approve');
        expect(mockRepo.lastLocationId, 'loc-2');
      },
    );

    testWidgets(
      'Persetujuan diblokir jika tidak ada Lokasi Dana yang sesuai dengan metode pembayaran',
      (tester) async {
        tester.view.physicalSize = const Size(800, 1400);
        tester.view.devicePixelRatio = 1.0;
        addTearDown(() {
          tester.view.resetPhysicalSize();
          tester.view.resetDevicePixelRatio();
        });

        final mockRepo = MockIuranRepository();
        // Repository with only cash locations (no bank accounts)
        final mockFundLocRepo = MockFundLocationRepository([
          FundLocation(
            id: 'loc-brankas-only',
            rtId: 'rt-1',
            name: 'Brankas RT',
            category: FundLocationCategory.brankas,
            isActive: true,
            currentBalance: '500000.00',
            createdAt: DateTime.now(),
            updatedAt: DateTime.now(),
          ),
        ]);

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                billDetailProvider('bill-1').overrideWith((ref) async => testBill),
                billPaymentsProvider('bill-1')
                    .overrideWith((ref) async => [pendingPayment]), // Transfer payment
                canVerifyPaymentProvider.overrideWith((ref) => true),
                iuranRepositoryProvider.overrideWithValue(mockRepo),
                fundLocationRepositoryProvider.overrideWithValue(mockFundLocRepo),
              ],
              child: const IuranDetailScreen(billId: 'bill-1'),
            ),
          ),
        );

        await tester.pumpAndSettle();

        // Tap the payment in Riwayat Pembayaran
        await tester.tap(find.textContaining('Transfer'));
        await tester.pumpAndSettle();

        // Tap Terima to trigger approval dialog
        await tester.tap(find.widgetWithText(InkWell, 'Terima'));
        await tester.pumpAndSettle();

        expect(find.text('Konfirmasi Persetujuan'), findsOneWidget);
        expect(find.text('Metode: Transfer Bank'), findsOneWidget);

        // Warning banner must be present
        expect(
          find.textContaining('Tidak ada rekening bank aktif yang terdaftar sebagai Lokasi Dana'),
          findsOneWidget,
        );

        // Dropdown must not be available
        expect(find.byType(DropdownButtonFormField<String>), findsNothing);

        // Tapping Setuju should be blocked
        await tester.tap(find.widgetWithText(InkWell, 'Setuju'));
        await tester.pumpAndSettle();

        // Repository must NOT have been called
        expect(mockRepo.lastVerifiedId, isNull);
      },
    );
  });
}
