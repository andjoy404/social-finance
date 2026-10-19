import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_arrears_screen.dart';

class _BillsFixtureNotifier extends BillsNotifier {
  _BillsFixtureNotifier(this.bills);

  final List<IuranBill> bills;

  @override
  Future<List<IuranBill>> build() async => bills;
}

class _DeferredBillsNotifier extends BillsNotifier {
  final completer = Completer<List<IuranBill>>();

  @override
  Future<List<IuranBill>> build() => completer.future;
}

class _FailedBillsNotifier extends BillsNotifier {
  @override
  Future<List<IuranBill>> build() async => throw StateError('load failed');
}

void main() {
  testWidgets('Tagihan shows scoped regular and voluntary bills only', (
    tester,
  ) async {
    final regularBill = IuranBill(
      id: 'own-regular-bill',
      householdName: 'Household in user scope',
      iuranType: 'Iuran Bulanan',
      periode: 'Oktober 2026',
      nominal: 50000,
    );
    final voluntaryBill = IuranBill(
      id: 'own-voluntary-bill',
      householdName: 'Voluntary household in user scope',
      iuranType: 'Sumbangan Sukarela',
      periode: 'Oktober 2026',
      nominal: 0,
      status: IuranStatus.sukarela,
      isVoluntary: true,
    );
    final unscopedBill = IuranBill(
      id: 'other-bill',
      householdName: 'Household outside user scope',
      iuranType: 'Iuran Bulanan',
      periode: 'Oktober 2026',
      nominal: 50000,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          myBillsProvider.overrideWith(
            () => _BillsFixtureNotifier([regularBill, voluntaryBill]),
          ),
          billsProvider.overrideWith(
            () => _BillsFixtureNotifier([
              regularBill,
              voluntaryBill,
              unscopedBill,
            ]),
          ),
        ],
        child: const MaterialApp(home: IuranArrearsScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Household in user scope'), findsOneWidget);
    expect(find.text('Voluntary household in user scope'), findsOneWidget);
    expect(find.text('Household outside user scope'), findsNothing);
    expect(find.text('Sumbangan Sukarela'), findsOneWidget);
    expect(find.text('Sukarela'), findsOneWidget);
    expect(find.text('Iuran Bulanan'), findsOneWidget);
    expect(find.text('Belum Bayar'), findsOneWidget);
    expect(find.text('OKTOBER 2026'), findsNWidgets(2));
    expect(find.text('Rp 50.000'), findsOneWidget);
    expect(find.text('Rp 0'), findsOneWidget);
    expect(find.byIcon(Icons.receipt_long_outlined), findsNWidgets(2));
    expect(find.byIcon(Icons.warning_amber_rounded), findsNothing);
  });

  testWidgets('empty scoped results do not fall back to all bills', (
    tester,
  ) async {
    final unscopedBill = IuranBill(
      id: 'other-bill',
      householdName: 'Household outside user scope',
      iuranType: 'Iuran Bulanan',
      periode: 'Oktober 2026',
      nominal: 50000,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          myBillsProvider.overrideWith(() => _BillsFixtureNotifier([])),
          billsProvider.overrideWith(
            () => _BillsFixtureNotifier([unscopedBill]),
          ),
        ],
        child: const MaterialApp(home: IuranArrearsScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Belum Ada Tagihan'), findsOneWidget);
    expect(find.text('Household outside user scope'), findsNothing);
  });

  testWidgets('Tagihan filters retain voluntary records in the scoped list', (
    tester,
  ) async {
    final regularBill = IuranBill(
      id: 'regular-bill',
      householdName: 'Regular household',
      iuranType: 'Iuran Bulanan',
      periode: 'Oktober 2026',
      nominal: 50000,
    );
    final voluntaryBill = IuranBill(
      id: 'voluntary-bill',
      householdName: 'Voluntary household',
      iuranType: 'Sumbangan Sukarela',
      periode: 'Oktober 2026',
      nominal: 0,
      status: IuranStatus.sukarela,
      isVoluntary: true,
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          myBillsProvider.overrideWith(
            () => _BillsFixtureNotifier([regularBill, voluntaryBill]),
          ),
          iuranSelectedPeriodeProvider.overrideWith((_) => 'Oktober 2026'),
          iuranSelectedStatusProvider.overrideWith((_) => 'sukarela'),
        ],
        child: const MaterialApp(home: IuranArrearsScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Voluntary household'), findsOneWidget);
    expect(find.text('Regular household'), findsNothing);
  });

  testWidgets('scoped loading state remains visible until loaded', (
    tester,
  ) async {
    final notifier = _DeferredBillsNotifier();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [myBillsProvider.overrideWith(() => notifier)],
        child: const MaterialApp(home: IuranArrearsScreen()),
      ),
    );
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    notifier.completer.complete([]);
    await tester.pumpAndSettle();
    expect(find.text('Belum Ada Tagihan'), findsOneWidget);
  });

  testWidgets('scoped loading errors show an error state, not all bills', (
    tester,
  ) async {
    final unscopedBill = IuranBill(
      id: 'other-bill',
      householdName: 'Household outside user scope',
      iuranType: 'Iuran Bulanan',
      periode: 'Oktober 2026',
      nominal: 50000,
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          myBillsProvider.overrideWith(() => _FailedBillsNotifier()),
          billsProvider.overrideWith(
            () => _BillsFixtureNotifier([unscopedBill]),
          ),
        ],
        child: const MaterialApp(home: IuranArrearsScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Gagal Memuat Data'), findsOneWidget);
    expect(find.text('Household outside user scope'), findsNothing);
  });
}
