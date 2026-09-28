-- Migration 011: Remove RT organizational positions and permission tables
-- NOTE: This migration ONLY removes position-related infrastructure.
-- It must NOT modify/remove the existing bendahara role or any other
-- tenant role in user_rt_memberships.role.

-- =====================================================================
-- Drop position-permission mapping
-- =====================================================================

DROP TABLE IF EXISTS position_permissions;

-- =====================================================================
-- Drop permission types
-- =====================================================================

DROP TABLE IF EXISTS permission_types;

-- =====================================================================
-- Remove jabatan column from user_rt_memberships
-- This only removes the jabatan column. The role column is NOT
-- touched — existing bendahara, pengurus, warga roles remain intact.
-- =====================================================================

ALTER TABLE user_rt_memberships DROP COLUMN IF EXISTS jabatan;
