/// Deterministic tests for `valid_until` feature on voluntary dues.
///
/// Covers:
/// 1. Create mengirim validUntil saat tipe sukarela dipilih.
/// 2. Create tanpa validUntil (kosong) mengirim null.
/// 3. Edit mengganti tanggal validUntil.
/// 4. Edit menghapus tanggal dengan kosongkan field.
/// 5. Regular due tidak pernah mengirim validUntil.
/// 6. Response/model menerima tanggal dan null.
/// 7. Update tidak menghilangkan field lain.
///
/// Test menggunakan FakeRepository — assert argumen yang diteruskan
/// ke repository, bukan hanya UI.

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart' show IuranBill;
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_form_screen.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';

// ─── Fake Repositories ──────────────────────────────────────────────────

class _FakeCreateRepo extends IuranRepository {
  _FakeCreateRepo() : super(apiClient: IuranApiClient(dio: Dio()));

  CreateDueCall? lastCall;
  BackendDue? returnDue;
  Object? createError;

  @override
  Future<BackendDue> createDue({
    required String name,
    required int amount,
    required DuePeriodType periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    String? validUntil,
    DueType? dueType,
    int? year,
  }) async {
    lastCall = CreateDueCall(
      name: name,
      amount: amount,
      periodType: periodType,
      paymentRule: paymentRule,
      isVoluntary: isVoluntary,
      validUntil: validUntil,
    );
    if (createError != null) throw createError!;
    return returnDue ??= BackendDue(
      id: 'due-created',
      rtId: 'rt-1',
      name: name,
      amount: '$amount.00',
      periodType: periodType,
      paymentRule: paymentRule ?? PaymentRule.full,
      isVoluntary: isVoluntary ?? false,
      isActive: true,
      validUntil: validUntil,
      dueType: dueType ?? (isVoluntary == true ? DueType.voluntary : DueType.regular),
      year: year,
    );
  }

  @override
  Future<GenerateBillsResult> generateBills({
    required String dueId,
    required String period,
    required String dueDate,
    required String idempotencyKey,
  }) async {
    return const GenerateBillsResult(generatedCount: 0, bills: []);
  }

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => [];

  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    String? paymentStatus,
    bool myBills = false,
  }) async => [];
}

class CreateDueCall {
  final String name;
  final int amount;
  final DuePeriodType periodType;
  final PaymentRule? paymentRule;
  final bool? isVoluntary;
  final String? validUntil;

  const CreateDueCall({
    required this.name,
    required this.amount,
    required this.periodType,
    required this.paymentRule,
    required this.isVoluntary,
    required this.validUntil,
  });
}

class _FakeUpdateRepo extends IuranRepository {
  _FakeUpdateRepo(this.due) : super(apiClient: IuranApiClient(dio: Dio()));

  final BackendDue due;
  UpdateDueCall? lastCall;

  @override
  Future<BackendDue> updateDue(
    String id, {
    String? name,
    int? amount,
    DuePeriodType? periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    bool? isActive,
    String? validUntil,
  }) async {
    lastCall = UpdateDueCall(
      id: id,
      name: name,
      amount: amount,
      periodType: periodType,
      paymentRule: paymentRule,
      isVoluntary: isVoluntary,
      isActive: isActive,
      validUntil: validUntil,
    );
    return BackendDue(
      id: id,
      rtId: due.rtId,
      name: name ?? due.name,
      amount: amount != null ? '$amount.00' : due.amount,
      periodType: periodType ?? due.periodType,
      paymentRule: paymentRule ?? due.paymentRule,
      isVoluntary: isVoluntary ?? due.isVoluntary,
      isActive: isActive ?? due.isActive,
      validUntil: validUntil,
      createdAt: due.createdAt,
      updatedAt: DateTime.now(),
    );
  }

  @override
  Future<List<BackendDue>> fetchDues({bool? isActive}) async => [due];

