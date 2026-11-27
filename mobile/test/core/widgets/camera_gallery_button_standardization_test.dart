import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
// ignore: depend_on_referenced_packages
import 'package:image_picker_platform_interface/image_picker_platform_interface.dart';

import 'package:social_finance/app/theme/app_theme.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/widgets/app_button.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/fund_location/data/fund_location_repository.dart';
import 'package:social_finance/features/fund_location/data/models/fund_location_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_payment_screen.dart';
import 'package:social_finance/features/kas/data/kas_api_client.dart';
import 'package:social_finance/features/kas/data/kas_api_models.dart';
import 'package:social_finance/features/kas/data/kas_repository.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_keluar_screen.dart';
import 'package:social_finance/features/kas/presentation/screens/kas_masuk_screen.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_management_screen.dart';

final Uint8List _kValid1x1Png = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
);

class _FakeImagePickerPlatform extends ImagePickerPlatform {
  _FakeImagePickerPlatform(this.tempFilePath);

  final String tempFilePath;
  final List<ImageSource> pickedSources = <ImageSource>[];

  @override
  Future<XFile?> getImageFromSource({
    required ImageSource source,
    ImagePickerOptions options = const ImagePickerOptions(),
  }) async {
    pickedSources.add(source);
    return XFile.fromData(
      _kValid1x1Png,
      name: 'proof_${source.name}.png',
      path: tempFilePath,
    );
  }
}

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

class _FakeKasApiClient extends KasApiClient {
  _FakeKasApiClient() : super(dio: Dio(BaseOptions(baseUrl: 'http://test.local')));

  @override
  Future<List<BackendCategory>> listCategories({bool? isActive}) async {
    return const [
      BackendCategory(
        id: 'cat-income-1',
        rtId: 'rt-002',
        name: 'Iuran Warga',
        type: 'income',
        isActive: true,
      ),
      BackendCategory(
        id: 'cat-expense-1',
        rtId: 'rt-002',
        name: 'Operasional RT',
        type: 'expense',
        isActive: true,
      ),
    ];
  }
}

class _FakeNotificationRepository extends NotificationRepository {
  _FakeNotificationRepository()
    : super(apiClient: NotificationApiClient(dio: Dio()));

  final List<String> uploadedFilenames = <String>[];

  @override
  Future<List<SystemNotificationType>> fetchSystemTypes() async {
    return const [
      SystemNotificationType(
        eventType: 'manual_push',
        name: 'Pengumuman Pengurus',
        trigger: 'Dikirim manual oleh pengurus RT',
        recipientsRule: 'Seluruh warga RT',
        defaultTitle: 'Pengumuman RT',
        defaultBody: 'Isi pengumuman warga.',
        allowedVariables: [],
      ),
    ];
  }

  @override
  Future<List<NotificationTemplate>> fetchTemplates() async => const [];

  @override
  Future<NotificationLogsResponse> fetchManagementHistory({
    required String category,
    int page = 1,
    int perPage = 50,
  }) async {
    return const NotificationLogsResponse(
      data: [],
      page: 1,
      perPage: 50,
      total: 0,
      totalPages: 1,
    );
  }

  @override
  Future<String> uploadImage({
    required String eventType,
    required List<int> bytes,
    required String filename,
  }) async {
    uploadedFilenames.add(filename);
    return '/api/v1/notifications/images/manual_push/$filename';
  }
}

List<FundLocation> _sampleLocations() => [
  FundLocation(
    id: 'loc-1',
    rtId: 'rt-002',
    name: 'Kas Di Tangan Bendahara',
    category: FundLocationCategory.coh,
    isActive: true,
    currentBalance: '2000000.00',
    createdAt: DateTime(2026, 1, 1),
    updatedAt: DateTime(2026, 1, 1),
  ),
  FundLocation(
    id: 'loc-2',
    rtId: 'rt-002',
    name: 'Rekening BCA RT',
    category: FundLocationCategory.rekening,
    bankName: 'BCA',
    accountNumber: '1234567890',
    accountHolder: 'RT 002',
    isActive: true,
    currentBalance: '5000000.00',
    createdAt: DateTime(2026, 1, 1),
    updatedAt: DateTime(2026, 1, 1),
  ),
];

