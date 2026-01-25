package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	mysql2 "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testDbName = "test_db"
	testDbUser = "test_user"
	testDbPass = "test_password"
)

// TestContainerConfig holds container connection details
type TestContainerConfig struct {
	Container testcontainers.Container
	Host      string
	Port      string
}

// testEnv holds shared test containers
type testEnv struct {
	postgres     *TestContainerConfig
	postgresOnce sync.Once
	postgresErr  error

	mysql     *TestContainerConfig
	mysqlOnce sync.Once
	mysqlErr  error
}

var sharedEnv = &testEnv{}

func TestMain(m *testing.M) {
	code := m.Run()

	// Cleanup containers after all tests
	ctx := context.Background()
	if sharedEnv.postgres != nil && sharedEnv.postgres.Container != nil {
		_ = sharedEnv.postgres.Container.Terminate(ctx)
	}
	if sharedEnv.mysql != nil && sharedEnv.mysql.Container != nil {
		_ = sharedEnv.mysql.Container.Terminate(ctx)
	}

	os.Exit(code)
}

func setupPostgresContainer(t *testing.T) *TestContainerConfig {
	t.Helper()

	sharedEnv.postgresOnce.Do(func() {
		ctx := context.Background()
		container, err := testcontainers.Run(
			ctx, "postgres:alpine",
			testcontainers.WithExposedPorts("5432/tcp"),
			testcontainers.WithWaitStrategy(
				wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
					return fmt.Sprintf(
						"postgres://%s:%s@%s:%s/%s?sslmode=disable",
						testDbUser, testDbPass, host, port.Port(), testDbName,
					)
				}).WithStartupTimeout(30*time.Second).WithQuery("SELECT 1"),
			),
			testcontainers.WithEnv(map[string]string{
				"POSTGRES_PASSWORD": testDbPass,
				"POSTGRES_USER":     testDbUser,
				"POSTGRES_DB":       testDbName,
			}),
		)
		if err != nil {
			sharedEnv.postgresErr = err
			return
		}

		port, err := container.MappedPort(ctx, "5432")
		if err != nil {
			sharedEnv.postgresErr = err
			return
		}

		host, err := container.Host(ctx)
		if err != nil {
			sharedEnv.postgresErr = err
			return
		}

		sharedEnv.postgres = &TestContainerConfig{
			Container: container,
			Host:      host,
			Port:      port.Port(),
		}
	})

	require.NoError(t, sharedEnv.postgresErr)
	return sharedEnv.postgres
}

func setupMySQLContainer(t *testing.T) *TestContainerConfig {
	t.Helper()

	sharedEnv.mysqlOnce.Do(func() {
		ctx := context.Background()
		container, err := testcontainers.Run(
			ctx, "mysql:9.6",
			testcontainers.WithExposedPorts("3306/tcp"),
			testcontainers.WithEnv(map[string]string{
				"MYSQL_PASSWORD":      testDbPass,
				"MYSQL_ROOT_PASSWORD": testDbPass,
				"MYSQL_USER":          testDbUser,
				"MYSQL_DATABASE":      testDbName,
			}),
		)
		if err != nil {
			sharedEnv.mysqlErr = err
			return
		}

		port, err := container.MappedPort(ctx, "3306")
		if err != nil {
			sharedEnv.mysqlErr = err
			return
		}

		host, err := container.Host(ctx)
		if err != nil {
			sharedEnv.mysqlErr = err
			return
		}

		// Wait for MySQL to be ready
		cfg := mysql2.NewConfig()
		cfg.User = testDbUser
		cfg.Passwd = testDbPass
		cfg.Net = "tcp"
		cfg.Addr = fmt.Sprintf("%s:%s", host, port.Port())
		cfg.DBName = testDbName
		cfg.ParseTime = true
		dsn := cfg.FormatDSN()

		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			db, err := sql.Open("mysql", dsn)
			if err == nil {
				if err = db.Ping(); err == nil {
					db.Close()
					break
				}
				db.Close()
			}
			time.Sleep(time.Second)
		}

		sharedEnv.mysql = &TestContainerConfig{
			Container: container,
			Host:      host,
			Port:      port.Port(),
		}
	})

	require.NoError(t, sharedEnv.mysqlErr)
	return sharedEnv.mysql
}

