/// Unit tests for `filteredBillsProvider` sorting rules.
///
/// Covers:
/// - Unpaid/partial bills appear before paid (LUNAS)
/// - Unpaid/partial group: oldest → newest
/// - Paid group: newest → oldest
/// - Chronological comparison by year and month, not alphabetical
/// - Year-boundary transitions (Desember → Januari)
/// - Same-period tie-break stability
/// - Active filters are not bypassed by sorting

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';

// ─── Mock notifier — returns fixed test data ──────────────────────────────

class _MockBillsNotifier extends BillsNotifier {
  final List<IuranBill> data;
  _MockBillsNotifier({required this.data});
  @override
  Future<List<IuranBill>> build() async => data;
}

// ─── Helpers ──────────────────────────────────────────────────────────────

/// Create a bill with a unique id to avoid Riverpod identity conflicts.
IuranBill _bill({
  int index = 0,
  required String householdName,
  required String iuranType,
  required String periode,
  required IuranStatus status,
}) {
  return IuranBill(
    id: 'sort-bill-$index',
    householdName: householdName,
    iuranType: iuranType,
    periode: periode,
    nominal: 50000,
    status: status,
  );
}

/// Create a ProviderContainer pre-wired with the given bills list.
ProviderContainer _makeContainer({
  required List<IuranBill> bills,
  List<Override>? extraOverrides,
}) {
  final overrides = <Override>[
    billsProvider.overrideWith(() => _MockBillsNotifier(data: bills)),
    ...(extraOverrides ?? []),
  ];
  final container = ProviderContainer(overrides: overrides);
  return container;
}

/// Read filtered bills after waiting for the async billsProvider to load.
Future<List<IuranBill>> _getFilteredAsync(
  ProviderContainer container, {
  required List<IuranBill> bills,
}) async {
  // Wait for the mock notifier to finish build()
  await container.read(billsProvider.future);
  return container.read(filteredBillsProvider);
}

// ─── Tests ────────────────────────────────────────────────────────────────