Map<String, dynamic> _rtPengurusUser() => {
  'id': 'u-pengurus-1',
  'name': 'Ketua RT',
  'email': 'ketua.rt@example.com',
  'role': AppRole.pengurus,
  'system_role': 'pengurus',
  'jabatan': 'ketua',
  'rt': '002',
  'rt_id': 'rt-002',
  'effective_permissions': [
    'kas.read',
    'kas.write',
    'notification.read',
    'notification.manage',
    'notification.send',
  ],
};

void _assertAccentSoftStandard(
  WidgetTester tester,
  Finder buttonKeyFinder, {
  required Brightness brightness,
  required String expectedLabel,
  required IconData expectedIcon,
}) {
  expect(buttonKeyFinder, findsOneWidget);

  // Must be backed by AppButton with accentSoft variant
  final appBtnFinder = find.ancestor(
    of: buttonKeyFinder,
    matching: find.byType(AppButton),
  );
  expect(appBtnFinder, findsOneWidget);
  final appBtn = tester.widget<AppButton>(appBtnFinder);
  expect(appBtn.variant, equals(AppButtonVariant.accentSoft));
  expect(appBtn.text, equals(expectedLabel));
  expect(appBtn.icon, equals(expectedIcon));
  expect(appBtn.size, equals(AppButtonSize.medium));

  // Verify rendered OutlinedButton style tokens
  final outlinedBtn = tester.widget<OutlinedButton>(buttonKeyFinder);
  final style = outlinedBtn.style!;
  final expectedBg = brightness == Brightness.dark
      ? AppColors.darkAccentSoft
      : AppColors.lightAccentSoft;
  final expectedFg = brightness == Brightness.dark
      ? AppColors.darkText
      : AppColors.lightText;
  final expectedBorderColor = AppColors.accent.withValues(
    alpha: brightness == Brightness.dark ? 0.55 : 0.45,
  );

  expect(style.backgroundColor?.resolve({}), equals(expectedBg));
  expect(style.foregroundColor?.resolve({}), equals(expectedFg));
  expect(
    style.side?.resolve({}),
    equals(BorderSide(color: expectedBorderColor, width: 1.25)),
  );
  expect(
    style.shape?.resolve({}),
    equals(RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppRadius.base))),
  );
  expect(style.textStyle?.resolve({})?.fontSize, equals(14.0));
  expect(style.textStyle?.resolve({})?.fontWeight, equals(FontWeight.w600));
  expect(tester.getSize(buttonKeyFinder).height, equals(40.0));

  // Verify violet accent icon inside button
  final iconFinder = find.descendant(
    of: buttonKeyFinder,
    matching: find.byIcon(expectedIcon),
  );
  expect(iconFinder, findsOneWidget);
  expect(tester.widget<Icon>(iconFinder).color, equals(AppColors.accent));
}

