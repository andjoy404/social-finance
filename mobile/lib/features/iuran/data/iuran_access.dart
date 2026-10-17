import 'package:social_finance/core/models/role.dart';

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

bool canAccessIuranPath(String path, Map<String, dynamic>? user) {
  if (!path.startsWith('/home/iuran/')) return true;
  final isManagePath =
      path == '/home/iuran/kelola' || path == '/home/iuran/form';
  return isManagePath ? canManageIuran(user) : canReadIuran(user);
}

bool _roleAllowsRead(Map<String, dynamic> user) {
  if (user['system_role'] == 'super_admin') return true;
  return switch (_roleValue(user['role'])) {
    'warga' || 'bendahara' || 'pengurus' => true,
    _ => false,
  };
}

bool _roleAllowsManagement(Map<String, dynamic> user) {
  if (user['system_role'] == 'super_admin') return true;
  return switch (_roleValue(user['role'])) {
    'bendahara' || 'pengurus' => true,
    _ => false,
  };
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
