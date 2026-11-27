import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/widgets/app_button.dart';

void main() {
  group('AppButton Design System Tests', () {
    testWidgets('Primary button uses AppColors.success and white foreground', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AppButton.primary(
              text: 'Simpan',
              onPressed: () {},
            ),
          ),
        ),
      );

      final elevatedButton = tester.widget<ElevatedButton>(find.byType(ElevatedButton));
      expect(elevatedButton.style?.backgroundColor?.resolve({}), AppColors.success);
      expect(elevatedButton.style?.foregroundColor?.resolve({}), Colors.white);
      expect(find.text('Simpan'), findsOneWidget);
    });

    testWidgets('Danger button uses AppColors.danger and white foreground', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AppButton.danger(
              text: 'Hapus',
              onPressed: () {},
            ),
          ),
        ),
      );

      final elevatedButton = tester.widget<ElevatedButton>(find.byType(ElevatedButton));
      expect(elevatedButton.style?.backgroundColor?.resolve({}), AppColors.danger);
      expect(elevatedButton.style?.foregroundColor?.resolve({}), Colors.white);
      expect(find.text('Hapus'), findsOneWidget);
    });

    testWidgets('DangerOutline button standardized to solid AppColors.danger with white text in Light & Dark mode', (tester) async {
      for (final brightness in [Brightness.light, Brightness.dark]) {
        await tester.pumpWidget(
          MaterialApp(
            key: ValueKey(brightness),
            theme: ThemeData(brightness: brightness),
            home: Scaffold(
              body: AppButton.dangerOutline(
                text: 'Batal',
                onPressed: () {},
              ),
            ),
          ),
        );

        final elevatedButton = tester.widget<ElevatedButton>(find.byType(ElevatedButton));
        expect(elevatedButton.style?.backgroundColor?.resolve({}), AppColors.danger);
        expect(elevatedButton.style?.foregroundColor?.resolve({}), Colors.white);
        expect(find.text('Batal'), findsOneWidget);
      }
    });

    testWidgets('Secondary button uses neutral/surface styling and theme colors', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AppButton.secondary(
              text: 'Kembali',
              onPressed: () {},
            ),
          ),
        ),
      );

      final button = tester.widget<OutlinedButton>(find.byType(OutlinedButton));
      expect(button.style?.foregroundColor?.resolve({}), isNot(AppColors.danger));
      expect(find.text('Kembali'), findsOneWidget);
    });

    testWidgets('AccentSoft button uses soft background, clear visible border, and consistent height in Light & Dark mode', (tester) async {
      for (final brightness in [Brightness.light, Brightness.dark]) {
        var tapped = false;
        await tester.pumpWidget(
          MaterialApp(
            key: ValueKey(brightness),
            theme: ThemeData(brightness: brightness),
            home: Scaffold(
              body: SizedBox(
                width: 150,
                child: AppButton.accentSoft(
                  text: 'Kamera',
                  icon: Icons.camera_alt_outlined,
                  fullWidth: true,
                  onPressed: () => tapped = true,
                ),
              ),
            ),
          ),
        );

        final button = tester.widget<OutlinedButton>(find.byType(OutlinedButton));
        final expectedBg = brightness == Brightness.dark
            ? AppColors.darkAccentSoft
            : AppColors.lightAccentSoft;
        expect(button.style?.backgroundColor?.resolve({}), equals(expectedBg));

        final side = button.style?.side?.resolve({});
        expect(side, isNotNull);
        expect(side!.width, greaterThan(0));
        expect(side.color.a, greaterThan(0));

        expect(tester.getSize(find.byType(OutlinedButton)).height, equals(40.0));
        expect(find.text('Kamera'), findsOneWidget);

        await tester.tap(find.byType(OutlinedButton));
        await tester.pump();
        expect(tapped, isTrue);
        expect(tester.takeException(), isNull);
      }
    });

    testWidgets('Form size produces 48px height', (tester) async {
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AppButton.primary(
              text: 'Simpan',
              size: AppButtonSize.form,
              onPressed: () {},
            ),
          ),
        ),
      );

      final buttonFinder = find.byType(ElevatedButton);
      final size = tester.getSize(buttonFinder);
      expect(size.height, 48.0);
    });

    testWidgets('Loading state prevents double-submission and displays spinner', (tester) async {
      var callCount = 0;
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: AppButton.primary(
              text: 'Simpan',
              isLoading: true,
              onPressed: () {
                callCount++;
              },
            ),
          ),
        ),
      );

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('Simpan'), findsNothing);

      await tester.tap(find.byType(AppButton));
      await tester.pump();

      expect(callCount, 0);
    });

    testWidgets('Disabled state when onPressed is null', (tester) async {
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: AppButton.primary(
              text: 'Simpan',
              onPressed: null,
            ),
          ),
        ),
      );

      final elevatedButton = tester.widget<ElevatedButton>(find.byType(ElevatedButton));
      expect(elevatedButton.onPressed, isNull);
    });
  });
}
