import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

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
}
