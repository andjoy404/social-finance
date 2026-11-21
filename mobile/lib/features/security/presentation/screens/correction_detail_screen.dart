import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import 'package:social_finance/core/theme/app_colors.dart';

import '../../data/security_models.dart';
import '../providers/security_providers.dart';

class CorrectionDetailScreen extends ConsumerWidget {
  final AttendanceCorrection correction;

  const CorrectionDetailScreen({super.key, required this.correction});

  String _formatDateTime(DateTime dt) {
    return DateFormat('dd MMM yyyy, HH:mm').format(dt);
  }

  String _formatDate(String dateStr) {
    if (dateStr.isEmpty) return '-';
    try {
      final parts = dateStr.split('-');
      if (parts.length == 3) {
        return DateFormat('dd MMM yyyy').format(
          DateTime(int.parse(parts[0]), int.parse(parts[1]), int.parse(parts[2])),
        );
      }
    } catch (_) {}
    return dateStr;
  }

  String _correctionTypeLabel(String type) {
    switch (type) {
      case 'status_change':
        return 'Perubahan Status';
      case 'time_adjustment':
        return 'Penyesuaian Jam';
      case 'manual_check_in':
        return 'Check-In Manual';
      case 'manual_check_out':
        return 'Check-Out Manual';
      default:
        return type;
    }
  }

  String _attendanceStatusLabel(String status) {
    switch (status) {
      case 'hadir':
        return 'Hadir';
      case 'terlambat':
        return 'Terlambat';
      case 'sudah_absen_pulang':
        return 'Sudah Absen Pulang';
      case 'tidak_hadir':
        return 'Tidak Hadir';
      case 'belum_hadir':
        return 'Belum Hadir';
      case 'belum_absen_pulang':
        return 'Belum Absen Pulang';
      default:
        return status;
    }
  }

  Color _statusColor(String status) {
    switch (status) {
      case 'hadir':
        return AppColors.success;
      case 'terlambat':
        return AppColors.warning;
      case 'sudah_absen_pulang':
        return AppColors.seed;
      case 'tidak_hadir':
        return AppColors.danger;
      default:
        return Colors.grey;
    }
  }

  Widget _statusBadge(String status) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: _statusColor(status).withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        _attendanceStatusLabel(status),
        style: TextStyle(
          color: _statusColor(status),
          fontSize: 11,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  Widget _decisionBadge(String status) {
    final isApp = status == 'approved';
    final isPending = status == 'pending';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: (isPending ? AppColors.warning : (isApp ? AppColors.success : AppColors.danger))
            .withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        isPending ? 'Menunggu' : (isApp ? 'Disetujui' : 'Ditolak'),
        style: TextStyle(
          color: isPending ? AppColors.warning : (isApp ? AppColors.success : AppColors.danger),
          fontSize: 10,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }

  Future<void> _decideCorrection(BuildContext context, WidgetRef ref, bool approve) async {
    final note = await showDialog<String?>(
      context: context,
      builder: (ctx) {
        final controller = TextEditingController();
        return AlertDialog(
          title: Text(approve ? 'Setujui Koreksi' : 'Tolak Koreksi'),
          content: TextField(
            controller: controller,
            decoration: const InputDecoration(labelText: 'Catatan keputusan'),
            maxLines: 3,
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Batal'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, controller.text.trim()),
              child: const Text('Konfirmasi'),
            ),
          ],
        );
      },
    );
    if (note == null) return;

    try {
      final api = ref.read(securityApiClientProvider);
      await api.decideCorrection(correction.id, approve: approve, note: note);
      if (context.mounted) {
        Navigator.pop(context, correction);
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString())),
        );
      }
    }
  }

  Widget _infoSection(BuildContext context, String title, IconData icon, List<Widget> children) {
    final theme = Theme.of(context);
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
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
              Icon(icon, size: 14, color: AppColors.seed),
              const SizedBox(width: 6),
              Text(
                title,
                style: const TextStyle(
                  fontWeight: FontWeight.w700,
                  fontSize: 12,
                  color: AppColors.seed,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          ...children,
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final c = correction;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Detail Koreksi'),
        elevation: 0,
      ),
      body: ListView(
        padding: const EdgeInsets.all(12),
        children: [
          // Header: Petugas, Tanggal, Status
          Card(
            margin: const EdgeInsets.only(bottom: 12),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          c.officerName,
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                      _decisionBadge(c.status),
                    ],
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${_formatDate(c.shiftDate)} · ${c.shiftName}',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Jenis: ${_correctionTypeLabel(c.requestType)}',
                    style: const TextStyle(fontSize: 12),
                  ),
                ],
              ),
            ),
          ),

          // Perubahan Status
          _infoSection(context,
            'Perubahan Status',
            Icons.swap_horiz_outlined,
            [
              Row(
                children: [
                  _statusBadge(c.originalStatus),
                  const Padding(
                    padding: EdgeInsets.symmetric(horizontal: 8),
                    child: Icon(Icons.arrow_forward, size: 16),
                  ),
                  _statusBadge(c.requestedStatus),
                ],
              ),
            ],
          ),

          const SizedBox(height: 12),

          // Blok Diajukan
          _infoSection(context,
            'Diajukan',
            Icons.send_outlined,
            [
              Text(
                'Alasan: ${c.reason}',
                style: const TextStyle(fontSize: 12),
              ),
              const SizedBox(height: 4),
              Text(
                'Oleh: ${c.requestedBy}',
                style: const TextStyle(fontSize: 12),
              ),
              if (c.createdAt != null) ...[
                const SizedBox(height: 2),
                Text(
                  'Waktu: ${_formatDateTime(c.createdAt!)}',
                  style: TextStyle(
                    fontSize: 11,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
            ],
          ),

          const SizedBox(height: 12),

          // Blok Keputusan
          _infoSection(context,
            'Keputusan',
            c.status == 'pending'
                ? Icons.hourglass_empty
                : (c.status == 'approved' ? Icons.check_circle_outline : Icons.cancel_outlined),
            [
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
                    'Diputuskan: ${_formatDateTime(c.decidedAt!)}',
                    style: TextStyle(
                      fontSize: 11,
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ],
            ],
          ),

          const SizedBox(height: 16),

          // Action buttons (only for pending corrections)
          if (c.status == 'pending')
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    icon: const Icon(Icons.close, color: AppColors.danger),
                    label: const Text('Tolak'),
                    style: OutlinedButton.styleFrom(
                      foregroundColor: AppColors.danger,
                      side: const BorderSide(color: AppColors.danger),
                    ),
                    onPressed: () => _decideCorrection(context, ref, false),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton.icon(
                    icon: const Icon(Icons.check),
                    label: const Text('Setujui'),
                    onPressed: () => _decideCorrection(context, ref, true),
                  ),
                ),
              ],
            ),
        ],
      ),
    );
  }
}
