-- Migration 068: Activities, Announcements, Banner Templates, Banners & Community Permissions

-- 1. Banner Templates table
CREATE TABLE IF NOT EXISTS banner_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID REFERENCES rts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL DEFAULT 'general',
    title_template VARCHAR(255) NOT NULL,
    body_template TEXT NOT NULL,
    image_url TEXT,
    default_priority INT NOT NULL DEFAULT 1,
    is_system BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_banner_templates_rt ON banner_templates(rt_id);
CREATE INDEX IF NOT EXISTS idx_banner_templates_category ON banner_templates(category);

-- Pre-seed System Templates (available globally, rt_id IS NULL)
INSERT INTO banner_templates (name, category, title_template, body_template, default_priority, is_system, is_active)
VALUES
    ('Pengingat IPL/Iuran', 'billing', 'Pengingat Pembayaran Iuran Warga', 'Bagi warga yang belum menyelesaikan iuran periode berjalan, mohon dapat melakukan pembayaran melalui menu Keuangan.', 10, true, true),
    ('Kerja Bakti', 'community', 'Kerja Bakti Lingkungan RT', 'Mari bersama-sama membersihkan dan merapikan lingkungan RT kita. Kumpul di pos warga.', 5, true, true),
    ('Rapat Warga', 'meeting', 'Rapat Koordinasi Warga RT', 'Undangan rapat warga rutin untuk membahas program lingkungan dan evaluasi bulanan.', 5, true, true),
    ('Pengumuman Umum', 'general', 'Pengumuman Penting Pengurus RT', 'Informasi penting dari pengurus RT untuk seluruh warga. Mohon diperhatikan bersama.', 3, true, true),
    ('Kegiatan / Event Lingkungan', 'event', 'Kegiatan Warga RT', 'Ikuti dan ramaikan kegiatan lingkungan bersama seluruh keluarga dan tetangga.', 4, true, true)
ON CONFLICT DO NOTHING;

-- 2. Activities table
CREATE TABLE IF NOT EXISTS activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    poster_url TEXT,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    location VARCHAR(255),
    organizer VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'upcoming' CHECK (status IN ('upcoming', 'ongoing', 'completed', 'cancelled')),
    publication_status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (publication_status IN ('draft', 'published')),
    show_on_banner BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activities_rt_starts ON activities(rt_id, starts_at);
CREATE INDEX IF NOT EXISTS idx_activities_rt_pub_status ON activities(rt_id, publication_status, status);

-- 3. Announcements table
CREATE TABLE IF NOT EXISTS announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    image_url TEXT,
    deep_link VARCHAR(255),
    template_id UUID REFERENCES banner_templates(id) ON DELETE SET NULL,
    scheduled_start TIMESTAMPTZ,
    scheduled_end TIMESTAMPTZ,
    priority INT NOT NULL DEFAULT 1,
    display_order INT NOT NULL DEFAULT 0,
    status VARCHAR(50) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'scheduled', 'active', 'expired', 'archived')),
    published_at TIMESTAMPTZ,
    snapshot_content JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_announcements_rt_status ON announcements(rt_id, status);
CREATE INDEX IF NOT EXISTS idx_announcements_rt_schedule ON announcements(rt_id, scheduled_start, scheduled_end);

-- 4. Banners table
CREATE TABLE IF NOT EXISTS banners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rt_id UUID NOT NULL REFERENCES rts(id) ON DELETE CASCADE,
    source_type VARCHAR(50) NOT NULL CHECK (source_type IN ('announcement', 'activity', 'iuran_reminder', 'custom')),
    source_id UUID,
    template_id UUID REFERENCES banner_templates(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT,
    image_url TEXT,
    deep_link VARCHAR(255),
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    priority INT NOT NULL DEFAULT 1,
    display_order INT NOT NULL DEFAULT 0,
    is_pinned BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    snapshot_content JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_banners_rt_active ON banners(rt_id, is_active, is_pinned, priority);
CREATE INDEX IF NOT EXISTS idx_banners_rt_timing ON banners(rt_id, start_at, end_at);

-- 5. Permission Types
INSERT INTO permission_types (code, domain, operation) VALUES
    ('activities.read', 'community', 'read'),
    ('activities.create', 'community', 'create'),
    ('activities.update', 'community', 'update'),
    ('activities.publish', 'community', 'publish'),
    ('activities.cancel', 'community', 'cancel'),
    ('announcements.read', 'community', 'read'),
    ('announcements.create', 'community', 'create'),
    ('announcements.update', 'community', 'update'),
    ('announcements.publish', 'community', 'publish'),
    ('announcements.archive', 'community', 'archive'),
    ('banner_templates.read', 'community', 'read'),
    ('banner_templates.manage', 'community', 'manage'),
    ('organization.read', 'community', 'read'),
    ('organization.contact.read', 'community', 'read')
ON CONFLICT (code) DO NOTHING;

-- 6. Position Permissions:
-- Ketua RT gets full management permissions
INSERT INTO position_permissions (position, permission_id)
SELECT 'ketua', id FROM permission_types
WHERE code IN (
    'activities.read', 'activities.create', 'activities.update', 'activities.publish', 'activities.cancel',
    'announcements.read', 'announcements.create', 'announcements.update', 'announcements.publish', 'announcements.archive',
    'banner_templates.read', 'banner_templates.manage',
    'organization.read', 'organization.contact.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- Sekretaris gets organization contact read and general read
INSERT INTO position_permissions (position, permission_id)
SELECT 'sekretaris', id FROM permission_types
WHERE code IN (
    'activities.read', 'announcements.read', 'banner_templates.read', 'organization.read', 'organization.contact.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;

-- Wakil Ketua, Bendahara, Keamanan, Kebersihan, Sosial, Operasional, Anggota get read permissions
INSERT INTO position_permissions (position, permission_id)
SELECT p.pos, pt.id
FROM (VALUES
    ('wakil_ketua'),
    ('bendahara'),
    ('keamanan'),
    ('kebersihan'),
    ('sosial'),
    ('operasional'),
    ('anggota')
) AS p(pos)
CROSS JOIN permission_types pt
WHERE pt.code IN (
    'activities.read',
    'announcements.read',
    'banner_templates.read',
    'organization.read'
)
ON CONFLICT (position, permission_id) DO NOTHING;
