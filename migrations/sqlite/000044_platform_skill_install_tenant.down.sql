-- Best-effort reverse: the pre-backfill tenant of each row is not recorded, so
-- the original workspace cannot be reconstructed. Leaving the rows on the
-- platform tenant is safe: it is where the platform-scoped reads, writes and
-- uninstalls expect the installs of a platform sandbox config to live.
SELECT 1;
