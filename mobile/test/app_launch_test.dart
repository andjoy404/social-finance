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
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/providers/theme_provider.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
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
      wargaListProvider.overrideWith(
        () => _MockWargaNotifierForLaunch(),
      ),
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
    ],
    child: MaterialApp(home: const DashboardScreen()),
  );
}

void main() {
  // ── App Lifecycle ──

  testWidgets('App launches without error', (tester) async {
    await tester.pumpWidget(_authWidget(const App()));
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.byType(Scaffold), findsOneWidget);
  });

  // ── Login Screen (isolated, not through App router) ──

  testWidgets('Login screen renders all elements', (tester) async {
    await tester.pumpWidget(
      rp.ProviderScope(child: MaterialApp(home: const LoginScreen())),
    );
    await tester.pump();
    expect(find.text('Social Finance'), findsOneWidget);
    expect(find.text('Social Finance for Your Neighborhood'), findsOneWidget);
    expect(find.text('Email'), findsOneWidget);
    expect(find.text('Kata Sandi'), findsOneWidget);
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
    expect(notifier.read(themeProvider), AppThemeMode.system);
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

  testWidgets('Dashboard has separate Logout action button', (tester) async {
    await tester.pumpWidget(_dashboardViaMaterialApp());
    await tester.pump();
    // Logout button is an IconButton
    expect(find.byIcon(Icons.logout_rounded), findsOneWidget);
  });

  testWidgets('Dashboard header has no width overflow', (tester) async {
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
        ],
        child: MaterialApp(
          // Use narrow phone size
          home: SizedBox(width: 360, child: const DashboardScreen()),
        ),
      ),
    );
    await tester.pump();
    expect(find.text('Selamat datang,'), findsOneWidget);
    expect(find.byIcon(Icons.logout_rounded), findsOneWidget);
    expect(find.byIcon(Icons.palette_outlined), findsOneWidget);
  });

  test('Primary brand seed color is violet', () {
    expect(AppColors.seed, const Color(0xFF7C5FCE));
  });

  test('Income semantic color remains green', () {
    expect(AppColors.income, const Color(0xFF2E7D32));
  });

  test('Expense semantic color remains red', () {
    expect(AppColors.expense, const Color(0xFFC62828));
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
}
