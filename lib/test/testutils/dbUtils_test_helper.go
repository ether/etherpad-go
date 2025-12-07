package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	hooks2 "github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

type TestDataStore struct {
	DS                  db.DataStore
	AuthorManager       *author.Manager
	PadManager          *pad.Manager
	PadMessageHandler   *ws.PadMessageHandler
	AdminMessageHandler *ws.AdminMessageHandler
	MockWebSocket       *ws.MockWebSocketConn
	Validator           *validator.Validate
	Hub                 *ws.Hub
}

type TestRunConfig struct {
	Name string
	Test func(t *testing.T, tsStore TestDataStore)
}

type TestDBHandler struct {
	testPostgresContainer *TestContainerConfiguration
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

	postgresConfig, err := PreparePostgresDB()
	if err != nil {
		t.Fatalf("Failed to prepare Postgres test container: %v", err)
	}
	testDBHandler := TestDBHandler{
		t:                     t,
		testPostgresContainer: postgresConfig,
	}

	return &testDBHandler
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
			wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", DbUser, DbPass, host, port.Port(), DbName)
			}).
				WithStartupTimeout(time.Second*5).
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
	conn, err := sql.Open("postgres", dsn)
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
	}

	for dsName, newDS := range datastores {
		test.t.Run(dsName, func(t *testing.T) {
			for _, testConf := range test.tests {
				test.TestRun(t, testConf, newDS)
			}
		})
	}

	// Cleanup containers
	if err := test.testPostgresContainer.Container.Terminate(context.Background()); err != nil {
		test.t.Fatalf("Failed to terminate Postgres test container: %v", err)
	}
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

func (test *TestDBHandler) TestRun(t *testing.T, testRun TestRunConfig, newDS func() db.DataStore) {
	t.Run(testRun.Name, func(t *testing.T) {
		ds := newDS()
		authManager := author.NewManager(ds)
		hooks := hooks2.NewHook()
		hub := ws.NewHub()
		go hub.Run()
		sess := ws.NewSessionStore()
		padManager := pad.NewManager(ds, &hooks)
		padMessageHandler := ws.NewPadMessageHandler(ds, &hooks, padManager, &sess, hub)
		loggerPart := zap.NewNop().Sugar()
		adminMessageHandler := ws.NewAdminMessageHandler(ds, &hooks, padManager, padMessageHandler, loggerPart, hub)
		validatorEvaluator := validator.New(validator.WithRequiredStructEnabled())
		testRun.Test(t, TestDataStore{
			DS:                  ds,
			AuthorManager:       authManager,
			PadManager:          padManager,
			PadMessageHandler:   padMessageHandler,
			AdminMessageHandler: &adminMessageHandler,
			MockWebSocket:       ws.NewActualMockWebSocketconn(),
			Validator:           validatorEvaluator,
			Hub:                 hub,
		})
		t.Cleanup(func() {
			if err := ds.Close(); err != nil {
				t.Fatalf("Failed to close SQLite DataStore: %v", err)
			}
			if test.testPostgresContainer != nil {
				if err := test.cleanupPostgresTables(); err != nil {
					t.Fatalf("Postgres cleanup failed: %v", err)
				}
			}
			if err := ds.Close(); err != nil {
				t.Fatalf("Failed to close DataStore: %v", err)
			}
		})
	})
}
