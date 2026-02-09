package testutils

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
)

// Container config file path - shared across all test packages
// Note: For best container reuse, run tests with: go test -p 1 ./lib/test/...
// This ensures packages run sequentially and can share containers.
// With parallel package execution, containers may be started per package.
var containerConfigFile = filepath.Join(os.TempDir(), "etherpad_test_containers.json")
var containerLockFile = filepath.Join(os.TempDir(), "etherpad_test_containers.lock")

// Global container instances - started once per test run
var (
	globalPostgresContainer *TestContainerConfiguration
	globalMysqlContainer    *TestContainerConfiguration
	containersInitialized   bool
	containersMutex         sync.Mutex

	// Locks for sequential access to shared database containers
	mysqlTestLock    sync.Mutex
	postgresTestLock sync.Mutex
)

// ContainerConfig is serialized to share container info between test packages
type ContainerConfig struct {
	PostgresHost string `json:"postgres_host"`
	PostgresPort string `json:"postgres_port"`
	MySQLHost    string `json:"mysql_host"`
	MySQLPort    string `json:"mysql_port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Database     string `json:"database"`
	CreatedAt    int64  `json:"created_at"`
}

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

const (
	DbName = "test_db"
	DbUser = "test_user"
	DbPass = "test_password"
	// Container config is valid for 30 minutes
	ContainerConfigTTL = 30 * time.Minute
)

type TestContainerConfiguration struct {
	Container *testcontainers.DockerContainer
	Host      string
	Port      string
	Username  string
	Password  string
	Database  string
}

// tryLoadExistingContainers tries to load container config from file
// Returns true if valid containers were found and are still running
func tryLoadExistingContainers() bool {
	data, err := os.ReadFile(containerConfigFile)
	if err != nil {
		return false
	}

	var config ContainerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		os.Remove(containerConfigFile)
		return false
	}

	// Check if config is too old
	if time.Since(time.Unix(config.CreatedAt, 0)) > ContainerConfigTTL {
		os.Remove(containerConfigFile)
		return false
	}

	// Verify Postgres is still running with retries
	postgresDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.Username, config.Password, config.PostgresHost, config.PostgresPort, config.Database)

	postgresOK := false
	for i := 0; i < 3; i++ {
		postgresConn, err := sql.Open("pgx", postgresDSN)
		if err == nil {
			err = postgresConn.Ping()
			postgresConn.Close()
			if err == nil {
				postgresOK = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !postgresOK {
		os.Remove(containerConfigFile)
		return false
	}

	// Verify MySQL is still running with retries
	mySQLConf := mysql2.NewConfig()
	mySQLConf.User = config.Username
	mySQLConf.Passwd = config.Password
	mySQLConf.Net = "tcp"
	mySQLConf.Addr = fmt.Sprintf("%s:%s", config.MySQLHost, config.MySQLPort)
	mySQLConf.DBName = config.Database
	mySQLConf.ParseTime = true
	mysqlDSN := mySQLConf.FormatDSN()

	mysqlOK := false
	for i := 0; i < 3; i++ {
		mysqlConn, err := sql.Open("mysql", mysqlDSN)
		if err == nil {
			err = mysqlConn.Ping()
			mysqlConn.Close()
			if err == nil {
				mysqlOK = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !mysqlOK {
		os.Remove(containerConfigFile)
		return false
	}

	// Containers are valid - use them
	globalPostgresContainer = &TestContainerConfiguration{
		Host:     config.PostgresHost,
		Port:     config.PostgresPort,
		Username: config.Username,
		Password: config.Password,
		Database: config.Database,
	}
	globalMysqlContainer = &TestContainerConfiguration{
		Host:     config.MySQLHost,
		Port:     config.MySQLPort,
		Username: config.Username,
		Password: config.Password,
		Database: config.Database,
	}

	fmt.Printf("Reusing existing test containers - Postgres: %s:%s, MySQL: %s:%s\n",
		config.PostgresHost, config.PostgresPort, config.MySQLHost, config.MySQLPort)

	return true
}

// saveContainerConfig saves the container configuration to a file
func saveContainerConfig() error {
	config := ContainerConfig{
		PostgresHost: globalPostgresContainer.Host,
		PostgresPort: globalPostgresContainer.Port,
		MySQLHost:    globalMysqlContainer.Host,
		MySQLPort:    globalMysqlContainer.Port,
		Username:     DbUser,
		Password:     DbPass,
		Database:     DbName,
		CreatedAt:    time.Now().Unix(),
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(containerConfigFile, data, 0644)
}

// initGlobalContainers initializes the global containers once
func initGlobalContainers(t *testing.T) {
	t.Helper()

	// Enable testcontainers reuse feature
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	containersMutex.Lock()
	defer containersMutex.Unlock()

	// Already initialized in this process
	if containersInitialized {
		return
	}

	// Use file-based locking to prevent multiple processes from starting containers
	lockFile, err := os.OpenFile(containerLockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		t.Fatalf("Failed to create lock file: %v", err)
	}
	defer lockFile.Close()

	// Try to acquire exclusive lock with retries
	maxRetries := 120 // 2 minutes max wait (120 * 1 second)
	lockAcquired := false

	for i := 0; i < maxRetries; i++ {
		// First, always check if containers are already available
		if tryLoadExistingContainers() {
			containersInitialized = true
			return
		}

		// Try to get exclusive access by checking file modification time
		// If the file was modified less than 2 seconds ago by another process, wait
		stat, err := lockFile.Stat()
		if err == nil && time.Since(stat.ModTime()) < 2*time.Second && i > 0 {
			// Another process is likely starting containers, wait and check again
			time.Sleep(1 * time.Second)
			continue
		}

		// Update file modification time to signal we're working
		lockFile.Truncate(0)
		lockFile.Seek(0, 0)
		fmt.Fprintf(lockFile, "%d-%d", os.Getpid(), time.Now().UnixNano())
		lockFile.Sync()

		// Wait a bit and verify we still have the lock
		time.Sleep(200 * time.Millisecond)

		lockFile.Seek(0, 0)
		content := make([]byte, 100)
		n, _ := lockFile.Read(content)
		contentStr := string(content[:n])

		expectedPrefix := fmt.Sprintf("%d-", os.Getpid())
		if strings.HasPrefix(contentStr, expectedPrefix) {
			lockAcquired = true
			break
		}

		// Another process got the lock, wait
		time.Sleep(1 * time.Second)
	}

	// Final check if containers are now available
	if tryLoadExistingContainers() {
		containersInitialized = true
		return
	}

	if !lockAcquired {
		t.Logf("Warning: Could not acquire exclusive lock, starting containers anyway")
	}

	// Start new containers
	var wg sync.WaitGroup
	var postgresErr, mysqlErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		globalPostgresContainer, postgresErr = PreparePostgresDB()
	}()

	go func() {
		defer wg.Done()
		globalMysqlContainer, mysqlErr = PrepareMySQLDB()
	}()

	wg.Wait()

	if postgresErr != nil {
		t.Fatalf("Failed to prepare Postgres container: %v", postgresErr)
	}
	if mysqlErr != nil {
		t.Fatalf("Failed to prepare MySQL container: %v", mysqlErr)
	}

	// Save config for other test packages
	if err := saveContainerConfig(); err != nil {
		t.Logf("Warning: Failed to save container config: %v", err)
	}

	containersInitialized = true
}

func NewTestDBHandler(t *testing.T) *TestDBHandler {
	t.Helper()

	// Initialize global containers (only runs once per test binary execution)
	initGlobalContainers(t)

	return &TestDBHandler{
		t: t,
	}
}

func PreparePostgresDB() (*TestContainerConfiguration, error) {
	var env = map[string]string{
		"POSTGRES_PASSWORD": DbPass,
		"POSTGRES_USER":     DbUser,
		"POSTGRES_DB":       DbName,
	}
	ctx := context.Background()

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         "etherpad-test-postgres",
			Image:        "postgres:alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env:          env,
			WaitingFor: wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", DbUser, DbPass, host, port.Port(), DbName)
			}).WithStartupTimeout(time.Second * 60).WithQuery("SELECT 10"),
		},
		Started: true,
		Reuse:   true, // Enable container reuse
	}

	postgresContainer, err := testcontainers.GenericContainer(ctx, req)
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

	dockerContainer, _ := postgresContainer.(*testcontainers.DockerContainer)
	tcfg := TestContainerConfiguration{
		Container: dockerContainer,
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

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         "etherpad-test-mysql",
			Image:        "mysql:9.6",
			ExposedPorts: []string{"3306/tcp"},
			Env:          env,
		},
		Started: true,
		Reuse:   true, // Enable container reuse
	}

	mysqlContainer, err := testcontainers.GenericContainer(ctx, req)
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

	dockerContainer, _ := mysqlContainer.(*testcontainers.DockerContainer)
	tcfg := TestContainerConfiguration{
		Container: dockerContainer,
		Host:      host,
		Port:      p.Port(),
		Username:  DbUser,
		Password:  DbPass,
		Database:  DbName,
	}

	return &tcfg, nil
}

// cleanupPostgresTables truncates all tables except migration tables
func cleanupPostgresTables() error {
	if globalPostgresContainer == nil {
		return nil
	}
	port, err := strconv.Atoi(globalPostgresContainer.Port)
	if err != nil {
		return err
	}
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		globalPostgresContainer.Username, globalPostgresContainer.Password,
		globalPostgresContainer.Host, port, globalPostgresContainer.Database)
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
		// Skip migration tables
		if t == "schema_migrations" || t == "migrations" || t == "goose_db_version" {
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

// cleanupMySQLTables truncates all tables except migration tables
func cleanupMySQLTables() error {
	if globalMysqlContainer == nil {
		return nil
	}
	port, err := strconv.Atoi(globalMysqlContainer.Port)
	if err != nil {
		return err
	}

	mySQLConf := mysql2.NewConfig()
	mySQLConf.User = globalMysqlContainer.Username
	mySQLConf.Passwd = globalMysqlContainer.Password
	mySQLConf.Net = "tcp"
	mySQLConf.Addr = fmt.Sprintf("%s:%d", globalMysqlContainer.Host, port)
	mySQLConf.DBName = globalMysqlContainer.Database
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

	rows, err := conn.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = ?",
		globalMysqlContainer.Database)
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
		// Skip migration tables
		if t == "schema_migrations" || t == "migrations" || t == "goose_db_version" {
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
				test.TestRun(t, testConf, newDS, dsName)
			}
		})
	}

	// Note: We don't terminate containers here anymore since they are global
	// They will be cleaned up by ryuk when test process exits
}

func (test *TestDBHandler) InitPostgres() *db.PostgresDB {
	port, err := strconv.Atoi(globalPostgresContainer.Port)
	if err != nil {
		panic(err)
	}
	postgresOpts := db.PostgresOptions{
		Username: globalPostgresContainer.Username,
		Password: globalPostgresContainer.Password,
		Database: globalPostgresContainer.Database,
		Host:     globalPostgresContainer.Host,
		Port:     port,
	}
	postresDB, err := db.NewPostgresDB(postgresOpts)
	if err != nil {
		panic(err)
	}
	return postresDB
}

func (test *TestDBHandler) InitMySQL() *db.MysqlDB {
	port, err := strconv.Atoi(globalMysqlContainer.Port)
	if err != nil {
		panic(err)
	}
	mysqlOpts := db.MySQLOptions{
		Username: globalMysqlContainer.Username,
		Password: globalMysqlContainer.Password,
		Database: globalMysqlContainer.Database,
		Host:     globalMysqlContainer.Host,
		Port:     port,
	}
	mysqlDB, err := db.NewMySQLDB(mysqlOpts)
	if err != nil {
		panic(err)
	}
	return mysqlDB
}

func (test *TestDBHandler) TestRun(
	t *testing.T,
	testRun TestRunConfig,
	newDS func() db.DataStore,
	dsName string,
) {
	t.Run(testRun.Name, func(t *testing.T) {
		// Lock for shared database containers to prevent concurrent access
		switch dsName {
		case "MySQL":
			mysqlTestLock.Lock()
			defer mysqlTestLock.Unlock()
		case "Postgres":
			postgresTestLock.Lock()
			defer postgresTestLock.Unlock()
		}

		ds := newDS()

		// Cleanup tables BEFORE test runs to ensure clean state
		switch dsName {
		case "Postgres":
			if err := cleanupPostgresTables(); err != nil {
				t.Fatalf("Postgres cleanup before test failed: %v", err)
			}
		case "MySQL":
			if err := cleanupMySQLTables(); err != nil {
				t.Fatalf("MySQL cleanup before test failed: %v", err)
			}
		}

		authManager := author.NewManager(ds)
		hooks := hooks2.NewHook()
		hub := ws.NewHub()
		go hub.Run()
		sess := ws.NewSessionStore()
		padManager := pad.NewManager(ds, &hooks)
		loggerPart := zap.NewNop().Sugar()
		importer := io.NewImporter(padManager, authManager, ds, loggerPart)
		padMessageHandler := ws.NewPadMessageHandler(
			ds, &hooks, padManager, &sess, hub, loggerPart, TestAssets,
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

		// Close the DataStore connection after test
		if err := ds.Close(); err != nil {
			t.Errorf("Failed to close DataStore: %v", err)
		}
	})
}