// DBTestCase represents a test case for a specific database driver
type DBTestCase struct {
	Name   string
	Driver DriverType
	Setup  func(t *testing.T) *sql.DB
}

func getTestCases(t *testing.T) []DBTestCase {
	t.Helper()

	return []DBTestCase{
		{
			Name:   "SQLite",
			Driver: DriverSQLite,
			Setup: func(t *testing.T) *sql.DB {
				db, err := sql.Open("sqlite", ":memory:")
				require.NoError(t, err)
				t.Cleanup(func() { db.Close() })
				return db
			},
		},
		{
			Name:   "Postgres",
			Driver: DriverPostgres,
			Setup: func(t *testing.T) *sql.DB {
				cfg := setupPostgresContainer(t)
				dsn := fmt.Sprintf(
					"postgres://%s:%s@%s:%s/%s?sslmode=disable",
					testDbUser, testDbPass, cfg.Host, cfg.Port, testDbName,
				)
				db, err := sql.Open("postgres", dsn)
				require.NoError(t, err)
				t.Cleanup(func() { db.Close() })
				return db
			},
		},
		{
			Name:   "MySQL",
			Driver: DriverMySQL,
			Setup: func(t *testing.T) *sql.DB {
				cfg := setupMySQLContainer(t)
				mysqlCfg := mysql2.NewConfig()
				mysqlCfg.User = testDbUser
				mysqlCfg.Passwd = testDbPass
				mysqlCfg.Net = "tcp"
				mysqlCfg.Addr = fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
				mysqlCfg.DBName = testDbName
				mysqlCfg.ParseTime = true
				db, err := sql.Open("mysql", mysqlCfg.FormatDSN())
				require.NoError(t, err)
				t.Cleanup(func() { db.Close() })
				return db
			},
		},
	}
}

// resetStoreTable drops and recreates the store table for a clean state
func resetStoreTable(t *testing.T, db *sql.DB, driver DriverType) {
	t.Helper()

	var createSQLs []string
	switch driver {
	case DriverPostgres:
		createSQLs = []string{`
			DROP TABLE IF EXISTS store;
			CREATE TABLE store (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);`}
	case DriverMySQL:
		createSQLs = []string{
			"DROP TABLE IF EXISTS store;",
			`
    CREATE TABLE store (
        ` + "`key`" + ` VARCHAR(100) PRIMARY KEY,
        ` + "`value`" + ` TEXT NOT NULL
    );
    `,
		}
	case DriverSQLite:
		createSQLs = []string{`
			DROP TABLE IF EXISTS store;
			CREATE TABLE store (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);`}
	}

	for _, createSQL := range createSQLs {
		_, err := db.Exec(createSQL)
		require.NoError(t, err)
	}
}

// insertStoreValue inserts a key-value pair into the store
func insertStoreValue(t *testing.T, db *sql.DB, driver DriverType, key string, value any) {
	t.Helper()

	jsonValue, err := json.Marshal(value)
	require.NoError(t, err)

	var insertSQL string
	switch driver {
	case DriverPostgres:
		insertSQL = "INSERT INTO store (key, value) VALUES ($1, $2)"
	case DriverMySQL:
		insertSQL = "INSERT INTO store (`key`, `value`) VALUES (?, ?)"
	case DriverSQLite:
		insertSQL = "INSERT INTO store (key, value) VALUES (?, ?)"
	}

	_, err = db.Exec(insertSQL, key, string(jsonValue))
	require.NoError(t, err)
}

// =============================================================================
// Test: GetNextPads
// =============================================================================

