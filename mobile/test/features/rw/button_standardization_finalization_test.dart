import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image_picker/image_picker.dart';
import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_api_client.dart';
import 'package:social_finance/features/rw/data/rw_models.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_kas_screen.dart';
import 'package:social_finance/features/rw/presentation/screens/rw_notification_screen.dart';

class _FakeAuthRepo extends AuthRepository {
  _FakeAuthRepo(Map<String, dynamic> user)
    : super(
        authService: AuthService(ApiClient(baseUrl: 'http://test.local')),
        apiClient: ApiClient(baseUrl: 'http://test.local'),
      ) {
    state = AsyncData(user);
  }

  @override
  Future<void> reloadProfile() async {}
}

class _FakeRwApiClient extends RwApiClient {
  _FakeRwApiClient({required this.templates})
    : super(ApiClient(baseUrl: 'http://test.local'));

  final List<RwNotificationTemplate> templates;

  @override
  Future<RwKasReport> getKas(String rwId, {String? period}) async {
    return const RwKasReport(
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      netBalance: '3000000.00',
      transactionCount: 2,
    );
  }

  @override
  Future<RwKasReportDetail> getKasReportDetail(
    String rwId, {
    String? period,
    String? startDate,
    String? endDate,
  }) async {
    return const RwKasReportDetail(
      period: '2026-11',
      startDate: '',
      endDate: '',
      openingBalance: '1000000.00',
      totalIncome: '5000000.00',
      totalExpense: '2000000.00',
      netBalance: '3000000.00',
      totalBalance: '4000000.00',
      transactionCount: 2,
    );
  }

  @override
  Future<RwTransactionsResponse> getTransactions({
    required String rwId,
    String? period,
    String? startDate,
    String? endDate,
    int page = 1,
    int pageSize = 20,
    String? type,
    String? categoryId,
  }) async {
    return RwTransactionsResponse(
      data: [
        RwTransaction(
          id: 'tx-1',
          rtId: 'rt-1',
          rtName: 'RT 001',
          type: 'income',
          amount: '1000000.00',
          description: 'Setoran RT 001',
          locationName: 'Rekening BCA RW',
          status: 'posted',
          occurredAt: DateTime(2026, 11, 10),
          createdAt: DateTime(2026, 11, 10),
        ),
      ],
      pagination: const RwPaginationInfo(
        page: 1,
        pageSize: 20,
        total: 1,
        totalPages: 1,
      ),
    );
  }

  @override
  Future<List<RwFundLocation>> getLocations(
    String rwId, {
    String? category,
    bool? isActive,
  }) async {
    return [
      RwFundLocation(
        id: 'loc-1',
        rtId: '',
        rtName: 'RW',
        name: 'Rekening BCA RW',
        category: 'rekening',
        isActive: true,
        currentBalance: '4000000.00',
        createdAt: DateTime(2026, 1, 1),
      ),
    ];
  }

  @override
  Future<List<RwRtInfo>> getRTRs(String rwId) async {
    return const [
      RwRtInfo(id: 'rt-1', name: 'RT 001', rt: '001', rwNumber: 2, label: 'RT 001'),
    ];
  }

  @override
  Future<List<RwFinancialCategory>> getCategories({bool? isActive}) async {
    return const [
      RwFinancialCategory(
        id: 'cat-1',
        name: 'Operasional',
        type: 'expense',
        isActive: true,
      ),
    ];
  }

  @override
  Future<List<RwNotificationTemplate>> listNotificationTemplates(
    String rwId, {
    String? eventType,
  }) async {
    return templates;
  }

  @override
  Future<RwNotificationLogsPage> getNotifications(
    String rwId, {
    int page = 1,
    int pageSize = 20,
    String? eventType,
  }) async {
    return const RwNotificationLogsPage(
      items: [],
      page: 1,
      pageSize: 20,
      total: 0,
      totalPages: 1,
    );
  }
}

