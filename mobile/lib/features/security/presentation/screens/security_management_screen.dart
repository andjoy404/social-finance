import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';

import '../utils/guest_visit_duration.dart' as guest_duration;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/authenticated_network_image.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/rw/data/rw_providers.dart';
import 'package:social_finance/core/widgets/accent_action_fab.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import '../../data/security_api_client.dart';
import '../../data/security_models.dart';
import '../providers/security_providers.dart';
import '../utils/attendance_order.dart';
import '../widgets/attendance_status_badge.dart';
import '../widgets/guest_visit_status_badge.dart';
import '../widgets/security_photo_viewer.dart';

enum SecurityManagementMode {
  full,
  officerPatrol,
  officerPaket,
}

class SecurityManagementScreen extends ConsumerStatefulWidget {
  final int? initialTopTabIndex;
  final SecurityManagementMode mode;
  final String? initialSubTab;
  final String? targetId;

  const SecurityManagementScreen({
    super.key,
    this.initialTopTabIndex,
    this.mode = SecurityManagementMode.full,
    this.initialSubTab,
    this.targetId,
  });

  @override
  ConsumerState<SecurityManagementScreen> createState() =>
      _SecurityManagementScreenState();
}

class _PaketItemDraft {
  final TextEditingController courierCtrl = TextEditingController();
  final TextEditingController resiCtrl = TextEditingController();
  final TextEditingController descCtrl = TextEditingController();
  XFile? photo;

  void dispose() {
    courierCtrl.dispose();
    resiCtrl.dispose();
    descCtrl.dispose();
  }
}

