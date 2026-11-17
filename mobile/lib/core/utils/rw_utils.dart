import 'package:flutter/material.dart';
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

/// Style definition (background, text color, and border) for RW badges ensuring
/// high contrast and crisp readability on both light and dark themes.
class RwBadgeStyle {
  final Color background;
  final Color text;
  final Color border;

  const RwBadgeStyle({
    required this.background,
    required this.text,
    required this.border,
  });

  /// Success / Income / Active badge (Green)
  /// - Light mode: soft green background (#DCFCE7) with dark green text (#14532D)
  /// - Dark mode: dark green background (#142817) with vibrant light green text (#4ADE80)
  static RwBadgeStyle success({required bool isDark}) {
    return RwBadgeStyle(
      background: isDark ? const Color(0xFF142817) : const Color(0xFFDCFCE7),
      text: isDark ? const Color(0xFF4ADE80) : const Color(0xFF14532D),
      border: isDark ? const Color(0xFF22542B) : const Color(0xFF86EFAC),
    );
  }

  /// Danger / Expense / Inactive / Reversal badge (Red)
  /// - Light mode: soft red background (#FEE2E2) with dark red text (#991B1B)
  /// - Dark mode: dark red background (#2D1418) with vibrant light red text (#F87171)
  static RwBadgeStyle danger({required bool isDark}) {
    return RwBadgeStyle(
      background: isDark ? const Color(0xFF2D1418) : const Color(0xFFFEE2E2),
      text: isDark ? const Color(0xFFF87171) : const Color(0xFF991B1B),
      border: isDark ? const Color(0xFF5C2229) : const Color(0xFFFCA5A5),
    );
  }

  /// Warning / Brankas badge (Orange/Amber)
  /// - Light mode: soft amber background (#FEF3C7) with dark amber text (#9A3412)
  /// - Dark mode: dark amber background (#2E1F12) with vibrant amber text (#FB923C)
  static RwBadgeStyle warning({required bool isDark}) {
    return RwBadgeStyle(
      background: isDark ? const Color(0xFF2E1F12) : const Color(0xFFFEF3C7),
      text: isDark ? const Color(0xFFFB923C) : const Color(0xFF9A3412),
      border: isDark ? const Color(0xFF5E3917) : const Color(0xFFFCD34D),
    );
  }

  /// Info / Rekening Bank badge (Blue)
  /// - Light mode: soft blue background (#DBEAFE) with dark blue text (#1E40AF)
  /// - Dark mode: dark blue background (#121E33) with vibrant blue text (#60A5FA)
  static RwBadgeStyle info({required bool isDark}) {
    return RwBadgeStyle(
      background: isDark ? const Color(0xFF121E33) : const Color(0xFFDBEAFE),
      text: isDark ? const Color(0xFF60A5FA) : const Color(0xFF1E40AF),
      border: isDark ? const Color(0xFF1E3A6B) : const Color(0xFF93C5FD),
    );
  }

  /// Accent / Violet badge (Keuangan RW, Periode)
  /// - Light mode: soft violet background (#F3E8FF) with dark violet text (#581C87)
  /// - Dark mode: dark violet background (#25163A) with vibrant violet text (#C084FC)
  static RwBadgeStyle accent({required bool isDark}) {
    return RwBadgeStyle(
      background: isDark ? const Color(0xFF25163A) : const Color(0xFFF3E8FF),
      text: isDark ? const Color(0xFFC084FC) : const Color(0xFF581C87),
      border: isDark ? const Color(0xFF4C287A) : const Color(0xFFD8B4FE),
    );
  }

  /// Neutral badge (Gray)
  /// - Light mode: soft gray background (#F3F4F6) with dark gray text (#374151)
  /// - Dark mode: dark gray background (#262626) with light gray text (#D1D5DB)
  static RwBadgeStyle neutral({required bool isDark}) {
    return RwBadgeStyle(
      background: isDark ? const Color(0xFF262626) : const Color(0xFFF3F4F6),
      text: isDark ? const Color(0xFFD1D5DB) : const Color(0xFF374151),
      border: isDark ? const Color(0xFF404040) : const Color(0xFFE5E7EB),
    );
  }
}

// =============================================================================
// RW Source-Aware Permission Helpers
// =============================================================================

/// Helper untuk mengecek apakah user memiliki akses tulis (write) ke RW tertentu.
///
/// Berdasarkan Final Hak Akses Matrix:
/// - Super Admin selalu dapat write.
/// - Membership dengan source === 'explicit' AND jabatan in ('ketua', 'bendahara') → write access.
/// - Pengurus RW lain (wakil_ketua, sekretaris) → read-only.
/// - Membership dengan source === 'rt_inferred' → read-only (dashboard only).
/// - Tidak ada membership → no access.
bool rwMembershipHasWriteAccess(
  Map<String, dynamic>? user,
  String rwId, {
  List<Map<String, dynamic>>? rwMemberships,
}) {
  if (user == null) return false;
  if (user['role'] == 'super_admin' || user['system_role'] == 'super_admin') return true;

  final memberships = rwMemberships ??
      ((user['rw_memberships'] as List?)
          ?.whereType<Map<String, dynamic>>()
          .toList() ?? []);

  final currentMembership = memberships
      .where((m) => m['rw_id'] == rwId)
      .firstOrNull;

  if (currentMembership == null) return false;

  final source = currentMembership['source'] as String?;
  if (source != 'explicit') return false;

  final jabatan = (currentMembership['jabatan'] as String? ?? '').toLowerCase();
  return jabatan == 'ketua' || jabatan == 'bendahara';
}

/// Helper untuk mengecek apakah user memiliki akses fitur RW (KAS, Lokasi Dana) - explicit members only.
bool rwMembershipHasFeatureAccess(
  Map<String, dynamic>? user,
  String rwId, {
  List<Map<String, dynamic>>? rwMemberships,
}) {
  if (user == null) return false;
  if (user['role'] == 'super_admin' || user['system_role'] == 'super_admin') return true;

  final memberships = rwMemberships ??
      ((user['rw_memberships'] as List?)
          ?.whereType<Map<String, dynamic>>()
          .toList() ?? []);

  final currentMembership = memberships
      .where((m) => m['rw_id'] == rwId)
      .firstOrNull;

  if (currentMembership == null) return false;
  final source = currentMembership['source'] as String?;
  return source == 'explicit';
}
