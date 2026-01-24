package migration

import (
	"database/sql"
	"fmt"
	"strings"
)

func NewDB(dsn string, dbType string) (*SQLDatabase, error) {
	if strings.Contains(dbType, "postgres") {
		return NewPostgresDatabase(dsn)
	} else if strings.Contains(dbType, "mysql") {
		return NewMySQLDatabase(dsn)
	} else if strings.HasPrefix(dsn, "sqlite") || strings.HasSuffix(dsn, ".db") || strings.HasSuffix(dsn, ".sqlite") {
		return NewSQLiteDatabase(dsn)
	}

	return nil, fmt.Errorf("unsupported database type in DSN: %s", dsn)
}

func NewSQLiteDatabase(dsn string) (*SQLDatabase, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	return NewSQLDatabase(db, DriverSQLite), nil
}

// NewPostgresDatabase creates a PostgreSQL-backed database
func NewPostgresDatabase(dsn string) (*SQLDatabase, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	return NewSQLDatabase(db, DriverPostgres), nil
}

// NewMySQLDatabase creates a MySQL-backed database
func NewMySQLDatabase(dsn string) (*SQLDatabase, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql database: %w", err)
	}

	return NewSQLDatabase(db, DriverMySQL), nil
}
