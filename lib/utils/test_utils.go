package utils

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

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

func waitForPostgresQuery(ctx context.Context, cfg *TestContainerConfiguration, query string, timeout time.Duration) error {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for postgres query: %w", ctx.Err())
		case <-ticker.C:
			db, err := sql.Open("postgres", dsn)
			if err != nil {
				// kurz warten und erneut versuchen
				continue
			}
			// Limitieren der Verbindungen für schnelle Open/Close Versuche
			db.SetConnMaxLifetime(2 * time.Second)
			db.SetMaxOpenConns(1)
			db.SetMaxIdleConns(0)

			// optional: Ping zuerst prüfen
			if err = db.PingContext(ctx); err != nil {
				_ = db.Close()
				continue
			}

			// Query ausführen (erwartet z.B. eine int-Spalte wie bei SELECT 1)
			var tmp interface{}
			if err = db.QueryRowContext(ctx, query).Scan(&tmp); err != nil {
				_ = db.Close()
				continue
			}

			_ = db.Close()
			return nil
		}
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
			wait.ForListeningPort("5432/tcp"),
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

	if err := waitForPostgresQuery(ctx, &tcfg, "SELECT 1", 30*time.Second); err != nil {
		return nil, err
	}

	return &tcfg, nil
}