func TestSQLDatabase_GetNextPads(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			// Test data: Pad objects
			pads := []struct {
				key   string
				value map[string]any
			}{
				{
					key: "pad:alpha",
					value: map[string]any{
						"atext":        map[string]any{"text": "Hello\n", "attribs": "|1+6"},
						"pool":         map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
						"head":         0,
						"chatHead":     -1,
						"publicStatus": false,
					},
				},
				{
					key: "pad:beta",
					value: map[string]any{
						"atext":        map[string]any{"text": "World\n", "attribs": "|1+6"},
						"pool":         map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
						"head":         5,
						"chatHead":     2,
						"publicStatus": true,
					},
				},
				{
					key: "pad:gamma",
					value: map[string]any{
						"atext":        map[string]any{"text": "Test\n", "attribs": "|1+5"},
						"pool":         map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
						"head":         10,
						"chatHead":     -1,
						"publicStatus": false,
					},
				},
				{
					key:   "pad:alpha:revs:0",
					value: map[string]any{"changeset": "Z:0>6|1+6$Hello\n"},
				},
				{
					key:   "pad:beta:revs:0",
					value: map[string]any{"changeset": "Z:0>6|1+6$World\n"},
				},
			}

			for _, p := range pads {
				insertStoreValue(t, db, tc.Driver, p.key, p.value)
			}

			t.Run("GetAllPads", func(t *testing.T) {
				result, err := sqlDB.GetNextPads("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				// Should be sorted by PadId
				assert.Equal(t, "alpha", result[0].PadId)
				assert.Equal(t, "beta", result[1].PadId)
				assert.Equal(t, "gamma", result[2].PadId)
			})

			t.Run("EmptyResult", func(t *testing.T) {
				resetStoreTable(t, db, tc.Driver)
				result, err := sqlDB.GetNextPads("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 0)
			})
		})
	}
}

// =============================================================================
// Test: GetPadRevisions
// =============================================================================

func TestSQLDatabase_GetPadRevisions(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			// Insert revision data
			revisions := []struct {
				key   string
				value map[string]any
			}{
				{
					key: "pad:testpad:revs:0",
					value: map[string]any{
						"changeset": "Z:0>6|1+6$Hello\n",
						"meta":      map[string]any{"author": "", "timestamp": 1700000000000},
					},
				},
				{
					key: "pad:testpad:revs:1",
					value: map[string]any{
						"changeset": "Z:6>6|1=6|1+6$ World\n",
						"meta":      map[string]any{"author": "a.author1", "timestamp": 1700000001000},
					},
				},
				{
					key: "pad:testpad:revs:2",
					value: map[string]any{
						"changeset": "Z:c>1|2=c+1$!",
						"meta":      map[string]any{"author": "a.author2", "timestamp": 1700000002000},
					},
				},
				{
					key: "pad:testpad:revs:10",
					value: map[string]any{
						"changeset": "Z:d>5|2=d+5$test!",
						"meta":      map[string]any{"author": "a.author1", "timestamp": 1700000010000},
					},
				},
				// Different pad - should be ignored
				{
					key: "pad:otherpad:revs:0",
					value: map[string]any{
						"changeset": "Z:0>5|1+5$Other",
						"meta":      map[string]any{"author": "", "timestamp": 1700000000000},
					},
				},
			}

			for _, r := range revisions {
				insertStoreValue(t, db, tc.Driver, r.key, r.value)
			}

			t.Run("GetAllRevisions", func(t *testing.T) {
				result, err := sqlDB.GetPadRevisions("testpad", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 4)

				// Should be sorted numerically
				assert.Equal(t, 0, result[0].RevNum)
				assert.Equal(t, 1, result[1].RevNum)
				assert.Equal(t, 2, result[2].RevNum)
				assert.Equal(t, 10, result[3].RevNum)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetPadRevisions("testpad", -1, 2)
				require.NoError(t, err)
				assert.Len(t, result, 2)
				assert.Equal(t, 0, result[0].RevNum)
				assert.Equal(t, 1, result[1].RevNum)

				// Continue from revision 1
				result, err = sqlDB.GetPadRevisions("testpad", 1, 2)
				require.NoError(t, err)
				assert.Len(t, result, 2)
				assert.Equal(t, 2, result[0].RevNum)
				assert.Equal(t, 10, result[1].RevNum)
			})

			t.Run("NumericOrdering", func(t *testing.T) {
				// Add revisions 3-9 to test numeric sorting
				for i := 3; i < 10; i++ {
					insertStoreValue(t, db, tc.Driver, fmt.Sprintf("pad:testpad:revs:%d", i), map[string]any{
						"changeset": "Z:1>1+1$x",
						"meta":      map[string]any{"author": "", "timestamp": 1700000000000 + int64(i*1000)},
					})
				}

				result, err := sqlDB.GetPadRevisions("testpad", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 11)

				// Verify numeric order
				for i, rev := range result {
					assert.Equal(t, i, rev.RevNum)
				}
			})

			t.Run("NonExistentPad", func(t *testing.T) {
				result, err := sqlDB.GetPadRevisions("nonexistent", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 0)
			})
		})
	}
}

