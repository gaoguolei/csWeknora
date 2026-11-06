-- Description: Promote one existing workspace sandbox config to the platform
-- default sandbox shared by every workspace (see the versioned 000128 pair for
-- the full rationale). The platform default is the tenant_sandbox_configs row
-- owned by the platform pseudo-tenant (10000) under the reserved name; agents
-- select it with the sentinel sandbox_config_id = 'platform'.
--
-- Candidate: the live, non-hidden config referenced by the most agents (ties
-- go to the earliest created). The promoted row's original display name moves
-- into description, and its install rows and snapshot ledger follow it to the
-- platform tenant. Agent rows are not rewritten: config lookups treat the
-- platform row as visible from every workspace.
-- Idempotent: an existing platform row (or no config at all) makes this a no-op.
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
          WHERE json_extract(a.config, '$.sandbox_config_id') = c.id
            AND a.deleted_at IS NULL) DESC,
         c.created_at ASC,
         c.id ASC
LIMIT 1;

UPDATE tenant_sandbox_configs
SET tenant_id = 10000,
    name = '__platform_sandbox__',
    description = CASE
        WHEN COALESCE(description, '') = ''
            THEN (SELECT from_name FROM platform_sandbox_candidate)
        ELSE (SELECT from_name FROM platform_sandbox_candidate) || ' — ' || description
    END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = (SELECT id FROM platform_sandbox_candidate);

-- The install rows belong to the config, not to the workspace that happened to
-- create them: one install on the platform default is the ready set of every
-- workspace, and the image snapshot is billed once.
UPDATE tenant_skills
SET tenant_id = 10000,
    updated_at = CURRENT_TIMESTAMP
WHERE sandbox_config_id = (SELECT id FROM platform_sandbox_candidate)
  AND tenant_id = (SELECT from_tenant FROM platform_sandbox_candidate);

UPDATE tenant_skill_snapshots
SET tenant_id = 10000,
    updated_at = CURRENT_TIMESTAMP
WHERE sandbox_config_id = (SELECT id FROM platform_sandbox_candidate)
  AND tenant_id = (SELECT from_tenant FROM platform_sandbox_candidate);

DROP TABLE IF EXISTS platform_sandbox_candidate;