void main() {
  late Directory tempDir;
  late File tempPngFile;
  late ImagePickerPlatform originalPickerPlatform;
  late _FakeImagePickerPlatform fakePicker;

  setUp(() {
    tempDir = Directory.systemTemp.createTempSync('sf_camera_gallery_test_');
    tempPngFile = File('${tempDir.path}/sample_proof.png')
      ..writeAsBytesSync(_kValid1x1Png);
    originalPickerPlatform = ImagePickerPlatform.instance;
    fakePicker = _FakeImagePickerPlatform(tempPngFile.path);
    ImagePickerPlatform.instance = fakePicker;
  });

  tearDown(() {
    ImagePickerPlatform.instance = originalPickerPlatform;
    if (tempDir.existsSync()) {
      tempDir.deleteSync(recursive: true);
    }
  });

  group('Camera & Gallery Button Standardization (All 4 Remaining Screens + Global Audit)', () {
    test('Global audit: zero inline OutlinedButton.icon for Kamera/Galeri and all 6 pairs use AppButton.accentSoft', () {
      final libDir = Directory('lib');
      expect(libDir.existsSync(), isTrue);

      final expectedStandardizedFiles = <String>{
        'lib/features/kas/presentation/screens/kas_masuk_screen.dart',
        'lib/features/kas/presentation/screens/kas_keluar_screen.dart',
        'lib/features/rw/presentation/screens/rw_kas_screen.dart',
        'lib/features/notification/presentation/screens/notification_management_screen.dart',
        'lib/features/rw/presentation/screens/rw_notification_screen.dart',
        'lib/features/iuran/presentation/screens/iuran_payment_screen.dart',
      };

      final foundStandardizedFiles = <String>{};
      final inlineOffenders = <String>[];

      final accentSoftKameraPattern = RegExp(
        r"AppButton\.accentSoft\([\s\S]{0,200}?text:\s*'Kamera'",
      );
      final accentSoftGaleriPattern = RegExp(
        r"AppButton\.accentSoft\([\s\S]{0,200}?text:\s*'Galeri'",
      );
      final inlineOutlinedKameraGaleriPattern = RegExp(
        r"OutlinedButton\.icon\([\s\S]{0,600}?Text\(\s*'(?:Kamera|Galeri)'",
      );

      for (final entity in libDir.listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final normalizedPath = entity.path.replaceAll('\\', '/');
        final content = entity.readAsStringSync();

        if (inlineOutlinedKameraGaleriPattern.hasMatch(content)) {
          inlineOffenders.add(normalizedPath);
        }
        if (accentSoftKameraPattern.hasMatch(content) &&
            accentSoftGaleriPattern.hasMatch(content)) {
          foundStandardizedFiles.add(normalizedPath);
        }
      }

      expect(
        inlineOffenders,
        isEmpty,
        reason: 'Found inline OutlinedButton.icon for Kamera/Galeri: $inlineOffenders',
      );
      expect(foundStandardizedFiles, equals(expectedStandardizedFiles));
    });

    testWidgets(
      'Kamera & Galeri AppButton.accentSoft row renders without overflow on narrow 320px width (both Light & Dark)',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(320, 568));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        for (final entry in [
          (name: 'Light', theme: AppTheme.lightTheme, brightness: Brightness.light),
          (name: 'Dark', theme: AppTheme.darkTheme, brightness: Brightness.dark),
        ]) {
          await tester.pumpWidget(
            MaterialApp(
              key: ValueKey('narrow_320_${entry.name}'),
              theme: entry.theme,
              home: Scaffold(
                body: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Row(
                    children: [
                      Expanded(
                        child: AppButton.accentSoft(
                          buttonKey: const Key('narrowCamBtn'),
                          text: 'Kamera',
                          icon: Icons.camera_alt_outlined,
                          size: AppButtonSize.medium,
                          fullWidth: true,
                          onPressed: () {},
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: AppButton.accentSoft(
                          buttonKey: const Key('narrowGalBtn'),
                          text: 'Galeri',
                          icon: Icons.photo_library_outlined,
                          size: AppButtonSize.medium,
                          fullWidth: true,
                          onPressed: () {},
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          );
          await tester.pumpAndSettle();

          _assertAccentSoftStandard(
            tester,
            find.byKey(const Key('narrowCamBtn')),
            brightness: entry.brightness,
            expectedLabel: 'Kamera',
            expectedIcon: Icons.camera_alt_outlined,
          );
          _assertAccentSoftStandard(
            tester,
            find.byKey(const Key('narrowGalBtn')),
            brightness: entry.brightness,
            expectedLabel: 'Galeri',
            expectedIcon: Icons.photo_library_outlined,
          );
          expect(tester.takeException(), isNull);
        }
      },
    );

    testWidgets(
      'KAS Masuk RT uses AppButton.accentSoft for Kamera & Galeri in Light/Dark mode and preserves pick/preview/replace/remove',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(800, 900));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        for (final entry in [
          (name: 'Light', theme: AppTheme.lightTheme, brightness: Brightness.light),
          (name: 'Dark', theme: AppTheme.darkTheme, brightness: Brightness.dark),
        ]) {
          fakePicker.pickedSources.clear();
          final fakeKasRepo = KasRepository(apiClient: _FakeKasApiClient());

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                kasRepositoryProvider.overrideWithValue(fakeKasRepo),
                fundLocationsProvider.overrideWith((_) async => _sampleLocations()),
              ],
              child: MaterialApp(
                key: ValueKey('kas_masuk_${entry.name}'),
                theme: entry.theme,
                home: const KasMasukScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          final camFinder = find.byKey(const Key('kasMasukProofCameraBtn'));
          final galFinder = find.byKey(const Key('kasMasukProofGalleryBtn'));
          await tester.ensureVisible(camFinder);

          _assertAccentSoftStandard(
            tester,
            camFinder,
            brightness: entry.brightness,
            expectedLabel: 'Kamera',
            expectedIcon: Icons.camera_alt_outlined,
          );
          _assertAccentSoftStandard(
            tester,
            galFinder,
            brightness: entry.brightness,
            expectedLabel: 'Galeri',
            expectedIcon: Icons.photo_library_outlined,
          );
          expect(tester.takeException(), isNull);

          // Pick from camera -> preview + replace + remove appear
          await tester.tap(camFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.camera]));
          expect(find.text('Ganti Foto'), findsOneWidget);
          expect(find.text('Hapus'), findsOneWidget);

          // Replace photo
          await tester.ensureVisible(find.text('Ganti Foto'));
          await tester.tap(find.text('Ganti Foto'));
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources.length, equals(2));

          // Remove photo -> Kamera & Galeri return
          await tester.ensureVisible(find.text('Hapus'));
          await tester.tap(find.text('Hapus'));
          await tester.pumpAndSettle();
          expect(galFinder, findsOneWidget);

          // Pick from gallery
          await tester.ensureVisible(galFinder);
          await tester.tap(galFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources.last, equals(ImageSource.gallery));
          expect(tester.takeException(), isNull);
        }
      },
    );

    testWidgets(
      'KAS Keluar RT uses AppButton.accentSoft for Kamera & Galeri in Light/Dark mode and preserves pick/preview/replace/remove',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(800, 900));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        for (final entry in [
          (name: 'Light', theme: AppTheme.lightTheme, brightness: Brightness.light),
          (name: 'Dark', theme: AppTheme.darkTheme, brightness: Brightness.dark),
        ]) {
          fakePicker.pickedSources.clear();
          final fakeKasRepo = KasRepository(apiClient: _FakeKasApiClient());

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                kasRepositoryProvider.overrideWithValue(fakeKasRepo),
                fundLocationsProvider.overrideWith((_) async => _sampleLocations()),
              ],
              child: MaterialApp(
                key: ValueKey('kas_keluar_${entry.name}'),
                theme: entry.theme,
                home: const KasKeluarScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          final camFinder = find.byKey(const Key('kasKeluarProofCameraBtn'));
          final galFinder = find.byKey(const Key('kasKeluarProofGalleryBtn'));
          await tester.ensureVisible(camFinder);

          _assertAccentSoftStandard(
            tester,
            camFinder,
            brightness: entry.brightness,
            expectedLabel: 'Kamera',
            expectedIcon: Icons.camera_alt_outlined,
          );
          _assertAccentSoftStandard(
            tester,
            galFinder,
            brightness: entry.brightness,
            expectedLabel: 'Galeri',
            expectedIcon: Icons.photo_library_outlined,
          );
          expect(tester.takeException(), isNull);

          // Pick from camera -> preview + replace + remove appear
          await tester.tap(camFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.camera]));
          expect(find.text('Ganti Foto'), findsOneWidget);
          expect(find.text('Hapus'), findsOneWidget);

          // Remove photo -> pick from gallery
          await tester.ensureVisible(find.text('Hapus'));
          await tester.tap(find.text('Hapus'));
          await tester.pumpAndSettle();
          await tester.ensureVisible(galFinder);
          await tester.tap(galFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.camera, ImageSource.gallery]));
          expect(tester.takeException(), isNull);
        }
      },
    );

    testWidgets(
      'Notification Management uses AppButton.accentSoft for Kamera & Galeri in Light/Dark mode and preserves upload/preview/remove',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(800, 900));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        for (final entry in [
          (name: 'Light', theme: AppTheme.lightTheme, brightness: Brightness.light),
          (name: 'Dark', theme: AppTheme.darkTheme, brightness: Brightness.dark),
        ]) {
          fakePicker.pickedSources.clear();
          final fakeRepo = _FakeNotificationRepository();

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWith((_) => _FakeAuthRepo(_rtPengurusUser())),
                notificationRepositoryProvider.overrideWithValue(fakeRepo),
              ],
              child: MaterialApp(
                key: ValueKey('notif_mgmt_${entry.name}'),
                theme: entry.theme,
                home: const NotificationManagementScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          // Navigate to TEMPLATE -> KUSTOM sub-tab -> Tambah Template
          await tester.tap(find.text('TEMPLATE'));
          await tester.pumpAndSettle();
          await tester.tap(find.text('KUSTOM'));
          await tester.pumpAndSettle();

          final addBtn = find.text('Tambah Template');
          await tester.ensureVisible(addBtn);
          await tester.tap(addBtn);
          await tester.pumpAndSettle();

          final camFinder = find.byKey(const Key('notifMgmtProofCameraBtn'));
          final galFinder = find.byKey(const Key('notifMgmtProofGalleryBtn'));
          await tester.ensureVisible(camFinder);

          _assertAccentSoftStandard(
            tester,
            camFinder,
            brightness: entry.brightness,
            expectedLabel: 'Kamera',
            expectedIcon: Icons.camera_alt_outlined,
          );
          _assertAccentSoftStandard(
            tester,
            galFinder,
            brightness: entry.brightness,
            expectedLabel: 'Galeri',
            expectedIcon: Icons.photo_library_outlined,
          );
          expect(tester.takeException(), isNull);

          // Tap Kamera -> triggers camera pick + repo.uploadImage + preview + Hapus Gambar
          await tester.tap(camFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.camera]));
          expect(fakeRepo.uploadedFilenames, equals(['sample_proof.png']));
          expect(find.text('Hapus Gambar'), findsOneWidget);

          // Remove image -> Kamera & Galeri reappear
          await tester.ensureVisible(find.text('Hapus Gambar'));
          await tester.tap(find.text('Hapus Gambar'));
          await tester.pumpAndSettle();
          expect(galFinder, findsOneWidget);

          // Tap Galeri -> triggers gallery pick + repo.uploadImage
          await tester.ensureVisible(galFinder);
          await tester.tap(galFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.camera, ImageSource.gallery]));
          expect(fakeRepo.uploadedFilenames, equals(['sample_proof.png', 'sample_proof.png']));
          expect(tester.takeException(), isNull);
        }
      },
    );

    testWidgets(
      'Iuran Payment uses AppButton.accentSoft for Galeri & Kamera in Light/Dark mode and preserves pick/preview/remove',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(800, 900));
        addTearDown(() => tester.binding.setSurfaceSize(null));

        const billId = 'bill-test-1';
        final testBill = IuranBill(
          id: billId,
          householdName: 'Bpk. Ahmad',
          iuranType: 'Iuran Keamanan',
          periode: 'November 2026',
          nominal: 50000,
          paidAmount: 0,
          status: IuranStatus.belumBayar,
        );

        for (final entry in [
          (name: 'Light', theme: AppTheme.lightTheme, brightness: Brightness.light),
          (name: 'Dark', theme: AppTheme.darkTheme, brightness: Brightness.dark),
        ]) {
          fakePicker.pickedSources.clear();

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                billDetailProvider(billId).overrideWith((_) async => testBill),
              ],
              child: MaterialApp(
                key: ValueKey('iuran_pay_${entry.name}'),
                theme: entry.theme,
                home: const IuranPaymentScreen(billId: billId),
              ),
            ),
          );
          await tester.pumpAndSettle();

          final galFinder = find.byKey(const Key('iuranPaymentProofGalleryBtn'));
          final camFinder = find.byKey(const Key('iuranPaymentProofCameraBtn'));
          await tester.scrollUntilVisible(
            galFinder,
            300,
            scrollable: find.byType(Scrollable).first,
          );
          await tester.pumpAndSettle();

          _assertAccentSoftStandard(
            tester,
            galFinder,
            brightness: entry.brightness,
            expectedLabel: 'Galeri',
            expectedIcon: Icons.photo_library_outlined,
          );
          _assertAccentSoftStandard(
            tester,
            camFinder,
            brightness: entry.brightness,
            expectedLabel: 'Kamera',
            expectedIcon: Icons.camera_alt_outlined,
          );
          expect(tester.takeException(), isNull);

          // Tap Galeri -> preview & delete button appear
          await tester.tap(galFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.gallery]));
          expect(find.byTooltip('Hapus Bukti'), findsOneWidget);

          // Remove proof -> buttons return -> tap Kamera
          await tester.ensureVisible(find.byTooltip('Hapus Bukti'));
          await tester.tap(find.byTooltip('Hapus Bukti'));
          await tester.pumpAndSettle();

          await tester.ensureVisible(camFinder);
          await tester.tap(camFinder);
          await tester.pumpAndSettle();
          expect(fakePicker.pickedSources, equals([ImageSource.gallery, ImageSource.camera]));
          expect(tester.takeException(), isNull);
        }
      },
    );
  });
}
