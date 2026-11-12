import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import 'package:excel/excel.dart' as xls;
import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import 'package:printing/printing.dart';
import 'package:social_finance/core/theme/app_radius.dart';

import '../utils/guest_visit_duration.dart' as guest_duration;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import 'package:qr_flutter/qr_flutter.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/monitoring/data/monitoring_models.dart';

import '../../data/pos_device_storage.dart';
import '../../data/security_api_client.dart';
import '../../data/security_models.dart';
import '../providers/security_providers.dart';
import '../widgets/security_waste_list.dart';
import '../widgets/pos_resident_image.dart';

class PosKioskScreen extends ConsumerStatefulWidget {
  const PosKioskScreen({super.key});

  @override
  ConsumerState<PosKioskScreen> createState() => _PosKioskScreenState();
}

class _PosKioskScreenState extends ConsumerState<PosKioskScreen>
    with WidgetsBindingObserver {
  int _tabIndex = 2; // Default on Absensi QR screen for quick pos readiness

  // Tab 0: Warga (read-only)
  final _wargaSearchCtrl = TextEditingController();
  Timer? _wargaSearchDebounce;
  bool _loadingWarga = false;
  List<PosHouseholdSummary> _households = [];
  String? _wargaError;
  int _wargaPage = 1;
  int _wargaTotal = 0;
  int _wargaTotalPages = 1;

  // Tab 1: Buku Tamu
  final _guestSearchCtrl = TextEditingController();
  Timer? _guestSearchDebounce;
  Timer? _guestTicker;
  int _guestPage = 1;
  int _guestTotal = 0;
  int _guestTotalPages = 1;
  bool _loadingGuests = false;
  String _guestStatusFilter = 'active';
  List<GuestBookEntry> _guests = [];
  String? _guestError;

  // Tab 2: Absensi Dynamic QR & Today's Board
  bool _loadingQr = false;
  DynamicQrPayload? _qrPayload;
  int _qrRemainingSeconds = 0;
  Timer? _qrCountdownTimer;
  List<AttendanceRecord> _todayAttendance = [];
  String? _qrError;

  // Tab 3: Sampah Tidak Terangkut (read-only)
  bool _loadingSampah = false;
  List<MonitoringHousehold> _sampahList = [];
  String _sampahEvalDate = '';
  String? _sampahError;
  bool _exportingSampah = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadTab(_tabIndex);
      _startQrCountdownTicker();
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
    _qrCountdownTimer?.cancel();
    _wargaSearchCtrl.dispose();
    _wargaSearchDebounce?.cancel();
    _guestSearchCtrl.dispose();
    _guestSearchDebounce?.cancel();
    _guestTicker?.cancel();
    super.dispose();
  }

  PosDeviceSession? get _session => ref.read(posSessionProvider).session;

  void _startQrCountdownTicker() {
    _qrCountdownTimer?.cancel();
    _qrCountdownTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (!mounted) return;
      if (_qrPayload == null) return;
      final remaining = _qrPayload!.expiresAt
          .difference(DateTime.now())
          .inSeconds;
      if (remaining <= 0) {
        setState(() => _qrRemainingSeconds = 0);
        if (_tabIndex == 2 && !_loadingQr) {
          _refreshQrAndTodayAttendance(silent: true);
        }
      } else {
        setState(() => _qrRemainingSeconds = remaining);
      }
    });
  }

  Future<void> _handleDeviceError(Object e) async {
    if (e is SecurityApiException && e.isDeviceRevokedOrUnauthorized) {
      await ref
          .read(posSessionProvider.notifier)
          .unpair(
            reason:
                'Sesi perangkat pos berakhir atau dicabut oleh Ketua RT. Silakan aktivasi ulang.',
          );
      if (mounted) {
        context.go('/pos/activate');
      }
    }
  }

  Future<void> _loadTab(int idx) async {
    switch (idx) {
      case 0:
        await _loadHouseholds();
        break;
      case 1:
        await _loadGuests();
        break;
      case 2:
        await _refreshQrAndTodayAttendance();
        break;
      case 3:
        await _loadSampah();
        break;
    }
  }

  Future<void> _loadHouseholds() async {
    final s = _session;
    if (s == null) return;
    setState(() {
      _loadingWarga = true;
      _wargaError = null;
    });
    try {
      final page = await ref
          .read(securityApiClientProvider)
          .fetchPosHouseholdsPage(
            s,
            search: _wargaSearchCtrl.text,
            page: _wargaPage,
            perPage: 20,
          );
      if (!mounted) return;
      setState(() {
        _households = page.data;
        _wargaTotal = page.total;
        _wargaTotalPages = page.totalPages;
        _loadingWarga = false;
      });
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      setState(() {
        _wargaError = e.toString();
        _loadingWarga = false;
      });
    }
  }

  Future<void> _showHouseholdDetail(PosHouseholdSummary summary) async {
    final s = _session;
    if (s == null) return;
    try {
      final detail = await ref
          .read(securityApiClientProvider)
          .fetchPosHouseholdDetail(s, summary.householdOccupancyId);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: Text('Rumah No. ${detail.houseNumber}'),
          content: SizedBox(
            width: double.maxFinite,
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text('Kepala Keluarga: ${detail.headOfFamilyName}'),
                  Text('Status Hunian: ${detail.occupancyType}'),
                  const Divider(height: 20),
                  const Text(
                    'Anggota Rumah (Read-Only):',
                    style: TextStyle(fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 8),
                  if (detail.residents.isEmpty)
                    const Text('Tidak ada data anggota aktif.')
                  else
                    ...detail.residents.map(
                      (r) => Padding(
                        padding: const EdgeInsets.only(bottom: 6),
                        child: Row(
                          children: [
                            PosResidentAvatar(
                              name: r.fullName,
                              avatarPath: r.avatarPath,
                              size: 32,
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                '${r.fullName} (${r.relationship}${r.isHeadOfFamily ? ', KK' : ''})',
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('Tutup'),
            ),
          ],
        ),
      );
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _loadGuests() async {
    final s = _session;
    if (s == null) return;
    setState(() {
      _loadingGuests = true;
      _guestError = null;
    });
    try {
      final list = await ref
          .read(securityApiClientProvider)
          .fetchPosGuestBooks(
            s,
            status: _guestStatusFilter,
            search: _guestSearchCtrl.text,
            page: _guestPage,
          );
      if (!mounted) return;
      setState(() {
        _guests = list.data;
        _guestTotal = list.total;
        _guestTotalPages = list.totalPages;
        _loadingGuests = false;
      });
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      setState(() {
        _guestError = e.toString();
        _loadingGuests = false;
      });
    }
  }

  Future<void> _openCreateGuestDialog() async {
    final s = _session;
    if (s == null) return;
    final nameCtrl = TextEditingController();
    final phoneCtrl = TextEditingController();
    final purposeCtrl = TextEditingController();
    final destinationCtrl = TextEditingController();
    final plateCtrl = TextEditingController();
    final notesCtrl = TextEditingController();

    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Catat Tamu Masuk'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: nameCtrl,
                decoration: const InputDecoration(
                  labelText: 'Nama Tamu *',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: phoneCtrl,
                decoration: const InputDecoration(
                  labelText: 'No. HP (Opsional)',
                  border: OutlineInputBorder(),
                ),
                keyboardType: TextInputType.phone,
              ),
              const SizedBox(height: 10),
              TextField(
                controller: destinationCtrl,
                decoration: const InputDecoration(
                  labelText: 'Rumah / Tujuan *',
                  hintText: 'Contoh: Rumah No. 12 Pak Budi',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: purposeCtrl,
                decoration: const InputDecoration(
                  labelText: 'Keperluan *',
                  hintText: 'Contoh: Mengantar Paket / Bertamu',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: plateCtrl,
                decoration: const InputDecoration(
                  labelText: 'Plat Kendaraan (Opsional)',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: notesCtrl,
                decoration: const InputDecoration(
                  labelText: 'Catatan (Opsional)',
                  border: OutlineInputBorder(),
                ),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Simpan Tamu'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    if (nameCtrl.text.trim().isEmpty ||
        destinationCtrl.text.trim().isEmpty ||
        purposeCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Nama tamu, tujuan, dan keperluan wajib diisi.'),
        ),
      );
      return;
    }

    try {
      await ref
          .read(securityApiClientProvider)
          .createPosGuestBook(
            s,
            guestName: nameCtrl.text,
            phone: phoneCtrl.text,
            purpose: purposeCtrl.text,
            destinationLabel: destinationCtrl.text,
            vehiclePlate: plateCtrl.text,
            notes: notesCtrl.text,
          );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Data tamu berhasil dicatat.')),
      );
      await _loadGuests();
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _checkoutGuest(GuestBookEntry entry) async {
    final s = _session;
    if (s == null) return;
    try {
      await ref
          .read(securityApiClientProvider)
          .checkoutPosGuestBook(s, entry.id);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('${entry.guestName} dicatat keluar.')),
      );
      await _loadGuests();
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _refreshQrAndTodayAttendance({bool silent = false}) async {
    final s = _session;
    if (s == null) return;
    if (!silent) {
      setState(() {
        _loadingQr = true;
        _qrError = null;
      });
    }
    try {
      final api = ref.read(securityApiClientProvider);
      final results = await Future.wait([
        api.issuePosQrToken(s),
        api.fetchPosTodayAttendance(s),
      ]);
      if (!mounted) return;
      final payload = results[0] as DynamicQrPayload;
      setState(() {
        _qrPayload = payload;
        _qrRemainingSeconds = payload.expiresInSeconds;
        _todayAttendance = results[1] as List<AttendanceRecord>;
        _loadingQr = false;
        _qrError = null;
      });
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      setState(() {
        _qrError = e.toString();
        _loadingQr = false;
      });
    }
  }

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

  Future<void> _loadSampah() async {
    final s = _session;
    if (s == null) return;
    setState(() {
      _loadingSampah = true;
      _sampahError = null;
    });
    try {
      final res = await ref
          .read(securityApiClientProvider)
          .fetchPosMonitoringSampah(s);
      if (!mounted) return;
      setState(() {
        _sampahList = res.items;
        _sampahEvalDate = res.evalDate;
        _loadingSampah = false;
      });
    } catch (e) {
      await _handleDeviceError(e);
      if (!mounted) return;
      setState(() {
        _sampahError = e.toString();
        _loadingSampah = false;
      });
    }
  }

  Future<void> _confirmUnpair() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Keluar Mode Perangkat Pos?'),
        content: const Text(
          'Token perangkat di ponsel ini akan dihapus dan Anda perlu token pairing baru dari Ketua RT untuk mengaktifkan ulang pos.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.danger),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text(
              'Ya, Keluar',
              style: TextStyle(color: Colors.white),
            ),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    await ref.read(posSessionProvider.notifier).unpair();
    if (mounted) {
      context.go('/login');
    }
  }

  @override
  Widget build(BuildContext context) {
    final posSession = ref.watch(posSessionProvider).session;
    final deviceName = posSession?.device.deviceName ?? 'Pos Keamanan';
    final locationLabel = posSession?.device.locationLabel ?? '';

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(deviceName, style: const TextStyle(fontSize: 16)),
            if (locationLabel.isNotEmpty)
              Text(
                locationLabel,
                style: const TextStyle(fontSize: 12, color: Colors.grey),
              ),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'Keluar Mode Pos',
            icon: const Icon(Icons.logout_outlined),
            onPressed: _confirmUnpair,
          ),
        ],
      ),
      body: IndexedStack(
        index: _tabIndex,
        children: [
          _buildWargaTab(),
          _buildGuestTab(),
          _buildQrAbsensiTab(),
          _buildSampahTab(),
        ],
      ),
      floatingActionButton: _tabIndex == 1
          ? FloatingActionButton.extended(
              onPressed: _openCreateGuestDialog,
              backgroundColor: AppColors.accent,
              foregroundColor: Colors.white,
              icon: const Icon(Icons.person_add_alt_1),
              label: const Text('Tamu Masuk'),
            )
          : null,
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _tabIndex,
        type: BottomNavigationBarType.fixed,
        selectedItemColor: AppColors.accent,
        onTap: (idx) {
          setState(() => _tabIndex = idx);
          _loadTab(idx);
        },
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.people_outline),
            activeIcon: Icon(Icons.people),
            label: 'Warga',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.menu_book_outlined),
            activeIcon: Icon(Icons.menu_book),
            label: 'Buku Tamu',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.qr_code_2),
            activeIcon: Icon(Icons.qr_code),
            label: 'Absensi',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.delete_sweep_outlined),
            activeIcon: Icon(Icons.delete_sweep),
            label: 'Sampah',
          ),
        ],
      ),
    );
  }

  Widget _buildWargaTab() {
    // Device context: warga directory for the paired RT, strictly read-only.
    // Mirrors the Petugas Keamanan Warga menu (search, detail, refresh) with
    // device-friendly pagination instead of a write-capable FAB.
    final theme = Theme.of(context);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 12, 12, 0),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _wargaSearchCtrl,
                  decoration: InputDecoration(
                    hintText: 'Cari no. rumah atau kepala keluarga...',
                    prefixIcon: const Icon(Icons.search, size: 20),
                    suffixIcon: _wargaSearchCtrl.text.isEmpty
                        ? null
                        : IconButton(
                            icon: const Icon(Icons.clear),
                            onPressed: () {
                              _wargaSearchCtrl.clear();
                              _wargaSearchDebounce?.cancel();
                              setState(() {
                                _wargaPage = 1;
                              });
                              _loadHouseholds();
                            },
                          ),
                    border: const OutlineInputBorder(),
                    isDense: true,
                  ),
                  onChanged: (value) {
                    setState(() {});
                    _wargaSearchDebounce?.cancel();
                    _wargaSearchDebounce = Timer(
                      const Duration(milliseconds: 350),
                      () {
                        if (!mounted) return;
                        setState(() => _wargaPage = 1);
                        _loadHouseholds();
                      },
                    );
                  },
                  onSubmitted: (_) => _loadHouseholds(),
                ),
              ),
              const SizedBox(width: 8),
              ElevatedButton(
                onPressed: _loadingWarga ? null : _loadHouseholds,
                child: const Text('Cari'),
              ),
            ],
          ),
        ),
        if (_wargaTotal > 0)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
            child: Text(
              '$_wargaTotal rumah · Halaman $_wargaPage / $_wargaTotalPages',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        Expanded(
          child: _loadingWarga && _households.isEmpty
              ? const Center(child: CircularProgressIndicator())
              : _wargaError != null && _households.isEmpty
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.error_outline, size: 40),
                        const SizedBox(height: 8),
                        Text(
                          'Gagal memuat data warga.',
                          style: theme.textTheme.titleMedium,
                        ),
                        const SizedBox(height: 4),
                        Text(
                          _wargaError!,
                          textAlign: TextAlign.center,
                          style: theme.textTheme.bodySmall,
                        ),
                        const SizedBox(height: 12),
                        ElevatedButton.icon(
                          onPressed: _loadHouseholds,
                          icon: const Icon(Icons.refresh),
                          label: const Text('Muat Ulang'),
                        ),
                      ],
                    ),
                  ),
                )
              : _households.isEmpty
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(Icons.home_outlined, size: 48),
                      const SizedBox(height: 8),
                      Text(
                        'Tidak ada data warga RT.',
                        style: theme.textTheme.titleMedium,
                      ),
                    ],
                  ),
                )
              : RefreshIndicator(
                  onRefresh: () async {
                    setState(() => _wargaPage = 1);
                    await _loadHouseholds();
                  },
                  child: ListView.separated(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 8,
                    ),
                    itemCount: _households.length,
                    separatorBuilder: (_, _) => const SizedBox(height: 8),
                    itemBuilder: (_, i) {
                      final h = _households[i];
                      return _HouseholdCard(
                        household: h,
                        onTap: () => _showHouseholdDetail(h),
                      );
                    },
                  ),
                ),
        ),
        if (_wargaTotalPages > 1)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                IconButton(
                  onPressed: _wargaPage > 1
                      ? () {
                          setState(() => _wargaPage--);
                          _loadHouseholds();
                        }
                      : null,
                  icon: const Icon(Icons.chevron_left),
                ),
                Text('Halaman $_wargaPage / $_wargaTotalPages'),
                IconButton(
                  onPressed: _wargaPage < _wargaTotalPages
                      ? () {
                          setState(() => _wargaPage++);
                          _loadHouseholds();
                        }
                      : null,
                  icon: const Icon(Icons.chevron_right),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Widget _buildGuestTab() {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            children: [
              TextField(
                controller: _guestSearchCtrl,
                decoration: InputDecoration(
                  hintText: 'Cari nama atau tujuan kunjungan...',
                  prefixIcon: const Icon(Icons.search),
                  suffixIcon: _guestSearchCtrl.text.isEmpty
                      ? null
                      : IconButton(
                          icon: const Icon(Icons.clear),
                          onPressed: () {
                            _guestSearchCtrl.clear();
                            setState(() => _guestPage = 1);
                            _loadGuests();
                          },
                        ),
                  border: const OutlineInputBorder(),
                  isDense: true,
                ),
                onChanged: (_) {
                  _guestSearchDebounce?.cancel();
                  _guestTicker?.cancel();
                  _guestSearchDebounce = Timer(
                    const Duration(milliseconds: 350),
                    () {
                      setState(() => _guestPage = 1);
                      _loadGuests();
                    },
                  );
                },
              ),
              const SizedBox(height: 8),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'active', label: Text('Masih di RT')),
                  ButtonSegment(
                    value: 'checked_out',
                    label: Text('Sudah Keluar'),
                  ),
                ],
                selected: {_guestStatusFilter},
                onSelectionChanged: (sel) {
                  setState(() {
                    _guestStatusFilter = sel.first;
                    _guestPage = 1;
                  });
                  _loadGuests();
                },
              ),
            ],
          ),
        ),
        Expanded(
          child: _loadingGuests
              ? const Center(child: CircularProgressIndicator())
              : _guestError != null
              ? Center(child: Text(_guestError!))
              : _guests.isEmpty
              ? const Center(child: Text('Belum ada catatan buku tamu.'))
              : RefreshIndicator(
                  onRefresh: _loadGuests,
                  child: ListView.builder(
                    padding: const EdgeInsets.only(bottom: 80),
                    itemCount: _guests.length,
                    itemBuilder: (_, i) {
                      final g = _guests[i];
                      return Card(
                        margin: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 6,
                        ),
                        child: ListTile(
                          title: Text(
                            '${g.guestName} → ${g.destinationLabel}',
                            style: const TextStyle(fontWeight: FontWeight.w600),
                          ),
                          subtitle: Text(
                            'Keperluan: ${g.purpose}\nMasuk: ${DateFormat('dd/MM HH:mm').format(g.checkedInAt)}${g.vehiclePlate != null ? ' · Plat: ${g.vehiclePlate}' : ''}${g.phoneMasked != null ? ' · HP: ${g.phoneMasked}' : ''}',
                          ),
                          isThreeLine: true,
                          trailing: g.checkedOutAt == null
                              ? OutlinedButton(
                                  onPressed: () => _checkoutGuest(g),
                                  child: const Text('Keluar'),
                                )
                              : Text(
                                  DateFormat('HH:mm').format(g.checkedOutAt!),
                                  style: const TextStyle(color: Colors.grey),
                                ),
                          onTap: () => _showGuestDetail(g),
                        ),
                      );
                    },
                  ),
                ),
        ),
        if (_guestTotalPages > 1)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                IconButton(
                  onPressed: _guestPage > 1
                      ? () {
                          setState(() => _guestPage--);
                          _loadGuests();
                        }
                      : null,
                  icon: const Icon(Icons.chevron_left),
                ),
                Text('Halaman $_guestPage / $_guestTotalPages · $_guestTotal'),
                IconButton(
                  onPressed: _guestPage < _guestTotalPages
                      ? () {
                          setState(() => _guestPage++);
                          _loadGuests();
                        }
                      : null,
                  icon: const Icon(Icons.chevron_right),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Future<void> _showGuestDetail(GuestBookEntry g) async {
    await showDialog<void>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setD) {
          // refresh duration label on rebuild ticks
          return AlertDialog(
            title: Text(g.guestName),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Tujuan: ${g.destinationLabel}'),
                  const SizedBox(height: 4),
                  Text('Keperluan: ${g.purpose}'),
                  const SizedBox(height: 4),
                  Text(
                    'Masuk: ${DateFormat('dd/MM/yyyy HH:mm').format(g.checkedInAt.toLocal())}',
                  ),
                  const SizedBox(height: 4),
                  Text(
                    g.checkedOutAt == null
                        ? 'Keluar: Belum keluar'
                        : 'Keluar: ${DateFormat('dd/MM/yyyy HH:mm').format(g.checkedOutAt!.toLocal())}',
                  ),
                  const SizedBox(height: 4),
                  Text(_guestDurationLabel(g)),
                  const SizedBox(height: 4),
                  Text('Perangkat: ${g.deviceName}'),
                  const SizedBox(height: 4),
                  Text('Catatan: ${g.notes ?? '-'}'),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(ctx),
                child: const Text('Tutup'),
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildQrAbsensiTab() {
    return RefreshIndicator(
      onRefresh: () => _refreshQrAndTodayAttendance(),
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Card(
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(16),
            ),
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                children: [
                  const Text(
                    'QR Dinamis Absensi Petugas',
                    style: TextStyle(fontSize: 17, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 6),
                  const Text(
                    'Petugas keamanan memindai QR ini untuk Check-In atau Check-Out shift.',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: Colors.grey, fontSize: 13),
                  ),
                  const SizedBox(height: 16),
                  if (_loadingQr && _qrPayload == null)
                    const SizedBox(
                      height: 220,
                      child: Center(child: CircularProgressIndicator()),
                    )
                  else if (_qrError != null && _qrPayload == null)
                    Column(
                      children: [
                        Text(
                          _qrError!,
                          style: const TextStyle(color: AppColors.danger),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: 12),
                        ElevatedButton(
                          onPressed: () => _refreshQrAndTodayAttendance(),
                          child: const Text('Coba Muat QR'),
                        ),
                      ],
                    )
                  else if (_qrPayload != null) ...[
                    Container(
                      padding: const EdgeInsets.all(14),
                      decoration: BoxDecoration(
                        color: Colors.white,
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(color: Colors.grey.shade300),
                      ),
                      child: QrImageView(
                        data: _qrPayload!.qrToken,
                        version: QrVersions.auto,
                        size: 220,
                      ),
                    ),
                    const SizedBox(height: 12),
                    Text(
                      _qrRemainingSeconds > 0
                          ? 'Berlaku $_qrRemainingSeconds detik lagi'
                          : 'Memperbarui QR...',
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        color: _qrRemainingSeconds <= 10
                            ? AppColors.warning
                            : AppColors.success,
                      ),
                    ),
                    const SizedBox(height: 8),
                    OutlinedButton.icon(
                      onPressed: _loadingQr
                          ? null
                          : () => _refreshQrAndTodayAttendance(),
                      icon: const Icon(Icons.refresh),
                      label: const Text('Refresh QR Sekarang'),
                    ),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Absensi Petugas Hari Ini',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          if (_todayAttendance.isEmpty)
            const Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('Belum ada jadwal absensi hari ini.'),
              ),
            )
          else
            ..._todayAttendance.map(
              (r) => Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: ListTile(
                  title: Text(
                    '${r.officerName} · ${r.shiftName}',
                    style: const TextStyle(fontWeight: FontWeight.w600),
                  ),
                  subtitle: Text(
                    'Jam ${r.startTime.substring(0, 5)}-${r.endTime.substring(0, 5)} | Masuk: ${r.checkedInAt != null ? DateFormat('HH:mm').format(r.checkedInAt!) : '-'} | Pulang: ${r.checkedOutAt != null ? DateFormat('HH:mm').format(r.checkedOutAt!) : '-'}',
                  ),
                  trailing: Text(
                    r.statusLabel,
                    style: const TextStyle(fontWeight: FontWeight.w600),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  /// Prompt the user to edit the note for a monitoring-household.
  Future<void> _showSampahNoteDialog(MonitoringHousehold item) async {
    final controller = TextEditingController(text: item.note ?? '');
    final save = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Catatan Rumah ${item.houseNumber}'),
        content: TextField(
          controller: controller,
          maxLines: 4,
          maxLength: 2000,
          decoration: const InputDecoration(
            border: OutlineInputBorder(),
            hintText: 'Contoh: rumah dikunjungi, sampah belum diangkut…',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Simpan'),
          ),
        ],
      ),
    );
    if (save != true) return;
    final note = controller.text.trim();
    if (note.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Catatan tidak boleh kosong')),
        );
      }
      return;
    }

    final session = _session;
    if (session == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Sesi pos sudah berakhir. Aktifkan ulang.')),
        );
      }
      return;
    }

    try {
      await ref.read(securityApiClientProvider).updateSampahNote(
        session,
        householdOccupancyId: item.householdOccupancyId,
        note: note,
      );
      await _loadSampah();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Catatan tersimpan')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gagal menyimpan catatan: $e'),
            backgroundColor: Colors.redAccent,
          ),
        );
      }
    }
  }

  /// RT code of the paired device, zero-padded for display.
  String get _deviceRtCodePadded {
    final code = _session?.device.rtCode.trim() ?? '';
    if (code.isEmpty) return '';
    final digits = int.tryParse(code);
    return digits != null ? digits.toString().padLeft(3, '0') : code;
  }

  /// Working rows for the Sampah report (mirrors the user-side export).
  List<List<String>> _sampahReportRows() {
    final rt = _deviceRtCodePadded;
    return [
      for (var i = 0; i < _sampahList.length; i++)
        [
          '${i + 1}',
          rt,
          _sampahList[i].houseNumber,
          (_sampahList[i].note ?? '').trim().isEmpty
              ? 'Belum tercatat diangkut'
              : _sampahList[i].note!,
        ],
    ];
  }

  Future<Uint8List> _buildSampahPdf() async {
    String evalLabel;
    if (_sampahEvalDate.isEmpty) {
      evalLabel = DateFormat('dd/MM/yyyy').format(DateTime.now());
    } else {
      try {
        evalLabel = DateFormat('dd/MM/yyyy').format(DateTime.parse(_sampahEvalDate));
      } catch (_) {
        evalLabel = _sampahEvalDate;
      }
    }
    final rt = _deviceRtCodePadded;

    final pdf = pw.Document();
    pdf.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        build: (context) => [
          pw.Text(
            'LAPORAN MONITORING KEBERSIHAN',
            style: pw.TextStyle(
              fontSize: 16,
              fontWeight: pw.FontWeight.bold,
            ),
          ),
          pw.SizedBox(height: 8),
          pw.Text('RT $rt'),
          pw.Text('Periode: $evalLabel'),
          pw.SizedBox(height: 12),
          pw.Table.fromTextArray(
            headers: const ['No.', 'RT', 'No. Rumah', 'Catatan'],
            data: _sampahReportRows(),
            border: pw.TableBorder.all(width: 0.5),
            headerDecoration: const pw.BoxDecoration(
              color: PdfColors.grey300,
            ),
            cellPadding: const pw.EdgeInsets.all(4),
          ),
        ],
      ),
    );
    return Uint8List.fromList(await pdf.save());
  }

  Future<Uint8List> _buildSampahExcel() async {
    final rt = _deviceRtCodePadded;
    final evalLabel =
        _sampahEvalDate.isEmpty ? DateTime.now().toIso8601String().split('T').first : _sampahEvalDate;

    final excel = xls.Excel.createExcel();
    final sheetName = excel.tables.keys.isNotEmpty ? excel.tables.keys.first : 'Sheet1';
    final sheet = excel[sheetName];

    void setCell(String ref, String value) {
      sheet.cell(xls.CellIndex.indexByString(ref)).value =
          xls.TextCellValue(value);
    }

    setCell('A1', 'LAPORAN MONITORING KEBERSIHAN');
    setCell('A2', 'RT $rt');
    setCell('A3', 'Periode: $evalLabel');
    setCell('A5', 'No.');
    setCell('B5', 'RT');
    setCell('C5', 'No. Rumah');
    setCell('D5', 'Catatan');

    var row = 6;
    for (var i = 0; i < _sampahList.length; i++) {
      setCell('A$row', '${i + 1}');
      setCell('B$row', rt);
      setCell('C$row', _sampahList[i].houseNumber);
      setCell(
        'D$row',
        (_sampahList[i].note ?? '').trim().isEmpty
            ? 'Belum tercatat diangkut'
            : _sampahList[i].note!,
      );
      row++;
    }

    return Uint8List.fromList(excel.save() ?? <int>[]);
  }

  Future<void> _saveSampahBytesToDownload(Uint8List bytes, String ext) async {
    final now = DateTime.now();
    final date =
        '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
    final rtLabel = _deviceRtCodePadded.isEmpty ? 'RT' : 'RT${_deviceRtCodePadded}';
    final fileName = 'Monitoring_Kebersihan_${rtLabel}_$date.$ext';

    Directory? targetDir;
    if (Platform.isAndroid) {
      final androidDownload = Directory('/storage/emulated/0/Download');
      if (await androidDownload.exists()) {
        targetDir = androidDownload;
      } else {
        try {
          targetDir = await getDownloadsDirectory();
        } catch (_) {}
        targetDir ??= await getExternalStorageDirectory() ??
            await getApplicationDocumentsDirectory();
      }
    } else {
      targetDir =
          await getDownloadsDirectory() ?? await getApplicationDocumentsDirectory();
    }
    if (!await targetDir.exists()) {
      await targetDir.create(recursive: true);
    }

    var counter = 1;
    var savePath = '${targetDir.path}/$fileName';
    var finalName = fileName;
    final dotIdx = fileName.lastIndexOf('.');
    final baseName =
        dotIdx != -1 ? fileName.substring(0, dotIdx) : fileName;
    final extDot = dotIdx != -1 ? fileName.substring(dotIdx) : '';
    while (await File(savePath).exists()) {
      finalName = '${baseName}_$counter$extDot';
      savePath = '${targetDir.path}/$finalName';
      counter++;
    }

    final file = File(savePath);
    await file.writeAsBytes(bytes, flush: true);
    if (await file.length() != bytes.length) {
      throw const FileSystemException('File export tidak tersimpan utuh.');
    }
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Tersimpan: $finalName'),
      ),
    );
  }

  Future<void> _exportSampahExcel() async {
    if (_exportingSampah) return;
    setState(() => _exportingSampah = true);
    try {
      final bytes = await _buildSampahExcel();
      if (bytes.isEmpty) throw const FormatException('File export kosong.');
      await _saveSampahBytesToDownload(bytes, 'xlsx');
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gagal mengekspor Excel: $e'),
            backgroundColor: Colors.redAccent,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _exportingSampah = false);
    }
  }

  Future<void> _exportSampahPDF() async {
    if (_exportingSampah) return;
    setState(() => _exportingSampah = true);
    try {
      final bytes = await _buildSampahPdf();
      if (bytes.isEmpty) throw const FormatException('File export kosong.');
      await _saveSampahBytesToDownload(bytes, 'pdf');
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gagal mengekspor PDF: $e'),
            backgroundColor: Colors.redAccent,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _exportingSampah = false);
    }
  }

  Future<void> _printSampahPDF() async {
    if (_exportingSampah) return;
    setState(() => _exportingSampah = true);
    try {
      final bytes = await _buildSampahPdf();
      if (bytes.isEmpty) throw const FormatException('File cetak kosong.');
      await Printing.layoutPdf(
        onLayout: (format) async => bytes,
        name: 'Laporan Monitoring Kebersihan',
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Gagal mencetak: $e'),
            backgroundColor: Colors.redAccent,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _exportingSampah = false);
    }
  }

  Widget _buildSampahTab() => Column(
        children: [
          Expanded(
            child: SecurityWasteList(
              households: _sampahList,
              evalDate: _sampahEvalDate,
              loading: _loadingSampah,
              error: _sampahError,
              onRetry: _loadSampah,
              onNoteEdit: _showSampahNoteDialog,
            ),
          ),
          if (!_loadingSampah &&
              _sampahError == null &&
              _sampahList.isNotEmpty)
            _SampahExportBar(
              exporting: _exportingSampah,
              onExportExcel: _exportSampahExcel,
              onExportPdf: _exportSampahPDF,
              onPrint: _printSampahPDF,
            ),
        ],
      );
}

