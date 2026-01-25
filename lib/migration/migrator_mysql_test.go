package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrator_MySQL_To_SQLite(t *testing.T) {
	sqlDB, cleanup := startMySQL(t)
	defer cleanup()

	source, err := NewSQLDatabase(sqlDB, DriverMySQL)
	assert.NoError(t, err)

	insertData(t, sqlDB, insertKV)

	target := setupSQLiteTarget(t)
	startMigratorPipeline(t, source, target)
}
