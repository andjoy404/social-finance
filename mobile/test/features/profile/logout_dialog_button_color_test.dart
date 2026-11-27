import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/app/theme/app_theme.dart';
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
  group('Point 2 — Logout Confirmation Dialog Batal Button Violet Exception & Height Parity', () {
    for (final entry in [
      (label: 'Android Light', theme: AppTheme.lightTheme, platform: TargetPlatform.android),
      (label: 'Android Dark', theme: AppTheme.darkTheme, platform: TargetPlatform.android),
      (label: 'iOS Dark', theme: AppTheme.darkTheme, platform: TargetPlatform.iOS),
    ]) {
      testWidgets(
        'Batal button uses violet (AppColors.accent) and matches Keluar height & alignment (${entry.label})',
        (tester) async {
          await tester.binding.setSurfaceSize(const Size(390, 844));
          addTearDown(() => tester.binding.setSurfaceSize(null));

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
                theme: entry.theme.copyWith(platform: entry.platform),
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

          // Confirm dialog appears with zero overflow even on narrow 320px width
          expect(tester.takeException(), isNull);
          expect(find.text('Konfirmasi Keluar'), findsOneWidget);
          expect(
            find.text('Apakah Anda yakin ingin keluar dari aplikasi?'),
            findsOneWidget,
          );

          // Verify Batal button uses AppColors.accent (violet) with white text
          final batalBtnFinder = find.descendant(
            of: find.byType(AlertDialog),
            matching: find.ancestor(
              of: find.text('Batal'),
              matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
            ),
          );
          expect(batalBtnFinder, findsOneWidget);
          final batalBtn = tester.widget<ButtonStyleButton>(batalBtnFinder);
          expect(batalBtn.style?.backgroundColor?.resolve({}), equals(AppColors.accent));
          expect(batalBtn.style?.foregroundColor?.resolve({}), equals(Colors.white));

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

          // Verify exact same height and vertical alignment between Batal and Keluar
          final batalRect = tester.getRect(batalBtnFinder);
          final keluarRect = tester.getRect(dialogKeluarFinder);
          expect(batalRect.height, equals(keluarRect.height));
          expect(batalRect.top, equals(keluarRect.top));

          // Tapping Batal dismisses dialog without triggering logout
          await tester.tap(find.text('Batal'));
          await tester.pumpAndSettle();
          expect(find.text('Konfirmasi Keluar'), findsNothing);
          expect(repo.logoutCalls, equals(0));
        },
      );
    }

    testWidgets(
      'Violet Batal exception applies ONLY to Konfirmasi Keluar; other Profile dialogs keep red Batal',
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
              theme: AppTheme.darkTheme,
              home: const ProfileScreen(),
            ),
          ),
        );
        await tester.pumpAndSettle();

        // Open Ubah Kata Sandi dialog
        final changePassTile = find.text('Ubah kata sandi');
        await tester.ensureVisible(changePassTile);
        await tester.tap(changePassTile);
        await tester.pumpAndSettle();

        expect(find.text('Ubah Kata Sandi'), findsOneWidget);

        final batalBtnFinder = find.descendant(
          of: find.byType(AlertDialog),
          matching: find.ancestor(
            of: find.text('Batal'),
            matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
          ),
        );
        expect(batalBtnFinder, findsOneWidget);
        final batalBtn = tester.widget<ButtonStyleButton>(batalBtnFinder);
        expect(batalBtn.style?.backgroundColor?.resolve({}), equals(AppColors.danger));
        expect(batalBtn.style?.backgroundColor?.resolve({}), isNot(AppColors.accent));

        await tester.tap(find.text('Batal'));
        await tester.pumpAndSettle();
        expect(find.text('Ubah Kata Sandi'), findsNothing);
      },
    );

    testWidgets(
      'Konfirmasi Keluar dialog has zero overflow on narrow 320px screen with identical button heights',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(320, 640));
        addTearDown(() => tester.binding.setSurfaceSize(null));

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
              theme: AppTheme.darkTheme,
              home: const ProfileScreen(),
            ),
          ),
        );
        await tester.pumpAndSettle();

        final logoutBtn = find.text('Keluar');
        await tester.ensureVisible(logoutBtn);
        await tester.tap(logoutBtn);
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        final batalBtnFinder = find.descendant(
          of: find.byType(AlertDialog),
          matching: find.ancestor(
            of: find.text('Batal'),
            matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
          ),
        );
        final keluarBtnFinder = find.descendant(
          of: find.byType(AlertDialog),
          matching: find.ancestor(
            of: find.text('Keluar'),
            matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
          ),
        );
        expect(
          tester.getSize(batalBtnFinder).height,
          equals(tester.getSize(keluarBtnFinder).height),
        );
      },
    );
  });
}
