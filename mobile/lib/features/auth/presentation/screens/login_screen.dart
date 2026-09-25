import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/constants/app_constants.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/primary_button.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _obscurePassword = true;

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    super.dispose();
  }

  void _handleLogin() {
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    final authRepo = ref.read(authRepositoryProvider.notifier);
    authRepo.login(email, password);
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final theme = Theme.of(context);
    final isLoading = authState is AsyncLoading;

    String? errorMessage;
    if (authState != null && authState.hasError) {
      final err = authState.error;
      if (err is AuthError) {
        errorMessage = err.message;
      } else if (err != null) {
        errorMessage = err.toString();
      }
    }

    return Scaffold(
      backgroundColor: theme.scaffoldBackgroundColor,
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 40),
            child: AppCard(
              padding: const EdgeInsets.all(28),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  // Brand icon
                  Icon(
                    Icons.account_balance_wallet,
                    size: 56,
                    color: theme.colorScheme.primary,
                  ),
                  const SizedBox(height: 20),
                  // App identity
                  Text(
                    AppConstants.appName,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.headlineSmall,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    AppConstants.appTagline,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 28),

                  // User-facing authentication error banner
                  if (errorMessage != null) ...[
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 14,
                        vertical: 10,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.expense.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(
                          color: AppColors.expense.withValues(alpha: 0.3),
                        ),
                      ),
                      child: Row(
                        children: [
                          const Icon(
                            Icons.error_outline,
                            color: AppColors.expense,
                            size: 20,
                          ),
                          const SizedBox(width: 10),
                          Expanded(
                            child: Text(
                              errorMessage,
                              style: theme.textTheme.bodySmall?.copyWith(
                                color: AppColors.expense,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 16),
                  ],

                  // Form
                  AppTextField(
                    controller: _emailController,
                    label: 'Email',
                    hintText: 'Masukkan email',
                    keyboardType: TextInputType.emailAddress,
                  ),
                  const SizedBox(height: 16),
                  AppTextField(
                    controller: _passwordController,
                    label: 'Kata Sandi',
                    hintText: 'Masukkan kata sandi',
                    obscureText: _obscurePassword,
                    suffixIcon: GestureDetector(
                      onTap: () {
                        setState(() => _obscurePassword = !_obscurePassword);
                      },
                      child: Icon(
                        _obscurePassword
                            ? Icons.visibility_off
                            : Icons.visibility,
                        size: 20,
                        color: theme.textTheme.bodySmall?.color,
                      ),
                    ),
                  ),
                  const SizedBox(height: 28),
                  PrimaryButton(
                    text: 'Masuk',
                    isLoading: isLoading,
                    onPressed: _handleLogin,
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'Gunakan email dan kata sandi apa saja untuk pengembangan.',
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodySmall,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
