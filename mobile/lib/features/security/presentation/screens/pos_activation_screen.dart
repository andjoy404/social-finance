import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/url_utils.dart';

import '../providers/security_providers.dart';

class PosActivationScreen extends ConsumerStatefulWidget {
  const PosActivationScreen({super.key});

  @override
  ConsumerState<PosActivationScreen> createState() =>
      _PosActivationScreenState();
}

class _PosActivationScreenState extends ConsumerState<PosActivationScreen> {
  final _serverCtrl = TextEditingController();
  final _tokenCtrl = TextEditingController();
  String? _error;

  @override
  void initState() {
    super.initState();
    _prefillServerUrl();
  }

  Future<void> _prefillServerUrl() async {
    final lastUrl = await SharedPrefsStorage().getLastApiUrl();
    if (!mounted) return;
    if (lastUrl != null && lastUrl.isNotEmpty) {
      _serverCtrl.text = lastUrl;
    } else if (ApiConfig.hasBaseUrl) {
      _serverCtrl.text = ApiConfig.normalizedBaseUrl;
    }
  }

  @override
  void dispose() {
    _serverCtrl.dispose();
    _tokenCtrl.dispose();
    super.dispose();
  }

  Future<void> _handleActivate() async {
    final serverUrl = _serverCtrl.text.trim();
    final pairingToken = _tokenCtrl.text.trim();
    if (serverUrl.isEmpty) {
      setState(() => _error = 'URL Server wajib diisi.');
      return;
    }
    final urlErr = UrlUtils.validateUrl(serverUrl);
    if (urlErr != null) {
      setState(() => _error = urlErr);
      return;
    }
    if (pairingToken.isEmpty) {
      setState(() => _error = 'Token pairing dari Ketua RT wajib diisi.');
      return;
    }

    setState(() => _error = null);
    try {
      await SharedPrefsStorage().setLastApiUrl(
        UrlUtils.normalizeUrl(serverUrl),
      );
      await ref
          .read(posSessionProvider.notifier)
          .activate(serverUrl: serverUrl, pairingToken: pairingToken);
      if (!mounted) return;
      context.go('/pos');
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = e.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    final posState = ref.watch(posSessionProvider);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Aktivasi Perangkat Pos'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.go('/login'),
        ),
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Icon(
                Icons.security_outlined,
                size: 56,
                color: AppColors.accent,
              ),
              const SizedBox(height: 12),
              const Text(
                'Mode Perangkat Pos Keamanan',
                textAlign: TextAlign.center,
                style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
              ),
              const SizedBox(height: 8),
              const Text(
                'Masukkan token pairing satu kali yang dibuat oleh Ketua RT melalui portal Web untuk mengubah perangkat ini menjadi Kiosk Pos RT.',
                textAlign: TextAlign.center,
                style: TextStyle(color: Colors.grey),
              ),
              const SizedBox(height: 24),
              if ((posState.errorMessage ?? _error) != null) ...[
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(alpha: 0.1),
                    border: Border.all(color: AppColors.danger),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Text(
                    _error ?? posState.errorMessage!,
                    style: const TextStyle(color: AppColors.danger),
                  ),
                ),
                const SizedBox(height: 16),
              ],
              TextField(
                controller: _serverCtrl,
                decoration: const InputDecoration(
                  labelText: 'URL Server API',
                  hintText: 'https://...',
                  border: OutlineInputBorder(),
                ),
                keyboardType: TextInputType.url,
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _tokenCtrl,
                decoration: const InputDecoration(
                  labelText: 'Token Pairing Perangkat',
                  hintText: 'Masukkan token pairing...',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: posState.loading ? null : _handleActivate,
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.accent,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                ),
                child: posState.loading
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: Colors.white,
                        ),
                      )
                    : const Text('Aktifkan Perangkat Pos'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
