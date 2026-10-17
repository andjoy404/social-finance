class AuthUser {
  final String id;
  final String email;
  final String name;
  final String role;
  final String? rtId;
  final String? rtCode;
  final String? systemRole;
  final String? jabatan;
  final List<String> effectivePermissions;
  final bool effectivePermissionsAvailable;

  const AuthUser({
    required this.id,
    required this.email,
    required this.name,
    required this.role,
    this.rtId,
    this.rtCode,
    this.systemRole,
    this.jabatan,
    this.effectivePermissions = const [],
    this.effectivePermissionsAvailable = false,
  });

  factory AuthUser.fromJson(Map<String, dynamic> json) {
    final rawRt = json['rt'];
    final rtCode =
        (json['rt_rt'] ?? (rawRt is Map ? rawRt['rt'] : null)) as String?;
    final rtId =
        json['rt_id'] as String? ??
        (rawRt is Map ? rawRt['id'] as String? : null);
    final rawPermissions = json['effective_permissions'];
    final effectivePermissionsAvailable =
        rawPermissions is List &&
        rawPermissions.every((permission) => permission is String);
    final effectivePermissions = effectivePermissionsAvailable
        ? List<String>.unmodifiable(rawPermissions.cast<String>())
        : const <String>[];
    return AuthUser(
      id: json['id'] as String? ?? '',
      email: json['email'] as String? ?? '',
      name: (json['name'] ?? json['full_name']) as String? ?? '',
      role: json['role'] as String? ?? '',
      rtId: rtId,
      rtCode: rtCode,
      systemRole: json['system_role'] as String?,
      jabatan: json['jabatan'] as String?,
      effectivePermissions: effectivePermissions,
      effectivePermissionsAvailable: effectivePermissionsAvailable,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'email': email,
    'name': name,
    'role': role,
    if (rtId != null) 'rt_id': rtId,
    if (rtCode != null) 'rt_rt': rtCode,
    if (systemRole != null) 'system_role': systemRole,
    if (jabatan != null) 'jabatan': jabatan,
    'effective_permissions': effectivePermissions,
  };
}

class LoginResponse {
  final String accessToken;
  final String refreshToken;
  final String tokenType;
  final int expiresIn;
  final AuthUser user;

  const LoginResponse({
    required this.accessToken,
    required this.refreshToken,
    required this.tokenType,
    required this.expiresIn,
    required this.user,
  });

  factory LoginResponse.fromJson(Map<String, dynamic> json) {
    final accessToken = json['access_token'] as String?;
    if (accessToken == null || accessToken.trim().isEmpty) {
      throw const FormatException(
        'Missing or empty access_token in login response',
      );
    }
    final userRaw = json['user'];
    if (userRaw == null || userRaw is! Map) {
      throw const FormatException(
        'Missing or invalid user object in login response',
      );
    }
    final userMap = userRaw is Map<String, dynamic>
        ? userRaw
        : userRaw.cast<String, dynamic>();

    return LoginResponse(
      accessToken: accessToken,
      refreshToken: json['refresh_token'] as String? ?? '',
      tokenType: json['token_type'] as String? ?? 'Bearer',
      expiresIn: (json['expires_in'] as num?)?.toInt() ?? 0,
      user: AuthUser.fromJson(userMap),
    );
  }

  Map<String, dynamic> toJson() => {
    'access_token': accessToken,
    'refresh_token': refreshToken,
    'token_type': tokenType,
    'expires_in': expiresIn,
    'user': user.toJson(),
  };
}
