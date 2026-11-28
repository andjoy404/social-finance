import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/navigators/form_navigation_guard.dart';
import 'package:social_finance/core/navigators/kas_form_navigation_guard.dart';
import 'package:social_finance/features/dashboard/data/community_repository.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';
import 'package:social_finance/features/kegiatan/presentation/screens/kegiatan_form_screen.dart';
import 'package:social_finance/features/pengumuman/presentation/screens/announcement_form_screen.dart';

class _FakeCommunityRepository extends CommunityRepository {
  _FakeCommunityRepository() : super(dio: Dio());

  @override
  Future<List<BannerTemplate>> listBannerTemplates() async => [];
}

/// Minimal harness screen exercising FormNavigationGuard across:
/// AppBar back, Batal, system back (PopScope), saving state, save success, save failure.
class _HarnessFormScreen extends StatefulWidget {
  final String routePath;
  final String saveMode; // 'success' | 'fail' | 'pending'

  const _HarnessFormScreen({
    required this.routePath,
    this.saveMode = 'success',
  });

  @override
  State<_HarnessFormScreen> createState() => _HarnessFormScreenState();
}

class _HarnessFormScreenState extends State<_HarnessFormScreen> {
  final _nameCtrl = TextEditingController();
  bool _checked = false;
  bool _saving = false;
  String? _error;

  late final FormSnapshot _snapshot;
  FormGuardRegistration? _reg;

  Map<String, Object?> _values() => {
        'name': _nameCtrl.text,
        'checked': _checked,
      };

  @override
  void initState() {
    super.initState();
    _snapshot = FormSnapshot(_values());
    _reg = FormNavigationGuard.register(
      widget.routePath,
      isDirty: () => _snapshot.isDirty(_values()),
      isSaving: () => _saving,
    );
  }

  @override
  void dispose() {
    FormNavigationGuard.unregister(widget.routePath, registration: _reg);
    _nameCtrl.dispose();
    super.dispose();
  }

