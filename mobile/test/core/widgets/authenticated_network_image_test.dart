import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/config.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/widgets/authenticated_network_image.dart';

// Minimal valid 1x1 transparent PNG
final Uint8List _validPngBytes = base64Decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
);

class _RecordingAdapter implements HttpClientAdapter {
  final Future<ResponseBody> Function(RequestOptions options, int callIndex)
      handler;
  final List<RequestOptions> requests = [];

  _RecordingAdapter(this.handler);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    requests.add(options);
    return handler(options, requests.length);
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  group('ApiConfig.resolveImageUrl double /api/v1 protection', () {
    test('does not duplicate /api/v1 when baseUrl already ends with /api/v1', () {
      final resolved = ApiConfig.resolveImageUrl(
        '/api/v1/notifications/images/notifications/waste-truck/rt-1/img.jpg',
        fallbackBaseUrl: 'http://localhost:8081/api/v1',
      );
      expect(
        resolved,
        equals(
          'http://localhost:8081/api/v1/notifications/images/notifications/waste-truck/rt-1/img.jpg',
        ),
      );
      expect(resolved, isNot(contains('/api/v1/api/v1')));
    });

    test('collapses accidental double /api/v1/api/v1 in absolute URL', () {
      final resolved = ApiConfig.resolveImageUrl(
        'http://localhost:8081/api/v1/api/v1/notifications/images/notifications/rw-manual/rw-1/banner.png',
      );
      expect(
        resolved,
        equals(
          'http://localhost:8081/api/v1/notifications/images/notifications/rw-manual/rw-1/banner.png',
        ),
      );
    });
  });

  group('AuthenticatedNetworkImage', () {
    testWidgets(
      'fetches RT and RW notification images via authenticated Dio with Bearer & X-RT-ID headers (never in URL)',
      (tester) async {
        final adapter = _RecordingAdapter((options, _) async {
          return ResponseBody.fromBytes(
            _validPngBytes,
            200,
            headers: {
              Headers.contentTypeHeader: ['image/png'],
            },
          );
        });
        final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8081'))
          ..httpClientAdapter = adapter;
        final apiClient = ApiClient(customDio: dio, baseUrl: 'http://localhost:8081')
          ..setToken('jwt-rt-token-123');

        const rtImageUrl =
            '/api/v1/notifications/images/notifications/waste-truck/rt-002/truck.png';

        await tester.pumpWidget(
          ProviderScope(
            overrides: [apiClientProvider.overrideWithValue(apiClient)],
            child: const MaterialApp(
              home: Scaffold(
                body: AuthenticatedNetworkImage(
                  imageUrl: rtImageUrl,
                  rtId: 'rt-002',
                  errorWidget: Text('Gambar lampiran tidak dapat dimuat.'),
                ),
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();

        expect(find.byType(Image), findsOneWidget);
        expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsNothing);
        expect(adapter.requests.length, equals(1));
        final req = adapter.requests.first;
        expect(req.uri.toString(), equals('http://localhost:8081$rtImageUrl'));
        expect(req.uri.toString(), isNot(contains('jwt-rt-token-123')));
        expect(req.headers['Authorization'], equals('Bearer jwt-rt-token-123'));
        expect(req.headers['X-RT-ID'], equals('rt-002'));
      },
    );

    testWidgets(
      'recovers from expired token (401) via ApiClient refresh-token retry and renders image',
      (tester) async {
        final adapter = _RecordingAdapter((options, callIndex) async {
          if (callIndex == 1) {
            return ResponseBody.fromString(
              '{"error":{"code":"unauthorized","message":"token expired"}}',
              401,
              headers: {
                Headers.contentTypeHeader: ['application/json'],
              },
            );
          }
          return ResponseBody.fromBytes(
            _validPngBytes,
            200,
            headers: {
              Headers.contentTypeHeader: ['image/png'],
            },
          );
        });
        final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8081'))
          ..httpClientAdapter = adapter;
        final apiClient = ApiClient(
          customDio: dio,
          baseUrl: 'http://localhost:8081',
          onRefreshToken: (_) async => LoginResponse.fromJson(const {
            'access_token': 'fresh-jwt-token-456',
            'refresh_token': 'fresh-refresh-token',
            'token_type': 'Bearer',
            'expires_in': 900,
            'user': {
              'id': 'u-1',
              'full_name': 'Warga RT 002',
              'email': 'warga@test.local',
              'role': 'warga',
              'rt_id': 'rt-002',
            },
          }),
        )
          ..setToken('expired-jwt-token')
          ..setRefreshToken('valid-refresh-token');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [apiClientProvider.overrideWithValue(apiClient)],
            child: const MaterialApp(
              home: Scaffold(
                body: AuthenticatedNetworkImage(
                  imageUrl:
                      '/api/v1/notifications/images/notifications/manual/rt-002/info.png',
                  rtId: 'rt-002',
                  errorWidget: Text('Gambar lampiran tidak dapat dimuat.'),
                ),
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();

        expect(adapter.requests.length, equals(2));
        expect(
          adapter.requests[1].headers['Authorization'],
          equals('Bearer fresh-jwt-token-456'),
        );
        expect(find.byType(Image), findsOneWidget);
        expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsNothing);
      },
    );

    testWidgets(
      'renders fallback errorWidget only on genuine 403 forbidden failure',
      (tester) async {
        final adapter = _RecordingAdapter((options, _) async {
          return ResponseBody.fromString(
            '{"error":{"code":"forbidden","message":"cross-RT denied"}}',
            403,
            headers: {
              Headers.contentTypeHeader: ['application/json'],
            },
          );
        });
        final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8081'))
          ..httpClientAdapter = adapter;
        final apiClient = ApiClient(customDio: dio, baseUrl: 'http://localhost:8081')
          ..setToken('valid-jwt');

        await tester.pumpWidget(
          ProviderScope(
            overrides: [apiClientProvider.overrideWithValue(apiClient)],
            child: const MaterialApp(
              home: Scaffold(
                body: AuthenticatedNetworkImage(
                  imageUrl:
                      '/api/v1/notifications/images/notifications/waste-truck/other-rt/secret.png',
                  errorWidget: Text('Gambar lampiran tidak dapat dimuat.'),
                ),
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();

        expect(find.text('Gambar lampiran tidak dapat dimuat.'), findsOneWidget);
        expect(find.byType(Image), findsNothing);
      },
    );
  });
}
