import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/authenticated_network_image.dart';
import '../../data/security_models.dart';
import 'security_photo_viewer.dart';

class PaketDetailContent extends StatelessWidget {
  final PaketReport report;
  final bool isModal;

  const PaketDetailContent({
    super.key,
    required this.report,
    this.isModal = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return SingleChildScrollView(
      padding: EdgeInsets.symmetric(
        horizontal: isModal ? 16 : 16,
        vertical: isModal ? 8 : 16,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ── Header Summary Card ──
          Card(
            key: const Key('paket_detail_summary_card'),
            elevation: 0,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
              side: BorderSide(
                color: theme.colorScheme.outlineVariant.withValues(alpha: 0.5),
              ),
            ),
            color: isDark
                ? theme.colorScheme.surfaceContainer
                : theme.colorScheme.surfaceContainerLow,
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        padding: const EdgeInsets.all(10),
                        decoration: BoxDecoration(
                          color: AppColors.seed.withValues(alpha: 0.12),
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(
                          Icons.inventory_2,
                          color: AppColors.seed,
                          size: 26,
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              report.receiverName.isNotEmpty
                                  ? report.receiverName
                                  : 'Warga Penerima',
                              key: const Key('paket_detail_receiver_name'),
                              style: const TextStyle(
                                fontWeight: FontWeight.bold,
                                fontSize: 16,
                              ),
                            ),
                            if (report.receiverHouseNumber.isNotEmpty) ...[
                              const SizedBox(height: 2),
                              Text(
                                'Rumah: ${report.receiverHouseNumber}',
                                key: const Key('paket_detail_house_number'),
                                style: TextStyle(
                                  color: theme.colorScheme.onSurfaceVariant,
                                  fontSize: 13,
                                ),
                              ),
                            ],
                          ],
                        ),
                      ),
                      _buildOverallBadge(report.overallStatus),
                    ],
                  ),
                  const SizedBox(height: 14),
                  const Divider(height: 1),
                  const SizedBox(height: 12),
                  _buildDetailRow(
                    theme,
                    icon: Icons.access_time,
                    label: 'Waktu Diterima',
                    value: DateFormat('dd MMM yyyy, HH:mm').format(report.createdAt),
                  ),
                  const SizedBox(height: 8),
                  _buildDetailRow(
                    theme,
                    icon: Icons.security,
                    label: 'Petugas Penerima',
                    value: report.createdByName.isNotEmpty
                        ? report.createdByName
                        : 'Petugas Keamanan',
                  ),
                  const SizedBox(height: 8),
                  _buildDetailRow(
                    theme,
                    icon: Icons.how_to_reg,
                    label: 'Petugas Penyerah',
                    value: () {
                      final total = report.totalItems;
                      final handed = report.handedOverCount;

                      // Use report-level name if explicitly set.
                      if (report.handedOverByName?.isNotEmpty == true) {
                        if (total > 0 && handed == total) {
                          return report.handedOverByName!;
                        }
                        if (handed > 0) {
                          return report.handedOverByName!;
                        }
                        return 'Belum diserahkan';
                      }

                      // Derive from item-level handedOverByName by comparing all handed items.
                      if (handed > 0) {
                        final handedNames = report.items
                            .where((it) => it.status == 'sudah_diambil' || it.handedOverAt != null)
                            .map((it) => it.handedOverByName)
                            .where((name) => name?.isNotEmpty == true)
                            .toSet();

                        if (handedNames.isEmpty) {
                          return 'Data serah terima belum tersedia';
                        }
                        if (handedNames.length == 1) {
                          return handedNames.first!;
                        }
                        // Multiple different officers handed over different items.
                        return 'Beberapa Petugas';
                      }
                      return 'Belum diserahkan';
                    }(),
                  ),
                  if (report.overallStatus == 'sudah_diambil' && report.handedOverAt != null) ...[
                    const SizedBox(height: 8),
                    _buildDetailRow(
                      theme,
                      icon: Icons.event_available,
                      label: 'Waktu Diserahkan',
                      value: DateFormat('dd MMM yyyy, HH:mm').format(report.handedOverAt!),
                    ),
                  ],
                  const SizedBox(height: 8),
                  _buildDetailRow(
                    theme,
                    icon: Icons.format_list_numbered,
                    label: 'Total Titipan',
                    value: '${report.totalItems} Barang',
                  ),
                  if (report.items.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    _buildDetailRow(
                      theme,
                      icon: Icons.inventory_2_outlined,
                      label: 'Status Penyerahan',
                      value: '${report.handedOverCount} diserahkan, ${report.pendingCount} belum diserahkan',
                    ),
                  ],
                ],
              ),
            ),
          ),
          const SizedBox(height: 16),

          // ── Section Title: Daftar Barang ──
          Row(
            children: [
              const Icon(Icons.all_inbox, size: 18, color: AppColors.seed),
              const SizedBox(width: 8),
              Text(
                'Daftar Barang Titipan (${report.items.length})',
                style: const TextStyle(
                  fontWeight: FontWeight.bold,
                  fontSize: 14,
                ),
              ),
            ],
          ),
          const SizedBox(height: 10),

          // ── Items List ──
          if (report.items.isEmpty)
            Card(
              child: const Padding(
                padding: EdgeInsets.all(24),
                child: Center(
                  child: Text('Tidak ada rincian item paket.'),
                ),
              ),
            )
          else
            ...report.items.asMap().entries.map((entry) {
              final idx = entry.key;
              final it = entry.value;
              return _buildItemCard(context, theme, it, idx);
            }),
        ],
      ),
    );
  }

  Widget _buildOverallBadge(String status) {
    final isDiambil = status == 'sudah_diambil';
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: isDiambil
            ? AppColors.success.withValues(alpha: 0.15)
            : AppColors.warning.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(
          color: isDiambil
              ? AppColors.success.withValues(alpha: 0.4)
              : AppColors.warning.withValues(alpha: 0.4),
        ),
      ),
      child: Text(
        isDiambil ? 'Sudah Diambil' : 'Dititipkan',
        key: const Key('paket_detail_overall_status'),
        style: TextStyle(
          color: isDiambil ? AppColors.success : AppColors.warning,
          fontSize: 12,
          fontWeight: FontWeight.bold,
        ),
      ),
    );
  }

  Widget _buildDetailRow(
    ThemeData theme, {
    required IconData icon,
    required String label,
    required String value,
  }) {
    return Row(
      children: [
        Icon(icon, size: 16, color: theme.colorScheme.onSurfaceVariant),
        const SizedBox(width: 8),
        Text(
          '$label: ',
          style: TextStyle(
            fontSize: 12,
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: const TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
            ),
          ),
        ),
      ],
    );
  }

  /// Parses description text separated by " · " into individual lines.
  /// If the description contains labels like "Kurir:", "Resi:", etc., each
  /// becomes its own line. Otherwise the entire description is shown as one line.
  List<Widget> _buildItemDescriptionLines(PaketItem it, ThemeData theme) {
    final raw = it.description?.trim();
    if (raw == null || raw.isEmpty) {
      return [
        Text(
          'Paket / Titipan',
          key: Key('paket_item_desc_${it.id}'),
          style: const TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
        ),
      ];
    }

    // Split on " · " separator if present
    final parts = raw.split(' · ').map((s) => s.trim()).where((s) => s.isNotEmpty).toList();

    if (parts.length <= 1) {
      // No separator — show as single line
      return [
        Text(
          raw,
          key: Key('paket_item_desc_${it.id}'),
          style: const TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
        ),
      ];
    }

    // Multiple parts — render each on its own line
    final widgets = <Widget>[];
    for (var i = 0; i < parts.length; i++) {
      if (i > 0) widgets.add(const SizedBox(height: 2));
      final part = parts[i];
      widgets.add(
        Text(
          part,
          key: i == 0 ? Key('paket_item_desc_${it.id}') : null,
          style: i == 0
              ? const TextStyle(fontSize: 14, fontWeight: FontWeight.bold)
              : TextStyle(fontSize: 13, color: theme.colorScheme.onSurfaceVariant),
        ),
      );
    }
    return widgets;
  }

  Widget _buildItemCard(
    BuildContext context,
    ThemeData theme,
    PaketItem it,
    int index,
  ) {
    final isDititipkan = it.status == 'dititipkan';
    final hours = DateTime.now().difference(it.createdAt).inHours;
    final isOverdue = isDititipkan && hours >= 24;

    return Card(
      key: Key('paket_item_card_${it.id}'),
      margin: const EdgeInsets.only(bottom: 12),
      elevation: 0,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(10),
        side: BorderSide(
          color: theme.colorScheme.outlineVariant.withValues(alpha: 0.4),
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Barang #${index + 1}',
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: theme.colorScheme.primary,
                        ),
                      ),
                      const SizedBox(height: 2),
                      ..._buildItemDescriptionLines(it, theme),
                    ],
                  ),
                ),
                if (isOverdue)
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: AppColors.danger.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: Text(
                      '> 24 Jam ($hours jam)',
                      style: const TextStyle(
                        color: AppColors.danger,
                        fontSize: 11,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  )
                else if (isDititipkan)
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: AppColors.warning.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: const Text(
                      'Dititipkan',
                      style: TextStyle(
                        color: AppColors.warning,
                        fontSize: 11,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  )
                else
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: AppColors.success.withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: const Text(
                      'Sudah Diambil',
                      style: TextStyle(
                        color: AppColors.success,
                        fontSize: 11,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
              ],
            ),

            if (it.status == 'sudah_diambil') ...[
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                decoration: BoxDecoration(
                  color: AppColors.success.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Row(
                  children: [
                    const Icon(
                      Icons.check_circle_outline,
                      size: 15,
                      color: AppColors.success,
                    ),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(
                        it.handedOverByName?.isNotEmpty == true
                            ? 'Diserahkan oleh: ${it.handedOverByName!}${it.handedOverAt != null ? ' · ${DateFormat('dd MMM yyyy, HH:mm').format(it.handedOverAt!)}' : ''}'
                            : (it.statusChangedAt != null
                                ? 'Diambil: ${DateFormat('dd MMM yyyy, HH:mm').format(it.statusChangedAt!)}${it.changedByName?.isNotEmpty == true ? ' · Petugas: ${it.changedByName}' : ''}'
                                : 'Data serah terima belum tersedia'),
                        style: const TextStyle(
                          fontSize: 11,
                          color: AppColors.success,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ] else ...[
              const SizedBox(height: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                decoration: BoxDecoration(
                  color: AppColors.warning.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: const Row(
                  children: [
                    Icon(
                      Icons.schedule,
                      size: 15,
                      color: AppColors.warning,
                    ),
                    SizedBox(width: 6),
                    Text(
                      'Status: Belum diserahkan',
                      style: TextStyle(
                        fontSize: 11,
                        color: AppColors.warning,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ],
                ),
              ),
            ],

            // Photos section
            if (it.photos.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text(
                'Foto (${it.photos.length}):',
                style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.w600,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 6),
              SizedBox(
                height: 72,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: it.photos.length,
                  separatorBuilder: (_, _) => const SizedBox(width: 8),
                  itemBuilder: (ctx, pIdx) {
                    final photo = it.photos[pIdx];
                    final isBukti = photo.photoType == 'bukti_pengambilan';

                    return GestureDetector(
                      key: Key('paket_photo_thumb_${photo.id}'),
                      onTap: () {
                        final viewerItems = it.photos
                            .where((p) => p.imageUrl.isNotEmpty)
                            .map(
                              (p) => SecurityPhotoViewerItem(
                                imageUrl: p.imageUrl,
                                title: it.description?.isNotEmpty == true
                                    ? it.description!
                                    : 'Paket',
                                subtitle: p.photoType == 'bukti_pengambilan'
                                    ? 'Bukti Pengambilan'
                                    : 'Foto Paket',
                              ),
                            )
                            .toList();
                        if (viewerItems.isNotEmpty) {
                          SecurityPhotoViewerScreen.show(
                            context,
                            photos: viewerItems,
                            initialIndex: pIdx < viewerItems.length ? pIdx : 0,
                          );
                        }
                      },
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(8),
                        child: Stack(
                          children: [
                            Container(
                              width: 72,
                              height: 72,
                              color: theme.colorScheme.surfaceContainerHighest,
                              child: photo.imageUrl.isNotEmpty
                                  ? AuthenticatedNetworkImage(
                                      imageUrl: photo.imageUrl,
                                      fit: BoxFit.cover,
                                    )
                                  : const Icon(
                                      Icons.image_not_supported,
                                      size: 24,
                                    ),
                            ),
                            if (isBukti)
                              Positioned(
                                bottom: 0,
                                left: 0,
                                right: 0,
                                child: Container(
                                  padding: const EdgeInsets.symmetric(
                                    vertical: 2,
                                    horizontal: 4,
                                  ),
                                  color: Colors.black.withValues(alpha: 0.65),
                                  child: const Text(
                                    'Bukti',
                                    textAlign: TextAlign.center,
                                    style: TextStyle(
                                      color: Colors.white,
                                      fontSize: 9,
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

