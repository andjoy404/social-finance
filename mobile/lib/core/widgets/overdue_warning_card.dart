import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_providers.dart';

/// Global overdue warning for plain residents. The summary stays inline while
/// the full balance and navigation action are shown in a dismissible dialog.
class OverdueWarningCard extends ConsumerStatefulWidget {
  const OverdueWarningCard({super.key});

  @override
  ConsumerState<OverdueWarningCard> createState() =>
      _OverdueWarningCardState();
}

class _OverdueWarningCardState extends ConsumerState<OverdueWarningCard>
    with WidgetsBindingObserver {
  String? _observedUserId;
  int _promptGeneration = 0;
  bool _dialogOpen = false;
  bool _wasBackgrounded = false;
  bool _initialPromptQueued = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _initialPromptQueued) return;
      _initialPromptQueued = true;
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final id = (user?['id'] ?? '').toString();
      if (id.isNotEmpty && isPlainWargaUser(user)) {
        _observedUserId = id;
        _queueAutoPrompt();
      }
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _promptGeneration++;
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    super.didChangeAppLifecycleState(state);
    if (state == AppLifecycleState.inactive ||
        state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached) {
      _wasBackgrounded = true;
      return;
    }
    if (state == AppLifecycleState.resumed && _wasBackgrounded) {
      _wasBackgrounded = false;
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      if (isPlainWargaUser(user)) {
        _queueAutoPrompt(refreshSummary: true);
      }
    }
  }

  void _queueAutoPrompt({bool refreshSummary = false}) {
    final generation = ++_promptGeneration;
    Future<void>(() async {
      if (refreshSummary) {
        await ref
            .read(overdueWarningSummaryProvider.notifier)
            .refresh(throttled: true);
      } else {
        await ref.read(overdueWarningSummaryProvider.future);
      }
      if (!mounted || generation != _promptGeneration || _dialogOpen) return;
      final user = ref.read(authRepositoryProvider)?.valueOrNull;
      final summary = ref.read(overdueWarningSummaryProvider).valueOrNull;
      final userId = (user?['id'] ?? '').toString();
      if (userId.isEmpty ||
          userId != _observedUserId ||
          !isPlainWargaUser(user) ||
          !_hasOverdue(summary)) {
        return;
      }
      await _showDetailsDialog(summary!);
    });
  }

  bool _hasOverdue(OverdueWarningSummary? summary) =>
      summary != null &&
      summary.hasOverdue3Months &&
      summary.totalOutstanding > 0 &&
      summary.overdueBillCount > 0;

  Future<void> _showDetailsDialog(OverdueWarningSummary summary) async {
    if (!mounted || _dialogOpen) return;
    _dialogOpen = true;
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    const amberPrimary = Color(0xFFD97706);
    const amberDarkText = Color(0xFFB45309);
    try {
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          key: const Key('overdue-warning-detail-dialog'),
          titlePadding: const EdgeInsets.fromLTRB(20, 14, 8, 0),
          title: Row(
            children: [
              const Expanded(
                child: Text(
                  'Peringatan Tunggakan Iuran',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              IconButton(
                key: const Key('overdue-warning-dialog-close'),
                tooltip: 'Tutup',
                onPressed: () => Navigator.of(dialogContext).pop(),
                icon: const Icon(Icons.close),
              ),
            ],
          ),
          contentPadding: const EdgeInsets.fromLTRB(24, 8, 24, 16),
          content: SingleChildScrollView(
            clipBehavior: Clip.antiAlias,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
                  decoration: BoxDecoration(
                    color: amberPrimary.withValues(alpha: 0.16),
                    borderRadius: BorderRadius.circular(999),
                    border: Border.all(color: amberPrimary.withValues(alpha: 0.35)),
                  ),
                  child: Text(
                    'Tunggakan 3 Bulan+',
                    style: theme.textTheme.labelMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                      color: isDark ? AppColors.warning : amberDarkText,
                    ),
                  ),
                ),
                const SizedBox(height: 14),
                const Text(
                  'Anda memiliki tagihan iuran yang belum lunas selama 3 bulan atau lebih. Mohon segera lakukan pembayaran agar kewajiban iuran tidak semakin menumpuk.',
                  maxLines: 4,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 16),
                Text(
                  'Total Tunggakan: ${RupiahFormatter.formatIdr(summary.totalOutstanding)}',
                  style: theme.textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 4),
                Text('Jumlah tagihan: ${summary.overdueBillCount}'),
              ],
            ),
          ),
          actions: [
            FilledButton.icon(
              key: const Key('overdue-warning-action-btn'),
              onPressed: () {
                Navigator.of(dialogContext).pop();
                context.go('/home/iuran');
              },
              icon: const Icon(Icons.arrow_forward),
              label: const Text('Lihat Tagihan Saya'),
            ),
          ],
        ),
      );
    } finally {
      _dialogOpen = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(authRepositoryProvider, (previous, next) {
      final user = next?.valueOrNull;
      final userId = (user?['id'] ?? '').toString();
      if (userId.isEmpty) {
        _observedUserId = null;
        _promptGeneration++;
        return;
      }
      if (userId != _observedUserId) {
        _observedUserId = userId;
        if (isPlainWargaUser(user)) _queueAutoPrompt();
      }
    });

    final user = ref.watch(authRepositoryProvider)?.valueOrNull;
    if (!isPlainWargaUser(user)) return const SizedBox.shrink();

    final summary = ref.watch(overdueWarningSummaryProvider).valueOrNull;
    if (!_hasOverdue(summary)) return const SizedBox.shrink();

    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
    const amberPrimary = Color(0xFFD97706);
    const amberDarkText = Color(0xFFB45309);

    return Material(
      color: Colors.transparent,
      child: InkWell(
        key: const Key('global-overdue-warning-card'),
        borderRadius: BorderRadius.circular(AppRadius.base),
        onTap: () => _showDetailsDialog(summary!),
        child: Container(
          width: double.infinity,
          margin: const EdgeInsets.fromLTRB(12, 8, 12, 4),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: isDark
                ? AppColors.warning.withValues(alpha: 0.14)
                : const Color(0xFFFFFBEB),
            borderRadius: BorderRadius.circular(AppRadius.base),
            border: Border.all(
              color: isDark
                  ? AppColors.warning.withValues(alpha: 0.45)
                  : const Color(0xFFF59E0B).withValues(alpha: 0.55),
            ),
          ),
          child: Row(
            children: [
              const Icon(
                Icons.warning_amber_rounded,
                key: Key('overdue-warning-inline-icon'),
                size: 20,
                color: amberPrimary,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Peringatan Tunggakan Iuran',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.w700,
                    fontSize: 13,
                    color: isDark ? AppColors.darkText : AppColors.lightText,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                decoration: BoxDecoration(
                  color: amberPrimary.withValues(alpha: 0.16),
                  borderRadius: BorderRadius.circular(999),
                  border: Border.all(color: amberPrimary.withValues(alpha: 0.35)),
                ),
                child: Text(
                  'Tunggakan 3 Bulan+',
                  style: theme.textTheme.labelSmall?.copyWith(
                    fontWeight: FontWeight.w700,
                    fontSize: 10.5,
                    color: isDark ? AppColors.warning : amberDarkText,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
