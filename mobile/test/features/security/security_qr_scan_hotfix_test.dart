import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/features/security/data/security_api_client.dart';

class _MockSecurityAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options) handler;
  RequestOptions? lastRequest;

  _MockSecurityAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    lastRequest = options;
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  group('SecurityApiClient POS QR Scan & Error Mapping', () {
    test('scanQrAttendance sends qr_token, action, Idempotency-Key, and Authorization Bearer', () async {
      final adapter = _MockSecurityAdapter((options) async {
        return ResponseBody.fromString(
          '''
          {
            "action": "check_in",
            "message": "Check-in berhasil",
            "attendance": {
              "id": "att-1",
              "rt_id": "rt-002",
              "schedule_id": "sched-1",
              "user_id": "usr-sec-1",
              "shift_date": "2026-11-23",
              "status": "hadir",
              "checked_in_at": "2026-11-23T06:05:00+07:00"
            }
          }
          ''',
          200,
          headers: {
            Headers.contentTypeHeader: ['application/json'],
          },
        );
      });
      final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8081'))
        ..httpClientAdapter = adapter;
      final apiClient = ApiClient(customDio: dio, baseUrl: 'http://localhost:8081')
        ..setToken('officer-jwt');
      final securityClient = SecurityApiClient(apiClient);

      final result = await securityClient.scanQrAttendance(
        qrToken: '  SFQR:valid-token-123  ',
        action: 'auto',
        idempotencyKey: 'idem-key-001',
      );

      expect(result.action, equals('check_in'));
      expect(adapter.lastRequest?.path, endsWith('/api/v1/security/attendance/scan'));
      expect(adapter.lastRequest?.headers['Authorization'], equals('Bearer officer-jwt'));
      expect(adapter.lastRequest?.headers['Idempotency-Key'], equals('idem-key-001'));
      expect(adapter.lastRequest?.data['qr_token'], equals('SFQR:valid-token-123'));
      expect(adapter.lastRequest?.data['action'], equals('auto'));
    });

    final errorCases = <String, Map<String, dynamic>>{
      'qr_expired': {'status': 400, 'expectContains': 'kedaluwarsa'},
      'qr_already_used': {'status': 409, 'expectContains': 'sudah digunakan'},
      'invalid_qr': {'status': 400, 'expectContains': 'tidak valid'},
      'device_revoked': {'status': 403, 'expectContains': 'dicabut'},
      'cross_rt_denied': {'status': 403, 'expectContains': 'RT lain'},
      'active_open_session_exists': {'status': 409, 'expectContains': 'belum check-out'},
      'internal_error': {'status': 500, 'expectContains': 'kendala saat memproses'},
    };

    for (final entry in errorCases.entries) {
      test('maps backend error code ${entry.key} to clear Indonesian message (never raw Internal security module error)', () async {
        final adapter = _MockSecurityAdapter((options) async {
          return ResponseBody.fromString(
            '{"error":{"code":"${entry.key}","message":"Internal security module error"}}',
            entry.value['status'] as int,
            headers: {
              Headers.contentTypeHeader: ['application/json'],
            },
          );
        });
        final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8081'))
          ..httpClientAdapter = adapter;
        final apiClient = ApiClient(customDio: dio, baseUrl: 'http://localhost:8081')
          ..setToken('officer-jwt');
        final securityClient = SecurityApiClient(apiClient);

        try {
          await securityClient.scanQrAttendance(qrToken: 'SFQR:test');
          fail('Expected SecurityApiException');
        } on SecurityApiException catch (e) {
          expect(e.code, equals(entry.key));
          expect(e.message.toLowerCase(), isNot(contains('internal security module error')));
          expect(
            e.message.toLowerCase(),
            contains((entry.value['expectContains'] as String).toLowerCase()),
          );
        }
      });
    }
  });
}
