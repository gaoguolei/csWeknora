-- Description: Skill definitions move from a per-workspace catalog to one
-- platform-wide catalog shared by every workspace. Installations stay in
-- tenant_skills, which is where the sandbox they run on lives; tenant_id on
-- tenant_skill_catalog now names the workspace whose storage holds the bundle,
-- not a visibility scope.
DO $$ BEGIN RAISE NOTICE '[Migration 000127] Making tenant_skill_catalog platform-wide'; END $$;

-- Two workspaces may already own a definition with the same name. Keep exactly
-- one per name: the row that still has an archive, then the most recently
-- written one. Installs of the losing rows follow the winner first, so no
-- sandbox loses the definition its files are read from.
WITH ranked AS (
    SELECT id,
           first_value(id) OVER (
               PARTITION BY name
               ORDER BY CASE WHEN bundle_ref IS NULL OR bundle_ref = '' THEN 1 ELSE 0 END,
                        updated_at DESC,
                        created_at DESC,
                        id
           ) AS winner_id
    FROM tenant_skill_catalog
    WHERE deleted_at IS NULL
)
UPDATE tenant_skills AS s
SET catalog_id = r.winner_id
FROM ranked AS r
WHERE s.catalog_id = r.id
  AND r.id <> r.winner_id;

WITH ranked AS (
    SELECT id,
           first_value(id) OVER (
               PARTITION BY name
               ORDER BY CASE WHEN bundle_ref IS NULL OR bundle_ref = '' THEN 1 ELSE 0 END,
                        updated_at DESC,
                        created_at DESC,
                        id
           ) AS winner_id
    FROM tenant_skill_catalog
    WHERE deleted_at IS NULL
)
UPDATE tenant_skill_catalog AS c
SET deleted_at = NOW(),
    updated_at = NOW()
FROM ranked AS r
WHERE c.id = r.id
  AND r.id <> r.winner_id;

COMMENT ON COLUMN tenant_skill_catalog.tenant_id IS
    'Workspace whose storage holds this definition''s bundle. Not a visibility scope: every workspace sees every definition.';

DROP INDEX IF EXISTS uq_tenant_skill_catalog_name;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_skill_catalog_name
ON tenant_skill_catalog (name) WHERE deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000127] Skill catalog is platform-wide'; END $$;
