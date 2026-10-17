import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_access.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';
import 'package:social_finance/features/iuran/presentation/screens/iuran_menu_screen.dart';

class IuranManagementScreen extends ConsumerWidget {
  const IuranManagementScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    final profileStatus = user?['profile_status'] as String? ?? 'ready';
    if (profileStatus != 'ready') {
      return IuranProfileStatusScreen(status: profileStatus);
    }
    if (!canManageIuran(user)) {
      return const Scaffold(
        body: Center(
          child: Text('Anda tidak memiliki akses untuk mengelola iuran.'),
        ),
      );
    }

    final duesState = ref.watch(duesProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Kelola Iuran'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.go('/home/iuran'),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push('/home/iuran/form'),
        icon: const Icon(Icons.add),
        label: const Text('Buat Iuran'),
      ),
      body: duesState.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('Gagal memuat daftar iuran: $error'),
                const SizedBox(height: AppSpacing.md),
                FilledButton(
                  onPressed: () => ref.read(duesProvider.notifier).refresh(),
                  child: const Text('Coba Lagi'),
                ),
              ],
            ),
          ),
        ),
        data: (dues) => dues.isEmpty
            ? const Center(child: Text('Belum ada jenis iuran.'))
            : RefreshIndicator(
                onRefresh: () => ref.read(duesProvider.notifier).refresh(),
                child: ListView.separated(
                  padding: const EdgeInsets.fromLTRB(
                    AppSpacing.base,
                    AppSpacing.base,
                    AppSpacing.base,
                    96,
                  ),
                  itemCount: dues.length,
                  separatorBuilder: (_, _) =>
                      const SizedBox(height: AppSpacing.sm),
                  itemBuilder: (context, index) {
                    final due = dues[index];
                    return Card(
                      child: ListTile(
                        title: Text(due.name),
                        subtitle: Text(
                          '${due.isVoluntary ? 'Sukarela' : formatRupiahString(due.amount)}'
                          ' · ${_periodLabel(due.periodType)}'
                          ' · ${due.isActive ? 'Aktif' : 'Nonaktif'}',
                        ),
                        trailing: Switch(
                          value: due.isActive,
                          onChanged: (_) => _setActive(context, ref, due),
                        ),
                      ),
                    );
                  },
                ),
              ),
      ),
    );
  }

  Future<void> _setActive(
    BuildContext context,
    WidgetRef ref,
    BackendDue due,
  ) async {
    try {
      await ref
          .read(iuranRepositoryProvider)
          .updateDue(due.id, isActive: !due.isActive);
      await ref.read(duesProvider.notifier).refresh();
    } catch (error) {
      if (context.mounted) {
        final message = ref.read(iuranRepositoryProvider).mapDioError(error);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(message.message)));
      }
    }
  }

  String _periodLabel(DuePeriodType periodType) => switch (periodType) {
    DuePeriodType.monthly => 'Bulanan',
    DuePeriodType.yearly => 'Tahunan',
    DuePeriodType.oneTime => 'Sekali',
  };
}
