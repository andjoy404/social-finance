import '../api/models.dart';

/// Utility untuk mentransform UUID RW menjadi RW Number (string "RW 016").
///
/// Digunakan di semua tempat UI yang menerima rw_id (UUID) dari backend
/// dan perlu menampilkannya sebagai RW Number yang user-friendly.

final _uuidRegExp = RegExp(
  r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
);

/// Check if a string looks like a UUID (36 chars with hyphens, canonical hex format or test UUID).
bool looksLikeUuid(String? s) {
  if (s == null) return false;
  final trimmed = s.trim();
  if (trimmed.length == 36 && trimmed.contains('-')) return true;
  return _uuidRegExp.hasMatch(trimmed);
}

/// Internal alias for [looksLikeUuid] to maintain backward compatibility.
bool _looksLikeUuid(String? s) => looksLikeUuid(s);

/// Transform a value that might be a UUID into a readable RW display string.
///
/// - If [rwId] is a UUID, returns null (caller should resolve from memberships).
/// - Otherwise returns [rwId] directly (already a readable number like "002").
String? rwIdToDisplayString(String? rwId) {
  if (rwId == null || rwId.trim().isEmpty) return null;
  if (_looksLikeUuid(rwId)) return null;
  return rwId.trim();
}

/// Extracts the 3-digit zero-padded RW number (e.g. "016") from a [rwId] and [rwMemberships].
///
/// Supports [rwMemberships] as `List<RWMembership>`, `List<Map<String, dynamic>>`,
/// or any `Iterable`.
/// Returns `null` if [rwId] is a UUID that cannot be matched to any membership,
/// if the matched membership does not have an rwNumber, or if [rwId] is null/empty.
String? resolveRwNumber(String? rwId, dynamic rwMemberships) {
  if (rwId == null || rwId.trim().isEmpty) return null;
  final cleanId = rwId.trim();

  if (rwMemberships is Iterable) {
    for (final item in rwMemberships) {
      if (item is RWMembership) {
        if (item.rwId == cleanId) {
          if (item.rwNumber != null) {
            return item.rwNumber!.toString().padLeft(3, '0');
          }
          return null;
        }
      } else if (item is Map) {
        final mId = (item['rw_id'] ?? item['id'] ?? '').toString().trim();
        if (mId == cleanId) {
          final numVal = item['rw_number'];
          if (numVal != null) {
            final n = numVal is int
                ? numVal
                : (numVal is num ? numVal.toInt() : int.tryParse(numVal.toString()));
            if (n != null) {
              return n.toString().padLeft(3, '0');
            }
          }
          return null;
        }
      }
    }
  }

  // If rwId is an identifier (UUID, test UUID, or contains hyphens), do NOT extract digits from it.
  if (looksLikeUuid(cleanId) || cleanId.contains('-') || cleanId.length > 10) {
    return null;
  }

  // If rwId is already a short number (e.g. "16", "016", "RW 016").
  final digitsOnly = cleanId.replaceAll(RegExp(r'[^0-9]'), '');
  if (digitsOnly.isNotEmpty) {
    final n = int.tryParse(digitsOnly);
    if (n != null) {
      return n.toString().padLeft(3, '0');
    }
  }

  return null;
}

/// Resolve a rw_id (UUID or code) to a human-readable display string using [rwMemberships].
///
/// Returns "RW NNN" (e.g. "RW 016") if a matching membership with rw_number is found.
/// If no matching number exists, returns the membership's rwName if present,
/// or fallback "RW". Under NO circumstances is a raw UUID returned to the user.
String rwIdToReadable(
  String? rwId,
  dynamic rwMemberships,
) {
  if (rwId == null || rwId.trim().isEmpty) return 'RW';
  final cleanId = rwId.trim();

  if (rwMemberships is Iterable) {
    for (final item in rwMemberships) {
      if (item is RWMembership) {
        if (item.rwId == cleanId) {
          if (item.rwNumber != null) {
            return 'RW ${item.rwNumber!.toString().padLeft(3, '0')}';
          }
          if (item.rwName != null && item.rwName!.trim().isNotEmpty) {
            return item.rwName!.trim();
          }
          return 'RW';
        }
      } else if (item is Map) {
        final mId = (item['rw_id'] ?? item['id'] ?? '').toString().trim();
        if (mId == cleanId) {
          final numVal = item['rw_number'];
          if (numVal != null) {
            final n = numVal is int
                ? numVal
                : (numVal is num ? numVal.toInt() : int.tryParse(numVal.toString()));
            if (n != null) {
              return 'RW ${n.toString().padLeft(3, '0')}';
            }
          }
          final nameVal = item['rw_name']?.toString().trim();
          if (nameVal != null && nameVal.isNotEmpty) {
            return nameVal;
          }
          return 'RW';
        }
      }
    }
  }

  // Not found in memberships. Check if cleanId itself is already a readable code.
  if (!looksLikeUuid(cleanId) && !cleanId.contains('-') && cleanId.length <= 10) {
    if (cleanId.toUpperCase().startsWith('RW')) {
      return cleanId;
    }
    final digits = cleanId.replaceAll(RegExp(r'[^0-9]'), '');
    if (digits.isNotEmpty) {
      final n = int.tryParse(digits);
      if (n != null) {
        return 'RW ${n.toString().padLeft(3, '0')}';
      }
    }
    return cleanId;
  }

  // Fallback for unresolvable UUID or identifier: never show UUID to user.
  return 'RW';
}
