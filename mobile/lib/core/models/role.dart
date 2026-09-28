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
