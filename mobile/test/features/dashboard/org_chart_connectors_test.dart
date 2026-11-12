import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
    : super(
        authService: AuthService(
          ApiClient(baseUrl: 'http://test.local'),
          DeviceIdService(),
        ),
        apiClient: ApiClient(baseUrl: 'http://test.local'),
      ) {
    state = AsyncData(user);
  }
}

class _StubWargaListNotifier extends WargaListNotifier {
  final List<MappedResident> initialResidents;
  _StubWargaListNotifier([this.initialResidents = const <MappedResident>[]]);

  @override
  Future<List<MappedResident>> build() async => initialResidents;
}

Future<void> _loadMaterialFontsIfAvailable() async {
  const fontDir =
      '/Users/Andriyan/Documents/data/homework/flutter/perpus-digital/flutter/bin/cache/artifacts/material_fonts';
  final regularFile = File('$fontDir/Roboto-Regular.ttf');
  if (!regularFile.existsSync()) return;

  final robotoLoader = FontLoader('Roboto');
  for (final name in [
    'Roboto-Regular.ttf',
    'Roboto-Medium.ttf',
    'Roboto-Bold.ttf',
  ]) {
    final f = File('$fontDir/$name');
    if (f.existsSync()) {
      final bytes = f.readAsBytesSync();
      robotoLoader.addFont(
        Future.value(ByteData.view(Uint8List.fromList(bytes).buffer)),
      );
    }
  }
  await robotoLoader.load();

  final iconsFile = File('$fontDir/MaterialIcons-Regular.otf');
  if (iconsFile.existsSync()) {
    final iconsLoader = FontLoader('MaterialIcons');
    final bytes = iconsFile.readAsBytesSync();
    iconsLoader.addFont(
      Future.value(ByteData.view(Uint8List.fromList(bytes).buffer)),
    );
    await iconsLoader.load();
  }
}

Future<void> _maybeCaptureScreenshot(
  WidgetTester tester,
  GlobalKey boundaryKey,
  String filename,
) async {
  final outDir = Platform.environment['ORG_CHART_SCREENSHOT_DIR'];
  if (outDir == null || outDir.isEmpty) return;

  final boundary =
      boundaryKey.currentContext?.findRenderObject() as RenderRepaintBoundary?;
  if (boundary == null) return;

  await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 2.0);
    final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
    if (byteData == null) return;

    final file = File('$outDir/$filename');
    await file.parent.create(recursive: true);
    await file.writeAsBytes(byteData.buffer.asUint8List());
  });
}

OrgNode _makeNode({
  required String position,
  required String label,
  required int backendLevel,
  required String parentPosition,
  required List<OrgMember> members,
}) {
  return OrgNode(
    position: position,
    positionLabel: label,
    level: backendLevel,
    parentPosition: parentPosition,
    members: members,
  );
}

