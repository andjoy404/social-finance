import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';
import 'package:social_finance/features/security/data/security_models.dart';
import 'package:social_finance/features/security/presentation/providers/security_providers.dart';
import 'package:social_finance/features/security/presentation/screens/security_guest_book_screen.dart';

class _Api extends SecurityApiClient {
  _Api() : super(ApiClient(baseUrl: 'http://localhost'));
  String? search;
  int page = 0;

  @override
  Future<GuestBookPage> listManagerGuestBooks({String? search, int page = 1, String? status}) async {
    this.search = search;
    this.page = page;
    final items = search == null || search.isEmpty
        ? [_activeEntry('A'), _checkedOutEntry('B')]
        : (search.toLowerCase().contains('kurir') ? [_activeEntry('Kurir Paket')] : <GuestBookEntry>[]);
    return GuestBookPage(items, items.length, 1);
  }

  GuestBookEntry _activeEntry(String name) => GuestBookEntry(
    id: '$name-active',
    rtId: 'rt-1',
    deviceId: 'device-1',
    deviceName: 'Pos Utama',
    guestName: name,
    purpose: 'Kunjungan',
    destinationLabel: 'Rumah 1',
    checkedInAt: DateTime.now().subtract(const Duration(minutes: 5)),
  );

  GuestBookEntry _checkedOutEntry(String name) => GuestBookEntry(
    id: '$name-out',
    rtId: 'rt-1',
    deviceId: 'device-1',
    deviceName: 'Pos Utama',
    guestName: name,
    purpose: 'Selesai',
    destinationLabel: 'Rumah 2',
    checkedInAt: DateTime(2026, 11, 12, 10, 0),
    checkedOutAt: DateTime(2026, 11, 12, 11, 10),
  );
}

void main() {
  testWidgets('Buku Tamu searches, clears, shows empty state and opens read-only detail with duration', (tester) async {
    final api = _Api();
    await tester.pumpWidget(ProviderScope(overrides: [securityApiClientProvider.overrideWithValue(api)], child: const MaterialApp(home: SecurityGuestBookScreen())));
    await tester.pumpAndSettle();
    expect(find.text('A → Rumah 1'), findsOneWidget);
    expect(find.text('B → Rumah 2'), findsOneWidget);
    expect(find.textContaining('Sedang berkunjung'), findsWidgets);
    expect(find.textContaining('Durasi kunjungan'), findsWidgets);

    await tester.enterText(find.byType(TextField), 'kurir');
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pumpAndSettle();
    expect(api.search, 'kurir');
    expect(find.text('Kurir Paket → Rumah 1'), findsOneWidget);

    await tester.tap(find.byIcon(Icons.chevron_right).first);
    await tester.pumpAndSettle();
    expect(find.textContaining('Perangkat: Pos Utama'), findsOneWidget);
    expect(find.text('Catat'), findsNothing);

    await tester.tap(find.text('Tutup'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'tidak-ada');
    await tester.pump(const Duration(milliseconds: 400));
    await tester.pumpAndSettle();
    expect(find.text('Tidak ada catatan Buku Tamu.'), findsOneWidget);
  });
}
