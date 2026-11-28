import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/navigators/form_navigation_guard.dart';
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
  static const String _routePath = '/pos/activate';

  final _serverCtrl = TextEditingController();
  final _tokenCtrl = TextEditingController();
  String? _error;

  late final FormSnapshot _snapshot;
  FormGuardRegistration? _guardReg;

  Map<String, Object?> _currentValues() => {
    'server': _serverCtrl.text,
    'token': _tokenCtrl.text,
  };

  @override
  void initState() {
    super.initState();
    _snapshot = FormSnapshot(_currentValues());
    _guardReg = FormNavigationGuard.register(
      _routePath,
      isDirty: () => _snapshot.isDirty(_currentValues()),
      isSaving: () => ref.read(posSessionProvider).loading,
    );
    _prefillServerUrl();
  }

  Future<void> _prefillServerUrl() async {
    final lastUrl = await SharedPrefsStorage().getLastApiUrl();
    if (!mounted) return;
    final wasDirty = _snapshot.isDirty(_currentValues());
    if (lastUrl != null && lastUrl.isNotEmpty) {
      _serverCtrl.text = lastUrl;
    } else {
      _serverCtrl.text = ApiConfig.effectiveDefaultBaseUrl;
    }
    if (!wasDirty) {
      _snapshot.reset(_currentValues());
    }
  }

  @override
  void dispose() {
    FormNavigationGuard.unregister(_routePath, registration: _guardReg);
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
      FormNavigationGuard.allowExitFor(_routePath);
      try {
        context.go('/pos');
      } catch (_) {
        FormNavigationGuard.revokeExitFor(_routePath);
        rethrow;
      }
    } catch (e) {
      if (!mounted) return;
      setState(() => _error = e.toString());
    }
  }

  Future<void> _handleCancel() async {
    await FormNavigationGuard.requestFormCancel(
      context,
      routePath: _routePath,
      onPop: () => context.go('/login'),
    );
  }

  @override
  Widget build(BuildContext context) {
    final posState = ref.watch(posSessionProvider);
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        await _handleCancel();
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Aktivasi Perangkat Pos'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: posState.loading ? null : _handleCancel,
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
      ),
    );
  }
}
