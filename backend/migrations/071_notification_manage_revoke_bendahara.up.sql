-- 071_notification_manage_revoke_bendahara
-- Remove the stale `notification.manage` grant from the `bendahara`
-- position. The canonical development seed grants `notification.manage`
-- only to `ketua`; this migration idempotently brings existing databases
-- in line with that seed without touching any other position or
-- permission.
--
-- Bendahara keeps every other grant (warga.read/create/update/import/export,
-- finance.read/create/update/approve, notification.read, reports.read,
-- activities.read, announcements.read, banner_templates.read,
-- organization.read) and can still act as an ordinary authenticated RT
-- member for notification delivery (iuran reminder, payment verification
-- notifications, etc.). Only the administrative notification surface
-- (templates/schedules/management history/upload-image/test-push) is
-- withheld.
--
-- Deploy gate: do NOT run against production without a verified backup
-- and sign-off. Running this migration is a HOTFIX — it removes a
-- privileged capability. Confirm the target database first.

BEGIN;

DELETE FROM position_permissions
WHERE position = 'bendahara'
  AND permission_id = (SELECT id FROM permission_types WHERE code = 'notification.manage');

COMMIT;
