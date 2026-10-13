import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_form_screen.dart';

const _saveKey = ValueKey('iuran-save-button');

Widget buildForm({IuranRepository? repository}) => MaterialApp(
  home: ProviderScope(
    overrides: [
      if (repository != null)
        iuranRepositoryProvider.overrideWithValue(repository),
    ],
    child: const Scaffold(body: SafeArea(child: IuranFormScreen())),
  ),
);

class FakeIuranRepository extends IuranRepository {
  FakeIuranRepository({this.generatorResults = const [], this.createError})
    : super(apiClient: IuranApiClient(dio: Dio()));

  final List<Object> generatorResults;
  final Object? createError;
  final calls = <String>[];
  BackendDue? due;
  int generateAttempt = 0;
  int duesRefreshes = 0;
  int billsRefreshes = 0;

  @override
  Future<BackendDue> createDue({
    required String name,
    required int amount,
    required DuePeriodType periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
  }) async {
    if (createError != null) throw createError!;
    calls.add('createDue');
    return due ??= BackendDue(
      id: 'due-created-1',
      rtId: 'rt-1',
      name: name,
      amount: '$amount.00',
      periodType: periodType,
      paymentRule: paymentRule ?? PaymentRule.full,
      isVoluntary: isVoluntary ?? false,
      isActive: true,
    );
  }

  @override
  Future<GenerateBillsResult> generateBills({
    required String dueId,
    required String period,
    required String dueDate,
    required String idempotencyKey,
  }) async {
    calls.add('generateBills:$dueId:$period:$dueDate:$idempotencyKey');
    if (generateAttempt >= generatorResults.length) {
      return const GenerateBillsResult(generatedCount: 0, bills: []);
    }
    final result = generatorResults[generateAttempt++];
    if (result is Exception) throw result;
    return result as GenerateBillsResult;
  }

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async {
    calls.add('refreshDues');
    duesRefreshes++;
    return [];
  }

  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
  }) async {
    calls.add('refreshBills');
    billsRefreshes++;
    return [];
  }
}

Future<void> tapSave(WidgetTester tester) async {
  final button = find.byKey(_saveKey);
  final scrollable = find.byType(Scrollable).first;
  await tester.scrollUntilVisible(button, 500, scrollable: scrollable);
  final position = tester.state<ScrollableState>(scrollable).position;
  position.jumpTo(position.maxScrollExtent);
  await tester.pump();
  expect(button, findsOneWidget);
  await tester.tap(button);
  await tester.pumpAndSettle();
  position.jumpTo(0);
  await tester.pump();
}

Future<void> fillValid(WidgetTester tester) async {
  await tester.enterText(find.byType(TextField).at(0), 'Iuran Test');
  await tester.enterText(find.byType(TextField).at(1), '50000');
}

void main() {
  group('IuranFormScreen rendering and validation', () {
    testWidgets('renders fields and submit controls', (tester) async {
      await tester.pumpWidget(buildForm());
      await tester.pumpAndSettle();
      expect(find.text('Tambah Iuran'), findsOneWidget);
      expect(find.text('Periode'), findsOneWidget);
      expect(find.text('Tanggal Jatuh Tempo'), findsOneWidget);
      await tester.scrollUntilVisible(
        find.byKey(_saveKey),
        500,
        scrollable: find.byType(Scrollable).first,
      );
      expect(find.byKey(_saveKey), findsOneWidget);
    });

    testWidgets('validates empty, zero, negative, and valid values', (
      tester,
    ) async {
      await tester.pumpWidget(buildForm());
      await tester.pumpAndSettle();
      await tapSave(tester);
      expect(find.textContaining('Nama iuran harus diisi.'), findsOneWidget);
      expect(
        find.textContaining('Nominal harus angka yang valid.'),
        findsOneWidget,
      );

      await tester.enterText(find.byType(TextField).at(0), 'Iuran');
      await tester.enterText(find.byType(TextField).at(1), '0');
      await tapSave(tester);
      expect(find.textContaining('Nominal harus'), findsOneWidget);
    });
  });

  group('IuranFormScreen submission behavior', () {
    testWidgets(
      'orders create, generate, refresh and displays zero explicitly',
      (tester) async {
        final fake = FakeIuranRepository(
          generatorResults: const [
            GenerateBillsResult(generatedCount: 0, bills: []),
          ],
        );
        await tester.pumpWidget(buildForm(repository: fake));
        await tester.pumpAndSettle();
        await fillValid(tester);
        await tapSave(tester);

        expect(fake.calls.first, 'createDue');
        expect(fake.calls[1], startsWith('generateBills:due-created-1:2026-'));
        expect(fake.calls[1].split(':').last, isNotEmpty);
        expect(fake.calls, contains('refreshDues'));
        expect(fake.calls, contains('refreshBills'));
        expect(fake.duesRefreshes, greaterThanOrEqualTo(1));
        expect(fake.billsRefreshes, greaterThanOrEqualTo(1));
        expect(find.textContaining('Belum ada tagihan baru'), findsOneWidget);
      },
    );

    testWidgets('does not generate when createDue fails', (tester) async {
      final fake = FakeIuranRepository(
        createError: const FormatException('failed'),
      );
      await tester.pumpWidget(buildForm(repository: fake));
      await tester.pumpAndSettle();
      await fillValid(tester);
      await tapSave(tester);
      expect(fake.calls, isEmpty);
      expect(
        find.textContaining('tagihan belum berhasil dibuat'),
        findsNothing,
      );
    });

    testWidgets(
      'generator failure is partial success and retry reuses due/key',
      (tester) async {
        final fake = FakeIuranRepository(
          generatorResults: [
            const FormatException('generator failed'),
            const GenerateBillsResult(generatedCount: 1, bills: []),
          ],
        );
        await tester.pumpWidget(buildForm(repository: fake));
        await tester.pumpAndSettle();
        await fillValid(tester);
        await tapSave(tester);
        expect(
          find.textContaining('tagihan belum berhasil dibuat'),
          findsOneWidget,
        );
        await tapSave(tester);

        final generated = fake.calls
            .where((c) => c.startsWith('generateBills'))
            .toList();
        expect(fake.calls.where((c) => c == 'createDue'), hasLength(1));
        expect(generated, hasLength(2));
        expect(generated[0], generated[1]);
      },
    );

    testWidgets('yearly period uses YYYY and due date remains YYYY-MM-DD', (
      tester,
    ) async {
      final fake = FakeIuranRepository();
      await tester.pumpWidget(buildForm(repository: fake));
      await tester.pumpAndSettle();
      await fillValid(tester);
      await tester.tap(find.text('Tahunan'));
      await tapSave(tester);
      final call = fake.calls.singleWhere((c) => c.startsWith('generateBills'));
      expect(call, contains(':2026:'));
      expect(call, matches(RegExp(r'.*:2026:\d{4}-\d{2}-\d{2}:.*')));
    });
  });
}
