import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_screen.dart';

KasTransaction _transaction({
  String? paymentMethod,
  KasJenis jenis = KasJenis.keluar,
}) => KasTransaction(
  id: 'test-transaction',
  tanggal: DateTime(2026, 10, 18),
  jenis: jenis,
  kategori: KasCategory.operasionalRT,
  keterangan: 'Pembelian perlengkapan',
  paymentMethod: paymentMethod,
  nominal: '10000',
  saldo: '90000',
);

Future<void> _pumpCard(
  WidgetTester tester, {
  String? paymentMethod,
  KasJenis jenis = KasJenis.keluar,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: KasTransactionCard(
          transaction: _transaction(paymentMethod: paymentMethod, jenis: jenis),
          isDark: false,
        ),
      ),
    ),
  );
}

void main() {
  group('KAS transaction payment method badge', () {
    testWidgets('CASH uses a blue cash icon with accessible label', (
      tester,
    ) async {
      await _pumpCard(tester, paymentMethod: 'CASH');

      final iconFinder = find.byIcon(Icons.payments_outlined);
      expect(iconFinder, findsOneWidget);
      expect(find.byTooltip('Tunai'), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (widget) => widget is Semantics && widget.properties.label == 'Tunai',
        ),
        findsOneWidget,
      );
      expect(tester.widget<Icon>(iconFinder).color, AppColors.info);
      final badge = tester.widget<Container>(
        find.ancestor(of: iconFinder, matching: find.byType(Container)).first,
      );
      expect(
        (badge.decoration! as BoxDecoration).color,
        AppColors.info.withValues(alpha: 0.12),
      );
      expect(find.text('Tunai'), findsNothing);
      expect(find.byIcon(Icons.arrow_upward), findsOneWidget);
    });

    testWidgets('TRANSFER uses a violet bank icon with accessible label', (
      tester,
    ) async {
      await _pumpCard(tester, paymentMethod: 'TRANSFER');

      final iconFinder = find.byIcon(Icons.account_balance_outlined);
      expect(iconFinder, findsOneWidget);
      expect(find.byTooltip('Transfer'), findsOneWidget);
      expect(
        find.byWidgetPredicate(
          (widget) =>
              widget is Semantics && widget.properties.label == 'Transfer',
        ),
        findsOneWidget,
      );
      expect(tester.widget<Icon>(iconFinder).color, AppColors.accent);
      final badge = tester.widget<Container>(
        find.ancestor(of: iconFinder, matching: find.byType(Container)).first,
      );
      expect(
        (badge.decoration! as BoxDecoration).color,
        AppColors.accent.withValues(alpha: 0.12),
      );
      expect(find.text('Transfer'), findsNothing);
      expect(find.byIcon(Icons.arrow_upward), findsOneWidget);
    });

    testWidgets('empty and unknown methods display a neutral accessible icon', (
      tester,
    ) async {
      await _pumpCard(tester);
      expect(find.byIcon(Icons.help_outline), findsOneWidget);
      expect(
        find.byTooltip('Metode pembayaran tidak diketahui'),
        findsOneWidget,
      );
      expect(
        find.byWidgetPredicate(
          (widget) =>
              widget is Semantics &&
              widget.properties.label == 'Metode pembayaran tidak diketahui',
        ),
        findsOneWidget,
      );
      expect(find.text('Tunai'), findsNothing);
      expect(find.text('Transfer'), findsNothing);

      await _pumpCard(tester, paymentMethod: 'CRYPTO');
      expect(find.byIcon(Icons.help_outline), findsOneWidget);
      expect(find.text('Tunai'), findsNothing);
      expect(find.text('Transfer'), findsNothing);
    });

    testWidgets('payment method matching is case- and whitespace-insensitive', (
      tester,
    ) async {
      await _pumpCard(tester, paymentMethod: ' transfer ');

      expect(find.byIcon(Icons.account_balance_outlined), findsOneWidget);
      expect(find.byTooltip('Transfer'), findsOneWidget);
    });

    testWidgets('incoming and outgoing direction indicators remain visible', (
      tester,
    ) async {
      await _pumpCard(tester, paymentMethod: 'CASH', jenis: KasJenis.masuk);
      expect(find.byIcon(Icons.arrow_downward), findsOneWidget);
      expect(find.byTooltip('Tunai'), findsOneWidget);

      await _pumpCard(
        tester,
        paymentMethod: 'TRANSFER',
        jenis: KasJenis.keluar,
      );
      expect(find.byIcon(Icons.arrow_upward), findsOneWidget);
      expect(find.byTooltip('Transfer'), findsOneWidget);
    });

    testWidgets('TUNAI and BANK/QRIS aliases map to Tunai and Transfer (C3)', (
      tester,
    ) async {
      await _pumpCard(tester, paymentMethod: 'TUNAI', jenis: KasJenis.masuk);
      expect(find.byIcon(Icons.payments_outlined), findsOneWidget);
      expect(find.byTooltip('Tunai'), findsOneWidget);

      await _pumpCard(tester, paymentMethod: 'QRIS', jenis: KasJenis.masuk);
      expect(find.byIcon(Icons.account_balance_outlined), findsOneWidget);
      expect(find.byTooltip('Transfer'), findsOneWidget);
    });
  });

  group('KAS incoming iuran reference & IPL badge (A5, C1)', () {
    testWidgets(
      'incoming iuran IPL transaction shows reference number and IPL AppBadge without houseNumber',
      (tester) async {
        final tx = KasTransaction(
          id: 'tx-ipl-1',
          tanggal: DateTime(2026, 10, 18),
          jenis: KasJenis.masuk,
          kategori: KasCategory.iuranWarga,
          keterangan: 'Pembayaran Iuran Warga B1/10',
          referensi: 'INV-2026-0099',
          dueName: 'IPL',
          citizenName: 'Budi Santoso',
          houseNumber: 'B1/10',
          paymentMethod: 'TRANSFER',
          nominal: '85000',
          saldo: '500000',
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: KasTransactionCard(transaction: tx, isDark: false),
            ),
          ),
        );

        expect(find.text('INV-2026-0099'), findsOneWidget);
        expect(find.text('IPL'), findsOneWidget);
        expect(find.textContaining('B1/10'), findsNothing);
        expect(find.textContaining('Warga ·'), findsNothing);
      },
    );

    testWidgets(
      'incoming iuran transaction without reference uses safe fallback and never shows null/unknown or houseNumber',
      (tester) async {
        final tx = KasTransaction(
          id: 'tx-no-ref',
          tanggal: DateTime(2026, 10, 18),
          jenis: KasJenis.masuk,
          kategori: KasCategory.iuranWarga,
          keterangan: 'Pembayaran Iuran',
          referensi: 'null',
          dueName: 'Iuran Keamanan',
          houseNumber: 'A2/05',
          paymentMethod: 'CASH',
          nominal: '50000',
          saldo: '500000',
        );

        await tester.pumpWidget(
          MaterialApp(
            home: Scaffold(
              body: KasTransactionCard(transaction: tx, isDark: false),
            ),
          ),
        );

        expect(find.text('Iuran Keamanan · Tanpa Referensi'), findsOneWidget);
        expect(find.textContaining('A2/05'), findsNothing);
        expect(find.text('IPL'), findsNothing);
      },
    );
  });
}