List<OrgNode> _buildFullNodes({
  List<OrgMember>? ketuaMembers,
  List<OrgMember>? wakilMembers,
  List<OrgMember>? bendaharaMembers,
  List<OrgMember>? sekretarisMembers,
  List<OrgMember>? keamananMembers,
  List<OrgMember>? kebersihanMembers,
  List<OrgMember>? sosialMembers,
  List<OrgMember>? operasionalMembers,
}) {
  // Note: backend levels intentionally mimic backend repository.go
  // (sekretaris at level 2, bendahara at level 3 with parent sekretaris)
  // to prove Android diagram uses canonical tier ordering instead of backend level.
  return [
    _makeNode(
      position: 'ketua',
      label: 'Ketua RT',
      backendLevel: 1,
      parentPosition: '',
      members:
          ketuaMembers ??
          const [
            OrgMember(
              userId: 'u1',
              membershipId: 'm1',
              name: 'H. Ahmad Subarjo',
              jabatan: 'ketua',
              phone: '081234567890',
            ),
          ],
    ),
    _makeNode(
      position: 'wakil_ketua',
      label: 'Wakil Ketua',
      backendLevel: 2,
      parentPosition: 'ketua',
      members:
          wakilMembers ??
          const [
            OrgMember(
              userId: 'u2',
              membershipId: 'm2',
              name: 'Bambang Wijaya',
              jabatan: 'wakil_ketua',
            ),
          ],
    ),
    _makeNode(
      position: 'sekretaris',
      label: 'Sekretaris',
      backendLevel: 2,
      parentPosition: 'ketua',
      members:
          sekretarisMembers ??
          const [
            OrgMember(
              userId: 'u3',
              membershipId: 'm3',
              name: 'Siti Rahmawati',
              jabatan: 'sekretaris',
            ),
          ],
    ),
    _makeNode(
      position: 'bendahara',
      label: 'Bendahara',
      backendLevel: 3,
      parentPosition: 'sekretaris',
      members:
          bendaharaMembers ??
          const [
            OrgMember(
              userId: 'u4',
              membershipId: 'm4',
              name: 'Dewi Sartika',
              jabatan: 'bendahara',
            ),
          ],
    ),
    _makeNode(
      position: 'keamanan',
      label: 'Keamanan',
      backendLevel: 4,
      parentPosition: 'ketua',
      members:
          keamananMembers ??
          const [
            OrgMember(
              userId: 'u5',
              membershipId: 'm5',
              name: 'Agus Pratama',
              jabatan: 'keamanan',
            ),
          ],
    ),
    _makeNode(
      position: 'kebersihan',
      label: 'Kebersihan',
      backendLevel: 4,
      parentPosition: 'ketua',
      members:
          kebersihanMembers ??
          const [
            OrgMember(
              userId: 'u6',
              membershipId: 'm6',
              name: 'Eko Saputra',
              jabatan: 'kebersihan',
            ),
          ],
    ),
    _makeNode(
      position: 'sosial',
      label: 'Seksi Sosial',
      backendLevel: 4,
      parentPosition: 'ketua',
      members:
          sosialMembers ??
          const [
            OrgMember(
              userId: 'u7',
              membershipId: 'm7',
              name: 'Rina Marlina',
              jabatan: 'sosial',
            ),
          ],
    ),
    _makeNode(
      position: 'operasional',
      label: 'Operasional',
      backendLevel: 4,
      parentPosition: 'ketua',
      members:
          operasionalMembers ??
          const [
            OrgMember(
              userId: 'u8',
              membershipId: 'm8',
              name: 'Hendro Kusumo',
              jabatan: 'operasional',
            ),
          ],
    ),
  ];
}

Future<GlobalKey> _pumpDashboardWithOrg(
  WidgetTester tester, {
  required List<OrgNode> nodes,
  List<MappedResident> wargaResidents = const <MappedResident>[],
  Size surfaceSize = const Size(640, 1100),
}) async {
  tester.view.physicalSize = surfaceSize;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(() {
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });

  final org = OrganizationStructure(
    rtId: 'rt-1',
    rtName: 'RT 002',
    rtNumber: '002',
    rwNumber: '016',
    nodes: nodes,
  );

  final testData = HomeDashboardData(
    greeting: 'Selamat datang,',
    userName: 'Warga Test',
    userRole: 'warga',
    userJabatan: '',
    rt: '002',
    rw: '016',
    banners: const [],
    security: const SecurityOnDutyInfo(
      isAvailable: false,
      officers: [],
      message: 'Tidak ada petugas keamanan yang sedang bertugas.',
      subtext: '',
    ),
    organization: org,
    upcomingActivities: const [],
  );

  final boundaryKey = GlobalKey();

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWith(
          (ref) => _MockAuthRepo({
            'id': 'user-1',
            'email': 'warga@example.com',
            'name': 'Warga Test',
            'role': AppRole.warga,
            'system_role': null,
            'jabatan': null,
            'rt': '002',
            'rw': '016',
            'rt_id': 'rt-1',
          }),
        ),
        homeDashboardFutureProvider.overrideWith(
          (ref) => Future<HomeDashboardData>.value(testData),
        ),
        wargaListProvider.overrideWith(
          () => _StubWargaListNotifier(wargaResidents),
        ),
      ],
      child: RepaintBoundary(
        key: boundaryKey,
        child: MaterialApp(
          debugShowCheckedModeBanner: false,
          theme: ThemeData(fontFamily: 'Roboto', useMaterial3: true),
          home: const DashboardScreen(),
        ),
      ),
    ),
  );

  await tester.pumpAndSettle();
  return boundaryKey;
}

