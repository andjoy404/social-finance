import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

import '../../data/security_models.dart';
import '../providers/security_providers.dart';
import '../utils/attendance_order.dart';
import '../widgets/attendance_status_badge.dart';
import 'security_qr_scanner_screen.dart';

class SecurityAbsenScreen extends ConsumerStatefulWidget {
  const SecurityAbsenScreen({super.key});

  @override
  ConsumerState<SecurityAbsenScreen> createState() =>
      _SecurityAbsenScreenState();
}

class _SecurityAbsenScreenState extends ConsumerState<SecurityAbsenScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  bool _loading = true;
  String? _error;

  OfficerAttendanceOverview? _overview;
  List<ShiftSwap> _swaps = [];
  List<AttendanceCorrection> _corrections = [];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadData());
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  Future<void> _loadData() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(securityApiClientProvider);
      final results = await Future.wait([
        api.fetchMyAttendanceOverview(),
        api.listSwaps(),
        api.listCorrections(),
      ]);
      if (!mounted) return;
      setState(() {
        _overview = results[0] as OfficerAttendanceOverview;
        _swaps = results[1] as List<ShiftSwap>;
        _corrections = results[2] as List<AttendanceCorrection>;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  String _formatTime(DateTime? dt) {
    if (dt == null) return '-';
    return DateFormat('HH:mm').format(dt);
  }

  Future<void> _openScanner({String actionMode = 'auto'}) async {
    final result = await Navigator.of(context).push<AttendanceScanResult>(
      MaterialPageRoute(
        builder: (_) => SecurityQrScannerScreen(actionMode: actionMode),
      ),
    );
    if (result == null || !mounted) return;
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        icon: const Icon(
          Icons.verified_outlined,
          color: AppColors.success,
          size: 48,
        ),
        title: Text(
          result.action == 'check_out'
              ? 'Check-Out Tercatat'
              : 'Check-In Tercatat',
        ),
        content: Text(
          '${result.message}\n\nPos: ${result.deviceName}\nStatus: ${result.record.statusLabel}\nWaktu Server: ${DateFormat('dd MMM yyyy HH:mm').format(result.serverTime)} WIB',
        ),
        actions: [
          ElevatedButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: const Text('Selesai'),
          ),
        ],
      ),
    );
    await _loadData();
  }

  Future<void> _openSwapDialog(SecuritySchedule mySchedule) async {
    final api = ref.read(securityApiClientProvider);
    final currentUserId =
        ref.read(authRepositoryProvider)?.valueOrNull?['id']?.toString() ?? '';
    List<SecuritySchedule> candidateSchedules = [];
    try {
      final all = await api.listSchedules(dateFrom: mySchedule.shiftDate);
      candidateSchedules = all
          .where(
            (s) =>
                s.id != mySchedule.id &&
                s.userId != currentUserId &&
                s.status == 'scheduled' &&
                s.checkedInAt == null,
          )
          .toList();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Gagal memuat jadwal pengganti: $e')),
      );
      return;
    }
    if (!mounted) return;

    if (candidateSchedules.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Belum ada jadwal petugas lain yang tersedia untuk ditukar.',
          ),
        ),
      );
      return;
    }

    String selectedTargetId = candidateSchedules.first.id;
    final reasonCtrl = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Ajukan Tukar Shift'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Shift Anda: ${mySchedule.shiftDate} · ${mySchedule.shiftName} (${mySchedule.assignmentStartDate} – ${mySchedule.assignmentEndDate})',
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: selectedTargetId,
                  isExpanded: true,
                  decoration: const InputDecoration(
                    labelText: 'Tukar Dengan Jadwal Petugas',
                    border: OutlineInputBorder(),
                  ),
                  items: candidateSchedules
                      .map(
                        (s) => DropdownMenuItem(
                          value: s.id,
                          child: Text(
                            '${s.officerName} · ${s.shiftDate} (${s.shiftName})',
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      )
                      .toList(),
                  onChanged: (v) {
                    if (v != null) {
                      setDialogState(() => selectedTargetId = v);
                    }
                  },
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: reasonCtrl,
                  maxLines: 2,
                  decoration: const InputDecoration(
                    labelText: 'Alasan Tukar Shift',
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
              child: const Text('Ajukan'),
            ),
          ],
        ),
      ),
    );
    if (ok != true || !mounted) return;
    if (reasonCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Alasan tukar shift wajib diisi.')),
      );
      return;
    }
    try {
      await api.createSwap(
        requesterScheduleId: mySchedule.id,
        targetScheduleId: selectedTargetId,
        reason: reasonCtrl.text.trim(),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Pengajuan tukar shift terkirim ke petugas pengganti.'),
        ),
      );
      await _loadData();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _respondSwap(ShiftSwap swap, bool accept) async {
    try {
      await ref
          .read(securityApiClientProvider)
          .respondToSwapTarget(swapId: swap.id, accept: accept);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            accept
                ? 'Permintaan diterima. Menunggu persetujuan Ketua RT.'
                : 'Permintaan tukar shift ditolak.',
          ),
        ),
      );
      await _loadData();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _openCorrectionDialog(AttendanceRecord record) async {
    String requestedStatus = 'hadir';
    final reasonCtrl = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Ajukan Koreksi Absensi'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                '${record.shiftDate} · ${record.shiftName} (${record.statusLabel})',
                style: const TextStyle(fontWeight: FontWeight.w600),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                initialValue: requestedStatus,
                decoration: const InputDecoration(
                  labelText: 'Status yang Diajukan',
                  border: OutlineInputBorder(),
                ),
                items: const [
                  DropdownMenuItem(value: 'hadir', child: Text('Hadir')),
                  DropdownMenuItem(
                    value: 'terlambat',
                    child: Text('Terlambat'),
                  ),
                  DropdownMenuItem(
                    value: 'sudah_absen_pulang',
                    child: Text('Sudah Absen Pulang'),
                  ),
                ],
                onChanged: (v) {
                  if (v != null) {
                    setDialogState(() => requestedStatus = v);
                  }
                },
              ),
              const SizedBox(height: 12),
              TextField(
                controller: reasonCtrl,
                maxLines: 3,
                decoration: const InputDecoration(
                  labelText: 'Alasan Koreksi',
                  hintText: 'Misal: Lupa scan pulang karena darurat',
                  border: OutlineInputBorder(),
                ),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: const Text('Batal'),
            ),
            ElevatedButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: const Text('Kirim Koreksi'),
            ),
          ],
        ),
      ),
    );
    if (ok != true || !mounted) return;
    if (reasonCtrl.text.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Alasan koreksi wajib diisi.')),
      );
      return;
    }
    try {
      await ref
          .read(securityApiClientProvider)
          .createCorrection(
            attendanceRecordId: record.id,
            requestType: 'status_change',
            requestedStatus: requestedStatus,
            reason: reasonCtrl.text.trim(),
          );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Pengajuan koreksi dikirim untuk persetujuan Ketua RT.',
          ),
        ),
      );
      await _loadData();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  @override
  Widget build(BuildContext context) {
    final currentUserId =
        ref.watch(authRepositoryProvider)?.valueOrNull?['id']?.toString() ?? '';

    return Scaffold(
      appBar: AppBar(
        title: const Text('Absensi Keamanan'),
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Absen & Jadwal'),
            Tab(text: 'Riwayat & Koreksi'),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'Muat Ulang',
            onPressed: _loadData,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.error_outline,
                      color: AppColors.danger,
                      size: 48,
                    ),
                    const SizedBox(height: 12),
                    Text(_error!, textAlign: TextAlign.center),
                    const SizedBox(height: 16),
                    ElevatedButton(
                      onPressed: _loadData,
                      child: const Text('Coba Lagi'),
                    ),
                  ],
                ),
              ),
            )
          : TabBarView(
              controller: _tabController,
              children: [
                Expanded(child: _buildAbsenAndScheduleTab(currentUserId)),
                Expanded(child: _buildHistoryAndCorrectionTab()),
              ],
            ),
    );
  }

  Widget _buildAbsenAndScheduleTab(String currentUserId) {
    final overview = _overview;
    final currentSchedule = overview?.currentSchedule;
    final currentRecord = overview?.currentRecord;
    final canCheckOut =
        currentRecord != null &&
        currentRecord.checkedInAt != null &&
        currentRecord.checkedOutAt == null &&
        (currentRecord.status == 'hadir' ||
            currentRecord.status == 'terlambat');
    final isDoneToday =
        currentRecord != null && currentRecord.status == 'sudah_absen_pulang';

    return RefreshIndicator(
      onRefresh: _loadData,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // Today Shift Card + QR Action
          Card(
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(14),
            ),
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      const Text(
                        'Shift Saat Ini',
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      if (currentRecord != null)
                        AttendanceStatusBadge(
                          statusCode: currentRecord.status,
                          statusLabel: currentRecord.statusLabel,
                          fontSize: 12,
                        ),
                    ],
                  ),
                  const SizedBox(height: 10),
                  if (currentSchedule == null)
                    const Text(
                      'Tidak ada shift aktif saat ini. Periksa jadwal mendatang di bawah.',
                      style: TextStyle(color: Colors.grey),
                    )
                  else ...[
                    Text(
                      '${currentSchedule.shiftName} · ${currentSchedule.shiftDate}',
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Jam Tugas: ${currentSchedule.startTime.substring(0, 5)} - ${currentSchedule.endTime.substring(0, 5)} WIB',
                    ),
                    if (currentRecord != null) ...[
                      const SizedBox(height: 8),
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              'Masuk: ${_formatTime(currentRecord.checkedInAt)}',
                            ),
                          ),
                          Expanded(
                            child: Text(
                              'Pulang: ${_formatTime(currentRecord.checkedOutAt)}',
                            ),
                          ),
                        ],
                      ),
                    ],
                    const SizedBox(height: 14),
                    if (!isDoneToday)
                      SizedBox(
                        width: double.infinity,
                        child: ElevatedButton.icon(
                          onPressed: () => _openScanner(
                            actionMode: canCheckOut ? 'check_out' : 'auto',
                          ),
                          icon: const Icon(Icons.qr_code_scanner),
                          label: Text(
                            canCheckOut
                                ? 'Scan QR untuk Check-Out'
                                : 'Scan QR untuk Check-In',
                          ),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: canCheckOut
                                ? AppColors.warning
                                : AppColors.accent,
                            foregroundColor: Colors.white,
                            padding: const EdgeInsets.symmetric(vertical: 12),
                          ),
                        ),
                      )
                    else
                      const Row(
                        children: [
                          Icon(Icons.check_circle, color: AppColors.success),
                          SizedBox(width: 8),
                          Text('Absensi shift ini telah selesai.'),
                        ],
                      ),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),

          // Incoming Swap Requests for Target
          if (_swaps.isNotEmpty) ...[
            const Text(
              'Permintaan Tukar Shift',
              style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            ..._swaps.map((sw) {
              final canRespondTarget =
                  sw.status == 'pending_target' &&
                  sw.targetUserId == currentUserId;
              return Card(
                margin: const EdgeInsets.only(bottom: 10),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '${sw.requesterName} (${sw.requesterShiftDate} · ${sw.requesterShiftName}) ↔ ${sw.targetName} (${sw.targetShiftDate} · ${sw.targetShiftName})',
                        style: const TextStyle(fontWeight: FontWeight.w600),
                      ),
                      const SizedBox(height: 4),
                      Text('Alasan: ${sw.reason}'),
                      const SizedBox(height: 4),
                      Text(
                        'Status: ${sw.status}',
                        style: const TextStyle(
                          fontSize: 12,
                          color: Colors.grey,
                        ),
                      ),
                      if (canRespondTarget) ...[
                        const SizedBox(height: 8),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.end,
                          children: [
                            OutlinedButton(
                              onPressed: () => _respondSwap(sw, false),
                              child: const Text('Tolak'),
                            ),
                            const SizedBox(width: 8),
                            ElevatedButton(
                              onPressed: () => _respondSwap(sw, true),
                              child: const Text('Setujui Tukar'),
                            ),
                          ],
                        ),
                      ],
                    ],
                  ),
                ),
              );
            }),
            const SizedBox(height: 12),
          ],

          // Upcoming Shifts
          const Text(
            'Jadwal Shift Mendatang',
            style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          if ((overview?.upcomingShifts ?? []).isEmpty)
            const Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('Belum ada jadwal mendatang yang ditetapkan.'),
              ),
            )
          else
            ...overview!.upcomingShifts.map(
              (s) => Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: ListTile(
                  leading: const Icon(Icons.calendar_today_outlined),
                  title: Text('${s.shiftDate} · ${s.shiftName}'),
                  subtitle: Text(
                    '${s.startTime.substring(0, 5)} - ${s.endTime.substring(0, 5)} WIB · Periode ${s.assignmentStartDate} – ${s.assignmentEndDate}',
                  ),
                  trailing: TextButton.icon(
                    onPressed: () => _openSwapDialog(s),
                    icon: const Icon(Icons.swap_horiz, size: 18),
                    label: const Text('Tukar'),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildHistoryAndCorrectionTab() {
    final history = [...?_overview?.recentHistory]
      ..sort(compareAttendanceRecency);
    return RefreshIndicator(
      onRefresh: _loadData,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          const Text(
            'Riwayat Absensi',
            style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          if (history.isEmpty)
            const Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('Belum ada riwayat absensi.'),
              ),
            )
          else
            ...history.map(
              (rec) => Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: ListTile(
                  title: Text('${rec.shiftDate} · ${rec.shiftName}'),
                  subtitle: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      AttendanceStatusBadge(
                        statusCode: rec.status,
                        statusLabel: rec.statusLabel,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        'Masuk: ${_formatTime(rec.checkedInAt)} | Pulang: ${_formatTime(rec.checkedOutAt)}',
                      ),
                    ],
                  ),
                  isThreeLine: true,
                  trailing: TextButton(
                    onPressed: () => _openCorrectionDialog(rec),
                    child: const Text('Koreksi'),
                  ),
                ),
              ),
            ),
          const SizedBox(height: 16),
          const Text(
            'Daftar Pengajuan Koreksi',
            style: TextStyle(fontSize: 15, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          if (_corrections.isEmpty)
            const Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('Belum ada pengajuan koreksi absensi.'),
              ),
            )
          else
            ..._corrections.map(
              (c) => Card(
                margin: const EdgeInsets.only(bottom: 8),
                child: ListTile(
                  title: Text('${c.shiftDate} · ${c.shiftName}'),
                  subtitle: Text(
                    'Usulan: ${c.requestedStatus} | Alasan: ${c.reason}\nStatus: ${c.status}${c.decisionNote != null ? ' (${c.decisionNote})' : ''}',
                  ),
                  isThreeLine: true,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
