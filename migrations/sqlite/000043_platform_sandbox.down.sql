-- Best-effort reverse: ownership moved in place, so the workspace that used to
-- own the promoted row cannot be recovered from the schema. The row keeps its
-- install rows and snapshot ledger and simply stops being the platform
-- default: it regains the display name the up migration stored in its
-- description.
UPDATE tenant_sandbox_configs
SET name = CASE
        WHEN instr(COALESCE(description, ''), ' — ') > 0
            THEN substr(description, 1, instr(description, ' — ') - 1)
        WHEN COALESCE(description, '') <> ''
            THEN description
        ELSE name
    END,
    description = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 10000
  AND name = '__platform_sandbox__';
