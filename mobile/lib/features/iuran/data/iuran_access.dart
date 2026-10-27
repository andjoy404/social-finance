import 'package:social_finance/core/models/role.dart';

/// Jabatan pengurus RT yang memiliki akses tulis iuran (position_permissions).
const Set<String> kIuranManageJabatans = {'ketua', 'wakil_ketua', 'bendahara'};

/// Jabatan pengurus RT yang memiliki akses verifikasi pembayaran.
const Set<String> kPaymentVerifyJabatans = {'ketua', 'bendahara'};

/// Jabatan pengurus RT yang memiliki akses management (bisa buat/edit/approve iuran).
/// Di-backend, jabatan ini juga harus punya finance.create + finance.update + finance.approve
/// di position_permissions untuk benar-benar bisa manage.
const Set<String> kIuranManagementJabatan = {
  'ketua',
  'wakil_ketua',
  'bendahara',
};

bool canReadIuran(Map<String, dynamic>? user) {
  if (user == null) return false;
  return _roleAllowsRead(user) && _hasPermission(user, 'finance.read');
}

bool canManageIuran(Map<String, dynamic>? user) {
  if (user == null) return false;
  return _roleAllowsManagement(user) &&
      _hasPermission(user, 'finance.create') &&
      _hasPermission(user, 'finance.update') &&
      _hasPermission(user, 'finance.approve');
}

bool canVerifyPayment(Map<String, dynamic>? user) {
  if (user == null) return false;
  if (user['system_role'] == 'super_admin') return true;

  final role = _roleValue(user['role']);
  final jabatan = user['jabatan'] as String?;
  final isRoleAuthorized =
      role == 'bendahara' ||
      role == 'pengurus' ||
      kPaymentVerifyJabatans.contains(jabatan ?? '');

  if (!isRoleAuthorized) return false;

  return _hasPermission(user, 'finance.approve');
}

bool canAccessIuranPath(String path, Map<String, dynamic>? user) {
  if (!path.startsWith('/home/iuran/')) return true;
  final isManagePath =
      path == '/home/iuran/kelola' || path == '/home/iuran/form';
  return isManagePath ? canManageIuran(user) : canReadIuran(user);
}

bool _roleAllowsRead(Map<String, dynamic> user) {
  if (user['system_role'] == 'super_admin') return true;
  final role = _roleValue(user['role']);
  if (role == 'warga' || role == 'bendahara' || role == 'pengurus') return true;
  final jabatan = user['jabatan'] as String?;
  if (jabatan != null && jabatan.isNotEmpty) return true;
  return false;
}

bool _roleAllowsManagement(Map<String, dynamic> user) {
  if (user['system_role'] == 'super_admin') return true;
  final role = _roleValue(user['role']);
  if (role == 'bendahara' || role == 'pengurus') return true;
  final jabatan = user['jabatan'] as String?;
  if (kIuranManagementJabatan.contains(jabatan)) return true;
  return false;
}

bool _hasPermission(Map<String, dynamic> user, String permission) {
  final permissions = user['effective_permissions'];
  return permissions is List &&
      permissions.every((item) => item is String) &&
      permissions.contains(permission);
}

String? _roleValue(Object? role) {
  if (role is AppRole) {
    return switch (role) {
      AppRole.superAdmin => 'super_admin',
      AppRole.pengurus => 'pengurus',
      AppRole.bendahara => 'bendahara',
      AppRole.warga => 'warga',
      AppRole.perangkat => 'perangkat',
    };
  }
  return role is String ? role : null;
}
