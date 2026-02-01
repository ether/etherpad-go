package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	hooks2 "github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	mysql2 "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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
	testPostgresContainer *TestContainerConfiguration
	testMysqlContainer    *TestContainerConfiguration
	t                     *testing.T
	tests                 []TestRunConfig
}

const (
	DbName = "test_db"
	DbUser = "test_user"
	DbPass = "test_password"
)

type TestContainerConfiguration struct {
	Container *testcontainers.DockerContainer
	Host      string
	Port      string
	Username  string
	Password  string
	Database  string
}

func NewTestDBHandler(t *testing.T) *TestDBHandler {
	t.Helper()

	var (
		postgresConfig *TestContainerConfiguration
		mysqlConfig    *TestContainerConfiguration
	)

	var g errgroup.Group

	g.Go(func() error {
		cfg, err := PreparePostgresDB()
		if err != nil {
			return fmt.Errorf("postgres: %w", err)
		}
		postgresConfig = cfg
		return nil
	})

	g.Go(func() error {
		cfg, err := PrepareMySQLDB()
		if err != nil {
			return fmt.Errorf("mysql: %w", err)
		}
		mysqlConfig = cfg
		return nil
	})

	if err := g.Wait(); err != nil {
		t.Fatalf("Failed to prepare test databases: %v", err)
	}

	return &TestDBHandler{
		t:                     t,
		testPostgresContainer: postgresConfig,
		testMysqlContainer:    mysqlConfig,
	}
}

func PreparePostgresDB() (*TestContainerConfiguration, error) {
	var env = map[string]string{
		"POSTGRES_PASSWORD": DbPass,
		"POSTGRES_USER":     DbUser,
		"POSTGRES_DB":       DbName,
	}
	ctx := context.Background()
	postgresContainer, err := testcontainers.Run(
		ctx, "postgres:alpine",
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", DbUser, DbPass, host, port.Port(), DbName)
			}).
				WithStartupTimeout(time.Second*30).
				WithQuery("SELECT 10"),
		),
		testcontainers.WithEnv(env),
	)
	if err != nil {
		return nil, err
	}

	p, err := postgresContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, err
	}

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Postgres test container started at %s:%s\n", host, p.Port())

	tcfg := TestContainerConfiguration{
		Container: postgresContainer,
		Host:      host,
		Port:      p.Port(),
		Username:  DbUser,
		Password:  DbPass,
		Database:  DbName,
	}

	return &tcfg, nil
}

func PrepareMySQLDB() (*TestContainerConfiguration, error) {
	var env = map[string]string{
		"MYSQL_PASSWORD":      DbPass,
		"MYSQL_ROOT_PASSWORD": DbPass,
		"MYSQL_USER":          DbUser,
		"MYSQL_DATABASE":      DbName,
	}
	ctx := context.Background()
	mysqlContainer, err := testcontainers.Run(
		ctx, "mysql:9.6",
		testcontainers.WithExposedPorts("3306/tcp"),
		testcontainers.WithEnv(env),
	)
	if err != nil {
		return nil, err
	}

	p, err := mysqlContainer.MappedPort(ctx, "3306")
	if err != nil {
		return nil, err
	}

	host, err := mysqlContainer.Host(ctx)
	if err != nil {
		return nil, err
	}

	mySQLConf := mysql2.NewConfig()
	mySQLConf.User = DbUser
	mySQLConf.Passwd = DbPass
	mySQLConf.Net = "tcp"
	mySQLConf.Addr = fmt.Sprintf("%s:%s", host, p.Port())
	mySQLConf.DBName = DbName
	mySQLConf.ParseTime = true
	dsn := mySQLConf.FormatDSN()

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				db.Close()
				break
			}
			db.Close()
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("MySQL test container started at %s:%s\n", host, p.Port())

	tcfg := TestContainerConfiguration{
		Container: mysqlContainer,
		Host:      host,
		Port:      p.Port(),
		Username:  DbUser,
		Password:  DbPass,
		Database:  DbName,
	}

	return &tcfg, nil
}

func (test *TestDBHandler) cleanupPostgresTables() error {
	if test.testPostgresContainer == nil {
		return nil
	}
	port, err := strconv.Atoi(test.testPostgresContainer.Port)
	if err != nil {
		return err
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		test.testPostgresContainer.Username, test.testPostgresContainer.Password, test.testPostgresContainer.Host, port, test.testPostgresContainer.Database)
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer conn.Close()

	rows, err := conn.Query("SELECT tablename FROM pg_tables WHERE schemaname = 'public'")
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		if t == "schema_migrations" || t == "migrations" {
			continue
		}
		quoted := `"` + strings.ReplaceAll(t, `"`, `""`) + `"`
		tables = append(tables, quoted)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(tables) == 0 {
		return nil
	}

	_, err = conn.Exec("TRUNCATE TABLE " + strings.Join(tables, ",") + " RESTART IDENTITY CASCADE")
	return err
}

