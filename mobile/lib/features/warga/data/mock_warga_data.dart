/// Resident item model for Warga presentation and mock data.
class ResidentItem {
  final String id;
  final String name;
  final String nik;
  final String houseNumber;
  final bool isHeadOfHousehold;
  final String relationship;
  final String? phoneNumber;
  final String? occupancyStatus;
  final bool isActive;

  const ResidentItem({
    required this.id,
    required this.name,
    required this.nik,
    required this.houseNumber,
    required this.isHeadOfHousehold,
    required this.relationship,
    this.phoneNumber,
    this.occupancyStatus,
    this.isActive = true,
  });

  /// Returns masked NIK preserving first 6 and last 4 digits (e.g. 320101******0001).
  String get maskedNik {
    if (nik.length < 10) return '***';
    return '${nik.substring(0, 6)}******${nik.substring(nik.length - 4)}';
  }
}

/// Mock data store for Warga feature. Used only for tests.
class WargaMockData {
  static const List<ResidentItem> residents = [
    ResidentItem(
      id: 'res-001',
      name: 'Bambang Sutrisno',
      nik: '3201012304750001',
      houseNumber: 'Blok A1 No. 12',
      isHeadOfHousehold: true,
      relationship: 'Kepala Keluarga',
      phoneNumber: '081234567890',
      occupancyStatus: 'Pemilik',
    ),
    ResidentItem(
      id: 'res-002',
      name: 'Siti Aminah',
      nik: '3201015508780002',
      houseNumber: 'Blok A1 No. 12',
      isHeadOfHousehold: false,
      relationship: 'Istri',
      phoneNumber: '081234567891',
      occupancyStatus: 'Pemilik',
    ),
    ResidentItem(
      id: 'res-003',
      name: 'Dimas Pratama',
      nik: '3201011210020003',
      houseNumber: 'Blok A1 No. 12',
      isHeadOfHousehold: false,
      relationship: 'Anak',
      phoneNumber: '081234567892',
      occupancyStatus: 'Pemilik',
    ),
    ResidentItem(
      id: 'res-004',
      name: 'Heri Prastyo',
      nik: '3201021405800004',
      houseNumber: 'Blok B2 No. 05',
      isHeadOfHousehold: true,
      relationship: 'Kepala Keluarga',
      phoneNumber: '081398765432',
      occupancyStatus: 'Pemilik',
    ),
    ResidentItem(
      id: 'res-005',
      name: 'Ratna Dewi',
      nik: '3201026011830005',
      houseNumber: 'Blok B2 No. 05',
      isHeadOfHousehold: false,
      relationship: 'Istri',
      phoneNumber: '081398765433',
      occupancyStatus: 'Pemilik',
    ),
    ResidentItem(
      id: 'res-006',
      name: 'Ahmad Fauzi',
      nik: '3201031908900006',
      houseNumber: 'Blok C3 No. 08',
      isHeadOfHousehold: true,
      relationship: 'Kepala Keluarga',
      phoneNumber: '085712345678',
      occupancyStatus: 'Penyewa',
    ),
    ResidentItem(
      id: 'res-007',
      name: 'Sri Wahyuni',
      nik: '3201034502920007',
      houseNumber: 'Blok C3 No. 08',
      isHeadOfHousehold: false,
      relationship: 'Istri',
      phoneNumber: '085712345679',
      occupancyStatus: 'Penyewa',
    ),
  ];
}
