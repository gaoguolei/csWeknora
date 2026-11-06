-- Best-effort reverse: ownership moved in place, so the workspace that used to
-- own the promoted row cannot be recovered from the schema. The row keeps its
-- install rows and snapshot ledger and simply stops being the platform
-- default: it regains the display name the up migration stored in its
-- description.
DO $$ BEGIN RAISE NOTICE '[Migration 000128 down] Releasing the platform default sandbox'; END $$;

UPDATE tenant_sandbox_configs
SET name = CASE
        WHEN POSITION(' — ' IN COALESCE(description, '')) > 0
            THEN substring(description FROM 1 FOR POSITION(' — ' IN description) - 1)
        WHEN COALESCE(description, '') <> ''
            THEN description
        ELSE name
    END,
    description = NULL,
    updated_at = NOW()
WHERE tenant_id = 10000
  AND name = '__platform_sandbox__';
