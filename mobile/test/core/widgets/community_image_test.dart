import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/auth_service.dart';
import 'package:social_finance/core/api/client.dart';
import 'package:social_finance/core/api/device_id_service.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/widgets/community_image.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

final tinyPng = Uint8List.fromList([
  0x89,
  0x50,
  0x4E,
  0x47,
  0x0D,
  0x0A,
  0x1A,
  0x0A,
  0x00,
  0x00,
  0x00,
  0x0D,
  0x49,
  0x48,
  0x44,
  0x52,
  0x00,
  0x00,
  0x00,
  0x01,
  0x00,
  0x00,
  0x00,
  0x01,
  0x08,
  0x06,
  0x00,
  0x00,
  0x00,
  0x1F,
  0x15,
  0xC4,
  0x89,
  0x00,
  0x00,
  0x00,
  0x0A,
  0x49,
  0x44,
  0x41,
  0x54,
  0x78,
  0x9C,
  0x63,
  0x00,
  0x01,
  0x00,
  0x00,
  0x05,
  0x00,
  0x01,
  0x0D,
  0x0A,
  0x2D,
  0xB4,
  0x00,
  0x00,
  0x00,
  0x00,
  0x49,
  0x45,
  0x4E,
  0x44,
  0xAE,
  0x42,
  0x60,
  0x82,
]);

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

ResponseBody _json(Object body, int status) => ResponseBody.fromString(
  jsonEncode(body),
  status,
  headers: {
    Headers.contentTypeHeader: [Headers.jsonContentType],
  },
);

class _MutableAuthRepo extends AuthRepository {
  _MutableAuthRepo(Map<String, dynamic>? user)
    : super(
        authService: AuthService(
          ApiClient(baseUrl: 'http://test.local'),
          DeviceIdService(),
        ),
        apiClient: ApiClient(baseUrl: 'http://test.local'),
      ) {
    state = AsyncData(user);
  }

  void setUser(Map<String, dynamic>? user) {
    state = AsyncData(user);
  }
}

Map<String, dynamic> _warga(String id, String rtId) => {
  'id': id,
  'role': AppRole.warga,
  'rt': rtId == 'rt-1' ? '002' : '003',
  'rt_id': rtId,
  'effective_permissions': <String>[],
};

Map<String, dynamic> _manager(String id, String rtId) => {
  'id': id,
  'role': AppRole.pengurus,
  'jabatan': 'ketua',
  'rt': rtId == 'rt-1' ? '002' : '003',
  'rt_id': rtId,
  'effective_permissions': <String>[
    'announcements.create',
    'activities.create',
  ],
};

ApiClient _apiClientWithAdapter(
  Future<ResponseBody> Function(RequestOptions) handler,
) {
  final client = ApiClient(baseUrl: 'http://test.local');
  client.dio.httpClientAdapter = _MockAdapter(handler);
  return client;
}

Future<void> pumpImage(WidgetTester tester, Widget child) async {
  await tester.pumpWidget(MaterialApp(home: Scaffold(body: child)));
  await tester.pumpAndSettle();
}

