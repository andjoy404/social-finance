import 'package:dio/dio.dart';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/iuran/data/iuran_api_client.dart';
import 'package:social_finance/features/iuran/data/iuran_api_models.dart';

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