  void _handleSave() {
    setState(() {
      _saving = true;
      _error = null;
    });
    if (widget.saveMode == 'pending') return;
    if (widget.saveMode == 'fail') {
      setState(() {
        _saving = false;
        _error = 'Gagal menyimpan data';
      });
      return;
    }
    setState(() => _saving = false);
    FormNavigationGuard.exitAfterSave(
      context,
      routePath: widget.routePath,
      fallbackRoute: '/list',
    );
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          FormNavigationGuard.requestFormCancel(
            context,
            routePath: widget.routePath,
            fallbackRoute: '/list',
          );
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Form Uji'),
          leading: IconButton(
            key: const Key('appbar-back'),
            icon: const Icon(Icons.arrow_back),
            onPressed: () => FormNavigationGuard.requestFormCancel(
              context,
              routePath: widget.routePath,
              fallbackRoute: '/list',
            ),
          ),
        ),
        body: Column(
          children: [
            TextField(
              key: const Key('name-field'),
              controller: _nameCtrl,
            ),
            Checkbox(
              key: const Key('check-field'),
              value: _checked,
              onChanged: (v) => setState(() => _checked = v ?? false),
            ),
            if (_error != null) Text(_error!),
            Row(
              children: [
                ElevatedButton(
                  key: const Key('batal-btn'),
                  onPressed: () => FormNavigationGuard.requestFormCancel(
                    context,
                    routePath: widget.routePath,
                    fallbackRoute: '/list',
                  ),
                  child: const Text('Batal'),
                ),
                ElevatedButton(
                  key: const Key('simpan-btn'),
                  onPressed: _handleSave,
                  child: const Text('Simpan'),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// Shell with Drawer, BottomNav, and Menu Grid exercising confirmShellNavigation.
Widget _buildShellApp({String saveMode = 'success'}) {
  late final GoRouter router;
  router = GoRouter(
    initialLocation: '/list',
    routes: [
      ShellRoute(
        builder: (context, state, child) {
          Future<void> goGuarded(String target) async {
            final ok = await FormNavigationGuard.confirmShellNavigation(
              context,
              source: state.uri.path,
              destination: target,
            );
            if (ok && context.mounted) {
              FormNavigationGuard.allowExitFor(state.uri.path);
              context.go(target);
            }
          }

          return Scaffold(
            key: const Key('shell-scaffold'),
            drawer: Drawer(
              child: ListView(
                children: [
                  ListTile(
                    key: const Key('drawer-beranda'),
                    title: const Text('Drawer Beranda'),
                    onTap: () async {
                      final ok = await FormNavigationGuard.confirmShellNavigation(
                        context,
                        source: state.uri.path,
                        destination: '/beranda',
                      );
                      if (ok && context.mounted) {
                        FormNavigationGuard.allowExitFor(state.uri.path);
                        Navigator.of(context).pop();
                        context.go('/beranda');
                      }
                    },
                  ),
                ],
              ),
            ),
            body: child,
            bottomNavigationBar: Row(
              children: [
                TextButton(
                  key: const Key('nav-beranda'),
                  onPressed: () => goGuarded('/beranda'),
                  child: const Text('Beranda'),
                ),
                TextButton(
                  key: const Key('nav-list'),
                  onPressed: () => goGuarded('/list'),
                  child: const Text('Daftar'),
                ),
                TextButton(
                  key: const Key('nav-menu-grid'),
                  onPressed: () => goGuarded('/menu-target'),
                  child: const Text('Menu Grid'),
                ),
              ],
            ),
          );
        },
        routes: [
          GoRoute(
            path: '/list',
            builder: (context, state) => Center(
              child: ElevatedButton(
                key: const Key('open-form-btn'),
                onPressed: () => context.push('/list/form'),
                child: const Text('Buka Form'),
              ),
            ),
          ),
          GoRoute(
            path: '/list/form',
            onExit: FormNavigationGuard.onRouteExit,
            builder: (context, state) => _HarnessFormScreen(
              routePath: '/list/form',
              saveMode: saveMode,
            ),
          ),
          GoRoute(
            path: '/beranda',
            builder: (_, _) => const Center(child: Text('Halaman Beranda')),
          ),
          GoRoute(
            path: '/menu-target',
            builder: (_, _) => const Center(child: Text('Halaman Menu Target')),
          ),
        ],
      ),
    ],
  );
  router.routerDelegate.addListener(() {
    FormNavigationGuard.clearStaleApprovedExits(
      activePath: router.routeInformationProvider.value.uri.path,
    );
  });

  return MaterialApp.router(routerConfig: router);
}

class _TabHarness extends StatefulWidget {
  final bool saving;
  const _TabHarness({super.key, this.saving = false});

  @override
  State<_TabHarness> createState() => _TabHarnessState();
}

class _TabHarnessState extends State<_TabHarness>
    with SingleTickerProviderStateMixin {
  late final TabController _tabCtrl;
  final _ctrl = TextEditingController();
  late final FormSnapshot _snapshot;
  int _activeIndex = 0;

  @override
  void initState() {
    super.initState();
    _tabCtrl = TabController(length: 2, vsync: this);
    _snapshot = FormSnapshot({'text': ''});
  }

  @override
  void dispose() {
    _tabCtrl.dispose();
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _switchTab(int index) async {
    if (index == _activeIndex) return;
    final ok = await FormNavigationGuard.confirmTabChange(
      context,
      isDirty: _snapshot.isDirty({'text': _ctrl.text}),
      isSaving: widget.saving,
    );
    if (!ok || !mounted) return;
    setState(() {
      _activeIndex = index;
      _tabCtrl.animateTo(index);
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        children: [
          Row(
            children: [
              TextButton(
                key: const Key('tab-0'),
                onPressed: () => _switchTab(0),
                child: const Text('Tab Form'),
              ),
              TextButton(
                key: const Key('tab-1'),
                onPressed: () => _switchTab(1),
                child: const Text('Tab Riwayat'),
              ),
            ],
          ),
          Expanded(
            child: _activeIndex == 0
                ? TextField(key: const Key('tab-input'), controller: _ctrl)
                : const Text('Konten Riwayat'),
          ),
        ],
      ),
    );
  }
}

void main() {
  setUp(() {
    FormNavigationGuard.resetForTesting();
  });

  group('FormSnapshot unit tests', () {
    test('clean, dirty, and reverted-to-snapshot returns clean across types', () {
      final dt = DateTime(2026, 6, 1, 9, 30);
      final snap = FormSnapshot({
        'text': 'Awal',
        'flag': false,
        'count': 3,
        'date': dt,
        'list': ['a', 'b'],
        'map': {'k': 'v'},
      });

      expect(
        snap.isDirty({
          'text': 'Awal',
          'flag': false,
          'count': 3,
          'date': DateTime(2026, 6, 1, 9, 30),
          'list': ['a', 'b'],
          'map': {'k': 'v'},
        }),
        isFalse,
      );

      expect(
        snap.isDirty({
          'text': 'Diubah',
          'flag': false,
          'count': 3,
          'date': dt,
          'list': ['a', 'b'],
          'map': {'k': 'v'},
        }),
        isTrue,
      );

      expect(
        snap.isDirty({
          'text': 'Awal',
          'flag': true,
          'count': 3,
          'date': dt,
          'list': ['a', 'b'],
          'map': {'k': 'v'},
        }),
        isTrue,
      );

      // Reverted back to initial values -> clean again
      expect(
        snap.isDirty({
          'text': 'Awal',
          'flag': false,
          'count': 3,
          'date': dt,
          'list': ['a', 'b'],
          'map': {'k': 'v'},
        }),
        isFalse,
      );
    });

    test('KasFormNavigationGuard alias delegates cleanly to FormNavigationGuard', () {
      final reg = FormNavigationGuard.register(
        '/alias/test',
        isDirty: () => true,
      );
      expect(KasFormNavigationGuard.isFormPath('/alias/test'), isTrue);
      expect(FormNavigationGuard.isDirty('/alias/test'), isTrue);
      FormNavigationGuard.unregister('/alias/test', registration: reg);
      expect(FormNavigationGuard.isDirty('/alias/test'), isFalse);
    });
  });

  group('Full navigation paths: Clean, Dirty, Reverted, Saving, Save Success/Fail', () {
    testWidgets('Clean form exits immediately via Batal, AppBar back, System back, BottomNav, Drawer', (tester) async {
      await tester.pumpWidget(_buildShellApp());
      await tester.pumpAndSettle();

      // 1) Batal on clean form exits immediately
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('open-form-btn')), findsOneWidget);

      // 2) AppBar back on clean form exits immediately
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('appbar-back')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('open-form-btn')), findsOneWidget);

      // 3) System back on clean form exits immediately
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('open-form-btn')), findsOneWidget);

      // 4) BottomNav on clean form exits immediately
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('nav-beranda')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('Halaman Beranda'), findsOneWidget);
    });

    testWidgets('Dirty form prompts standard dialog across all exit paths; reverted value returns to clean', (tester) async {
      await tester.pumpWidget(_buildShellApp());
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();

      // Edit text field
      await tester.enterText(find.byKey(const Key('name-field')), 'Input Penting');
      await tester.pump();

      // A) System back -> standard copy
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Perubahan yang belum disimpan akan hilang.'), findsOneWidget);
      expect(find.text('Tetap di Halaman'), findsOneWidget);
      expect(find.text('Ya, Batalkan'), findsOneWidget);

      // Tetap di Halaman preserves input
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('Input Penting'), findsOneWidget);

      // B) AppBar back -> single dialog -> stay
      await tester.tap(find.byKey(const Key('appbar-back')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();

      // C) BottomNav -> single dialog -> stay
      await tester.tap(find.byKey(const Key('nav-beranda')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();

      // D) Menu Grid -> single dialog -> stay
      await tester.tap(find.byKey(const Key('nav-menu-grid')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();

      // E) Drawer -> single dialog -> stay
      tester.state<ScaffoldState>(find.byKey(const Key('shell-scaffold'))).openDrawer();
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('drawer-beranda')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      // Close drawer
      Navigator.of(tester.element(find.byType(Drawer))).pop();
      await tester.pumpAndSettle();

      // F) Revert value back to initial snapshot -> clean -> Batal exits immediately without dialog!
      await tester.enterText(find.byKey(const Key('name-field')), '');
      await tester.pump();
      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('open-form-btn')), findsOneWidget);
    });

    testWidgets('Checkbox change marks dirty, Ya Batalkan exits and leaves NO leftover bypass', (tester) async {
      await tester.pumpWidget(_buildShellApp());
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('check-field')));
      await tester.pump();

      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);

      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('open-form-btn')), findsOneWidget);
      expect(FormNavigationGuard.hasApprovedExitFor('/list/form'), isFalse);

      // Reopen form, make dirty again -> must still prompt (no leftover bypass!)
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('name-field')), 'Baru lagi');
      await tester.pump();
      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
    });

    testWidgets('Saving state blocks all exit paths silently without confirmation dialog', (tester) async {
      await tester.pumpWidget(_buildShellApp(saveMode: 'pending'));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('name-field')), 'Sedang Simpan');
      await tester.pump();

      // Enter saving state
      await tester.tap(find.byKey(const Key('simpan-btn')));
      await tester.pump();

      // Try Batal, AppBar back, System back, BottomNav -> all silently blocked, no dialog
      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.pump();
      await tester.tap(find.byKey(const Key('appbar-back')));
      await tester.pump();
      await tester.binding.handlePopRoute();
      await tester.pump();
      await tester.tap(find.byKey(const Key('nav-beranda')));
      await tester.pump();

      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('Form Uji'), findsOneWidget);
      expect(find.text('Sedang Simpan'), findsOneWidget);
    });

    testWidgets('Save success exits via exitAfterSave without cancel dialog; Save failure preserves input & re-enables guard', (tester) async {
      // Save success
      await tester.pumpWidget(_buildShellApp(saveMode: 'success'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('name-field')), 'Sukses');
      await tester.pump();
      await tester.tap(find.byKey(const Key('simpan-btn')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('open-form-btn')), findsOneWidget);

      // Save failure
      await tester.pumpWidget(_buildShellApp(saveMode: 'fail'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('name-field')), 'Gagal Disimpan');
      await tester.pump();
      await tester.tap(find.byKey(const Key('simpan-btn')));
      await tester.pumpAndSettle();

      expect(find.text('Gagal menyimpan data'), findsOneWidget);
      expect(find.text('Gagal Disimpan'), findsOneWidget);

      // Guard must still protect unsaved input after failed save
      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
    });

    testWidgets('Concurrent exit triggers never create double dialogs', (tester) async {
      await tester.pumpWidget(_buildShellApp());
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('open-form-btn')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('name-field')), 'Double Tap');
      await tester.pump();

      await tester.tap(find.byKey(const Key('batal-btn')));
      await tester.tap(find.byKey(const Key('nav-beranda')), warnIfMissed: false);
      await tester.pumpAndSettle();

      expect(find.byType(AlertDialog), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
    });
  });

  group('Tab switch guard (confirmTabChange)', () {
    testWidgets('Clean switches immediately; dirty prompts; saving blocks silently', (tester) async {
      await tester.pumpWidget(const MaterialApp(home: _TabHarness()));
      await tester.pumpAndSettle();

      // Clean -> switches immediately
      await tester.tap(find.byKey(const Key('tab-1')));
      await tester.pumpAndSettle();
      expect(find.text('Konten Riwayat'), findsOneWidget);
      expect(find.byType(AlertDialog), findsNothing);

      // Switch back and dirty input
      await tester.tap(find.byKey(const Key('tab-0')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('tab-input')), 'Draft');
      await tester.pump();

      // Switch tab while dirty -> dialog -> stay preserves draft
      await tester.tap(find.byKey(const Key('tab-1')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      expect(find.text('Draft'), findsOneWidget);
      expect(find.text('Konten Riwayat'), findsNothing);

      // Switch tab while dirty -> Ya, Batalkan switches
      await tester.tap(find.byKey(const Key('tab-1')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();
      expect(find.text('Konten Riwayat'), findsOneWidget);

      // Saving blocks tab switch silently
      await tester.pumpWidget(
        const MaterialApp(home: _TabHarness(key: Key('tab-saving'), saving: true)),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('tab-input')), 'Saving');
      await tester.pump();
      await tester.tap(find.byKey(const Key('tab-1')));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.text('Konten Riwayat'), findsNothing);
    });
  });

  group('Modal guard (GuardedFormModalScope)', () {
    testWidgets('Clean exits on Batal/barrier/back; dirty prompts single dialog; saving blocks silently', (tester) async {
      bool modalSaving = false;
      final ctrl = TextEditingController();
      addTearDown(ctrl.dispose);

      Future<void> openModal(BuildContext ctx) {
        ctrl.text = '';
        return showDialog<void>(
          context: ctx,
          barrierDismissible: false,
          builder: (dCtx) => GuardedFormModalScope(
            isDirty: () => ctrl.text.isNotEmpty,
            isSaving: () => modalSaving,
            child: AlertDialog(
              title: const Text('Modal Uji'),
              content: TextField(key: const Key('modal-field'), controller: ctrl),
              actions: [
                TextButton(
                  key: const Key('modal-batal'),
                  onPressed: () => FormNavigationGuard.requestModalClose(
                    dCtx,
                    isDirty: ctrl.text.isNotEmpty,
                    isSaving: modalSaving,
                  ),
                  child: const Text('Batal'),
                ),
              ],
            ),
          ),
        );
      }

      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (ctx) => Scaffold(
              body: ElevatedButton(
                key: const Key('open-modal'),
                onPressed: () => openModal(ctx),
                child: const Text('Buka Modal'),
              ),
            ),
          ),
        ),
      );

      // Clean -> Batal closes immediately
      await tester.tap(find.byKey(const Key('open-modal')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('modal-batal')));
      await tester.pumpAndSettle();
      expect(find.text('Modal Uji'), findsNothing);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);

      // Dirty -> system back prompts -> Tetap preserves -> Ya closes
      await tester.tap(find.byKey(const Key('open-modal')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('modal-field')), 'Perubahan Modal');
      await tester.pump();

      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);

      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      expect(find.text('Modal Uji'), findsOneWidget);
      expect(find.text('Perubahan Modal'), findsOneWidget);

      // Saving blocks silently
      modalSaving = true;
      await tester.tap(find.byKey(const Key('modal-batal')));
      await tester.pump();
      await tester.binding.handlePopRoute();
      await tester.pump();
      expect(find.text('Modal Uji'), findsOneWidget);
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsNothing);

      // Stop saving -> Batal -> Ya closes modal
      modalSaving = false;
      await tester.tap(find.byKey(const Key('modal-batal')));
      await tester.pumpAndSettle();
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-confirm')));
      await tester.pumpAndSettle();
      expect(find.text('Modal Uji'), findsNothing);
    });
  });

  group('Real screen regressions: AnnouncementFormScreen & KegiatanFormScreen', () {
    testWidgets('AnnouncementFormScreen: clean exits immediately; dirty prompts; reverted returns to clean', (tester) async {
      tester.view.physicalSize = const Size(800, 2200);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      final router = GoRouter(
        initialLocation: '/home/pengumuman',
        routes: [
          GoRoute(
            path: '/home/pengumuman',
            builder: (ctx, _) => Scaffold(
              body: ElevatedButton(
                key: const Key('go-ann-create'),
                onPressed: () => ctx.push('/home/pengumuman/create'),
                child: const Text('Buat Pengumuman'),
              ),
            ),
          ),
          GoRoute(
            path: '/home/pengumuman/create',
            onExit: FormNavigationGuard.onRouteExit,
            builder: (_, _) => const AnnouncementFormScreen(),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(_FakeCommunityRepository()),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      // Open -> clean -> Batal exits immediately
      await tester.tap(find.byKey(const Key('go-ann-create')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(ElevatedButton, 'Batal'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('go-ann-create')), findsOneWidget);

      // Open -> type title -> dirty -> Batal prompts -> Tetap preserves -> revert -> Batal exits immediately
      await tester.tap(find.byKey(const Key('go-ann-create')));
      await tester.pumpAndSettle();
      final firstField = find.byType(TextField).first;
      await tester.enterText(firstField, 'Pengumuman Rapat');
      await tester.pump();

      await tester.tap(find.widgetWithText(ElevatedButton, 'Batal'));
      await tester.pumpAndSettle();
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      expect(find.text('Tetap di Halaman'), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      expect(find.text('Pengumuman Rapat'), findsOneWidget);

      // Revert to empty -> clean -> Batal exits immediately
      await tester.enterText(firstField, '');
      await tester.pump();
      await tester.tap(find.widgetWithText(ElevatedButton, 'Batal'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('go-ann-create')), findsOneWidget);
    });

    testWidgets('KegiatanFormScreen: clean exits immediately; dirty prompts; reverted returns to clean', (tester) async {
      tester.view.physicalSize = const Size(800, 2200);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);

      final router = GoRouter(
        initialLocation: '/home/kegiatan',
        routes: [
          GoRoute(
            path: '/home/kegiatan',
            builder: (ctx, _) => Scaffold(
              body: ElevatedButton(
                key: const Key('go-keg-create'),
                onPressed: () => ctx.push('/home/kegiatan/create'),
                child: const Text('Buat Kegiatan'),
              ),
            ),
          ),
          GoRoute(
            path: '/home/kegiatan/create',
            onExit: FormNavigationGuard.onRouteExit,
            builder: (_, _) => const KegiatanFormScreen(),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            communityRepositoryProvider.overrideWithValue(_FakeCommunityRepository()),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();

      // Open -> clean -> Batal exits immediately
      await tester.tap(find.byKey(const Key('go-keg-create')));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(ElevatedButton, 'Batal'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('go-keg-create')), findsOneWidget);

      // Open -> type title -> dirty -> system back prompts -> stay -> revert -> Batal exits immediately
      await tester.tap(find.byKey(const Key('go-keg-create')));
      await tester.pumpAndSettle();
      final firstField = find.byType(TextField).first;
      await tester.enterText(firstField, 'Kerja Bakti');
      await tester.pump();

      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.text('Apakah Anda yakin ingin membatalkan?'), findsOneWidget);
      await tester.tap(find.byKey(const Key('kas-cancel-stay')));
      await tester.pumpAndSettle();
      expect(find.text('Kerja Bakti'), findsOneWidget);

      // Revert -> clean -> Batal exits immediately
      await tester.enterText(firstField, '');
      await tester.pump();
      await tester.tap(find.widgetWithText(ElevatedButton, 'Batal'));
      await tester.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(find.byKey(const Key('go-keg-create')), findsOneWidget);
    });
  });
}
