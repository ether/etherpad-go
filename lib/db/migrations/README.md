# Database Migrations

This directory contains the versioned SQL migration system for the Etherpad Go database.

## How Migrations Work

Migrations are applied automatically when the database connection is established. Each migration has a version number and is only applied once. The applied migrations are tracked in the `schema_migrations` table.

## Adding a New Migration

1. Create a new file with the naming convention `XXX_description.go` where `XXX` is the next version number (e.g., `002_add_new_column.go`).

2. Define a migration function following this pattern:

```go
package migrations

import (
    "database/sql"
)

func migration002AddNewColumn() Migration {
    return Migration{
        Version:     2,
        Description: "Add new column to table",
        Up: func(db *sql.DB, dialect Dialect) error {
            var query string
            switch dialect {
            case DialectMySQL:
                query = "ALTER TABLE mytable ADD COLUMN newcol VARCHAR(255)"
            case DialectPostgres:
                query = "ALTER TABLE mytable ADD COLUMN newcol TEXT"
            default: // SQLite
                query = "ALTER TABLE mytable ADD COLUMN newcol TEXT"
            }
            _, err := db.Exec(query)
            return err
        },
    }
}
```

3. Register the migration in `001_initial_schema.go` by adding it to the `GetMigrations()` function:

```go
func GetMigrations() []Migration {
    return []Migration{
        migration001InitialSchema(),
        migration002AddNewColumn(), // Add your new migration here
    }
}
```

## Supported Dialects

- `DialectSQLite` - SQLite database
- `DialectPostgres` - PostgreSQL database  
- `DialectMySQL` - MySQL database

## Important Notes

- Migrations are applied in version order (ascending).
- Each migration should be idempotent when possible (use `IF NOT EXISTS`, `IF EXISTS`, etc.).
- Once a migration is deployed to production, it should never be modified. Create a new migration instead.
- Always test migrations with all supported database dialects before deploying.
- The migration system uses a `schema_migrations` table to track which migrations have been applied.

## Schema Migrations Table

The migration system automatically creates a `schema_migrations` table with the following structure:

| Column      | Type      | Description                              |
|-------------|-----------|------------------------------------------|
| version     | INTEGER   | The migration version number (primary key) |
| description | TEXT      | A description of the migration           |
| applied_at  | TIMESTAMP | When the migration was applied           |

