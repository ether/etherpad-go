package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrator_Postgres_To_SQLite(t *testing.T) {
	pgDB, cleanup := startPostgres(t)
	defer cleanup()

	source, err := NewSQLDatabase(pgDB, DriverPostgres)
	assert.NoError(t, err)

	insertData(t, pgDB, insertKVPostgres)

	target := setupSQLiteTarget(t)
	startMigratorPipeline(t, source, target)
}
