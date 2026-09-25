import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

class AppError {}

class AppWithMessageError extends AppError {
  final String message;
  AppWithMessageError(this.message);
}

class AppWithCauseError extends AppError {
  final String message;
  final Object? cause;
  AppWithCauseError(this.message, [this.cause]);
}

class ValidationError extends AppError {
  final String message;
  ValidationError(this.message);
}

class AuthError extends AppError {
  final String message;
  AuthError(this.message);

  @override
  String toString() => message;
}

class ServerException implements AppError {
  final int? statusCode;
  final String message;
  ServerException({this.statusCode, required this.message});
}

String errorMessageToString(AppError error) {
  switch (error) {
    case AppWithMessageError(:final message):
      return message;
    case AppWithCauseError(:final message, :final cause):
      return cause != null ? '$message: $cause' : message;
    default:
      return 'Terjadi kesalahan yang tidak diketahui.';
  }
}

class ErrorHandlingMixin {
  void handleAppError(AppError error) {
    debugPrint('Error: ${errorMessageToString(error)}');
  }
}
