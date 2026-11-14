enum AppRole { superAdmin, pengurus, bendahara, warga, perangkat }

String appRoleDisplayName(AppRole role) {
  switch (role) {
    case AppRole.superAdmin:
      return 'Super Admin';
    case AppRole.pengurus:
      return 'Pengurus';
    case AppRole.bendahara:
      return 'Bendahara';
    case AppRole.warga:
      return 'Warga';
    case AppRole.perangkat:
      return 'Perangkat';
  }
}

// NOTE: Role visibility on the client is a UX-only concern.
// All authorization decisions must be enforced server-side.
// Do NOT rely on client-side role checks for security.

/// Roles that may create or modify warga data.
const Set<AppRole> kWargaWriteRoles = {
  AppRole.superAdmin,
  AppRole.pengurus,
  AppRole.bendahara,
  AppRole.perangkat,
};

/// Displays a readable job position name from API format.
String appJabatanDisplayName(String jabatan) {
  final normalized = jabatan.trim().toLowerCase().replaceAll(RegExp(r'\s+'), '_');
  switch (normalized) {
    case 'ketua':
    case 'ketua_rt':
      return 'Ketua';
    case 'wakil_ketua':
    case 'wakil_ketua_rt':
      return 'Wakil Ketua';
    case 'sekretaris':
    case 'sekretaris_rt':
      return 'Sekretaris';
    case 'bendahara':
    case 'bendahara_rt':
      return 'Bendahara';
    case 'keamanan':
    case 'keamanan_rt':
    case 'seksi_keamanan':
    case 'seksi_keamanan_rt':
      return 'Petugas Keamanan';
    case 'operasional':
    case 'operasional_rt':
    case 'seksi_operasional':
    case 'seksi_operasional_rt':
      return 'Petugas Operasional';
    case 'kebersihan':
    case 'kebersihan_rt':
    case 'seksi_kebersihan':
    case 'seksi_kebersihan_rt':
      return 'Petugas Kebersihan';
    case 'sosial':
    case 'sosial_rt':
    case 'seksi_sosial':
    case 'seksi_sosial_rt':
      return 'Petugas Sosial';
    default:
      return jabatan.trim().isNotEmpty ? jabatan.trim() : 'Warga';
  }
}
