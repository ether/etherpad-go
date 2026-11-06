package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	hooks2 "github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testdb"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type TestDataStore struct {
	DS                  db.DataStore
	Logger              *zap.SugaredLogger
	Hooks               *hooks2.Hook
	ReadOnlyManager     *pad.ReadOnlyManager
	SecurityManager     *pad.SecurityManager
	AuthorManager       *author.Manager
	PadManager          *pad.Manager
	PadMessageHandler   *ws.PadMessageHandler
	AdminMessageHandler *ws.AdminMessageHandler
	MockWebSocket       *ws.MockWebSocketConn
	Validator           *validator.Validate
	Hub                 *ws.Hub
	App                 *fiber.App
	PrivateAPI          fiber.Router
	Importer            *io.Importer
}

func (t *TestDataStore) ToInitStore() *lib.InitStore {
	settings.Displayed.LoadTest = true
	settings.Displayed.EnableMetrics = true
	return &lib.InitStore{
		SecurityManager:   t.SecurityManager,
		RetrievedSettings: &settings.Displayed,
		Store:             t.DS,
		AuthorManager:     t.AuthorManager,
		PadManager:        t.PadManager,
		Handler:           t.PadMessageHandler,
		Validator:         t.Validator,
		Logger:            t.Logger,
		Hooks:             t.Hooks,
		ReadOnlyManager:   t.ReadOnlyManager,
		C:                 t.App,
		PrivateAPI:        t.PrivateAPI,
		UiAssets:          GetTestAssets(),
		Importer:          t.Importer,
	}
}

func (t *TestDataStore) ToPluginStore() *interfaces.EpPluginStore {
	return &interfaces.EpPluginStore{
		Logger:            t.Logger,
		HookSystem:        t.Hooks,
		PadManager:        t.PadManager,
		App:               t.App,
		RetrievedSettings: &settings.Displayed,
		UIAssets:          GetTestAssets(),
	}
}

type TestRunConfig struct {
	Name string
	Test func(t *testing.T, tsStore TestDataStore)
}

type TestDBHandler struct {
	t     *testing.T
	tests []TestRunConfig
}

// Postgres and MySQL run on the shared servers from lib/test/testdb. Each test
// process (one per Go package) gets its own database there, migrated once
// and emptied before every test: packages run in parallel without touching
// each other's data, and tests within a package take turns on it.
type processDatabase struct {
	once   sync.Once
	server *testdb.Server
	name   string
	err    error
	// One test at a time on this process's database.
	inUse sync.Mutex
}

var processDatabases = map[testdb.Kind]*processDatabase{
	testdb.Postgres: {},
	testdb.MySQL:    {},
}

func NewTestDBHandler(t *testing.T) *TestDBHandler {
	t.Helper()
	return &TestDBHandler{
		t: t,
	}
}

func (test *TestDBHandler) AddTests(testConfs ...TestRunConfig) {
	for _, testConf := range testConfs {
		test.tests = append(test.tests, testConf)
	}
}

func (test *TestDBHandler) StartTestDBHandler() {
	datastores := map[string]func(t *testing.T) db.DataStore{
		"Memory": func(*testing.T) db.DataStore {
			return db.NewMemoryDataStore()
		},
		"SQLite": func(t *testing.T) db.DataStore {
			sqliteDB, err := db.NewSQLiteDB(":memory:")
			if err != nil {
				t.Fatalf("Failed to create SQLite DataStore: %v", err)
			}
			return sqliteDB
		},
		"Postgres": func(t *testing.T) db.DataStore {
			return newSQLDataStore(t, testdb.Postgres)
		},
		"MySQL": func(t *testing.T) db.DataStore {
			return newSQLDataStore(t, testdb.MySQL)
		},
	}

	for dsName, newDS := range datastores {
		dsName := dsName
		newDS := newDS

		test.t.Run(dsName, func(t *testing.T) {
			t.Parallel()

			for _, testConf := range test.tests {
				testConf := testConf
				test.TestRun(t, testConf, newDS, dsName)
			}
		})
	}
}

// newSQLDataStore returns a datastore on this process's database of the given
// kind, emptied for the test. The database is reserved for the test until it
// finishes. Skips the test when EP_TEST_SKIP_SQL_SERVERS is set.
func newSQLDataStore(t *testing.T, kind testdb.Kind) db.DataStore {
	t.Helper()
	if testdb.SkipSQLServers() {
		t.Skipf("%s skipped: EP_TEST_SKIP_SQL_SERVERS is set", kind)
	}
	pdb := processDatabases[kind]
	pdb.once.Do(func() { pdb.server, pdb.name, pdb.err = setUpProcessDatabase(kind) })
	if pdb.err != nil {
		t.Fatalf("%s test database unavailable: %v", kind, pdb.err)
	}

	pdb.inUse.Lock()
	t.Cleanup(pdb.inUse.Unlock)

	if err := truncateTables(pdb.server, pdb.name); err != nil {
		t.Fatalf("emptying %s test database: %v", kind, err)
	}
	ds, err := openDataStore(pdb.server, pdb.name)
	if err != nil {
		t.Fatalf("opening %s datastore: %v", kind, err)
	}
	return ds
}

