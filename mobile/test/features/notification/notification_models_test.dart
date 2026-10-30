import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';

void main() {
  group('SystemNotificationType', () {
    test('fromJson & toJson serialization', () {
      final json = {
        'event_type': 'manual_push',
        'name': 'Pesan Siaran Manual',
        'trigger': 'Dipicu manual oleh Pengurus / RT',
        'recipients_rule': 'Seluruh warga aktif dalam RT',
        'default_title': 'Pengumuman Penting',
        'default_body': 'Pemberitahuan kepada seluruh warga',
        'allowed_variables': ['title', 'body'],
        'supports_image': true,
        'supports_test': true,
      };

      final model = SystemNotificationType.fromJson(json);
      expect(model.eventType, equals('manual_push'));
      expect(model.name, equals('Pesan Siaran Manual'));
      expect(model.allowedVariables, equals(['title', 'body']));
      expect(model.supportsImage, isTrue);
      expect(model.supportsTest, isTrue);

      final outputJson = model.toJson();
      expect(outputJson['event_type'], equals('manual_push'));
      expect(outputJson['allowed_variables'], equals(['title', 'body']));
      expect(outputJson['supports_image'], isTrue);
      expect(outputJson['supports_test'], isTrue);
    });
  });

  group('NotificationTemplate', () {
    test('fromJson, toJson & copyWith', () {
      final json = {
        'id': 'tmpl-123',
        'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
        'event_type': 'manual_push',
        'name': 'Template Iuran RT',
        'title': 'Ada tagihan baru',
        'body': 'Halo warga, tagihan {{bill_id}} terbit',
        'image_url': 'https://example.com/banner.jpg',
        'is_active': true,
        'created_at': '2026-10-29T00:00:00Z',
        'updated_at': '2026-10-29T01:00:00Z',
      };

      final tmpl = NotificationTemplate.fromJson(json);
      expect(tmpl.id, equals('tmpl-123'));
      expect(tmpl.imageUrl, equals('https://example.com/banner.jpg'));
      expect(tmpl.isActive, isTrue);
      expect(tmpl.createdAt, isNotNull);

      final updated = tmpl.copyWith(isActive: false, name: 'Template Baru', imageUrl: 'https://example.com/new.jpg');
      expect(updated.id, equals('tmpl-123'));
      expect(updated.isActive, isFalse);
      expect(updated.name, equals('Template Baru'));
      expect(updated.imageUrl, equals('https://example.com/new.jpg'));
    });
  });

  group('NotificationLogEntry & NotificationLogsResponse', () {
    test('fromJson and pagination parsing', () {
      final json = {
        'data': [
          {
            'id': 'log-1',
            'rt_id': 'a95c1cb5-2f67-4ea5-9197-bfec537b6d7b',
            'template_id': 'tmpl-1',
            'event_type': 'iuran_bill_created',
            'title': 'Tagihan Terbit',
            'body': 'Segera bayar nomor 101',
            'variables': {'bill_id': '101'},
            'recipients_count': 12,
            'created_at': '2026-10-29T01:00:00Z',
          }
        ],
        'pagination': {
          'page': 1,
          'per_page': 20,
          'total': 1,
          'total_pages': 1,
        }
      };

      final resp = NotificationLogsResponse.fromJson(json);
      expect(resp.total, equals(1));
      expect(resp.data.length, equals(1));
      final log = resp.data.first;
      expect(log.id, equals('log-1'));
      expect(log.recipientsCount, equals(12));
      expect(log.variables['bill_id'], equals('101'));
    });
  });
}