// =============================================================================
// Test: GetNextAuthors
// =============================================================================

func TestSQLDatabase_GetNextAuthors(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			authors := []struct {
				key   string
				value map[string]any
			}{
				{
					key: "globalAuthor:a.abc123",
					value: map[string]any{
						"colorId": 10,
						"name":    "Alice",
						"padIDs":  map[string]int{"pad1": 1, "pad2": 1},
					},
				},
				{
					key: "globalAuthor:a.def456",
					value: map[string]any{
						"colorId": 12,
						"name":    "Bob",
						"padIDs":  map[string]int{"pad1": 1},
					},
				},
				{
					key: "globalAuthor:a.ghi789",
					value: map[string]any{
						"colorId": 13,
						"name":    "",
						"padIDs":  map[string]int{},
					},
				},
			}

			for _, a := range authors {
				insertStoreValue(t, db, tc.Driver, a.key, a.value)
			}

			t.Run("GetAllAuthors", func(t *testing.T) {
				result, err := sqlDB.GetNextAuthors("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				// Should be sorted by ID
				assert.Equal(t, "a.abc123", result[0].Id)
				assert.Equal(t, "a.def456", result[1].Id)
				assert.Equal(t, "a.ghi789", result[2].Id)
			})

			t.Run("AuthorFields", func(t *testing.T) {
				result, err := sqlDB.GetNextAuthors("", 1)
				require.NoError(t, err)
				require.Len(t, result, 1)

				assert.Equal(t, "a.abc123", result[0].Id)
				assert.Equal(t, 10, result[0].ColorId)
				assert.Equal(t, "Alice", result[0].Name)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextAuthors("", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)

				result, err = sqlDB.GetNextAuthors("a.abc123", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "a.def456", result[0].Id)

				result, err = sqlDB.GetNextAuthors("a.ghi789", 10)
				require.NoError(t, err)
				assert.Len(t, result, 0)
			})
		})
	}
}

// =============================================================================
// Test: GetNextReadonly2Pad
// =============================================================================

func TestSQLDatabase_GetNextReadonly2Pad(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			mappings := []struct {
				key   string
				value string
			}{
				{"readonly2pad:r.readonly1", "pad1"},
				{"readonly2pad:r.readonly2", "pad2"},
				{"readonly2pad:r.readonly3", "pad3"},
			}

			for _, m := range mappings {
				insertStoreValue(t, db, tc.Driver, m.key, m.value)
			}

			t.Run("GetAllMappings", func(t *testing.T) {
				result, err := sqlDB.GetNextReadonly2Pad("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "r.readonly1", result[0].ReadonlyId)
				assert.Equal(t, "pad1", result[0].PadId)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextReadonly2Pad("", 2)
				require.NoError(t, err)
				assert.Len(t, result, 2)

				result, err = sqlDB.GetNextReadonly2Pad("r.readonly2", 10)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "r.readonly3", result[0].ReadonlyId)
			})
		})
	}
}

// =============================================================================
// Test: GetNextPad2Readonly
// =============================================================================

func TestSQLDatabase_GetNextPad2Readonly(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			mappings := []struct {
				key   string
				value string
			}{
				{"pad2readonly:alpha", "r.readonly_alpha"},
				{"pad2readonly:beta", "r.readonly_beta"},
				{"pad2readonly:gamma", "r.readonly_gamma"},
			}

			for _, m := range mappings {
				insertStoreValue(t, db, tc.Driver, m.key, m.value)
			}

			t.Run("GetAllMappings", func(t *testing.T) {
				result, err := sqlDB.GetNextPad2Readonly("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "alpha", result[0].PadId)
				assert.Equal(t, "r.readonly_alpha", result[0].ReadonlyId)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextPad2Readonly("alpha", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "beta", result[0].PadId)
			})
		})
	}
}

