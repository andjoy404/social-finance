-- Migration 062 down: Remove notification.read permission and extra notification.manage grants

-- Remove notification.read grants for all positions
DELETE FROM position_permissions WHERE permission_id = (
    SELECT id FROM permission_types WHERE code = 'notification.read'
) AND position IN ('ketua', 'wakil_ketua', 'sekretaris', 'bendahara');

-- Remove notification.manage grants added in this migration (wakil_ketua, bendahara).
-- Ketua's grant from migration 052 is preserved.
DELETE FROM position_permissions WHERE permission_id = (
    SELECT id FROM permission_types WHERE code = 'notification.manage'
) AND position IN ('wakil_ketua', 'bendahara');

-- Remove notification.read permission type if unused.
DELETE FROM permission_types WHERE code = 'notification.read';
