import 'package:dio/dio.dart';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';
import 'package:social_finance/features/iuran/data/iuran_models.dart';
import 'package:social_finance/features/iuran/data/iuran_repository.dart';

class _MockAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions) handler;

  _MockAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) => handler(options);

  @override
  void close({bool force = false}) {}
}

ResponseBody _jsonResponse(Object body, int status) => ResponseBody.fromString(
  jsonEncode(body),
  status,
  headers: {
    Headers.contentTypeHeader: [Headers.jsonContentType],
  },
);

void main() {
  final client = IuranApiClient(dio: Dio());

  Map<String, dynamic> bill(String id) => {
    'id': id,
    'rt_id': 'rt-1',
    'household_occupancy_id': 'occ-$id',
    'due_id': 'due-1',
    'amount': '50000.00',
    'period': '2026-10',
    'due_date': '2026-10-25',
    'status': 'unpaid',
  };

  test('parses generated_count and bills from backend response', () {
    final result = client.parseGenerateBillsResponse({
      'generated_count': 1,
      'bills': [bill('bill-1')],
    });

    expect(result.generatedCount, 1);
    expect(result.bills.single.id, 'bill-1');
  });

  test('accepts a valid empty bills response', () {
    final result = client.parseGenerateBillsResponse({
      'generated_count': 0,
      'bills': <dynamic>[],
    });

    expect(result.generatedCount, 0);
    expect(result.bills, isEmpty);
  });

  test('rejects an unknown response shape', () {
    expect(
      () => client.parseGenerateBillsResponse({'data': []}),
      throwsFormatException,
    );
  });

  test('rejects generated_count that disagrees with bills', () {
    expect(
      () => client.parseGenerateBillsResponse({
        'generated_count': 2,
        'bills': [bill('bill-1')],
      }),
      throwsFormatException,
    );
  });

  test('rejects malformed bill items', () {
    expect(
      () => client.parseGenerateBillsResponse({
        'generated_count': 1,
        'bills': ['not-a-bill'],
      }),
      throwsFormatException,
    );
  });

  test('reports an unexpected dues response instead of hiding dues', () async {
    final api = IuranApiClient(dio: Dio());
    api.dio.httpClientAdapter = _MockAdapter(
      (_) async => _jsonResponse({'data': []}, 200),
    );

    await expectLater(api.listDues(), throwsFormatException);
    api.dio.close();
  });

  test(
    'scopes bill requests to the authenticated user when requested',
    () async {
      final api = IuranApiClient(dio: Dio());
      final requests = <RequestOptions>[];
      api.dio.httpClientAdapter = _MockAdapter((options) async {
        requests.add(options);
        return _jsonResponse({
          'data': [],
          'pagination': {
            'page': 2,
            'page_size': 100,
            'total': 0,
            'total_pages': 2,
          },
        }, 200);
      });

      await api.listBills(myBills: true, page: 2, pageSize: 100);
      await api.listBills(page: 1);

      expect(requests[0].queryParameters['my_bills'], 'true');
      expect(requests[0].queryParameters['page'], '2');
      expect(requests[1].queryParameters.containsKey('my_bills'), isFalse);
      api.dio.close();
    },
  );

  test('keeps every Tagihan pagination request scoped', () async {
    final api = IuranApiClient(dio: Dio());
    final requests = <RequestOptions>[];
    api.dio.httpClientAdapter = _MockAdapter((options) async {
      requests.add(options);
      return _jsonResponse({
        'data': [],
        'pagination': {
          'page': int.parse(options.queryParameters['page'] as String),
          'page_size': 100,
          'total': 0,
          'total_pages': 2,
        },
      }, 200);
    });
    final repository = IuranRepository(apiClient: api);

    await repository.fetchAllBills(myBills: true);

    expect(requests.map((request) => request.queryParameters['page']), [
      '1',
      '2',
    ]);
    expect(
      requests.every(
        (request) => request.queryParameters['my_bills'] == 'true',
      ),
      isTrue,
    );
    api.dio.close();
  });

  test(
    'maps scoped regular and voluntary bills once with API values intact',
    () async {
      final api = IuranApiClient(dio: Dio());
      final billRequests = <RequestOptions>[];
      api.dio.httpClientAdapter = _MockAdapter((options) async {
        if (options.path.endsWith('/bills')) {
          billRequests.add(options);
          return _jsonResponse({
            'data': [
              {
                'id': 'regular-1',
                'rt_id': 'rt-1',
                'household_occupancy_id': 'occ-1',
                'due_id': 'due-regular',
                'due_name': 'Iuran Bulanan',
                'head_name': 'Kepala Keluarga',
                'amount': '50000.00',
                'period': '2026-10',
                'due_date': '2026-10-25',
                'status': 'partial',
                'is_voluntary': false,
              },
              {
                'id': 'voluntary-1',
                'rt_id': 'rt-1',
                'household_occupancy_id': 'occ-1',
                'due_id': 'due-voluntary',
                'due_name': 'Sumbangan Sukarela',
                'head_name': 'Kepala Keluarga',
                'amount': '0.00',
                'period': '2026-10',
                'due_date': '2026-10-25',
                'status': 'unpaid',
                'is_voluntary': true,
              },
              {
                'id': 'zero-mandatory-1',
                'rt_id': 'rt-1',
                'household_occupancy_id': 'occ-1',
                'due_id': 'due-zero-mandatory',
                'due_name': 'Tagihan Nominal Nol',
                'head_name': 'Kepala Keluarga',
                'amount': '0.00',
                'period': '2026-10',
                'due_date': '2026-10-25',
                'status': 'unpaid',
                'is_voluntary': false,
              },
            ],
            'pagination': {
              'page': 1,
              'page_size': 100,
              'total': 3,
              'total_pages': 1,
            },
          }, 200);
        }

        final billId = options.queryParameters['bill_id'];
        return _jsonResponse({
          'data': billId == 'regular-1'
              ? [
                  {
                    'id': 'payment-1',
                    'rt_id': 'rt-1',
                    'bill_id': billId,
                    'amount': '10000.00',
                    'method': 'TRANSFER',
                    'origin': 'STAFF_RECORDED',
                    'status': 'APPROVED',
                  },
                ]
              : <Map<String, Object?>>[],
          'pagination': {
            'page': 1,
            'page_size': 20,
            'total': billId == 'regular-1' ? 1 : 0,
            'total_pages': billId == 'regular-1' ? 1 : 0,
          },
        }, 200);
      });
      final repository = IuranRepository(apiClient: api);

      final bills = await repository.fetchAllBills(myBills: true);

      expect(billRequests, hasLength(1));
      expect(billRequests.single.queryParameters['my_bills'], 'true');
      expect(bills, hasLength(3));
      expect(bills.map((bill) => bill.id).toSet(), {
        'regular-1',
        'voluntary-1',
        'zero-mandatory-1',
      });
      final regular = bills.singleWhere((bill) => bill.id == 'regular-1');
      expect(regular.iuranType, 'Iuran Bulanan');
      expect(regular.nominal, 50000);
      expect(regular.periode, 'Oktober 2026');
      expect(regular.paidAmount, 10000);
      expect(regular.status, IuranStatus.sebagian);
      expect(regular.isVoluntary, isFalse);

      final voluntary = bills.singleWhere((bill) => bill.id == 'voluntary-1');
      expect(voluntary.iuranType, 'Sumbangan Sukarela');
      expect(voluntary.nominal, 0);
      expect(voluntary.periode, 'Oktober 2026');
      expect(voluntary.status, IuranStatus.sukarela);
      expect(voluntary.isVoluntary, isTrue);

      final zeroMandatory = bills.singleWhere(
        (bill) => bill.id == 'zero-mandatory-1',
      );
      expect(zeroMandatory.nominal, 0);
      expect(zeroMandatory.status, IuranStatus.belumBayar);
      expect(zeroMandatory.isVoluntary, isFalse);

      api.dio.close();
    },
  );

  test('creates a donation with due_id and parses nullable bill_id', () async {
    final api = IuranApiClient(dio: Dio());
    RequestOptions? request;
    api.dio.httpClientAdapter = _MockAdapter((options) async {
      request = options;
      return _jsonResponse({
        'id': 'payment-1',
        'rt_id': 'rt-1',
        'bill_id': null,
        'due_id': 'due-voluntary',
        'amount': '5000.00',
        'method': 'TRANSFER',
        'origin': 'SELF_SUBMITTED',
        'status': 'PENDING',
      }, 201);
    });

    final payment = await api.createDonation(
      dueId: 'due-voluntary',
      amount: '5000.00',
      method: PaymentMethod.transfer,
      notes: 'Sumbangan',
      idempotencyKey: 'retry-key',
    );

    expect(request!.data, {
      'due_id': 'due-voluntary',
      'amount': '5000.00',
      'method': 'TRANSFER',
      'notes': 'Sumbangan',
    });
    expect(request!.headers['Idempotency-Key'], 'retry-key');
    expect(payment.billId, isNull);
    expect(payment.dueId, 'due-voluntary');
    expect(payment.status, PaymentStatus.pending);
    api.dio.close();
  });

  test('propagates backend validation errors for donation requests', () async {
    final api = IuranApiClient(dio: Dio());
    api.dio.httpClientAdapter = _MockAdapter(
      (_) async => _jsonResponse({
        'error': {
          'code': 'validation_error',
          'message': 'Nominal harus positif',
        },
      }, 400),
    );

    await expectLater(
      api.createDonation(
        dueId: 'due-voluntary',
        amount: '0.00',
        method: PaymentMethod.transfer,
      ),
      throwsA(isA<DioException>()),
    );
    api.dio.close();
  });

  test(
    'rejects a create request with no target or two targets locally',
    () async {
      await expectLater(
        client.createPayment(
          billId: '',
          amount: '1000.00',
          method: PaymentMethod.cash,
        ),
        throwsArgumentError,
      );
      await expectLater(
        client.createDonation(
          dueId: '',
          amount: '1000.00',
          method: PaymentMethod.cash,
        ),
        throwsArgumentError,
      );
    },
  );
}
