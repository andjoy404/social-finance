// Widget tests for IuranPaymentScreen — payment form feature.
//
// Covers:
// - Rendering of bill summary, date, method, amount, and notes fields
// - Validation: amount must be positive number
// - Payment method selection: Tunai/Transfer
// - Loading state: spinner + button disable during save

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';

// ─── Test Helpers ─────────────────────────────────────────────────────

const _kBillId = 'test-bill-id';

late IuranBill _kTestBill;

Widget _buildApp() {
  _kTestBill = IuranBill(
    id: _kBillId,
    householdName: 'Bpk. Ahmad',
    iuranType: 'Iuran Keamanan',
    periode: 'Oktober 2026',
    nominal: 50000,
    paidAmount: 0,
    status: IuranStatus.belumBayar,
  );
  return MaterialApp(
    home: ProviderScope(
      overrides: [
        billDetailProvider(_kBillId).overrideWith(
          (ref) async => _kTestBill,
        ),
      ],
      child: IuranPaymentScreen(billId: _kBillId),
    ),
  );
}

/// Pump once to kick off futures, then wait for idle.
Future<void> _pumpPage(WidgetTester tester) async {
  await tester.pumpWidget(_buildApp());
  await tester.pump();
  await tester.pumpAndSettle(const Duration(seconds: 5));
}

/// Scroll the ListView so the target becomes visible.
Future<void> _scrollToVisible(WidgetTester tester, Finder target) async {
  final elements = target.evaluate();
  if (elements.isEmpty) return;
  final element = elements.first;
  final scrollable = Scrollable.of(element);
  final position = scrollable.position;
  final maxScroll = position.maxScrollExtent;
  final currentScroll = position.pixels;

  if (currentScroll < maxScroll - 1.0) {
    // Drag upward to scroll down
    await tester.dragFrom(
      const Offset(400, 300),
      const Offset(0, -400),
    );
    await tester.pumpAndSettle();
    // Recursively scroll if still not at bottom
    await _scrollToVisible(tester, target);
  }
}

/// Finder for SegmentedButton with PaymentMethod type — byType doesn't work with
/// typedef-based generic widgets in this Flutter version.
Finder _findSegmentedButton() => find.byWidgetPredicate(
      (Widget w) => w.runtimeType.toString() == 'SegmentedButton<PaymentMethod>',
      description: 'SegmentedButton<PaymentMethod>',
    );

// ─── Tests ────────────────────────────────────────────────────────────