/// Card matching the Petugas Keamanan Warga list layout (avatar left → name +
/// badges → divider → icon rows). The device is read-only and receives only
/// head-of-family name + house metadata — no phone/email/NIK.
class _HouseholdCard extends StatelessWidget {
  final PosHouseholdSummary household;
  final VoidCallback onTap;

  const _HouseholdCard({required this.household, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    final isOwner =
        household.occupancyType.trim().toUpperCase() == 'OWNER';
    final occupancyColor = isOwner ? AppColors.accent : AppColors.info;
    final occupiedCount = household.residentCount;
    final occupancyLabel = isOwner ? 'Pemilik' : 'Penyewa';
    final homeIconColor = isDark ? const Color(0xFFFBBF24) : const Color(0xFFD97706);
    final memberIconColor = isDark ? AppColors.success : const Color(0xFF16A34A);

    return InkWell(
      borderRadius: BorderRadius.circular(12),
      onTap: onTap,
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: theme.colorScheme.outlineVariant),
        ),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  PosResidentAvatar(
                    name: household.headOfFamilyName,
                    avatarPath: household.headAvatarPath,
                    size: 48,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          household.headOfFamilyName,
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Wrap(
                          spacing: 6,
                          runSpacing: 4,
                          children: [
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 2,
                              ),
                              decoration: BoxDecoration(
                                color: occupancyColor.withValues(
                                  alpha: isDark ? 0.18 : 0.10,
                                ),
                                borderRadius: BorderRadius.circular(4),
                                border: Border.all(
                                  color: occupancyColor.withValues(
                                    alpha: isDark ? 0.35 : 0.25,
                                  ),
                                  width: 0.5,
                                ),
                              ),
                              child: Text(
                                occupancyLabel,
                                style: theme.textTheme.bodySmall?.copyWith(
                                  fontSize: 10,
                                  fontWeight: FontWeight.w500,
                                  color: occupancyColor,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                  const Icon(Icons.chevron_right, size: 20),
                ],
              ),
              const Divider(height: 20),
              Row(
                children: [
                  Icon(Icons.home_outlined, size: 16, color: homeIconColor),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      [
                        'No. ${household.houseNumber}',
                        if (household.block != null)
                          'Blok ${household.block}',
                      ].join(' · '),
                      style: theme.textTheme.bodyMedium,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              Row(
                children: [
                  Icon(Icons.people_outline,
                      size: 16, color: memberIconColor),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      '$occupiedCount Penghuni',
                      style: theme.textTheme.bodyMedium,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Deterministic 1–2 letter initials helper for household cards.
class AvatarInitials {
  static String fromName(String name) {
    final trimmed = name.trim();
    if (trimmed.isEmpty) return '?';
    final words = trimmed.split(RegExp(r'\s+'));
    if (words.length == 1) return words.first.characters.first.toUpperCase();
    return (words.first.characters.first +
            words[words.length - 1].characters.first)
        .toUpperCase();
  }
}

/// Bottom action bar for the kiosk "Sampah" tab. Device is read-only;
/// export/print is generated on-device from already-loaded data.
class _SampahExportBar extends StatelessWidget {
  final bool exporting;
  final VoidCallback onExportExcel;
  final VoidCallback onExportPdf;
  final VoidCallback onPrint;

  const _SampahExportBar({
    required this.exporting,
    required this.onExportExcel,
    required this.onExportPdf,
    required this.onPrint,
  });

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (exporting) ...[
              const LinearProgressIndicator(),
              const SizedBox(height: 8),
              Text(
                'Menyiapkan file...',
                style: Theme.of(context).textTheme.bodySmall,
              ),
              const SizedBox(height: 8),
            ],
            Row(
              children: [
                Expanded(
                  child: _SampahExportButton(
                    label: 'Export Excel',
                    icon: Icons.table_view_outlined,
                    background: AppColors.success,
                    foreground: Colors.white,
                    onPressed: exporting ? null : onExportExcel,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: _SampahExportButton(
                    label: 'Export PDF',
                    icon: Icons.picture_as_pdf_outlined,
                    background: AppColors.danger,
                    foreground: Colors.white,
                    onPressed: exporting ? null : onExportPdf,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            _SampahExportButton(
              width: double.infinity,
              label: 'Cetak',
              icon: Icons.print_outlined,
              background: AppColors.accent,
              foreground: Colors.white,
              onPressed: exporting ? null : onPrint,
            ),
          ],
        ),
      ),
    );
  }
}

class _SampahExportButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color background;
  final Color foreground;
  final VoidCallback? onPressed;
  final double? width;

  const _SampahExportButton({
    required this.label,
    required this.icon,
    required this.background,
    required this.foreground,
    required this.onPressed,
    this.width,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: width ?? double.infinity,
      height: 48,
      child: FilledButton.icon(
        onPressed: onPressed,
        icon: Icon(icon, size: 18),
        label: FittedBox(
          fit: BoxFit.scaleDown,
          child: Text(label, maxLines: 1),
        ),
        style: FilledButton.styleFrom(
          backgroundColor: background,
          foregroundColor: foreground,
          disabledBackgroundColor: background.withValues(alpha: 0.38),
          disabledForegroundColor: foreground.withValues(alpha: 0.38),
          padding: const EdgeInsets.symmetric(horizontal: 12),
          textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppRadius.sm),
          ),
        ),
      ),
    );
  }
}
