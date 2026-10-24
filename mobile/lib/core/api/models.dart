class AuthUser {
  final String id;
  final String email;
  final String? phone;
  final String name;
  final String role;
  final String? rtId;
  final String? rtCode;
  final String? rtName;
  final String? rtAddress;
  final String? rwId;
  final String? rwCode;
  final String? rwName;
  final String? systemRole;
  final String? jabatan;
  final String? houseNumber;
  final List<String> effectivePermissions;
  final bool effectivePermissionsAvailable;

  const AuthUser({
    required this.id,
    required this.email,
    this.phone,
    required this.name,
    required this.role,
    this.rtId,
    this.rtCode,
    this.rtName,
    this.rtAddress,
    this.rwId,
    this.rwCode,
    this.rwName,
    this.systemRole,
    this.jabatan,
    this.houseNumber,
    this.effectivePermissions = const [],
    this.effectivePermissionsAvailable = false,
  });

  factory AuthUser.fromJson(Map<String, dynamic> json) {
    final rawRt = json['rt'];
    final rtName = json['rt_name'] as String?;
    String? rtCode =
        (json['rt_rt'] ?? (rawRt is Map ? rawRt['rt'] : null)) as String?;
    if ((rtCode == null || rtCode.trim().isEmpty) &&
        rtName != null &&
        rtName.trim().isNotEmpty) {
      final digits = RegExp(r'\d+').firstMatch(rtName)?.group(0);
      if (digits != null) {
        rtCode = digits.padLeft(3, '0');
      }
    }
    final rtId =
        json['rt_id'] as String? ??
        (rawRt is Map ? rawRt['id'] as String? : null);
    final rtAddress = json['rt_address'] as String?;
    final rwId = json['rw_id'] as String?;
    final rwName = json['rw_name'] as String?;
    final rawRtRw = json['rt_rw'] ?? (rawRt is Map ? rawRt['rw'] : null);
    String? rwCode;
    if (rawRtRw is num) {
      rwCode = rawRtRw.toInt().toString().padLeft(3, '0');
    } else if (rawRtRw is String && rawRtRw.trim().isNotEmpty) {
      final parsed = int.tryParse(rawRtRw.trim());
      rwCode = parsed != null
          ? parsed.toString().padLeft(3, '0')
          : rawRtRw.trim();
    } else if (rwName != null && rwName.trim().isNotEmpty) {
      final digits = RegExp(r'\d+').firstMatch(rwName)?.group(0);
      if (digits != null) {
        rwCode = digits.padLeft(3, '0');
      }
    }
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
      phone: json['phone'] as String?,
      name: (json['name'] ?? json['full_name']) as String? ?? '',
      role: json['role'] as String? ?? '',
      rtId: rtId,
      rtCode: rtCode,
      rtName: rtName,
      rtAddress: rtAddress,
      rwId: rwId,
      rwCode: rwCode,
      rwName: rwName,
      systemRole: json['system_role'] as String?,
      jabatan: json['jabatan'] as String?,
      houseNumber: (json['house_number'] ?? json['houseNumber']) as String?,
      effectivePermissions: effectivePermissions,
      effectivePermissionsAvailable: effectivePermissionsAvailable,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'email': email,
    if (phone != null) 'phone': phone,
    'name': name,
    'role': role,
    if (rtId != null) 'rt_id': rtId,
    if (rtCode != null) 'rt_rt': rtCode,
    if (rtName != null) 'rt_name': rtName,
    if (rtAddress != null) 'rt_address': rtAddress,
    if (rwId != null) 'rw_id': rwId,
    if (rwCode != null) 'rt_rw': int.tryParse(rwCode!) ?? rwCode,
    if (rwName != null) 'rw_name': rwName,
    if (systemRole != null) 'system_role': systemRole,
    if (jabatan != null) 'jabatan': jabatan,
    if (houseNumber != null) 'house_number': houseNumber,
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
