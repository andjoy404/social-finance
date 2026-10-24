import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/app/navigation_shell.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

void main() {
  group('RT Restriction - Auth Models & Helper', () {
    test('AuthUser.fromJson parses rt_rt and nested rt.rt into rtCode', () {
      final jsonWithRtRt = {
        'id': 'user-1',
        'email': 'user1@local.com',
        'full_name': 'User One',
        'role': 'bendahara',
        'rt_id': '8dfa1e36-6727-45be-8fea-24d958efc155',
        'rt_rt': '003',
        'rt_name': 'RT 003 Urena',
        'rt_address': 'Jl. Urena 1',
        'rw_name': 'RW 016',
      };
      final user1 = AuthUser.fromJson(jsonWithRtRt);
      expect(user1.rtId, equals('8dfa1e36-6727-45be-8fea-24d958efc155'));
      expect(user1.rtCode, equals('003'));
      expect(user1.rtName, equals('RT 003 Urena'));
      expect(user1.rtAddress, equals('Jl. Urena 1'));
      expect(user1.rwName, equals('RW 016'));

      final jsonWithNestedRt = {
        'id': 'user-2',
        'email': 'user2@local.com',
        'full_name': 'User Two',
        'role': 'warga',
        'rt': {
          'id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          'name': 'Urena',
          'rt': '002',
          'rw': 16,
        },
      };
      final user2 = AuthUser.fromJson(jsonWithNestedRt);
      expect(user2.rtId, equals('a95c1cb5-2f67-4ea5-9197-bfec537b6d7b'));
      expect(user2.rtCode, equals('002'));
    });

    test(
      'isAllowedRT correctly identifies allowed RTs (002, 003) and rejects non-allowed RTs',
      () {
        // Allowed RT by code 002
        expect(isAllowedRT({'rt': '002', 'rt_id': 'any-id'}), isTrue);
        // Allowed RT by code 003
        expect(isAllowedRT({'rt': '003', 'rt_id': 'any-id'}), isTrue);
        // Allowed RT by canonical ID RT 002
        expect(
          isAllowedRT({
            'rt': null,
            'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
          }),
          isTrue,
        );
        // Allowed RT by canonical ID RT 003
        expect(
          isAllowedRT({
            'rt': null,
            'rt_id': '8dfa1e36-6727-45be-8fea-24d958efc155',
          }),
          isTrue,
        );

        // Non-allowed RTs (004, 005, 006)
        expect(
          isAllowedRT({
            'rt': '004',
            'rt_id': 'efb70405-09f5-48a0-ad14-34802cf09e3b',
          }),
          isFalse,
        );
        expect(
          isAllowedRT({
            'rt': '005',
            'rt_id': '5106197a-c4e4-4bbc-8ef7-921d7208717d',
          }),
          isFalse,
        );
        expect(
          isAllowedRT({
            'rt': '006',
            'rt_id': 'a3390720-dd56-4b28-92b1-43f384f584cd',
          }),
          isFalse,
        );
        expect(isAllowedRT({'rt': '001', 'rt_id': 'unknown-id'}), isFalse);

        // Super admin passes
        expect(isAllowedRT({'system_role': 'super_admin'}), isTrue);

        // Null user
        expect(isAllowedRT(null), isFalse);
      },
    );
  });

  group('RT Restriction - Navigation Shell UI', () {
    testWidgets(
      'displays all 5 tabs (Beranda, Kas, Iuran, Warga, Profil) for RT 002/003 user',
      (tester) async {
        final allowedUser = {
          'id': 'user-allowed',
          'email': 'bendahara-002@local.com',
          'name': 'Bendahara RT 002',
          'role': AppRole.bendahara,
          'rt': '002',
          'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        };

        final router = GoRouter(
          initialLocation: '/home',
          routes: [
            ShellRoute(
              builder: (context, state, child) =>
                  MainNavigationShell(child: child),
              routes: [
                GoRoute(
                  path: '/home',
                  builder: (context, state) => const Text('Home Screen'),
                ),
                GoRoute(
                  path: '/home/kas',
                  builder: (context, state) => const Text('Kas Screen'),
                ),
                GoRoute(
                  path: '/home/iuran',
                  builder: (context, state) => const Text('Iuran Screen'),
                ),
                GoRoute(
                  path: '/home/warga',
                  builder: (context, state) => const Text('Warga Screen'),
                ),
                GoRoute(
                  path: '/home/profile',
                  builder: (context, state) => const Text('Profile Screen'),
                ),
              ],
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _FakeAuthRepository(AsyncData(allowedUser)),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Beranda'), findsOneWidget);
        expect(find.text('Kas'), findsOneWidget);
        expect(find.text('Iuran'), findsOneWidget);
        expect(find.text('Warga'), findsOneWidget);
        expect(find.text('Profil'), findsOneWidget);
      },
    );

    testWidgets(
      'hides RT-scoped tabs (Kas, Iuran, Warga) and only shows Beranda and Profil for RT 004 user',
      (tester) async {
        final nonAllowedUser = {
          'id': 'user-non-allowed',
          'email': 'bendahara-004@local.com',
          'name': 'Bendahara RT 004',
          'role': AppRole.bendahara,
          'rt': '004',
          'rt_id': 'efb70405-09f5-48a0-ad14-34802cf09e3b',
        };

        final router = GoRouter(
          initialLocation: '/home',
          routes: [
            ShellRoute(
              builder: (context, state, child) =>
                  MainNavigationShell(child: child),
              routes: [
                GoRoute(
                  path: '/home',
                  builder: (context, state) => const Text('Home Screen'),
                ),
                GoRoute(
                  path: '/home/kas',
                  builder: (context, state) => const Text('Kas Screen'),
                ),
                GoRoute(
                  path: '/home/iuran',
                  builder: (context, state) => const Text('Iuran Screen'),
                ),
                GoRoute(
                  path: '/home/warga',
                  builder: (context, state) => const Text('Warga Screen'),
                ),
                GoRoute(
                  path: '/home/profile',
                  builder: (context, state) => const Text('Profile Screen'),
                ),
              ],
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authRepositoryProvider.overrideWith(
                (ref) => _FakeAuthRepository(AsyncData(nonAllowedUser)),
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Beranda'), findsOneWidget);
        expect(find.text('Profil'), findsOneWidget);
        expect(find.text('Kas'), findsNothing);
        expect(find.text('Iuran'), findsNothing);
        expect(find.text('Warga'), findsNothing);
      },
    );
  });
}

class _FakeAuthRepository
    extends StateNotifier<AsyncValue<Map<String, dynamic>?>?>
    implements AuthRepository {
  _FakeAuthRepository(super.state);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}
