package testutils

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"runtime"
	"strconv"
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
	// Settings is this test's own copy of the settings. The SecurityManager
	// and the InitStore read it, so a test may change it without affecting
	// tests running in parallel.
	Settings *settings.Settings
}

func (t *TestDataStore) ToInitStore() *lib.InitStore {
	t.Settings.LoadTest = true
	t.Settings.EnableMetrics = true
	return &lib.InitStore{
		SecurityManager:   t.SecurityManager,
		RetrievedSettings: t.Settings,
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
		RetrievedSettings: t.Settings,
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
// process (one per Go package) keeps a pool of databases there, as many as
// tests may run at once (-parallel). A database is migrated once when it is
// created and emptied before every test that borrows it, so tests run in
// parallel without touching each other's data.
type databasePool struct {
	once   sync.Once
	server *testdb.Server
	err    error

	mu      sync.Mutex
	created int
	free    chan string
}

var databasePools = map[testdb.Kind]*databasePool{
	testdb.Postgres: {},
	testdb.MySQL:    {},
}

// testDBMaxConns caps each test's connection pool. A test needs only a few
// connections, and up to -parallel tests per package run at once.
const testDBMaxConns = 4

// poolSize is the number of databases a process creates per server at most:
// the -parallel value, which defaults to GOMAXPROCS.
func poolSize() int {
	if f := flag.Lookup("test.parallel"); f != nil {
		if n, err := strconv.Atoi(f.Value.String()); err == nil && n > 0 {
			return n
		}
	}
	return runtime.GOMAXPROCS(0)
}

// borrow returns a database for the test: a free one, a new one while the
// pool is below its size, or else the next one another test gives back. The
// database goes back to the pool when the test finishes.
func (p *databasePool) borrow(t *testing.T, kind testdb.Kind) string {
	t.Helper()
	p.once.Do(func() {
		p.server, p.err = testdb.Acquire(kind)
		p.free = make(chan string, poolSize())
	})
	if p.err != nil {
		t.Fatalf("%s test server unavailable: %v", kind, p.err)
	}

	var name string
	select {
	case name = <-p.free:
	default:
		p.mu.Lock()
		create := p.created < cap(p.free)
		if create {
			p.created++
		}
		p.mu.Unlock()
		if create {
			var err error
			if name, err = createMigratedDatabase(p.server); err != nil {
				p.mu.Lock()
				p.created--
				p.mu.Unlock()
				t.Fatalf("creating %s test database: %v", kind, err)
			}
		} else {
			name = <-p.free
		}
	}
	t.Cleanup(func() { p.free <- name })
	return name
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
	pool := databasePools[kind]
	name := pool.borrow(t, kind)

	if err := truncateTables(pool.server, name); err != nil {
		t.Fatalf("emptying %s test database: %v", kind, err)
	}
	ds, err := openDataStore(pool.server, name)
	if err != nil {
		t.Fatalf("opening %s datastore: %v", kind, err)
	}
	return ds
}

// createMigratedDatabase creates a database for the pool and runs the
// migrations on it.
func createMigratedDatabase(server *testdb.Server) (string, error) {
	name, err := server.CreateDatabase()
	if err != nil {
		return "", err
	}
	ds, err := openDataStore(server, name) // runs the migrations
	if err != nil {
		return "", err
	}
	return name, ds.Close()
}

func openDataStore(server *testdb.Server, name string) (db.DataStore, error) {
	if server.Kind == testdb.Postgres {
		return db.NewPostgresDB(db.PostgresOptions{
			Username: server.AdminUser(),
			Password: testdb.Password,
			Database: name,
			Host:     server.Host,
			Port:     server.Port,
			MaxConns: testDBMaxConns,
		})
	}
	return db.NewMySQLDB(db.MySQLOptions{
		Username: server.AdminUser(),
		Password: testdb.Password,
		Database: name,
		Host:     server.Host,
		Port:     server.Port,
		MaxConns: testDBMaxConns,
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
		// Each test has its own datastore, hub, handlers and settings copy,
		// so tests run in parallel, up to -parallel at a time.
		t.Parallel()
		ds := newDS(t)
		cfg := settings.Displayed

		authManager := author.NewManager(ds)
		hooks := hooks2.NewHook()
		hub := ws.NewHub()
		go hub.Run()
		sess := ws.NewSessionStore()
		padManager := pad.NewManager(ds, &hooks)
		loggerPart := zap.NewNop().Sugar()
		importer := io.NewImporter(padManager, authManager, ds, loggerPart, &hooks)
		padMessageHandler := ws.NewPadMessageHandler(
			ds, &hooks, padManager, &sess, hub, loggerPart, TestAssets, &cfg,
		)
		// As the server: see server.FiberConfig.
		app := fiber.New(fiber.Config{Immutable: true})
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
			SecurityManager:     pad.NewSecurityManager(ds, &hooks, padManager, &cfg),
			Importer:            importer,
			Settings:            &cfg,
		})

		// Close the DataStore connection after test
		if err := ds.Close(); err != nil {
			t.Errorf("Failed to close DataStore: %v", err)
		}
	})
}
