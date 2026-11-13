import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/core/widgets/resident_avatar.dart';

void main() {
  testWidgets('ResidentAvatar renders initials fallback when no avatar_path', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: ResidentAvatar(name: 'Budi Santoso')),
      ),
    );
    expect(find.text('BS'), findsOneWidget);
  });

  testWidgets('ResidentAvatar shows ? for empty name', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: ResidentAvatar(name: '')),
      ),
    );
    expect(find.text('?'), findsOneWidget);
  });

  testWidgets('ResidentAvatar initials for single word', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(body: ResidentAvatar(name: 'Hendra')),
      ),
    );
    expect(find.text('H'), findsOneWidget);
  });

  testWidgets('ResidentAvatar prefers avatarThumbPath with avatarPath as fallback', (tester) async {
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: ResidentAvatar(
              name: 'Ahmad Dahlan',
              avatarPath: 'community/rt-1/profile/orig.jpg',
              avatarThumbPath: 'community/rt-1/profile/thumb.jpg',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final imageFinder = find.byType(CommunityImage);
    expect(imageFinder, findsOneWidget);
    final widget = tester.widget<CommunityImage>(imageFinder);
    expect(widget.imageRef, equals('community/rt-1/profile/thumb.jpg'));
    expect(widget.fallbackRef, equals('community/rt-1/profile/orig.jpg'));
  });

  testWidgets('ResidentAvatar uses avatarPath when avatarThumbPath is null', (tester) async {
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: ResidentAvatar(
              name: 'Ahmad Dahlan',
              avatarPath: 'community/rt-1/profile/orig.jpg',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final imageFinder = find.byType(CommunityImage);
    expect(imageFinder, findsOneWidget);
    final widget = tester.widget<CommunityImage>(imageFinder);
    expect(widget.imageRef, equals('community/rt-1/profile/orig.jpg'));
    expect(widget.fallbackRef, isNull);
  });
}