Map<String, dynamic> _rwAdminUser() => {
  'id': 'u-rw-1',
  'name': 'Ketua RW',
  'email': 'ketua.rw@example.com',
  'role': AppRole.pengurus,
  'system_role': 'pengurus',
  'jabatan': 'ketua',
  'rw_id': 'rw-002',
  'rw_membership_source': 'explicit',
  'rw_memberships': [
    {
      'rw_id': 'rw-002',
      'rw_number': 2,
      'jabatan': 'ketua',
      'source': 'explicit',
    },
  ],
  'rw_permissions': [
    'rw.kas.read',
    'rw.kas.write',
    'rw.notification.read',
    'rw.notification.write',
  ],
  'effective_permissions': [
    'rw.kas.read',
    'rw.kas.write',
    'rw.notification.read',
    'rw.notification.write',
  ],
};

({
  Color? bg,
  Color? fg,
  BorderSide? side,
  OutlinedBorder? shape,
  double height,
}) _extractSoftButtonMetrics(WidgetTester tester, Finder finder) {
  final btn = tester.widget<OutlinedButton>(finder);
  final style = btn.style!;
  return (
    bg: style.backgroundColor?.resolve({}),
    fg: style.foregroundColor?.resolve({}),
    side: style.side?.resolve({}),
    shape: style.shape?.resolve({}),
    height: tester.getSize(finder).height,
  );
}

