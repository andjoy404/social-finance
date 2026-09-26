import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/constants/app_constants.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

/// Login screen that matches the Web login design.
///
/// Visual reference: [frontend/src/styles/login.css] and
/// [frontend/src/features/auth/Login.tsx].
class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _obscurePassword = true;
  String _error = '';

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _handleLogin() {
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    if (email.isEmpty || password.isEmpty) {
      setState(() {
        _error = 'Email dan kata sandi harus diisi.';
      });
      return;
    }
    final authRepo = ref.read(authRepositoryProvider.notifier);
    authRepo.login(email, password);
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
      backgroundColor: const Color(0xFF07060B),
      body: _buildBackground(
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

  Widget _buildBackground({required Widget child}) {
    return Stack(
      children: [
        child,
        Positioned.fill(
          child: IgnorePointer(
            child: CustomPaint(painter: _LoginBackgroundPainter()),
          ),
        ),
      ],
    );
  }

  Widget _buildCard(BuildContext context, bool isLoading) {
    return GestureDetector(
      onTap: () => FocusScope.of(context).unfocus(),
      child: Container(
        width: double.infinity,
        constraints: const BoxConstraints(maxWidth: 320),
        padding: const EdgeInsets.fromLTRB(20, 20, 20, 18),
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [
              Color(0xE015111F),
              Color(0xF50D0A14),
            ],
          ),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: const Color(0x1AA970FF),
            width: 1,
          ),
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
            // Brand lockup
            const SizedBox(height: 4),
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
            const SizedBox(height: 12),

            // Error message
            if (_error.isNotEmpty) ...[
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                decoration: BoxDecoration(
                  color: const Color(0xFFE8453C).withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(7),
                  border: Border.all(
                    color: const Color(0x80F2495C),
                  ),
                ),
                child: Text(
                  _error,
                  style: const TextStyle(
                    fontSize: 12,
                    color: Color(0xFFFF98A6),
                  ),
                ),
              ),
              const SizedBox(height: 10),
            ],

            // Email field
            _buildField(
              context,
              label: 'Email',
              controller: _emailController,
              hintText: 'Masukkan email',
              keyboardType: TextInputType.emailAddress,
              prefixIcon: Icons.email_outlined,
              autoFocus: !isLoading,
            ),
            const SizedBox(height: 11),

            // Password field
            _buildField(
              context,
              label: 'Kata Sandi',
              controller: _passwordController,
              hintText: 'Masukkan kata sandi',
              obscureText: _obscurePassword,
              prefixIcon: Icons.lock_outline,
              suffixIcon: _buildPasswordToggle(),
            ),
            const SizedBox(height: 16),

            // Submit button
            _buildSubmitButton(isLoading),

            // Help text
            const SizedBox(height: 16),
            Text(
              'Gunakan email dan kata sandi apa saja untuk pengembangan.',
              textAlign: TextAlign.center,
              style: const TextStyle(
                fontSize: 11,
                color: Color(0xFF6E777D),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildField(
    BuildContext context, {
    required String label,
    required TextEditingController controller,
    required String hintText,
    TextInputType? keyboardType,
    bool obscureText = false,
    IconData? prefixIcon,
    Widget? suffixIcon,
    bool autoFocus = false,
    bool enabled = true,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          label,
          style: const TextStyle(
            fontSize: 12,
            color: Color(0xFFCFC9DE),
            height: 1.35,
          ),
        ),
        const SizedBox(height: 6),
        SizedBox(
          height: 36,
          child: TextField(
            controller: controller,
            obscureText: obscureText,
            keyboardType: keyboardType,
            autofocus: autoFocus,
            enabled: enabled,
            style: const TextStyle(
              fontSize: 12,
              color: Color(0xFFF2EFF9),
              fontFamily: 'sans-serif',
            ),
            decoration: InputDecoration(
              hintText: hintText,
              hintStyle: const TextStyle(
                fontSize: 12,
                color: Color(0xFF7A7292),
                fontFamily: 'sans-serif',
              ),
              prefixIcon: prefixIcon != null
                  ? Padding(
                      padding: const EdgeInsets.only(left: 10),
                      child: Icon(
                        prefixIcon,
                        size: 13,
                        color: const Color(0xFF8D82AB),
                      ),
                    )
                  : null,
              suffixIcon: suffixIcon != null
                  ? Padding(
                      padding: const EdgeInsets.only(right: 4),
                      child: suffixIcon,
                    )
                  : null,
              filled: true,
              fillColor: const Color(0x0F09070F),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(7),
                borderSide: const BorderSide(
                  color: Color(0x38A970FF),
                ),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(7),
                borderSide: const BorderSide(
                  color: Color(0x38A970FF),
                ),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(7),
                borderSide: const BorderSide(
                  color: Color(0xFFA970FF),
                  width: 2,
                ),
              ),
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 32,
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
          size: 13,
          color: const Color(0xFF8D82AB),
        ),
      ),
    );
  }

  Widget _buildSubmitButton(bool isLoading) {
    final isDisabled = isLoading;
    return SizedBox(
      width: double.infinity,
      height: 36,
      child: ElevatedButton(
        onPressed: isDisabled ? null : _handleLogin,
        style: ElevatedButton.styleFrom(
          backgroundColor: isDisabled
              ? const Color(0x387C5AC7)
              : null,
          foregroundColor: isDisabled
              ? const Color(0x73C8C8EB)
              : const Color(0xFFF8F5FD),
          elevation: 0,
          padding: EdgeInsets.zero,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(7),
            side: BorderSide(
              color: isDisabled
                  ? const Color(0x2DA970FF)
                  : const Color(0x73A970FF),
            ),
          ),
          textStyle: const TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            letterSpacing: 0.01,
          ),
        ),
        child: isLoading
            ? SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  backgroundColor: Colors.transparent,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    const Color(0xFFF8F5FD),
                  ),
                ),
              )
            : Text(isLoading ? 'Memproses...' : 'Masuk'),
      ),
    );
  }
}

/// Background painter that reproduces the Web login's dark industrial
/// wall treatment using the same radial gradients, seam lines, and glow.
class _LoginBackgroundPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint1 = Paint()
      ..shader = LinearGradient(
        begin: Alignment.centerLeft,
        end: Alignment.centerRight,
        colors: [
          Colors.white.withValues(alpha: 0.025),
          Colors.transparent,
        ],
        stops: const [0.34, 1.0],
      ).createShader(
        Rect.fromLTWH(0, 0, size.width, size.height),
      );

    // White highlight fade
    canvas.drawRect(
      Rect.fromLTWH(0, 0, size.width, size.height),
      paint1,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
