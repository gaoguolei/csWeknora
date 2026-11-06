-- Description: Reparent skill installs and snapshot ledger rows that still live
-- under a workspace tenant but point at a platform sandbox config (the row owned
-- by the platform pseudo-tenant 10000).
--
-- Migration 000128 only moved the installs present when it promoted a config.
-- Installs made afterwards through the catalog path were recorded under the
-- caller's workspace instead of the platform tenant, so the platform-scoped
-- reads, writes and uninstalls (which resolve a platform config to tenant 10000)
-- could not see them: the sandbox manage drawer rendered empty and offered no
-- uninstall, and the install was invisible to every other workspace.
--
-- The one install on the shared default is the ready set of every workspace, so
-- the rows belong to the platform tenant regardless of which workspace created
-- them. Idempotent: rows already on 10000 are left untouched.
UPDATE tenant_skills
SET tenant_id = 10000,
    updated_at = NOW()
WHERE tenant_id <> 10000
  AND sandbox_config_id IN (
      SELECT id FROM tenant_sandbox_configs WHERE tenant_id = 10000);

UPDATE tenant_skill_snapshots
SET tenant_id = 10000,
    updated_at = NOW()
WHERE tenant_id <> 10000
  AND sandbox_config_id IN (
      SELECT id FROM tenant_sandbox_configs WHERE tenant_id = 10000);
