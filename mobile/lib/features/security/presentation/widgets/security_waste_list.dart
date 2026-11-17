import 'package:flutter/material.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/empty_state.dart';
import 'package:social_finance/core/widgets/error_state.dart';
import 'package:social_finance/core/widgets/loading_indicator.dart';
import 'package:social_finance/features/monitoring/data/monitoring_models.dart';

/// Shared read-only waste-monitoring list used by the officer and paired POS.
class SecurityWasteList extends StatelessWidget {
  final List<MonitoringHousehold> households;
  final String evalDate;
  final bool loading;
  final String? error;
  final Future<void> Function() onRetry;

  /// Optional trailing action for each row. When non-null, an edit button is
  /// rendered and tapping it invokes [onNoteEdit] with the tapped row.
  final void Function(MonitoringHousehold)? onNoteEdit;

  const SecurityWasteList({
    super.key,
    required this.households,
    required this.evalDate,
    required this.loading,
    required this.error,
    required this.onRetry,
    this.onNoteEdit,
  });

  @override
  Widget build(BuildContext context) {
    if (loading && households.isEmpty) {
      return const Center(child: LoadingIndicator());
    }
    if (error != null && households.isEmpty) {
      return ErrorState(
        message: error!,
        onRetry: onRetry,
        actionText: 'Coba Lagi',
      );
    }
    if (households.isEmpty) {
      return const EmptyState(
        icon: Icons.home_outlined,
        title: 'Tidak Ada Rumah dalam Monitoring',
        description: 'Belum ada rumah yang memenuhi kriteria monitoring.',
      );
    }
    return RefreshIndicator(
      onRefresh: onRetry,
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        itemCount: households.length + 1,
        itemBuilder: (context, index) {
          if (index == 0) {
            return Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Text(
                evalDate.isEmpty
                    ? 'Daftar Sampah Belum Diangkut'
                    : 'Daftar Sampah Belum Diangkut ($evalDate)',
                style: const TextStyle(fontWeight: FontWeight.bold),
              ),
            );
          }
          final item = households[index - 1];
          return Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: const Icon(
                Icons.home_outlined,
                color: AppColors.warning,
              ),
              title: Text('Rumah No. ${item.houseNumber}'),
              subtitle: item.note != null && item.note!.trim().isNotEmpty
                  ? Text(item.note!)
                  : const Text('Belum tercatat diangkut'),
              trailing: onNoteEdit == null
                  ? null
                  : IconButton(
                      icon: const Icon(Icons.edit_outlined),
                      color: AppColors.accent,
                      tooltip: 'Ubah Catatan',
                      onPressed: () => onNoteEdit!(item),
                    ),
            ),
          );
        },
      ),
    );
  }
}