void main() {
  group('IuranPaymentScreen — Rendering', () {
    testWidgets(
      'renders AppBar title',
      (tester) async {
        await _pumpPage(tester);
        expect(find.text('Pembayaran Iuran'), findsOneWidget);
      },
    );

    testWidgets(
      'renders bill summary with correct data',
      (tester) async {
        await _pumpPage(tester);
        expect(find.text('Ringkasan Tagihan'), findsOneWidget);
        expect(find.text('Bpk. Ahmad', skipOffstage: false), findsOneWidget);
        expect(find.text('Iuran Keamanan', skipOffstage: false), findsOneWidget);
        expect(find.textContaining('OKTOBER 2026'), findsOneWidget);
      },
    );

    testWidgets(
      'renders date, method, amount, and notes fields',
      (tester) async {
        await _pumpPage(tester);
        expect(find.text('Tanggal Pembayaran'), findsOneWidget);
        expect(find.text('Metode Pembayaran', skipOffstage: false), findsOneWidget);
        expect(find.text('Tunai', skipOffstage: false), findsOneWidget);
        expect(find.text('Transfer', skipOffstage: false), findsOneWidget);
        // SegmentedButton requires widgetPredicate in this Flutter version.
        expect(_findSegmentedButton(), findsOneWidget);
        expect(find.text('Nominal Pembayaran'), findsOneWidget);
        expect(find.text('Catatan Pembayaran'), findsOneWidget);
        // 2 TextFields: nominal + notes
        expect(find.byType(TextField), findsNWidgets(2));
      },
    );

    testWidgets(
      'renders Bayar and Batal buttons',
      (tester) async {
        await _pumpPage(tester);
        // Buttons are below the fold in ListView — use skipOffstage: false
        expect(find.text('Bayar', skipOffstage: false), findsOneWidget);
        expect(find.text('Batal', skipOffstage: false), findsOneWidget);
      },
    );

    testWidgets(
      'shows remaining amount',
      (tester) async {
        await _pumpPage(tester);
        expect(find.textContaining('Rp 50.000'), findsOneWidget);
      },
    );
  });

  group('IuranPaymentScreen — Payment Method', () {
    testWidgets(
      'default payment method is Tunai (cash)',
      (tester) async {
        await _pumpPage(tester);
        final segButt = _findSegmentedButton().evaluate().first.widget as SegmentedButton<PaymentMethod>;
        expect(segButt.selected, contains(PaymentMethod.cash));
        expect(segButt.selected, isNot(contains(PaymentMethod.transfer)));
      },
    );

    testWidgets(
      'tapping Transfer segment selects transfer method',
      (tester) async {
        await _pumpPage(tester);
        // Tap "Transfer" text inside SegmentedButton (visible on screen)
        await tester.tap(find.text('Transfer', skipOffstage: false));
        await tester.pumpAndSettle();

        final segButt = _findSegmentedButton().evaluate().first.widget as SegmentedButton<PaymentMethod>;
        expect(segButt.selected, contains(PaymentMethod.transfer));
        expect(segButt.selected, isNot(contains(PaymentMethod.cash)));
      },
    );
  });

  group('IuranPaymentScreen — Validation', () {
    testWidgets(
      'rejects save with empty amount',
      (tester) async {
        await _pumpPage(tester);

        // Scroll to make the button visible, then tap
        await _scrollToVisible(tester, find.text('Bayar', skipOffstage: false));
        await tester.tap(find.text('Bayar', skipOffstage: false));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Nominal harus diisi.'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'rejects save with non-numeric amount',
      (tester) async {
        await _pumpPage(tester);

        await tester.enterText(find.byType(TextField).at(0), 'abc');
        await tester.pumpAndSettle();

        await _scrollToVisible(tester, find.text('Bayar', skipOffstage: false));
        await tester.tap(find.text('Bayar', skipOffstage: false));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Nominal harus berupa angka'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'rejects save with zero amount',
      (tester) async {
        await _pumpPage(tester);

        await tester.enterText(find.byType(TextField).at(0), '0');
        await tester.pumpAndSettle();

        await _scrollToVisible(tester, find.text('Bayar', skipOffstage: false));
        await tester.tap(find.text('Bayar', skipOffstage: false));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Nominal harus berupa angka'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'rejects save with negative amount',
      (tester) async {
        await _pumpPage(tester);

        await tester.enterText(find.byType(TextField).at(0), '-100');
        await tester.pumpAndSettle();

        await _scrollToVisible(tester, find.text('Bayar', skipOffstage: false));
        await tester.tap(find.text('Bayar', skipOffstage: false));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Nominal harus berupa angka'),
          findsOneWidget,
        );
      },
    );
  });

  group('IuranPaymentScreen — Loading State', () {
    testWidgets(
      'shows spinner and disables buttons while saving',
      (tester) async {
        await _pumpPage(tester);

        // Enter valid amount — _validate() will pass since PaymentMethod
        // defaults to .cash.  The repo call will fail (no API),
        // but _saving is set to true BEFORE the async call.
        await tester.enterText(find.byType(TextField).at(0), '50000');
        await tester.pumpAndSettle();

        // Scroll to make button visible
        await _scrollToVisible(tester, find.text('Bayar', skipOffstage: false));

        // Tap Bayar → setState(_saving=true) runs synchronously
        await tester.tap(find.text('Bayar', skipOffstage: false));
        // One pump triggers the rebuild where _saving == true.
        await tester.pump();

        expect(find.byType(CircularProgressIndicator), findsOneWidget);

        // Verify the Bayar button text is replaced with the spinner.
        expect(find.text('Bayar', skipOffstage: false), findsNothing);

        // Let the pending API call complete so the test cleans up.
        await tester.pumpAndSettle();
      },
    );
  });

  group('IuranPaymentScreen — Payment Proof Option', () {
    testWidgets('renders proof upload buttons (Galeri and Kamera)', (tester) async {
      await _pumpPage(tester);

      expect(find.text('Bukti Pembayaran (Opsional)', skipOffstage: false), findsOneWidget);
      expect(find.text('Galeri', skipOffstage: false), findsOneWidget);
      expect(find.text('Kamera', skipOffstage: false), findsOneWidget);
    });
  });
}