void main() {
  group('filteredBillsProvider — sorting rules', () {
    test(
      'BELUM BAYAR and SEBAGIAN appear before LUNAS',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.belumBayar),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Maret 2026', status: IuranStatus.sebagian),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        expect(result[0].id, equals('sort-bill-1')); // belumBayar
        expect(result[1].id, equals('sort-bill-2')); // sebagian
        expect(result[2].id, equals('sort-bill-0')); // lunas
        container.dispose();
      },
    );

    test(
      'Unpaid/partial group: oldest → newest',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Maret 2026', status: IuranStatus.belumBayar),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.sebagian),
          _bill(index: 3, householdName: 'D', iuranType: 'Keamanan', periode: 'Desember 2025', status: IuranStatus.belumBayar),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        // Unpaid/partial: Des 2025 < Feb 2026 < Mar 2026
        // Paid: Jan 2026
        expect(result[0].id, equals('sort-bill-3')); // Des 2025 - belumBayar
        expect(result[1].id, equals('sort-bill-2')); // Feb 2026 - sebagian
        expect(result[2].id, equals('sort-bill-0')); // Mar 2026 - belumBayar
        expect(result[3].id, equals('sort-bill-1')); // Jan 2026 - lunas
        container.dispose();
      },
    );

    test(
      'Paid group: newest → oldest',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Mei 2026', status: IuranStatus.lunas),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Maret 2026', status: IuranStatus.lunas),
          _bill(index: 3, householdName: 'D', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.belumBayar),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        expect(result[0].id, equals('sort-bill-3')); // belumBayar
        expect(result[1].id, equals('sort-bill-1')); // Mei - lunas (newest)
        expect(result[2].id, equals('sort-bill-2')); // Maret - lunas
        expect(result[3].id, equals('sort-bill-0')); // Jan - lunas (oldest)
        container.dispose();
      },
    );

    test(
      'Chronological comparison by year and month — year boundary',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Desember 2025', status: IuranStatus.belumBayar),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'November 2025', status: IuranStatus.lunas),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        expect(result[0].id, equals('sort-bill-1')); // Des 2025 - belumBayar
        expect(result[1].id, equals('sort-bill-0')); // Jan 2026 - lunas (newest paid)
        expect(result[2].id, equals('sort-bill-2')); // Nov 2025 - lunas (oldest paid)
        container.dispose();
      },
    );

    test(
      'Same-period bills: unpaid appears before paid',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Oktober 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Oktober 2026', status: IuranStatus.belumBayar),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        expect(result[0].id, equals('sort-bill-1')); // belumBayar
        expect(result[1].id, equals('sort-bill-0')); // lunas
        container.dispose();
      },
    );

    test(
      'Full example: August 2026 LUNAS, September BELUM BAYAR, '
      'October LUNAS, August SEBAGIAN, September LUNAS',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Agustus 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'September 2026', status: IuranStatus.belumBayar),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Oktober 2026', status: IuranStatus.lunas),
          _bill(index: 3, householdName: 'D', iuranType: 'Keamanan', periode: 'Agustus 2026', status: IuranStatus.sebagian),
          _bill(index: 4, householdName: 'E', iuranType: 'Keamanan', periode: 'September 2026', status: IuranStatus.lunas),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        // Expected order:
        // Unpaid/partial (oldest→newest):
        //   1. Agustus 2026 — SEBAGIAN (index 3)
        //   2. September 2026 — BELUM BAYAR (index 1)
        // Paid (newest→oldest):
        //   3. Oktober 2026 — LUNAS (index 2)
        //   4. September 2026 — LUNAS (index 4)
        //   5. Agustus 2026 — LUNAS (index 0)
        expect(result[0].id, equals('sort-bill-3')); // Agt 2026 - sebagian
        expect(result[1].id, equals('sort-bill-1')); // Sep 2026 - belumBayar
        expect(result[2].id, equals('sort-bill-2')); // Okt 2026 - lunas (newest)
        expect(result[3].id, equals('sort-bill-4')); // Sep 2026 - lunas
        expect(result[4].id, equals('sort-bill-0')); // Agt 2026 - lunas (oldest)
        container.dispose();
      },
    );

    test(
      'All unpaid/partial: only oldest→newest sort',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Desember 2025', status: IuranStatus.belumBayar),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.sebagian),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Oktober 2026', status: IuranStatus.belumBayar),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        expect(result[0].id, equals('sort-bill-0')); // Des 2025
        expect(result[1].id, equals('sort-bill-1')); // Jan 2026
        expect(result[2].id, equals('sort-bill-2')); // Okt 2026
        container.dispose();
      },
    );

    test(
      'All paid: only newest→oldest sort',
      () async {
        final container = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Maret 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
          _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Mei 2026', status: IuranStatus.lunas),
        ]);

        final result = await _getFilteredAsync(container, bills: []);

        expect(result[0].id, equals('sort-bill-2')); // Mei (newest)
        expect(result[1].id, equals('sort-bill-0')); // Maret
        expect(result[2].id, equals('sort-bill-1')); // Jan (oldest)
        container.dispose();
      },
    );

    test(
      'Sorting does not bypass active periode filter',
      () async {
        final container = _makeContainer(
          bills: [
            _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
            _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.belumBayar),
            _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Maret 2026', status: IuranStatus.belumBayar),
          ],
          extraOverrides: [
            iuranSelectedPeriodeProvider.overrideWith((_) => 'Februari 2026'),
          ],
        );

        final result = await _getFilteredAsync(container, bills: []);

        expect(result.length, equals(1));
        expect(result[0].id, equals('sort-bill-1'));
        expect(result[0].periode, equals('Februari 2026'));
        container.dispose();
      },
    );

    test(
      'Sorting does not bypass active status filter',
      () async {
        final container = _makeContainer(
          bills: [
            _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
            _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.lunas),
            _bill(index: 2, householdName: 'C', iuranType: 'Keamanan', periode: 'Maret 2026', status: IuranStatus.lunas),
          ],
          extraOverrides: [
            iuranSelectedStatusProvider.overrideWith((_) => 'lunas'),
          ],
        );

        final result = await _getFilteredAsync(container, bills: []);

        expect(result.length, equals(3));
        expect(result[0].id, equals('sort-bill-2')); // Maret (newest)
        expect(result[1].id, equals('sort-bill-1')); // Februari
        expect(result[2].id, equals('sort-bill-0')); // Jan (oldest)
        container.dispose();
      },
    );

    test(
      'Different input order produces same sorted result',
      () async {
        final containerA = _makeContainer(bills: [
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.belumBayar),
        ]);

        final containerB = _makeContainer(bills: [
          _bill(index: 1, householdName: 'B', iuranType: 'Keamanan', periode: 'Februari 2026', status: IuranStatus.belumBayar),
          _bill(index: 0, householdName: 'A', iuranType: 'Keamanan', periode: 'Januari 2026', status: IuranStatus.lunas),
        ]);

        final resultA = await _getFilteredAsync(containerA, bills: []);
        final resultB = await _getFilteredAsync(containerB, bills: []);

        expect(resultA[0].id, equals('sort-bill-1'));
        expect(resultA[1].id, equals('sort-bill-0'));
        expect(resultB[0].id, equals('sort-bill-1'));
        expect(resultB[1].id, equals('sort-bill-0'));
        containerA.dispose();
        containerB.dispose();
      },
    );
  });
}
