import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_form_screen.dart';

Widget _buildFormWithScreen() {
  return MaterialApp(
    home: ProviderScope(
      child: Scaffold(body: SafeArea(child: const IuranFormScreen())),
    ),
  );
}

void main() {
  group('IuranFormScreen — Rendering', () {
    testWidgets(
      'renders all form fields and buttons',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        expect(find.text('Tambah Iuran'), findsOneWidget);
        expect(find.text('Nama Iuran'), findsOneWidget);
        expect(find.text('Nominal'), findsOneWidget);
        expect(find.text('Rp '), findsOneWidget);
        expect(find.textContaining('Dalam Rupiah'), findsOneWidget);
        expect(find.text('Frekuensi'), findsOneWidget);
        expect(find.text('Bulanan'), findsOneWidget);
        expect(find.text('Tahunan'), findsOneWidget);
        expect(find.text('Sekali'), findsOneWidget);
        expect(find.text('Simpan'), findsOneWidget);
        expect(find.text('Batal'), findsOneWidget);
      },
    );

    testWidgets('has 2 text fields', (tester) async {
      await tester.pumpWidget(_buildFormWithScreen());
      await tester.pumpAndSettle();
      expect(find.byType(TextField), findsNWidgets(2));
    });

    testWidgets('amount field has prefix Rp and helper text', (tester) async {
      await tester.pumpWidget(_buildFormWithScreen());
      await tester.pumpAndSettle();

      final textField = tester.widget<TextField>(
        find.byType(TextField).at(1),
      );
      expect(textField.decoration?.prefixText, 'Rp ');
      expect(textField.decoration?.helperText, 'Dalam Rupiah');
    });
  });

  group('IuranFormScreen — Validation', () {
    testWidgets(
      'empty form shows validation errors on submit',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();

        expect(find.text('Data belum lengkap:'), findsOneWidget);
        expect(
          find.textContaining('Nama iuran harus diisi.'),
          findsOneWidget,
        );
        expect(
          find.textContaining('Nominal harus angka'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'rejects zero amount with valid name',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byType(TextField).at(0),
          'Iuran Test',
        );
        await tester.enterText(
          find.byType(TextField).at(1),
          '0',
        );
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();

        expect(find.textContaining('Nominal harus angka'), findsOneWidget);
      },
    );

    testWidgets(
      'accepts valid data (no validation errors)',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byType(TextField).at(0),
          'Iuran Keamanan Bulanan',
        );
        await tester.enterText(
          find.byType(TextField).at(1),
          '50000',
        );
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Data belum lengkap:'),
          findsNothing,
        );
      },
    );

    testWidgets(
      'rejects empty name with valid amount',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byType(TextField).at(1),
          '50000',
        );
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Nama iuran harus diisi.'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'rejects negative amount',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byType(TextField).at(0),
          'Iuran Test',
        );
        await tester.enterText(
          find.byType(TextField).at(1),
          '-100',
        );
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Nominal harus angka'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'accepts large positive amount',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byType(TextField).at(0),
          'Iuran Besar',
        );
        await tester.enterText(
          find.byType(TextField).at(1),
          '9999999999',
        );
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Data belum lengkap:'),
          findsNothing,
        );
      },
    );

    testWidgets(
      'validation errors clear when user types in name field',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        // Tap name field first
        await tester.tap(find.byType(TextField).at(0));
        tester.testTextInput.enterText('Iuran Test');
        await tester.pumpAndSettle();

        // Now trigger validation
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();
        expect(find.text('Data belum lengkap:'), findsOneWidget);

        // Type again — triggers onChange which clears errors
        await tester.tap(find.byType(TextField).at(0));
        tester.testTextInput.enterText('Iuran Test Baru');
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Data belum lengkap:'),
          findsNothing,
        );
      },
    );

    testWidgets(
      'validation errors clear when user types in amount field',
      (tester) async {
        await tester.pumpWidget(_buildFormWithScreen());
        await tester.pumpAndSettle();

        // Tap amount field first
        await tester.tap(find.byType(TextField).at(1));
        tester.testTextInput.enterText('50000');
        await tester.pumpAndSettle();

        // Now trigger validation
        await tester.tap(find.text('Simpan'));
        await tester.pumpAndSettle();
        expect(find.text('Data belum lengkap:'), findsOneWidget);

        // Type again — triggers onChange which clears errors
        await tester.tap(find.byType(TextField).at(1));
        tester.testTextInput.enterText('75000');
        await tester.pumpAndSettle();

        expect(
          find.textContaining('Data belum lengkap:'),
          findsNothing,
        );
      },
    );
  });

  // NOTE: Save-state tests (spinner swap, button disabled, error display)
  // require API mocking — a real Dio call always fires and creates pending
  // timers in the test harness.  The _saving / spinner logic is trivial
  // one-liner in _save() and covered by code review.
}
