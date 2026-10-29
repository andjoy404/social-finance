import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/errors/app_errors.dart';
import 'package:social_finance/core/widgets/error_state.dart';
import 'package:social_finance/features/notification/data/notification_api_client.dart';
import 'package:social_finance/features/notification/data/notification_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/features/notification/presentation/screens/notification_detail_screen.dart';

/// Repository pemalsuan yang melemparkan error tipe tertentu.
class _ThrowingNotificationRepository extends NotificationRepository {
  final Object? throwable;

  _ThrowingNotificationRepository({
    required apiClient,
    this.throwable,
  }) : super(apiClient: apiClient);

  @override
  Future<NotificationLogEntry> fetchNotificationDetail(String id) async {
    if (throwable != null) throw throwable!;
    throw Exception('unexpected');
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  /// Membangun widget NotificationDetailScreen dengan repository pemalsuan.
  Widget _buildDetailScreen({
    required Object errorToThrow,
    String? notificationId,
    bool openedFromNotificationList = false,
  }) {
    final dio = Dio();
    dio.options.baseUrl = 'http://192.168.1.1:8085/api/v1/';
    final apiClient = NotificationApiClient(dio: dio);
    final repo = _ThrowingNotificationRepository(
      apiClient: apiClient,
      throwable: errorToThrow,
    );

    return MaterialApp(
      home: ProviderScope(
        overrides: [
          notificationRepositoryProvider.overrideWithValue(repo),
        ],
        child: NotificationDetailScreen(
          notificationId: notificationId ?? 'notif-test',
          openedFromNotificationList: openedFromNotificationList,
        ),
      ),
    );
  }

  /// Menunggu widget siap dan menampung semua animation frame.
  Future<void> _pumpAndSettle(WidgetTester tester) async {
    await tester.pumpAndSettle(const Duration(seconds: 5));
  }

  group('_mapErrorToUserMessage regression — ServerException', () {
    testWidgets(
      'ServerException dengan status 401 menampilkan pesan sesi berakhir',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(
            statusCode: 401,
            message: 'Unauthorized',
          ),
        ));
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Sesi Anda telah berakhir. Silakan login kembali.'),
        );
        expect(
          find.text("Instance of 'ServerException'"),
          findsNothing,
          reason: 'Raw exception instance tidak boleh ditampilkan ke pengguna',
        );
      },
    );

    testWidgets(
      'ServerException dengan status 403 menampilkan pesan akses ditolak',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(
            statusCode: 403,
            message: 'Forbidden',
          ),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Anda tidak memiliki akses untuk melihat notifikasi ini.'),
        );
      },
    );

    testWidgets(
      'ServerException dengan status 404 menampilkan pesan tidak ditemukan',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(
            statusCode: 404,
            message: 'Not Found',
          ),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Detail notifikasi tidak ditemukan.'),
        );
      },
    );

    testWidgets(
      'ServerException dengan status 408 menampilkan pesan timeout',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(
            statusCode: 408,
            message: 'Request Timeout',
          ),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Permintaan timeout. Periksa koneksi Anda dan coba lagi.'),
        );
      },
    );

    testWidgets(
      'ServerException dengan status 500 menampilkan pesan server error',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(
            statusCode: 500,
            message: 'Internal Server Error',
          ),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Server sedang mengalami gangguan. Coba lagi nanti.'),
        );
      },
    );

    testWidgets(
      'ServerException tanpa status dengan pesan menampilkan pesan tersebut',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(message: 'Custom server error'),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(errorStateWidget.message, equals('Custom server error'));
      },
    );

    testWidgets(
      'ServerException tanpa status dengan pesan kosong menampilkan pesan umum',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(message: ''),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Terjadi kesalahan. Coba lagi nanti.'),
        );
      },
    );

    testWidgets(
      'ServerException tanpa status dengan pesan hanya whitespace menampilkan pesan umum',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(message: '   '),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Terjadi kesalahan. Coba lagi nanti.'),
        );
      },
    );
  });

  group('_mapErrorToUserMessage regression — DioException', () {
    /// Helper untuk membuat DioException dengan response kustom.
    DioException _makeDioException({
      int? statusCode,
      Object? responseBody,
      Object? dioError,
    }) {
      return DioException(
        requestOptions: RequestOptions(path: '/api/v1/notification/detail/test'),
        error: dioError ?? 'dio error',
        response: Response(
          statusCode: statusCode,
          data: responseBody,
          requestOptions: RequestOptions(path: '/api/v1/notification/detail/test'),
        ),
      );
    }

    testWidgets(
      'DioException dengan 401 dan body error backend valid menampilkan pesan dari backend',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(
            statusCode: 401,
            responseBody: {'error': {'message': 'Token kadaluarsa'}},
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-401',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        expect(
          find.text('Instance of'),
          findsNothing,
          reason: 'Raw DioException instance tidak boleh ditampilkan ke pengguna',
        );
      },
    );

    testWidgets(
      'DioException dengan 403 dan body error backend valid menampilkan pesan dari backend',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(
            statusCode: 403,
            responseBody: {'error': {'message': 'Tidak memiliki izin'}},
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-403',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(errorStateWidget.message, equals('Tidak memiliki izin'));
        expect(
          find.text('Instance of'),
          findsNothing,
        );
      },
    );

    testWidgets(
      'DioException dengan status 404 menampilkan pesan tidak ditemukan',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(statusCode: 404),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-404',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Detail notifikasi tidak ditemukan.'),
        );
      },
    );

    testWidgets(
      'DioException dengan status 408 menampilkan pesan timeout',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(statusCode: 408),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-408',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Permintaan timeout. Periksa koneksi Anda dan coba lagi.'),
        );
      },
    );

    testWidgets(
      'DioException dengan status 500 menampilkan pesan server error',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(statusCode: 500),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-500',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Server sedang mengalami gangguan. Coba lagi nanti.'),
        );
      },
    );

    testWidgets(
      'DioException dengan 403 dan body backend kosong menggunakan fallback HTTP status',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(
            statusCode: 403,
            responseBody: {'error': {'message': ''}},
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-403-empty',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Anda tidak memiliki akses untuk melihat notifikasi ini.'),
        );
      },
    );

    testWidgets(
      'DioException dengan response body non-Map tidak crash',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: _makeDioException(
            statusCode: 500,
            responseBody: 'plain text error response',
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-malformed',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        expect(
          find.text('Instance of'),
          findsNothing,
          reason: 'No raw exception instance should be visible',
        );
      },
    );

    testWidgets(
      'DioException dengan null response data tidak crash',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: DioException(
            requestOptions: RequestOptions(path: '/api/v1/notification/detail/test'),
            error: null,
            response: Response(
              statusCode: 500,
              data: null,
              requestOptions: RequestOptions(path: '/api/v1/notification/detail/test'),
            ),
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-null',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        expect(
          find.text('Instance of'),
          findsNothing,
        );
      },
    );

    testWidgets(
      'DioException dengan null response object tidak crash',
      (tester) async {
        final apiClient = NotificationApiClient(dio: Dio());
        final repo = _ThrowingNotificationRepository(
          apiClient: apiClient,
          throwable: DioException(
            requestOptions: RequestOptions(path: '/api/v1/notification/detail/test'),
            error: null,
            response: null,
          ),
        );

        await tester.pumpWidget(
          MaterialApp(
            home: ProviderScope(
              overrides: [
                notificationRepositoryProvider.overrideWithValue(repo),
              ],
              child: const NotificationDetailScreen(
                notificationId: 'notif-null-response',
              ),
            ),
          ),
        );
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        expect(
          find.text('Instance of'),
          findsNothing,
        );
      },
    );
  });

  group('_mapErrorToUserMessage regression — AppWithMessageError & unknown', () {
    testWidgets(
      'AppWithMessageError menampilkan pesan yang diberikan',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: AppWithMessageError('Perintah tidak valid'),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(errorStateWidget.message, equals('Perintah tidak valid'));
      },
    );

    testWidgets(
      'FormatException menampilkan pesan error tidak terduga',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: FormatException('Unexpected data format'),
        ));
        await _pumpAndSettle(tester);

        final errorStateWidget =
            find.byType(ErrorState).evaluate().first.widget as ErrorState;
        expect(
          errorStateWidget.message,
          equals('Terjadi kesalahan tidak terduga. Silakan coba lagi.'),
        );
        expect(find.text('Instance of'), findsNothing);
      },
    );

    testWidgets(
      'Generic Exception tidak crash dan menampilkan pesan umum',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: Exception('some random error'),
        ));
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        expect(find.text('Instance of'), findsNothing);
      },
    );

    testWidgets(
      'TypeError (rare exception) tidak crash dan menampilkan pesan umum',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: TypeError(),
        ));
        await _pumpAndSettle(tester);

        expect(find.byType(ErrorState), findsOneWidget);
        expect(find.text('Instance of'), findsNothing);
      },
    );
  });

  group(
    'UI regression — tidak ada teks exception mentah yang pernah ditampilkan ke pengguna',
    () {
      final allErrors = <Object>[
        ServerException(statusCode: 401, message: 'Unauthorized'),
        ServerException(statusCode: 403, message: 'Forbidden'),
        ServerException(statusCode: 404, message: 'Not Found'),
        ServerException(statusCode: 408, message: 'Timeout'),
        ServerException(statusCode: 500, message: 'Server Error'),
        ServerException(message: ''),
        ServerException(message: '   '),
        DioException(
          requestOptions: RequestOptions(path: '/detail/1'),
          error: 'test',
          response: Response(
            data: 'bad',
            requestOptions: RequestOptions(path: '/detail/1'),
            statusCode: 500,
          ),
        ),
        DioException(
          requestOptions: RequestOptions(path: '/detail/1'),
          error: null,
          response: null,
        ),
        AppWithMessageError('Validation failed'),
        FormatException('bad data'),
        Exception('unknown'),
        TypeError(),
      ];

      for (final error in allErrors) {
        testWidgets(
          'Tipe error ${error.runtimeType} tidak menampilkan teks "Instance of"',
          (tester) async {
            final dio = Dio();
            final apiClient = NotificationApiClient(dio: dio);
            final repo = _ThrowingNotificationRepository(
              apiClient: apiClient,
              throwable: error,
            );

            await tester.pumpWidget(
              MaterialApp(
                home: ProviderScope(
                  overrides: [
                    notificationRepositoryProvider.overrideWithValue(repo),
                  ],
                  child: NotificationDetailScreen(
                    notificationId: 'notif-${allErrors.indexOf(error)}',
                  ),
                ),
              ),
            );
            await _pumpAndSettle(tester);

            expect(find.byType(ErrorState), findsOneWidget);
            final textWidgets =
                find.byType(Text).evaluate().map((e) => e.widget as Text);
            for (final textWidget in textWidgets) {
              expect(
                textWidget.data,
                isNot(contains('Instance of')),
                reason:
                    'Raw exception toString() bocor ke UI: ${error.runtimeType} menampilkan "${textWidget.data}"',
              );
            }
          },
        );
      }
    },
  );

  group('ErrorState widget — tombol retry', () {
    testWidgets(
      'ErrorState menampilkan "Coba Lagi" sebagai teks tombol aksi default',
      (tester) async {
        await tester.pumpWidget(_buildDetailScreen(
          errorToThrow: ServerException(message: 'Test error'),
        ));
        await _pumpAndSettle(tester);

        expect(
          find.text('Coba Lagi'),
          findsOneWidget,
          reason: 'ErrorState harus menampilkan teks aksi retry',
        );
      },
    );
  });
}
