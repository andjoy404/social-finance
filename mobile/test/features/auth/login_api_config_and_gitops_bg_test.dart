import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/storage/app_storage.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/auth/presentation/screens/login_screen.dart';
import 'package:social_finance/features/auth/presentation/widgets/gitops_login_background.dart';

void main() {
  group('Point 1 — Login API Endpoint Configuration', () {
    testWidgets(
      'defaults to locked https://api.urena0216.id without custom server indicator',
      (tester) async {
        final prefStorage = InMemoryPreferencesStorage();
        final secureStorage = InMemorySecureAuthStorage();
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(client),
              preferencesStorageProvider.overrideWithValue(prefStorage),
              secureAuthStorageProvider.overrideWithValue(secureStorage),
              authRepositoryProvider.overrideWith((ref) => repo),
            ],
            child: const MaterialApp(home: LoginScreen()),
          ),
        );
        await tester.pumpAndSettle();

        final urlField = tester.widget<TextField>(
          find.byKey(const Key('login-api-url-field')),
        );
        expect(urlField.controller?.text, equals('https://api.urena0216.id'));
        expect(urlField.readOnly, isTrue);
        expect(client.baseUrl, equals('https://api.urena0216.id'));
        expect(find.byKey(const Key('login-api-url-lock-btn')), findsOneWidget);
        expect(
          find.descendant(
            of: find.byKey(const Key('login-api-url-lock-btn')),
            matching: find.byIcon(Icons.lock_outline),
          ),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('login-custom-server-indicator')),
          findsNothing,
        );
      },
    );

    testWidgets(
      'unlocking allows editing local HTTP endpoint, normalizes, persists, isolates session, and shows custom indicator',
      (tester) async {
        final prefStorage = InMemoryPreferencesStorage();
        final secureStorage = InMemorySecureAuthStorage(
          initialSession: const StoredSession(
            accessToken: 'old-official-token',
            refreshToken: 'old-official-refresh',
            serverUrl: 'https://api.urena0216.id',
            userData: {'id': 'u1', 'name': 'Warga'},
          ),
        );
        final client = ApiClient();
        client.setToken('old-official-token');
        client.setRefreshToken('old-official-refresh');

        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(client),
              preferencesStorageProvider.overrideWithValue(prefStorage),
              secureAuthStorageProvider.overrideWithValue(secureStorage),
              authRepositoryProvider.overrideWith((ref) => repo),
            ],
            child: const MaterialApp(home: LoginScreen()),
          ),
        );
        await tester.pumpAndSettle();

        // Tap lock icon to enter edit mode
        await tester.tap(find.byKey(const Key('login-api-url-lock-btn')));
        await tester.pumpAndSettle();

        var urlField = tester.widget<TextField>(
          find.byKey(const Key('login-api-url-field')),
        );
        expect(urlField.readOnly, isFalse);
        expect(
          find.descendant(
            of: find.byKey(const Key('login-api-url-lock-btn')),
            matching: find.byIcon(Icons.lock_open_outlined),
          ),
          findsOneWidget,
        );
        expect(find.byKey(const Key('login-api-url-save-btn')), findsOneWidget);
        expect(find.byKey(const Key('login-api-url-cancel-btn')), findsOneWidget);
        expect(find.byKey(const Key('login-api-url-reset-btn')), findsOneWidget);

        // Enter local HTTP development URL with trailing slash
        await tester.enterText(
          find.byKey(const Key('login-api-url-field')),
          'http://192.168.1.10:8080/',
        );
        await tester.tap(find.byKey(const Key('login-api-url-save-btn')));
        await tester.pumpAndSettle();

        // Normalized, saved, locked again, session isolated
        urlField = tester.widget<TextField>(
          find.byKey(const Key('login-api-url-field')),
        );
        expect(urlField.controller?.text, equals('http://192.168.1.10:8080'));
        expect(urlField.readOnly, isTrue);
        expect(client.baseUrl, equals('http://192.168.1.10:8080'));
        expect(client.token, isNull);
        expect(client.refreshToken, isNull);
        expect(await secureStorage.getSession(), isNull);
        expect(
          await prefStorage.getLastApiUrl(),
          equals('http://192.168.1.10:8080'),
        );
        expect(
          find.byKey(const Key('login-custom-server-indicator')),
          findsOneWidget,
        );

        // Quick Reset to Official Server works and hides indicator
        await tester.tap(find.byKey(const Key('login-api-url-quick-reset-btn')));
        await tester.pumpAndSettle();

        urlField = tester.widget<TextField>(
          find.byKey(const Key('login-api-url-field')),
        );
        expect(urlField.controller?.text, equals('https://api.urena0216.id'));
        expect(client.baseUrl, equals('https://api.urena0216.id'));
        expect(
          await prefStorage.getLastApiUrl(),
          equals('https://api.urena0216.id'),
        );
        expect(
          find.byKey(const Key('login-custom-server-indicator')),
          findsNothing,
        );
      },
    );

    testWidgets(
      'cancel edit reverts input and invalid URL is rejected without saving',
      (tester) async {
        final prefStorage = InMemoryPreferencesStorage();
        final secureStorage = InMemorySecureAuthStorage();
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(client),
              preferencesStorageProvider.overrideWithValue(prefStorage),
              secureAuthStorageProvider.overrideWithValue(secureStorage),
              authRepositoryProvider.overrideWith((ref) => repo),
            ],
            child: const MaterialApp(home: LoginScreen()),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('login-api-url-lock-btn')));
        await tester.pumpAndSettle();

        // Enter invalid scheme
        await tester.enterText(
          find.byKey(const Key('login-api-url-field')),
          'ftp://192.168.1.10:8080',
        );
        await tester.tap(find.byKey(const Key('login-api-url-save-btn')));
        await tester.pumpAndSettle();

        // Remains in edit mode and not persisted
        expect(
          find.textContaining('Hanya protokol HTTP dan HTTPS'),
          findsWidgets,
        );
        expect(client.baseUrl, equals('https://api.urena0216.id'));
        expect(await prefStorage.getLastApiUrl(), isNull);

        // Cancel reverts to official server and locks field
        await tester.tap(find.byKey(const Key('login-api-url-cancel-btn')));
        await tester.pumpAndSettle();

        final urlField = tester.widget<TextField>(
          find.byKey(const Key('login-api-url-field')),
        );
        expect(urlField.controller?.text, equals('https://api.urena0216.id'));
        expect(urlField.readOnly, isTrue);
      },
    );

    test(
      'persisted custom HTTP endpoint survives app restart (restoreSession overrides default official URL)',
      () async {
        final prefStorage = InMemoryPreferencesStorage(
          initialUrl: 'http://192.168.1.10:8080',
        );
        final secureStorage = InMemorySecureAuthStorage();

        // Simulate fresh app launch where ApiClient initializes to official default
        final freshClient = ApiClient();
        expect(freshClient.baseUrl, equals(ApiConfig.defaultOfficialBaseUrl));

        final freshRepo = AuthRepository(
          authService: AuthService(freshClient),
          apiClient: freshClient,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await freshRepo.restoreSession();

        // Must restore persisted custom HTTP URL over default official URL
        expect(freshClient.baseUrl, equals('http://192.168.1.10:8080'));
      },
    );
  });

  group('Point 2 — GitOps Login Background Responsiveness', () {
    for (final entry in const [
      ('small portrait 320x568', Size(320, 568)),
      ('iPhone portrait 390x844', Size(390, 844)),
      ('large tablet 834x1194', Size(834, 1194)),
      ('landscape 844x390', Size(844, 390)),
    ]) {
      testWidgets('renders GitOpsLoginBackground cleanly at ${entry.$1}', (
        tester,
      ) async {
        await tester.binding.setSurfaceSize(entry.$2);
        addTearDown(() => tester.binding.setSurfaceSize(null));

        final prefStorage = InMemoryPreferencesStorage();
        final secureStorage = InMemorySecureAuthStorage();
        final client = ApiClient();
        final repo = AuthRepository(
          authService: AuthService(client),
          apiClient: client,
          preferencesStorage: prefStorage,
          secureAuthStorage: secureStorage,
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              apiClientProvider.overrideWithValue(client),
              preferencesStorageProvider.overrideWithValue(prefStorage),
              secureAuthStorageProvider.overrideWithValue(secureStorage),
              authRepositoryProvider.overrideWith((ref) => repo),
            ],
            child: const MaterialApp(home: LoginScreen()),
          ),
        );
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byType(GitOpsLoginBackground), findsOneWidget);
        expect(
          find.byKey(const Key('gitops-login-background-paint')),
          findsOneWidget,
        );
      });
    }

    test('computes responsive panel metrics preserving 300:340 ratio', () {
      final phone = GitOpsLoginBackgroundPainter.computePanelMetrics(
        const Size(390, 844),
      );
      expect(phone.panelW, closeTo(195.0, 0.1));
      expect(phone.panelH / phone.panelW, closeTo(340.0 / 300.0, 0.01));

      final tablet = GitOpsLoginBackgroundPainter.computePanelMetrics(
        const Size(834, 1194),
      );
      expect(tablet.panelW, equals(300.0));
      expect(tablet.panelH, equals(340.0));
    });
  });
}