// =============================================================================
// Test: GetNextToken2Author
// =============================================================================

func TestSQLDatabase_GetNextToken2Author(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			mappings := []struct {
				key   string
				value string
			}{
				{"token2author:t.token1", "a.author1"},
				{"token2author:t.token2", "a.author2"},
				{"token2author:t.token3", "a.author1"}, // Same author, different token
			}

			for _, m := range mappings {
				insertStoreValue(t, db, tc.Driver, m.key, m.value)
			}

			t.Run("GetAllMappings", func(t *testing.T) {
				result, err := sqlDB.GetNextToken2Author("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "t.token1", result[0].Token)
				assert.Equal(t, "a.author1", result[0].AuthorId)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextToken2Author("t.token1", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "t.token2", result[0].Token)
			})
		})
	}
}

// =============================================================================
// Test: GetPadChatMessages
// =============================================================================

func TestSQLDatabase_GetPadChatMessages(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			messages := []struct {
				key   string
				value map[string]any
			}{
				{
					key: "pad:testpad:chat:0",
					value: map[string]any{
						"text":     "Hello everyone!",
						"authorid": "a.author1",
						"time":     1700000000000,
						"userName": "Alice",
					},
				},
				{
					key: "pad:testpad:chat:1",
					value: map[string]any{
						"text":     "Hi Alice!",
						"authorid": "a.author2",
						"time":     1700000001000,
						"userName": "Bob",
					},
				},
				{
					key: "pad:testpad:chat:2",
					value: map[string]any{
						"text":     "How are you?",
						"authorid": "a.author1",
						"time":     1700000002000,
						"userName": "Alice",
					},
				},
				{
					key: "pad:testpad:chat:10",
					value: map[string]any{
						"text":     "Message 10",
						"authorid": "a.author3",
						"time":     1700000010000,
						"userName": "Charlie",
					},
				},
				// Different pad
				{
					key: "pad:otherpad:chat:0",
					value: map[string]any{
						"text":     "Other pad message",
						"authorid": "a.author1",
						"time":     1700000000000,
						"userName": "Alice",
					},
				},
			}

			for _, m := range messages {
				insertStoreValue(t, db, tc.Driver, m.key, m.value)
			}

			t.Run("GetAllMessages", func(t *testing.T) {
				result, err := sqlDB.GetPadChatMessages("testpad", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 4)

				// Should be sorted numerically
				assert.Equal(t, 0, result[0].ChatNum)
				assert.Equal(t, "testpad", result[0].PadId)
				assert.Equal(t, 1, result[1].ChatNum)
				assert.Equal(t, 2, result[2].ChatNum)
				assert.Equal(t, 10, result[3].ChatNum)
			})

			t.Run("MessageFields", func(t *testing.T) {
				result, err := sqlDB.GetPadChatMessages("testpad", -1, 1)
				require.NoError(t, err)
				require.Len(t, result, 1)

				assert.Equal(t, "Hello everyone!", result[0].Text)
				assert.Equal(t, "a.author1", result[0].AuthorId)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetPadChatMessages("testpad", -1, 2)
				require.NoError(t, err)
				assert.Len(t, result, 2)

				result, err = sqlDB.GetPadChatMessages("testpad", 1, 2)
				require.NoError(t, err)
				assert.Len(t, result, 2)
				assert.Equal(t, 2, result[0].ChatNum)
				assert.Equal(t, 10, result[1].ChatNum)
			})

			t.Run("NumericOrdering", func(t *testing.T) {
				// Add messages 3-9
				for i := 3; i < 10; i++ {
					insertStoreValue(t, db, tc.Driver, fmt.Sprintf("pad:testpad:chat:%d", i), map[string]any{
						"text":     fmt.Sprintf("Message %d", i),
						"userId":   "a.author1",
						"time":     1700000000000 + int64(i*1000),
						"userName": "Test",
					})
				}

				result, err := sqlDB.GetPadChatMessages("testpad", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 11)

				for i, msg := range result {
					assert.Equal(t, i, msg.ChatNum)
				}
			})

			t.Run("NonExistentPad", func(t *testing.T) {
				result, err := sqlDB.GetPadChatMessages("nonexistent", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 0)
			})
		})
	}
}

