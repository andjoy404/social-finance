import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/presentation/screens/special_resident_create_screen.dart';
import 'package:social_finance/features/warga/presentation/screens/warga_screen.dart';

void main() {
  group('Special Resident Positions (Keamanan, Operasional, Kebersihan)', () {
    test('kPetugasKhususJabatan contains keamanan, operasional, kebersihan and excludes kebersihan_pembangunan', () {
      expect(kPetugasKhususJabatan, equals({'keamanan', 'operasional', 'kebersihan'}));
      expect(kPetugasKhususJabatan.contains('kebersihan_pembangunan'), isFalse);
    });

    test('MappedResident.displayJabatan and jabatanColor normalize jabatan badges correctly', () {
      const ketua = MappedResident(id: 'k', name: 'Ketua', jabatan: 'ketua');
      const wakil = MappedResident(id: 'w', name: 'Wakil', jabatan: 'wakil_ketua');
      const sekretaris = MappedResident(id: 's', name: 'Sekretaris', jabatan: 'sekretaris');
      const bendahara = MappedResident(id: 'b', name: 'Bendahara', jabatan: 'bendahara');
      const keamanan = MappedResident(
        id: '1',
        name: 'Petugas Keamanan',
        jabatan: 'keamanan',
      );
      const operasional = MappedResident(
        id: '2',
        name: 'Petugas Operasional',
        jabatan: 'operasional',
      );
      const kebersihan = MappedResident(
        id: '3',
        name: 'Petugas Kebersihan',
        jabatan: 'kebersihan',
      );
      const sosial = MappedResident(id: '4', name: 'Seksi Sosial', jabatan: 'sosial');

      // Ketua RT retains 'Ketua RT'; other RT positions have 'RT' stripped
      expect(ketua.displayJabatan, equals('Ketua RT'));
      expect(wakil.displayJabatan, equals('Wakil Ketua'));
      expect(sekretaris.displayJabatan, equals('Sekretaris'));
      expect(bendahara.displayJabatan, equals('Bendahara'));
      expect(keamanan.displayJabatan, equals('Keamanan'));
      expect(operasional.displayJabatan, equals('Operasional'));
      expect(kebersihan.displayJabatan, equals('Kebersihan'));
      expect(sosial.displayJabatan, equals('Seksi Sosial'));

      expect(keamanan.jabatanColor, equals(AppColors.danger));
      expect(operasional.jabatanColor, equals(AppColors.info));
      expect(kebersihan.jabatanColor, equals(AppColors.success));
    });

    testWidgets('SpecialResidentCreateScreen dropdown displays Keamanan, Operasional, and Kebersihan', (tester) async {
      await tester.pumpWidget(
        const ProviderScope(
          child: MaterialApp(
            home: SpecialResidentCreateScreen(isSuperAdmin: false),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Default selected item is Keamanan
      expect(find.text('Keamanan'), findsOneWidget);

      // Open Jenis Petugas dropdown
      await tester.tap(find.byType(DropdownButtonFormField<String>).first);
      await tester.pumpAndSettle();

      expect(find.text('Keamanan'), findsWidgets);
      expect(find.text('Operasional'), findsWidgets);
      expect(find.text('Kebersihan'), findsWidgets);
      expect(find.text('kebersihan_pembangunan'), findsNothing);

      // Select Kebersihan
      await tester.tap(find.text('Kebersihan').last);
      await tester.pumpAndSettle();

      expect(find.text('Kebersihan'), findsOneWidget);
    });
  });
}
