-- Skill definitions move from a per-workspace catalog to one platform-wide
-- catalog shared by every workspace. Installations stay in tenant_skills;
-- tenant_id on tenant_skill_catalog now names the workspace whose storage
-- holds the bundle, not a visibility scope.

-- Two workspaces may already own a definition with the same name. Keep exactly
-- one per name: the row that still has an archive, then the most recently
-- written one. Installs of the losing rows follow the winner first, so no
-- sandbox loses the definition its files are read from.
DROP TABLE IF EXISTS skill_catalog_winners;

CREATE TEMP TABLE skill_catalog_winners AS
SELECT id,
       first_value(id) OVER (
           PARTITION BY name
           ORDER BY CASE WHEN bundle_ref IS NULL OR bundle_ref = '' THEN 1 ELSE 0 END,
                    updated_at DESC,
                    created_at DESC,
                    id
       ) AS winner_id
FROM tenant_skill_catalog
WHERE deleted_at IS NULL;

UPDATE tenant_skills
SET catalog_id = (
    SELECT w.winner_id FROM skill_catalog_winners AS w WHERE w.id = tenant_skills.catalog_id
)
WHERE catalog_id IN (SELECT id FROM skill_catalog_winners WHERE id <> winner_id);

UPDATE tenant_skill_catalog
SET deleted_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id IN (SELECT id FROM skill_catalog_winners WHERE id <> winner_id);

DROP INDEX IF EXISTS uq_tenant_skill_catalog_name;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_skill_catalog_name
ON tenant_skill_catalog (name) WHERE deleted_at IS NULL;

DROP TABLE IF EXISTS skill_catalog_winners;
