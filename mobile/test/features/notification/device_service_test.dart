import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_service.dart';

class MockAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options) handler;

  MockAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

ResponseBody _jsonResponse(dynamic data, int statusCode) {
  return ResponseBody.fromString(
    jsonEncode(data),
    statusCode,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}

void main() {
  group('DeviceService', () {
    test('registerDevice sends correct payload and endpoint', () async {
      RequestOptions? recordedOptions;
      final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8080'));
      dio.httpClientAdapter = MockAdapter((options) async {
        recordedOptions = options;
        return _jsonResponse({}, 204);
      });

      final client = ApiClient(customDio: dio);
      client.setToken('test-jwt-token');
      final deviceService = DeviceService(client);

      await deviceService.registerDevice(
        fcmToken: 'fcm-token-12345',
        platform: 'android',
        deviceId: 'device-id-abc',
      );

      expect(recordedOptions, isNotNull);
      expect(recordedOptions!.path, '/api/v1/notifications/devices');
      expect(recordedOptions!.method, 'POST');
      expect(recordedOptions!.headers['Authorization'], 'Bearer test-jwt-token');
      final data = recordedOptions!.data as Map<String, dynamic>;
      expect(data['fcm_token'], 'fcm-token-12345');
      expect(data['platform'], 'android');
      expect(data['device_id'], 'device-id-abc');
    });

    test('unregisterDevice sends correct DELETE payload', () async {
      RequestOptions? recordedOptions;
      final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8080'));
      dio.httpClientAdapter = MockAdapter((options) async {
        recordedOptions = options;
        return _jsonResponse({}, 204);
      });

      final client = ApiClient(customDio: dio);
      client.setToken('test-jwt-token');
      final deviceService = DeviceService(client);

      await deviceService.unregisterDevice('fcm-token-12345');

      expect(recordedOptions, isNotNull);
      expect(recordedOptions!.path, '/api/v1/notifications/devices');
      expect(recordedOptions!.method, 'DELETE');
      final data = recordedOptions!.data as Map<String, dynamic>;
      expect(data['fcm_token'], 'fcm-token-12345');
    });

    test('triggerWasteTruckArrival sends correct POST request', () async {
      RequestOptions? recordedOptions;
      final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8080'));
      dio.httpClientAdapter = MockAdapter((options) async {
        recordedOptions = options;
        return _jsonResponse({
          'status': 'accepted',
          'message': 'Waste truck arrival notification dispatched',
        }, 202);
      });

      final client = ApiClient(customDio: dio);
      client.setToken('test-jwt-token');
      final deviceService = DeviceService(client);

      final res = await deviceService.triggerWasteTruckArrival(
        message: 'Truk sampah sudah di depan gang',
      );

      expect(recordedOptions, isNotNull);
      expect(recordedOptions!.path, '/api/v1/notifications/waste-truck');
      expect(recordedOptions!.method, 'POST');
      final data = recordedOptions!.data as Map<String, dynamic>;
      expect(data['message'], 'Truk sampah sudah di depan gang');
      expect(res['status'], 'accepted');
    });
  });
}

