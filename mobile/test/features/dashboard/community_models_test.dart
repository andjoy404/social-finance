import 'package:flutter_test/flutter_test.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/utils/permissions.dart';
import 'package:social_finance/features/dashboard/data/models/community_models.dart';

void main() {
  group('CommunityBanner.fromJson', () {
    test('parses backend keys (deep_link, start_at, body, source_id)', () {
      final banner = CommunityBanner.fromJson({
        'id': 'b1',
        'rt_id': 'rt-1',
        'title': 'Kerja Bakti',
        'body': 'Sabtu pagi',
        'image_url': '/api/v1/community/images/community/rt-1/announcement/x.png',
        'source_type': 'activity',
        'source_id': 'a1',
        'deep_link': '/kegiatan/a1',
        'start_at': '2026-11-10T07:00:00+07:00',
        'end_at': '2026-11-10T09:00:00+07:00',
        'priority': 4,
        'display_order': 2,
        'is_pinned': true,
        'is_active': true,
        'created_at': '2026-11-01T00:00:00Z',
      });

      expect(banner.actionUrl, '/kegiatan/a1');
      expect(banner.description, 'Sabtu pagi');
      expect(banner.imageUrl, contains('/community/rt-1/'));
      expect(banner.startDate, isNotNull);
      expect(banner.endDate, isNotNull);
      expect(banner.sourceType, 'activity');
      expect(banner.sourceId, 'a1');
      expect(banner.priority, 4);
      expect(banner.isPinned, true);
    });

    test('falls back to legacy keys (action_url/start_date) when backend keys absent', () {
      final banner = CommunityBanner.fromJson({
        'id': 'b2',
        'rt_id': 'rt-1',
        'title': 'Legacy',
        'description': 'legacy body',
        'action_url': '/keuangan',
        'start_date': '2026-11-10T07:00:00+07:00',
        'end_date': null,
        'created_at': '2026-11-01T00:00:00Z',
      });

      expect(banner.actionUrl, '/keuangan');
      expect(banner.description, 'legacy body');
      expect(banner.startDate, isNotNull);
    });
  });

  group('CommunityActivity.fromJson', () {
    test('parses starts_at/ends_at (backend shape)', () {
      final activity = CommunityActivity.fromJson({
        'id': 'a1',
        'rt_id': 'rt-1',
        'title': 'Kerja Bakti',
        'description': 'bersih-bersih',
        'starts_at': '2026-11-10T07:00:00+07:00',
        'ends_at': '2026-11-10T09:00:00+07:00',
        'location': 'Balai RT',
        'status': 'upcoming',
        'publication_status': 'published',
        'created_at': '2026-11-01T00:00:00Z',
        'updated_at': '2026-11-01T00:00:00Z',
      });

      expect(activity.status, 'upcoming');
      expect(activity.publicationStatus, 'published');
      expect(activity.startDate.year, 2026);
      expect(activity.endDate, isNotNull);
    });
  });

  group('OrganizationStructure.fromJson', () {
    test('parses backend nodes array and derives levels incl. level 4', () {
      final org = OrganizationStructure.fromJson({
        'rt_id': 'rt-1',
        'rt_name': 'Urena',
        'rt_number': '002',
        'rw_number': '016',
        'nodes': [
          {
            'position': 'ketua',
            'position_label': 'Ketua RT',
            'level': 1,
            'parent_position': '',
            'members': [
              {'user_id': 'u1', 'membership_id': 'm1', 'full_name': 'Andriyan', 'jabatan': 'ketua'},
            ],
          },
          {
            'position': 'keamanan',
            'position_label': 'Keamanan',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u2', 'membership_id': 'm2', 'full_name': 'Budi', 'jabatan': 'keamanan', 'phone': '0812'},
            ],
          },
        ],
      });

      expect(org.levels.keys, containsAll([1, 4]));
      expect(org.levels[1]!.single.positionLabel, 'Ketua RT');
      expect(org.levels[4]!.single.members.single.name, 'Budi');
      expect(org.levels[4]!.single.members.single.phone, '0812');
      expect(org.rtNumber, '002');
    });

    test('phone is absent when backend withholds it (no contact permission)', () {
      final org = OrganizationStructure.fromJson({
        'rt_id': 'rt-1',
        'nodes': [
          {
            'position': 'sekretaris',
            'position_label': 'Sekretaris',
            'level': 2,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u3', 'membership_id': 'm3', 'full_name': 'Sekret', 'jabatan': 'sekretaris'},
            ],
          },
        ],
      });

      expect(org.levels[2]!.single.members.single.phone, isNull);
    });
  });

  group('HomeDashboardData.fromJson', () {
    test('parses backend security_on_duty + user_full_name keys', () {
      final data = HomeDashboardData.fromJson({
        'greeting': 'Selamat pagi',
        'user_full_name': 'Andriyan',
        'rt_number': '002',
        'rw_number': '016',
        'banners': [],
        'upcoming_activities': [],
        'security_on_duty': {
          'is_available': false,
          'message': 'Belum ada petugas',
          'officers': [],
        },
        'organization': {'nodes': []},
      });

      expect(data.userName, 'Andriyan');
      expect(data.rt, '002');
      expect(data.rw, '016');
      expect(data.security.message, 'Belum ada petugas');
    });
  });

  group('community permissions', () {
    test('warga without permissions cannot manage community content', () {
      final user = <String, dynamic>{
        'role': AppRole.warga,
        'effective_permissions': <String>[],
      };
      expect(canManageAnnouncements(user), false);
      expect(canManageActivities(user), false);
      expect(userHasPermission(user, 'announcements.publish'), false);
    });

    test('manager permissions grant content management', () {
      final user = <String, dynamic>{
        'role': AppRole.warga,
        'jabatan': 'ketua',
        'effective_permissions': <String>[
          'activities.create',
          'announcements.publish',
        ],
      };
      expect(canManageActivities(user), true);
      expect(userHasPermission(user, 'announcements.publish'), true);
    });

    test('super admin bypasses permission checks', () {
      final user = <String, dynamic>{'role': AppRole.superAdmin};
      expect(userHasPermission(user, 'announcements.publish'), true);
      expect(canManageActivities(user), true);
    });
  });

  group('OrganizationStructure.buildActiveDiagramTiers', () {
    test('enforces canonical 4-tier hierarchy regardless of backend node level or order', () {
      final org = OrganizationStructure.fromJson({
        'rt_id': 'rt-1',
        'nodes': [
          // Intentionally scrambled order and backend levels (sekretaris at level 2, bendahara at level 3)
          {
            'position': 'operasional',
            'position_label': 'Operasional',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u8', 'membership_id': 'm8', 'full_name': 'Warga Operasional', 'jabatan': 'operasional'},
            ],
          },
          {
            'position': 'sekretaris',
            'position_label': 'Sekretaris',
            'level': 2,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u3', 'membership_id': 'm3', 'full_name': 'Warga Sekretaris', 'jabatan': 'sekretaris'},
            ],
          },
          {
            'position': 'sosial',
            'position_label': 'Sosial',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u7', 'membership_id': 'm7', 'full_name': 'Warga Sosial', 'jabatan': 'sosial'},
            ],
          },
          {
            'position': 'wakil_ketua',
            'position_label': 'Wakil Ketua',
            'level': 2,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u2', 'membership_id': 'm2', 'full_name': 'Warga Wakil', 'jabatan': 'wakil_ketua'},
            ],
          },
          {
            'position': 'kebersihan',
            'position_label': 'Kebersihan',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u6', 'membership_id': 'm6', 'full_name': 'Warga Kebersihan', 'jabatan': 'kebersihan'},
            ],
          },
          {
            'position': 'bendahara',
            'position_label': 'Bendahara',
            'level': 3,
            'parent_position': 'sekretaris',
            'members': [
              {'user_id': 'u4', 'membership_id': 'm4', 'full_name': 'Warga Bendahara', 'jabatan': 'bendahara'},
            ],
          },
          {
            'position': 'keamanan',
            'position_label': 'Keamanan',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u5', 'membership_id': 'm5', 'full_name': 'Warga Keamanan', 'jabatan': 'keamanan'},
            ],
          },
          {
            'position': 'ketua',
            'position_label': 'Ketua',
            'level': 1,
            'parent_position': '',
            'members': [
              {'user_id': 'u1', 'membership_id': 'm1', 'full_name': 'Warga Ketua', 'jabatan': 'ketua'},
            ],
          },
        ],
      });

      final tiers = org.buildActiveDiagramTiers();
      expect(tiers.length, 4);
      expect(tiers[0].map((n) => n.position).toList(), ['ketua']);
      expect(tiers[1].map((n) => n.position).toList(), ['wakil_ketua']);
      expect(tiers[2].map((n) => n.position).toList(), ['bendahara', 'sekretaris']);
      expect(
        tiers[3].map((n) => n.position).toList(),
        ['keamanan', 'kebersihan', 'sosial', 'operasional'],
      );
    });

    test('filters out empty positions, inactive members, and empty tiers', () {
      final org = OrganizationStructure.fromJson({
        'rt_id': 'rt-1',
        'nodes': [
          {
            'position': 'ketua',
            'position_label': 'Ketua',
            'level': 1,
            'parent_position': '',
            'members': [
              {'user_id': 'u1', 'membership_id': 'm1', 'full_name': 'Pak Ketua', 'jabatan': 'ketua', 'is_active': true},
            ],
          },
          {
            'position': 'wakil_ketua',
            'position_label': 'Wakil Ketua',
            'level': 2,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u2', 'membership_id': 'm2', 'full_name': 'Pak Wakil', 'jabatan': 'wakil_ketua', 'is_active': true},
            ],
          },
          {
            'position': 'bendahara',
            'position_label': 'Bendahara',
            'level': 3,
            'parent_position': 'sekretaris',
            'members': [], // empty
          },
          {
            'position': 'sekretaris',
            'position_label': 'Sekretaris',
            'level': 2,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u3', 'membership_id': 'm3', 'full_name': 'Bu Sekretaris', 'jabatan': 'sekretaris', 'is_active': true},
            ],
          },
          {
            'position': 'keamanan',
            'position_label': 'Keamanan',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u5a', 'membership_id': 'm5a', 'full_name': 'Aktif Keamanan', 'jabatan': 'keamanan', 'is_active': true},
              {'user_id': 'u5b', 'membership_id': 'm5b', 'full_name': 'Nonaktif Keamanan', 'jabatan': 'keamanan', 'is_active': false},
            ],
          },
          {
            'position': 'kebersihan',
            'position_label': 'Kebersihan',
            'level': 4,
            'parent_position': 'ketua',
            'members': [
              {'user_id': 'u6', 'membership_id': 'm6', 'full_name': 'Nonaktif Kebersihan', 'jabatan': 'kebersihan', 'status': 'nonaktif'},
            ],
          },
        ],
      });

      final tiers = org.buildActiveDiagramTiers();
      expect(tiers.length, 4);
      expect(tiers[0].map((n) => n.position).toList(), ['ketua']);
      expect(tiers[1].map((n) => n.position).toList(), ['wakil_ketua']);
      expect(tiers[2].map((n) => n.position).toList(), ['sekretaris']);
      expect(tiers[3].map((n) => n.position).toList(), ['keamanan']);
      expect(tiers[3].single.members.map((m) => m.name).toList(), ['Aktif Keamanan']);
    });
  });
}