void main() {
  group('Global Button Standardization Finalization', () {
    test('All Tutup buttons across mobile/lib use AppButton.danger (zero raw TextButton/OutlinedButton/FilledButton)', () {
      final libDir = Directory('lib');
      expect(libDir.existsSync(), isTrue);

      final rawTutupPattern = RegExp(r"Text\(\s*'Tutup'\s*\)");
      final offending = <String>[];
      var standardizedCount = 0;

      for (final entity in libDir.listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final content = entity.readAsStringSync();
        if (rawTutupPattern.hasMatch(content)) {
          offending.add(entity.path);
        }
        final matches = RegExp(r"AppButton\.danger\([^)]*text:\s*'Tutup'", dotAll: true)
            .allMatches(content);
        standardizedCount += matches.length;
      }

      expect(offending, isEmpty, reason: 'Found unstandardized raw Text(\'Tutup\') buttons: $offending');
      expect(standardizedCount, greaterThanOrEqualTo(28));
    });

    testWidgets('Tutup buttons in RW Kas & RW Notification render solid red AppColors.danger with white text and close cleanly', (tester) async {
      final api = _FakeRwApiClient(
        templates: [
          const RwNotificationTemplate(
            id: 't1',
            rwId: 'rw-002',
            name: 'Template Info',
            eventType: 'manual_push',
            channel: 'push',
            title: 'Judul',
            body: 'Isi pesan',
            isActive: true,
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(_rwAdminUser())),
            rwApiClientProvider.overrideWithValue(api),
          ],
          child: MaterialApp(
            theme: AppTheme.darkTheme,
            home: const RwKasScreen(rwId: 'rw-002'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Open transaction detail dialog to inspect its Tutup button
      final txTile = find.text('Setoran RT 001');
      await tester.ensureVisible(txTile);
      await tester.tap(txTile);
      await tester.pumpAndSettle();

      final tutupBtnFinder = find.descendant(
        of: find.byType(AlertDialog),
        matching: find.ancestor(
          of: find.text('Tutup'),
          matching: find.byType(ElevatedButton),
        ),
      );
      expect(tutupBtnFinder, findsOneWidget);
      final tutupBtn = tester.widget<ElevatedButton>(tutupBtnFinder);
      expect(tutupBtn.style?.backgroundColor?.resolve({}), equals(AppColors.danger));
      expect(tutupBtn.style?.foregroundColor?.resolve({}), equals(Colors.white));

      await tester.tap(tutupBtnFinder);
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });

    testWidgets(
      'RW Kas (Masuk & Keluar) and RW Notification (Tambah & Ubah Template) share identical soft-bordered Kamera & Galeri buttons, remove old labels, fire callbacks, and avoid narrow-screen overflow',
      (tester) async {
        for (final entry in [
          (name: 'Light', theme: AppTheme.lightTheme, expectedBg: AppColors.lightAccentSoft),
          (name: 'Dark', theme: AppTheme.darkTheme, expectedBg: AppColors.darkAccentSoft),
        ]) {
          // Standard mobile viewport for RW Kas screen
          await tester.binding.setSurfaceSize(const Size(390, 844));
          addTearDown(() => tester.binding.setSurfaceSize(null));

          final api = _FakeRwApiClient(
            templates: [
              const RwNotificationTemplate(
                id: 't-img',
                rwId: 'rw-002',
                name: 'Template Bergambar',
                eventType: 'manual_push',
                channel: 'push',
                title: 'Judul',
                body: 'Isi pesan',
                imageUrl: '/api/v1/notifications/images/rw-manual/rw-002/existing.png',
                isActive: true,
              ),
            ],
          );

          // 1) Inspect RW Kas Masuk & Kas Keluar dialogs
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(_rwAdminUser())),
                rwApiClientProvider.overrideWithValue(api),
              ],
              child: MaterialApp(
                key: ValueKey('kas_${entry.name}'),
                theme: entry.theme,
                home: const RwKasScreen(rwId: 'rw-002'),
              ),
            ),
          );
          await tester.pumpAndSettle();

          // Open Catat KAS Masuk RW
          final masukBtn = find.text('Catat Kas Masuk');
          await tester.ensureVisible(masukBtn);
          await tester.tap(masukBtn);
          await tester.pumpAndSettle();

          expect(find.text('Catat KAS Masuk RW'), findsOneWidget);
          final kasCamFinder = find.byKey(const Key('rwKasProofCameraBtn'));
          final kasGalFinder = find.byKey(const Key('rwKasProofGalleryBtn'));
          await tester.ensureVisible(kasCamFinder);
          expect(kasCamFinder, findsOneWidget);
          expect(kasGalFinder, findsOneWidget);
          expect(tester.takeException(), isNull);

          final kasCamMetrics = _extractSoftButtonMetrics(tester, kasCamFinder);
          final kasGalMetrics = _extractSoftButtonMetrics(tester, kasGalFinder);
          expect(kasCamMetrics.bg, equals(entry.expectedBg));
          expect(kasGalMetrics.bg, equals(entry.expectedBg));
          expect(kasCamMetrics.side, isNotNull);
          expect(kasCamMetrics.side!.width, greaterThan(0));
          expect(kasCamMetrics.side, equals(kasGalMetrics.side));
          expect(kasCamMetrics.height, equals(40.0));
          expect(kasGalMetrics.height, equals(40.0));
          expect(
            kasCamMetrics.shape,
            equals(RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.base))),
          );

          // Close Masuk dialog and verify Catat KAS Keluar RW too
          await tester.tap(find.text('Batal'));
          await tester.pumpAndSettle();

          final keluarBtn = find.text('Catat Kas Keluar');
          await tester.ensureVisible(keluarBtn);
          await tester.tap(keluarBtn);
          await tester.pumpAndSettle();

          expect(find.text('Catat KAS Keluar RW'), findsOneWidget);
          await tester.ensureVisible(kasCamFinder);
          expect(kasCamFinder, findsOneWidget);
          expect(kasGalFinder, findsOneWidget);
          expect(tester.takeException(), isNull);
          final kasKeluarCamMetrics = _extractSoftButtonMetrics(tester, kasCamFinder);
          expect(kasKeluarCamMetrics.bg, equals(kasCamMetrics.bg));
          expect(kasKeluarCamMetrics.side, equals(kasCamMetrics.side));
          expect(kasKeluarCamMetrics.height, equals(kasCamMetrics.height));

          await tester.tap(find.text('Batal'));
          await tester.pumpAndSettle();

          // 2) Inspect RW Notification Tambah Template & Ubah Template dialogs on narrow 320px screen
          await tester.binding.setSurfaceSize(const Size(320, 740));
          final pickedSources = <ImageSource>[];
          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(_rwAdminUser())),
                rwApiClientProvider.overrideWithValue(api),
              ],
              child: MaterialApp(
                key: ValueKey('notif_${entry.name}'),
                theme: entry.theme,
                home: RwNotificationScreen(
                  rwId: 'rw-002',
                  imagePickerOverride: (source) async {
                    pickedSources.add(source);
                    return (
                      bytes: base64Decode(
                        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
                      ),
                      filename: 'test.png',
                    );
                  },
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();

          // Open Tambah Template
          final createBtn = find.byKey(const Key('rwNotifCreateTemplateBtn'));
          await tester.ensureVisible(createBtn);
          await tester.tap(createBtn);
          await tester.pumpAndSettle();

          expect(
            find.descendant(
              of: find.byType(AlertDialog),
              matching: find.text('Tambah Template'),
            ),
            findsOneWidget,
          );
          final notifCamFinder = find.byKey(const Key('rwTemplatePickCameraBtn'));
          final notifGalFinder = find.byKey(const Key('rwTemplatePickGalleryBtn'));
          await tester.ensureVisible(notifCamFinder);
          expect(notifCamFinder, findsOneWidget);
          expect(notifGalFinder, findsOneWidget);

          // Old labels must NOT exist; new labels MUST exist
          expect(find.text('Kamera'), findsOneWidget);
          expect(find.text('Galeri'), findsOneWidget);
          expect(find.text('Ganti dari Galeri'), findsNothing);
          expect(find.text('Ganti dari galeri'), findsNothing);
          expect(find.text('Pilih dari Galeri'), findsNothing);
          expect(find.text('Ambil Foto'), findsNothing);

          // Exact visual parity with RW Kas Kamera/Galeri buttons
          final notifCamMetrics = _extractSoftButtonMetrics(tester, notifCamFinder);
          final notifGalMetrics = _extractSoftButtonMetrics(tester, notifGalFinder);
          expect(notifCamMetrics.bg, equals(kasCamMetrics.bg));
          expect(notifCamMetrics.fg, equals(kasCamMetrics.fg));
          expect(notifCamMetrics.side, equals(kasCamMetrics.side));
          expect(notifCamMetrics.shape, equals(kasCamMetrics.shape));
          expect(notifCamMetrics.height, equals(kasCamMetrics.height));
          expect(notifGalMetrics.bg, equals(kasGalMetrics.bg));
          expect(notifGalMetrics.side, equals(kasGalMetrics.side));
          expect(notifGalMetrics.height, equals(kasGalMetrics.height));
          expect(tester.takeException(), isNull);

          // Callbacks fire cleanly
          await tester.ensureVisible(notifCamFinder);
          await tester.tap(notifCamFinder);
          await tester.pumpAndSettle();
          await tester.ensureVisible(notifGalFinder);
          await tester.tap(notifGalFinder);
          await tester.pumpAndSettle();
          expect(pickedSources, equals([ImageSource.camera, ImageSource.gallery]));

          await tester.tap(find.text('Batal'));
          await tester.pumpAndSettle();

          // Open Ubah Template (existing image -> previously showed "Ganti dari Galeri")
          final editBtn = find.byKey(const Key('rwNotifTplEdit_t-img'));
          await tester.ensureVisible(editBtn);
          await tester.tap(editBtn);
          await tester.pumpAndSettle();

          expect(find.text('Ubah Template'), findsOneWidget);
          await tester.ensureVisible(notifCamFinder);
          expect(find.text('Kamera'), findsOneWidget);
          expect(find.text('Galeri'), findsOneWidget);
          expect(find.text('Ganti dari Galeri'), findsNothing);
          expect(find.text('Ganti dari galeri'), findsNothing);
          expect(find.text('Ambil Foto'), findsNothing);
          expect(tester.takeException(), isNull);

          await tester.tap(find.text('Batal'));
          await tester.pumpAndSettle();
        }
      },
    );
  });
}
