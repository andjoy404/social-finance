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
