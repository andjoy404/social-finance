enum AppRole { superAdmin, pengurus, bendahara, warga }

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
  }
}

// NOTE: Role visibility on the client is a UX-only concern.
// All authorization decisions must be enforced server-side.
// Do NOT rely on client-side role checks for security.