class _SecurityManagementScreenState
    extends ConsumerState<SecurityManagementScreen>
    with TickerProviderStateMixin, WidgetsBindingObserver {
  late final TabController _topTabs;
  late final TabController _kelolaTabs;
  late final TabController _absensiTabs;
  late final TabController _patroliTabs;
  late final TabController _paketTabs;
  bool _loading = true;
  String? _error;
  List<SecurityOfficer> _officers = [];
  List<ShiftConfig> _shifts = [];
  List<SecuritySchedule> _schedules = [];
  List<AttendanceRecord> _attendance = [];
  List<AttendanceCorrection> _corrections = [];
  List<SecurityDevice> _devices = [];
  List<GuestBookEntry> _guests = [];
  int _guestPage = 1;
  int _guestTotal = 0;
  Timer? _guestDebounce;
  Timer? _guestTicker;
  String _guestSearch = '';
  String _attendanceStatus = '';
  String? _officerFilter;
  DateTimeRange? _period;

  // Patrol state
  List<PatrolTemplate> _patrolTemplates = [];
  List<PatrolBundle> _patrolBundles = [];
  final Map<String, XFile> _patrolPhotos = {};
  final _patrolNotesCtrl = TextEditingController();
  bool _submittingPatrol = false;

  // Paket state
  List<PaketReport> _paketReports = [];
  List<MaskedWargaSearchResult> _wargaResults = [];
  MaskedWargaSearchResult? _selectedWarga;
  final _wargaSearchCtrl = TextEditingController();
  Timer? _wargaSearchDebounce;
  bool _searchingWarga = false;
  final List<_PaketItemDraft> _paketDraftItems = [_PaketItemDraft()];
  bool _submittingPaket = false;
  String _paketFilterStatus = '';

  Map<String, dynamic>? get _user =>
      ref.read(authRepositoryProvider)?.valueOrNull;
  bool get _canWrite => userHasPermission(_user, 'security.schedule.write');
  bool get _canManageDevices =>
      userHasPermission(_user, 'security.device.manage');
  bool get _canDecide => userHasPermission(_user, 'security.correction');
  bool get _canGuestRead => userHasPermission(_user, 'security.guest_book');
  bool get _canManageTemplates =>
      userHasPermission(_user, 'patrol.template.manage');
  bool get _canPatrolRead =>
      userHasPermission(_user, 'patrol.read') ||
      userHasPermission(_user, 'patrol.create');
  bool get _canPatrolWrite => userHasPermission(_user, 'patrol.create');
  bool get _canPaketRead =>
      userHasPermission(_user, 'paket.read') ||
      userHasPermission(_user, 'paket.create');
  bool get _canPaketWrite => userHasPermission(_user, 'paket.create');
  bool get _canPaketStatusChange =>
      userHasPermission(_user, 'paket.status_change');
  bool _refreshing = false;
  bool _targetDialogOpened = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    final initialIndex = widget.mode == SecurityManagementMode.officerPatrol
        ? 3
        : (widget.mode == SecurityManagementMode.officerPaket
            ? 4
            : (widget.initialTopTabIndex ?? 0));
    _topTabs = TabController(length: 5, vsync: this, initialIndex: initialIndex);
    _kelolaTabs = TabController(length: 3, vsync: this);
    _absensiTabs = TabController(length: 2, vsync: this);
    int patrolSubIndex = 0;
    if (widget.initialSubTab == 'history' || widget.initialSubTab == 'riwayat') {
      patrolSubIndex = 1;
    }
    _patroliTabs = TabController(length: 2, vsync: this, initialIndex: patrolSubIndex);
    int paketSubIndex = 0;
    if (widget.initialSubTab == 'list' || widget.initialSubTab == 'daftar') {
      paketSubIndex = _canPaketWrite ? 1 : 0;
    }
    _paketTabs = TabController(length: _canPaketWrite ? 2 : 1, vsync: this, initialIndex: paketSubIndex);
    for (final ctrl in [
      _topTabs,
      _kelolaTabs,
      _absensiTabs,
      _patroliTabs,
      _paketTabs,
    ]) {
      ctrl.addListener(() {
        if (mounted) setState(() {});
        if (!ctrl.indexIsChanging && mounted) {
          if (ctrl == _topTabs) {
            if (_topTabs.index == 3) {
              ref.read(securityApiClientProvider).markReadReceipt(domain: 'patrol');
              ref.invalidate(securityAlertsDataProvider);
              _reloadPatrolTemplates();
              _reloadPatrolBundles();
            } else if (_topTabs.index == 4) {
              ref.read(securityApiClientProvider).markReadReceipt(domain: 'paket');
              ref.invalidate(securityAlertsDataProvider);
              _reloadPaketReports();
            } else if (_topTabs.index == 1 && _absensiTabs.index == 1) {
              ref.read(securityApiClientProvider).markReadReceipt(domain: 'correction');
              ref.invalidate(securityAlertsDataProvider);
              _loadData();
            } else {
              _loadData();
            }
          } else {
            if (_topTabs.index == 1 && _absensiTabs.index == 1) {
              ref.read(securityApiClientProvider).markReadReceipt(domain: 'correction');
              ref.invalidate(securityAlertsDataProvider);
            }
            _loadData();
          }
        }
      });
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (initialIndex == 3) {
        ref.read(securityApiClientProvider).markReadReceipt(domain: 'patrol');
        ref.invalidate(securityAlertsDataProvider);
        _reloadPatrolTemplates();
        _reloadPatrolBundles();
      } else if (initialIndex == 4) {
        ref.read(securityApiClientProvider).markReadReceipt(domain: 'paket');
        ref.invalidate(securityAlertsDataProvider);
        _reloadPaketReports();
      }
      _loadData();
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _ensureGuestTicker();
    } else if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.inactive ||
        state == AppLifecycleState.detached ||
        state == AppLifecycleState.hidden) {
      _guestTicker?.cancel();
      _guestTicker = null;
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _topTabs.dispose();
    _kelolaTabs.dispose();
    _absensiTabs.dispose();
    _patroliTabs.dispose();
    _paketTabs.dispose();
    _patrolNotesCtrl.dispose();
    _wargaSearchCtrl.dispose();
    _wargaSearchDebounce?.cancel();
    for (final item in _paketDraftItems) {
      item.dispose();
    }
    _guestDebounce?.cancel();
    _guestTicker?.cancel();
    _guestTicker = null;
    super.dispose();
  }

  /// Menjalankan reload secara terbatas (mencegah refresh ganda).
  Future<void> _guardedRefresh(Future<void> Function() reload) async {
    if (_refreshing) return;
    _refreshing = true;
    try {
      await reload();
    } finally {
      _refreshing = false;
    }
  }

  void _showRefreshError(Object error) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Gagal memuat ulang: $error')),
    );
  }

  Future<void> _reloadAttendance() async {
    final api = ref.read(securityApiClientProvider);
    final canReadSchedules =
        userHasPermission(_user, 'security.schedule.read') || _canWrite;
    if (!canReadSchedules) {
      _showRefreshError('Anda tidak memiliki izin memuat data absensi.');
      return;
    }
    try {
      final period = _period;
      final attendance = await api.listManagerAttendance(
        dateFrom: period == null
            ? null
            : DateFormat('yyyy-MM-dd').format(period.start),
        dateTo: period == null
            ? null
            : DateFormat('yyyy-MM-dd').format(period.end),
        userId: _officerFilter,
        status: _attendanceStatus.isEmpty ? null : _attendanceStatus,
      );
      final sortedAttendance = [...attendance]
        ..sort(compareAttendanceRecency);
      final officers = await api.listManagerOfficers();
      if (!mounted) return;
      setState(() {
        _attendance = sortedAttendance;
        _officers = officers;
      });
    } catch (e) {
      if (!mounted) return;
      _showRefreshError(e);
    }
  }

  Future<void> _reloadCorrections() async {
    final api = ref.read(securityApiClientProvider);
    final canReadCorrections =
        _canDecide || userHasPermission(_user, 'security.attendance');
    if (!canReadCorrections) {
      _showRefreshError('Anda tidak memiliki izin memuat data koreksi.');
      return;
    }
    try {
      final corrections = await api.listManagerCorrections();
      if (!mounted) return;
      setState(() => _corrections = corrections);
    } catch (e) {
      if (!mounted) return;
      _showRefreshError(e);
    }
  }

  Future<void> _reloadDevices() async {
    if (!_canManageDevices) {
      _showRefreshError('Anda tidak memiliki izin memuat data perangkat.');
      return;
    }
    try {
      final devices =
          await ref.read(securityApiClientProvider).listManagerDevices();
      if (!mounted) return;
      setState(() => _devices = devices);
    } catch (e) {
      if (!mounted) return;
      _showRefreshError(e);
    }
  }

  Future<void> _reloadGuests({int? page}) async {
    if (!_canGuestRead) {
      _showRefreshError('Anda tidak memiliki izin membaca Buku Tamu.');
      return;
    }
    final targetPage = page ?? _guestPage;
    try {
      final result =
          await ref.read(securityApiClientProvider).listManagerGuestBooks(
            search: _guestSearch,
            page: targetPage,
          );
      if (!mounted) return;
      final sortedGuests = [...result.data]
        ..sort(compareGuestBookChronological);
      setState(() {
        _guests = sortedGuests;
        _guestTotal = result.total;
        _guestPage = targetPage;
      });
      _ensureGuestTicker();
    } catch (e) {
      if (!mounted) return;
      _showRefreshError(e);
    }
  }

  Future<void> _reloadPatrolTemplates() async {
    try {
      final list = await ref.read(securityApiClientProvider).listPatrolTemplates();
      if (!mounted) return;
      setState(() => _patrolTemplates = list);
    } catch (_) {}
  }

  Future<void> _reloadPatrolBundles() async {
    try {
      final list = await ref.read(securityApiClientProvider).listPatrolBundles();
      if (!mounted) return;
      setState(() => _patrolBundles = list);
      if (widget.targetId != null && widget.targetId!.isNotEmpty && !_targetDialogOpened) {
        final found = _patrolBundles.where((b) => b.id == widget.targetId).firstOrNull;
        if (found != null) {
          _targetDialogOpened = true;
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted) _viewPatrolPhotos(found);
          });
        }
      }
    } catch (_) {}
  }

  Future<void> _reloadPaketReports() async {
    try {
      final list = await ref.read(securityApiClientProvider).listPaketReports();
      if (!mounted) return;
      setState(() => _paketReports = list);
      if (widget.targetId != null && widget.targetId!.isNotEmpty && !_targetDialogOpened) {
        final found = _paketReports.where((r) => r.id == widget.targetId).firstOrNull;
        if (found != null && found.items.isNotEmpty) {
          _targetDialogOpened = true;
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted) _viewPaketPhotos(found.items.first);
          });
        }
      }
    } catch (_) {}
  }

  Future<ImageSource?> _selectImageSource() async {
    return showModalBottomSheet<ImageSource>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.camera_alt),
              title: const Text('Kamera'),
              onTap: () => Navigator.pop(ctx, ImageSource.camera),
            ),
            ListTile(
              leading: const Icon(Icons.photo_library),
              title: const Text('Galeri'),
              onTap: () => Navigator.pop(ctx, ImageSource.gallery),
            ),
          ],
        ),
      ),
    );
  }

  Future<XFile?> _pickImage(ImageSource source) async {
    try {
      final picker = ImagePicker();
      return await picker.pickImage(
        source: source,
        imageQuality: 75,
        maxWidth: 1200,
        maxHeight: 1200,
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal mengambil foto: $e')),
        );
      }
      return null;
    }
  }

  Future<void> _takePatrolPhoto(String templateId) async {
    final file = await _pickImage(ImageSource.camera);
    if (file != null && mounted) {
      setState(() {
        _patrolPhotos[templateId] = file;
      });
    }
  }

  Future<void> _showAddTemplateDialog() async {
    final nameCtrl = TextEditingController();
    final locCtrl = TextEditingController();
    final formKey = GlobalKey<FormState>();

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Tambah Titik Kontrol Gang'),
        content: Form(
          key: formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                controller: nameCtrl,
                decoration: const InputDecoration(
                  labelText: 'Nama Gang',
                  hintText: 'Cth: Gang Mawar 1',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
                validator: (v) =>
                    (v == null || v.trim().isEmpty) ? 'Nama gang wajib diisi' : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: locCtrl,
                decoration: const InputDecoration(
                  labelText: 'Label Lokasi / Patokan',
                  hintText: 'Cth: Depan Pos Ronda / Rumah No. 12',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
                validator: (v) =>
                    (v == null || v.trim().isEmpty) ? 'Label lokasi wajib diisi' : null,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () {
              if (formKey.currentState?.validate() == true) {
                Navigator.pop(ctx, true);
              }
            },
            child: const Text('Simpan'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        final api = ref.read(securityApiClientProvider);
        await api.createPatrolTemplate(
          name: nameCtrl.text.trim(),
          locationLabel: locCtrl.text.trim(),
        );
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Titik kontrol gang berhasil ditambahkan.')),
        );
        await _reloadPatrolTemplates();
      } catch (e) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal menambahkan gang: $e')),
        );
      }
    }
  }

  Future<void> _showEditTemplateDialog(PatrolTemplate t) async {
    final nameCtrl = TextEditingController(text: t.name);
    final locCtrl = TextEditingController(text: t.locationLabel);
    final formKey = GlobalKey<FormState>();

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Ubah Titik Kontrol Gang'),
        content: Form(
          key: formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                controller: nameCtrl,
                decoration: const InputDecoration(
                  labelText: 'Nama Gang',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
                validator: (v) =>
                    (v == null || v.trim().isEmpty) ? 'Nama gang wajib diisi' : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: locCtrl,
                decoration: const InputDecoration(
                  labelText: 'Label Lokasi / Patokan',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
                validator: (v) =>
                    (v == null || v.trim().isEmpty) ? 'Label lokasi wajib diisi' : null,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () {
              if (formKey.currentState?.validate() == true) {
                Navigator.pop(ctx, true);
              }
            },
            child: const Text('Simpan Perubahan'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        final api = ref.read(securityApiClientProvider);
        await api.updatePatrolTemplate(
          t.id,
          name: nameCtrl.text.trim(),
          locationLabel: locCtrl.text.trim(),
        );
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Titik kontrol gang berhasil diubah.')),
        );
        await _reloadPatrolTemplates();
      } catch (e) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal mengubah gang: $e')),
        );
      }
    }
  }

  Future<void> _confirmDeleteTemplate(PatrolTemplate t) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hapus Titik Kontrol?'),
        content: Text('Apakah Anda yakin ingin menghapus "${t.name}"?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: AppColors.danger),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        final api = ref.read(securityApiClientProvider);
        await api.deletePatrolTemplate(t.id);
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Titik kontrol gang berhasil dihapus.')),
        );
        await _reloadPatrolTemplates();
      } catch (e) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal menghapus gang: $e')),
        );
      }
    }
  }

  Widget _patrolTemplatesTab(ThemeData theme) {
    return RefreshIndicator(
      onRefresh: _reloadPatrolTemplates,
      child: ListView(
        padding: const EdgeInsets.all(12),
        children: [
          Card(
            color: AppColors.accent.withValues(alpha: 0.08),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  const Icon(Icons.info_outline, color: AppColors.accent),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'Kelola titik kontrol patroli wilayah RT. Petugas keamanan akan mengambil foto kontrol pada setiap titik kontrol yang aktif.',
                      style: theme.textTheme.bodySmall?.copyWith(fontSize: 12),
                    ),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 10),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Daftar Gang & Titik Kontrol (${_patrolTemplates.length})',
                style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
              ),
              FilledButton.icon(
                icon: const Icon(Icons.add, size: 18),
                label: const Text('Tambah Gang'),
                onPressed: _showAddTemplateDialog,
              ),
            ],
          ),
          const SizedBox(height: 10),
          if (_patrolTemplates.isEmpty) ...[
            const SizedBox(height: 40),
            const Center(child: Text('Belum ada titik kontrol gang.')),
            const SizedBox(height: 12),
            Center(
              child: OutlinedButton.icon(
                icon: const Icon(Icons.add),
                label: const Text('Tambah Titik Kontrol Pertama'),
                onPressed: _showAddTemplateDialog,
              ),
            ),
          ] else ...[
            ..._patrolTemplates.map((t) {
              return Card(
                margin: const EdgeInsets.only(bottom: 10),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Row(
                    children: [
                      const Icon(Icons.location_on, color: AppColors.seed, size: 24),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              t.name,
                              style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                            ),
                            if (t.locationLabel.isNotEmpty)
                              Text(
                                t.locationLabel,
                                style: TextStyle(
                                  color: theme.colorScheme.onSurfaceVariant,
                                  fontSize: 12,
                                ),
                              ),
                          ],
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.edit_outlined, size: 20),
                        tooltip: 'Ubah',
                        onPressed: () => _showEditTemplateDialog(t),
                      ),
                      IconButton(
                        icon: const Icon(Icons.delete_outline, color: AppColors.danger, size: 20),
                        tooltip: 'Hapus',
                        onPressed: () => _confirmDeleteTemplate(t),
                      ),
                    ],
                  ),
                ),
              );
            }),
          ],
        ],
      ),
    );
  }

  Future<void> _submitPatrol() async {
    if (_submittingPatrol) return;
    if (_patrolPhotos.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Ambil minimal 1 foto titik kontrol patroli.')),
      );
      return;
    }
    setState(() => _submittingPatrol = true);
    try {
      final api = ref.read(securityApiClientProvider);
      final List<Map<String, dynamic>> photos = [];
      for (final entry in _patrolPhotos.entries) {
        final bytes = await entry.value.readAsBytes();
        final key = await api.uploadPatrolPhoto(bytes, entry.value.name);
        photos.add({
          'patrol_template_id': entry.key,
          'image_key': key,
          'captured_at': DateTime.now().toUtc().toIso8601String(),
        });
      }
      final notes = _patrolNotesCtrl.text.trim();
      await api.createPatrolBundle(
        notes: notes.isNotEmpty ? notes : null,
        photos: photos,
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Laporan patroli berhasil dikirim.')),
      );
      setState(() {
        _patrolPhotos.clear();
        _patrolNotesCtrl.clear();
      });
      await _reloadPatrolBundles();
      if (mounted) {
        setState(() => _patroliTabs.index = 1);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal mengirim patroli: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _submittingPatrol = false);
    }
  }

  Future<void> _viewPatrolPhotos(PatrolBundle bundle) async {
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(
          'Foto Patroli - ${bundle.dutyOfficerName.isNotEmpty ? bundle.dutyOfficerName : bundle.createdByName}',
        ),
        content: SizedBox(
          width: 360,
          child: bundle.photos.isEmpty
              ? const Text('Tidak ada foto terlampir.')
              : ListView.builder(
                  shrinkWrap: true,
                  itemCount: bundle.photos.length,
                  itemBuilder: (ctx, idx) {
                    final p = bundle.photos[idx];
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            p.templateName.isNotEmpty
                                ? p.templateName
                                : 'Titik Kontrol #${idx + 1}',
                            style: const TextStyle(fontWeight: FontWeight.bold),
                          ),
                          if (p.locationLabel.isNotEmpty)
                            Text(
                              p.locationLabel,
                              style: const TextStyle(fontSize: 12, color: Colors.grey),
                            ),
                          const SizedBox(height: 6),
                          GestureDetector(
                            onTap: p.imageUrl.isNotEmpty
                                ? () {
                                    final viewerItems = bundle.photos
                                        .where((ph) => ph.imageUrl.isNotEmpty)
                                        .map((ph) => SecurityPhotoViewerItem(
                                              imageUrl: ph.imageUrl,
                                              title: ph.templateName.isNotEmpty
                                                  ? ph.templateName
                                                  : 'Titik Kontrol #${idx + 1}',
                                              subtitle: ph.locationLabel,
                                            ))
                                        .toList();
                                    final curIdx = bundle.photos
                                        .where((ph) => ph.imageUrl.isNotEmpty)
                                        .toList()
                                        .indexOf(p);
                                    SecurityPhotoViewerScreen.show(
                                      ctx,
                                      photos: viewerItems,
                                      initialIndex: curIdx >= 0 ? curIdx : 0,
                                    );
                                  }
                                : null,
                            child: ClipRRect(
                              borderRadius: BorderRadius.circular(8),
                              child: p.imageUrl.isNotEmpty
                                  ? AuthenticatedNetworkImage(
                                      imageUrl: p.imageUrl,
                                      height: 180,
                                      width: double.infinity,
                                      fit: BoxFit.cover,
                                    )
                                  : Container(
                                      height: 180,
                                      color: Colors.grey.shade200,
                                      child: const Center(
                                        child: Icon(Icons.broken_image),
                                      ),
                                    ),
                            ),
                          ),
                        ],
                      ),
                    );
                  },
                ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Tutup')),
        ],
      ),
    );
  }

  void _searchWarga(String q) {
    _wargaSearchDebounce?.cancel();
    _wargaSearchDebounce = Timer(const Duration(milliseconds: 300), () async {
      final term = q.trim();
      if (term.isEmpty) {
        setState(() => _wargaResults = []);
        return;
      }
      setState(() => _searchingWarga = true);
      try {
        final api = ref.read(securityApiClientProvider);
        final results = await api.searchWargaMasked(term);
        if (mounted) {
          setState(() {
            _wargaResults = results;
            _searchingWarga = false;
          });
        }
      } catch (_) {
        if (mounted) setState(() => _searchingWarga = false);
      }
    });
  }

  void _addPaketDraftItem() {
    setState(() {
      _paketDraftItems.add(_PaketItemDraft());
    });
  }

  void _removePaketDraftItem(int index) {
    if (_paketDraftItems.length <= 1) return;
    setState(() {
      _paketDraftItems[index].dispose();
      _paketDraftItems.removeAt(index);
    });
  }

  Future<void> _pickPaketItemPhoto(_PaketItemDraft item) async {
    final file = await _pickImage(ImageSource.camera);
    if (file != null && mounted) {
      setState(() => item.photo = file);
    }
  }

  Future<void> _submitPaket() async {
    if (_submittingPaket) return;
    if (_selectedWarga == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Pilih warga penerima paket terlebih dahulu.')),
      );
      return;
    }
    setState(() => _submittingPaket = true);
    try {
      final api = ref.read(securityApiClientProvider);
      final List<Map<String, dynamic>> items = [];
      for (final draft in _paketDraftItems) {
        final List<String> imageKeys = [];
        if (draft.photo != null) {
          final bytes = await draft.photo!.readAsBytes();
          final key = await api.uploadPaketPhoto(bytes, draft.photo!.name);
          if (key.isNotEmpty) imageKeys.add(key);
        }
        final courier = draft.courierCtrl.text.trim();
        final resi = draft.resiCtrl.text.trim();
        final desc = draft.descCtrl.text.trim();
        final parts = [
          if (courier.isNotEmpty) 'Kurir: $courier',
          if (resi.isNotEmpty) 'Resi: $resi',
          if (desc.isNotEmpty) desc,
        ];
        final fullDesc = parts.isEmpty ? 'Paket Titipan' : parts.join(' · ');
        items.add({
          'description': fullDesc,
          'image_keys': imageKeys,
        });
      }

      await api.createPaketReport(
        receiverWargaId: _selectedWarga!.id,
        items: items,
      );

      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Penitipan paket berhasil dicatat & notifikasi terkirim.'),
        ),
      );

      setState(() {
        _selectedWarga = null;
        _wargaSearchCtrl.clear();
        _wargaResults.clear();
        for (final item in _paketDraftItems) {
          item.dispose();
        }
        _paketDraftItems.clear();
        _paketDraftItems.add(_PaketItemDraft());
      });
      await _reloadPaketReports();
      if (mounted) {
        setState(() => _paketTabs.index = 1);
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Gagal menyimpan paket: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _submittingPaket = false);
    }
  }

  Future<void> _openSerahTerimaDialog(PaketReport report, PaketItem item) async {
    final noteCtrl = TextEditingController(text: report.receiverName);
    XFile? proofPhoto;

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Serah Terima Paket'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Penerima: ${report.receiverName} (${report.receiverHouseNumber})'),
                if (item.description != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    'Paket: ${item.description}',
                    style: const TextStyle(fontSize: 12, color: Colors.grey),
                  ),
                ],
                const SizedBox(height: 12),
                TextField(
                  controller: noteCtrl,
                  decoration: const InputDecoration(
                    labelText: 'Nama Pengambil / Catatan',
                    hintText: 'Cth: Warga bersangkutan / Keluarga',
                    border: OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
                const SizedBox(height: 12),
                const Text(
                  'Foto Bukti Pengambilan (Wajib):',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 13),
                ),
                const SizedBox(height: 6),
                if (proofPhoto != null)
                  Stack(
                    alignment: Alignment.topRight,
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(8),
                        child: kIsWeb
                            ? Image.network(
                                proofPhoto!.path,
                                height: 140,
                                width: double.infinity,
                                fit: BoxFit.cover,
                              )
                            : Image.file(
                                File(proofPhoto!.path),
                                height: 140,
                                width: double.infinity,
                                fit: BoxFit.cover,
                              ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close, color: Colors.red),
                        onPressed: () => setDialogState(() => proofPhoto = null),
                      ),
                    ],
                  )
                else
                  OutlinedButton.icon(
                    icon: const Icon(Icons.camera_alt),
                    label: const Text('Ambil Foto Bukti Serah Terima'),
                    onPressed: () async {
                      final f = await _pickImage(ImageSource.camera);
                      if (f != null) {
                        setDialogState(() => proofPhoto = f);
                      }
                    },
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Batal'),
            ),
            FilledButton(
              onPressed: proofPhoto == null
                  ? null
                  : () => Navigator.pop(ctx, true),
              child: const Text('Konfirmasi'),
            ),
          ],
        ),
      ),
    );

    if (confirmed == true && proofPhoto != null) {
      try {
        final api = ref.read(securityApiClientProvider);
        final bytes = await proofPhoto!.readAsBytes();
        final key = await api.uploadPaketPhoto(bytes, proofPhoto!.name);
        await api.updatePaketItemStatus(
          report.id,
          item.id,
          status: 'sudah_diambil',
          proofImageKey: key,
        );
        ref.invalidate(securityAlertsProvider);
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Paket berhasil diserahkan.')),
        );
        await _reloadPaketReports();
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Gagal serah terima paket: $e')),
          );
        }
      }
    }
  }

  Future<void> _viewPaketPhotos(PaketItem item) async {
    await showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Foto Paket'),
        content: SizedBox(
          width: 360,
          child: item.photos.isEmpty
              ? const Text('Tidak ada foto terlampir.')
              : ListView.builder(
                  shrinkWrap: true,
                  itemCount: item.photos.length,
                  itemBuilder: (ctx, idx) {
                    final p = item.photos[idx];
                    final isProof = p.photoType == 'bukti_pengambilan';
                    return Padding(
                      padding: const EdgeInsets.only(bottom: 12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            isProof ? 'Bukti Serah Terima' : 'Foto Paket Saat Dititipkan',
                            style: TextStyle(
                              fontWeight: FontWeight.bold,
                              color: isProof ? AppColors.success : AppColors.seed,
                            ),
                          ),
                          const SizedBox(height: 6),
                          GestureDetector(
                            onTap: p.imageUrl.isNotEmpty
                                ? () {
                                    final viewerItems = item.photos
                                        .where((ph) => ph.imageUrl.isNotEmpty)
                                        .map((ph) => SecurityPhotoViewerItem(
                                              imageUrl: ph.imageUrl,
                                              title: ph.photoType == 'bukti_pengambilan'
                                                  ? 'Bukti Serah Terima'
                                                  : ((item.description?.isNotEmpty ?? false)
                                                      ? item.description!
                                                      : 'Foto Paket'),
                                              subtitle: isProof
                                                  ? 'Bukti Serah Terima / Pengambilan Paket'
                                                  : 'Foto Paket Saat Dititipkan',
                                            ))
                                        .toList();
                                    final curIdx = item.photos
                                        .where((ph) => ph.imageUrl.isNotEmpty)
                                        .toList()
                                        .indexOf(p);
                                    SecurityPhotoViewerScreen.show(
                                      ctx,
                                      photos: viewerItems,
                                      initialIndex: curIdx >= 0 ? curIdx : 0,
                                    );
                                  }
                                : null,
                            child: ClipRRect(
                              borderRadius: BorderRadius.circular(8),
                              child: p.imageUrl.isNotEmpty
                                  ? AuthenticatedNetworkImage(
                                      imageUrl: p.imageUrl,
                                      height: 180,
                                      width: double.infinity,
                                      fit: BoxFit.cover,
                                    )
                                  : Container(
                                      height: 180,
                                      color: Colors.grey.shade200,
                                      child: const Center(
                                        child: Icon(Icons.broken_image),
                                      ),
                                    ),
                            ),
                          ),
                        ],
                      ),
                    );
                  },
                ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Tutup')),
        ],
      ),
    );
  }

  void _debounceGuestSearch(String value) {
    _guestDebounce?.cancel();
    _guestDebounce = Timer(
      const Duration(milliseconds: 350),
      () => _loadGuestSearch(value),
    );
  }

  Future<void> _loadGuestPage(int page) async {
    setState(() {
      _guestPage = page;
      _loading = true;
    });
    try {
      final result = await ref
          .read(securityApiClientProvider)
          .listManagerGuestBooks(search: _guestSearch, page: page);
      if (!mounted) {
        return;
      }
      final sortedGuests = [...result.data]
        ..sort(compareGuestBookChronological);
      setState(() {
        _guests = sortedGuests;
        _guestTotal = result.total;
        _loading = false;
      });
      _ensureGuestTicker();
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  Future<void> _loadData() async {
    final api = ref.read(securityApiClientProvider);
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final canReadSchedules =
          userHasPermission(_user, 'security.schedule.read') || _canWrite;
      final canReadCorrections =
          _canDecide || userHasPermission(_user, 'security.attendance');
      final officers = canReadSchedules
          ? await api.listManagerOfficers()
          : <SecurityOfficer>[];
      final shifts = canReadSchedules
          ? await api.listManagerShifts()
          : <ShiftConfig>[];
      final schedules = canReadSchedules
          ? await api.listManagerSchedules(
              dateFrom: _period == null
                  ? null
                  : DateFormat('yyyy-MM-dd').format(_period!.start),
              dateTo: _period == null
                  ? null
                  : DateFormat('yyyy-MM-dd').format(_period!.end),
              userId: _officerFilter,
            )
          : <SecuritySchedule>[];
      final attendance = canReadSchedules
          ? await api.listManagerAttendance(
              dateFrom: _period == null
                  ? null
                  : DateFormat('yyyy-MM-dd').format(_period!.start),
              dateTo: _period == null
                  ? null
                  : DateFormat('yyyy-MM-dd').format(_period!.end),
              userId: _officerFilter,
              status: _attendanceStatus.isEmpty ? null : _attendanceStatus,
            )
          : <AttendanceRecord>[];
      final corrections = canReadCorrections
          ? await api.listManagerCorrections()
          : <AttendanceCorrection>[];
      final devices = _canManageDevices
          ? await api.listManagerDevices()
          : <SecurityDevice>[];
      final guestPage = _canGuestRead
          ? await api.listManagerGuestBooks(
              search: _guestSearch,
              page: _guestPage,
            )
          : const GuestBookPage([], 0, 1);
      final patrolTemplates = _canPatrolRead
          ? await api.listPatrolTemplates()
          : <PatrolTemplate>[];
      final patrolBundles = _canPatrolRead
          ? await api.listPatrolBundles()
          : <PatrolBundle>[];
      final paketReports = _canPaketRead
          ? await api.listPaketReports()
          : <PaketReport>[];
      final sortedAttendance = [...attendance]
        ..sort(compareAttendanceRecency);
      final sortedGuests = [...guestPage.data]
        ..sort(compareGuestBookChronological);
      if (!mounted) {
        return;
      }
      setState(() {
        _officers = officers;
        _shifts = shifts;
        _schedules = schedules;
        _attendance = sortedAttendance;
        _corrections = corrections;
        _devices = devices;
        _guests = sortedGuests;
        _guestTotal = guestPage.total;
        _patrolTemplates = patrolTemplates;
        _patrolBundles = patrolBundles;
        _paketReports = paketReports;
        _loading = false;
      });
      _ensureGuestTicker();
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _loadGuestSearch(String value) async {
    setState(() {
      _guestSearch = value;
      _guestPage = 1;
      _loading = true;
      _error = null;
    });
    try {
      final result = await ref
          .read(securityApiClientProvider)
          .listManagerGuestBooks(search: value, page: 1);
      if (!mounted) {
        return;
      }
      final sortedGuests = [...result.data]
        ..sort(compareGuestBookChronological);
      setState(() {
        _guests = sortedGuests;
        _guestTotal = result.total;
        _loading = false;
      });
      _ensureGuestTicker();
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  Future<void> _showGuestDetail(GuestBookEntry g) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(g.guestName),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _detail('Tujuan', g.destinationLabel),
              _detail('Keperluan', g.purpose),
              _detail(
                'Masuk',
                DateFormat('dd/MM/yyyy HH:mm').format(g.checkedInAt),
              ),
              _detail(
                'Keluar',
                g.checkedOutAt == null
                    ? 'Belum keluar'
                    : DateFormat('dd/MM/yyyy HH:mm').format(g.checkedOutAt!),
              ),
              _detail('Telepon', g.phoneMasked ?? '-'),
              _detail('Perangkat', g.deviceName),
              _detail('Catatan', g.notes ?? '-'),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Tutup'),
          ),
        ],
      ),
    );
  }

  Widget _detail(String label, String value) => Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Text('$label: $value'),
  );

  Future<void> _setDateFilter() async {
    final now = DateTime.now();
    final selected = await showDateRangePicker(
      context: context,
      firstDate: DateTime(now.year - 5),
      lastDate: DateTime(now.year + 2),
      initialDateRange: _period,
    );
    if (selected != null) {
      setState(() => _period = selected);
      await _loadData();
    }
  }

  /// Context-sensitive accent FAB showing the primary "Tambah" action for
  /// the active sub-tab. Android-only per current design rules; matches
  /// Lokasi Dana's pill accent button pattern.
  Widget? _buildSecurityFab() {
    if (widget.mode != SecurityManagementMode.full) {
      return null;
    }
    // Kelola group → Shift / Jadwal only (Petugas has none)
    if (_topTabs.index == 0) {
      if (_kelolaTabs.index == 1 && _canWrite) {
        return AccentActionFab(
          tooltip: 'Tambah Shift',
          icon: Icons.schedule,
          onPressed: () => _openShiftForm(),
        );
      }
      if (_kelolaTabs.index == 2 && _canWrite) {
        return AccentActionFab(
          tooltip: 'Tambah Jadwal',
          icon: Icons.event,
          onPressed: () => _openScheduleForm(),
        );
      }
      return null;
    }
    // Perangkat group
    if (_topTabs.index == 2 && _canManageDevices) {
      return AccentActionFab(
        tooltip: 'Tambah Perangkat',
        icon: Icons.tablet_android,
        onPressed: _openCreateDeviceForm,
      );
    }
    // Patroli group
    if (_topTabs.index == 3 && _patroliTabs.index == 0 && _canPatrolWrite) {
      return AccentActionFab(
        tooltip: 'Kirim Patroli',
        icon: Icons.send,
        onPressed: _submitPatrol,
      );
    }
    // Paket group
    if (_topTabs.index == 4 && _paketTabs.index == 1 && _canPaketWrite) {
      return AccentActionFab(
        tooltip: 'Titip Paket',
        icon: Icons.add_box,
        onPressed: () => setState(() => _paketTabs.index = 0),
      );
    }
    return null;
  }

  Future<void> _openCreateDeviceForm() async {
    if (!_canManageDevices) {
      return;
    }
    final name = TextEditingController(text: 'Gerbang Urena');
    final location = TextEditingController(text: 'Pos Keamanan Urena');
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Tambah Perangkat Pos'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: name,
              decoration: const InputDecoration(
                labelText: 'Nama perangkat',
                hintText: 'Contoh: Gerbang Urena',
              ),
            ),
            TextField(
              controller: location,
              decoration: const InputDecoration(
                labelText: 'Lokasi pemasangan',
                hintText: 'Contoh: Pos Keamanan Urena',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Simpan'),
          ),
        ],
      ),
    );
    if (confirm != true || !mounted) {
      return;
    }
    final deviceName = name.text.trim();
    final locationLabel = location.text.trim();
    if (deviceName.length < 2 || deviceName.length > 100) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Nama perangkat harus 2-100 karakter.')),
      );
      return;
    }
    try {
      final api = ref.read(securityApiClientProvider);
      final result = await api.createManagerDevice(
        deviceName: deviceName,
        locationLabel: locationLabel.isEmpty
            ? 'Pos Keamanan Utama'
            : locationLabel,
      );
      await _loadData();
      if (!mounted) return;
      await _showPairingTokenDialog(
        result,
        title: 'Perangkat Pos berhasil dibuat',
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }

  Future<void> _openScheduleForm({SecuritySchedule? item}) async {
    if (!_canWrite) {
      return;
    }
    final userId =
        item?.userId ?? (_officers.isEmpty ? '' : _officers.first.userId);
    final shiftId =
        item?.shiftConfigId ?? (_shifts.isEmpty ? '' : _shifts.first.id);
    String selectedUser = userId;
    String selectedShift = shiftId;
    bool isActive = item?.isActive ?? true;
    final notes = TextEditingController(text: item?.notes ?? '');
    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialog) => AlertDialog(
          title: Text(
            item == null ? 'Tambah Jadwal Petugas' : 'Edit Jadwal Petugas',
          ),
          content: SizedBox(
            width: 420,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DropdownButtonFormField<String>(
                    value: selectedShift.isEmpty ? null : selectedShift,
                    decoration: const InputDecoration(labelText: 'Shift'),
                    items: _shifts
                        .where((s) => s.isActive)
                        .map(
                          (s) => DropdownMenuItem(
                            value: s.id,
                            child: Text(
                              '${s.name} (${s.startTime}–${s.endTime})',
                            ),
                          ),
                        )
                        .toList(),
                    onChanged: (v) => setDialog(() => selectedShift = v ?? ''),
                  ),
                  const SizedBox(height: 12),
                  DropdownButtonFormField<String>(
                    value: selectedUser.isEmpty ? null : selectedUser,
                    decoration: const InputDecoration(labelText: 'Petugas'),
                    items: _officers
                        .where((o) => o.isActive)
                        .map(
                          (o) => DropdownMenuItem(
                            value: o.userId,
                            child: Text(o.fullName),
                          ),
                        )
                        .toList(),
                    onChanged: (v) => setDialog(() => selectedUser = v ?? ''),
                  ),
                  const SizedBox(height: 12),
                  SwitchListTile(
                    title: const Text('Status Penugasan'),
                    subtitle: Text(isActive ? 'Aktif' : 'Nonaktif'),
                    value: isActive,
                    onChanged: (val) => setDialog(() => isActive = val),
                    contentPadding: EdgeInsets.zero,
                  ),
                  TextField(
                    controller: notes,
                    decoration: const InputDecoration(labelText: 'Catatan (Opsional)'),
                  ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Batal'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Simpan'),
            ),
          ],
        ),
      ),
    );
    if (saved != true || selectedUser.isEmpty || selectedShift.isEmpty) {
      return;
    }
    final payload = {
      'user_id': selectedUser,
      'shift_config_id': selectedShift,
      'is_active': isActive,
      'notes': notes.text.trim(),
    };
    try {
      final api = ref.read(securityApiClientProvider);
      if (item == null) {
        await api.createManagerSchedule(payload);
      } else {
        await api.updateManagerSchedule(item.id, payload);
      }
      await _loadData();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }

  TimeOfDay _parseShiftTime(String? raw, TimeOfDay fallback) {
    if (raw == null || raw.trim().isEmpty) {
      return fallback;
    }
    final parts = raw.trim().split(':');
    if (parts.length >= 2) {
      final hour = int.tryParse(parts[0]);
      final minute = int.tryParse(parts[1]);
      if (hour != null &&
          minute != null &&
          hour >= 0 &&
          hour <= 23 &&
          minute >= 0 &&
          minute <= 59) {
        return TimeOfDay(hour: hour, minute: minute);
      }
    }
    return fallback;
  }

  Future<void> _openShiftForm({ShiftConfig? item}) async {
    if (!_canWrite) {
      return;
    }
    final startTime = _parseShiftTime(
      item?.startTime,
      const TimeOfDay(hour: 8, minute: 0),
    );
    final endTime = _parseShiftTime(
      item?.endTime,
      const TimeOfDay(hour: 16, minute: 0),
    );
    final api = ref.read(securityApiClientProvider);

    final savedName = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => _ShiftFormDialog(
        item: item,
        api: api,
        initialStartTime: startTime,
        initialEndTime: endTime,
      ),
    );

    if (savedName != null && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            item == null
                ? 'Shift "$savedName" berhasil ditambahkan'
                : 'Shift "$savedName" berhasil diperbarui',
          ),
          backgroundColor: AppColors.success,
        ),
      );
      await _loadData();
    }
  }

  Future<void> _decideCorrection(AttendanceCorrection c, bool approve) async {
    if (!_canDecide || c.requestedBy == (_user?['id']?.toString() ?? '')) {
      return;
    }
    final note = TextEditingController();
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(approve ? 'Setujui Koreksi' : 'Tolak Koreksi'),
        content: TextField(
          controller: note,
          decoration: const InputDecoration(labelText: 'Catatan keputusan'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Konfirmasi'),
          ),
        ],
      ),
    );
    if (confirm != true) {
      return;
    }
    try {
      await ref
          .read(securityApiClientProvider)
          .decideCorrection(c.id, approve: approve, note: note.text.trim());
      await _loadData();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }

  Future<void> _revokeDevice(SecurityDevice d) async {
    if (!_canManageDevices) {
      return;
    }
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Nonaktifkan perangkat?'),
        content: Text('${d.deviceName} — ${d.locationLabel}'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Nonaktifkan'),
          ),
        ],
      ),
    );
    if (ok != true) {
      return;
    }
    try {
      await ref.read(securityApiClientProvider).revokeDevice(d.id);
      await _loadData();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }

  Future<void> _rotatePairingToken(SecurityDevice d) async {
    if (!_canManageDevices) {
      return;
    }
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Buat token baru?'),
        content: Text(
          '${d.deviceName} — token pairing lama akan diganti. '
          'Perangkat perlu diaktifkan ulang dengan token baru.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Ya, Buat Ulang'),
          ),
        ],
      ),
    );
    if (ok != true) {
      return;
    }
    try {
      final result = await ref
          .read(securityApiClientProvider)
          .rotateDevicePairing(d.id);
      await _loadData();
      if (!mounted) {
        return;
      }
      await _showPairingTokenDialog(result, title: 'Token pairing baru dibuat');
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }

  Future<void> _showPairingTokenDialog(
    DeviceProvisionResult result, {
    required String title,
  }) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Nama: ${result.device.deviceName}'),
            const SizedBox(height: 8),
            const Text(
              'Gunakan kode pairing ini di halaman Aktivasi Pos pada perangkat kiosk. Kode ini hanya tampil satu kali.',
            ),
            const SizedBox(height: 8),
            SelectableText(
              result.pairingToken,
              style: const TextStyle(
                fontFamily: 'monospace',
                fontSize: 16,
                fontWeight: FontWeight.bold,
              ),
            ),
          ],
        ),
        actions: [
          FilledButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Tutup'),
          ),
        ],
      ),
    );
  }

  /// Solid background for the tab header (same as app bar surface).
  Widget _securityTabsHeader() {
    final theme = Theme.of(context);
    final alertsData =
        ref.watch(securityAlertsDataProvider).valueOrNull ??
        const SecurityAlertsData();

    return Material(
      color: theme.scaffoldBackgroundColor,
      child: Column(
        children: [
          // Level 1 — group tabs (responsive 5 tabs with zero empty gap)
          TabBar(
            controller: _topTabs,
            isScrollable: false,
            labelColor: AppColors.accent,
            unselectedLabelColor: theme.colorScheme.onSurfaceVariant,
            indicatorColor: AppColors.accent,
            indicatorSize: TabBarIndicatorSize.tab,
            indicatorWeight: 3,
            dividerColor: Colors.transparent,
            labelStyle: const TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w700,
            ),
            unselectedLabelStyle: const TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
            ),
            tabs: [
              const Tab(icon: Icon(Icons.groups_outlined, size: 20), text: 'Kelola'),
              Tab(
                icon: Badge(
                  isLabelVisible: alertsData.pendingCorrectionsUnread > 0,
                  label: Text('${alertsData.pendingCorrectionsUnread}'),
                  child: const Icon(Icons.qr_code_scanner, size: 20),
                ),
                text: 'Absensi',
              ),
              const Tab(icon: Icon(Icons.tablet, size: 20), text: 'Perangkat'),
              Tab(
                icon: Badge(
                  isLabelVisible: alertsData.patrolUnread > 0,
                  label: Text('${alertsData.patrolUnread}'),
                  child: const Icon(Icons.local_police_outlined, size: 20),
                ),
                text: 'Patroli',
              ),
              Tab(
                icon: Badge(
                  isLabelVisible: alertsData.paketUnread > 0,
                  label: Text('${alertsData.paketUnread}'),
                  child: const Icon(Icons.inventory_2_outlined, size: 20),
                ),
                text: 'Paket',
              ),
            ],
          ),
          const Divider(height: 1),
          // Level 2 — contextual sub tabs
          _subTabsBar(theme, alertsData),
        ],
      ),
    );
  }

  Widget _subTabsBar(ThemeData theme, SecurityAlertsData alertsData) {
    const unselected = Color(0xff8a9ab0);
    switch (_topTabs.index) {
      case 1:
        return TabBar(
          controller: _absensiTabs,
          isScrollable: false,
          labelColor: theme.colorScheme.onSurface,
          unselectedLabelColor: unselected,
          indicatorColor: AppColors.accent,
          indicatorSize: TabBarIndicatorSize.tab,
          indicatorWeight: 2,
          dividerColor: Colors.transparent,
          labelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w700,
          ),
          unselectedLabelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
          ),
          tabs: [
            const Tab(icon: Icon(Icons.fingerprint, size: 16), text: 'Absensi'),
            Tab(
              icon: Badge(
                isLabelVisible: alertsData.pendingCorrectionsUnread > 0,
                label: Text('${alertsData.pendingCorrectionsUnread}'),
                child: const Icon(Icons.edit_note, size: 16),
              ),
              text: 'Koreksi',
            ),
          ],
        );
      case 2:
        return const SizedBox.shrink();
      case 3:
        final manageOnly = _canManageTemplates && !_canPatrolWrite;
        return TabBar(
          controller: _patroliTabs,
          isScrollable: false,
          labelColor: theme.colorScheme.onSurface,
          unselectedLabelColor: unselected,
          indicatorColor: AppColors.accent,
          indicatorSize: TabBarIndicatorSize.tab,
          indicatorWeight: 2,
          dividerColor: Colors.transparent,
          labelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w700,
          ),
          unselectedLabelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
          ),
          tabs: [
            Tab(
              icon: Icon(
                manageOnly ? Icons.list_alt : Icons.camera_alt_outlined,
                size: 16,
              ),
              text: manageOnly ? 'Template Gang' : 'Ambil Foto',
            ),
            Tab(
              icon: Badge(
                isLabelVisible: alertsData.patrolUnread > 0,
                label: Text('${alertsData.patrolUnread}'),
                child: const Icon(Icons.history, size: 16),
              ),
              text: 'Riwayat',
            ),
          ],
        );
      case 4:
        if (!_canPaketWrite) {
          return TabBar(
            controller: _paketTabs,
            isScrollable: false,
            labelColor: theme.colorScheme.onSurface,
            unselectedLabelColor: unselected,
            indicatorColor: AppColors.accent,
            indicatorSize: TabBarIndicatorSize.tab,
            indicatorWeight: 2,
            dividerColor: Colors.transparent,
            labelStyle: const TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w700,
            ),
            unselectedLabelStyle: const TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
            ),
            tabs: [
              Tab(
                icon: Badge(
                  isLabelVisible: alertsData.paketUnread > 0,
                  label: Text('${alertsData.paketUnread}'),
                  child: const Icon(Icons.inventory, size: 16),
                ),
                text: 'Daftar Paket',
              ),
            ],
          );
        }
        return TabBar(
          controller: _paketTabs,
          isScrollable: false,
          labelColor: theme.colorScheme.onSurface,
          unselectedLabelColor: unselected,
          indicatorColor: AppColors.accent,
          indicatorSize: TabBarIndicatorSize.tab,
          indicatorWeight: 2,
          dividerColor: Colors.transparent,
          labelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w700,
          ),
          unselectedLabelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
          ),
          tabs: [
            const Tab(icon: Icon(Icons.add_box_outlined, size: 16), text: 'Penerimaan'),
            Tab(
              icon: Badge(
                isLabelVisible: alertsData.paketUnread > 0,
                label: Text('${alertsData.paketUnread}'),
                child: const Icon(Icons.inventory, size: 16),
              ),
              text: 'Daftar Paket',
            ),
          ],
        );
      default:
        return TabBar(
          controller: _kelolaTabs,
          isScrollable: false,
          labelColor: theme.colorScheme.onSurface,
          unselectedLabelColor: unselected,
          indicatorColor: AppColors.accent,
          indicatorSize: TabBarIndicatorSize.tab,
          indicatorWeight: 2,
          dividerColor: Colors.transparent,
          labelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w700,
          ),
          unselectedLabelStyle: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
          ),
          tabs: const [
            Tab(icon: Icon(Icons.person_outline, size: 16), text: 'Petugas'),
            Tab(icon: Icon(Icons.schedule, size: 16), text: 'Shift'),
            Tab(icon: Icon(Icons.event, size: 16), text: 'Jadwal'),
          ],
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final permissions = _user?['effective_permissions'] is List
        ? (_user!['effective_permissions'] as List).whereType<String>().toSet()
        : <String>{};
    final canRead =
        permissions.contains('security.schedule.read') ||
        permissions.contains('security.schedule.write') ||
        permissions.contains('security.device.manage') ||
        permissions.contains('security.correction') ||
        permissions.contains('patrol.read') ||
        permissions.contains('paket.read') ||
        permissions.contains('security.attendance') ||
        _canGuestRead;
    if (!canRead) {
      return Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Keamanan'),
        ),
        body: const Center(
          child: Text('Anda tidak memiliki akses ke modul keamanan.'),
        ),
      );
    }

    if (widget.mode == SecurityManagementMode.officerPatrol) {
      return Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Patroli Wilayah'),
          actions: [
            IconButton(
              icon: const Icon(Icons.refresh),
              onPressed: () {
                _reloadPatrolTemplates();
                _reloadPatrolBundles();
              },
            ),
          ],
          bottom: TabBar(
            controller: _patroliTabs,
            labelColor: theme.colorScheme.onSurface,
            unselectedLabelColor: const Color(0xff8a9ab0),
            indicatorColor: AppColors.accent,
            indicatorSize: TabBarIndicatorSize.tab,
            indicatorWeight: 2,
            tabs: const [
              Tab(icon: Icon(Icons.camera_alt_outlined, size: 16), text: 'Ambil Foto'),
              Tab(icon: Icon(Icons.history, size: 16), text: 'Riwayat'),
            ],
          ),
        ),
        body: _loading
            ? const Center(child: CircularProgressIndicator())
            : TabBarView(
                controller: _patroliTabs,
                children: [
                  _patrolFormTab(),
                  _patrolHistoryTab(),
                ],
              ),
      );
    }

    if (widget.mode == SecurityManagementMode.officerPaket) {
      return Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Pengelolaan Paket'),
          actions: [
            IconButton(
              icon: const Icon(Icons.refresh),
              onPressed: _reloadPaketReports,
            ),
          ],
          bottom: TabBar(
            controller: _paketTabs,
            labelColor: theme.colorScheme.onSurface,
            unselectedLabelColor: const Color(0xff8a9ab0),
            indicatorColor: AppColors.accent,
            indicatorSize: TabBarIndicatorSize.tab,
            indicatorWeight: 2,
            tabs: const [
              Tab(icon: Icon(Icons.add_box_outlined, size: 16), text: 'Penerimaan'),
              Tab(icon: Icon(Icons.inventory, size: 16), text: 'Daftar Paket'),
            ],
          ),
        ),
        body: _loading
            ? const Center(child: CircularProgressIndicator())
            : TabBarView(
                controller: _paketTabs,
                children: [
                  _paketFormTab(),
                  _paketListTab(),
                ],
              ),
      );
    }

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        centerTitle: false,
        title: const MenuAppBarTitle(title: 'Keamanan'),
        actions: [
          IconButton(onPressed: _loadData, icon: const Icon(Icons.refresh)),
        ],
      ),
      floatingActionButton: _buildSecurityFab(),
      body: Column(
        children: [
          _securityTabsHeader(),
          const Divider(height: 1),
          Expanded(
            child: _loading
                ? const Center(child: CircularProgressIndicator())
                : _error != null
                ? Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(_error!, textAlign: TextAlign.center),
                        const SizedBox(height: 12),
                        FilledButton(
                          onPressed: _loadData,
                          child: const Text('Coba Lagi'),
                        ),
                      ],
                    ),
                  )
                : TabBarView(
                    controller: _topTabs,
                    children: [
                      TabBarView(
                        controller: _kelolaTabs,
                        children: [
                          _officerTab(),
                          if (userHasPermission(
                                _user,
                                'security.schedule.read',
                              ) ||
                              _canWrite)
                            _shiftTab()
                          else
                            _scheduleReadOnlyTab(),
                          _scheduleTab(),
                        ],
                      ),
                      TabBarView(
                        controller: _absensiTabs,
                        children: [_attendanceTab(), _correctionTab()],
                      ),
                      _deviceTab(),
                      TabBarView(
                        controller: _patroliTabs,
                        children: [
                          _canManageTemplates && !_canPatrolWrite
                              ? _patrolTemplatesTab(theme)
                              : _patrolFormTab(),
                          _patrolHistoryTab(),
                        ],
                      ),
                      _canPaketWrite
                          ? TabBarView(
                              controller: _paketTabs,
                              children: [_paketFormTab(), _paketListTab()],
                            )
                          : _paketListTab(),
                    ],
                  ),
          ),
        ],
      ),
    );
  }

  Widget _officerTab() => !_canWrite && !_canGuestRead
      ? _scheduleReadOnlyTab()
      : RefreshIndicator(
          onRefresh: _loadData,
          child: _officers.isEmpty
              ? ListView(
                  children: const [
                    SizedBox(height: 180),
                    Center(child: Text('Belum ada petugas keamanan.')),
                  ],
                )
              : ListView(
                  children: _officers
                      .map(
                        (o) => Card(
                          child: ListTile(
                            title: Text(o.fullName),
                            subtitle: Text(
                              'Keamanan · ${o.isActive ? 'Aktif' : 'Tidak Aktif'}',
                            ),
                            trailing: const Icon(Icons.chevron_right),
                            onTap: () => showDialog<void>(
                              context: context,
                              builder: (ctx) => AlertDialog(
                                title: Text(o.fullName),
                                content: Text(
                                  'Jabatan: Keamanan\nStatus: ${o.isActive ? 'Aktif' : 'Tidak Aktif'}${o.email.isEmpty ? '' : '\nEmail: ${o.email}'}',
                                ),
                                actions: [
                                  TextButton(
                                    onPressed: () => Navigator.pop(ctx),
                                    child: const Text('Tutup'),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                      )
                      .toList(),
                ),
        );

  Widget _scheduleReadOnlyTab() {
    return RefreshIndicator(
      onRefresh: _loadData,
      child: _schedules.isEmpty
          ? ListView(
              children: const [
                SizedBox(height: 180),
                Center(child: Text('Belum ada petugas terjadwal.')),
              ],
            )
          : ListView(
              children: _schedules
                  .map(
                    (s) => Card(
                      child: ListTile(
                        title: Text(s.officerName),
                        subtitle: Text(
                          'Keamanan · ${s.attendanceStatusLabel ?? s.attendanceStatus ?? '-'}',
                        ),
                        trailing: const Icon(Icons.chevron_right),
                        onTap: () => showDialog<void>(
                          context: context,
                          builder: (ctx) => AlertDialog(
                            title: Text(s.officerName),
                            content: Text(
                              'Penugasan: ${s.shiftName} ${s.startTime}–${s.endTime}\nStatus: ${(s.isActive && s.status == 'scheduled') ? 'Aktif' : 'Nonaktif'}',
                            ),
                            actions: [
                              TextButton(
                                onPressed: () => Navigator.pop(ctx),
                                child: const Text('Tutup'),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  )
                  .toList(),
            ),
    );
  }

  Widget _shiftTab() => RefreshIndicator(
    onRefresh: _loadData,
    child: Column(
      children: [
        Expanded(
          child: _shifts.isEmpty
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text('Belum ada shift.'),
                      if (_canWrite) ...[
                        const SizedBox(height: 12),
                        FilledButton.icon(
                          onPressed: () => _openShiftForm(),
                          icon: const Icon(Icons.add),
                          label: const Text('Tambah Shift'),
                        ),
                      ],
                    ],
                  ),
                )
              : ListView(
                  children: _shifts
                      .map(
                        (s) => Card(
                          child: ListTile(
                            title: Text(s.name),
                            subtitle: Text(
                              '${s.startTime.length >= 5 ? s.startTime.substring(0, 5) : s.startTime}–${s.endTime.length >= 5 ? s.endTime.substring(0, 5) : s.endTime} · Toleransi ${s.latenessToleranceMinutes} menit · ${s.isActive ? 'Aktif' : 'Nonaktif'}',
                            ),
                            onTap: _canWrite
                                ? () => _openShiftForm(item: s)
                                : null,
                            trailing: _canWrite
                                ? IconButton(
                                    icon: const Icon(Icons.edit),
                                    tooltip: 'Ubah Shift',
                                    onPressed: () => _openShiftForm(item: s),
                                  )
                                : null,
                          ),
                        ),
                      )
                      .toList(),
                ),
        ),
      ],
    ),
  );

  Widget _scheduleTab() => RefreshIndicator(
    onRefresh: _loadData,
    child: Column(
      children: [
        Wrap(
          spacing: 8,
          children: [
            TextButton.icon(
              onPressed: _setDateFilter,
              icon: const Icon(Icons.date_range),
              label: Text(
                _period == null
                    ? 'Periode'
                    : '${DateFormat('dd/MM').format(_period!.start)}–${DateFormat('dd/MM').format(_period!.end)}',
              ),
            ),
            DropdownButton<String?>(
              value: _officerFilter,
              hint: const Text('Semua Petugas'),
              items: [
                const DropdownMenuItem(
                  value: null,
                  child: Text('Semua Petugas'),
                ),
                ..._officers.map(
                  (o) => DropdownMenuItem(
                    value: o.userId,
                    child: Text(o.fullName),
                  ),
                ),
              ],
              onChanged: (v) {
                setState(() => _officerFilter = v);
                _loadData();
              },
            ),
          ],
        ),
        Expanded(
          child: _schedules.isEmpty
              ? const Center(child: Text('Tidak ada jadwal.'))
              : ListView(
                  children: _schedules
                      .map(
                        (s) => Card(
                          child: ListTile(
                            title: Text('${s.officerName} · ${s.shiftName}'),
                            subtitle: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text('${s.startTime}–${s.endTime} WIB'),
                                const SizedBox(height: 4),
                                Wrap(
                                  spacing: 8,
                                  runSpacing: 4,
                                  crossAxisAlignment: WrapCrossAlignment.center,
                                  children: [
                                    Container(
                                      padding: const EdgeInsets.symmetric(
                                        horizontal: 8,
                                        vertical: 2,
                                      ),
                                      decoration: BoxDecoration(
                                        color: (s.isActive && s.status == 'scheduled')
                                            ? Colors.green.withValues(alpha: 0.15)
                                            : Colors.grey.withValues(alpha: 0.15),
                                        borderRadius: BorderRadius.circular(4),
                                      ),
                                      child: Text(
                                        (s.isActive && s.status == 'scheduled')
                                            ? 'Aktif'
                                            : 'Nonaktif',
                                        style: TextStyle(
                                          color: (s.isActive && s.status == 'scheduled')
                                              ? Colors.green[800]
                                              : Colors.grey[700],
                                          fontSize: 12,
                                          fontWeight: FontWeight.w600,
                                        ),
                                      ),
                                    ),
                                    if (s.attendanceStatus != null)
                                      AttendanceStatusBadge(
                                        statusCode: s.attendanceStatus,
                                        statusLabel: s.attendanceStatusLabel,
                                      ),
                                  ],
                                ),
                              ],
                            ),
                            trailing: _canWrite
                                ? IconButton(
                                    icon: const Icon(Icons.edit),
                                    onPressed: () => _openScheduleForm(item: s),
                                  )
                                : const Icon(Icons.chevron_right),
                            onTap: () => showDialog<void>(
                              context: context,
                              builder: (ctx) => AlertDialog(
                                title: Text(s.officerName),
                                content: Text(
                                  'Periode tugas: ${s.shiftName} (${s.startTime}–${s.endTime})\nStatus: ${(s.isActive && s.status == 'scheduled') ? 'Aktif' : 'Nonaktif'}\nAbsensi: ${s.attendanceStatusLabel ?? '-'}${s.notes != null && s.notes!.isNotEmpty ? '\nCatatan: ${s.notes}' : ''}',
                                ),
                                actions: [
                                  TextButton(
                                    onPressed: () => Navigator.pop(ctx),
                                    child: const Text('Tutup'),
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                      )
                      .toList(),
                ),
        ),
      ],
    ),
  );

  Widget _attendanceTab() => Column(
    children: [
      Wrap(
        spacing: 8,
        children: [
          TextButton.icon(
            onPressed: _setDateFilter,
            icon: const Icon(Icons.date_range),
            label: const Text('Tanggal'),
          ),
          DropdownButton<String>(
            value: _attendanceStatus.isEmpty ? null : _attendanceStatus,
            hint: const Text('Semua Status'),
            items: const [
              DropdownMenuItem(value: 'hadir', child: Text('Hadir')),
              DropdownMenuItem(value: 'terlambat', child: Text('Terlambat')),
              DropdownMenuItem(
                value: 'belum_hadir',
                child: Text('Belum Hadir'),
              ),
              DropdownMenuItem(
                value: 'tidak_hadir',
                child: Text('Tidak Hadir'),
              ),
              DropdownMenuItem(
                value: 'belum_absen_pulang',
                child: Text('Belum Checkout'),
              ),
            ],
            onChanged: (v) {
              setState(() => _attendanceStatus = v ?? '');
              _loadData();
            },
          ),
          DropdownButton<String?>(
            value: _officerFilter,
            hint: const Text('Petugas'),
            items: [
              const DropdownMenuItem(value: null, child: Text('Semua Petugas')),
              ..._officers.map(
                (o) =>
                    DropdownMenuItem(value: o.userId, child: Text(o.fullName)),
              ),
            ],
            onChanged: (v) {
              setState(() => _officerFilter = v);
              _loadData();
            },
          ),
        ],
      ),
      Expanded(
        child: RefreshIndicator(
        onRefresh: () => _guardedRefresh(_reloadAttendance),
        child: _attendance.isEmpty
            ? ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: const [
                  SizedBox(height: 220),
                  Center(child: Text('Belum ada catatan absensi.')),
                ],
              )
            : ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: _attendance
                    .map(
                      (a) => Card(
                        child: ListTile(
                          title: Text(
                            '${a.officerName} · ${a.shiftName} · ${a.shiftDate}',
                          ),
                          subtitle: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              AttendanceStatusBadge(
                                statusCode: a.status,
                                statusLabel: a.statusLabel,
                              ),
                              const SizedBox(height: 4),
                              Text(
                                'Masuk: ${a.checkedInAt == null ? '-' : DateFormat('dd/MM HH:mm').format(a.checkedInAt!)} · Keluar: ${a.checkedOutAt == null ? '-' : DateFormat('dd/MM HH:mm').format(a.checkedOutAt!)}',
                              ),
                            ],
                          ),
                          onTap: () => showDialog<void>(
                            context: context,
                            builder: (ctx) => AlertDialog(
                              title: Text(a.officerName),
                              content: Text(
                                'Status: ${a.statusLabel}\nWaktu server masuk: ${a.checkedInAt ?? '-'}\nWaktu server keluar: ${a.checkedOutAt ?? '-'}\nPerangkat masuk: ${a.checkInDeviceName ?? '-'}\nPerangkat keluar: ${a.checkOutDeviceName ?? '-'}\nKoreksi terkunci: ${a.correctionLocked ? 'Ya' : 'Tidak'}',
                              ),
                              actions: [
                                TextButton(
                                  onPressed: () => Navigator.pop(ctx),
                                  child: const Text('Tutup'),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    )
                    .toList(),
              ),
        ),
      ),
    ],
  );

  Widget _statusBadge(String status) {
    Color bg;
    Color fg;
    String label;
    switch (status.toLowerCase().trim()) {
      case 'hadir':
        bg = AppColors.success.withValues(alpha: 0.15);
        fg = AppColors.success;
        label = 'Hadir';
        break;
      case 'belum_hadir':
        bg = AppColors.danger.withValues(alpha: 0.15);
        fg = AppColors.danger;
        label = 'Belum Hadir';
        break;
      case 'izin':
        bg = Colors.blue.withValues(alpha: 0.15);
        fg = Colors.blue;
        label = 'Izin';
        break;
      case 'sakit':
        bg = AppColors.warning.withValues(alpha: 0.15);
        fg = AppColors.warning;
        label = 'Sakit';
        break;
      case 'alpa':
        bg = AppColors.danger.withValues(alpha: 0.15);
        fg = AppColors.danger;
        label = 'Alpa';
        break;
      default:
        bg = Colors.grey.withValues(alpha: 0.15);
        fg = Colors.grey;
        label = status;
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: fg,
          fontSize: 11,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }

  Widget _decisionBadge(String status) {
    if (status == 'pending') {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
        decoration: BoxDecoration(
          color: AppColors.warning.withValues(alpha: 0.15),
          borderRadius: BorderRadius.circular(4),
        ),
        child: const Text(
          'Pending',
          style: TextStyle(
            color: AppColors.warning,
            fontSize: 10,
            fontWeight: FontWeight.w700,
          ),
        ),
      );
    }
    final isApp = status == 'approved';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: (isApp ? AppColors.success : AppColors.danger).withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        isApp ? 'Disetujui' : 'Ditolak',
        style: TextStyle(
          color: isApp ? AppColors.success : AppColors.danger,
          fontSize: 10,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }

  Widget _buildCorrectionCard(AttendanceCorrection c) {
    final theme = Theme.of(context);
    final canDecideThis = _canDecide &&
        c.status == 'pending' &&
        c.requestedBy != (_user?['id']?.toString() ?? '');

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      elevation: 1,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header: Petugas, Tanggal, Status (includes exact string for tests)
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '${c.officerName} · ${c.shiftDate} · ${c.status}',
                        style: theme.textTheme.titleSmall?.copyWith(
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      if (c.shiftName.isNotEmpty) ...[
                        const SizedBox(height: 2),
                        Text(
                          'Shift: ${c.shiftName}',
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                            fontSize: 11,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                if (canDecideThis)
                  PopupMenuButton<bool>(
                    icon: const Icon(Icons.more_vert, size: 20),
                    onSelected: (approve) => _decideCorrection(c, approve),
                    itemBuilder: (_) => const [
                      PopupMenuItem(
                        value: true,
                        child: Row(
                          children: [
                            Icon(Icons.check, color: AppColors.success, size: 18),
                            SizedBox(width: 8),
                            Text('Setujui'),
                          ],
                        ),
                      ),
                      PopupMenuItem(
                        value: false,
                        child: Row(
                          children: [
                            Icon(Icons.close, color: AppColors.danger, size: 18),
                            SizedBox(width: 8),
                            Text('Tolak'),
                          ],
                        ),
                      ),
                    ],
                  ),
              ],
            ),
            const SizedBox(height: 8),

            // Status change formatted
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
              decoration: BoxDecoration(
                color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.3),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Row(
                children: [
                  Text(
                    c.requestType == 'status_change'
                        ? 'Perubahan Status: '
                        : '${c.requestType}: ',
                    style: const TextStyle(
                      fontWeight: FontWeight.w600,
                      fontSize: 12,
                    ),
                  ),
                  _statusBadge(c.originalStatus),
                  const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 6),
                    child: Icon(Icons.arrow_forward, size: 14),
                  ),
                  _statusBadge(c.requestedStatus),
                ],
              ),
            ),
            const SizedBox(height: 10),

            // Blok Diajukan
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.25),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: theme.colorScheme.outlineVariant.withValues(alpha: 0.5),
                ),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const Icon(Icons.send_outlined, size: 14, color: AppColors.seed),
                      const SizedBox(width: 6),
                      Text(
                        'Diajukan',
                        style: TextStyle(
                          fontWeight: FontWeight.w700,
                          fontSize: 11.5,
                          color: AppColors.seed,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Alasan: ${c.reason}',
                    style: const TextStyle(fontSize: 12),
                  ),
                  if (c.createdAt != null) ...[
                    const SizedBox(height: 2),
                    Text(
                      'Waktu: ${DateFormat('dd/MM/yyyy HH:mm').format(c.createdAt!)}',
                      style: TextStyle(
                        fontSize: 11,
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 8),

            // Blok Keputusan
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: c.status == 'pending'
                    ? AppColors.warning.withValues(alpha: 0.08)
                    : (c.status == 'approved'
                        ? AppColors.success.withValues(alpha: 0.08)
                        : AppColors.danger.withValues(alpha: 0.08)),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: c.status == 'pending'
                      ? AppColors.warning.withValues(alpha: 0.3)
                      : (c.status == 'approved'
                          ? AppColors.success.withValues(alpha: 0.3)
                          : AppColors.danger.withValues(alpha: 0.3)),
                ),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        c.status == 'pending'
                            ? Icons.hourglass_empty
                            : (c.status == 'approved'
                                ? Icons.check_circle_outline
                                : Icons.cancel_outlined),
                        size: 14,
                        color: c.status == 'pending'
                            ? AppColors.warning
                            : (c.status == 'approved'
                                ? AppColors.success
                                : AppColors.danger),
                      ),
                      const SizedBox(width: 6),
                      Text(
                        'Keputusan',
                        style: TextStyle(
                          fontWeight: FontWeight.w700,
                          fontSize: 11.5,
                          color: c.status == 'pending'
                              ? AppColors.warning
                              : (c.status == 'approved'
                                  ? AppColors.success
                                  : AppColors.danger),
                        ),
                      ),
                      const Spacer(),
                      _decisionBadge(c.status),
                    ],
                  ),
                  const SizedBox(height: 4),
                  if (c.status == 'pending')
                    Row(
                      children: [
                        Icon(
                          Icons.schedule,
                          size: 14,
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          'Menunggu Konfirmasi',
                          style: TextStyle(
                            fontSize: 12,
                            fontStyle: FontStyle.italic,
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    )
                  else ...[
                    if (c.decidedByName != null || c.decidedBy != null) ...[
                      Text(
                        'Peninjau: ${c.decidedByName ?? c.decidedBy}',
                        style: const TextStyle(fontSize: 12),
                      ),
                    ],
                    if (c.decisionNote != null && c.decisionNote!.isNotEmpty) ...[
                      const SizedBox(height: 2),
                      Text(
                        'Catatan: ${c.decisionNote}',
                        style: const TextStyle(fontSize: 12),
                      ),
                    ],
                    if (c.decidedAt != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        'Diputuskan: ${DateFormat('dd/MM/yyyy HH:mm').format(c.decidedAt!)}',
                        style: TextStyle(
                          fontSize: 11,
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _correctionTab() => RefreshIndicator(
    onRefresh: () => _guardedRefresh(_reloadCorrections),
    child: _corrections.isEmpty
      ? ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          children: const [
            SizedBox(height: 220),
            Center(child: Text('Tidak ada pengajuan koreksi.')),
          ],
        )
      : ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          children: _corrections.map(_buildCorrectionCard).toList(),
        ),
  );

  Widget _patrolFormTab() {
    final theme = Theme.of(context);
    return RefreshIndicator(
      onRefresh: _reloadPatrolTemplates,
      child: ListView(
        padding: const EdgeInsets.all(12),
        children: [
          Card(
            color: AppColors.accent.withValues(alpha: 0.08),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Row(
                children: [
                  const Icon(Icons.info_outline, color: AppColors.accent),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'Ambil foto di setiap titik kontrol patroli berikut untuk memastikan keamanan wilayah RT.',
                      style: theme.textTheme.bodySmall?.copyWith(fontSize: 12),
                    ),
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(height: 10),
          if (_patrolTemplates.isEmpty) ...[
            const SizedBox(height: 40),
            const Center(child: Text('Tidak ada template titik kontrol aktif.')),
            const SizedBox(height: 12),
            Center(
              child: OutlinedButton.icon(
                icon: const Icon(Icons.refresh),
                label: const Text('Muat Ulang'),
                onPressed: _reloadPatrolTemplates,
              ),
            ),
          ] else ...[
            ..._patrolTemplates.map((t) {
              final photo = _patrolPhotos[t.id];
              return Card(
                margin: const EdgeInsets.only(bottom: 10),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          const Icon(Icons.location_on_outlined, size: 20, color: AppColors.seed),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(t.name, style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14)),
                                if (t.locationLabel.isNotEmpty)
                                  Text(
                                    t.locationLabel,
                                    style: TextStyle(color: theme.colorScheme.onSurfaceVariant, fontSize: 12),
                                  ),
                              ],
                            ),
                          ),
                          if (photo != null)
                            const Icon(Icons.check_circle, color: AppColors.success, size: 22),
                        ],
                      ),
                      const SizedBox(height: 10),
                      if (photo != null) ...[
                        Row(
                          children: [
                            ClipRRect(
                              borderRadius: BorderRadius.circular(8),
                              child: kIsWeb
                                  ? Image.network(photo.path, width: 80, height: 80, fit: BoxFit.cover)
                                  : Image.file(File(photo.path), width: 80, height: 80, fit: BoxFit.cover),
                            ),
                            const SizedBox(width: 12),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  const Text(
                                    'Foto terlampir',
                                    style: TextStyle(
                                      fontWeight: FontWeight.w600,
                                      fontSize: 12,
                                      color: AppColors.success,
                                    ),
                                  ),
                                  const SizedBox(height: 4),
                                  Row(
                                    children: [
                                      TextButton.icon(
                                        icon: const Icon(Icons.refresh, size: 16),
                                        label: const Text('Ganti', style: TextStyle(fontSize: 12)),
                                        onPressed: () => _takePatrolPhoto(t.id),
                                      ),
                                      TextButton.icon(
                                        icon: const Icon(Icons.delete_outline, size: 16, color: AppColors.danger),
                                        label: const Text('Hapus', style: TextStyle(fontSize: 12, color: AppColors.danger)),
                                        onPressed: () => setState(() => _patrolPhotos.remove(t.id)),
                                      ),
                                    ],
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ] else ...[
                        FilledButton.tonalIcon(
                          icon: const Icon(Icons.camera_alt, size: 18),
                          label: const Text('Ambil Foto Kontrol'),
                          onPressed: () => _takePatrolPhoto(t.id),
                        ),
                      ],
                    ],
                  ),
                ),
              );
            }),
            const SizedBox(height: 10),
            TextField(
              controller: _patrolNotesCtrl,
              maxLines: 2,
              decoration: const InputDecoration(
                labelText: 'Catatan Patroli (Opsional)',
                hintText: 'Cth: Kondisi gang aman, lampu jalan menyala...',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 14),
            FilledButton.icon(
              icon: _submittingPatrol
                  ? const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                    )
                  : const Icon(Icons.send),
              label: const Text('Kirim Laporan Patroli'),
              onPressed: _submittingPatrol ? null : _submitPatrol,
            ),
            const SizedBox(height: 24),
          ],
        ],
      ),
    );
  }

  Widget _patrolHistoryTab() {
    final theme = Theme.of(context);
    return RefreshIndicator(
      onRefresh: _reloadPatrolBundles,
      child: _patrolBundles.isEmpty
          ? ListView(
              children: const [
                SizedBox(height: 180),
                Center(child: Text('Belum ada riwayat patroli.')),
              ],
            )
          : ListView.builder(
              padding: const EdgeInsets.all(12),
              itemCount: _patrolBundles.length,
              itemBuilder: (ctx, idx) {
                final b = _patrolBundles[idx];
                final officer =
                    b.dutyOfficerName.isNotEmpty ? b.dutyOfficerName : b.createdByName;
                return Card(
                  margin: const EdgeInsets.only(bottom: 8),
                  child: ListTile(
                    leading: const CircleAvatar(
                      backgroundColor: AppColors.hoverTint,
                      child: Icon(Icons.local_police, color: AppColors.seed),
                    ),
                    title: Text(officer, style: const TextStyle(fontWeight: FontWeight.bold)),
                    subtitle: Text(
                      '${DateFormat('dd MMM yyyy, HH:mm').format(b.submittedAt)} · ${b.photoCount} Foto Titik Kontrol\nCatatan: ${b.notes ?? '-'}',
                    ),
                    isThreeLine: true,
                    trailing: const Icon(Icons.arrow_forward_ios, size: 14),
                    onTap: () => _viewPatrolPhotos(b),
                  ),
                );
              },
            ),
    );
  }

  Widget _paketFormTab() {
    final theme = Theme.of(context);
    return ListView(
      padding: const EdgeInsets.all(12),
      children: [
        // Section: Warga
        Card(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Penerima Paket (Warga RT)',
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                ),
                const SizedBox(height: 8),
                if (_selectedWarga != null) ...[
                  Container(
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: AppColors.accent.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: AppColors.accent.withValues(alpha: 0.3)),
                    ),
                    child: Row(
                      children: [
                        const Icon(Icons.person, color: AppColors.accent),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                _selectedWarga!.fullName,
                                style: const TextStyle(fontWeight: FontWeight.bold),
                              ),
                              Text(
                                'No. Rumah: ${_selectedWarga!.houseNumber}',
                                style: const TextStyle(fontSize: 12),
                              ),
                            ],
                          ),
                        ),
                        IconButton(
                          icon: const Icon(Icons.close),
                          onPressed: () => setState(() => _selectedWarga = null),
                        ),
                      ],
                    ),
                  ),
                ] else ...[
                  TextField(
                    controller: _wargaSearchCtrl,
                    onChanged: _searchWarga,
                    decoration: InputDecoration(
                      labelText: 'Cari Nama Warga atau No. Rumah',
                      hintText: 'Ketik minimal 1 huruf...',
                      prefixIcon: const Icon(Icons.search),
                      suffixIcon: _searchingWarga
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: Padding(
                                padding: EdgeInsets.all(10),
                                child: CircularProgressIndicator(strokeWidth: 2),
                              ),
                            )
                          : null,
                      border: const OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                  if (_wargaSearchCtrl.text.trim().isNotEmpty &&
                      !_searchingWarga &&
                      _wargaResults.isEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        'Tidak ada warga ditemukan dengan kata kunci "${_wargaSearchCtrl.text.trim()}".',
                        style: TextStyle(
                          color: theme.colorScheme.onSurfaceVariant,
                          fontSize: 12,
                          fontStyle: FontStyle.italic,
                        ),
                      ),
                    ),
                  if (_wargaResults.isNotEmpty) ...[
                    const SizedBox(height: 6),
                    Container(
                      constraints: const BoxConstraints(maxHeight: 180),
                      decoration: BoxDecoration(
                        border: Border.all(color: Colors.grey.shade300),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: ListView.builder(
                        shrinkWrap: true,
                        itemCount: _wargaResults.length,
                        itemBuilder: (ctx, idx) {
                          final w = _wargaResults[idx];
                          return ListTile(
                            dense: true,
                            title: Text(w.fullName, style: const TextStyle(fontWeight: FontWeight.w600)),
                            subtitle: Text('Rumah: ${w.houseNumber}'),
                            onTap: () {
                              setState(() {
                                _selectedWarga = w;
                                _wargaResults.clear();
                                _wargaSearchCtrl.clear();
                              });
                            },
                          );
                        },
                      ),
                    ),
                  ],
                ],
              ],
            ),
          ),
        ),
        const SizedBox(height: 12),

        // Section: Items
        ..._paketDraftItems.asMap().entries.map((entry) {
          final idx = entry.key;
          final item = entry.value;
          return Card(
            margin: const EdgeInsets.only(bottom: 10),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('Paket #${idx + 1}', style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13)),
                      if (_paketDraftItems.length > 1)
                        IconButton(
                          icon: const Icon(Icons.delete_outline, color: AppColors.danger, size: 20),
                          onPressed: () => _removePaketDraftItem(idx),
                        ),
                    ],
                  ),
                  const SizedBox(height: 6),
                  TextField(
                    controller: item.courierCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Ekspedisi / Kurir',
                      hintText: 'Cth: JNE, J&T, SiCepat, Shopee...',
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                  const SizedBox(height: 8),
                  TextField(
                    controller: item.resiCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Nomor Resi (Opsional)',
                      hintText: 'Cth: JP1234567890',
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                  const SizedBox(height: 8),
                  TextField(
                    controller: item.descCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Deskripsi Paket',
                      hintText: 'Cth: Kardus coklat sedang / Plastik hitam',
                      border: OutlineInputBorder(),
                      isDense: true,
                    ),
                  ),
                  const SizedBox(height: 8),
                  if (item.photo != null) ...[
                    Row(
                      children: [
                        ClipRRect(
                          borderRadius: BorderRadius.circular(6),
                          child: kIsWeb
                              ? Image.network(item.photo!.path, width: 60, height: 60, fit: BoxFit.cover)
                              : Image.file(File(item.photo!.path), width: 60, height: 60, fit: BoxFit.cover),
                        ),
                        const SizedBox(width: 10),
                        const Expanded(
                          child: Text(
                            'Foto paket terlampir',
                            style: TextStyle(color: AppColors.success, fontSize: 12),
                          ),
                        ),
                        IconButton(
                          icon: const Icon(Icons.close, color: Colors.red),
                          onPressed: () => setState(() => item.photo = null),
                        ),
                      ],
                    ),
                  ] else ...[
                    OutlinedButton.icon(
                      icon: const Icon(Icons.camera_alt, size: 16),
                      label: const Text('Foto Paket'),
                      onPressed: () => _pickPaketItemPhoto(item),
                    ),
                  ],
                ],
              ),
            ),
          );
        }),

        OutlinedButton.icon(
          icon: const Icon(Icons.add),
          label: const Text('Tambah Paket Lain (Multi-Item)'),
          onPressed: _addPaketDraftItem,
        ),
        const SizedBox(height: 14),

        FilledButton.icon(
          icon: _submittingPaket
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : const Icon(Icons.save),
          label: const Text('Simpan Penitipan Paket'),
          onPressed: _submittingPaket ? null : _submitPaket,
        ),
        const SizedBox(height: 24),
      ],
    );
  }

  Widget _paketListTab() {
    final theme = Theme.of(context);
    final filtered = _paketReports.where((r) {
      if (_paketFilterStatus.isEmpty) return true;
      if (_paketFilterStatus == 'dititipkan') return r.overallStatus == 'dititipkan';
      if (_paketFilterStatus == 'sudah_diambil') return r.overallStatus == 'sudah_diambil';
      return true;
    }).toList();

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            children: [
              ChoiceChip(
                label: const Text('Semua'),
                selected: _paketFilterStatus == '',
                onSelected: (_) => setState(() => _paketFilterStatus = ''),
              ),
              const SizedBox(width: 8),
              ChoiceChip(
                label: const Text('Dititipkan'),
                selected: _paketFilterStatus == 'dititipkan',
                onSelected: (_) => setState(() => _paketFilterStatus = 'dititipkan'),
              ),
              const SizedBox(width: 8),
              ChoiceChip(
                label: const Text('Sudah Diambil'),
                selected: _paketFilterStatus == 'sudah_diambil',
                onSelected: (_) => setState(() => _paketFilterStatus = 'sudah_diambil'),
              ),
            ],
          ),
        ),
        Expanded(
          child: RefreshIndicator(
            onRefresh: _reloadPaketReports,
            child: filtered.isEmpty
                ? ListView(
                    children: const [
                      SizedBox(height: 180),
                      Center(child: Text('Tidak ada paket.')),
                    ],
                  )
                : ListView.builder(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                    itemCount: filtered.length,
                    itemBuilder: (ctx, idx) {
                      final r = filtered[idx];
                      return Card(
                        margin: const EdgeInsets.only(bottom: 10),
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  const Icon(Icons.inventory_2, color: AppColors.seed, size: 22),
                                  const SizedBox(width: 8),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          '${r.receiverName} (${r.receiverHouseNumber})',
                                          style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                                        ),
                                        Text(
                                          'Diterima: ${DateFormat('dd MMM yyyy, HH:mm').format(r.createdAt)} · Petugas: ${r.createdByName}',
                                          style: TextStyle(
                                            color: theme.colorScheme.onSurfaceVariant,
                                            fontSize: 11,
                                          ),
                                        ),
                                      ],
                                    ),
                                  ),
                                ],
                              ),
                              const Divider(height: 16),
                              ...r.items.map((it) {
                                final isDititipkan = it.status == 'dititipkan';
                                final hours = DateTime.now().difference(it.createdAt).inHours;
                                final isOverdue = isDititipkan && hours >= 24;

                                return Container(
                                  margin: const EdgeInsets.only(bottom: 8),
                                  padding: const EdgeInsets.all(8),
                                  decoration: BoxDecoration(
                                    color: theme.colorScheme.surfaceContainerHighest.withValues(alpha: 0.25),
                                    borderRadius: BorderRadius.circular(6),
                                  ),
                                  child: Column(
                                    crossAxisAlignment: CrossAxisAlignment.start,
                                    children: [
                                      Row(
                                        children: [
                                          Expanded(
                                            child: Text(
                                              it.description ?? 'Paket',
                                              style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                                            ),
                                          ),
                                          if (isOverdue)
                                            Container(
                                              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                                              decoration: BoxDecoration(
                                                color: AppColors.danger.withValues(alpha: 0.15),
                                                borderRadius: BorderRadius.circular(4),
                                              ),
                                              child: Text(
                                                '> 24 Jam ($hours jam)',
                                                style: const TextStyle(
                                                  color: AppColors.danger,
                                                  fontSize: 10,
                                                  fontWeight: FontWeight.bold,
                                                ),
                                              ),
                                            )
                                          else if (isDititipkan)
                                            Container(
                                              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                                              decoration: BoxDecoration(
                                                color: AppColors.warning.withValues(alpha: 0.15),
                                                borderRadius: BorderRadius.circular(4),
                                              ),
                                              child: const Text(
                                                'Dititipkan',
                                                style: TextStyle(
                                                  color: AppColors.warning,
                                                  fontSize: 10,
                                                  fontWeight: FontWeight.bold,
                                                ),
                                              ),
                                            )
                                          else
                                            Container(
                                              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                                              decoration: BoxDecoration(
                                                color: AppColors.success.withValues(alpha: 0.15),
                                                borderRadius: BorderRadius.circular(4),
                                              ),
                                              child: const Text(
                                                'Sudah Diambil',
                                                style: TextStyle(
                                                  color: AppColors.success,
                                                  fontSize: 10,
                                                  fontWeight: FontWeight.bold,
                                                ),
                                              ),
                                            ),
                                        ],
                                      ),
                                      const SizedBox(height: 6),
                                      Row(
                                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                        children: [
                                          if (it.photos.isNotEmpty)
                                            TextButton.icon(
                                              icon: const Icon(Icons.photo_outlined, size: 16),
                                              label: Text('Foto (${it.photos.length})', style: const TextStyle(fontSize: 11)),
                                              onPressed: () => _viewPaketPhotos(it),
                                            )
                                          else
                                            const SizedBox.shrink(),
                                          if (isDititipkan && _canPaketStatusChange)
                                            FilledButton.tonalIcon(
                                              icon: const Icon(Icons.assignment_turned_in, size: 16),
                                              label: const Text('Serah Terima', style: TextStyle(fontSize: 12)),
                                              onPressed: () => _openSerahTerimaDialog(r, it),
                                            ),
                                          if (!isDititipkan && it.statusChangedAt != null)
                                            Text(
                                              'Diambil: ${DateFormat('dd/MM HH:mm').format(it.statusChangedAt!)}',
                                              style: TextStyle(
                                                color: theme.colorScheme.onSurfaceVariant,
                                                fontSize: 11,
                                              ),
                                            ),
                                        ],
                                      ),
                                    ],
                                  ),
                                );
                              }),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
          ),
        ),
      ],
    );
  }

  Widget _guestTab() => !_canGuestRead
      ? const Center(child: Text('Tidak memiliki izin membaca Buku Tamu.'))
      : Column(
          children: [
            Padding(
              padding: const EdgeInsets.all(12),
              child: TextField(
                onChanged: _debounceGuestSearch,
                decoration: InputDecoration(
                  hintText: 'Cari nama atau tujuan kunjungan...',
                  prefixIcon: const Icon(Icons.search),
                  suffixIcon: IconButton(
                    onPressed: () => _loadGuestSearch(''),
                    icon: const Icon(Icons.clear),
                  ),
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
              ),
            ),
            Expanded(
              child: RefreshIndicator(
                onRefresh: () => _guardedRefresh(_reloadGuests),
                child: _loading
                    ? ListView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        children: const [
                          SizedBox(height: 220),
                          Center(child: CircularProgressIndicator()),
                        ],
                      )
                    : _error != null
                    ? ListView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        children: [
                          const SizedBox(height: 220),
                          Center(
                            child: Text(_error!, textAlign: TextAlign.center),
                          ),
                        ],
                      )
                    : _guests.isEmpty
                    ? ListView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        children: const [
                          SizedBox(height: 220),
                          Center(child: Text('Tidak ada catatan Buku Tamu.')),
                        ],
                      )
                    : ListView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        children: _guests
                            .map(
                              (g) => Card(
                              child: ListTile(
                                title: Text(
                                  '${g.guestName} → ${g.destinationLabel}',
                                ),
                                subtitle: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    GuestVisitStatusBadge(
                                      checkedOutAt: g.checkedOutAt,
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      '${g.purpose}\nMasuk: ${DateFormat('dd/MM HH:mm').format(g.checkedInAt.toLocal())} · ${g.checkedOutAt == null ? 'Belum keluar' : 'Keluar: ${DateFormat('dd/MM HH:mm').format(g.checkedOutAt!.toLocal())}'}\n${_guestDurationLabel(g)}',
                                    ),
                                  ],
                                ),
                                isThreeLine: true,
                                trailing: const Icon(Icons.chevron_right),
                                onTap: () => _showGuestDetail(g),
                              ),
                            ),
                          )
                          .toList(),
                    ),
              ),
            ),
            if (_guestTotal > 25)
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  IconButton(
                    onPressed: _guestPage > 1
                        ? () => _loadGuestPage(_guestPage - 1)
                        : null,
                    icon: const Icon(Icons.chevron_left),
                  ),
                  Text('Halaman $_guestPage · $_guestTotal catatan'),
                  IconButton(
                    onPressed: _guestPage * 25 < _guestTotal
                        ? () => _loadGuestPage(_guestPage + 1)
                        : null,
                    icon: const Icon(Icons.chevron_right),
                  ),
                ],
              ),
          ],
        );

  Widget _deviceTab() => RefreshIndicator(
    onRefresh: () => _guardedRefresh(_reloadDevices),
    child: _devices.isEmpty
        ? ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            children: const [
              SizedBox(height: 220),
              Center(child: Text('Belum ada perangkat pos.')),
            ],
          )
        : ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            children: _devices
              .map(
                (d) => Card(
                  child: ListTile(
                    title: Text(d.deviceName),
                    subtitle: Text(
                      '${d.locationLabel} · ${d.isActive ? 'Aktif' : 'Nonaktif'} · ${d.isPaired ? 'Terpasang' : 'Belum dipasangkan'}',
                    ),
                    trailing: _canManageDevices && d.isActive
                        ? Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              IconButton(
                                icon: const Icon(Icons.vpn_key_outlined),
                                color: AppColors.accent,
                                tooltip: 'Token Baru',
                                onPressed: () => _rotatePairingToken(d),
                              ),
                              IconButton(
                                icon: const Icon(Icons.power_settings_new),
                                tooltip: 'Cabut',
                                onPressed: () => _revokeDevice(d),
                              ),
                            ],
                          )
                        : null,
                  ),
                ),
              )
              .toList(),
        ),
  );

  void _ensureGuestTicker() {
    final hasActive = _guests.any((g) => g.checkedOutAt == null);
    if (hasActive && _guestTicker == null) {
      _guestTicker = Timer.periodic(const Duration(seconds: 30), (_) {
        if (!mounted) return;
        setState(() {});
      });
    } else if (!hasActive) {
      _guestTicker?.cancel();
      _guestTicker = null;
    }
  }

  String _guestDurationLabel(GuestBookEntry g) {
    return guest_duration.guestVisitDurationLabel(
      g.checkedInAt,
      g.checkedOutAt,
    );
  }
}

