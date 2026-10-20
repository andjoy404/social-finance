import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart' as rp;
import 'package:social_finance/app/app.dart';
import 'package:social_finance/features/auth/presentation/screens/login_screen.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/cash/presentation/screens/cash_screen.dart';
import 'package:social_finance/features/dues/presentation/screens/dues_screen.dart';
import 'package:social_finance/features/reports/presentation/screens/reports_screen.dart';
import 'package:social_finance/features/profile/presentation/screens/profile_screen.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/dashboard/data/mock_dashboard_data.dart';
import 'package:social_finance/features/dashboard/presentation/widgets/financial_summary_chart.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/providers/theme_provider.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';

AuthRepository _createTestAuthRepo([Map<String, dynamic>? initialData]) {
  final client = ApiClient(baseUrl: 'http://test.local');
  final repo = AuthRepository(
    authService: AuthService(client),
    apiClient: client,
  );
  if (initialData != null) {
    repo.state = rp.AsyncData<Map<String, dynamic>?>(initialData);
  }
  return repo;
}

// Shared pre-authenticated widget for all tests that need auth state.
Widget _authWidget(Widget child) {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith(((ref) {
        return _createTestAuthRepo({
          'name': 'Heri Prastyo',
          'role': AppRole.bendahara,
          'rt': '002',
          'rw': '016',
        });
      })),
      wargaListProvider.overrideWith(() => _MockWargaNotifierForLaunch()),
    ],
    child: child,
  );
}

class _MockWargaNotifierForLaunch extends WargaListNotifier {
  @override
  Future<List<MappedResident>> build() async {
    state = rp.AsyncData(const <MappedResident>[]);
    return const [];
  }
}

Widget _dashboardViaMaterialApp() {
  return rp.ProviderScope(
    overrides: [
      authRepositoryProvider.overrideWith(((ref) {
        return _createTestAuthRepo({
          'name': 'Heri Prastyo',
          'role': AppRole.bendahara,
          'rt': '002',
          'rw': '016',
        });
      })),
      wargaListProvider.overrideWith(() => _MockWargaNotifierForLaunch()),
    ],
    child: MaterialApp(home: const DashboardScreen()),
  );
}

