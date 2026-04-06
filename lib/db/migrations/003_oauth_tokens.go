package migrations

import "database/sql"

func migration004OAuthTokens() Migration {
	return Migration{
		Version:     4,
		Description: "Create dedicated OAuth token tables",
		Up: func(db *sql.DB, dialect Dialect) error {
			tables := []string{
				oauthAccessTokensTable(dialect),
				oauthRefreshTokensTable(dialect),
				oauthAuthCodesTable(dialect),
				oauthPKCETable(dialect),
				oauthOIDCSessionsTable(dialect),
			}
			for _, q := range tables {
				if _, err := db.Exec(q); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func oauthAccessTokensTable(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return `CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			signature VARCHAR(512) PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	case DialectPostgres:
		return `CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	default:
		return `CREATE TABLE IF NOT EXISTS oauth_access_tokens (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
}

func oauthRefreshTokensTable(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return `CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
			signature VARCHAR(512) PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			active BOOLEAN DEFAULT TRUE,
			access_token_signature TEXT,
			requested_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	case DialectPostgres:
		return `CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			active BOOLEAN DEFAULT TRUE,
			access_token_signature TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	default:
		return `CREATE TABLE IF NOT EXISTS oauth_refresh_tokens (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			active BOOLEAN DEFAULT TRUE,
			access_token_signature TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
}

func oauthAuthCodesTable(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return `CREATE TABLE IF NOT EXISTS oauth_auth_codes (
			signature VARCHAR(512) PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			active BOOLEAN DEFAULT TRUE,
			requested_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	case DialectPostgres:
		return `CREATE TABLE IF NOT EXISTS oauth_auth_codes (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			active BOOLEAN DEFAULT TRUE,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	default:
		return `CREATE TABLE IF NOT EXISTS oauth_auth_codes (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			active BOOLEAN DEFAULT TRUE,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
}

func oauthPKCETable(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return `CREATE TABLE IF NOT EXISTS oauth_pkce (
			signature VARCHAR(512) PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	case DialectPostgres:
		return `CREATE TABLE IF NOT EXISTS oauth_pkce (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	default:
		return `CREATE TABLE IF NOT EXISTS oauth_pkce (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
}

func oauthOIDCSessionsTable(dialect Dialect) string {
	switch dialect {
	case DialectMySQL:
		return `CREATE TABLE IF NOT EXISTS oauth_oidc_sessions (
			signature VARCHAR(512) PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at DATETIME,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`
	case DialectPostgres:
		return `CREATE TABLE IF NOT EXISTS oauth_oidc_sessions (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	default:
		return `CREATE TABLE IF NOT EXISTS oauth_oidc_sessions (
			signature TEXT PRIMARY KEY,
			client_id TEXT NOT NULL,
			request_id TEXT NOT NULL,
			scopes TEXT,
			granted_scopes TEXT,
			form_data TEXT,
			session_data TEXT,
			requested_at TIMESTAMP,
			expires_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
}