OrgConnectorsPainter _getConnectorsPainter(WidgetTester tester) {
  final customPaint = tester.widget<CustomPaint>(
    find.byKey(const ValueKey('org-connectors-custom-paint')),
  );
  return customPaint.painter! as OrgConnectorsPainter;
}

void _assertStrictlyOrthogonalAndClean(
  List<OrgConnectorLine> lines,
  Map<String, Rect> rects,
) {
  expect(lines, isNotEmpty);
  for (final line in lines) {
    expect(line.points.length, greaterThanOrEqualTo(2));
    for (var i = 0; i < line.points.length - 1; i++) {
      final p0 = line.points[i];
      final p1 = line.points[i + 1];
      final isVertical = (p0.dx - p1.dx).abs() < 0.05;
      final isHorizontal = (p0.dy - p1.dy).abs() < 0.05;
      // Every segment must be strictly horizontal or strictly vertical (90° right angles)
      expect(
        isVertical || isHorizontal,
        isTrue,
        reason: 'Segment $p0 -> $p1 is not orthogonal',
      );

      // Verify segment never pierces the interior of any badge rect
      final segMinX = p0.dx < p1.dx ? p0.dx : p1.dx;
      final segMaxX = p0.dx > p1.dx ? p0.dx : p1.dx;
      final segMinY = p0.dy < p1.dy ? p0.dy : p1.dy;
      final segMaxY = p0.dy > p1.dy ? p0.dy : p1.dy;

      for (final entry in rects.entries) {
        final r = entry.value.deflate(0.5);
        final overlapsX = segMaxX > r.left && segMinX < r.right;
        final overlapsY = segMaxY > r.top && segMinY < r.bottom;
        expect(
          overlapsX && overlapsY,
          isFalse,
          reason:
              'Connector segment $p0 -> $p1 pierces badge ${entry.key} ($r)',
        );
      }
    }
  }
}

