-- Description: Reparent skill installs and snapshot ledger rows that still live
-- under a workspace tenant but point at a platform sandbox config (the row owned
-- by the platform pseudo-tenant 10000). See the versioned 000129 pair for the
-- full rationale: installs recorded after migration 000043 through the catalog
-- path landed under the caller's workspace, so the platform-scoped reads, writes
-- and uninstalls (tenant 10000) could not see them.
-- Idempotent: rows already on 10000 are left untouched.
UPDATE tenant_skills
SET tenant_id = 10000,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id <> 10000
  AND sandbox_config_id IN (
      SELECT id FROM tenant_sandbox_configs WHERE tenant_id = 10000);

UPDATE tenant_skill_snapshots
SET tenant_id = 10000,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id <> 10000
  AND sandbox_config_id IN (
      SELECT id FROM tenant_sandbox_configs WHERE tenant_id = 10000);
