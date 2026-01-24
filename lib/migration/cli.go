package migration

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

const EpDbPassword = "EP_DB_PASSWORD" // environment variable for the database password

func RunFromCLI(logger *zap.SugaredLogger, args []string) {
	logger.Info("Migration CLI called with args:", args)
	dbPassword := os.Getenv(EpDbPassword)

	host, user, db, typ, err := parseCLIArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logger.Fatal(err)
	}

	_, dsn, err := buildDSN(typ, host, user, dbPassword, db)
	if err != nil {
		logger.Fatal(err)
	}

	sqlDB, err := NewDB(dsn, typ)
	if err != nil {
		logger.Fatalf("Failed to connect to source database: %v", err)
	}

	settings2.InitSettings(logger)
	var settings = settings2.Displayed

	// plan: first migrate the authors because they don't depend on anything else
	dbToSaveTo, err := utils.GetDB(settings, logger)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
		return
	}

	migrator := NewMigrator(sqlDB, dbToSaveTo, logger)
	if err := migrator.MigrateAuthors(); err != nil {
		logger.Fatalf("Failed to migrate authors: %v", err)
	}

	if err := migrator.MigratePads(); err != nil {
		logger.Fatalf("Failed to migrate pads: %v", err)
	}

	if err := migrator.MigrateRevisions(); err != nil {
		logger.Fatalf("Failed to migrate revisions: %v", err)
	}
}

func parseCLIArgs(
	args []string,
) (host, username, database, dbType string, err error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)

	// Flags
	fs.StringVar(&host, "host", "", "The database host to migrate from")
	fs.StringVar(&host, "h", "", "The database host to migrate from (shorthand)")

	fs.StringVar(&username, "username", "", "The username to use for authentication")
	fs.StringVar(&username, "u", "", "The username to use for authentication (shorthand)")

	fs.StringVar(&database, "database", "", "The database name to use")
	fs.StringVar(&database, "d", "", "The database name to use (shorthand)")

	fs.StringVar(
		&dbType,
		"type",
		"",
		"The database type: sqlite, mysql, postgres",
	)
	fs.StringVar(
		&dbType,
		"t",
		"",
		"The database type: sqlite, mysql, postgres (shorthand)",
	)

	// Positional host support
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		host = args[0]
		args = args[1:]
	}

	// Parse flags
	if err = fs.Parse(args); err != nil {
		return
	}

	// Validation
	switch dbType {
	case "sqlite", "mysql", "postgres":
		// ok
	case "":
		err = fmt.Errorf("database type is required (--type)")
		return
	default:
		err = fmt.Errorf("unsupported database type: %s", dbType)
		return
	}

	if database == "" && dbType != "sqlite" {
		err = fmt.Errorf("database name is required for %s", dbType)
		return
	}

	return
}

func buildDSN(
	dbType string,
	host string,
	user string,
	password string,
	database string,
) (driver string, dsn string, err error) {
	switch dbType {
	case "sqlite":
		if database == "" {
			return "", "", fmt.Errorf("sqlite requires a database file path")
		}
		return "sqlite", database, nil

	case "postgres":
		if host == "" || user == "" || database == "" {
			return "", "", fmt.Errorf("postgres requires host, user, and database")
		}

		var hostWithoutPort string
		var port int

		if strings.Contains(host, ":") {
			parts := strings.Split(host, ":")
			hostWithoutPort = parts[0]
			fmt.Sscanf(parts[1], "%d", &port)
		}

		dsn = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			hostWithoutPort,
			port,
			user,
			password,
			database,
		)
		return "postgres", dsn, nil

	case "mysql":
		if host == "" || user == "" || database == "" {
			return "", "", fmt.Errorf("mysql requires host, user, and database")
		}

		escapedUser := url.QueryEscape(user)
		escapedPass := url.QueryEscape(password)

		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4",
			escapedUser,
			escapedPass,
			host,
			database,
		)
		return "mysql", dsn, nil

	default:
		return "", "", fmt.Errorf("unsupported database type: %s", dbType)
	}
}