func (test *TestDBHandler) cleanupMySQLTables() error {
	if test.testMysqlContainer == nil {
		return nil
	}
	port, err := strconv.Atoi(test.testMysqlContainer.Port)
	if err != nil {
		return err
	}

	mySQLConf := mysql2.NewConfig()
	mySQLConf.User = test.testMysqlContainer.Username
	mySQLConf.Passwd = test.testMysqlContainer.Password
	mySQLConf.Net = "tcp"
	mySQLConf.Addr = fmt.Sprintf("%s:%d", test.testMysqlContainer.Host, port)
	mySQLConf.DBName = test.testMysqlContainer.Database
	mySQLConf.ParseTime = true
	dsn := mySQLConf.FormatDSN()

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Exec("SET FOREIGN_KEY_CHECKS = 0")
	if err != nil {
		return err
	}

	rows, err := conn.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = ?", test.testMysqlContainer.Database)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		if t == "schema_migrations" || t == "migrations" {
			continue
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, table := range tables {
		quoted := "`" + strings.ReplaceAll(table, "`", "``") + "`"
		_, err = conn.Exec("TRUNCATE TABLE " + quoted)
		if err != nil {
			conn.Exec("SET FOREIGN_KEY_CHECKS = 1")
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}

	_, err = conn.Exec("SET FOREIGN_KEY_CHECKS = 1")
	return err
}

func (test *TestDBHandler) AddTests(testConfs ...TestRunConfig) {
	for _, testConf := range testConfs {
		test.tests = append(test.tests, testConf)
	}
}

func (test *TestDBHandler) StartTestDBHandler() {
	datastores := map[string]func() db.DataStore{
		"Memory": func() db.DataStore {
			return db.NewMemoryDataStore()
		},
		"SQLite": func() db.DataStore {
			sqliteDB, err := db.NewSQLiteDB(":memory:")
			if err != nil {
				test.t.Fatalf("Failed to create SQLite DataStore: %v", err)
			}
			return sqliteDB
		},
		"Postgres": func() db.DataStore {
			return test.InitPostgres()
		},
		"MySQL": func() db.DataStore {
			return test.InitMySQL()
		},
	}

	for dsName, newDS := range datastores {
		dsName := dsName
		newDS := newDS

		test.t.Run(dsName, func(t *testing.T) {
			t.Parallel()

			for _, testConf := range test.tests {
				testConf := testConf
				test.TestRun(t, testConf, newDS)
			}
		})
	}

	test.t.Cleanup(func() {
		if test.testPostgresContainer != nil {
			_ = test.testPostgresContainer.Container.Terminate(context.Background())
		}
		if test.testMysqlContainer != nil {
			_ = test.testMysqlContainer.Container.Terminate(context.Background())
		}
	})
}

func (test *TestDBHandler) InitPostgres() *db.PostgresDB {
	port, err := strconv.Atoi(test.testPostgresContainer.Port)
	if err != nil {
		panic(err)
	}
	postgresOpts := db.PostgresOptions{
		Username: test.testPostgresContainer.Username,
		Password: test.testPostgresContainer.Password,
		Database: test.testPostgresContainer.Database,
		Host:     test.testPostgresContainer.Host,
		Port:     port,
	}
	postresDB, err := db.NewPostgresDB(postgresOpts)
	if err != nil {
		panic(err)
	}
	return postresDB
}

func (test *TestDBHandler) InitMySQL() *db.MysqlDB {
	port, err := strconv.Atoi(test.testMysqlContainer.Port)
	if err != nil {
		panic(err)
	}
	mysqlOpts := db.MySQLOptions{
		Username: test.testMysqlContainer.Username,
		Password: test.testMysqlContainer.Password,
		Database: test.testMysqlContainer.Database,
		Host:     test.testMysqlContainer.Host,
		Port:     port,
	}
	mysqlDB, err := db.NewMySQLDB(mysqlOpts)
	if err != nil {
		panic(err)
	}
	return mysqlDB
}

var (
	mysqlTestLock    sync.Mutex
	postgresTestLock sync.Mutex
)

func (test *TestDBHandler) TestRun(
	t *testing.T,
	testRun TestRunConfig,
	newDS func() db.DataStore,
) {
	t.Run(testRun.Name, func(t *testing.T) {
		ds := newDS()

		switch ds.(type) {
		case *db.MysqlDB:
			mysqlTestLock.Lock()
		case *db.PostgresDB:
			postgresTestLock.Lock()
		}

		t.Cleanup(func() {
			if err := ds.Close(); err != nil {
				t.Fatalf("Failed to close DataStore: %v", err)
			}
			switch ds.(type) {
			case *db.MysqlDB:
				mysqlTestLock.Unlock()
			case *db.PostgresDB:
				postgresTestLock.Unlock()
			}
		})

		authManager := author.NewManager(ds)
		hooks := hooks2.NewHook()
		hub := ws.NewHub()
		go hub.Run()
		sess := ws.NewSessionStore()
		padManager := pad.NewManager(ds, &hooks)
		loggerPart := zap.NewNop().Sugar()
		importer := io.NewImporter(padManager, authManager, ds, loggerPart)
		padMessageHandler := ws.NewPadMessageHandler(
			ds, &hooks, padManager, &sess, hub, loggerPart,
		)
		app := fiber.New()
		adminMessageHandler := ws.NewAdminMessageHandler(
			ds, &hooks, padManager, padMessageHandler, loggerPart, hub,
			app,
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
		t.Cleanup(func() {
			switch ds.(type) {
			case *db.PostgresDB:
				if err := test.cleanupPostgresTables(); err != nil {
					t.Fatalf("Postgres cleanup failed: %v", err)
				}
			case *db.MysqlDB:
				if err := test.cleanupMySQLTables(); err != nil {
					t.Fatalf("MySQL cleanup failed: %v", err)
				}
			}

			if err := ds.Close(); err != nil {
				t.Fatalf("Failed to close DataStore: %v", err)
			}
		})
	})
}
