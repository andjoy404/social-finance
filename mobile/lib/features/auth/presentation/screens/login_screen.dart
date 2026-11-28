import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/constants/app_constants.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/navigators/form_navigation_guard.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/phone_utils.dart';
import 'package:social_finance/core/utils/url_utils.dart';
import 'package:social_finance/core/widgets/one_shot_animated_gif.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/auth/presentation/widgets/gitops_login_background.dart';

enum LoginMethod { email, phone }

/// Login screen that supports manual API URL entry, Email or Phone login,
/// and "Ingat Saya" session persistence.
class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _serverUrlController = TextEditingController(
    text: ApiConfig.effectiveDefaultBaseUrl,
  );
  final _serverUrlFocus = FocusNode();

  final _emailController = TextEditingController();
  final _emailFocus = FocusNode();

  final _phoneController = TextEditingController();
  final _phoneFocus = FocusNode();

  final _passwordController = TextEditingController();
  final _passwordFocus = FocusNode();

  String _activeServerUrl = ApiConfig.effectiveDefaultBaseUrl;
  bool _isEditingServerUrl = false;
  bool _savingServerUrl = false;
  String? _serverUrlError;

  LoginMethod _loginMethod = LoginMethod.email;
  bool _obscurePassword = true;
  bool _rememberMe = true;
  bool _updatingRememberMe = false;
  StoredLoginCredentials? _restoredLoginCredentials;
  String _error = '';

  bool get _isCustomServerUrl => !UrlUtils.isOfficialServer(_activeServerUrl);

  @override
  void initState() {
    super.initState();
    // Load last remembered login details from preferences
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      try {
        final prefs = ref.read(preferencesStorageProvider);
        final lastUrl = await prefs.getLastApiUrl();
        final lastEmail = await prefs.getLastEmail();
        final lastPhone = await prefs.getLastPhone();
        final lastMethod = await prefs.getLastLoginMethod();
        final rememberMe = await prefs.getRememberMe();
        // Purge any legacy stored login password — persistent login uses secure
        // refresh tokens only and never stores or prefills user passwords.
        try {
          await ref.read(secureAuthStorageProvider).clearLoginCredentials();
        } catch (_) {}
        final expiredMessage = ref
            .read(authRepositoryProvider.notifier)
            .consumeSessionExpiredMessage();

        if (mounted) {
          setState(() {
            _rememberMe = rememberMe;
            _restoredLoginCredentials = null;
            if (expiredMessage != null && expiredMessage.isNotEmpty) {
              _error = expiredMessage;
            }
            if (lastMethod == 'phone') {
              _loginMethod = LoginMethod.phone;
            } else if (lastMethod == 'email') {
              _loginMethod = LoginMethod.email;
            }
            if (lastEmail != null && lastEmail.isNotEmpty) {
              _emailController.text = lastEmail;
            }
            if (lastPhone != null && lastPhone.isNotEmpty) {
              _phoneController.text = PhoneUtils.formatDisplay(lastPhone);
            }
            final startupUrl = lastUrl;
            if (startupUrl != null && startupUrl.trim().isNotEmpty) {
              final urlErr = UrlUtils.validateUrl(startupUrl);
              if (urlErr != null) {
                _serverUrlController.text = startupUrl;
                _isEditingServerUrl = true;
                _serverUrlError = urlErr;
                _error =
                    'URL server tersimpan tidak valid: $urlErr Silakan perbaiki.';
              } else {
                final normalized = UrlUtils.normalizeUrl(startupUrl);
                _activeServerUrl = normalized;
                _serverUrlController.text = normalized;
                _isEditingServerUrl = false;
                _serverUrlError = null;
                final apiClient = ref.read(apiClientProvider);
                if (apiClient.baseUrl != normalized) {
                  apiClient.setBaseUrl(normalized);
                }
              }
            } else {
              _activeServerUrl = ApiConfig.effectiveDefaultBaseUrl;
              _serverUrlController.text = ApiConfig.effectiveDefaultBaseUrl;
              _isEditingServerUrl = false;
              _serverUrlError = null;
            }
          });
        }
      } catch (_) {
        if (mounted) {
          setState(() {
            _error =
                'Kredensial tersimpan tidak dapat diakses dengan aman. Silakan masukkan kembali.';
          });
        }
      }
    });
  }

  @override
  void dispose() {
    _serverUrlController.dispose();
    _serverUrlFocus.dispose();
    _emailController.dispose();
    _emailFocus.dispose();
    _phoneController.dispose();
    _phoneFocus.dispose();
    _passwordController.dispose();
    _passwordFocus.dispose();
    super.dispose();
  }

  Future<void> _setRememberMe(bool value) async {
    if (_updatingRememberMe || value == _rememberMe) return;
    setState(() {
      _updatingRememberMe = true;
      _error = '';
    });
    try {
      final storage = ref.read(secureAuthStorageProvider);
      final prefs = ref.read(preferencesStorageProvider);
      if (!value) {
        await storage.clearLoginCredentials();
      }
      await prefs.setRememberMe(value);
      if (!value) {
        _passwordController.clear();
        _restoredLoginCredentials = null;
      }
      if (mounted) {
        setState(() {
          _rememberMe = value;
          _updatingRememberMe = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _updatingRememberMe = false;
          _error =
              'Pengaturan Ingat Saya tidak dapat disimpan dengan aman. Coba lagi.';
        });
      }
    }
  }

  Future<void> _invalidateRestoredCredentials() async {
    final credentials = _restoredLoginCredentials;
    if (credentials == null) return;
    _passwordController.clear();
    if (mounted) {
      setState(() => _restoredLoginCredentials = null);
    }
    try {
      await ref.read(secureAuthStorageProvider).clearLoginCredentials();
    } catch (_) {
      if (mounted) {
        setState(() {
          _error =
              'Kredensial tersimpan tidak dapat dihapus dengan aman. Silakan coba lagi.';
        });
      }
    }
  }

  Future<void> _handleServerUrlChanged(String value) async {
    if (_serverUrlError != null) {
      setState(() => _serverUrlError = null);
    }
    if (_passwordController.text.isNotEmpty) {
      _passwordController.clear();
    }
    final credentials = _restoredLoginCredentials;
    if (credentials != null &&
        UrlUtils.normalizeUrl(value) !=
            UrlUtils.normalizeUrl(credentials.serverUrl)) {
      await _invalidateRestoredCredentials();
    }
  }

  void _toggleServerUrlEdit() {
    if (_savingServerUrl) return;
    if (_isEditingServerUrl) {
      _cancelServerUrlEdit();
    } else {
      setState(() {
        _isEditingServerUrl = true;
        _serverUrlError = null;
      });
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          _serverUrlFocus.requestFocus();
        }
      });
    }
  }

  void _cancelServerUrlEdit() {
    _serverUrlFocus.unfocus();
    setState(() {
      _serverUrlController.text = _activeServerUrl;
      _isEditingServerUrl = false;
      _serverUrlError = null;
      if (_error.startsWith('URL server tersimpan tidak valid')) {
        _error = '';
      }
    });
  }

  Future<bool> _saveServerUrlEdit() async {
    if (_savingServerUrl) return false;
    final raw = _serverUrlController.text.trim();
    if (raw.isEmpty) {
      setState(() {
        _serverUrlError = 'URL API / Server wajib diisi.';
        _error = 'URL API / Server wajib diisi.';
      });
      return false;
    }

    final urlErr = UrlUtils.validateUrl(raw);
    if (urlErr != null) {
      setState(() {
        _serverUrlError = urlErr;
        _error = urlErr;
      });
      return false;
    }

    final normalized = UrlUtils.normalizeUrl(raw);
    final serverChanged =
        UrlUtils.normalizeUrl(_activeServerUrl) != normalized;

    setState(() {
      _savingServerUrl = true;
      _serverUrlError = null;
      _error = '';
    });

    try {
      await ref.read(authRepositoryProvider.notifier).changeServer(normalized);
      if (serverChanged) {
        _passwordController.clear();
        _restoredLoginCredentials = null;
      }
      _serverUrlFocus.unfocus();
      if (mounted) {
        setState(() {
          _activeServerUrl = normalized;
          _serverUrlController.text = normalized;
          _isEditingServerUrl = false;
          _savingServerUrl = false;
          _serverUrlError = null;
        });
      }
      return true;
    } catch (_) {
      if (mounted) {
        setState(() {
          _savingServerUrl = false;
          _serverUrlError = 'Gagal menyimpan URL server. Coba lagi.';
          _error = 'Gagal menyimpan URL server. Coba lagi.';
        });
      }
      return false;
    }
  }

  Future<void> _resetServerUrlToOfficial() async {
    if (_savingServerUrl) return;
    const official = ApiConfig.defaultOfficialBaseUrl;
    final serverChanged =
        UrlUtils.normalizeUrl(_activeServerUrl) != official;

    setState(() {
      _savingServerUrl = true;
      _serverUrlError = null;
      _error = '';
    });

    try {
      await ref.read(authRepositoryProvider.notifier).changeServer(official);
      if (serverChanged) {
        _passwordController.clear();
        _restoredLoginCredentials = null;
      }
      _serverUrlFocus.unfocus();
      if (mounted) {
        setState(() {
          _activeServerUrl = official;
          _serverUrlController.text = official;
          _isEditingServerUrl = false;
          _savingServerUrl = false;
          _serverUrlError = null;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _savingServerUrl = false;
          _serverUrlError = 'Gagal mengatur ulang ke server resmi. Coba lagi.';
        });
      }
    }
  }

  Future<void> _handleIdentifierChanged(String value) async {
    if (_passwordController.text.isNotEmpty) {
      _passwordController.clear();
    }
    final credentials = _restoredLoginCredentials;
    if (credentials == null) return;
    final isMatchingMethod =
        (credentials.loginMethod == 'email' &&
            _loginMethod == LoginMethod.email) ||
        (credentials.loginMethod == 'phone' &&
            _loginMethod == LoginMethod.phone);
    final normalizedValue = credentials.loginMethod == 'phone'
        ? PhoneUtils.normalizePhone(value)
        : value.trim().toLowerCase();
    final normalizedIdentifier = credentials.loginMethod == 'phone'
        ? PhoneUtils.normalizePhone(credentials.identifier)
        : credentials.identifier.trim().toLowerCase();
    if (!isMatchingMethod || normalizedValue != normalizedIdentifier) {
      await _invalidateRestoredCredentials();
    }
  }

  Future<void> _handleLogin() async {
    if (_isEditingServerUrl) {
      final saved = await _saveServerUrlEdit();
      if (!saved || !mounted) return;
    }

    final serverUrl = _activeServerUrl.trim();
    if (serverUrl.isEmpty) {
      setState(() => _error = 'URL API / Server wajib diisi.');
      return;
    }

    final urlErr = UrlUtils.validateUrl(serverUrl);
    if (urlErr != null) {
      setState(() => _error = urlErr);
      return;
    }

    String? email;
    String? phone;

    if (_loginMethod == LoginMethod.email) {
      email = _emailController.text.trim();
      if (email.isEmpty) {
        setState(() => _error = 'Email wajib diisi.');
        return;
      }
      if (!email.contains('@') || !email.contains('.')) {
        setState(() => _error = 'Format email tidak valid.');
        return;
      }
    } else {
      phone = _phoneController.text.trim();
      final phoneErr = PhoneUtils.validatePhone(phone);
      if (phoneErr != null) {
        setState(() => _error = phoneErr);
        return;
      }
    }

    final password = _passwordController.text;
    if (password.isEmpty) {
      setState(() => _error = 'Kata sandi wajib diisi.');
      return;
    }

    setState(() => _error = '');

    final authRepo = ref.read(authRepositoryProvider.notifier);
    authRepo.login(
      _loginMethod == LoginMethod.email ? email! : '',
      password,
      phone: _loginMethod == LoginMethod.phone ? phone : null,
      serverUrl: serverUrl,
      rememberMe: _rememberMe,
    );
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final isLoading = authState is AsyncLoading;

    // Update error from state changes
    if ((authState != null && authState.hasError) && _error.isEmpty) {
      final err = authState.error;
      if (err is AuthError) {
        _error = err.message;
      } else if (err != null) {
        _error = err.toString();
      }
    }

    return Scaffold(
      backgroundColor: GitOpsLoginBackground.bg,
      body: GitOpsLoginBackground(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(
              horizontal: 20,
              vertical: 24,
            ),
            child: _buildCard(context, isLoading),
          ),
        ),
      ),
    );
  }

  Widget _buildCard(BuildContext context, bool isLoading) {
    return GestureDetector(
      behavior: HitTestBehavior.translucent,
      onTap: () => FocusScope.of(context).unfocus(),
      child: Container(
        width: double.infinity,
        constraints: const BoxConstraints(maxWidth: 340),
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 18),
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xE015111F), Color(0xF50D0A14)],
          ),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: const Color(0x1AA970FF), width: 1),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.6),
              blurRadius: 80,
              offset: const Offset(0, 30),
              spreadRadius: 0,
            ),
            BoxShadow(
              color: const Color(0x1F7C5AC7),
              blurRadius: 70,
              offset: const Offset(0, 0),
              spreadRadius: 0,
            ),
          ],
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            // 1. Logo and title Social Finance
            const SizedBox(height: 4),
            OneShotAnimatedLogo(
              assetPath: 'assets/images/logo_animated.gif',
              fallbackAssetPath: 'assets/images/logo.png',
              width: 104,
              height: 104,
            ),
            const SizedBox(height: 6),
            Text(
              AppConstants.appName,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontFamily: 'monospace',
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: Color(0xFFF5F3FB),
                letterSpacing: -0.02,
                height: 1.2,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              AppConstants.appTagline,
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 12,
                color: Color(0xFF9B93AD),
                height: 1.35,
              ),
            ),
            const SizedBox(height: 14),

            // Error message banner
            if (_error.isNotEmpty) ...[
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 10,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: const Color(0xFFE8453C).withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(7),
                  border: Border.all(color: const Color(0x80F2495C)),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _error,
                      style: const TextStyle(
                        fontSize: 12,
                        color: Color(0xFFFF98A6),
                      ),
                    ),
                    if (_error.contains('kode reset')) ...[
                      const SizedBox(height: 6),
                      GestureDetector(
                        onTap: _showResetPasswordDialog,
                        child: const Text(
                          'Buka Menu Reset Kata Sandi →',
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.bold,
                            color: AppColors.accent,
                            decoration: TextDecoration.underline,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],

            // 2. Input URL API / Server (locked by default to official server)
            _buildServerUrlSection(context, isLoading),
            const SizedBox(height: 12),

            // 3. Choice for login method: Email or Nomor HP
            _buildMethodSelector(isLoading),
            const SizedBox(height: 12),

            // 4. Input email or phone based on choice
            if (_loginMethod == LoginMethod.email)
              _buildField(
                context,
                label: 'Email',
                controller: _emailController,
                focusNode: _emailFocus,
                hintText: 'Masukkan email',
                keyboardType: TextInputType.emailAddress,
                textInputAction: TextInputAction.next,
                onChanged: _handleIdentifierChanged,
                onFieldSubmitted: (_) {
                  FocusScope.of(context).requestFocus(_passwordFocus);
                },
                prefixIcon: Icons.email_outlined,
                enabled: !isLoading,
              )
            else
              _buildField(
                context,
                label: 'Nomor HP',
                controller: _phoneController,
                focusNode: _phoneFocus,
                hintText: '081200000001',
                keyboardType: TextInputType.phone,
                textInputAction: TextInputAction.next,
                onChanged: _handleIdentifierChanged,
                onFieldSubmitted: (_) {
                  FocusScope.of(context).requestFocus(_passwordFocus);
                },
                prefixIcon: Icons.phone_outlined,
                enabled: !isLoading,
              ),
            const SizedBox(height: 12),

            // 5. Input Kata Sandi
            _buildField(
              context,
              label: 'Kata Sandi',
              controller: _passwordController,
              focusNode: _passwordFocus,
              hintText: 'Masukkan kata sandi',
              obscureText: _obscurePassword,
              textInputAction: TextInputAction.done,
              onFieldSubmitted: (_) {
                _handleLogin();
              },
              prefixIcon: Icons.lock_outline,
              suffixIcon: _buildPasswordToggle(),
              enabled: !isLoading,
            ),
            const SizedBox(height: 10),

            // 6. Checkbox Ingat saya & Lupa Kata Sandi?
            Wrap(
              alignment: WrapAlignment.spaceBetween,
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 8,
              runSpacing: 6,
              children: [
                _buildRememberMeRow(isLoading),
                GestureDetector(
                  onTap: isLoading ? null : _showResetPasswordDialog,
                  child: const Text(
                    'Lupa Kata Sandi?',
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.accent,
                      decoration: TextDecoration.none,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 14),

            // 7. Submit button
            _buildSubmitButton(isLoading),
            const SizedBox(height: 10),
            TextButton.icon(
              onPressed: isLoading
                  ? null
                  : () {
                      final router = GoRouter.maybeOf(context);
                      if (router != null) {
                        router.go('/pos/activate');
                      }
                    },
              icon: const Icon(
                Icons.qr_code_2_outlined,
                size: 16,
                color: Color(0xFFC4B5FD),
              ),
              label: const Text(
                'Mode Perangkat Pos Keamanan',
                style: TextStyle(fontSize: 12, color: Color(0xFFC4B5FD)),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMethodSelector(bool isLoading) {
    return Container(
      decoration: BoxDecoration(
        color: const Color(0x1F09070F),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0x28A970FF)),
      ),
      padding: const EdgeInsets.all(2),
      child: Row(
        children: [
          Expanded(
            child: GestureDetector(
              onTap: isLoading
                  ? null
                  : () {
                      if (_loginMethod != LoginMethod.email) {
                        setState(() {
                          _loginMethod = LoginMethod.email;
                          _error = '';
                        });
                        _passwordController.clear();
                        _invalidateRestoredCredentials();
                      }
                    },
              child: Container(
                padding: const EdgeInsets.symmetric(vertical: 6),
                decoration: BoxDecoration(
                  color: _loginMethod == LoginMethod.email
                      ? const Color(0x38A970FF)
                      : Colors.transparent,
                  borderRadius: BorderRadius.circular(6),
                  border: _loginMethod == LoginMethod.email
                      ? Border.all(
                          color: AppColors.accent.withValues(alpha: 0.6),
                        )
                      : null,
                ),
                child: Text(
                  'Email',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: _loginMethod == LoginMethod.email
                        ? FontWeight.w600
                        : FontWeight.normal,
                    color: _loginMethod == LoginMethod.email
                        ? const Color(0xFFF5F3FB)
                        : const Color(0xFF9B93AD),
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(width: 4),
          Expanded(
            child: GestureDetector(
              onTap: isLoading
                  ? null
                  : () {
                      if (_loginMethod != LoginMethod.phone) {
                        setState(() {
                          _loginMethod = LoginMethod.phone;
                          _error = '';
                        });
                        _passwordController.clear();
                        _invalidateRestoredCredentials();
                      }
                    },
              child: Container(
                padding: const EdgeInsets.symmetric(vertical: 6),
                decoration: BoxDecoration(
                  color: _loginMethod == LoginMethod.phone
                      ? const Color(0x38A970FF)
                      : Colors.transparent,
                  borderRadius: BorderRadius.circular(6),
                  border: _loginMethod == LoginMethod.phone
                      ? Border.all(
                          color: AppColors.accent.withValues(alpha: 0.6),
                        )
                      : null,
                ),
                child: Text(
                  'Nomor HP',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: _loginMethod == LoginMethod.phone
                        ? FontWeight.w600
                        : FontWeight.normal,
                    color: _loginMethod == LoginMethod.phone
                        ? const Color(0xFFF5F3FB)
                        : const Color(0xFF9B93AD),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildRememberMeRow(bool isLoading) {
    final disabled = isLoading || _updatingRememberMe;
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: disabled ? null : () => _setRememberMe(!_rememberMe),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 24,
            height: 24,
            child: Checkbox(
              value: _rememberMe,
              activeColor: AppColors.accent,
              checkColor: Colors.white,
              side: const BorderSide(color: Color(0x80A970FF), width: 1.5),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(4),
              ),
              onChanged: disabled ? null : (val) => _setRememberMe(val ?? true),
            ),
          ),
          const SizedBox(width: 8),
          const Text(
            'Ingat saya',
            style: TextStyle(fontSize: 12, color: Color(0xFFCFC9DE)),
          ),
        ],
      ),
    );
  }

  Widget _buildServerUrlSection(BuildContext context, bool isLoading) {
    final isLocked = !_isEditingServerUrl;
    final canInteract = !isLoading && !_savingServerUrl;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _buildField(
          context,
          fieldKey: const Key('login-api-url-field'),
          label: 'URL API / Server',
          controller: _serverUrlController,
          focusNode: _serverUrlFocus,
          hintText: ApiConfig.defaultOfficialBaseUrl,
          keyboardType: TextInputType.url,
          textInputAction: TextInputAction.done,
          readOnly: isLocked,
          isLockedAppearance: isLocked,
          onChanged: _handleServerUrlChanged,
          onFieldSubmitted: (_) {
            if (_isEditingServerUrl) {
              _saveServerUrlEdit();
            } else if (_loginMethod == LoginMethod.email) {
              FocusScope.of(context).requestFocus(_emailFocus);
            } else {
              FocusScope.of(context).requestFocus(_phoneFocus);
            }
          },
          prefixIcon: Icons.dns_outlined,
          suffixIcon: Tooltip(
            message: isLocked
                ? 'Buka kunci untuk mengubah URL server'
                : 'Batal ubah URL server',
            child: InkWell(
              key: const Key('login-api-url-lock-btn'),
              borderRadius: BorderRadius.circular(6),
              onTap: canInteract ? _toggleServerUrlEdit : null,
              child: Padding(
                padding: const EdgeInsets.all(6),
                child: Icon(
                  isLocked ? Icons.lock_outline : Icons.lock_open_outlined,
                  size: 16,
                  color: isLocked
                      ? GitOpsLoginBackground.muted
                      : GitOpsLoginBackground.accent,
                ),
              ),
            ),
          ),
          enabled: !isLoading,
        ),
        if (_serverUrlError != null && _serverUrlError!.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            _serverUrlError!,
            style: const TextStyle(
              fontSize: 11,
              color: Color(0xFFFF98A6),
            ),
          ),
        ],
        if (_isEditingServerUrl) ...[
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: SizedBox(
                  height: 32,
                  child: ElevatedButton(
                    key: const Key('login-api-url-save-btn'),
                    onPressed: canInteract ? _saveServerUrlEdit : null,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: GitOpsLoginBackground.accent,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(horizontal: 10),
                      visualDensity: VisualDensity.compact,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                    ),
                    child: _savingServerUrl
                        ? const SizedBox(
                            width: 14,
                            height: 14,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              color: Colors.white,
                            ),
                          )
                        : const Text(
                            'Simpan',
                            style: TextStyle(
                              fontSize: 11,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: SizedBox(
                  height: 32,
                  child: ElevatedButton(
                    key: const Key('login-api-url-cancel-btn'),
                    onPressed: canInteract ? _cancelServerUrlEdit : null,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.danger,
                      foregroundColor: Colors.white,
                      elevation: 0,
                      padding: const EdgeInsets.symmetric(horizontal: 10),
                      visualDensity: VisualDensity.compact,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                    ),
                    child: const Text(
                      'Batal',
                      style: TextStyle(fontSize: 11, color: Colors.white),
                    ),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          SizedBox(
            height: 32,
            child: OutlinedButton.icon(
              key: const Key('login-api-url-reset-btn'),
              onPressed: canInteract ? _resetServerUrlToOfficial : null,
              style: OutlinedButton.styleFrom(
                foregroundColor: AppColors.warning,
                side: BorderSide(
                  color: AppColors.warning.withValues(alpha: 0.5),
                ),
                padding: const EdgeInsets.symmetric(horizontal: 10),
                visualDensity: VisualDensity.compact,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(AppRadius.base),
                ),
              ),
              icon: const Icon(Icons.restore, size: 14),
              label: const Text(
                'Reset ke Server Resmi',
                style: TextStyle(fontSize: 11, fontWeight: FontWeight.w500),
              ),
            ),
          ),
        ],
        if (_isCustomServerUrl) ...[
          const SizedBox(height: 6),
          Container(
            key: const Key('login-custom-server-indicator'),
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
            decoration: BoxDecoration(
              color: AppColors.warning.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(7),
              border: Border.all(
                color: AppColors.warning.withValues(alpha: 0.45),
              ),
            ),
            child: Wrap(
              alignment: WrapAlignment.spaceBetween,
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: 8,
              runSpacing: 4,
              children: [
                const Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.warning_amber_rounded,
                      size: 14,
                      color: AppColors.warning,
                    ),
                    SizedBox(width: 5),
                    Flexible(
                      child: Text(
                        'Mode Server Kustom (Non-Resmi)',
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: AppColors.warning,
                        ),
                      ),
                    ),
                  ],
                ),
                if (!_isEditingServerUrl)
                  InkWell(
                    key: const Key('login-api-url-quick-reset-btn'),
                    onTap: canInteract ? _resetServerUrlToOfficial : null,
                    borderRadius: BorderRadius.circular(4),
                    child: const Padding(
                      padding: EdgeInsets.symmetric(horizontal: 2, vertical: 1),
                      child: Text(
                        'Reset ke Server Resmi',
                        style: TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: GitOpsLoginBackground.accent,
                          decoration: TextDecoration.underline,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildField(
    BuildContext context, {
    Key? fieldKey,
    required String label,
    required TextEditingController controller,
    required String hintText,
    FocusNode? focusNode,
    TextInputType? keyboardType,
    TextInputAction textInputAction = TextInputAction.next,
    void Function(String)? onFieldSubmitted,
    bool obscureText = false,
    IconData? prefixIcon,
    Widget? suffixIcon,
    bool autoFocus = false,
    bool enabled = true,
    bool readOnly = false,
    bool isLockedAppearance = false,
    void Function(String)? onChanged,
  }) {
    final textColor = isLockedAppearance
        ? GitOpsLoginBackground.muted
        : GitOpsLoginBackground.text;
    final fillColor = isLockedAppearance
        ? const Color(0x268E8A9C)
        : GitOpsLoginBackground.inputBg;
    final borderColor = isLockedAppearance
        ? const Color(0x4D8E8A9C)
        : GitOpsLoginBackground.inputBorder;
    final focusedBorderColor = isLockedAppearance
        ? const Color(0x668E8A9C)
        : GitOpsLoginBackground.accent;
    final iconColor = isLockedAppearance
        ? GitOpsLoginBackground.placeholder
        : GitOpsLoginBackground.icon;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          label,
          style: const TextStyle(
            fontSize: 12,
            color: GitOpsLoginBackground.label,
            height: 1.35,
          ),
        ),
        const SizedBox(height: 6),
        SizedBox(
          height: 38,
          child: TextField(
            key: fieldKey,
            controller: controller,
            focusNode: focusNode,
            obscureText: obscureText,
            keyboardType: keyboardType,
            textInputAction: textInputAction,
            onSubmitted: onFieldSubmitted,
            onChanged: onChanged,
            autofocus: autoFocus,
            enabled: enabled,
            readOnly: readOnly,
            style: TextStyle(
              fontSize: 12,
              color: textColor,
              fontFamily: 'sans-serif',
            ),
            decoration: InputDecoration(
              hintText: hintText,
              hintStyle: const TextStyle(
                fontSize: 12,
                color: GitOpsLoginBackground.placeholder,
                fontFamily: 'sans-serif',
              ),
              prefixIcon: prefixIcon != null
                  ? Padding(
                      padding: const EdgeInsets.only(left: 10, right: 6),
                      child: Icon(
                        prefixIcon,
                        size: 15,
                        color: iconColor,
                      ),
                    )
                  : null,
              suffixIcon: suffixIcon != null
                  ? Padding(
                      padding: const EdgeInsets.only(right: 6),
                      child: suffixIcon,
                    )
                  : null,
              filled: true,
              fillColor: fillColor,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(7),
                borderSide: BorderSide(color: borderColor),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(7),
                borderSide: BorderSide(color: borderColor),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(7),
                borderSide: BorderSide(
                  color: focusedBorderColor,
                  width: isLockedAppearance ? 1 : 2,
                ),
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 12,
                vertical: 0,
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildPasswordToggle() {
    final obscure = _obscurePassword;
    return GestureDetector(
      onTap: () {
        setState(() => _obscurePassword = !obscure);
      },
      child: Padding(
        padding: const EdgeInsets.only(right: 4),
        child: Icon(
          obscure ? Icons.visibility_off : Icons.visibility,
          size: 15,
          color: const Color(0xFF8D82AB),
        ),
      ),
    );
  }

  Widget _buildSubmitButton(bool isLoading) {
    return Container(
      decoration: BoxDecoration(
        boxShadow: [
          BoxShadow(
            color: AppColors.accent.withValues(alpha: 0.15),
            blurRadius: 8,
            offset: const Offset(0, 0),
          ),
        ],
      ),
      child: SizedBox(
        width: double.infinity,
        height: 38,
        child: OutlinedButton(
          onPressed: isLoading ? null : _handleLogin,
          style: OutlinedButton.styleFrom(
            side: const BorderSide(color: AppColors.accent),
            foregroundColor: AppColors.accent,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppRadius.base),
            ),
          ),
          child: isLoading
              ? SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    backgroundColor: Colors.transparent,
                    valueColor: AlwaysStoppedAnimation<Color>(AppColors.accent),
                  ),
                )
              : const Text('Masuk'),
        ),
      ),
    );
  }

  void _showResetPasswordDialog() {
    final serverUrl = _activeServerUrl.trim();
    if (serverUrl.isNotEmpty && UrlUtils.validateUrl(serverUrl) == null) {
      ref.read(apiClientProvider).setBaseUrl(serverUrl);
    }

    final currentIdentifier = _loginMethod == LoginMethod.email
        ? _emailController.text.trim()
        : PhoneUtils.normalizePhone(_phoneController.text.trim());

    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (dialogCtx) => _ResetPasswordDialog(
        initialIdentifier: currentIdentifier,
        onSuccess: (newIdentifier) {
          if (newIdentifier.contains('@')) {
            _loginMethod = LoginMethod.email;
            _emailController.text = newIdentifier;
          } else {
            _loginMethod = LoginMethod.phone;
            _phoneController.text = PhoneUtils.formatDisplay(newIdentifier);
          }
          _passwordController.clear();
          setState(() {});
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Kata sandi berhasil diubah. Silakan masuk menggunakan kata sandi baru.'),
              backgroundColor: AppColors.success,
            ),
          );
        },
      ),
    );
  }
}

class _ResetPasswordDialog extends ConsumerStatefulWidget {
  final String initialIdentifier;
  final ValueChanged<String> onSuccess;

  const _ResetPasswordDialog({
    required this.initialIdentifier,
    required this.onSuccess,
  });

  @override
  ConsumerState<_ResetPasswordDialog> createState() => _ResetPasswordDialogState();
}

class _ResetPasswordDialogState extends ConsumerState<_ResetPasswordDialog> {
  late final TextEditingController _identifierController;
  late final TextEditingController _codeController;
  late final TextEditingController _newPasswordController;
  late final TextEditingController _confirmPasswordController;

  bool _obscureNewPassword = true;
  bool _obscureConfirmPassword = true;
  bool _loading = false;
  String? _errorMessage;

  bool get _isDirty =>
      _identifierController.text != widget.initialIdentifier ||
      _codeController.text.isNotEmpty ||
      _newPasswordController.text.isNotEmpty ||
      _confirmPasswordController.text.isNotEmpty;

  @override
  void initState() {
    super.initState();
    _identifierController = TextEditingController(text: widget.initialIdentifier);
    _codeController = TextEditingController();
    _newPasswordController = TextEditingController();
    _confirmPasswordController = TextEditingController();
  }

  @override
  void dispose() {
    _identifierController.dispose();
    _codeController.dispose();
    _newPasswordController.dispose();
    _confirmPasswordController.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final identifier = _identifierController.text.trim();
    final code = _codeController.text.trim();
    final newPassword = _newPasswordController.text;
    final confirmPassword = _confirmPasswordController.text;

    if (identifier.isEmpty) {
      setState(() => _errorMessage = 'Email atau nomor HP wajib diisi.');
      return;
    }
    if (code.length != 4 || !RegExp(r'^\d{4}$').hasMatch(code)) {
      setState(() => _errorMessage = 'Kode reset harus berupa 4 digit angka.');
      return;
    }
    if (newPassword.length < 8) {
      setState(() => _errorMessage = 'Kata sandi baru minimal 8 karakter.');
      return;
    }
    if (newPassword != confirmPassword) {
      setState(() => _errorMessage = 'Konfirmasi kata sandi tidak cocok.');
      return;
    }

    setState(() {
      _loading = true;
      _errorMessage = null;
    });

    try {
      await ref.read(authServiceProvider).resetPasswordWithCode(
        identifier: identifier,
        code: code,
        newPassword: newPassword,
        confirmPassword: confirmPassword,
      );
      if (!mounted) return;
      Navigator.of(context).pop();
      widget.onSuccess(identifier);
    } catch (e) {
      if (!mounted) return;
      String msg = 'Gagal mengatur ulang kata sandi.';
      if (e is DioException) {
        final data = e.response?.data;
        if (data is Map && data['error'] is Map) {
          msg = data['error']['message']?.toString() ?? msg;
        } else if (data is Map && data['message'] != null) {
          msg = data['message'].toString();
        }
      }
      setState(() {
        _loading = false;
        _errorMessage = msg;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return GuardedFormModalScope(
      isDirty: () => _isDirty,
      isSaving: () => _loading,
      child: Dialog(
        backgroundColor: const Color(0xFF1E192B),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Expanded(
                  child: Text(
                    'Reset Kata Sandi Warga',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                      color: Color(0xFFF2EFF9),
                    ),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.close, size: 20, color: Color(0xFF9B93AD)),
                  onPressed: _loading
                      ? null
                      : () => FormNavigationGuard.requestModalClose(
                          context,
                          isDirty: _isDirty,
                          isSaving: _loading,
                        ),
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(),
                ),
              ],
            ),
            const SizedBox(height: 8),
            const Text(
              'Masukkan email atau nomor HP beserta 4 digit kode reset yang telah diberikan oleh Pengurus RT.',
              style: TextStyle(fontSize: 12, color: Color(0xFF9B93AD)),
            ),
            if (_errorMessage != null) ...[
              const SizedBox(height: 12),
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: AppColors.danger.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: AppColors.danger.withValues(alpha: 0.3)),
                ),
                child: Text(
                  _errorMessage!,
                  style: const TextStyle(fontSize: 12, color: AppColors.danger),
                ),
              ),
            ],
            const SizedBox(height: 16),
            // Identifier
            _buildDialogField(
              label: 'Email atau Nomor HP',
              controller: _identifierController,
              hint: 'nama@email.com atau 0812...',
              keyboardType: TextInputType.emailAddress,
              prefixIcon: Icons.person_outline,
              enabled: !_loading,
            ),
            const SizedBox(height: 12),
            // Kode 4 Digit
            _buildDialogField(
              label: 'Kode Reset (4 Digit dari Pengurus)',
              controller: _codeController,
              hint: 'Contoh: 1234',
              keyboardType: TextInputType.number,
              prefixIcon: Icons.vpn_key_outlined,
              enabled: !_loading,
              formatters: [
                FilteringTextInputFormatter.digitsOnly,
                LengthLimitingTextInputFormatter(4),
              ],
              style: const TextStyle(
                letterSpacing: 6,
                fontWeight: FontWeight.bold,
                fontSize: 16,
                fontFamily: 'monospace',
                color: Color(0xFFF2EFF9),
              ),
            ),
            const SizedBox(height: 12),
            // Kata Sandi Baru
            _buildDialogField(
              label: 'Kata Sandi Baru (Min. 8 Karakter)',
              controller: _newPasswordController,
              hint: 'Masukkan kata sandi baru',
              obscureText: _obscureNewPassword,
              prefixIcon: Icons.lock_outline,
              enabled: !_loading,
              suffixIcon: IconButton(
                icon: Icon(
                  _obscureNewPassword ? Icons.visibility_off : Icons.visibility,
                  size: 18,
                  color: const Color(0xFF9B93AD),
                ),
                onPressed: () => setState(() => _obscureNewPassword = !_obscureNewPassword),
              ),
            ),
            const SizedBox(height: 12),
            // Konfirmasi Kata Sandi
            _buildDialogField(
              label: 'Konfirmasi Kata Sandi Baru',
              controller: _confirmPasswordController,
              hint: 'Ulangi kata sandi baru',
              obscureText: _obscureConfirmPassword,
              prefixIcon: Icons.lock_outline,
              enabled: !_loading,
              suffixIcon: IconButton(
                icon: Icon(
                  _obscureConfirmPassword ? Icons.visibility_off : Icons.visibility,
                  size: 18,
                  color: const Color(0xFF9B93AD),
                ),
                onPressed: () => setState(() => _obscureConfirmPassword = !_obscureConfirmPassword),
              ),
            ),
            const SizedBox(height: 20),
            Row(
              children: [
                Expanded(
                  child: ElevatedButton(
                    onPressed: _loading
                        ? null
                        : () => FormNavigationGuard.requestModalClose(
                            context,
                            isDirty: _isDirty,
                            isSaving: _loading,
                          ),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.danger,
                      foregroundColor: Colors.white,
                      elevation: 0,
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                    ),
                    child: const Text('Batal'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: FilledButton(
                    onPressed: _loading ? null : _submit,
                    style: FilledButton.styleFrom(
                      backgroundColor: AppColors.accent,
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
                    ),
                    child: _loading
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                          )
                        : const Text('Simpan'),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
      ),
    );
  }

  Widget _buildDialogField({
    required String label,
    required TextEditingController controller,
    required String hint,
    TextInputType? keyboardType,
    bool obscureText = false,
    IconData? prefixIcon,
    Widget? suffixIcon,
    bool enabled = true,
    List<TextInputFormatter>? formatters,
    TextStyle? style,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(fontSize: 12, color: Color(0xFFCFC9DE)),
        ),
        const SizedBox(height: 4),
        TextField(
          controller: controller,
          obscureText: obscureText,
          keyboardType: keyboardType,
          enabled: enabled,
          inputFormatters: formatters,
          style: style ?? const TextStyle(fontSize: 13, color: Color(0xFFF2EFF9)),
          decoration: InputDecoration(
            hintText: hint,
            hintStyle: const TextStyle(fontSize: 13, color: Color(0xFF706785)),
            prefixIcon: prefixIcon != null ? Icon(prefixIcon, size: 18, color: const Color(0xFF9B93AD)) : null,
            suffixIcon: suffixIcon,
            filled: true,
            fillColor: const Color(0x3309070F),
            isDense: true,
            contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: const BorderSide(color: Color(0x28A970FF)),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: const BorderSide(color: Color(0x28A970FF)),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(8),
              borderSide: const BorderSide(color: AppColors.accent),
            ),
          ),
        ),
      ],
    );
  }
}
