import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dio/dio.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/features/warga/data/api_warga_models.dart';
import 'package:social_finance/features/warga/data/warga_providers.dart';
import 'package:social_finance/features/warga/data/warga_repository.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/warga/presentation/screens/warga_screen.dart';

class _MockAuthRepo extends AuthRepository {
  _MockAuthRepo(Map<String, dynamic>? user)
      : super(
          authService: AuthService(ApiClient(baseUrl: 'http://test.local')),
          apiClient: ApiClient(baseUrl: 'http://test.local'),
        ) {
    state = AsyncData(user);
  }
}

Widget _createWargaTestApp({
  required List<MappedResident> residents,
  String initialQuery = '',
  Map<String, dynamic>? currentUser,
}) {
  return ProviderScope(
    overrides: [
      wargaListProvider.overrideWith(() => _TestNotifier(residents)),
      wargaSearchQueryProvider.overrideWith((_) => initialQuery),
      if (currentUser != null)
        authRepositoryProvider.overrideWith((ref) => _MockAuthRepo(currentUser)),
    ],
    child: const MaterialApp(home: WargaScreen()),
  );
}

class _TestNotifier extends WargaListNotifier {
  final List<MappedResident> residents;

  _TestNotifier(this.residents);

  @override
  Future<List<MappedResident>> build() async {
    state = AsyncData(residents);
    return residents;
  }
}

