import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

class LinkSegment {
  final String text;
  final String? url;

  const LinkSegment({required this.text, this.url});

  bool get isLink => url != null && url!.isNotEmpty;
}

class UrlDetector {
  UrlDetector._();

  static final _urlRegex = RegExp(r'https?://[^\s<>"{}|\\^`\[\]]+', caseSensitive: false);
  static final _trailingPunctRegex = RegExp(r'[.,;:!?)\]>}]+$');

  /// Splits [text] into text and link segments, cleanly stripping trailing
  /// punctuation from the link URL and placing it into following text.
  static List<LinkSegment> parseSegments(String text) {
    if (text.isEmpty) return const [];
    final segments = <LinkSegment>[];
    int lastIndex = 0;

    for (final match in _urlRegex.allMatches(text)) {
      if (match.start > lastIndex) {
        segments.add(LinkSegment(text: text.substring(lastIndex, match.start)));
      }
      final raw = match.group(0)!;
      final punctMatch = _trailingPunctRegex.firstMatch(raw);
      String cleanUrl = raw;
      String punct = '';
      if (punctMatch != null) {
        cleanUrl = raw.substring(0, punctMatch.start);
        punct = raw.substring(punctMatch.start);
      }

      final uri = Uri.tryParse(cleanUrl);
      if (uri != null && (uri.scheme == 'http' || uri.scheme == 'https') && uri.hasAuthority) {
        segments.add(LinkSegment(text: cleanUrl, url: cleanUrl));
      } else {
        segments.add(LinkSegment(text: cleanUrl));
      }

      if (punct.isNotEmpty) {
        segments.add(LinkSegment(text: punct));
      }
      lastIndex = match.end;
    }

    if (lastIndex < text.length) {
      segments.add(LinkSegment(text: text.substring(lastIndex)));
    }
    return segments;
  }

  /// Extract all validated HTTP/HTTPS URLs found in [text].
  static List<String> extractUrls(String text) {
    return parseSegments(text).where((s) => s.isLink).map((s) => s.url!).toList();
  }

  /// Open [url] in the default browser.
  static Future<bool> openUrl(String url) async {
    try {
      final uri = Uri.tryParse(url);
      if (uri == null) return false;
      if (uri.scheme != 'http' && uri.scheme != 'https') return false;
      return await launchUrl(uri, mode: LaunchMode.externalApplication);
    } catch (e) {
      debugPrint('Failed to open URL $url: $e');
      return false;
    }
  }
}
