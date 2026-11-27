import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import '../../data/security_models.dart';
import '../providers/security_providers.dart';
import '../widgets/paket_detail_content.dart';

class PaketDetailScreen extends ConsumerStatefulWidget {
  final String reportId;
  final String from;

  const PaketDetailScreen({
    super.key,
    required this.reportId,
    this.from = 'notifikasi',
  });

  @override
  ConsumerState<PaketDetailScreen> createState() => _PaketDetailScreenState();
}

class _PaketDetailScreenState extends ConsumerState<PaketDetailScreen> {
  bool _isLoading = true;
  String? _errorMessage;
  PaketReport? _report;

  @override
  void initState() {
    super.initState();
    _fetchDetail();
  }

  Future<void> _fetchDetail() async {
    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    try {
      final client = ref.read(securityApiClientProvider);
      final report = await client.getPaketReport(widget.reportId);
      if (mounted) {
        setState(() {
          _report = report;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _errorMessage = e.toString().replaceFirst('Exception: ', '');
          _isLoading = false;
        });
      }
    }
  }

  void _handleBack() {
    if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    } else {
      context.go('/home/notifikasi');
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final canPop = Navigator.of(context).canPop();

    return PopScope(
      canPop: canPop,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop) {
          context.go('/home/notifikasi');
        }
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Detail Paket'),
          leading: IconButton(
            key: const Key('paket_detail_back_button'),
            icon: const Icon(Icons.arrow_back),
            tooltip: 'Kembali',
            onPressed: _handleBack,
          ),
          actions: [
            IconButton(
              icon: const Icon(Icons.refresh),
              tooltip: 'Segarkan',
              onPressed: _isLoading ? null : _fetchDetail,
            ),
          ],
        ),
        body: _buildBody(theme),
      ),
    );
  }

  Widget _buildBody(ThemeData theme) {
    if (_isLoading) {
      return const Center(
        child: CircularProgressIndicator(
          key: Key('paket_detail_loading'),
        ),
      );
    }

    if (_errorMessage != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(
                Icons.error_outline,
                size: 48,
                color: AppColors.danger,
              ),
              const SizedBox(height: 12),
              Text(
                'Gagal memuat detail paket',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                _errorMessage!,
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontSize: 13,
                ),
              ),
              const SizedBox(height: 16),
              ElevatedButton.icon(
                key: const Key('paket_detail_retry_btn'),
                onPressed: _fetchDetail,
                icon: const Icon(Icons.refresh, size: 18),
                label: const Text('Coba Lagi'),
              ),
            ],
          ),
        ),
      );
    }

    if (_report == null) {
      return const Center(
        child: Text('Data paket tidak ditemukan.'),
      );
    }

    return PaketDetailContent(
      report: _report!,
      isModal: false,
    );
  }
}
