/// Model for an RW (Rukun Warga) entity.
class RwModel {
  final String id;
  final int rwNumber;
  final String name;
  final String? headName;
  final String? address;
  final bool isActive;

  const RwModel({
    required this.id,
    required this.rwNumber,
    required this.name,
    this.headName,
    this.address,
    this.isActive = true,
  });

  factory RwModel.fromJson(Map<String, dynamic> json) {
    return RwModel(
      id: json['id'] as String? ?? '',
      rwNumber: (json['rw_number'] as num?)?.toInt() ?? 0,
      name: json['name'] as String? ?? '',
      headName: json['head_name'] as String?,
      address: json['address'] as String?,
      isActive: json['is_active'] as bool? ?? true,
    );
  }

  String get displayName {
    if (name.trim().isNotEmpty) return name;
    return 'RW ${rwNumber.toString().padLeft(3, '0')}';
  }
}
