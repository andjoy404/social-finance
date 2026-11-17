String formatGuestVisitDuration(
  DateTime checkedInAt,
  DateTime? checkedOutAt, {
  DateTime? now,
}) {
  final start = checkedInAt.toLocal();
  final end = (checkedOutAt ?? (now ?? DateTime.now())).toLocal();
  var diff = end.difference(start);
  if (diff.isNegative) diff = Duration.zero;
  final days = diff.inDays;
  final hours = diff.inHours % 24;
  final minutes = diff.inMinutes % 60;
  final parts = <String>[];
  if (days > 0) parts.add('$days hari');
  if (hours > 0) parts.add('$hours jam');
  if (minutes > 0 || parts.isEmpty) parts.add('$minutes menit');
  return parts.join(' ');
}

String guestVisitDurationLabel(
  DateTime checkedInAt,
  DateTime? checkedOutAt, {
  DateTime? now,
}) {
  final duration = formatGuestVisitDuration(
    checkedInAt,
    checkedOutAt,
    now: now,
  );
  if (checkedOutAt == null) {
    return 'Sedang berkunjung · $duration';
  }
  return 'Durasi kunjungan · $duration';
}