void main() {
  group('MappedResident Model', () {
    test('maskedNik masks middle digits correctly for standard 16-digit NIK', () {
      const resident = MappedResident(
        id: 'res-1',
        name: 'Test Resident',
        nik: '3201012304750001',
        houseNumber: 'A1',
      );

      expect(resident.maskedNik, equals('320101******0001'));
    });

    test('maskedNik handles short NIK safely', () {
      const resident = MappedResident(
        id: 'res-2',
        name: 'Test Short',
        nik: '12345',
        houseNumber: 'A2',
      );

      expect(resident.maskedNik, equals('***'));
    });

    test('maskedNik handles null NIK safely', () {
      const resident = MappedResident(
        id: 'res-3',
        name: 'Test Null NIK',
        nik: null,
        houseNumber: 'A3',
      );

      expect(resident.maskedNik, equals('***'));
    });

    test('formattedRtRw formats full RT, RW, and RT name correctly', () {
      const resident = MappedResident(
        id: 'res-4',
        name: 'Dedi Kurniawan',
        rtNumber: '03',
        rw: 16,
        rtName: 'Wisma Rukun Tunggal',
      );

      expect(
        resident.formattedRtRw,
        equals('RT 03 · RW 16 · Wisma Rukun Tunggal'),
      );
    });

    test('formattedRtRw handles partial and missing RT/RW fields', () {
      const residentPartial = MappedResident(
        id: 'res-5',
        name: 'Test Partial',
        rtNumber: '03',
        rw: 16,
      );
      expect(residentPartial.formattedRtRw, equals('RT 03 · RW 16'));

      const residentEmpty = MappedResident(
        id: 'res-6',
        name: 'Test Empty',
      );
      expect(residentEmpty.formattedRtRw, isNull);
    });
  });

  group('BackendResident Parsing', () {
    test('parses all fields correctly including RT/RW', () {
      final json = {
        'id': 'uuid-1',
        'rt_id': 'rt-uuid-1',
        'household_id': 'hh-uuid-1',
        'full_name': 'Budi Santoso',
        'phone': '+6281234567890',
        'nik': '3201012304750001',
        'email': 'budi@example.com',
        'is_active': true,
        'created_at': '2024-01-01T00:00:00Z',
        'updated_at': '2024-01-01T00:00:00Z',
        'rt_number': '03',
        'rw': 16,
        'rt_name': 'Wisma Rukun Tunggal',
      };

      final resident = BackendResident.fromJson(json);

      expect(resident.id, equals('uuid-1'));
      expect(resident.fullName, equals('Budi Santoso'));
      expect(resident.nik, equals('3201012304750001'));
      expect(resident.householdId, equals('hh-uuid-1'));
      expect(resident.rtNumber, equals('03'));
      expect(resident.rw, equals(16));
      expect(resident.rtName, equals('Wisma Rukun Tunggal'));
    });

    test('handles nullable fields', () {
      final json = {
        'id': 'uuid-2',
        'rt_id': 'rt-uuid-2',
        'full_name': 'Test User',
        'household_id': null,
        'phone': null,
        'nik': null,
        'email': null,
        'is_active': true,
      };

      final resident = BackendResident.fromJson(json);

      expect(resident.nik, isNull);
      expect(resident.phone, isNull);
      expect(resident.householdId, isNull);
    });
  });

  group('BackendHousehold Parsing', () {
    test('parses all fields correctly', () {
      final json = {
        'id': 'hh-uuid-1',
        'rt_id': 'rt-uuid-1',
        'house_number': 'Blok A1 No. 12',
        'head_name': 'Budi Santoso',
        'address': 'Jalan Raya No. 1',
        'occupancy_status': 'OWNER',
        'is_active': true,
      };

      final household = BackendHousehold.fromJson(json);

      expect(household.id, equals('hh-uuid-1'));
      expect(household.houseNumber, equals('Blok A1 No. 12'));
      expect(household.occupancyStatus, equals('OWNER'));
    });

    test('handles optional fields', () {
      final json = {
        'id': 'hh-uuid-2',
        'rt_id': 'rt-uuid-2',
        'head_name': 'Test Head',
        'house_number': null,
        'address': null,
        'occupancy_status': null,
        'is_active': true,
      };

      final household = BackendHousehold.fromJson(json);

      expect(household.houseNumber, isNull);
      expect(household.occupancyStatus, isNull);
    });
  });

  group('PaginatedResponse Parsing', () {
    test('parses residents response correctly', () {
      final json = {
        'data': [
          {
            'id': 'uuid-1',
            'rt_id': 'rt-uuid-1',
            'full_name': 'Budi',
            'nik': '3201012304750001',
            'is_active': true,
          },
        ],
        'pagination': {
          'page': 1,
          'page_size': 20,
          'total': 1,
          'total_pages': 1,
        },
      };

      final response = PaginatedResponse.fromJson(
        json,
        (dynamic e) => BackendResident.fromJson(e.cast<String, dynamic>()),
      );

      expect(response.data.length, equals(1));
      expect(response.data.first.fullName, equals('Budi'));
      expect(response.pagination.total, equals(1));
    });
  });

  group('Filtering Logic', () {
    bool matches(MappedResident r, String query) {
      return r.name.toLowerCase().contains(query) ||
          (r.nik?.toLowerCase().contains(query) ?? false) ||
          r.houseNumber.toLowerCase().contains(query);
    }

    test('filters by name', () {
      const residents = [
        MappedResident(id: 'r1', name: 'Bambang Sutrisno', houseNumber: 'A1'),
        MappedResident(id: 'r2', name: 'Siti Aminah', houseNumber: 'A2'),
      ];

      final filtered = residents.where((r) => matches(r, 'bambang')).toList();

      expect(filtered.length, equals(1));
      expect(filtered.first.name, equals('Bambang Sutrisno'));
    });

    test('filters by NIK substring', () {
      const residents = [
        MappedResident(id: 'r1', name: 'Bambang', nik: '3201012304750001', houseNumber: 'A1'),
        MappedResident(id: 'r2', name: 'Siti', nik: '3201015508780002', houseNumber: 'A2'),
      ];

      final filtered = residents.where((r) => matches(r, '0006')).toList();

      expect(filtered.length, equals(0));
    });

    test('filters by house number', () {
      const residents = [
        MappedResident(id: 'r1', name: 'Bambang', houseNumber: 'Blok A1 No. 12'),
        MappedResident(id: 'r3', name: 'Dimas', houseNumber: 'Blok B2 No. 05'),
      ];

      final filtered = residents.where((r) => matches(r, 'b2')).toList();

      expect(filtered.length, equals(1));
      expect(filtered.first.name, equals('Dimas'));
    });

    test('empty query returns all residents', () {
      const residents = [
        MappedResident(id: 'r1', name: 'Bambang', houseNumber: 'A1'),
        MappedResident(id: 'r2', name: 'Siti', houseNumber: 'A2'),
      ];

      final query = ''.trim().toLowerCase();
      final filtered = query.isEmpty ? residents : residents.where((r) => matches(r, query)).toList();

      expect(filtered.length, equals(2));
    });

    test('no match returns empty list', () {
      const residents = [
        MappedResident(id: 'r1', name: 'Test User', houseNumber: 'A1'),
      ];

      final filtered = residents.where((r) => matches(r, 'NonExistentPerson')).toList();

      expect(filtered.length, equals(0));
    });
  });

  group('Warga Screen Rendering', () {
    testWidgets(
      'Warga screen renders with title, description, and search field',
      (tester) async {
        await tester.pumpWidget(_createWargaTestApp(
          residents: [],
        ));
        await tester.pumpAndSettle();

        expect(find.text('WARGA'), findsWidgets);
        expect(
          find.textContaining('Lingkungan RT'),
          findsWidgets,
        );
        expect(find.byType(TextField), findsOneWidget);
      },
    );

    testWidgets('Warga screen displays residents with details', (
      tester,
    ) async {
      const residents = [
        MappedResident(
          id: 'r1',
          name: 'Bambang Sutrisno',
          nik: '3201012304750001',
          houseNumber: 'Blok A1 No. 12',
          phoneNumber: '081234567890',
          occupancyStatus: 'Pemilik',
          rtNumber: '03',
          rw: 16,
          rtName: 'Wisma Rukun Tunggal',
        ),
      ];
      await tester.pumpWidget(_createWargaTestApp(
        residents: residents,
      ));
      await tester.pumpAndSettle();

      expect(find.text('Bambang Sutrisno'), findsOneWidget);
      // RT and Alamat RT removed from card per Part 4
      expect(find.text('RT 03 · RW 16 · Wisma Rukun Tunggal'), findsNothing);
      expect(find.text('Blok A1 No. 12'), findsOneWidget);
      expect(find.textContaining('NIK'), findsNothing);
      expect(find.text('Pemilik'), findsOneWidget);
      expect(find.text('081234567890'), findsOneWidget);
    });

    testWidgets('Empty resident list displays empty state', (tester) async {
      await tester.pumpWidget(_createWargaTestApp(
        residents: <MappedResident>[],
      ));
      await tester.pumpAndSettle();

      expect(find.text('Belum Ada Warga'), findsOneWidget);
      expect(
        find.text('Data kependudukan warga RT belum tersedia.'),
        findsOneWidget,
      );
    });

    test('Loading state can be set directly', () {
      final container = ProviderContainer(
        overrides: [
          wargaListProvider.overrideWith(
            () => _TestNotifier([]),
          ),
        ],
      );

      container.read(wargaListProvider.notifier).state = const AsyncLoading();

      expect(container.read(wargaListProvider), isA<AsyncLoading>());

      container.dispose();
    });

    test('Error state can be set directly', () {
      final container = ProviderContainer(
        overrides: [
          wargaListProvider.overrideWith(
            () => _TestNotifier([]),
          ),
        ],
      );

      const errorMsg = 'Sesi Anda telah berakhir. Silakan login kembali.';
      container.read(wargaListProvider.notifier).state = AsyncError(
        ServerException(message: errorMsg),
        StackTrace.current,
      );

      final state = container.read(wargaListProvider);
      expect(state, isA<AsyncError>());
      final error = (state as AsyncError).error;
      expect(error, isA<ServerException>());
      expect((error as ServerException).message, contains('Sesi Anda telah berakhir'));

      container.dispose();
    });
  });

  group('Repository', () {
    test('mapDioError maps 401 correctly', () {
      final repo = WargaRepository(client: ApiClient(baseUrl: 'http://test.local'));
      final requestOptions = RequestOptions(path: '/api/v1/residents');
      final exception = DioException(
        requestOptions: requestOptions,
        type: DioExceptionType.badResponse,
        response: Response(requestOptions: requestOptions, statusCode: 401),
      );
      final err = repo.mapDioError(exception);
      expect(err.message, contains('Sesi Anda telah berakhir'));
    });

    test('mapDioError maps 500 correctly', () {
      final repo = WargaRepository(client: ApiClient(baseUrl: 'http://test.local'));
      final requestOptions = RequestOptions(path: '/api/v1/residents');
      final exception = DioException(
        requestOptions: requestOptions,
        type: DioExceptionType.badResponse,
        response: Response(requestOptions: requestOptions, statusCode: 500),
      );
      final err = repo.mapDioError(exception);
      expect(err.message, contains('kesalahan pada server'));
    });

    test('mapDioError maps 403 correctly', () {
      final repo = WargaRepository(client: ApiClient(baseUrl: 'http://test.local'));
      final requestOptions = RequestOptions(path: '/api/v1/residents');
      final exception = DioException(
        requestOptions: requestOptions,
        type: DioExceptionType.badResponse,
        response: Response(requestOptions: requestOptions, statusCode: 403),
      );
      final err = repo.mapDioError(exception);
      expect(err.message, contains('tidak memiliki akses'));
    });
  });

  group('WargaScreen write action authorization', () {
    const testResident = MappedResident(
      id: 'res-auth-1',
      name: 'Ahmad Dahlan',
      householdId: 'hh-1',
    );

    testWidgets('hides FAB and edit icon for read-only jabatan (keamanan)', (tester) async {
      await tester.pumpWidget(
        _createWargaTestApp(
          residents: [testResident],
          currentUser: {
            'id': 'user-keamanan',
            'email': 'keamanan@example.com',
            'role': AppRole.pengurus,
            'system_role': null,
            'jabatan': 'keamanan',
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Ahmad Dahlan'), findsOneWidget);
      expect(find.byIcon(Icons.person_add), findsNothing);
      expect(find.byIcon(Icons.edit_outlined), findsNothing);
    });

    testWidgets('shows FAB and edit icon for authorized jabatan (ketua)', (tester) async {
      await tester.pumpWidget(
        _createWargaTestApp(
          residents: [testResident],
          currentUser: {
            'id': 'user-ketua',
            'email': 'ketua@example.com',
            'role': AppRole.pengurus,
            'system_role': null,
            'jabatan': 'ketua',
          },
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Ahmad Dahlan'), findsOneWidget);
      expect(find.byIcon(Icons.person_add), findsOneWidget);
      expect(find.byIcon(Icons.edit_outlined), findsOneWidget);
    });

    testWidgets('FAB has single person_add icon without duplicate plus text, and resident card icons have distinct colors', (tester) async {
      const fullResident = MappedResident(
        id: 'res-full-1',
        name: 'Ahmad Dahlan',
        houseNumber: 'A-12',
        phoneNumber: '081234567890',
        email: 'ahmad@example.com',
        householdId: 'hh-1',
      );
      await tester.pumpWidget(
        _createWargaTestApp(
          residents: [fullResident],
          currentUser: {
            'id': 'user-ketua',
            'email': 'ketua@example.com',
            'role': AppRole.warga,
            'system_role': null,
            'jabatan': 'ketua',
          },
        ),
      );
      await tester.pumpAndSettle();

      // FAB has person_add icon and NO extra '+' text widget
      expect(find.byIcon(Icons.person_add), findsOneWidget);
      expect(find.text('+'), findsNothing);

      // Info icons are present with distinct colors
      final homeFinder = find.byIcon(Icons.home_outlined);
      final phoneFinder = find.byIcon(Icons.phone_outlined);
      final emailFinder = find.byIcon(Icons.email_outlined);

      expect(homeFinder, findsOneWidget);
      expect(phoneFinder, findsOneWidget);
      expect(emailFinder, findsOneWidget);

      final homeIcon = tester.widget<Icon>(homeFinder);
      final phoneIcon = tester.widget<Icon>(phoneFinder);
      final emailIcon = tester.widget<Icon>(emailFinder);

      expect(homeIcon.color, isNotNull);
      expect(phoneIcon.color, isNotNull);
      expect(emailIcon.color, isNotNull);
      expect(homeIcon.color, isNot(equals(phoneIcon.color)));
      expect(phoneIcon.color, isNot(equals(emailIcon.color)));
    });
  });
}
