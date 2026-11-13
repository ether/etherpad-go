package db

import (
	"database/sql"
	"testing"

	"github.com/ether/etherpad-go/lib/db/migrations"
	"github.com/ether/etherpad-go/lib/test/testdb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 13 on MySQL: ids compare case-sensitively afterwards, and a
// revision stored under another case of its pad's id (which matched
// before) is kept and found under the pad's id.
func TestMySQLBinaryIDsMigration(t *testing.T) {
	if testdb.SkipSQLServers() {
		t.Skip("EP_TEST_SKIP_SQL_SERVERS")
	}
	server := testdb.Get(t, testdb.MySQL)
	name := server.NewDatabase(t)
	conn, err := sql.Open(server.DriverName(), server.DSN(name))
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, migrations.NewMigrationManagerUpTo(conn, migrations.DialectMySQL, 12).Run())
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := conn.Exec(q, args...)
		require.NoError(t, err)
	}
	exec(`INSERT INTO pad (id, head) VALUES ('Foo', 1)`)
	exec(`INSERT INTO padRev (id, rev, changeset) VALUES ('Foo', 0, 'cs0')`)
	exec(`INSERT INTO padRev (id, rev, changeset) VALUES ('foo', 1, 'cs1')`)
	exec(`INSERT INTO padChat (padId, padHead, chatText) VALUES ('FOO', 0, 'hi')`)
	var n int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM pad WHERE id = 'foo'`).Scan(&n))
	require.Equal(t, 1, n, "before: MySQL ignores the case of ids")

	require.NoError(t, migrations.NewMigrationManager(conn, migrations.DialectMySQL).Run())

	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM pad WHERE id = 'foo'`).Scan(&n))
	assert.Equal(t, 0, n, "after: ids compare as they are")
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM padRev WHERE id = 'Foo'`).Scan(&n))
	assert.Equal(t, 2, n, "both revisions belong to Foo")
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM padChat WHERE padId = 'Foo'`).Scan(&n))
	assert.Equal(t, 1, n, "the chat message belongs to Foo")

	exec(`INSERT INTO pad (id, head) VALUES ('foo', 0)`)
	exec(`DELETE FROM pad WHERE id = 'Foo'`)
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM padRev`).Scan(&n))
	assert.Equal(t, 0, n, "the foreign keys are back: deleting the pad deletes its revisions")
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM pad WHERE id = 'foo'`).Scan(&n))
	assert.Equal(t, 1, n)

	// Running it again changes nothing.
	m := migrations.NewMigrationManager(conn, migrations.DialectMySQL)
	require.NoError(t, m.Run())
}
