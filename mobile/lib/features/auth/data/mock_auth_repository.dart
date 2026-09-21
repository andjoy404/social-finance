import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/models/role.dart';
import '../../../core/errors/app_errors.dart';

/// Mock authentication repository for development only.
class AuthRepository extends StateNotifier<AsyncValue<Map<String, dynamic>?>?> {
  AuthRepository() : super(null);

  Map<String, dynamic>? _currentUser;

  Future<void> login(String email, String password) async {
    state = const AsyncLoading();

    await Future.delayed(const Duration(milliseconds: 600));

    if (email.isNotEmpty && password.isNotEmpty) {
      _currentUser = {
        'name': 'Heri Prastyo',
        'role': AppRole.bendahara,
        'rt': '002',
        'rw': '016',
      };
      state = AsyncData(_currentUser!);
    } else {
      state = AsyncError(
        AuthError('Email dan kata sandi wajib diisi.'),
        StackTrace.current,
      );
    }
  }

  Future<void> logout() async {
    await Future.delayed(const Duration(milliseconds: 300));
    _currentUser = null;
    state = null;
  }

  AppRole get currentRole => _currentUser?['role'] ?? AppRole.warga;
}

final authRepositoryProvider =
    StateNotifierProvider<AuthRepository, AsyncValue<Map<String, dynamic>?>?>((
      ref,
    ) {
      return AuthRepository();
    });