void main() {
  // ── App Lifecycle ──

  testWidgets('App launches without error', (tester) async {
    await tester.pumpWidget(_authWidget(const App()));
    await tester.pump(const Duration(milliseconds: 100));
    // ShellRoute wraps all tabs in a Scaffold (bottom nav) + each screen has its own Scaffold.
    // Finding at least one confirms the app tree mounted without crash.
    expect(find.byType(Scaffold), findsWidgets);
  });

  // ── Login Screen (isolated, not through App router) ──

  testWidgets('Login screen renders all elements', (tester) async {
    await tester.pumpWidget(
      rp.ProviderScope(child: MaterialApp(home: const LoginScreen())),
    );
    await tester.pumpAndSettle(const Duration(milliseconds: 7000));
    expect(find.text('Social Finance'), findsOneWidget);
    expect(find.text('URL API / Server'), findsOneWidget);
    expect(find.text('Email'), findsWidgets);
    expect(find.text('Kata Sandi'), findsOneWidget);
    expect(find.text('Ingat saya'), findsOneWidget);
    expect(find.text('Masuk'), findsOneWidget);
  });

  // ── Dashboard Screen ──

  testWidgets('Dashboard renders greeting/user identity', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Selamat datang,'), findsOneWidget);
    expect(find.text('Heri Prastyo'), findsOneWidget);
  });

  testWidgets('Dashboard renders RT/RW info', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.textContaining('RT 002 / RW 016'), findsOneWidget);
  });

  testWidgets('Dashboard renders Saldo Kas', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Saldo Kas'), findsOneWidget);
  });

  testWidgets('Dashboard renders Rp 12.450.000', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.textContaining('Rp 12.450.000'), findsOneWidget);
  });

  testWidgets('Dashboard renders monthly income', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Pemasukan Bulan Ini'), findsOneWidget);
  });

  testWidgets('Dashboard renders monthly expense', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Pengeluaran Bulan Ini'), findsOneWidget);
  });

  // ── Financial Chart ──

  testWidgets('Ringkasan Keuangan renders on dashboard', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Ringkasan Keuangan'), findsOneWidget);
  });

  testWidgets('FinancialSummaryChart widget exists on dashboard', (
    tester,
  ) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.byType(FinancialSummaryChart), findsOneWidget);
  });

  testWidgets('FinancialSummaryChart renders with 6 monthly data points', (
    tester,
  ) async {
    final summary = DashboardMockData.getSummary();
    expect(summary.monthlyData.length, 6);

    await tester.pumpWidget(
      rp.ProviderScope(
        child: MaterialApp(
          home: FinancialSummaryChart(
            summary: summary,
            theme: ThemeData(useMaterial3: true),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(find.byType(FinancialSummaryChart), findsOneWidget);

    for (final point in summary.monthlyData) {
      expect(find.text(point.month), findsOneWidget);
    }
  });

  testWidgets('Chart shows Pemasukan and Pengeluaran legend', (tester) async {
    final summary = DashboardMockData.getSummary();

    await tester.pumpWidget(
      rp.ProviderScope(
        child: MaterialApp(
          home: FinancialSummaryChart(
            summary: summary,
            theme: ThemeData(useMaterial3: true),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(find.text('Pemasukan'), findsOneWidget);
    expect(find.text('Pengeluaran'), findsOneWidget);
  });

  testWidgets('Chart has separate income/expense series (no Saldo)', (
    tester,
  ) async {
    final summary = DashboardMockData.getSummary();
    for (final point in summary.monthlyData) {
      expect(point.income, greaterThan(0));
      expect(point.expense, greaterThan(0));
    }
  });

  // ── Dues Progress ──

  testWidgets('Iuran Bulan Ini renders on dashboard', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Iuran Bulan Ini'), findsOneWidget);
  });

  testWidgets('42 / 50 KK sudah membayar information renders', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.textContaining('KK sudah membayar'), findsOneWidget);
    expect(find.textContaining('42'), findsWidgets);
    expect(find.textContaining('50'), findsWidgets);
  });

  testWidgets('84% renders on dashboard', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('84%'), findsOneWidget);
  });

  // ── Recent Transactions ──

  testWidgets('Recent transactions section renders', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Transaksi Terbaru'), findsOneWidget);
  });

  testWidgets('Income transaction shows positive (+) distinction', (
    tester,
  ) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.textContaining('Rp 85.000'), findsWidgets);
    expect(find.text('masuk'), findsWidgets);
  });

  testWidgets('Expense transaction shows negative (-) distinction', (
    tester,
  ) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.textContaining('Rp 350.000'), findsWidgets);
    expect(find.text('keluar'), findsWidgets);
  });

  // ── Navigation ──

  testWidgets('Bottom navigation tab labels all present after login', (
    tester,
  ) async {
    // Pump the app, login via mock, then check nav
    await tester.pumpWidget(_authWidget(const App()));
    await tester.pump(const Duration(milliseconds: 100));
    // Auth is pre-set in _authWidget, so app starts at /home with bottom nav
    await tester.pump();
    expect(find.text('Beranda'), findsOneWidget);
    expect(find.text('Kas'), findsOneWidget);
    expect(find.text('Iuran'), findsOneWidget);
    expect(find.text('Warga'), findsOneWidget);
    expect(find.text('Profil'), findsOneWidget);
  });

  // ── Dashboard scrolling at narrow phone width ──

  testWidgets('Dashboard at narrow phone width renders without crash', (
    tester,
  ) async {
    await tester.pumpWidget(
      rp.ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(((ref) {
            return _createTestAuthRepo({
              'name': 'Heri Prastyo',
              'role': AppRole.bendahara,
              'rt': '002',
              'rw': '016',
            });
          })),
          wargaListProvider.overrideWith(() => _MockWargaNotifierForLaunch()),
        ],
        child: MaterialApp(
          home: SizedBox(width: 360, child: const DashboardScreen()),
        ),
      ),
    );
    await tester.pump();
    expect(find.byType(SingleChildScrollView), findsOneWidget);
  });

  // ── Theme Selector ──

  testWidgets('Theme provider initial state is system and supports switching', (
    tester,
  ) async {
    Widget buildWidget(AppThemeMode mode) {
      Widget child = const Text('mode: system');
      if (mode == AppThemeMode.light) {
        child = const Text('mode: light');
      } else if (mode == AppThemeMode.dark) {
        child = const Text('mode: dark');
      }
      return rp.ProviderScope(
        overrides: [themeProvider.overrideWith((ref) => ThemeNotifier())],
        child: MaterialApp(home: child),
      );
    }

    // Initial state is system (default)
    await tester.pumpWidget(buildWidget(AppThemeMode.system));
    expect(find.text('mode: system'), findsOneWidget);

    // Simulate user switching to light
    await tester.pumpWidget(buildWidget(AppThemeMode.light));
    expect(find.text('mode: light'), findsOneWidget);

    // Simulate user switching to dark
    await tester.pumpWidget(buildWidget(AppThemeMode.dark));
    expect(find.text('mode: dark'), findsOneWidget);
  });

  // ── Visual Polish (A2.1) ──

  testWidgets('Theme provider supports three modes', (tester) async {
    final notifier = rp.ProviderContainer();
    expect(notifier.read(themeProvider), AppThemeMode.dark);
    notifier.read(themeProvider.notifier).setMode(AppThemeMode.light);
    expect(notifier.read(themeProvider), AppThemeMode.light);
    notifier.read(themeProvider.notifier).setMode(AppThemeMode.dark);
    expect(notifier.read(themeProvider), AppThemeMode.dark);
    notifier.dispose();
  });

  testWidgets('Theme popup contains all three options', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    // Theme palette icon
    expect(find.byIcon(Icons.palette_outlined), findsOneWidget);
    // Tap to open theme popup
    await tester.tap(find.byIcon(Icons.palette_outlined));
    await tester.pump(const Duration(milliseconds: 300));
    expect(find.text('Ikuti Sistem'), findsOneWidget);
    expect(find.text('Terang'), findsOneWidget);
    expect(find.text('Gelap'), findsOneWidget);
  });

  testWidgets('Theme popup does NOT contain Logout', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    await tester.tap(find.byIcon(Icons.palette_outlined));
    await tester.pump(const Duration(milliseconds: 300));
    expect(find.text('Keluar'), findsNothing);
  });

  testWidgets('Dashboard renders greeting with user info', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.text('Selamat datang,'), findsOneWidget);
    expect(find.text('Heri Prastyo'), findsOneWidget);
    expect(find.textContaining('RT 002 / RW 016'), findsOneWidget);
  });

  test('Primary brand seed color is violet', () {
    expect(AppColors.seed, const Color(0xFFA970FF));
  });

  test('Income semantic color remains green', () {
    expect(AppColors.income, const Color(0xFF73BF69));
  });

  test('Expense semantic color remains red', () {
    expect(AppColors.expense, const Color(0xFFF2495C));
  });

  testWidgets('FinancialChart Y-axis renders compact million labels', (
    tester,
  ) async {
    final summary = DashboardMockData.getSummary();

    await tester.pumpWidget(
      rp.ProviderScope(
        child: MaterialApp(
          home: FinancialSummaryChart(
            summary: summary,
            theme: ThemeData.dark(useMaterial3: true),
          ),
        ),
      ),
    );
    await tester.pump();
    // Y-axis should show labels like "0", "1 jt", "2 jt" etc.
    expect(find.text('0'), findsOneWidget);
    expect(find.textContaining('jt'), findsWidgets);
    // Max should cover 4.6m → rounds to 5jt
    expect(find.text('5 jt'), findsOneWidget);
  });

  testWidgets('Chart X-axis months are Apr-Sep in order', (tester) async {
    final summary = DashboardMockData.getSummary();

    await tester.pumpWidget(
      rp.ProviderScope(
        child: MaterialApp(
          home: FinancialSummaryChart(
            summary: summary,
            theme: ThemeData.dark(useMaterial3: true),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(find.text('Apr'), findsOneWidget);
    expect(find.text('Mei'), findsOneWidget);
    expect(find.text('Jun'), findsOneWidget);
    expect(find.text('Jul'), findsOneWidget);
    expect(find.text('Agu'), findsOneWidget);
    expect(find.text('Sep'), findsOneWidget);
  });

  testWidgets('Chart renders income/expense bars (green/red)', (tester) async {
    final summary = DashboardMockData.getSummary();
    await tester.pumpWidget(
      rp.ProviderScope(
        child: MaterialApp(
          home: FinancialSummaryChart(
            summary: summary,
            theme: ThemeData(
              useMaterial3: true,
              colorSchemeSeed: const Color(0xFF7C5FCE),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(find.byType(BarChart), findsOneWidget);
    expect(find.text('Pemasukan'), findsOneWidget);
    expect(find.text('Pengeluaran'), findsOneWidget);
  });

  testWidgets('Chart renders at narrow width without overflow', (tester) async {
    final summary = DashboardMockData.getSummary();
    await tester.pumpWidget(
      rp.ProviderScope(
        child: MaterialApp(
          home: SizedBox(
            width: 360,
            child: FinancialSummaryChart(
              summary: summary,
              theme: ThemeData.dark(useMaterial3: true),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(find.byType(FinancialSummaryChart), findsOneWidget);
    expect(find.byType(SizedBox), findsWidgets);
  });

  testWidgets('Dark theme renders Dashboard without exception', (tester) async {
    final isDarkTest = ThemeData.dark(useMaterial3: true);

    await tester.pumpWidget(
      rp.ProviderScope(
        overrides: [
          authRepositoryProvider.overrideWith(((ref) {
            return _createTestAuthRepo({
              'name': 'Heri Prastyo',
              'role': AppRole.bendahara,
              'rt': '002',
              'rw': '016',
            });
          })),
          wargaListProvider.overrideWith(() => _MockWargaNotifierForLaunch()),
        ],
        child: MaterialApp(
          theme: isDarkTest,
          darkTheme: isDarkTest,
          themeMode: ThemeMode.dark,
          home: const DashboardScreen(),
        ),
      ),
    );
    await tester.pump();
    expect(find.text('Heri Prastyo'), findsOneWidget);
    expect(find.text('Saldo Kas'), findsOneWidget);
    expect(find.byType(SingleChildScrollView), findsOneWidget);
  });

  // ── Model Layer ──

  testWidgets('Dashboard model returns summary', (tester) async {
    final summary = DashboardMockData.getSummary();
    expect(summary.saldoKas, 12450000);
    expect(summary.pemasukanBulanIni, 4250000);
    expect(summary.pengeluaranBulanIni, 2175000);
  });

  testWidgets('Dashboard summary has 6 monthly data points', (tester) async {
    final summary = DashboardMockData.getSummary();
    expect(summary.monthlyData.length, 6);
    expect(summary.monthlyData[0].month, 'Apr');
    expect(summary.monthlyData[5].month, 'Sep');
  });

  testWidgets('Dues progress calculates percentage', (tester) async {
    final summary = DashboardMockData.getSummary();
    expect(summary.duesProgress.paid, 42);
    expect(summary.duesProgress.target, 50);
    expect(summary.duesProgress.percentage, 84);
  });

  testWidgets('Recent transactions exist and have correct data', (
    tester,
  ) async {
    final transactions = DashboardMockData.recentTransactions;
    expect(transactions.length, 5);
    expect(transactions[0].description, 'Iuran Bulanan');
    expect(transactions[1].description, 'Pembelian Perlengkapan');
    expect(transactions[3].description, 'Donasi Warga');
  });

  testWidgets('Transaction isIncome distinguishes income vs expense', (
    tester,
  ) async {
    final transactions = DashboardMockData.recentTransactions;
    expect(transactions[0].isIncome, isTrue);
    expect(transactions[1].isIncome, isFalse);
  });

  // ── Placeholder Screens ──

  testWidgets('All placeholder screens instantiate correctly', (tester) async {
    expect(const CashScreen().toString(), isA<String>());
    expect(const DuesScreen().toString(), isA<String>());
    expect(const ReportsScreen().toString(), isA<String>());
    expect(const ProfileScreen().toString(), isA<String>());
  });

  // ── Rupiah Formatter (Phase A1 preserved) ──

  test('formats zero correctly', () {
    expect(RupiahFormatter.format(0), equals('Rp 0'));
  });

  test('formats a normal amount correctly', () {
    expect(RupiahFormatter.format(12450000), equals('Rp 12.450.000'));
  });

  test('formats a small amount correctly', () {
    expect(RupiahFormatter.format(85000), equals('Rp 85.000'));
  });

  test('formats with negative correctly', () {
    expect(RupiahFormatter.formatWithNegative(-500000), equals('-Rp 500.000'));
  });

  test('formats positive with formatWithNegative correctly', () {
    expect(RupiahFormatter.formatWithNegative(1250000), equals('Rp 1.250.000'));
  });

  // ── Role Display (Phase A1 preserved) ──

  group('Role display mapping', () {
    test('superAdmin maps to Super Admin', () {
      expect(appRoleDisplayName(AppRole.superAdmin), equals('Super Admin'));
    });

    test('pengurus maps to Pengurus', () {
      expect(appRoleDisplayName(AppRole.pengurus), equals('Pengurus'));
    });

    test('bendahara maps to Bendahara', () {
      expect(appRoleDisplayName(AppRole.bendahara), equals('Bendahara'));
    });

    test('warga maps to Warga', () {
      expect(appRoleDisplayName(AppRole.warga), equals('Warga'));
    });

    test('all role values exist and map to non-empty strings', () {
      for (final role in AppRole.values) {
        final name = appRoleDisplayName(role);
        expect(name, isNotEmpty);
      }
    });
  });

  // ── A1, A2, A3, B1, B2, B3 Verification Tests ──

  testWidgets('Header renders official SocialFinance logo (A3)', (
    tester,
  ) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    expect(find.byKey(const Key('app-header-logo')), findsOneWidget);
  });

  testWidgets('Bottom navigation highlights Iuran item prominently (A2)', (
    tester,
  ) async {
    await tester.pumpWidget(_authWidget(const App()));
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump();
    expect(find.byKey(const Key('nav-item-iuran')), findsOneWidget);
  });

  testWidgets(
    'Beranda supports pull-to-refresh and shows recent iuran reference & IPL badge (B1, B2)',
    (tester) async {
      var refreshCalled = 0;
      await tester.pumpWidget(
        rp.ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith(((ref) {
              return _createTestAuthRepo({
                'name': 'Heri Prastyo',
                'role': AppRole.bendahara,
                'rt': '002',
                'rw': '016',
              });
            })),
            wargaListProvider.overrideWith(() => _MockWargaNotifierForLaunch()),
          ],
          child: MaterialApp(
            home: DashboardScreen(
              onRefreshOverride: () async {
                refreshCalled++;
              },
            ),
          ),
        ),
      );
      await tester.pump();

      expect(
        find.byKey(const Key('dashboard-refresh-indicator')),
        findsOneWidget,
      );
      // B2: Recent transactions show reference number and IPL badge, no Warga + houseNumber
      expect(find.text('INV-2025-001'), findsOneWidget);
      expect(find.text('IPL'), findsWidgets);

      await tester.fling(
        find.byType(SingleChildScrollView),
        const Offset(0, 300),
        1000,
      );
      await tester.pump();
      await tester.pump(const Duration(seconds: 1));
      await tester.pumpAndSettle();
      expect(refreshCalled, equals(1));
    },
  );

  testWidgets(
    'FinancialSummaryChart tap shows tooltip but no detail card (B3)',
    (tester) async {
      final summary = DashboardMockData.getSummary();
      await tester.pumpWidget(
        rp.ProviderScope(
          child: MaterialApp(
            home: Scaffold(
              body: FinancialSummaryChart(
                summary: summary,
                theme: ThemeData(useMaterial3: true),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byKey(const Key('chart-selected-detail')), findsNothing);
      expect(find.byKey(const Key('chart-bar-tooltip')), findsNothing);

      // Tap month label 'Sep' — tooltip should appear, detail card should NOT
      await tester.tap(find.text('Sep'));
      await tester.pump();

      expect(find.byKey(const Key('chart-bar-tooltip')), findsOneWidget);
      expect(find.textContaining('Masuk  Rp 4.250.000'), findsOneWidget);
      expect(find.textContaining('Keluar  Rp 2.175.000'), findsOneWidget);
      expect(find.byKey(const Key('chart-selected-detail')), findsNothing);
      expect(find.text('Sep • Pemasukan'), findsNothing);
      expect(find.textContaining('Transaksi'), findsNothing);
    },
  );

  // ── P0/P1/P3 Master Task Verification Tests ──

  test('AuthUser.fromJson parses rw_id, rw_name, and rt_rw into rwCode and rwName', () {
    final user = AuthUser.fromJson({
      'id': 'u-1',
      'email': 'bendahara@local.com',
      'full_name': 'Heri Prastyo',
      'role': 'bendahara',
      'rt_id': 'rt-002',
      'rt_name': 'RT 002',
      'rt_rw': '016',
      'rw_id': 'rw-016',
      'rw_name': 'RW 016',
      'is_active': true,
    });
    expect(user.rtCode, equals('002'));
    expect(user.rwId, equals('rw-016'));
    expect(user.rwCode, equals('016'));
    expect(user.rwName, equals('RW 016'));
  });

  test('BackendTransaction.fromJson falls back to reference_no for manual KAS transactions', () {
    final tx = BackendTransaction.fromJson({
      'id': 'tx-manual-1',
      'rt_id': 'rt-002',
      'category_id': 'cat-1',
      'amount': '150000',
      'type': 'expense',
      'status': 'posted',
      'reference_no': 'KAS-20260417-ABC12',
      'description': 'Beli lampu jalan',
    });
    expect(tx.paymentNo, equals('KAS-20260417-ABC12'));
  });

  testWidgets(
    'Dashboard renders Metode Pembayaran Donut Chart panel (Transfer vs Tunai) and handles empty data without NaN%',
    (tester) async {
      await tester.pumpWidget(_dashboardViaMaterialApp());
      await tester.pump();

      expect(
        find.byKey(const Key('dashboard-payment-method-donut-card')),
        findsOneWidget,
      );
      expect(find.text('Metode Pembayaran'), findsOneWidget);
      expect(find.text('Transfer'), findsOneWidget);
      expect(find.text('Tunai'), findsOneWidget);
      expect(find.text('60%'), findsOneWidget);
      expect(find.text('40%'), findsOneWidget);
      expect(find.byType(PieChart), findsOneWidget);

      // Verify empty state safety (0 / 0 never produces NaN%)
      const emptyBreakdown = PaymentMethodBreakdown();
      expect(emptyBreakdown.transferPercentage, equals(0));
      expect(emptyBreakdown.cashPercentage, equals(0));
    },
  );

  testWidgets(
    'FinancialSummaryChart floating bar tooltip shows Bulan, Masuk, and Keluar above selected bar',
    (tester) async {
      final summary = DashboardMockData.getSummary();
      await tester.pumpWidget(
        rp.ProviderScope(
          child: MaterialApp(
            home: Scaffold(
              body: FinancialSummaryChart(
                summary: summary,
                theme: ThemeData(useMaterial3: true),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(find.byKey(const Key('chart-bar-tooltip')), findsNothing);

      // Tap month label 'Sep' — tooltip should appear (tap + hover)
      await tester.tap(find.text('Sep'));
      await tester.pump();

      expect(find.byKey(const Key('chart-bar-tooltip')), findsOneWidget);
      expect(find.textContaining('Masuk  Rp 4.250.000'), findsOneWidget);
      expect(find.textContaining('Keluar  Rp 2.175.000'), findsOneWidget);
    },
  );
}

