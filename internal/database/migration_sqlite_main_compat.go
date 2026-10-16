package database

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/golang-migrate/migrate/v4/source"
)

// main used SQLite version 33 for operations administrators; dev used it for
// workspace tests. Migration 40 reconciles both histories without resetting
// schema_migrations or losing an existing operations-admin grant.
type sqliteMainCompatibilitySource struct {
	source.Driver
	hasOperationsAdmin bool
}

func newSQLiteMainCompatibilitySource(path string, db *sql.DB) (source.Driver, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'is_operations_admin'").Scan(&count); err != nil {
		return nil, fmt.Errorf("inspect operations-admin schema: %w", err)
	}
	driver, err := source.Open(path)
	if err != nil {
		return nil, err
	}
	return &sqliteMainCompatibilitySource{Driver: driver, hasOperationsAdmin: count > 0}, nil
}

func (s *sqliteMainCompatibilitySource) ReadUp(version uint) (io.ReadCloser, string, error) {
	reader, name, err := s.Driver.ReadUp(version)
	if err != nil || version != 40 || !s.hasOperationsAdmin {
		return reader, name, err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, name, err
	}
	const column = "ALTER TABLE users ADD COLUMN is_operations_admin BOOLEAN NOT NULL DEFAULT 0;"
	if strings.Count(string(data), column) != 1 {
		return nil, name, fmt.Errorf("unexpected operations-admin migration; existing grants were not modified")
	}
	return io.NopCloser(strings.NewReader(strings.Replace(string(data), column, "", 1))), name, nil
}