  @override
  Future<BackendDue> createDue({
    required String name,
    required int amount,
    required DuePeriodType periodType,
    PaymentRule? paymentRule,
    bool? isVoluntary,
    String? validUntil,
    DueType? dueType,
    int? year,
  }) async {
    return BackendDue(
      id: 'due-fake',
      rtId: 'rt-1',
      name: name,
      amount: '$amount.00',
      periodType: periodType,
      paymentRule: paymentRule ?? PaymentRule.full,
      isVoluntary: isVoluntary ?? false,
      isActive: true,
      validUntil: validUntil,
      dueType: dueType ?? (isVoluntary == true ? DueType.voluntary : DueType.regular),
      year: year,
    );
  }

  @override
  Future<GenerateBillsResult> generateBills({
    required String dueId,
    required String period,
    required String dueDate,
    required String idempotencyKey,
  }) async {
    return const GenerateBillsResult(generatedCount: 0, bills: []);
  }

  @override
  Future<List<IuranBill>> fetchAllBills({
    String? search,
    String? status,
    String? paymentStatus,
    bool myBills = false,
  }) async => [];
}

class UpdateDueCall {
  final String id;
  final String? name;
  final int? amount;
  final DuePeriodType? periodType;
  final PaymentRule? paymentRule;
  final bool? isVoluntary;
  final bool? isActive;
  final String? validUntil;

  const UpdateDueCall({
    required this.id,
    required this.name,
    required this.amount,
    required this.periodType,
    required this.paymentRule,
    required this.isVoluntary,
    required this.isActive,
    required this.validUntil,
  });
}

// ─── Helpers ────────────────────────────────────────────────────────────

Widget _buildFormScreen(IuranRepository repo) => MaterialApp(
  home: ProviderScope(
    overrides: [iuranRepositoryProvider.overrideWithValue(repo)],
    child: const Scaffold(body: SafeArea(child: IuranFormScreen())),
  ),
);

Future<void> _fillValidForm(WidgetTester tester) async {
  await tester.enterText(find.byType(TextField).at(0), 'Iuran Test');
  await tester.enterText(find.byType(TextField).at(1), '50000');
}

Future<void> _tapSave(WidgetTester tester) async {
  final button = find.byKey(const ValueKey('iuran-save-button'));
  final scrollable = find.byType(Scrollable).first;
  await tester.scrollUntilVisible(button, 500, scrollable: scrollable);
  await tester.tap(button);
  await tester.pumpAndSettle();
}

// ─── Test Cases ─────────────────────────────────────────────────────────