// =============================================================================
// Test: GetNextGroups
// =============================================================================

func TestSQLDatabase_GetNextGroups(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			groups := []struct {
				key   string
				value map[string]any
			}{
				{
					key: "group:g.group1",
					value: map[string]any{
						"pads": map[string]int{"pad1": 1, "pad2": 1},
					},
				},
				{
					key: "group:g.group2",
					value: map[string]any{
						"pads": map[string]int{"pad3": 1},
					},
				},
				{
					key: "group:g.group3",
					value: map[string]any{
						"pads": map[string]int{},
					},
				},
			}

			for _, g := range groups {
				insertStoreValue(t, db, tc.Driver, g.key, g.value)
			}

			t.Run("GetAllGroups", func(t *testing.T) {
				result, err := sqlDB.GetNextGroups("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "g.group1", result[0].GroupId)
				assert.Equal(t, "g.group2", result[1].GroupId)
				assert.Equal(t, "g.group3", result[2].GroupId)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextGroups("g.group1", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "g.group2", result[0].GroupId)
			})
		})
	}
}

// =============================================================================
// Test: GetNextGroup2Sessions
// =============================================================================

func TestSQLDatabase_GetNextGroup2Sessions(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			mappings := []struct {
				key   string
				value map[string]int
			}{
				{"group2sessions:g.group1", map[string]int{"s.session1": 1, "s.session2": 1}},
				{"group2sessions:g.group2", map[string]int{"s.session3": 1}},
				{"group2sessions:g.group3", map[string]int{}},
			}

			for _, m := range mappings {
				insertStoreValue(t, db, tc.Driver, m.key, m.value)
			}

			t.Run("GetAllMappings", func(t *testing.T) {
				result, err := sqlDB.GetNextGroup2Sessions("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "g.group1", result[0].GroupId)
				assert.Len(t, result[0].Sessions, 2)
			})

			t.Run("SessionMap", func(t *testing.T) {
				result, err := sqlDB.GetNextGroup2Sessions("", 1)
				require.NoError(t, err)
				require.Len(t, result, 1)

				assert.Contains(t, result[0].Sessions, "s.session1")
				assert.Contains(t, result[0].Sessions, "s.session2")
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextGroup2Sessions("g.group1", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "g.group2", result[0].GroupId)
			})
		})
	}
}

// =============================================================================
// Test: GetNextAuthor2Sessions
// =============================================================================

func TestSQLDatabase_GetNextAuthor2Sessions(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			mappings := []struct {
				key   string
				value map[string]int
			}{
				{"author2sessions:a.author1", map[string]int{"s.session1": 1, "s.session2": 1}},
				{"author2sessions:a.author2", map[string]int{"s.session3": 1}},
				{"author2sessions:a.author3", map[string]int{}},
			}

			for _, m := range mappings {
				insertStoreValue(t, db, tc.Driver, m.key, m.value)
			}

			t.Run("GetAllMappings", func(t *testing.T) {
				result, err := sqlDB.GetNextAuthor2Sessions("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "a.author1", result[0].AuthorId)
				assert.Len(t, result[0].Sessions, 2)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextAuthor2Sessions("a.author1", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "a.author2", result[0].AuthorId)
			})
		})
	}
}

// =============================================================================
// Test: GetNextSessions
// =============================================================================

