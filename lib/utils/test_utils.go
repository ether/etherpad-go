package utils

import (
	"context"
	"fmt"

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

	return &TestContainerConfiguration{
		Container: postgresContainer,
		Host:      host,
		Port:      p.Port(),
		Username:  DbUser,
		Password:  DbPass,
		Database:  DbName,
	}, nil
}