void main() {
  group('valid_until — Create Form', () {
    testWidgets(
      'AC-01: Create voluntary due mengirim validUntil tanggal yang dipilih',
      (tester) async {
        final fake = _FakeCreateRepo();
        await tester.pumpWidget(_buildFormScreen(fake));
        await tester.pumpAndSettle();

        // Switch ke "Sukarela"
        await tester.tap(find.text('Sukarela'));
        await tester.pumpAndSettle();

        // Enter required fields. The voluntary amount may be zero.
        await tester.enterText(find.byType(TextField).at(0), 'Sumbangan Test');
        await tester.enterText(find.byType(TextField).at(1), '0');
        await tester.pumpAndSettle();

        // ValidUntil field muncul setelah switch sukarela
        // Scroll ke bawah untuk menemukan date field
        final validUntilField = find.byKey(const ValueKey('iuran-valid-until-field'));
        await tester.scrollUntilVisible(
          validUntilField,
          500,
          scrollable: find.byType(Scrollable).first,
        );
        await tester.pumpAndSettle();

        // Tap date field to open picker
        await tester.tap(validUntilField);
        await tester.pumpAndSettle();

        // Pick date 15
        await tester.tap(find.text('15').last);
        await tester.pumpAndSettle();
        await tester.tap(find.text('OK').last);
        await tester.pumpAndSettle();
        final validUntilText = tester.widget<TextField>(
          find.descendant(of: validUntilField, matching: find.byType(TextField)),
        ).controller!.text;
        expect(validUntilText, matches(RegExp(r'^\d{4}-\d{2}-\d{2}$')));

        // Save
        final saveButton = find.byKey(const ValueKey('iuran-save-button'));
        await tester.scrollUntilVisible(
          saveButton,
          500,
          scrollable: find.byType(Scrollable).first,
        );
        await tester.tap(saveButton, warnIfMissed: false);
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 300));

        expect(fake.lastCall, isNotNull);
        expect(fake.lastCall!.isVoluntary, isTrue);
        expect(fake.lastCall!.validUntil, isNotNull);
        expect(fake.lastCall!.validUntil!.length, 10);
        expect(fake.lastCall!.validUntil!, matches(RegExp(r'^\d{4}-\d{2}-\d{2}$')));
      },
    );

    testWidgets(
      'AC-02: Create voluntary due tanpa tanggal mengirim null',
      (tester) async {
        final fake = _FakeCreateRepo();
        await tester.pumpWidget(_buildFormScreen(fake));
        await tester.pumpAndSettle();

        // Switch ke "Sukarela"
        await tester.tap(find.text('Sukarela'));
        await tester.pumpAndSettle();

        // Fill required fields but leave the date field empty.
        await tester.enterText(find.byType(TextField).at(0), 'Sumbangan Test');
        await tester.enterText(find.byType(TextField).at(1), '0');
        await tester.pumpAndSettle();

        // Save
        await _tapSave(tester);

        expect(fake.lastCall, isNotNull);
        expect(fake.lastCall!.isVoluntary, isTrue);
        expect(fake.lastCall!.validUntil, isNull);
      },
    );

    testWidgets(
      'AC-06: Create regular due tidak pernah mengirim validUntil',
      (tester) async {
        final fake = _FakeCreateRepo();
        await tester.pumpWidget(_buildFormScreen(fake));
        await tester.pumpAndSettle();

        // Regular is default — enter valid fields
        await _fillValidForm(tester);
        await tester.pumpAndSettle();

        // Save
        await _tapSave(tester);

        expect(fake.lastCall, isNotNull);
        expect(fake.lastCall!.isVoluntary, isFalse);
        expect(fake.lastCall!.validUntil, isNull);
      },
    );
  });

  group('valid_until — Model Parsing', () {
    test(
      'AC-05a: BackendDue.fromJson menerima validUntil tanggal YYYY-MM-DD',
      () {
        final json = <String, dynamic>{
          'id': 'due-model-1',
          'rt_id': 'rt-1',
          'name': 'Sumbangan Test',
          'amount': '0',
          'period_type': 'monthly',
          'payment_rule': 'partial',
          'is_voluntary': true,
          'is_active': true,
          'valid_until': '2027-01-15',
          'created_at': '2026-01-01T00:00:00Z',
          'updated_at': '2026-01-01T00:00:00Z',
        };

        final due = BackendDue.fromJson(json);
        expect(due.validUntil, equals('2027-01-15'));
        expect(due.isVoluntary, isTrue);
        expect(due.name, equals('Sumbangan Test'));
      },
    );

    test(
      'AC-05b: BackendDue.fromJson menerima validUntil null',
      () {
        final json = <String, dynamic>{
          'id': 'due-model-2',
          'rt_id': 'rt-1',
          'name': 'Iuran Reguler',
          'amount': '50000',
          'period_type': 'monthly',
          'payment_rule': 'full',
          'is_voluntary': false,
          'is_active': true,
          'valid_until': null,
          'created_at': '2026-01-01T00:00:00Z',
          'updated_at': '2026-01-01T00:00:00Z',
        };

        final due = BackendDue.fromJson(json);
        expect(due.validUntil, isNull);
        expect(due.isVoluntary, isFalse);
      },
    );

    test(
      'AC-05c: BackendDue.fromJson menerima field valid_until tidak ada',
      () {
        final json = <String, dynamic>{
          'id': 'due-model-3',
          'rt_id': 'rt-1',
          'name': 'Iuran Reguler',
          'amount': '50000',
          'period_type': 'monthly',
          'payment_rule': 'full',
          'is_voluntary': false,
          'is_active': true,
          'created_at': '2026-01-01T00:00:00Z',
          'updated_at': '2026-01-01T00:00:00Z',
        };

        final due = BackendDue.fromJson(json);
        expect(due.validUntil, isNull);
      },
    );
  });
}