func TestSQLDatabase_GetNextSessions(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			sessions := []struct {
				key   string
				value map[string]any
			}{
				{
					key: "session:s.session1",
					value: map[string]any{
						"groupID":    "g.group1",
						"authorID":   "a.author1",
						"validUntil": 1700000000,
					},
				},
				{
					key: "session:s.session2",
					value: map[string]any{
						"groupID":    "g.group1",
						"authorID":   "a.author2",
						"validUntil": 1700001000,
					},
				},
				{
					key: "session:s.session3",
					value: map[string]any{
						"groupID":    "g.group2",
						"authorID":   "a.author1",
						"validUntil": 1700002000,
					},
				},
			}

			for _, s := range sessions {
				insertStoreValue(t, db, tc.Driver, s.key, s.value)
			}

			t.Run("GetAllSessions", func(t *testing.T) {
				result, err := sqlDB.GetNextSessions("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 3)

				assert.Equal(t, "s.session1", result[0].SessionId)
				assert.Equal(t, "s.session2", result[1].SessionId)
				assert.Equal(t, "s.session3", result[2].SessionId)
			})

			t.Run("SessionFields", func(t *testing.T) {
				result, err := sqlDB.GetNextSessions("", 1)
				require.NoError(t, err)
				require.Len(t, result, 1)

				assert.Equal(t, "s.session1", result[0].SessionId)
				assert.Equal(t, "g.group1", result[0].GroupId)
				assert.Equal(t, "a.author1", result[0].AuthorId)
			})

			t.Run("Pagination", func(t *testing.T) {
				result, err := sqlDB.GetNextSessions("s.session1", 1)
				require.NoError(t, err)
				assert.Len(t, result, 1)
				assert.Equal(t, "s.session2", result[0].SessionId)

				result, err = sqlDB.GetNextSessions("s.session3", 10)
				require.NoError(t, err)
				assert.Len(t, result, 0)
			})
		})
	}
}

// =============================================================================
// Test: Close
// =============================================================================

func TestSQLDatabase_Close(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	sqlDB, err := NewSQLDatabase(db, DriverSQLite)
	require.NoError(t, err)

	err = sqlDB.Close()
	require.NoError(t, err)

	// Verify connection is closed
	err = db.Ping()
	assert.Error(t, err)
}

// =============================================================================
// Test: NewSQLDatabase with different drivers
// =============================================================================

func TestNewSQLDatabase_PlaceholderStyles(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	t.Run("PostgresPlaceholder", func(t *testing.T) {
		sqlDB, err := NewSQLDatabase(db, DriverPostgres)
		require.NoError(t, err)
		assert.Equal(t, "$1", sqlDB.placeholder(1))
		assert.Equal(t, "$2", sqlDB.placeholder(2))
		assert.Equal(t, "$10", sqlDB.placeholder(10))
	})

	t.Run("SQLitePlaceholder", func(t *testing.T) {
		sqlDB, err := NewSQLDatabase(db, DriverSQLite)
		require.NoError(t, err)
		assert.Equal(t, "?", sqlDB.placeholder(1))
		assert.Equal(t, "?", sqlDB.placeholder(2))
		assert.Equal(t, "?", sqlDB.placeholder(10))
	})

	t.Run("MySQLPlaceholder", func(t *testing.T) {
		sqlDB, err := NewSQLDatabase(db, DriverMySQL)
		require.NoError(t, err)
		assert.Equal(t, "?", sqlDB.placeholder(1))
		assert.Equal(t, "?", sqlDB.placeholder(2))
		assert.Equal(t, "?", sqlDB.placeholder(10))
	})
}

// =============================================================================
// Test: Edge Cases
// =============================================================================

