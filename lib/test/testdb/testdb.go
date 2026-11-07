// Package testdb gives tests one shared Postgres and one shared MariaDB
// server per machine, and isolated databases on them.
//
// `go test ./...` runs every package in its own process, so process-global
// state can't keep packages from starting their own containers. Instead:
//
//   - In CI (EP_CI=1) the servers are the workflow's service containers
//     (EP_TEST_POSTGRES_HOST/PORT, EP_TEST_MYSQL_HOST/PORT).
//   - Locally each server is a testcontainers container with a fixed name and
//     Reuse enabled. Finding or starting it happens under an OS file lock, so
//     concurrent test processes agree on one container instead of racing.
//     The containers keep running between `go test` runs (their data lives in
//     RAM, without fsync); remove them with
//     `docker rm -f etherpad-test-postgres-v2 etherpad-test-mysql-v2`.
//   - Tests never share a database: each caller gets its own (NewDatabase),
//     so packages can run in parallel against the same servers.
//
// Set EP_TEST_SKIP_SQL_SERVERS=1 to skip everything that needs a server.
package testdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	mysql2 "github.com/go-sql-driver/mysql"
	"github.com/gofrs/flock"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
)

const (
	// User and password of both servers (CI services and containers).
	User     = "test_user"
	Password = "test_password"
	// adminDatabase exists on both servers; databases are created from it.
	adminDatabase = "test_db"

	dbPrefix = "ep_"
	// staleAfter: databases older than this are dropped when a container is
	// acquired. Test processes can't drop their per-process database on exit
	// (there is no hook without a TestMain in every package), and go test
	// stops a test binary after 10 minutes by default, so anything this old
	// is no longer in use.
	staleAfter = time.Hour
)

// Kind is the database server type.
type Kind string

const (
	Postgres Kind = "postgres"
	MySQL    Kind = "mysql"
)

// Server is a running database server tests can create databases on.
type Server struct {
	Kind Kind
	Host string
	Port int
}

// SkipSQLServers reports whether tests needing Postgres/MySQL should be skipped.
func SkipSQLServers() bool {
	return envBool("EP_TEST_SKIP_SQL_SERVERS")
}

func isCI() bool { return envBool("EP_CI") }

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

type serverOnce struct {
	once   sync.Once
	server *Server
	err    error
}

var servers = map[Kind]*serverOnce{Postgres: {}, MySQL: {}}

// Acquire returns the shared server of the given kind, starting or
// attaching to it on first use in this process.
func Acquire(kind Kind) (*Server, error) {
	s := servers[kind]
	s.once.Do(func() { s.server, s.err = acquire(kind) })
	return s.server, s.err
}

// Get is Acquire for tests: it skips the test when EP_TEST_SKIP_SQL_SERVERS
// is set and fails it when the server is unavailable.
func Get(t testing.TB, kind Kind) *Server {
	t.Helper()
	if SkipSQLServers() {
		t.Skipf("%s skipped: EP_TEST_SKIP_SQL_SERVERS is set", kind)
	}
	srv, err := Acquire(kind)
	if err != nil {
		t.Fatalf("%s test server unavailable: %v", kind, err)
	}
	return srv
}

func acquire(kind Kind) (*Server, error) {
	var srv *Server
	if isCI() {
		hostEnv, portEnv, defPort := "EP_TEST_POSTGRES_HOST", "EP_TEST_POSTGRES_PORT", "5432"
		if kind == MySQL {
			hostEnv, portEnv, defPort = "EP_TEST_MYSQL_HOST", "EP_TEST_MYSQL_PORT", "3306"
		}
		port, err := strconv.Atoi(envOrDefault(portEnv, defPort))
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", portEnv, err)
		}
		srv = &Server{Kind: kind, Host: envOrDefault(hostEnv, "127.0.0.1"), Port: port}
		if err := srv.waitReady(2 * time.Minute); err != nil {
			return nil, err
		}
		return srv, nil
	}

	// One container per kind and machine. The lock only guards find-or-start;
	// testcontainers finds the running container by its name.
	lock := flock.New(filepath.Join(os.TempDir(), "etherpad-testdb-"+string(kind)+".lock"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 200*time.Millisecond)
	if err != nil || !locked {
		return nil, fmt.Errorf("waiting for the %s container lock: %v", kind, err)
	}
	defer lock.Unlock()

	srv, err = startContainer(ctx, kind)
	if err != nil {
		return nil, err
	}
	if err := srv.waitReady(2 * time.Minute); err != nil {
		return nil, err
	}
	srv.dropStaleDatabases()
	return srv, nil
}

// containerVersion is part of the container names. Bump it when the
// container configuration changes, so the next run starts a new container
// instead of reusing one with the old settings.
const containerVersion = "v3"

func containerName(kind Kind) string {
	return "etherpad-test-" + string(kind) + "-" + containerVersion
}