void main() {
  setUpAll(() async {
    await _loadMaterialFontsIfAvailable();
  });

  testWidgets(
    'Skenario 1: Semua jabatan terisi warga aktif — urutan vertikal 4 level, simetris, dan orthogonal connector',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(),
      );

      expect(find.text('Struktur Organisasi RT'), findsOneWidget);
      expect(find.text('KETUA RT'), findsOneWidget);
      expect(find.text('WAKIL KETUA'), findsOneWidget);
      expect(find.text('BENDAHARA'), findsOneWidget);
      expect(find.text('SEKRETARIS'), findsOneWidget);
      expect(find.text('KEAMANAN'), findsOneWidget);
      expect(find.text('KEBERSIHAN'), findsOneWidget);
      expect(find.text('SEKSI SOSIAL'), findsOneWidget);
      expect(find.text('OPERASIONAL'), findsOneWidget);

      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();

      expect(rects.keys.length, 8);
      final rKetua = rects['ketua']!;
      final rWakil = rects['wakil_ketua']!;
      final rBendahara = rects['bendahara']!;
      final rSekretaris = rects['sekretaris']!;
      final rKeamanan = rects['keamanan']!;
      final rKebersihan = rects['kebersihan']!;
      final rSosial = rects['sosial']!;
      final rOperasional = rects['operasional']!;

      // 1. Vertical top-to-bottom tier ordering:
      // Level 1 (Ketua) < Level 2 (Wakil Ketua) < Level 3 (Bendahara = Sekretaris) < Level 4 (Keamanan = Kebersihan = Sosial = Operasional)
      expect(rKetua.bottom, lessThan(rWakil.top));
      expect(rWakil.bottom, lessThan(rBendahara.top));
      expect((rBendahara.top - rSekretaris.top).abs(), lessThan(0.5));
      expect(rBendahara.bottom, lessThan(rKeamanan.top));
      expect((rKeamanan.top - rKebersihan.top).abs(), lessThan(0.5));
      expect((rKebersihan.top - rSosial.top).abs(), lessThan(0.5));
      expect((rSosial.top - rOperasional.top).abs(), lessThan(0.5));

      // 2. Exact left-to-right ordering within Level 3 and Level 4:
      // Bendahara = Sekretaris
      expect(rBendahara.right, lessThan(rSekretaris.left));
      // Keamanan = Kebersihan = Sosial = Operasional
      expect(rKeamanan.right, lessThan(rKebersihan.left));
      expect(rKebersihan.right, lessThan(rSosial.left));
      expect(rSosial.right, lessThan(rOperasional.left));

      // 3. Symmetry around vertical center axis
      expect((rKetua.center.dx - rWakil.center.dx).abs(), lessThan(0.5));
      final tier3MidX = (rBendahara.left + rSekretaris.right) / 2.0;
      final tier4MidX = (rKeamanan.left + rOperasional.right) / 2.0;
      expect((rKetua.center.dx - tier3MidX).abs(), lessThan(1.0));
      expect((rKetua.center.dx - tier4MidX).abs(), lessThan(1.0));

      // 4. Orthogonal connectors verification
      _assertStrictlyOrthogonalAndClean(lines, rects);

      // Direct vertical line Ketua -> Wakil Ketua stopping at exact badge edges
      expect(
        lines.any(
          (l) =>
              l.points.length == 2 &&
              (l.points.first.dx - rKetua.center.dx).abs() < 0.5 &&
              (l.points.first.dy - rKetua.bottom).abs() < 0.5 &&
              (l.points.last.dx - rWakil.center.dx).abs() < 0.5 &&
              (l.points.last.dy - rWakil.top).abs() < 0.5,
        ),
        isTrue,
      );

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_1_semua_terisi.png',
      );
    },
  );

  testWidgets(
    'Skenario 2: Bendahara kosong — badge Bendahara disembunyikan, Sekretaris di tengah dengan garis vertikal langsung dari Wakil Ketua',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(bendaharaMembers: const []),
      );

      expect(find.text('BENDAHARA'), findsNothing);
      expect(find.text('SEKRETARIS'), findsOneWidget);

      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();

      expect(rects.containsKey('bendahara'), isFalse);
      final rKetua = rects['ketua']!;
      final rWakil = rects['wakil_ketua']!;
      final rSekretaris = rects['sekretaris']!;

      // Sekretaris is centered directly below Wakil Ketua
      expect((rSekretaris.center.dx - rWakil.center.dx).abs(), lessThan(0.5));
      expect((rSekretaris.center.dx - rKetua.center.dx).abs(), lessThan(0.5));

      // Direct vertical connector Wakil Ketua -> Sekretaris
      expect(
        lines.any(
          (l) =>
              l.points.length == 2 &&
              (l.points.first.dx - rWakil.center.dx).abs() < 0.5 &&
              (l.points.first.dy - rWakil.bottom).abs() < 0.5 &&
              (l.points.last.dx - rSekretaris.center.dx).abs() < 0.5 &&
              (l.points.last.dy - rSekretaris.top).abs() < 0.5,
        ),
        isTrue,
      );

      _assertStrictlyOrthogonalAndClean(lines, rects);

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_2_bendahara_kosong.png',
      );
    },
  );

  testWidgets(
    'Skenario 3: Sekretaris kosong — badge Sekretaris disembunyikan, Bendahara di tengah dengan garis vertikal langsung dari Wakil Ketua',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(sekretarisMembers: const []),
      );

      expect(find.text('SEKRETARIS'), findsNothing);
      expect(find.text('BENDAHARA'), findsOneWidget);

      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();

      expect(rects.containsKey('sekretaris'), isFalse);
      final rWakil = rects['wakil_ketua']!;
      final rBendahara = rects['bendahara']!;

      // Bendahara is centered directly below Wakil Ketua
      expect((rBendahara.center.dx - rWakil.center.dx).abs(), lessThan(0.5));

      // Direct vertical connector Wakil Ketua -> Bendahara
      expect(
        lines.any(
          (l) =>
              l.points.length == 2 &&
              (l.points.first.dx - rWakil.center.dx).abs() < 0.5 &&
              (l.points.first.dy - rWakil.bottom).abs() < 0.5 &&
              (l.points.last.dx - rBendahara.center.dx).abs() < 0.5 &&
              (l.points.last.dy - rBendahara.top).abs() < 0.5,
        ),
        isTrue,
      );

      _assertStrictlyOrthogonalAndClean(lines, rects);

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_3_sekretaris_kosong.png',
      );
    },
  );

  testWidgets(
    'Skenario 4: Salah satu jabatan level keempat kosong (Kebersihan kosong) — level 4 tetap simetris tanpa garis ke jabatan kosong',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(kebersihanMembers: const []),
      );

      expect(find.text('KEBERSIHAN'), findsNothing);
      expect(find.text('KEAMANAN'), findsOneWidget);
      expect(find.text('SEKSI SOSIAL'), findsOneWidget);
      expect(find.text('OPERASIONAL'), findsOneWidget);

      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();

      expect(rects.containsKey('kebersihan'), isFalse);
      final rKetua = rects['ketua']!;
      final rKeamanan = rects['keamanan']!;
      final rSosial = rects['sosial']!;
      final rOperasional = rects['operasional']!;

      expect(rKeamanan.right, lessThan(rSosial.left));
      expect(rSosial.right, lessThan(rOperasional.left));
      final tier4MidX = (rKeamanan.left + rOperasional.right) / 2.0;
      expect((rKetua.center.dx - tier4MidX).abs(), lessThan(1.0));

      _assertStrictlyOrthogonalAndClean(lines, rects);

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_4_level4_satu_kosong.png',
      );
    },
  );

  testWidgets(
    'Skenario 5: Seluruh pemegang salah satu jabatan dinonaktifkan — jabatan tidak ditampilkan di diagram',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(
          bendaharaMembers: const [
            OrgMember(
              userId: 'u4',
              membershipId: 'm4',
              name: 'Dewi Nonaktif',
              jabatan: 'bendahara',
              isActive: false,
            ),
          ],
          operasionalMembers: const [
            OrgMember(
              userId: 'u8',
              membershipId: 'm8',
              name: 'Hendro Nonaktif',
              jabatan: 'operasional',
              isActive: true, // active in org payload, but deactivated in warga module
            ),
          ],
        ),
        wargaResidents: const [
          MappedResident(
            id: 'res-8',
            userId: 'u8',
            name: 'Hendro Nonaktif',
            houseNumber: '-',
            rtNumber: '002',
            jabatan: 'operasional',
            isActive: false,
          ),
        ],
      );

      // Both Bendahara (inactive in OrgMember) and Operasional (inactive in warga module) are hidden
      expect(find.text('BENDAHARA'), findsNothing);
      expect(find.text('OPERASIONAL'), findsNothing);
      expect(find.text('SEKRETARIS'), findsOneWidget);
      expect(find.text('KEAMANAN'), findsOneWidget);
      expect(find.text('KEBERSIHAN'), findsOneWidget);
      expect(find.text('SEKSI SOSIAL'), findsOneWidget);

      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();

      expect(rects.containsKey('bendahara'), isFalse);
      expect(rects.containsKey('operasional'), isFalse);
      _assertStrictlyOrthogonalAndClean(lines, rects);

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_5_pemegang_nonaktif.png',
      );
    },
  );

  testWidgets(
    'Skenario 6: Satu jabatan memiliki beberapa anggota (sebagian aktif, sebagian nonaktif) — hanya warga aktif yang ditampilkan',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(
          keamananMembers: const [
            OrgMember(
              userId: 'u5a',
              membershipId: 'm5a',
              name: 'Agus Aktif',
              jabatan: 'keamanan',
              isActive: true,
            ),
            OrgMember(
              userId: 'u5b',
              membershipId: 'm5b',
              name: 'Doni Nonaktif',
              jabatan: 'keamanan',
              isActive: false,
            ),
            OrgMember(
              userId: 'u5c',
              membershipId: 'm5c',
              name: 'Budi Aktif',
              jabatan: 'keamanan',
              isActive: true,
            ),
          ],
        ),
      );

      // Badge KEAMANAN appears once in Level 4
      expect(find.text('KEAMANAN'), findsOneWidget);

      // Tap KEAMANAN badge to verify detail sheet displays only active members and hides inactive member
      await tester.tap(find.byKey(const ValueKey('org-badge-keamanan')));
      await tester.pumpAndSettle();

      expect(find.text('Agus Aktif'), findsOneWidget);
      expect(find.text('Budi Aktif'), findsOneWidget);
      expect(find.text('Doni Nonaktif'), findsNothing);

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_6_multi_anggota_aktif_nonaktif.png',
      );

      await tester.tap(find.text('Tutup'));
      await tester.pumpAndSettle();
    },
  );

  testWidgets(
    'Skenario 7: Hanya satu jabatan aktif pada suatu level (Ketua -> Wakil Ketua -> Sekretaris)',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(
          bendaharaMembers: const [],
          keamananMembers: const [],
          kebersihanMembers: const [],
          sosialMembers: const [],
          operasionalMembers: const [],
        ),
      );

      expect(find.text('KETUA RT'), findsOneWidget);
      expect(find.text('WAKIL KETUA'), findsOneWidget);
      expect(find.text('SEKRETARIS'), findsOneWidget);
      expect(find.text('BENDAHARA'), findsNothing);
      expect(find.text('KEAMANAN'), findsNothing);
      expect(find.text('KEBERSIHAN'), findsNothing);
      expect(find.text('SEKSI SOSIAL'), findsNothing);
      expect(find.text('OPERASIONAL'), findsNothing);

      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();

      expect(rects.keys.toSet(), {'ketua', 'wakil_ketua', 'sekretaris'});
      final rKetua = rects['ketua']!;
      final rWakil = rects['wakil_ketua']!;
      final rSekretaris = rects['sekretaris']!;

      // All 3 single badges are vertically aligned on the center axis
      expect((rKetua.center.dx - rWakil.center.dx).abs(), lessThan(0.5));
      expect((rWakil.center.dx - rSekretaris.center.dx).abs(), lessThan(0.5));

      // Exactly 2 direct vertical lines: Ketua -> Wakil Ketua and Wakil Ketua -> Sekretaris
      expect(lines.length, 2);
      for (final l in lines) {
        expect(l.points.length, 2);
        expect((l.points.first.dx - rKetua.center.dx).abs(), lessThan(0.5));
        expect((l.points.last.dx - rKetua.center.dx).abs(), lessThan(0.5));
      }

      _assertStrictlyOrthogonalAndClean(lines, rects);

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_7_satu_jabatan_per_level.png',
      );
    },
  );

  testWidgets(
    'Skenario 8: Seluruh diagram di-zoom out dan di-pan — connector tetap menempel presisi pada badge',
    (WidgetTester tester) async {
      final boundaryKey = await _pumpDashboardWithOrg(
        tester,
        nodes: _buildFullNodes(),
      );

      final viewerFinder = find.byKey(
        const ValueKey('org-chart-interactive-viewer'),
      );
      expect(viewerFinder, findsOneWidget);

      final center = tester.getCenter(viewerFinder);

      // Perform two-finger pinch-to-zoom out
      final gesture1 = await tester.startGesture(center + const Offset(-80, 0));
      final gesture2 = await tester.startGesture(center + const Offset(80, 0));
      await tester.pump();
      await gesture1.moveTo(center + const Offset(-25, 0));
      await gesture2.moveTo(center + const Offset(25, 0));
      await tester.pump();
      await gesture1.up();
      await gesture2.up();
      await tester.pumpAndSettle();

      // Perform one-finger pan
      await tester.drag(viewerFinder, const Offset(35, 20));
      await tester.pumpAndSettle();

      final viewer = tester.widget<InteractiveViewer>(viewerFinder);
      final matrix = viewer.transformationController!.value;
      // Scale should be zoomed out (< 1.0)
      expect(matrix.getMaxScaleOnAxis(), lessThan(0.95));

      // Verify connectors in local diagram space remain 100% attached to badge edges
      final painter = _getConnectorsPainter(tester);
      final rects = painter.computeCurrentRects();
      final lines = painter.computeCurrentLines();
      _assertStrictlyOrthogonalAndClean(lines, rects);

      final rKetua = rects['ketua']!;
      final rWakil = rects['wakil_ketua']!;
      expect(
        lines.any(
          (l) =>
              (l.points.first.dx - rKetua.center.dx).abs() < 0.5 &&
              (l.points.first.dy - rKetua.bottom).abs() < 0.5 &&
              (l.points.last.dx - rWakil.center.dx).abs() < 0.5 &&
              (l.points.last.dy - rWakil.top).abs() < 0.5,
        ),
        isTrue,
      );

      await _maybeCaptureScreenshot(
        tester,
        boundaryKey,
        'skenario_8_zoom_out_dan_pan.png',
      );
    },
  );
}