class _ShiftFormDialog extends StatefulWidget {
  final ShiftConfig? item;
  final SecurityApiClient api;
  final TimeOfDay initialStartTime;
  final TimeOfDay initialEndTime;

  const _ShiftFormDialog({
    required this.item,
    required this.api,
    required this.initialStartTime,
    required this.initialEndTime,
  });

  @override
  State<_ShiftFormDialog> createState() => _ShiftFormDialogState();
}

class _ShiftFormDialogState extends State<_ShiftFormDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameController;
  late final TextEditingController _toleranceController;
  late TimeOfDay _startTime;
  late TimeOfDay _endTime;
  late bool _crosses;
  late bool _isActive;
  bool _isSubmitting = false;
  String? _formError;

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.item?.name ?? '');
    _toleranceController = TextEditingController(
      text: '${widget.item?.latenessToleranceMinutes ?? 15}',
    );
    _startTime = widget.initialStartTime;
    _endTime = widget.initialEndTime;
    _crosses = widget.item?.crossesMidnight ?? false;
    _isActive = widget.item?.isActive ?? true;
  }

  @override
  void dispose() {
    _nameController.dispose();
    _toleranceController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) {
      return;
    }
    if (_startTime.hour == _endTime.hour &&
        _startTime.minute == _endTime.minute) {
      setState(() {
        _formError = 'Jam mulai dan jam selesai tidak boleh sama';
      });
      return;
    }
    setState(() {
      _isSubmitting = true;
      _formError = null;
    });

    final payload = {
      'name': _nameController.text.trim(),
      'start_time':
          '${_startTime.hour.toString().padLeft(2, '0')}:${_startTime.minute.toString().padLeft(2, '0')}',
      'end_time':
          '${_endTime.hour.toString().padLeft(2, '0')}:${_endTime.minute.toString().padLeft(2, '0')}',
      'crosses_midnight': _crosses,
      'lateness_tolerance_minutes':
          int.tryParse(_toleranceController.text.trim()) ?? 15,
      'check_in_window_before_minutes': 60,
      'check_in_window_after_minutes': 120,
      'absent_cutoff_minutes': 120,
      'missing_checkout_cutoff_minutes': 180,
      'is_active': _isActive,
    };

    try {
      if (widget.item == null) {
        await widget.api.createManagerShift(payload);
      } else {
        await widget.api.updateManagerShift(widget.item!.id, payload);
      }
      if (mounted) {
        Navigator.pop(context, _nameController.text.trim());
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSubmitting = false;
          _formError = e.toString().replaceFirst(RegExp(r'^Exception:\s*'), '');
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.item == null ? 'Tambah Shift' : 'Ubah Shift'),
      content: SingleChildScrollView(
        child: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (_formError != null) ...[
                Container(
                  padding: const EdgeInsets.all(10),
                  margin: const EdgeInsets.only(bottom: 12),
                  decoration: BoxDecoration(
                    color: Theme.of(context).colorScheme.errorContainer,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.error_outline,
                        size: 18,
                        color: Theme.of(context).colorScheme.error,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text(
                          _formError!,
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ],
              TextFormField(
                controller: _nameController,
                enabled: !_isSubmitting,
                decoration: const InputDecoration(
                  labelText: 'Nama shift *',
                  hintText: 'Contoh: Shift Pagi',
                ),
                validator: (v) {
                  final trimmed = v?.trim() ?? '';
                  if (trimmed.isEmpty) {
                    return 'Nama shift wajib diisi';
                  }
                  if (trimmed.length < 2 || trimmed.length > 80) {
                    return 'Nama shift harus antara 2 dan 80 karakter';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 8),
              ListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Jam mulai *'),
                subtitle: Text(
                  '${_startTime.hour.toString().padLeft(2, '0')}:${_startTime.minute.toString().padLeft(2, '0')}',
                ),
                trailing: const Icon(
                  Icons.access_time,
                  color: AppColors.accent,
                ),
                onTap: _isSubmitting
                    ? null
                    : () async {
                        final picked = await showTimePicker(
                          context: context,
                          initialTime: _startTime,
                        );
                        if (picked != null && mounted) {
                          setState(() {
                            _startTime = picked;
                            _formError = null;
                          });
                        }
                      },
              ),
              ListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Jam selesai *'),
                subtitle: Text(
                  '${_endTime.hour.toString().padLeft(2, '0')}:${_endTime.minute.toString().padLeft(2, '0')}',
                ),
                trailing: const Icon(
                  Icons.access_time,
                  color: AppColors.accent,
                ),
                onTap: _isSubmitting
                    ? null
                    : () async {
                        final picked = await showTimePicker(
                          context: context,
                          initialTime: _endTime,
                        );
                        if (picked != null && mounted) {
                          setState(() {
                            _endTime = picked;
                            _formError = null;
                          });
                        }
                      },
              ),
              TextFormField(
                controller: _toleranceController,
                enabled: !_isSubmitting,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Toleransi keterlambatan (menit) *',
                ),
                validator: (v) {
                  final val = int.tryParse(v?.trim() ?? '');
                  if (val == null || val < 0 || val > 180) {
                    return 'Toleransi keterlambatan harus antara 0 dan 180 menit';
                  }
                  return null;
                },
              ),
              const SizedBox(height: 8),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: _crosses,
                title: const Text('Melewati tengah malam'),
                onChanged: _isSubmitting
                    ? null
                    : (v) => setState(() => _crosses = v),
              ),
              if (widget.item != null)
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  value: _isActive,
                  title: const Text('Status aktif'),
                  onChanged: _isSubmitting
                      ? null
                      : (v) => setState(() => _isActive = v),
                ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSubmitting ? null : () => Navigator.pop(context, null),
          child: const Text('Batal'),
        ),
        FilledButton(
          onPressed: _isSubmitting ? null : _submit,
          child: _isSubmitting
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Simpan'),
        ),
      ],
    );
  }
}
