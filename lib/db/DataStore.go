package db

import (
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
)

type OAuthTokenRow struct {
	Signature     string
	ClientID      string
	RequestID     string
	Scopes        string
	GrantedScopes string
	FormData      string
	SessionData   string
	Active        bool
	RequestedAt   time.Time
	ExpiresAt     time.Time
}

type OAuthRefreshTokenRow struct {
	OAuthTokenRow
	AccessTokenSignature string
}

type PadMethods interface {
	DoesPadExist(padID string) (*bool, error)
	RemovePad(padID string) error
	CreatePad(padID string, padDB db.PadDB) error
	GetPadIds() (*[]string, error)
	SaveRevision(padId string, rev int, changeset string, text db.AText, pool db.RevPool, authorId *string, timestamp int64) error
	GetRevision(padId string, rev int) (*db.PadSingleRevision, error)
	RemoveRevisionsOfPad(padId string) error
	GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error)
	GetPad(padID string) (*db.PadDB, error)
	GetReadonlyPad(padId string) (*string, error)
	SetReadOnlyId(padId string, readOnlyId string) error
	GetPadByReadOnlyId(id string) (*string, error)
	SaveChatHeadOfPad(padId string, head int) error
	QueryPad(offset int, limit int, sortBy string, ascending bool, pattern string) (*db.PadDBSearchResult, error)
}

type AuthorMethods interface {
	GetAuthor(author string) (*db.AuthorDB, error)
	GetPadIdsOfAuthor(authorId string) (*[]string, error)
	GetAuthorByToken(token string) (*string, error)
	SetAuthorByToken(token string, author string) error
	SaveAuthor(author db.AuthorDB) error
	SaveAuthorName(authorId string, authorName string) error
	SaveAuthorColor(authorId string, authorColor string) error
	GetAuthors(ids []string) (*[]db.AuthorDB, error)
	// RemoveTokenOfAuthor severs the token binding that links a person to the
	// given author id (GDPR erasure). It is a no-op if the author does not
	// exist or has no token.
	RemoveTokenOfAuthor(authorId string) error
}

type SessionMethods interface {
	GetSessionById(sessionID string) (*session2.Session, error)
	SetSessionById(sessionID string, session session2.Session) error
	RemoveSessionById(sessionID string) error
}

type GroupMethods interface {
	GetGroup(groupId string) (*string, error)
	GetGroups() (*[]string, error)
	SaveGroup(groupId string) error
	RemoveGroup(groupId string) error
}

type ChatMethods interface {
	RemoveChat(padId string) error
	SaveChatMessage(padId string, head int, authorId *string, timestamp int64, text string) error
	GetChatsOfPad(padId string, start int, end int) (*[]db.ChatMessageDBWithDisplayName, error)
	GetAuthorIdsOfPadChats(id string) (*[]string, error)
	// ClearChatAuthorship nulls the authorship of all chat messages posted by
	// the given author while preserving the messages themselves (GDPR erasure).
	ClearChatAuthorship(authorId string) error
}

type ServerMethods interface {
	GetServerVersion() (*db.ServerVersion, error)
	SaveServerVersion(version string) error
}

type OIDCMethods interface {
	// Existing key-value methods (keep for signing key storage)
	GetOIDCStorageValue(key string) (*string, error)
	SetOIDCStorageValue(key string, payload string) error
	DeleteOIDCStorageValue(key string) error

	// Access tokens
	CreateAccessToken(signature, clientID, requestID, scopes, grantedScopes, formData, sessionData string, requestedAt, expiresAt time.Time) error
	GetAccessToken(signature string) (*OAuthTokenRow, error)
	DeleteAccessToken(signature string) error
	DeleteAccessTokensByRequestID(requestID string) error

	// Refresh tokens
	CreateRefreshToken(signature, clientID, requestID, scopes, grantedScopes, formData, sessionData string, active bool, accessTokenSignature string, requestedAt, expiresAt time.Time) error
	GetRefreshToken(signature string) (*OAuthRefreshTokenRow, error)
	DeleteRefreshToken(signature string) error
	RevokeRefreshToken(signature string) error
	RevokeRefreshTokensByRequestID(requestID string) error

	// Auth codes
	CreateAuthCode(signature, clientID, requestID, scopes, grantedScopes, formData, sessionData string, requestedAt, expiresAt time.Time) error
	GetAuthCode(signature string) (*OAuthTokenRow, error)
	InvalidateAuthCode(signature string) error

	// PKCE
	CreatePKCE(signature, clientID, requestID, scopes, grantedScopes, formData, sessionData string, requestedAt, expiresAt time.Time) error
	GetPKCE(signature string) (*OAuthTokenRow, error)
	DeletePKCE(signature string) error

	// OIDC Sessions
	CreateOIDCSession(signature, clientID, requestID, scopes, grantedScopes, formData, sessionData string, requestedAt, expiresAt time.Time) error
	GetOIDCSession(signature string) (*OAuthTokenRow, error)
	DeleteOIDCSession(signature string) error
}

// SecretMethods back the SecretRotator. Each published parameter set is
// addressed by a random id within a rotator namespace (prefix), allowing
// several rotators (and several Etherpad instances) to coexist in one table.
type SecretMethods interface {
	// SaveSecretParams upserts one published parameter set.
	SaveSecretParams(id string, prefix string, payload string) error
	// ListSecretParams returns all parameter sets for the given prefix as a
	// map of id -> payload.
	ListSecretParams(prefix string) (map[string]string, error)
	// DeleteSecretParams removes a single parameter set by id.
	DeleteSecretParams(id string) error
}

// SheetMethods persist spreadsheet documents (header snapshot + op-log),
// keyed by pad id (a sheet document is a pad with document_type "sheet").
type SheetMethods interface {
	SaveSheet(padId string, head int, snapshot string) error
	GetSheet(padId string) (*db.SheetDB, error)
	DoesSheetExist(padId string) (*bool, error)
	RemoveSheet(padId string) error
	SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error
	GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error)
}

type DataStore interface {
	PadMethods
	AuthorMethods
	SessionMethods
	GroupMethods
	ChatMethods
	ServerMethods
	OIDCMethods
	SecretMethods
	SheetMethods
	Close() error
	Ping() error
}