void main() {
  const draftKey = 'community/rt-1/announcement/draft.png';
  const draftKeyB = 'community/rt-1/announcement/draft-b.png';
  const otherRtKey = 'community/rt-2/announcement/draft.png';
  const publishedKey = 'community/rt-1/announcement/live.png';

  testWidgets('manager renders draft image through Bearer fetch', (
    tester,
  ) async {
    final apiClient = _apiClientWithAdapter((options) async {
      if (options.method == 'POST' &&
          options.path == '/api/v1/community/images/sign') {
        return _json({
          'error': {'code': 'not_found'},
        }, 404);
      }
      if (options.method == 'GET' &&
          options.path == '/api/v1/community/images/$draftKey') {
        return ResponseBody.fromBytes(
          tinyPng,
          200,
          headers: {
            Headers.contentTypeHeader: ['image/png'],
          },
        );
      }
      return _json({
        'error': {'code': 'unexpected'},
      }, 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith(
            (ref) => _MutableAuthRepo(_manager('u1', 'rt-1')),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKey)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final image = tester.widget<Image>(find.byType(Image));
    expect(image.image, isA<MemoryImage>());
  });

  testWidgets('warga sees placeholder for draft image', (tester) async {
    final apiClient = _apiClientWithAdapter((options) async {
      if (options.method == 'POST' &&
          options.path == '/api/v1/community/images/sign') {
        return _json({
          'error': {'code': 'not_found'},
        }, 404);
      }
      if (options.method == 'GET') {
        return _json({
          'error': {'code': 'not_found'},
        }, 404);
      }
      return _json({
        'error': {'code': 'unexpected'},
      }, 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith(
            (ref) => _MutableAuthRepo(_warga('u2', 'rt-1')),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKey)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(Image), findsNothing);
    expect(find.byIcon(Icons.image_outlined), findsOneWidget);
  });

  testWidgets('published image renders via signed URL for warga', (
    tester,
  ) async {
    final apiClient = _apiClientWithAdapter((options) async {
      if (options.method == 'POST' &&
          options.path == '/api/v1/community/images/sign') {
        return _json({
          'url':
              'http://test.local/api/v1/community/images/$publishedKey?exp=1&sig=abc',
        }, 200);
      }
      if (options.method == 'GET' && options.path.contains(publishedKey)) {
        return ResponseBody.fromBytes(
          tinyPng,
          200,
          headers: {
            Headers.contentTypeHeader: ['image/png'],
          },
        );
      }
      return _json({
        'error': {'code': 'unexpected'},
      }, 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith(
            (ref) => _MutableAuthRepo(_warga('u2', 'rt-1')),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: publishedKey)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final image = tester.widget<Image>(find.byType(Image));
    expect(image.image, isA<NetworkImage>());
  });

  testWidgets('imageRef change does not reuse stale bytes', (tester) async {
    final bytesA = Uint8List.fromList([...tinyPng, 0xAA]);
    final bytesB = Uint8List.fromList([...tinyPng, 0xBB]);
    final apiClient = _apiClientWithAdapter((options) async {
      if (options.method == 'POST' &&
          options.path == '/api/v1/community/images/sign') {
        return _json({
          'error': {'code': 'not_found'},
        }, 404);
      }
      if (options.method == 'GET') {
        if (options.path.endsWith(draftKey)) {
          return ResponseBody.fromBytes(
            bytesA,
            200,
            headers: {
              Headers.contentTypeHeader: ['image/png'],
            },
          );
        }
        if (options.path.endsWith(draftKeyB)) {
          return ResponseBody.fromBytes(
            bytesB,
            200,
            headers: {
              Headers.contentTypeHeader: ['image/png'],
            },
          );
        }
      }
      return _json({
        'error': {'code': 'unexpected'},
      }, 500);
    });
    final repo = _MutableAuthRepo(_manager('u1', 'rt-1'));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith((ref) => repo),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKey)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final first =
        (tester.widget<Image>(find.byType(Image)).image as MemoryImage).bytes;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith((ref) => repo),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKeyB)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final second =
        (tester.widget<Image>(find.byType(Image)).image as MemoryImage).bytes;
    expect(identical(first, second), isFalse);
  });

  testWidgets(
    'permission upgrade warga->manager resolves image on auth change',
    (tester) async {
      final apiClient = _apiClientWithAdapter((options) async {
        if (options.method == 'POST' &&
            options.path == '/api/v1/community/images/sign') {
          return _json({
            'error': {'code': 'not_found'},
          }, 404);
        }
        if (options.method == 'GET' &&
            options.path == '/api/v1/community/images/$draftKey') {
          return ResponseBody.fromBytes(
            tinyPng,
            200,
            headers: {
              Headers.contentTypeHeader: ['image/png'],
            },
          );
        }
        return _json({
          'error': {'code': 'unexpected'},
        }, 500);
      });
      final repo = _MutableAuthRepo(_warga('u2', 'rt-1'));

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiClientProvider.overrideWithValue(apiClient),
            authRepositoryProvider.overrideWith((ref) => repo),
          ],
          child: const MaterialApp(
            home: Scaffold(body: CommunityImage(imageRef: draftKey)),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Image), findsNothing);

      // Upgrade warga to manager; the auth listener must re-resolve.
      repo.setUser(_manager('u2', 'rt-1'));
      await tester.pumpAndSettle();

      final image = tester.widget<Image>(find.byType(Image));
      expect(image.image, isA<MemoryImage>());
    },
  );

  testWidgets('stale async result does not overwrite newer asset', (
    tester,
  ) async {
    final completer = Completer<ResponseBody>();
    final apiClient = _apiClientWithAdapter((options) async {
      if (options.method == 'POST' &&
          options.path == '/api/v1/community/images/sign') {
        if (options.data is Map && (options.data as Map)['key'] == draftKey) {
          return completer.future;
        }
        return _json({
          'error': {'code': 'not_found'},
        }, 404);
      }
      if (options.method == 'GET' && options.path.endsWith(draftKeyB)) {
        return ResponseBody.fromBytes(
          tinyPng,
          200,
          headers: {
            Headers.contentTypeHeader: ['image/png'],
          },
        );
      }
      return _json({
        'error': {'code': 'not_found'},
      }, 404);
    });
    final repo = _MutableAuthRepo(_manager('u1', 'rt-1'));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith((ref) => repo),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKey)),
        ),
      ),
    );
    await tester.pump();

    // Switch to draft-b before the stale A request resolves.
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith((ref) => repo),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKeyB)),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final providerB = tester.widget<Image>(find.byType(Image)).image;

    // Complete the stale A request now.
    if (!completer.isCompleted) {
      completer.complete(
        ResponseBody.fromBytes(
          tinyPng,
          200,
          headers: {
            Headers.contentTypeHeader: ['image/png'],
          },
        ),
      );
    }
    await tester.pumpAndSettle();

    final providerNow = tester.widget<Image>(find.byType(Image)).image;
    expect(identical(providerNow, providerB), isTrue);
  });

  testWidgets('provider evicted from image cache after RT/user switch', (
    tester,
  ) async {
    final apiClient = _apiClientWithAdapter((options) async {
      if (options.method == 'POST' &&
          options.path == '/api/v1/community/images/sign') {
        return _json({
          'error': {'code': 'not_found'},
        }, 404);
      }
      if (options.method == 'GET' &&
          options.path == '/api/v1/community/images/$draftKey') {
        return ResponseBody.fromBytes(
          tinyPng,
          200,
          headers: {
            Headers.contentTypeHeader: ['image/png'],
          },
        );
      }
      if (options.method == 'GET' &&
          options.path == '/api/v1/community/images/$otherRtKey') {
        return ResponseBody.fromBytes(
          Uint8List.fromList([...tinyPng, 0xCC]),
          200,
          headers: {
            Headers.contentTypeHeader: ['image/png'],
          },
        );
      }
      return _json({
        'error': {'code': 'unexpected'},
      }, 500);
    });
    final repo = _MutableAuthRepo(_manager('u1', 'rt-1'));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          authRepositoryProvider.overrideWith((ref) => repo),
        ],
        child: const MaterialApp(
          home: Scaffold(body: CommunityImage(imageRef: draftKey)),
        ),
      ),
    );
    await tester.pumpAndSettle();
    final oldProvider = tester.widget<Image>(find.byType(Image)).image;

    // Switch to a different RT (private scope change).
    repo.setUser(_manager('u1', 'rt-2'));
    await tester.pumpAndSettle();

    expect(
      PaintingBinding.instance.imageCache.containsKey(oldProvider),
      isFalse,
    );
  });
}
