import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/dashboard/data/mock_dashboard_data.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/financial_summary_chart.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';

void main() {
  final points = List.generate(12, (index) {
    final months = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'];
    return MonthlyFinancePoint(
      month: months[index],
      income: index == 10 ? 10000000 : 0,
      expense: index == 10 ? 500000 : 0,
    );
  });

  final summary = DashboardSummary(
    saldoKas: 9500000,
    pemasukanBulanIni: 10000000,
    pengeluaranBulanIni: 500000,
    duesProgress: const DuesProgress(paid: 0, target: 0),
    monthlyData: points,
  );

  for (final width in [320.0, 360.0, 412.0]) {
    testWidgets('FinancialSummaryChart renders on Android at width $width without overflow', (tester) async {
      tester.view.physicalSize = Size(width, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(() {
        tester.view.resetPhysicalSize();
        tester.view.resetDevicePixelRatio();
      });

      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData(platform: TargetPlatform.android),
          home: Scaffold(
            body: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 20),
              child: FinancialSummaryChart(
                summary: summary,
                theme: ThemeData(platform: TargetPlatform.android),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byType(FinancialSummaryChart), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('FinancialSummaryChart handles tap tooltip on Android', (tester) async {
    tester.view.physicalSize = const Size(360, 800);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });

    await tester.pumpWidget(
      MaterialApp(
        theme: ThemeData(platform: TargetPlatform.android),
        home: Scaffold(
          body: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: FinancialSummaryChart(
              summary: summary,
              theme: ThemeData(platform: TargetPlatform.android),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Tap on month 'Nov' label
    await tester.tap(find.text('Nov'));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chart-bar-tooltip')), findsOneWidget);
    expect(find.textContaining('10.000.000'), findsOneWidget);
  });

  test('RwTransaction parseDate handles Postgres timestamptz formats', () {
    final tx1 = RwTransaction.fromJson({
      'id': '1',
      'rt_id': 'rt-1',
      'rt_name': 'RT 01',
      'type': 'income',
      'amount': '1000000',
      'description': 'Iuran Warga',
      'status': 'posted',
      'occurred_at': '2026-11-23T04:45:31.003812+07:00',
      'created_at': '2026-11-23T04:45:31.003812+07:00',
    });
    expect(tx1.occurredAt.year, 2026);
    expect(tx1.occurredAt.month, 11);

    final tx2 = RwTransaction.fromJson({
      'id': '2',
      'rt_id': 'rt-1',
      'rt_name': 'RT 01',
      'type': 'expense',
      'amount': '500000',
      'description': 'Biaya Sampah',
      'status': 'posted',
      'occurred_at': '2026-11-23 04:45:31.003812+07',
      'created_at': '2026-11-23 04:45:31.003812+07',
    });
    expect(tx2.occurredAt.year, 2026);
    expect(tx2.occurredAt.month, 11);
  });
}
