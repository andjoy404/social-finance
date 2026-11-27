import '../models/role.dart';

/// Client-side permission helpers.
///
/// NOTE: client-side permission checks are a UX concern only. All
/// authorization is enforced server-side. These helpers exist so the UI
/// only offers actions the backend will accept; they are NEVER the
/// security boundary.
bool userHasPermission(Map<String, dynamic>? user, String permission) {
  if (user == null) return false;
  final role = user['role'];
  if (role == AppRole.superAdmin) return true;
  final raw = user['effective_permissions'];
  final perms = raw is List
      ? raw.whereType<String>().toSet()
      : const <String>{};
  return perms.contains(permission);
}

/// Activities management: create or update grants draft management.
bool canManageActivities(Map<String, dynamic>? user) =>
    userHasPermission(user, 'activities.create') ||
    userHasPermission(user, 'activities.update');

/// Announcements management: create or update grants draft management.
bool canManageAnnouncements(Map<String, dynamic>? user) =>
    userHasPermission(user, 'announcements.create') ||
    userHasPermission(user, 'announcements.update');

/// Resident profile-photo management: upload, replace, delete, and view
/// action for residents' profile photos (Warga biasa and Petugas Keamanan).
/// Server enforces this; this helper only hides/shows the UI actions.
bool canManageResidentProfilePhoto(Map<String, dynamic>? user) =>
    userHasPermission(user, 'resident.profile_photo.manage');

const _rwNotificationWritePermissions = <String>{
  'rw.notification.write',
  'rw.notification.manage',
  'notification.manage',
  'notification.write',
};

const _rwNotificationReadPermissions = <String>{
  'rw.notification.read',
  'notification.read',
};

bool _isSuperAdminPermissionUser(Map<String, dynamic>? user) {
  if (user == null) return false;
  final role = user['role'];
  if (role == AppRole.superAdmin) return true;
  final roleStr = role?.toString().trim().toLowerCase() ?? '';
  final sysRole = (user['system_role'] ?? '').toString().trim().toLowerCase();
  return roleStr == 'super_admin' || sysRole == 'super_admin';
}

Map<String, dynamic>? _explicitRwMembership(Map<String, dynamic>? user, String rwId) {
  if (user == null) return null;
  final memberships =
      (user['rw_memberships'] as List?)?.whereType<Map<String, dynamic>>() ??
      const [];
  for (final m in memberships) {
    if (m['rw_id'] == rwId && (m['source'] as String?) == 'explicit') {
      return m;
    }
  }
  return null;
}

const _rwScopedNotificationPermissions = <String>{
  'rw.notification.write',
  'rw.notification.manage',
  'rw.notification.read',
};

const _allNotificationPermissionAliases = <String>{
  'rw.notification.write',
  'rw.notification.manage',
  'rw.notification.read',
  'notification.manage',
  'notification.write',
  'notification.read',
};

/// RW Notification WRITE permission check (UI convenience; server enforces).
///
/// Resolution priority:
/// 1. Super Admin -> allowed.
/// 2. Explicit RW membership matching `rwId` is required.
/// 3. Server-resolved `membership['permissions']` for that `rwId` (primary source
///    of truth from `/api/v1/auth/me`, isolated from RT-scoped `effective_permissions`).
/// 4. If `membership['permissions']` is absent (legacy/minimal payloads):
///    - Evaluate explicit RW-scoped notification permissions (`rw.notification.*`)
///      or notification-only permission payloads in `user['effective_permissions']`.
///    - Do NOT let unrelated RT-only `effective_permissions` (e.g. `finance.read`,
///      `activities.read` for a Warga who is Ketua RW) suppress RW write access.
bool canManageRwNotifications(Map<String, dynamic>? user, String rwId) {
  if (user == null) return false;
  if (_isSuperAdminPermissionUser(user)) return true;

  final membership = _explicitRwMembership(user, rwId);
  if (membership == null) return false;

  // 1. Primary: server-resolved RW-scoped permissions on the matching membership.
  final memberPermsRaw = membership['permissions'];
  if (memberPermsRaw is List && memberPermsRaw.isNotEmpty) {
    final memberPerms =
        memberPermsRaw.whereType<String>().map((e) => e.trim()).toSet();
    return memberPerms.any(_rwNotificationWritePermissions.contains);
  }

  // 2. Secondary: RW-scoped notification permissions in effective_permissions
  //    (for explicit overrides or tests without membership.permissions).
  final rawPerms = user['effective_permissions'];
  if (rawPerms is List && rawPerms.isNotEmpty) {
    final perms = rawPerms.whereType<String>().map((e) => e.trim()).toSet();
    if (perms.any(_rwScopedNotificationPermissions.contains)) {
      return perms.any(_rwNotificationWritePermissions.contains);
    }
    // Only treat generic notification.* as RW override when the payload carries
    // solely notification permissions (not mixed RT permissions like finance.read).
    if (perms.every(_allNotificationPermissionAliases.contains)) {
      return perms.any(_rwNotificationWritePermissions.contains);
    }
  }

  // 3. Fallback when membership.permissions is absent on older backend payloads.
  final jabatan = (membership['jabatan'] as String? ?? '').trim().toLowerCase();
  return jabatan == 'ketua' || jabatan == 'bendahara';
}

/// RW Notification READ permission check (UI convenience; server enforces).
bool canReadRwNotifications(Map<String, dynamic>? user, String rwId) {
  if (user == null) return false;
  if (canManageRwNotifications(user, rwId)) return true;

  final membership = _explicitRwMembership(user, rwId);
  if (membership == null) return false;

  final memberPermsRaw = membership['permissions'];
  if (memberPermsRaw is List && memberPermsRaw.isNotEmpty) {
    final memberPerms =
        memberPermsRaw.whereType<String>().map((e) => e.trim()).toSet();
    return memberPerms.any(_rwNotificationReadPermissions.contains);
  }

  final rawPerms = user['effective_permissions'];
  if (rawPerms is List && rawPerms.isNotEmpty) {
    final perms = rawPerms.whereType<String>().map((e) => e.trim()).toSet();
    if (perms.any(_rwScopedNotificationPermissions.contains) ||
        perms.every(_allNotificationPermissionAliases.contains)) {
      return perms.any(_rwNotificationReadPermissions.contains);
    }
  }
  return true;
}
