# Fork migration numbering

This repository keeps the deployed custom migration IDs unchanged:

- PostgreSQL `000111`–`000115`: coding projects, folders, embeddings repair and workspace test runs.
- SQLite `000031`–`000033`: coding projects, folders and workspace test runs.

Starting with upstream v0.8.3, incoming official migrations are renumbered to avoid those historical IDs. PostgreSQL upstream IDs >= 111 use an offset of +5; SQLite upstream IDs >= 31 use an offset of +3. Keep the SQL and order of each upstream migration unchanged.

| Engine | Official v0.8.3 range | Fork range |
| --- | --- | --- |
| PostgreSQL | 111–118 | 116–123 |
| SQLite | 31–36 | 34–39 |

An existing fork database at PostgreSQL 115 or SQLite 33 therefore runs all newly added release migrations. A fresh database runs both the custom schema and the release schema in order. Never rename the historical custom migrations, force an existing database back to an earlier version, or reset its migration table.

For later upstream merges, apply the same offsets to newly added upstream migrations and adjust exact filename references in tests. Check that each engine has exactly one up/down pair per migration ID. Test both a new database and an upgrade from the pre-0.8.3 fork schema before deployment.

## Cross-repository main/dev reconciliation

Older main used PostgreSQL 114 and SQLite 33 for operations-admin grants; dev used those IDs for workspace test reports. Do not force either database backwards. PostgreSQL 115 tolerates the missing test table on old main; 124 backfills that table and control columns, then 125 adds operations-admin support idempotently. SQLite 40 creates missing test storage and adds the role; the application source adapter skips the duplicate role-column ALTER for an existing main database, preserving all grants. Both branches are covered by migration tests. Rollback keeps these records and role grants.

The earlier v0.8.3 upstream mappings remain unchanged. Future upstream PostgreSQL IDs >=119 need an offset of +7 (first new ID 126); SQLite IDs >=37 need +4 (first new ID 41). When importing future dev migrations already carrying +5/+3, move only the new files beyond the reserved main IDs; never renumber an already-applied main migration.
