import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_screen.dart';

class _TestBillsNotifier extends BillsNotifier {
  _TestBillsNotifier(this.bills);

  final List<IuranBill> bills;

  @override
  Future<List<IuranBill>> build() async => bills;
}

void main() {
  final currentPeriod =
      '${DateTime.now().year}-${DateTime.now().month.toString().padLeft(2, '0')}';

  final billWithPending = IuranBill(
    id: 'bill-pending',
    householdName: 'Keluarga Budi',
    iuranType: 'Iuran Sampah',
    periode: currentPeriod,
    nominal: 500000,
    paidAmount: 0,
    pendingAmount: 500000,
    rejectedAmount: 0,
    status: IuranStatus.belumBayar,
  );

  final billWithApproved = IuranBill(
    id: 'bill-approved',
    householdName: 'Keluarga Siti',
    iuranType: 'Iuran Keamanan',
    periode: currentPeriod,
    nominal: 300000,
    paidAmount: 300000,
    pendingAmount: 0,
    rejectedAmount: 0,
    status: IuranStatus.lunas,
  );

  final billWithRejected = IuranBill(
    id: 'bill-rejected',
    householdName: 'Keluarga Agus',
    iuranType: 'Iuran Kebersihan',
    periode: currentPeriod,
    nominal: 200000,
    paidAmount: 0,
    pendingAmount: 0,
    rejectedAmount: 200000,
    status: IuranStatus.belumBayar,
  );

  final billPartialApprovedAndPending = IuranBill(
    id: 'bill-mixed',
    householdName: 'Keluarga Joko',
    iuranType: 'Iuran IPL',
    periode: currentPeriod,
    nominal: 1000000,
    paidAmount: 400000,
    pendingAmount: 300000,
    rejectedAmount: 100000,
    status: IuranStatus.sebagian,
  );

  final pengurusUser = {
    'id': 'user-pengurus',
    'email': 'pengurus@test.com',
    'name': 'Ketua RT',
    'role': 'pengurus',
    'permissions': [
      'finance.read',
      'finance.create',
      'finance.update',
      'finance.approve',
    ],
  };

  final wargaUser = {
    'id': 'user-warga',
    'email': 'warga@test.com',
    'name': 'Warga Biasa',
    'role': 'warga',
    'permissions': ['bill.read', 'payment.create'],
  };

  Widget buildScreen({
    required List<IuranBill> bills,
    required Map<String, dynamic> user,
  }) {
    return MaterialApp(
      home: ProviderScope(
        overrides: [
          iuranFilterExpandedProvider(0).overrideWith((ref) => true),
          iuranFilterExpandedProvider(1).overrideWith((ref) => true),
          iuranSelectedPaymentStatusProvider.overrideWith((ref) => ''),
          iuranSelectedStatusProvider.overrideWith((ref) => ''),
          billsProvider.overrideWith(() => _TestBillsNotifier(bills)),
          myBillsProvider.overrideWith(() => _TestBillsNotifier(bills)),
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
        child: const IuranScreen(),
      ),
    );
  }

  group('_IuranCard — Pending & Sisa nominal display', () {
    testWidgets(
      'displays "Tertunda Rp 500.000" and "Sisa Rp 500.000" when pendingAmount > 0 and isInArrears',
      (tester) async {
        await tester.pumpWidget(
          buildScreen(
            bills: [billWithPending],
            user: pengurusUser,
          ),
        );

        await tester.pumpAndSettle();
        await tester.tap(find.text('Iuran Warga'));
        await tester.pumpAndSettle();

        expect(find.text('Keluarga Budi'), findsOneWidget);
        expect(find.text(formatRupiah(500000)), findsOneWidget);
        expect(find.text('Tertunda ${formatRupiah(500000)}'), findsOneWidget);
        expect(find.text('Sisa ${formatRupiah(500000)}'), findsOneWidget);
      },
    );

    testWidgets(
      'hides "Tertunda" line when pendingAmount is 0',
      (tester) async {
        await tester.pumpWidget(
          buildScreen(
            bills: [billWithApproved],
            user: pengurusUser,
          ),
        );

        await tester.pumpAndSettle();
        await tester.tap(find.text('Iuran Warga'));
        await tester.pumpAndSettle();

        expect(find.text('Keluarga Siti'), findsOneWidget);
        expect(find.textContaining('Tertunda'), findsNothing);
        expect(find.text('Dibayar: ${formatRupiah(300000)}'), findsOneWidget);
      },
    );

    testWidgets(
      'partial approved (400k) and pending (300k) on 1000k bill shows Sisa Rp 600.000 and Tertunda Rp 300.000',
      (tester) async {
        await tester.pumpWidget(
          buildScreen(
            bills: [billPartialApprovedAndPending],
            user: pengurusUser,
          ),
        );

        await tester.pumpAndSettle();
        await tester.tap(find.text('Iuran Warga'));
        await tester.pumpAndSettle();

        expect(find.text('Keluarga Joko'), findsOneWidget);
        expect(find.text(formatRupiah(1000000)), findsOneWidget);
        expect(find.text('Dibayar: ${formatRupiah(400000)}'), findsOneWidget);
        expect(find.text('Tertunda ${formatRupiah(300000)}'), findsOneWidget);
        // Sisa must be 1.000.000 - 400.000 = 600.000 (pending does NOT reduce Sisa!)
        expect(find.text('Sisa ${formatRupiah(600000)}'), findsOneWidget);
      },
    );
  });

  group('IuranScreen — Payment Status Filter', () {
    testWidgets(
      'filtering by Menunggu Verifikasi, Disetujui, and Ditolak works correctly',
      (tester) async {
        final allBills = [
          billWithPending,
          billWithApproved,
          billWithRejected,
        ];

        await tester.pumpWidget(
          buildScreen(
            bills: allBills,
            user: pengurusUser,
          ),
        );

        await tester.pumpAndSettle();
        await tester.tap(find.text('Iuran Warga'));
        await tester.pumpAndSettle();

        // Initially all 3 bills are visible
        expect(find.text('Keluarga Budi', skipOffstage: false), findsOneWidget);
        expect(find.text('Keluarga Siti', skipOffstage: false), findsOneWidget);
        expect(find.text('Keluarga Agus', skipOffstage: false), findsOneWidget);

        // Find "Pembayaran" dropdown and tap it
        final pembayaranChip = find.text('Pembayaran', skipOffstage: false);
        expect(pembayaranChip, findsOneWidget);
        await tester.tap(pembayaranChip, warnIfMissed: false);
        await tester.pumpAndSettle();

        // Tap "Menunggu"
        await tester.tap(find.text('Menunggu').last);
        await tester.pumpAndSettle();

        // Only billWithPending should be visible
        expect(find.text('Keluarga Budi', skipOffstage: false), findsOneWidget);
        expect(find.text('Keluarga Siti', skipOffstage: false), findsNothing);
        expect(find.text('Keluarga Agus', skipOffstage: false), findsNothing);

        // Tap dropdown again (now showing "Menunggu")
        await tester.tap(find.text('Menunggu'));
        await tester.pumpAndSettle();

        // Tap "Disetujui"
        await tester.tap(find.text('Disetujui').last);
        await tester.pumpAndSettle();

        // Only billWithApproved should be visible
        expect(find.text('Keluarga Budi', skipOffstage: false), findsNothing);
        expect(find.text('Keluarga Siti', skipOffstage: false), findsOneWidget);
        expect(find.text('Keluarga Agus', skipOffstage: false), findsNothing);

        // Tap dropdown again (now showing "Disetujui")
        await tester.tap(find.text('Disetujui'));
        await tester.pumpAndSettle();

        // Tap "Ditolak"
        await tester.tap(find.text('Ditolak').last);
        await tester.pumpAndSettle();

        // Only billWithRejected should be visible
        expect(find.text('Keluarga Budi', skipOffstage: false), findsNothing);
        expect(find.text('Keluarga Siti', skipOffstage: false), findsNothing);
        expect(find.text('Keluarga Agus', skipOffstage: false), findsOneWidget);

        // Tap dropdown again and reset to "Semua"
        await tester.tap(find.text('Ditolak'));
        await tester.pumpAndSettle();
        await tester.tap(find.text('Semua').last);
        await tester.pumpAndSettle();

        // All 3 restored
        expect(find.text('Keluarga Budi', skipOffstage: false), findsOneWidget);
        expect(find.text('Keluarga Siti', skipOffstage: false), findsOneWidget);
        expect(find.text('Keluarga Agus', skipOffstage: false), findsOneWidget);
      },
    );

    testWidgets(
      'Warga role can use payment status filter without receiving verification controls',
      (tester) async {
        await tester.pumpWidget(
          buildScreen(
            bills: [billWithPending],
            user: wargaUser,
          ),
        );

        await tester.pumpAndSettle();

        // Warga sees the bills
        expect(find.text('Iuran Sampah'), findsOneWidget);

        // Warga sees the payment status filter dropdown
        expect(find.text('Pembayaran', skipOffstage: false), findsOneWidget);

        // Warga does NOT see manage tabs (TabBar)
        expect(find.text('Kelola'), findsNothing);

        // Warga does NOT see verification action buttons
        expect(find.text('Terima'), findsNothing);
        expect(find.text('Tolak'), findsNothing);
      },
    );
  });
}
