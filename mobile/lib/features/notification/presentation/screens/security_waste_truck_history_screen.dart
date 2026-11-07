import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/jakarta_time_utils.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/loading_indicator.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';

class SecurityWasteTruckHistoryScreen extends ConsumerStatefulWidget {
  const SecurityWasteTruckHistoryScreen({super.key});

  @override
  ConsumerState<SecurityWasteTruckHistoryScreen> createState() =>
      _SecurityWasteTruckHistoryScreenState();
}

class _SecurityWasteTruckHistoryScreenState
    extends ConsumerState<SecurityWasteTruckHistoryScreen> {
  final List<WasteTruckDispatchEntry> _entries = [];
  bool _isLoading = true;
  bool _isLoadingMore = false;
  String? _error;
  int _page = 1;
  int _total = 0;
  String? _currentUserId;

  @override
  void initState() {
    super.initState();
    final user = ref.read(authRepositoryProvider)?.valueOrNull;
    _currentUserId = user?['id'] as String?;
    _loadHistory();
  }

  Future<void> _loadHistory({bool loadMore = false}) async {
    if (loadMore && (_isLoadingMore || _entries.length >= _total)) return;
    setState(() {
      _error = null;
      if (loadMore) {
        _isLoadingMore = true;
      } else {
        _isLoading = true;
        _page = 1;
      }
    });

    try {
      final nextPage = loadMore ? _page + 1 : 1;
      final response = await ref
          .read(notificationRepositoryProvider)
          .fetchWasteTruckHistory(page: nextPage);
      if (!mounted) return;
      setState(() {
        if (!loadMore) _entries.clear();
        _entries.addAll(response.data);
        _page = response.page;
        _total = response.total;
        _isLoading = false;
        _isLoadingMore = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        _isLoadingMore = false;
        _error = error is ServerException
            ? error.message
            : 'Riwayat pengiriman gagal dimuat.';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<AsyncValue<Map<String, dynamic>?>?>(authRepositoryProvider, (previous, next) {
      final user = next?.valueOrNull;
      final newUserId = user?['id'] as String?;
      if (_currentUserId != newUserId) {
        _currentUserId = newUserId;
        _entries.clear();
        _page = 1;
        _total = 0;
        if (newUserId != null) {
          _loadHistory();
        } else {
          setState(() {
            _isLoading = false;
            _error = null;
          });
        }
      }
    });

    if (_isLoading && _entries.isEmpty) {
      return const Center(child: LoadingIndicator());
    }
    if (_error != null && _entries.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(
                Icons.error_outline,
                color: AppColors.danger,
                size: 40,
              ),
              const SizedBox(height: 12),
              Text(_error!, textAlign: TextAlign.center),
              const SizedBox(height: 12),
              TextButton(
                onPressed: () => _loadHistory(),
                child: const Text('Coba lagi'),
              ),
            ],
          ),
        ),
      );
    }
    if (_entries.isEmpty) {
      return RefreshIndicator(
        onRefresh: _loadHistory,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          children: const [
            SizedBox(height: 100),
            Icon(Icons.history, size: 48),
            SizedBox(height: 12),
            Center(child: Text('Belum ada riwayat pengiriman dari akun ini.')),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadHistory,
      child: ListView.builder(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.all(16),
        itemCount: _entries.length + (_entries.length < _total ? 1 : 0),
        itemBuilder: (context, index) {
          if (index == _entries.length) {
            return Center(
              child: TextButton(
                onPressed: _isLoadingMore
                    ? null
                    : () => _loadHistory(loadMore: true),
                child: _isLoadingMore
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Muat riwayat lainnya'),
              ),
            );
          }
          final entry = _entries[index];
          return _DispatchCard(
            entry: entry,
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) => _DispatchDetailScreen(entry: entry),
              ),
            ),
          );
        },
      ),
    );
  }
}

class _DispatchCard extends StatelessWidget {
  final WasteTruckDispatchEntry entry;
  final VoidCallback onTap;

