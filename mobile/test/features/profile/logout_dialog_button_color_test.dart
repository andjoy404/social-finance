import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/profile/presentation/screens/profile_screen.dart';

class _LogoutTestAuthRepo extends AuthRepository {
  int logoutCalls = 0;

  _LogoutTestAuthRepo(ApiClient client, Map<String, dynamic> initialUser)
    : super(
        authService: AuthService(client),
        apiClient: client,
      ) {
    state = AsyncData(initialUser);
  }

  @override
  Future<void> reloadProfile() async {}

  @override
  Future<void> logout() async {
    logoutCalls++;
  }
}

void main() {
  group('Point 3 — Logout Confirmation Dialog Batal Button Color', () {
    for (final platform in const [TargetPlatform.android, TargetPlatform.iOS]) {
      testWidgets(
        'Batal button uses GitOps violet (#A970FF) on ${platform.name} without changing Keluar or other dialogs',
        (tester) async {
          final client = ApiClient();
          final user = {
            'id': 'user-1',
            'name': 'Budi Santoso',
            'email': 'budi@example.com',
            'phone': '081234567890',
            'role': AppRole.warga,
            'rt': '002',
            'rw': '003',
          };
          final repo = _LogoutTestAuthRepo(client, user);

          await tester.pumpWidget(
            ProviderScope(
              overrides: [
                authRepositoryProvider.overrideWith((ref) => repo),
              ],
              child: MaterialApp(
                theme: ThemeData(platform: platform),
                home: const ProfileScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          // Scroll to Keluar button and tap
          final logoutBtn = find.text('Keluar');
          await tester.ensureVisible(logoutBtn);
          await tester.tap(logoutBtn);
          await tester.pumpAndSettle();

          // Confirm dialog appears
          expect(find.text('Konfirmasi Keluar'), findsOneWidget);
          expect(
            find.text('Apakah Anda yakin ingin keluar dari aplikasi?'),
            findsOneWidget,
          );

          // Verify Batal button uses #A970FF (works on both TextButton baseline & OutlinedButton WIP)
          final batalBtnFinder = find.descendant(
            of: find.byType(AlertDialog),
            matching: find.ancestor(
              of: find.text('Batal'),
              matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
            ),
          );
          expect(batalBtnFinder, findsOneWidget);
          final batalBtn = tester.widget<ButtonStyleButton>(batalBtnFinder);
          final batalBg = batalBtn.style?.backgroundColor?.resolve({});
          expect(batalBg, equals(const Color(0xFFA970FF)));

          // Verify Keluar button remains AppColors.expense
          final dialogKeluarFinder = find.descendant(
            of: find.byType(AlertDialog),
            matching: find.ancestor(
              of: find.text('Keluar'),
              matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
            ),
          );
          expect(dialogKeluarFinder, findsOneWidget);
          final keluarBtn = tester.widget<ButtonStyleButton>(dialogKeluarFinder);
          expect(
            keluarBtn.style?.backgroundColor?.resolve({}),
            equals(AppColors.expense),
          );

          // Tapping Batal dismisses dialog without triggering logout
          await tester.tap(find.text('Batal'));
          await tester.pumpAndSettle();
          expect(find.text('Konfirmasi Keluar'), findsNothing);
          expect(repo.logoutCalls, equals(0));
        },
      );
    }
  });
}
