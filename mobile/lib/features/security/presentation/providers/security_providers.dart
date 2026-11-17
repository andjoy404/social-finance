import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:social_finance/core/api/client.dart';

import '../../data/pos_device_storage.dart';
import '../../data/security_api_client.dart';

final posDeviceStorageProvider = Provider<PosDeviceStorage>((ref) {
  return PosDeviceStorage();
});

final securityApiClientProvider = Provider<SecurityApiClient>((ref) {
  return SecurityApiClient(ref.watch(apiClientProvider));
});

class PosSessionState {
  final bool initialized;
  final bool loading;
  final PosDeviceSession? session;
  final String? errorMessage;

  const PosSessionState({
    this.initialized = false,
    this.loading = false,
    this.session,
    this.errorMessage,
  });

  bool get isPaired => session != null;

  PosSessionState copyWith({
    bool? initialized,
    bool? loading,
    PosDeviceSession? session,
    bool clearSession = false,
    String? errorMessage,
    bool clearError = false,
  }) {
    return PosSessionState(
      initialized: initialized ?? this.initialized,
      loading: loading ?? this.loading,
      session: clearSession ? null : (session ?? this.session),
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }
}

class PosSessionNotifier extends StateNotifier<PosSessionState> {
  final SecurityApiClient _api;
  final PosDeviceStorage _storage;

  PosSessionNotifier(this._api, this._storage) : super(const PosSessionState());

  Future<void> restoreSession() async {
    if (state.initialized) return;
    final saved = await _storage.loadSession();
    if (saved == null) {
      state = state.copyWith(initialized: true, clearSession: true);
      return;
    }
    state = state.copyWith(initialized: true, session: saved, clearError: true);
    // Validate in background; if revoked, clear and show explanation.
    try {
      final freshDevice = await _api.fetchPosDeviceMe(saved);
      if (!freshDevice.isActive) {
        await unpair(
          reason: 'Akses perangkat pos telah dicabut oleh pengurus RT.',
        );
        return;
      }
      final updated = PosDeviceSession(
        serverUrl: saved.serverUrl,
        deviceToken: saved.deviceToken,
        device: freshDevice,
      );
      await _storage.saveSession(updated);
      state = state.copyWith(session: updated);
    } on SecurityApiException catch (e) {
      if (e.isDeviceRevokedOrUnauthorized) {
        await unpair(
          reason: 'Token perangkat pos sudah tidak berlaku atau dicabut.',
        );
      }
    } catch (_) {
      // Transient network errors keep local session intact.
    }
  }

  Future<PosDeviceSession> activate({
    required String serverUrl,
    required String pairingToken,
  }) async {
    state = state.copyWith(loading: true, clearError: true);
    try {
      final session = await _api.activatePosDevice(
        serverUrl: serverUrl,
        pairingToken: pairingToken,
      );
      await _storage.saveSession(session);
      state = state.copyWith(
        initialized: true,
        loading: false,
        session: session,
        clearError: true,
      );
      return session;
    } catch (e) {
      state = state.copyWith(loading: false, errorMessage: e.toString());
      rethrow;
    }
  }

  Future<void> unpair({String? reason}) async {
    await _storage.clearSession();
    state = state.copyWith(
      initialized: true,
      loading: false,
      clearSession: true,
      errorMessage: reason,
      clearError: reason == null,
    );
  }
}

final posSessionProvider =
    StateNotifierProvider<PosSessionNotifier, PosSessionState>((ref) {
      return PosSessionNotifier(
        ref.watch(securityApiClientProvider),
        ref.watch(posDeviceStorageProvider),
      );
    });