  const _DispatchCard({required this.entry, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final (label, color, icon) = switch (entry.status.toUpperCase()) {
      'SENT' => ('Terkirim', AppColors.success, Icons.check_circle_outline),
      'PARTIAL' => (
        'Sebagian terkirim',
        AppColors.warning,
        Icons.warning_amber,
      ),
      'FAILED' => ('Gagal', AppColors.danger, Icons.error_outline),
      'NO_DEVICE' => (
        'Perangkat tidak tersedia',
        AppColors.warning,
        Icons.sim_card_alert,
      ),
      'UNKNOWN' => (
        'Status historis tidak tersedia',
        Theme.of(context).colorScheme.onSurfaceVariant,
        Icons.help_outline,
      ),
      _ => ('Diproses', Theme.of(context).colorScheme.primary, Icons.schedule),
    };
    final resolvedImage = entry.imageUrl == null
        ? null
        : ApiConfig.resolveImageUrl(entry.imageUrl!);

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(12),
          child: AppCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    entry.title.isEmpty
                        ? 'Kedatangan Truk Sampah'
                        : entry.title,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
                Icon(icon, size: 18, color: color),
                const SizedBox(width: 6),
                Text(
                  label,
                  style: TextStyle(color: color, fontWeight: FontWeight.w600),
                ),
              ],
            ),
            if (entry.createdAt != null) ...[
              const SizedBox(height: 6),
              Text(
                formatJakartaDateTime(entry.createdAt),
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
                if (entry.body.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(entry.body, maxLines: 2, overflow: TextOverflow.ellipsis),
                ],
                if (resolvedImage != null) ...[
                  const SizedBox(height: 12),
                  ClipRRect(
                    borderRadius: BorderRadius.circular(8),
                    child: Image.network(
                      resolvedImage,
                      height: 150,
                      width: double.infinity,
                      fit: BoxFit.cover,
                      errorBuilder: (_, _, _) => const SizedBox.shrink(),
                    ),
                  ),
                ],
                if (entry.failureReason?.isNotEmpty == true) ...[
                  const SizedBox(height: 10),
                  Text(
                    entry.failureReason!,
                    style: TextStyle(color: color, fontSize: 13),
                  ),
                ],
                const SizedBox(height: 8),
                Text(
                  'Ketuk untuk melihat detail',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.primary,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  String _formatTimestamp(DateTime time) {
    return formatJakartaDateTime(time);
  }
}

class _DispatchDetailScreen extends StatelessWidget {
  final WasteTruckDispatchEntry entry;

  const _DispatchDetailScreen({required this.entry});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final status = switch (entry.status.toUpperCase()) {
      'SENT' => ('Terkirim ke provider', AppColors.success),
      'PARTIAL' => ('Sebagian gagal', AppColors.warning),
      'FAILED' => ('Gagal', AppColors.danger),
      'NO_DEVICE' => ('Perangkat tidak tersedia', AppColors.warning),
      'UNKNOWN' => ('Status tidak tersedia', theme.colorScheme.onSurfaceVariant),
      _ => ('Diproses', theme.colorScheme.primary),
    };
    final resolvedImage = entry.imageUrl == null
        ? null
        : ApiConfig.resolveImageUrl(entry.imageUrl!);

    return Scaffold(
      appBar: AppBar(title: const Text('Detail Pengiriman')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Text(
            entry.title.isEmpty ? 'Kedatangan Truk Sampah' : entry.title,
            style: theme.textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text('Event: ${entry.eventType}'),
          if (entry.createdAt != null) ...[
            const SizedBox(height: 8),
            Text('Waktu: ${formatJakartaDateTime(entry.createdAt)}'),
          ],
          const SizedBox(height: 8),
          Text('Status: ${status.$1}', style: TextStyle(color: status.$2)),
          const SizedBox(height: 16),
          Text('Pesan', style: theme.textTheme.titleMedium),
          const SizedBox(height: 4),
          Text(entry.body.isNotEmpty ? entry.body : 'Tidak ada pesan tambahan.'),
          if (entry.failureReason?.isNotEmpty == true) ...[
            const SizedBox(height: 16),
            Text('Informasi kegagalan', style: theme.textTheme.titleMedium),
            const SizedBox(height: 4),
            Text(entry.failureReason!, style: const TextStyle(color: AppColors.danger)),
          ],
          const SizedBox(height: 20),
          Text('Foto truk sampah', style: theme.textTheme.titleMedium),
          const SizedBox(height: 8),
          if (resolvedImage == null)
            const _ImageUnavailable(message: 'Tidak ada foto pada pengiriman ini.')
          else
            InkWell(
              onTap: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => _FullImageScreen(imageUrl: resolvedImage),
                ),
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: Image.network(
                  resolvedImage,
                  height: 240,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  loadingBuilder: (context, child, progress) {
                    if (progress == null) return child;
                    return const SizedBox(
                      height: 240,
                      child: Center(child: CircularProgressIndicator()),
                    );
                  },
                  errorBuilder: (_, _, _) => const _ImageUnavailable(
                    message: 'Foto gagal dimuat atau URL tidak valid.',
                  ),
                ),
              ),
            ),
          if (resolvedImage != null) ...[
            const SizedBox(height: 6),
            Text(
              'Ketuk foto untuk memperbesar dan menggeser.',
              style: theme.textTheme.bodySmall,
            ),
          ],
          const SizedBox(height: 20),
          Text(
            'Jumlah penerima dan rincian tiap perangkat tidak tersedia pada data dispatch.',
            style: theme.textTheme.bodySmall,
          ),
        ],
      ),
    );
  }
}

class _FullImageScreen extends StatelessWidget {
  final String imageUrl;

  const _FullImageScreen({required this.imageUrl});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: const Text('Foto Pengiriman'),
      ),
      body: Center(
        child: InteractiveViewer(
          minScale: 0.8,
          maxScale: 5,
          child: Image.network(
            imageUrl,
            fit: BoxFit.contain,
            loadingBuilder: (context, child, progress) => progress == null
                ? child
                : const CircularProgressIndicator(color: Colors.white),
            errorBuilder: (_, _, _) => const _ImageUnavailable(
              message: 'Foto tidak dapat dimuat.',
              dark: true,
            ),
          ),
        ),
      ),
    );
  }
}

class _ImageUnavailable extends StatelessWidget {
  final String message;
  final bool dark;

  const _ImageUnavailable({required this.message, this.dark = false});

  @override
  Widget build(BuildContext context) {
    final color = dark ? Colors.white70 : Theme.of(context).colorScheme.onSurfaceVariant;
    return SizedBox(
      height: 180,
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.image_not_supported_outlined, color: color, size: 36),
            const SizedBox(height: 8),
            Text(message, textAlign: TextAlign.center, style: TextStyle(color: color)),
          ],
        ),
      ),
    );
  }
}
