-- Migration 068 Rollback: Community, Activities, Announcements, Banners

DELETE FROM position_permissions
WHERE permission_id IN (
    SELECT id FROM permission_types WHERE code IN (
        'activities.read', 'activities.create', 'activities.update', 'activities.publish', 'activities.cancel',
        'announcements.read', 'announcements.create', 'announcements.update', 'announcements.publish', 'announcements.archive',
        'banner_templates.read', 'banner_templates.manage',
        'organization.read', 'organization.contact.read'
    )
);

DELETE FROM permission_types
WHERE code IN (
    'activities.read', 'activities.create', 'activities.update', 'activities.publish', 'activities.cancel',
    'announcements.read', 'announcements.create', 'announcements.update', 'announcements.publish', 'announcements.archive',
    'banner_templates.read', 'banner_templates.manage',
    'organization.read', 'organization.contact.read'
);

DROP TABLE IF EXISTS banners;
DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS banner_templates;
