import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/constants/app_constants.dart';
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
    if (_emailController.text.trim().isEmpty ||
        _passwordController.text.isEmpty) {
      return;
    }
    final authRepo = ref.read(authRepositoryProvider.notifier);
    authRepo.login(_emailController.text.trim(), _passwordController.text);
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final theme = Theme.of(context);

    return Scaffold(
      backgroundColor: theme.scaffoldBackgroundColor,
      body: SafeArea(
        child: switch (authState) {
          null => _buildLoginContent(theme),
          AsyncData() => _buildLoginContent(theme),
          AsyncLoading() => _buildLoadingContent(theme),
          AsyncError(:final error) => _buildErrorContent(error, theme),
          _ => _buildLoadingContent(theme),
        },
      ),
    );
  }

  Widget _buildLoginContent(ThemeData theme) {
    return Center(
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
              const SizedBox(height: 32),
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
                    _obscurePassword ? Icons.visibility_off : Icons.visibility,
                    size: 20,
                    color: theme.textTheme.bodySmall?.color,
                  ),
                ),
              ),
              const SizedBox(height: 28),
              PrimaryButton(text: 'Masuk', onPressed: _handleLogin),
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
    );
  }

  Widget _buildLoadingContent(ThemeData theme) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(),
          const SizedBox(height: 16),
          Text('Memproses...', style: theme.textTheme.bodyMedium),
        ],
      ),
    );
  }

  Widget _buildErrorContent(dynamic error, ThemeData theme) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: AppColors.expense),
            const SizedBox(height: 16),
            Text(
              error.toString(),
              textAlign: TextAlign.center,
              style: theme.textTheme.bodyMedium,
            ),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => setState(() {}),
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
    );
  }
}
