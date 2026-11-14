package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/ether/etherpad-go/lib/db/migrations"
	"github.com/ether/etherpad-go/lib/test/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// Migration 16 makes the author mapper unique. Duplicates from before keep
// the mapper on the oldest author id; authors without a mapper are fine.
func TestUniqueAuthorMapperMigration(t *testing.T) {
	type target struct {
		name    string
		dialect migrations.Dialect
		open    func(t *testing.T) *sql.DB
	}
	targets := []target{
		{"SQLite", migrations.DialectSQLite, func(t *testing.T) *sql.DB {
			conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "m.db"))
			require.NoError(t, err)
			return conn
		}},
		{"Postgres", migrations.DialectPostgres, func(t *testing.T) *sql.DB {
			srv := testdb.Get(t, testdb.Postgres)
			conn, err := sql.Open(srv.DriverName(), srv.DSN(srv.NewDatabase(t)))
			require.NoError(t, err)
			return conn
		}},
		{"MySQL", migrations.DialectMySQL, func(t *testing.T) *sql.DB {
			srv := testdb.Get(t, testdb.MySQL)
			conn, err := sql.Open(srv.DriverName(), srv.DSN(srv.NewDatabase(t)))
			require.NoError(t, err)
			return conn
		}},
	}
	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			conn := tc.open(t)
			defer conn.Close()
			require.NoError(t, migrations.NewMigrationManagerUpTo(conn, tc.dialect, 15).Run())
			ph := "?"
			if tc.dialect == migrations.DialectPostgres {
				ph = "$1"
			}
			for _, id := range []string{"a.2", "a.1", "a.3", "a.4", "a.5"} {
				_, err := conn.Exec(`INSERT INTO globalAuthor (id, colorId) VALUES (`+ph+`, '1')`, id)
				require.NoError(t, err)
			}
			for _, row := range [][2]string{{"a.2", "lms-1"}, {"a.1", "lms-1"}, {"a.3", "lms-1"}, {"a.4", "lms-2"}} {
				q := `UPDATE globalAuthor SET mapper = ? WHERE id = ?`
				if tc.dialect == migrations.DialectPostgres {
					q = `UPDATE globalAuthor SET mapper = $1 WHERE id = $2`
				}
				_, err := conn.Exec(q, row[1], row[0])
				require.NoError(t, err)
			}

			require.NoError(t, migrations.NewMigrationManager(conn, tc.dialect).Run())

			mappers := map[string]sql.NullString{}
			rows, err := conn.Query(`SELECT id, mapper FROM globalAuthor`)
			require.NoError(t, err)
			for rows.Next() {
				var id string
				var m sql.NullString
				require.NoError(t, rows.Scan(&id, &m))
				mappers[id] = m
			}
			require.NoError(t, rows.Err())
			assert.Equal(t, "lms-1", mappers["a.1"].String, "the oldest id keeps the mapper")
			assert.False(t, mappers["a.2"].Valid)
			assert.False(t, mappers["a.3"].Valid)
			assert.Equal(t, "lms-2", mappers["a.4"].String)
			assert.False(t, mappers["a.5"].Valid)

			q := `UPDATE globalAuthor SET mapper = 'lms-1' WHERE id = 'a.5'`
			_, err = conn.Exec(q)
			assert.Error(t, err, "the mapper is unique now")
			// Running again changes nothing.
			require.NoError(t, migrations.NewMigrationManager(conn, tc.dialect).Run())
		})
	}
}