func TestSQLDatabase_EdgeCases(t *testing.T) {
	for _, tc := range getTestCases(t) {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.Driver != DriverSQLite {
				t.Parallel()
			}

			db := tc.Setup(t)
			resetStoreTable(t, db, tc.Driver)
			sqlDB, err := NewSQLDatabase(db, tc.Driver)
			require.NoError(t, err)

			t.Run("SpecialCharactersInPadId", func(t *testing.T) {
				resetStoreTable(t, db, tc.Driver)

				// Pad IDs with special characters
				specialPads := []struct {
					key   string
					value map[string]any
				}{
					{
						key: "pad:test-pad-with-dashes",
						value: map[string]any{
							"atext": map[string]any{"text": "Test\n", "attribs": "|1+5"},
							"pool":  map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
							"head":  0,
						},
					},
					{
						key: "pad:test_pad_with_underscores",
						value: map[string]any{
							"atext": map[string]any{"text": "Test\n", "attribs": "|1+5"},
							"pool":  map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
							"head":  0,
						},
					},
				}

				for _, p := range specialPads {
					insertStoreValue(t, db, tc.Driver, p.key, p.value)
				}

				result, err := sqlDB.GetNextPads("", 10)
				require.NoError(t, err)
				assert.Len(t, result, 2)
			})

			t.Run("UnicodeContent", func(t *testing.T) {
				resetStoreTable(t, db, tc.Driver)

				insertStoreValue(t, db, tc.Driver, "pad:unicode", map[string]any{
					"atext": map[string]any{
						"text":    "Hello 世界 🌍 مرحبا\n",
						"attribs": "|1+20",
					},
					"pool": map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
					"head": 0,
				})

				result, err := sqlDB.GetNextPads("", 10)
				require.NoError(t, err)
				require.Len(t, result, 1)
				assert.Contains(t, result[0].AText.Text, "世界")
				assert.Contains(t, result[0].AText.Text, "🌍")
			})

			t.Run("LargeRevisionNumbers", func(t *testing.T) {
				resetStoreTable(t, db, tc.Driver)

				// Insert revisions with large numbers
				largeRevNums := []int{0, 100, 1000, 10000, 99999}
				for _, rev := range largeRevNums {
					insertStoreValue(t, db, tc.Driver, fmt.Sprintf("pad:largerev:revs:%d", rev), map[string]any{
						"changeset": "Z:1>1+1$x",
						"meta":      map[string]any{"author": "", "timestamp": 1700000000000},
					})
				}

				result, err := sqlDB.GetPadRevisions("largerev", -1, 100)
				require.NoError(t, err)
				assert.Len(t, result, 5)

				// Verify numeric ordering
				assert.Equal(t, 0, result[0].RevNum)
				assert.Equal(t, 100, result[1].RevNum)
				assert.Equal(t, 1000, result[2].RevNum)
				assert.Equal(t, 10000, result[3].RevNum)
				assert.Equal(t, 99999, result[4].RevNum)
			})

			t.Run("EmptyStringValues", func(t *testing.T) {
				resetStoreTable(t, db, tc.Driver)

				insertStoreValue(t, db, tc.Driver, "globalAuthor:a.emptyname", map[string]any{
					"colorId": 10,
					"name":    "",
					"padIDs":  map[string]int{},
				})

				result, err := sqlDB.GetNextAuthors("", 10)
				require.NoError(t, err)
				require.Len(t, result, 1)
				assert.Equal(t, "", result[0].Name)
				assert.Equal(t, 10, result[0].ColorId)
			})

			t.Run("ZeroLimit", func(t *testing.T) {
				resetStoreTable(t, db, tc.Driver)

				insertStoreValue(t, db, tc.Driver, "pad:test", map[string]any{
					"atext": map[string]any{"text": "Test\n", "attribs": "|1+5"},
					"pool":  map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
					"head":  0,
				})

				result, err := sqlDB.GetNextPads("", 0)
				require.NoError(t, err)
				assert.Len(t, result, 0)
			})
		})
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkSQLDatabase_GetNextPads(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE store (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	if err != nil {
		b.Fatal(err)
	}

	// Insert 1000 pads
	for i := 0; i < 1000; i++ {
		padData := map[string]any{
			"atext": map[string]any{"text": "Test\n", "attribs": "|1+5"},
			"pool":  map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
			"head":  0,
		}
		jsonData, _ := json.Marshal(padData)
		_, err = db.Exec("INSERT INTO store (key, value) VALUES (?, ?)",
			fmt.Sprintf("pad:pad%04d", i), string(jsonData))
		if err != nil {
			b.Fatal(err)
		}
	}

	sqlDB, err := NewSQLDatabase(db, DriverSQLite)
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sqlDB.GetNextPads("", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSQLDatabase_GetPadRevisions(b *testing.B) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE store (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
	if err != nil {
		b.Fatal(err)
	}

	// Insert 1000 revisions
	for i := 0; i < 1000; i++ {
		revData := map[string]any{
			"changeset": "Z:1>1+1$x",
			"meta":      map[string]any{"author": "", "timestamp": 1700000000000},
		}
		jsonData, _ := json.Marshal(revData)
		_, err = db.Exec("INSERT INTO store (key, value) VALUES (?, ?)",
			fmt.Sprintf("pad:benchpad:revs:%d", i), string(jsonData))
		if err != nil {
			b.Fatal(err)
		}
	}

	sqlDB, err := NewSQLDatabase(db, DriverSQLite)
	assert.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sqlDB.GetPadRevisions("benchpad", -1, 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}
