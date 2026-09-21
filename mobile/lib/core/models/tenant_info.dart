class TenantInfo {
  final String id;
  final String name;
  final String rt;
  final String rw;

  const TenantInfo({
    required this.id,
    required this.name,
    required this.rt,
    required this.rw,
  });

  String get displayText => '$name (RT $rt / RW $rw)';
}