// setUpProcessDatabase creates this process's database and runs the
// migrations on it once.
func setUpProcessDatabase(kind testdb.Kind) (*testdb.Server, string, error) {
	server, err := testdb.Acquire(kind)
	if err != nil {
		return nil, "", err
	}
	name, err := server.CreateDatabase()
	if err != nil {
		return nil, "", err
	}
	ds, err := openDataStore(server, name) // runs the migrations
	if err != nil {
		return nil, "", err
	}
	return server, name, ds.Close()
}

func openDataStore(server *testdb.Server, name string) (db.DataStore, error) {
	if server.Kind == testdb.Postgres {
		return db.NewPostgresDB(db.PostgresOptions{
			Username: server.AdminUser(),
			Password: testdb.Password,
			Database: name,
			Host:     server.Host,
			Port:     server.Port,
		})
	}
	return db.NewMySQLDB(db.MySQLOptions{
		Username: server.AdminUser(),
		Password: testdb.Password,
		Database: name,
		Host:     server.Host,
		Port:     server.Port,
	})
}

// truncateTables empties every table except the migration bookkeeping.
func truncateTables(server *testdb.Server, name string) error {
	conn, err := sql.Open(server.DriverName(), server.DSN(name))
	if err != nil {
		return err
	}
	defer conn.Close()

	query := "SELECT tablename FROM pg_tables WHERE schemaname = 'public'"
	if server.Kind == testdb.MySQL {
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()"
	}
	rows, err := conn.Query(query)
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return err
		}
		if table == "schema_migrations" || table == "migrations" || table == "goose_db_version" {
			continue
		}
		tables = append(tables, table)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}

	if server.Kind == testdb.Postgres {
		// DELETE in one transaction is several times faster than TRUNCATE for
		// the few rows a test writes. The schema has no sequences to reset;
		// replica mode skips the foreign key triggers so the order of the
		// tables doesn't matter (test_user is a superuser on both servers).
		var sb strings.Builder
		sb.WriteString("BEGIN; SET LOCAL session_replication_role = replica;")
		for _, table := range tables {
			sb.WriteString(` DELETE FROM "` + strings.ReplaceAll(table, `"`, `""`) + `";`)
		}
		sb.WriteString(" COMMIT;")
		_, err = conn.Exec(sb.String())
		return err
	}

	// MySQL: DELETE rather than TRUNCATE, which recreates the table (and its
	// file) every time and is far slower for the few rows a test writes.
	// Foreign key checks are per connection, so pin one.
	c, err := conn.Conn(context.Background())
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err := c.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := c.ExecContext(context.Background(), "DELETE FROM `"+strings.ReplaceAll(table, "`", "``")+"`"); err != nil {
			return fmt.Errorf("emptying %s: %w", table, err)
		}
	}
	_, err = c.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS = 1")
	return err
}

func (test *TestDBHandler) TestRun(
	t *testing.T,
	testRun TestRunConfig,
	newDS func(t *testing.T) db.DataStore,
	dsName string,
) {
	t.Run(testRun.Name, func(t *testing.T) {
		ds := newDS(t)

		authManager := author.NewManager(ds)
		hooks := hooks2.NewHook()
		hub := ws.NewHub()
		go hub.Run()
		sess := ws.NewSessionStore()
		padManager := pad.NewManager(ds, &hooks)
		loggerPart := zap.NewNop().Sugar()
		importer := io.NewImporter(padManager, authManager, ds, loggerPart, &hooks)
		padMessageHandler := ws.NewPadMessageHandler(
			ds, &hooks, padManager, &sess, hub, loggerPart, TestAssets,
		)
		app := fiber.New()
		adminMessageHandler := ws.NewAdminMessageHandler(
			ds, &hooks, padManager, padMessageHandler, loggerPart, hub,
			app, nil,
		)
		validatorEvaluator := validator.New(validator.WithRequiredStructEnabled())

		privateAPI := app.Group("/admin/api")

		testRun.Test(t, TestDataStore{
			DS:                  ds,
			AuthorManager:       authManager,
			PadManager:          padManager,
			PadMessageHandler:   padMessageHandler,
			AdminMessageHandler: &adminMessageHandler,
			MockWebSocket:       ws.NewActualMockWebSocketconn(),
			Validator:           validatorEvaluator,
			Hub:                 hub,
			ReadOnlyManager:     pad.NewReadOnlyManager(ds),
			Hooks:               &hooks,
			App:                 app,
			PrivateAPI:          privateAPI,
			Logger:              loggerPart,
			SecurityManager:     pad.NewSecurityManager(ds, &hooks, padManager),
			Importer:            importer,
		})

		// Close the DataStore connection after test
		if err := ds.Close(); err != nil {
			t.Errorf("Failed to close DataStore: %v", err)
		}
	})
}
