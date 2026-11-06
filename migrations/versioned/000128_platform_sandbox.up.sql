-- Description: Promote one existing workspace sandbox config to the platform
-- default sandbox shared by every workspace.
--
-- The platform default is the single tenant_sandbox_configs row owned by the
-- platform pseudo-tenant (10000) under the reserved name; agents select it
-- with the sentinel sandbox_config_id = 'platform'. Users who already
-- configured a sandbox expect it to keep working, so the most referenced
-- workspace config is promoted in place instead of leaving everyone on the
-- deployment default:
--   1. candidate = live, non-hidden config referenced by the most agents
--      (ties go to the earliest created);
--   2. the row moves to the platform tenant under the reserved name, and its
--      original display name is kept in description;
--   3. its install rows and snapshot ledger move with it, so the ready set and
--      the billable image history stay attached to the same config;
--   4. agents that reference the config ID are NOT rewritten — GetByID treats
--      the platform row as visible from every workspace, so they resolve to
--      the same backend they always used.
-- Idempotent: a second run finds the platform row already present and does
-- nothing, and an instance with no configurable sandbox is skipped.
DO $$ BEGIN RAISE NOTICE '[Migration 000128] Promoting workspace sandbox config to the platform default'; END $$;

DROP TABLE IF EXISTS platform_sandbox_candidate;

CREATE TEMP TABLE platform_sandbox_candidate AS
SELECT c.id            AS id,
       c.tenant_id     AS from_tenant,
       c.name          AS from_name
FROM tenant_sandbox_configs c
WHERE c.deleted_at IS NULL
  AND c.name <> '__workspace_scripts_policy__'
  AND NOT EXISTS (
      SELECT 1 FROM tenant_sandbox_configs p
      WHERE p.tenant_id = 10000
        AND p.name = '__platform_sandbox__'
        AND p.deleted_at IS NULL)
ORDER BY (
          SELECT count(*) FROM custom_agents a
          WHERE a.config->>'sandbox_config_id' = c.id
            AND a.deleted_at IS NULL) DESC,
         c.created_at ASC,
         c.id ASC
LIMIT 1;

UPDATE tenant_sandbox_configs c
SET tenant_id = 10000,
    name = '__platform_sandbox__',
    description = CASE
        WHEN COALESCE(c.description, '') = '' THEN t.from_name
        ELSE t.from_name || ' — ' || c.description
    END,
    updated_at = NOW()
FROM platform_sandbox_candidate t
WHERE c.id = t.id;

-- The install rows belong to the config, not to the workspace that happened to
-- create them: one install on the platform default is the ready set of every
-- workspace, and the image snapshot is billed once.
UPDATE tenant_skills s
SET tenant_id = 10000,
    updated_at = NOW()
FROM platform_sandbox_candidate t
WHERE s.sandbox_config_id = t.id
  AND s.tenant_id = t.from_tenant;

UPDATE tenant_skill_snapshots sn
SET tenant_id = 10000,
    updated_at = NOW()
FROM platform_sandbox_candidate t
WHERE sn.sandbox_config_id = t.id
  AND sn.tenant_id = t.from_tenant;

DROP TABLE IF EXISTS platform_sandbox_candidate;
