import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Provider yang tracking apakah user sedang berada di form KAS Masuk/Keluar.
/// Form KAS dibuka sebagai modal route (context.push) di atas ShellRoute,
/// sehingga GoRouter location tidak berubah — kita perlu provider untuk
/// mendeteksi keberadaan form.
final class KasFormState {
  final bool isOnKasForm;
  const KasFormState({required this.isOnKasForm});
}

final kasFormProvider = StateProvider<KasFormState>((ref) {
  return const KasFormState(isOnKasForm: false);
});
