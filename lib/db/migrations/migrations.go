package migrations

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a single database migration
type Migration struct {
	Version     int
	Description string
	Up          func(db *sql.DB, dialect Dialect) error
}

// Dialect represents the SQL dialect for different databases
type Dialect int

const (
	DialectSQLite Dialect = iota
	DialectPostgres
	DialectMySQL
)

// MigrationManager handles database migrations
type MigrationManager struct {
	db         *sql.DB
	dialect    Dialect
	migrations []Migration
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB, dialect Dialect) *MigrationManager {
	return &MigrationManager{
		db:         db,
		dialect:    dialect,
		migrations: GetMigrations(),
	}
}

// Run executes all pending migrations
func (m *MigrationManager) Run() error {
	// Create migrations table if not exists
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get current version
	currentVersion, err := m.getCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Sort migrations by version
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	// Execute pending migrations
	for _, migration := range m.migrations {
		if migration.Version > currentVersion {
			fmt.Printf("Running migration %d: %s\n", migration.Version, migration.Description)
			if err := migration.Up(m.db, m.dialect); err != nil {
				return fmt.Errorf("migration %d failed: %w", migration.Version, err)
			}
			if err := m.setVersion(migration.Version, migration.Description); err != nil {
				return fmt.Errorf("failed to update migration version: %w", err)
			}
		}
	}

	return nil
}

// createMigrationsTable creates the schema_migrations table
func (m *MigrationManager) createMigrationsTable() error {
	var query string
	switch m.dialect {
	case DialectMySQL:
		query = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			description VARCHAR(255),
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	default:
		query = `CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
	_, err := m.db.Exec(query)
	return err
}

// getCurrentVersion returns the current migration version
func (m *MigrationManager) getCurrentVersion() (int, error) {
	var version int
	row := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	err := row.Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// setVersion inserts a new migration record
func (m *MigrationManager) setVersion(version int, description string) error {
	var query string
	switch m.dialect {
	case DialectMySQL:
		query = "INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)"
	case DialectPostgres:
		query = "INSERT INTO schema_migrations (version, description, applied_at) VALUES ($1, $2, $3)"
	default:
		query = "INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)"
	}
	_, err := m.db.Exec(query, version, description, time.Now())
	return err
}

// GetCurrentVersion returns the current migration version (public method)
func (m *MigrationManager) GetCurrentVersion() (int, error) {
	return m.getCurrentVersion()
}