func startContainer(ctx context.Context, kind Kind) (*Server, error) {
	// Ryuk would remove the container when the process that created it ends,
	// while other test processes are still using it.
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	// Test data needn't survive a crash: keep it in RAM and skip fsync. On
	// Docker Desktop (Windows, macOS) every fsync crosses the VM boundary,
	// which made MySQL tests take minutes.
	req := testcontainers.ContainerRequest{
		Name:         containerName(Postgres),
		Image:        "postgres:alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     User,
			"POSTGRES_PASSWORD": Password,
			"POSTGRES_DB":       adminDatabase,
		},
		// Every package runs up to -parallel tests at once, each with its
		// own connection pool: allow more than the default 100 connections.
		Cmd:   []string{"postgres", "-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off", "-c", "max_connections=500"},
		Tmpfs: map[string]string{"/var/lib/postgresql": "rw"},
	}
	containerPort := "5432"
	if kind == MySQL {
		req = testcontainers.ContainerRequest{
			Name:         containerName(MySQL),
			Image:        "mariadb:lts",
			ExposedPorts: []string{"3306/tcp"},
			Env: map[string]string{
				"MARIADB_USER":          User,
				"MARIADB_PASSWORD":      Password,
				"MARIADB_ROOT_PASSWORD": Password,
				"MARIADB_DATABASE":      adminDatabase,
			},
			Cmd: []string{
				"--innodb-flush-log-at-trx-commit=0",
				"--innodb-doublewrite=0",
				"--character-set-server=utf8mb4",
				"--collation-server=utf8mb4_unicode_ci",
				"--max-connections=500",
			},
			Tmpfs: map[string]string{"/var/lib/mysql": "rw"},
		}
		containerPort = "3306"
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("starting %s container: %w", kind, err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		return nil, err
	}
	mapped, err := c.MappedPort(ctx, containerPort)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(mapped.Port())
	if err != nil {
		return nil, err
	}
	return &Server{Kind: kind, Host: host, Port: port}, nil
}

// adminUser is the account that may create and drop databases. MariaDB's
// MARIADB_USER only has rights on MARIADB_DATABASE, so use root there.
func (s *Server) adminUser() string {
	if s.Kind == MySQL {
		return "root"
	}
	return User
}

// AdminUser is the account tests should connect as: it owns the databases
// created by NewDatabase.
func (s *Server) AdminUser() string { return s.adminUser() }

// DSN returns a database/sql DSN for database (driver "pgx" or "mysql").
func (s *Server) DSN(database string) string {
	if s.Kind == Postgres {
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			s.adminUser(), Password, s.Host, s.Port, database)
	}
	cfg := mysql2.NewConfig()
	cfg.User = s.adminUser()
	cfg.Passwd = Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", s.Host, s.Port)
	cfg.DBName = database
	cfg.ParseTime = true
	return cfg.FormatDSN()
}

// DriverName is the database/sql driver for this server.
func (s *Server) DriverName() string {
	if s.Kind == Postgres {
		return "pgx"
	}
	return "mysql"
}

func (s *Server) admin() (*sql.DB, error) {
	return sql.Open(s.DriverName(), s.DSN(adminDatabase))
}

func (s *Server) waitReady(timeout time.Duration) error {
	var lastErr error
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := s.admin()
		if err == nil {
			lastErr = conn.Ping()
			_ = conn.Close()
			if lastErr == nil {
				return nil
			}
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("%s at %s:%d not ready: %v", s.Kind, s.Host, s.Port, lastErr)
}

// newDatabaseName returns "ep_<unix seconds>_<random>"; the timestamp lets
// dropStaleDatabases find leftovers of crashed runs.
func newDatabaseName() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s%d_%s", dbPrefix, time.Now().Unix(), hex.EncodeToString(b))
}

func (s *Server) quote(name string) string {
	if s.Kind == Postgres {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// CreateDatabase creates a new empty database and returns its name. The
// caller is responsible for dropping it (see NewDatabase for tests).
func (s *Server) CreateDatabase() (string, error) {
	conn, err := s.admin()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	name := newDatabaseName()
	stmt := "CREATE DATABASE " + s.quote(name)
	if s.Kind == MySQL {
		// Don't depend on the server default (latin1 before MariaDB 11.6).
		stmt += " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
	}
	if _, err := conn.Exec(stmt); err != nil {
		return "", fmt.Errorf("creating database %s: %w", name, err)
	}
	return name, nil
}

// DropDatabase drops a database created by CreateDatabase.
func (s *Server) DropDatabase(name string) error {
	conn, err := s.admin()
	if err != nil {
		return err
	}
	defer conn.Close()
	stmt := "DROP DATABASE IF EXISTS " + s.quote(name)
	if s.Kind == Postgres {
		stmt += " WITH (FORCE)" // close connections the test left open
	}
	_, err = conn.Exec(stmt)
	return err
}

// NewDatabase creates a database for this test and drops it when the test
// and its subtests have finished.
func (s *Server) NewDatabase(t testing.TB) string {
	t.Helper()
	name, err := s.CreateDatabase()
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Cleanup(func() {
		if err := s.DropDatabase(name); err != nil {
			t.Logf("dropping test database %s: %v", name, err)
		}
	})
	return name
}

// dropStaleDatabases removes databases of earlier test processes. Only
// called locally, where the containers outlive test runs.
func (s *Server) dropStaleDatabases() {
	conn, err := s.admin()
	if err != nil {
		return
	}
	defer conn.Close()
	query := "SELECT datname FROM pg_database WHERE datname LIKE 'ep\\_%'"
	if s.Kind == MySQL {
		query = "SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'ep\\_%'"
	}
	rows, err := conn.Query(query)
	if err != nil {
		return
	}
	var stale []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) != nil {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(name, dbPrefix), "_", 2)
		created, err := strconv.ParseInt(parts[0], 10, 64)
		if err == nil && time.Since(time.Unix(created, 0)) > staleAfter {
			stale = append(stale, name)
		}
	}
	rows.Close()
	for _, name := range stale {
		_ = s.DropDatabase(name)
	}
}
