-- Rollback: revoke activities management grants added to Sekretaris by 076.
-- Does not remove permission_types or grants held by other positions.
BEGIN;

DELETE FROM position_permissions
WHERE position = 'sekretaris'
  AND permission_id IN (
    SELECT id FROM permission_types
    WHERE code IN (
      'activities.create',
      'activities.update',
      'activities.publish',
      'activities.cancel',
      'activities.delete'
    )
  );

COMMIT;
