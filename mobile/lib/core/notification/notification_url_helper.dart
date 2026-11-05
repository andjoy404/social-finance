import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

class NotificationTextSegment {
  final bool isLink;
  final String text;
  final String? url;

  const NotificationTextSegment.text(this.text)
      : isLink = false,
        url = null;

  const NotificationTextSegment.link({
    required this.text,
    required this.url,
  }) : isLink = true;
}

/// Menentukan apakah suatu notifikasi merupakan Notifikasi Manual (`manual_push`)
/// berdasarkan sumber data tepercaya (`eventType` dan `dedupKey`),
/// bukan sekadar karena teks mengandung URL.
/// Notifikasi dari Scheduler (`dedupKey` diawali `schedule:`) atau notifikasi sistem
/// lainnya tidak termasuk Notifikasi Manual.
bool isManualPushNotification({
  String? eventType,
  String? dedupKey,
}) {
  final normalizedType = (eventType ?? '').trim();
  if (normalizedType != 'manual_push') {
    return false;
  }
  final normalizedDedup = (dedupKey ?? '').trim();
  if (normalizedDedup.startsWith('schedule:')) {
    return false;
  }
  return true;
}

final RegExp _httpSchemePrefixRegex = RegExp(r'^https?:\/\/', caseSensitive: false);
final RegExp _whitespaceOrControlRegex = RegExp(r'[\s\x00-\x1F\x7F]');
final RegExp _httpCandidateRegex = RegExp(
  r'''(?:^|(?<=[\s(\[{"'<]))https?:\/\/[^\s<>"'`]+''',
  caseSensitive: false,
);

/// Memvalidasi apakah [rawUrl] merupakan URL `http://` atau `https://` yang aman dan valid.
/// Mengembalikan string URL asli tanpa merusak query string/parameter download jika valid,
/// atau `null` jika skema selain HTTP/HTTPS atau URL malformed.
String? validateSafeHttpUrl(String? rawUrl) {
  if (rawUrl == null) return null;
  final trimmed = rawUrl.trim();
  if (trimmed.isEmpty) return null;

  if (!_httpSchemePrefixRegex.hasMatch(trimmed)) {
    return null;
  }
  if (_whitespaceOrControlRegex.hasMatch(trimmed)) {
    return null;
  }

  final uri = Uri.tryParse(trimmed);
  if (uri == null || !uri.hasScheme || !uri.hasAuthority) {
    return null;
  }

  final scheme = uri.scheme.toLowerCase();
  if (scheme != 'http' && scheme != 'https') {
    return null;
  }

  final host = uri.host.trim();
  if (host.isEmpty ||
      host.startsWith('.') ||
      host.endsWith('.') ||
      host.contains('..')) {
    return null;
  }

  final afterScheme = trimmed.replaceFirst(_httpSchemePrefixRegex, '');
  if (afterScheme.isEmpty ||
      afterScheme.startsWith('/') ||
      afterScheme.startsWith(':')) {
    return null;
  }

  return trimmed;
}

int _countChar(String input, String char) {
  var count = 0;
  for (var i = 0; i < input.length; i++) {
    if (input[i] == char) count++;
  }
  return count;
}

({String candidate, String trailing}) _splitTrailingPunctuation(String rawToken) {
  var candidate = rawToken;
  var trailing = '';

  while (candidate.isNotEmpty) {
    final lastChar = candidate[candidate.length - 1];
    if ('.!,;:?\'"'.contains(lastChar)) {
      trailing = lastChar + trailing;
      candidate = candidate.substring(0, candidate.length - 1);
      continue;
    }
    if (lastChar == ')' && _countChar(candidate, ')') > _countChar(candidate, '(')) {
      trailing = lastChar + trailing;
      candidate = candidate.substring(0, candidate.length - 1);
      continue;
    }
    if (lastChar == ']' && _countChar(candidate, ']') > _countChar(candidate, '[')) {
      trailing = lastChar + trailing;
      candidate = candidate.substring(0, candidate.length - 1);
      continue;
    }
    if (lastChar == '}' && _countChar(candidate, '}') > _countChar(candidate, '{')) {
      trailing = lastChar + trailing;
      candidate = candidate.substring(0, candidate.length - 1);
      continue;
    }
    break;
  }

  return (candidate: candidate, trailing: trailing);
}

/// Memecah teks notifikasi menjadi segmen teks biasa dan segmen link HTTP/HTTPS.
/// Jika [isManualNotification] bernilai `false`, seluruh teks dikembalikan sebagai teks biasa.
List<NotificationTextSegment> parseNotificationTextSegments(
  String text, {
  required bool isManualNotification,
}) {
  if (text.isEmpty || !isManualNotification) {
    return [NotificationTextSegment.text(text)];
  }

  final segments = <NotificationTextSegment>[];
  var lastIndex = 0;

  void appendText(String chunk) {
    if (chunk.isEmpty) return;
    if (segments.isNotEmpty && !segments.last.isLink) {
      final prev = segments.removeLast();
      segments.add(NotificationTextSegment.text(prev.text + chunk));
    } else {
      segments.add(NotificationTextSegment.text(chunk));
    }
  }

  for (final match in _httpCandidateRegex.allMatches(text)) {
    final matchStart = match.start;
    final rawToken = match.group(0) ?? '';

    if (matchStart > lastIndex) {
      appendText(text.substring(lastIndex, matchStart));
    }

    final split = _splitTrailingPunctuation(rawToken);
    final validUrl = validateSafeHttpUrl(split.candidate);

    if (validUrl != null) {
      segments.add(
        NotificationTextSegment.link(
          text: validUrl,
          url: validUrl,
        ),
      );
      if (split.trailing.isNotEmpty) {
        appendText(split.trailing);
      }
    } else {
      appendText(rawToken);
    }

    lastIndex = match.end;
  }

  if (lastIndex < text.length) {
    appendText(text.substring(lastIndex));
  }

  if (segments.isEmpty) {
    return [NotificationTextSegment.text(text)];
  }

  return segments;
}

