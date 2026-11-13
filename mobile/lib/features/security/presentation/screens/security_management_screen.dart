import 'dart:async';

import 'package:flutter/material.dart';

import '../utils/guest_visit_duration.dart' as guest_duration;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/core/widgets/accent_action_fab.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import '../../data/security_models.dart';
import '../providers/security_providers.dart';
import '../utils/attendance_order.dart';
import '../widgets/attendance_status_badge.dart';

class SecurityManagementScreen extends ConsumerStatefulWidget {
  const SecurityManagementScreen({super.key});

  @override
  ConsumerState<SecurityManagementScreen> createState() =>
      _SecurityManagementScreenState();
}

class _SecurityManagementScreenState
    extends ConsumerState<SecurityManagementScreen>
    with TickerProviderStateMixin, WidgetsBindingObserver {
  late final TabController _topTabs;
  late final TabController _kelolaTabs;
  late final TabController _absensiTabs;
  late final TabController _perangkatTabs;
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

  Map<String, dynamic>? get _user =>
      ref.read(authRepositoryProvider)?.valueOrNull;
  bool get _canWrite => userHasPermission(_user, 'security.schedule.write');
  bool get _canManageDevices =>
      userHasPermission(_user, 'security.device.manage');
  bool get _canDecide => userHasPermission(_user, 'security.correction');
  bool get _canGuestRead => userHasPermission(_user, 'security.guest_book');

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _topTabs = TabController(length: 3, vsync: this);
    _kelolaTabs = TabController(length: 3, vsync: this);
    _absensiTabs = TabController(length: 2, vsync: this);
    _perangkatTabs = TabController(length: 2, vsync: this);
    for (final ctrl in [_topTabs, _kelolaTabs, _absensiTabs, _perangkatTabs]) {
      ctrl.addListener(() {
        if (!ctrl.indexIsChanging && mounted) {
          _loadData();
        }
      });
    }
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadData());
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
    _perangkatTabs.dispose();
    _guestDebounce?.cancel();
    super.dispose();
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
      setState(() {
        _guests = result.data;
        _guestTotal = result.total;
        _loading = false;
      });
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
      attendance.sort(compareAttendanceRecency);
      if (!mounted) {
        return;
      }
      setState(() {
        _officers = officers;
        _shifts = shifts;
        _schedules = schedules;
        _attendance = attendance;
        _corrections = corrections;
        _devices = devices;
        _guests = guestPage.data;
        _guestTotal = guestPage.total;
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
      setState(() {
        _guests = result.data;
        _guestTotal = result.total;
        _loading = false;
      });
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
    // Perangkat group → Perangkat only
    if (_topTabs.index == 2 && _perangkatTabs.index == 0 && _canManageDevices) {
      return AccentActionFab(
        tooltip: 'Tambah Perangkat',
        icon: Icons.tablet_android,
        onPressed: _openCreateDeviceForm,
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
    DateTime start =
        DateTime.tryParse(item?.assignmentStartDate ?? '') ?? DateTime.now();
    DateTime end = DateTime.tryParse(item?.assignmentEndDate ?? '') ?? start;
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
                  ListTile(
                    title: const Text('Tanggal mulai'),
                    subtitle: Text(DateFormat('yyyy-MM-dd').format(start)),
                    onTap: () async {
                      final d = await showDatePicker(
                        context: ctx,
                        firstDate: DateTime(2020),
                        lastDate: DateTime(2035),
                        initialDate: start,
                      );
                      if (d != null) {
                        setDialog(() {
                          start = d;
                          if (end.isBefore(start)) end = start;
                        });
                      }
                    },
                  ),
                  ListTile(
                    title: const Text('Tanggal selesai'),
                    subtitle: Text(DateFormat('yyyy-MM-dd').format(end)),
                    onTap: () async {
                      final d = await showDatePicker(
                        context: ctx,
                        firstDate: start,
                        lastDate: DateTime(2035),
                        initialDate: end,
                      );
                      if (d != null) {
                        setDialog(() => end = d);
                      }
                    },
                  ),
                  TextField(
                    controller: notes,
                    decoration: const InputDecoration(labelText: 'Catatan'),
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
      'start_date': DateFormat('yyyy-MM-dd').format(start),
      'end_date': DateFormat('yyyy-MM-dd').format(end),
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

  Future<void> _openShiftForm({ShiftConfig? item}) async {
    if (!_canWrite) {
      return;
    }
    final name = TextEditingController(text: item?.name ?? '');
    TimeOfDay startTime = TimeOfDay.fromDateTime(
      DateTime.parse('${item?.startTime ?? '08:00'}:00'),
    );
    TimeOfDay endTime = TimeOfDay.fromDateTime(
      DateTime.parse('${item?.endTime ?? '16:00'}:00'),
    );
    final tolerance = TextEditingController(
      text: '${item?.latenessToleranceMinutes ?? 15}',
    );
    bool crosses = item?.crossesMidnight ?? false;
    final save = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialog) => AlertDialog(
          title: Text(item == null ? 'Tambah Shift' : 'Edit Shift'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: name,
                decoration: const InputDecoration(labelText: 'Nama shift'),
              ),
              ListTile(
                title: const Text('Jam mulai'),
                subtitle: Text(DateFormat('HH:mm').format(
                  DateTime(2000, 1, 1, startTime.hour, startTime.minute),
                )),
                trailing: const Icon(Icons.access_time, color: AppColors.accent),
                onTap: () async {
                  final picked = await showTimePicker(
                    context: ctx,
                    initialTime: startTime,
                  );
                  if (picked != null) {
                    setDialog(() => startTime = picked);
                  }
                },
              ),
              ListTile(
                title: const Text('Jam selesai'),
                subtitle: Text(DateFormat('HH:mm').format(
                  DateTime(2000, 1, 1, endTime.hour, endTime.minute),
                )),
                trailing: const Icon(Icons.access_time, color: AppColors.accent),
                onTap: () async {
                  final picked = await showTimePicker(
                    context: ctx,
                    initialTime: endTime,
                  );
                  if (picked != null) {
                    setDialog(() => endTime = picked);
                  }
                },
              ),
              TextField(
                controller: tolerance,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                  labelText: 'Toleransi keterlambatan (menit)',
                ),
              ),
              SwitchListTile(
                value: crosses,
                title: const Text('Melewati tengah malam'),
                onChanged: (v) => setDialog(() => crosses = v),
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
      ),
    );
    if (save != true || name.text.trim().isEmpty) {
      return;
    }
    final payload = {
      'name': name.text.trim(),
      'start_time': '${startTime.hour.toString().padLeft(2, '0')}:${startTime.minute.toString().padLeft(2, '0')}',
      'end_time': '${endTime.hour.toString().padLeft(2, '0')}:${endTime.minute.toString().padLeft(2, '0')}',
      'crosses_midnight': crosses,
      'lateness_tolerance_minutes': int.tryParse(tolerance.text) ?? 15,
      'check_in_window_before_minutes': 60,
      'check_in_window_after_minutes': 120,
      'absent_cutoff_minutes': 120,
      'missing_checkout_cutoff_minutes': 180,
      'is_active': true,
    };
    try {
      final api = ref.read(securityApiClientProvider);
      if (item == null) {
        await api.createManagerShift(payload);
      } else {
        await api.updateManagerShift(item.id, payload);
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
    return Material(
      color: theme.scaffoldBackgroundColor,
      child: Column(
        children: [
          // Level 1 — group tabs (matching web NOTIFIKASI primary row)
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
              fontSize: 12,
              fontWeight: FontWeight.w700,
            ),
            unselectedLabelStyle: const TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
            ),
            tabs: const [
              Tab(icon: Icon(Icons.groups_outlined), text: 'Kelola'),
              Tab(icon: Icon(Icons.qr_code_scanner), text: 'Absensi'),
              Tab(icon: Icon(Icons.tablet), text: 'Perangkat'),
            ],
          ),
          const Divider(height: 1),
          // Level 2 — contextual sub tabs (matching web NOTIFIKASI secondary row)
          _subTabsBar(theme),
        ],
      ),
    );
  }

  Widget _subTabsBar(ThemeData theme) {
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
          tabs: const [
            Tab(icon: Icon(Icons.fingerprint, size: 16), text: 'Absensi'),
            Tab(icon: Icon(Icons.edit_note, size: 16), text: 'Koreksi'),
          ],
        );
      case 2:
        return TabBar(
          controller: _perangkatTabs,
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
            Tab(icon: Icon(Icons.tablet_android, size: 16), text: 'Perangkat'),
            Tab(
              icon: Icon(Icons.menu_book_outlined, size: 16),
              text: 'Buku Tamu',
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
    final permissions = _user?['effective_permissions'] is List
        ? (_user!['effective_permissions'] as List).whereType<String>().toSet()
        : <String>{};
    final canRead =
        permissions.contains('security.schedule.read') ||
        permissions.contains('security.schedule.write') ||
        permissions.contains('security.device.manage') ||
        permissions.contains('security.correction') ||
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
                      TabBarView(
                        controller: _perangkatTabs,
                        children: [_deviceTab(), _guestTab()],
                      ),
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
                              'Penugasan: ${s.shiftName} ${s.startTime}–${s.endTime}\\nPeriode: ${s.assignmentStartDate} – ${s.assignmentEndDate}\\nStatus: ${s.attendanceStatusLabel ?? '-'}',
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

  Widget _shiftTab() {
    return Column(
      children: [
        Expanded(
          child: _shifts.isEmpty
              ? const Center(child: Text('Belum ada shift.'))
              : ListView(
                  children: _shifts
                      .map(
                        (s) => Card(
                          child: ListTile(
                            title: Text(s.name),
                            subtitle: Text(
                              '${s.startTime}–${s.endTime} · Toleransi ${s.latenessToleranceMinutes} menit · ${s.isActive ? 'Aktif' : 'Nonaktif'}',
                            ),
                            trailing: _canWrite
                                ? IconButton(
                                    icon: const Icon(Icons.edit),
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
    );
  }

  Widget _scheduleTab() => Column(
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
                              Text(
                                '${s.assignmentStartDate} – ${s.assignmentEndDate}\n${s.shiftDate} · ${s.startTime}–${s.endTime}',
                              ),
                              const SizedBox(height: 4),
                              AttendanceStatusBadge(
                                statusCode: s.attendanceStatus,
                                statusLabel: s.attendanceStatusLabel,
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
                                'Periode tugas ${s.assignmentStartDate} – ${s.assignmentEndDate}\n${s.shiftName} ${s.startTime}–${s.endTime}\nAbsensi: ${s.attendanceStatusLabel ?? '-'}',
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
        child: _attendance.isEmpty
            ? const Center(child: Text('Belum ada catatan absensi.'))
            : ListView(
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
    ],
  );

  Widget _correctionTab() => _corrections.isEmpty
      ? const Center(child: Text('Tidak ada pengajuan koreksi.'))
      : ListView(
          children: _corrections
              .map(
                (c) => Card(
                  child: ListTile(
                    title: Text(
                      '${c.officerName} · ${c.shiftDate} · ${c.status}',
                    ),
                    subtitle: Text(
                      'Alasan: ${c.reason}\n${c.requestType}: ${c.originalStatus} → ${c.requestedStatus}\nDiajukan: ${c.createdAt ?? '-'} · Keputusan: ${c.decidedAt ?? '-'}',
                    ),
                    trailing:
                        _canDecide &&
                            c.status == 'pending' &&
                            c.requestedBy != (_user?['id']?.toString() ?? '')
                        ? PopupMenuButton<bool>(
                            onSelected: (approve) =>
                                _decideCorrection(c, approve),
                            itemBuilder: (_) => const [
                              PopupMenuItem(
                                value: true,
                                child: Text('Setujui'),
                              ),
                              PopupMenuItem(value: false, child: Text('Tolak')),
                            ],
                          )
                        : null,
                  ),
                ),
              )
              .toList(),
        );

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
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                  ? Center(child: Text(_error!))
                  : _guests.isEmpty
                  ? const Center(child: Text('Tidak ada catatan Buku Tamu.'))
                  : ListView(
                      children: _guests
                          .map(
                            (g) => Card(
                              child: ListTile(
                                title: Text(
                                  '${g.guestName} → ${g.destinationLabel}',
                                ),
                                subtitle: Text(
                                  '${g.purpose}\nMasuk: ${DateFormat('dd/MM HH:mm').format(g.checkedInAt.toLocal())} · ${g.checkedOutAt == null ? 'Belum keluar' : 'Keluar: ${DateFormat('dd/MM HH:mm').format(g.checkedOutAt!.toLocal())}'}\n${_guestDurationLabel(g)}',
                                ),
                                trailing: const Icon(Icons.chevron_right),
                                onTap: () => _showGuestDetail(g),
                              ),
                            ),
                          )
                          .toList(),
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

  Widget _deviceTab() => _devices.isEmpty
      ? const Center(child: Text('Belum ada perangkat pos.'))
      : ListView(
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