/// Mengambil URL HTTP/HTTPS pertama yang valid dari teks apabila notifikasi adalah Notifikasi Manual.
String? extractFirstValidHttpUrl(
  String? text, {
  required bool isManualNotification,
}) {
  if (text == null || text.isEmpty || !isManualNotification) {
    return null;
  }
  final segments = parseNotificationTextSegments(
    text,
    isManualNotification: true,
  );
  for (final segment in segments) {
    if (segment.isLink && segment.url != null) {
      return segment.url;
    }
  }
  return null;
}

/// Menentukan apakah tap pada push notification dari notification tray harus
/// langsung membuka browser eksternal (khusus `manual_push` yang memiliki URL HTTP/HTTPS valid).
String? resolveManualNotificationTrayExternalUrl(
  String? deepLink, {
  Map<String, dynamic>? payload,
}) {
  Uri? parsedLink;
  if (deepLink != null && deepLink.trim().isNotEmpty) {
    parsedLink = Uri.tryParse(deepLink.trim());
  }

  final rawType = (payload?['type']?.toString() ??
          payload?['event_type']?.toString() ??
          parsedLink?.queryParameters['type'] ??
          parsedLink?.queryParameters['event_type'] ??
          '')
      .trim();

  final rawDedupKey = (payload?['dedup_key']?.toString() ??
          parsedLink?.queryParameters['dedup_key'] ??
          '')
      .trim();

  if (!isManualPushNotification(eventType: rawType, dedupKey: rawDedupKey)) {
    return null;
  }

  final rawBody = (payload?['body']?.toString() ??
          parsedLink?.queryParameters['body'] ??
          '')
      .trim();

  if (rawBody.isEmpty) {
    return null;
  }

  return extractFirstValidHttpUrl(
    rawBody,
    isManualNotification: true,
  );
}

/// Widget yang merender isi pesan notifikasi.
/// Untuk Notifikasi Manual yang memiliki URL HTTP/HTTPS valid, setiap URL dapat diketuk.
/// Untuk Notifikasi Sistem / Scheduler atau Notifikasi Manual tanpa URL, dirender sebagai `Text` biasa.
class NotificationBodyText extends StatefulWidget {
  final String text;
  final String? eventType;
  final String? dedupKey;
  final TextStyle? style;
  final TextStyle? linkStyle;
  final int? maxLines;
  final TextOverflow? overflow;
  final ValueChanged<String> onOpenUrl;

  const NotificationBodyText({
    super.key,
    required this.text,
    required this.eventType,
    this.dedupKey,
    this.style,
    this.linkStyle,
    this.maxLines,
    this.overflow,
    required this.onOpenUrl,
  });

  @override
  State<NotificationBodyText> createState() => _NotificationBodyTextState();
}

class _NotificationBodyTextState extends State<NotificationBodyText> {
  final List<TapGestureRecognizer> _recognizers = [];

  void _disposeRecognizers() {
    for (final recognizer in _recognizers) {
      recognizer.dispose();
    }
    _recognizers.clear();
  }

  @override
  void dispose() {
    _disposeRecognizers();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    _disposeRecognizers();

    final isManual = isManualPushNotification(
      eventType: widget.eventType,
      dedupKey: widget.dedupKey,
    );

    if (!isManual || widget.text.isEmpty) {
      return Text(
        widget.text,
        style: widget.style,
        maxLines: widget.maxLines,
        overflow: widget.overflow,
      );
    }

    final segments = parseNotificationTextSegments(
      widget.text,
      isManualNotification: true,
    );

    final hasLinks = segments.any((s) => s.isLink && s.url != null);
    if (!hasLinks) {
      return Text(
        widget.text,
        style: widget.style,
        maxLines: widget.maxLines,
        overflow: widget.overflow,
      );
    }

    final baseStyle = widget.style ?? DefaultTextStyle.of(context).style;
    final effectiveLinkStyle = widget.linkStyle ??
        baseStyle.copyWith(
          color: const Color(0xFF60A5FA),
          decoration: TextDecoration.underline,
          decorationColor: const Color(0xFF60A5FA),
        );

    final spans = <InlineSpan>[];
    for (final segment in segments) {
      if (segment.isLink && segment.url != null) {
        final targetUrl = segment.url!;
        final recognizer = TapGestureRecognizer()
          ..onTap = () {
            widget.onOpenUrl(targetUrl);
          };
        _recognizers.add(recognizer);
        spans.add(
          TextSpan(
            text: segment.text,
            style: effectiveLinkStyle,
            recognizer: recognizer,
          ),
        );
      } else {
        spans.add(
          TextSpan(
            text: segment.text,
            style: baseStyle,
          ),
        );
      }
    }

    return Text.rich(
      TextSpan(children: spans),
      maxLines: widget.maxLines,
      overflow: widget.overflow ?? TextOverflow.clip,
    );
  }
}
