package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// Storage implements op.Storage for zitadel/oidc. Auth requests, codes and
// tokens live in the datastore's oauth_* tables, so several Etherpad
// instances on one database share them. Each row carries its state as JSON
// in session_data; the other columns are only keys.
type Storage struct {
	db      db.DataStore
	clients map[string]*Client
	users   map[string]user
	key     *signingKey
	now     func() time.Time
}

type user struct {
	Username string
	Password string
	Admin    bool
}

var errNotFound = errors.New("not found")

func newStorage(store db.DataStore, clients map[string]*Client, users map[string]user, key *rsa.PrivateKey) *Storage {
	return &Storage{
		db:      store,
		clients: clients,
		users:   users,
		key:     newSigningKey(key),
		now:     time.Now,
	}
}

// --- auth requests -------------------------------------------------------

type codeChallenge struct {
	Challenge string `json:"challenge"`
	Method    string `json:"method"`
}

// authRequest is an authorization request from /oauth2/auth. It is created
// before the user logs in and marked done by the login page.
type authRequest struct {
	ID            string            `json:"id"`
	ClientID      string            `json:"clientId"`
	RedirectURI   string            `json:"redirectUri"`
	State         string            `json:"state"`
	Nonce         string            `json:"nonce"`
	Scopes        []string          `json:"scopes"`
	ResponseType  oidc.ResponseType `json:"responseType"`
	ResponseMode  oidc.ResponseMode `json:"responseMode"`
	CodeChallenge *codeChallenge    `json:"codeChallenge,omitempty"`
	UserID        string            `json:"userId"`
	IsDone        bool              `json:"done"`
	AuthTime      time.Time         `json:"authTime"`
	ExpiresAt     time.Time         `json:"expiresAt"`
}

func (a *authRequest) GetID() string          { return a.ID }
func (a *authRequest) GetACR() string         { return "" }
func (a *authRequest) GetAudience() []string  { return []string{a.ClientID} }
func (a *authRequest) GetAuthTime() time.Time { return a.AuthTime }
func (a *authRequest) GetClientID() string    { return a.ClientID }
func (a *authRequest) GetNonce() string       { return a.Nonce }
func (a *authRequest) GetRedirectURI() string { return a.RedirectURI }
func (a *authRequest) GetResponseType() oidc.ResponseType {
	return a.ResponseType
}
func (a *authRequest) GetResponseMode() oidc.ResponseMode {
	return a.ResponseMode
}
func (a *authRequest) GetScopes() []string { return a.Scopes }
func (a *authRequest) GetState() string    { return a.State }
func (a *authRequest) GetSubject() string  { return a.UserID }
func (a *authRequest) Done() bool          { return a.IsDone }

func (a *authRequest) GetAMR() []string {
	if a.IsDone {
		return []string{"pwd"}
	}
	return nil
}

func (a *authRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.CodeChallenge == nil {
		return nil
	}
	return &oidc.CodeChallenge{
		Challenge: a.CodeChallenge.Challenge,
		Method:    oidc.CodeChallengeMethod(a.CodeChallenge.Method),
	}
}

// normalizeScopes maps the legacy "offline" scope to offline_access, which
// zitadel/oidc requires for refresh tokens.
func normalizeScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if s == legacyOfflineScope {
			s = oidc.ScopeOfflineAccess
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func (s *Storage) CreateAuthRequest(_ context.Context, req *oidc.AuthRequest, userID string) (op.AuthRequest, error) {
	now := s.now()
	ar := &authRequest{
		ID:           uuid.NewString(),
		ClientID:     req.ClientID,
		RedirectURI:  req.RedirectURI,
		State:        req.State,
		Nonce:        req.Nonce,
		Scopes:       normalizeScopes(req.Scopes),
		ResponseType: req.ResponseType,
		ResponseMode: req.ResponseMode,
		UserID:       userID,
		ExpiresAt:    now.Add(authRequestLifetime),
	}
	if req.CodeChallenge != "" {
		ar.CodeChallenge = &codeChallenge{Challenge: req.CodeChallenge, Method: string(req.CodeChallengeMethod)}
	}
	if err := s.putAuthRequest(ar); err != nil {
		return nil, err
	}
	return ar, nil
}

func (s *Storage) putAuthRequest(ar *authRequest) error {
	data, err := json.Marshal(ar)
	if err != nil {
		return err
	}
	_ = s.db.DeleteOIDCSession(ar.ID)
	return s.db.CreateOIDCSession(ar.ID, ar.ClientID, ar.ID, "", "", "", string(data), s.now(), ar.ExpiresAt)
}

func (s *Storage) getAuthRequest(id string) (*authRequest, error) {
	if id == "" {
		return nil, errNotFound
	}
	row, err := s.db.GetOIDCSession(id)
	if err != nil || row == nil {
		return nil, errNotFound
	}
	var ar authRequest
	if err := json.Unmarshal([]byte(row.SessionData), &ar); err != nil {
		return nil, err
	}
	if s.now().After(ar.ExpiresAt) {
		_ = s.db.DeleteOIDCSession(id)
		return nil, errNotFound
	}
	return &ar, nil
}

func (s *Storage) AuthRequestByID(_ context.Context, id string) (op.AuthRequest, error) {
	return s.getAuthRequest(id)
}

func (s *Storage) AuthRequestByCode(_ context.Context, code string) (op.AuthRequest, error) {
	row, err := s.db.GetAuthCode(hashToken(code))
	if err != nil || row == nil {
		return nil, errNotFound
	}
	// The auth request is deleted once the code was exchanged, which makes
	// the code single-use.
	return s.getAuthRequest(row.RequestID)
}

func (s *Storage) SaveAuthCode(_ context.Context, id string, code string) error {
	ar, err := s.getAuthRequest(id)
	if err != nil {
		return err
	}
	return s.db.CreateAuthCode(hashToken(code), ar.ClientID, ar.ID, "", "", "", "", s.now(), ar.ExpiresAt)
}

func (s *Storage) DeleteAuthRequest(_ context.Context, id string) error {
	return s.db.DeleteOIDCSession(id)
}

// completeAuthRequest marks the auth request as authenticated by username.
func (s *Storage) completeAuthRequest(id, username string) error {
	ar, err := s.getAuthRequest(id)
	if err != nil {
		return err
	}
	ar.UserID = username
	ar.IsDone = true
	ar.AuthTime = s.now()
	return s.putAuthRequest(ar)
}

// checkPassword reports whether username/password match a configured user.
func (s *Storage) checkPassword(username, password string) bool {
	u, ok := s.users[username]
	if !ok || u.Password == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(u.Password), []byte(password)) == 1
}

// --- tokens ----------------------------------------------------------------

type accessToken struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"clientId"`
	Subject        string    `json:"subject"`
	Audience       []string  `json:"audience"`
	Scopes         []string  `json:"scopes"`
	RefreshTokenID string    `json:"refreshTokenId,omitempty"`
	IssuedAt       time.Time `json:"issuedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

type refreshToken struct {
	ID            string    `json:"id"`
	ClientID      string    `json:"clientId"`
	Subject       string    `json:"subject"`
	Audience      []string  `json:"audience"`
	Scopes        []string  `json:"scopes"`
	AMR           []string  `json:"amr"`
	AuthTime      time.Time `json:"authTime"`
	AccessTokenID string    `json:"accessTokenId"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// refreshTokenRequest implements op.RefreshTokenRequest.
type refreshTokenRequest struct {
	*refreshToken
}

func (r *refreshTokenRequest) GetAMR() []string            { return r.AMR }
func (r *refreshTokenRequest) GetAudience() []string       { return r.Audience }
func (r *refreshTokenRequest) GetAuthTime() time.Time      { return r.AuthTime }
func (r *refreshTokenRequest) GetClientID() string         { return r.ClientID }
func (r *refreshTokenRequest) GetScopes() []string         { return r.Scopes }
func (r *refreshTokenRequest) GetSubject() string          { return r.Subject }
func (r *refreshTokenRequest) SetCurrentScopes(s []string) { r.Scopes = s }

// clientCredentialsRequest implements op.TokenRequest for the
// client_credentials grant; the client is its own subject.
type clientCredentialsRequest struct {
	clientID string
	scopes   []string
}

func (c *clientCredentialsRequest) GetSubject() string    { return c.clientID }
func (c *clientCredentialsRequest) GetAudience() []string { return []string{c.clientID} }
func (c *clientCredentialsRequest) GetScopes() []string   { return c.scopes }

// hashToken is the key under which a secret token (refresh token, auth
// code) is stored, so a database dump does not hand out usable tokens.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func requestInfo(req op.TokenRequest) (clientID string, authTime time.Time, amr []string) {
	switch r := req.(type) {
	case *authRequest:
		return r.ClientID, r.AuthTime, r.GetAMR()
	case *refreshTokenRequest:
		return r.ClientID, r.AuthTime, r.AMR
	case *clientCredentialsRequest:
		return r.clientID, time.Time{}, nil
	}
	return "", time.Time{}, nil
}

func (s *Storage) newAccessToken(clientID, refreshTokenID string, req op.TokenRequest) (*accessToken, error) {
	now := s.now()
	t := &accessToken{
		ID:             uuid.NewString(),
		ClientID:       clientID,
		Subject:        req.GetSubject(),
		Audience:       req.GetAudience(),
		Scopes:         req.GetScopes(),
		RefreshTokenID: refreshTokenID,
		IssuedAt:       now,
		ExpiresAt:      now.Add(accessTokenLifetime),
	}
	data, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	if err := s.db.CreateAccessToken(t.ID, clientID, refreshTokenID, "", "", "", string(data), now, t.ExpiresAt); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Storage) getAccessToken(id string) (*accessToken, error) {
	if id == "" {
		return nil, errNotFound
	}
	row, err := s.db.GetAccessToken(id)
	if err != nil || row == nil {
		return nil, errNotFound
	}
	var t accessToken
	if err := json.Unmarshal([]byte(row.SessionData), &t); err != nil {
		return nil, err
	}
	if s.now().After(t.ExpiresAt) {
		_ = s.db.DeleteAccessToken(id)
		return nil, errNotFound
	}
	return &t, nil
}

func (s *Storage) getRefreshToken(id string) (*refreshToken, error) {
	if id == "" {
		return nil, errNotFound
	}
	row, err := s.db.GetRefreshToken(id)
	if err != nil || row == nil || !row.Active {
		return nil, errNotFound
	}
	var t refreshToken
	if err := json.Unmarshal([]byte(row.SessionData), &t); err != nil {
		return nil, err
	}
	if s.now().After(t.ExpiresAt) {
		_ = s.db.DeleteRefreshToken(id)
		return nil, errNotFound
	}
	return &t, nil
}

func (s *Storage) CreateAccessToken(_ context.Context, req op.TokenRequest) (string, time.Time, error) {
	clientID, _, _ := requestInfo(req)
	t, err := s.newAccessToken(clientID, "", req)
	if err != nil {
		return "", time.Time{}, err
	}
	return t.ID, t.ExpiresAt, nil
}

func (s *Storage) CreateAccessAndRefreshTokens(_ context.Context, req op.TokenRequest, currentRefreshToken string) (string, string, time.Time, error) {
	clientID, authTime, amr := requestInfo(req)

	if currentRefreshToken != "" {
		// Rotation: the used refresh token and its access token go away.
		oldID := hashToken(currentRefreshToken)
		old, err := s.getRefreshToken(oldID)
		if err != nil {
			return "", "", time.Time{}, op.ErrInvalidRefreshToken
		}
		_ = s.db.DeleteRefreshToken(oldID)
		_ = s.db.DeleteAccessToken(old.AccessTokenID)
	}

	token, err := randomToken()
	if err != nil {
		return "", "", time.Time{}, err
	}
	refreshID := hashToken(token)
	access, err := s.newAccessToken(clientID, refreshID, req)
	if err != nil {
		return "", "", time.Time{}, err
	}
	rt := &refreshToken{
		ID:            refreshID,
		ClientID:      clientID,
		Subject:       req.GetSubject(),
		Audience:      req.GetAudience(),
		Scopes:        req.GetScopes(),
		AMR:           amr,
		AuthTime:      authTime,
		AccessTokenID: access.ID,
		ExpiresAt:     s.now().Add(refreshTokenLifetime),
	}
	data, err := json.Marshal(rt)
	if err != nil {
		return "", "", time.Time{}, err
	}
	if err := s.db.CreateRefreshToken(refreshID, clientID, refreshID, "", "", "", string(data), true, access.ID, s.now(), rt.ExpiresAt); err != nil {
		return "", "", time.Time{}, err
	}
	return access.ID, token, access.ExpiresAt, nil
}

func (s *Storage) TokenRequestByRefreshToken(_ context.Context, token string) (op.RefreshTokenRequest, error) {
	rt, err := s.getRefreshToken(hashToken(token))
	if err != nil {
		return nil, op.ErrInvalidRefreshToken
	}
	return &refreshTokenRequest{rt}, nil
}

// TerminateSession is called by the end_session endpoint. The tables have
// no index by user, so the tokens simply expire.
func (s *Storage) TerminateSession(context.Context, string, string) error { return nil }

func (s *Storage) GetRefreshTokenInfo(_ context.Context, clientID string, token string) (string, string, error) {
	id := hashToken(token)
	rt, err := s.getRefreshToken(id)
	if err != nil || rt.ClientID != clientID {
		return "", "", op.ErrInvalidRefreshToken
	}
	return rt.Subject, id, nil
}

func (s *Storage) RevokeToken(_ context.Context, tokenOrID string, _ string, clientID string) *oidc.Error {
	if t, err := s.getAccessToken(tokenOrID); err == nil {
		if t.ClientID != clientID {
			return oidc.ErrInvalidClient().WithDescription("token was not issued for this client")
		}
		_ = s.db.DeleteAccessToken(t.ID)
		return nil
	}
	if rt, err := s.getRefreshToken(tokenOrID); err == nil {
		if rt.ClientID != clientID {
			return oidc.ErrInvalidClient().WithDescription("token was not issued for this client")
		}
		_ = s.db.DeleteRefreshToken(rt.ID)
		_ = s.db.DeleteAccessToken(rt.AccessTokenID)
	}
	// Unknown tokens are no error (RFC 7009).
	return nil
}

// --- keys ------------------------------------------------------------------

type signingKey struct {
	id  string
	key *rsa.PrivateKey
}

// newSigningKey derives a stable key ID from the public key, so it stays
// the same across restarts and instances sharing the persisted key.
func newSigningKey(key *rsa.PrivateKey) *signingKey {
	jwk := jose.JSONWebKey{Key: &key.PublicKey}
	thumb, err := jwk.Thumbprint(crypto.SHA256)
	id := "etherpad"
	if err == nil {
		id = base64.RawURLEncoding.EncodeToString(thumb)
	}
	return &signingKey{id: id, key: key}
}

func (k *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *signingKey) Key() any                                    { return k.key }
func (k *signingKey) ID() string                                  { return k.id }

type publicKey struct{ *signingKey }

func (k publicKey) Algorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k publicKey) Use() string                        { return "sig" }
func (k publicKey) Key() any                           { return &k.key.PublicKey }

func (s *Storage) SigningKey(context.Context) (op.SigningKey, error) { return s.key, nil }

func (s *Storage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}

func (s *Storage) KeySet(context.Context) ([]op.Key, error) {
	return []op.Key{publicKey{s.key}}, nil
}

// --- clients and users -------------------------------------------------------

func (s *Storage) GetClientByClientID(_ context.Context, clientID string) (op.Client, error) {
	c, ok := s.clients[clientID]
	if !ok {
		return nil, fmt.Errorf("client %q: %w", clientID, errNotFound)
	}
	return c, nil
}

func (s *Storage) AuthorizeClientIDSecret(_ context.Context, clientID, clientSecret string) error {
	c, ok := s.clients[clientID]
	if !ok || !c.checkSecret(clientSecret) {
		return errors.New("invalid client credentials")
	}
	return nil
}

func (s *Storage) ClientCredentials(_ context.Context, clientID, clientSecret string) (op.Client, error) {
	c, ok := s.clients[clientID]
	if !ok || !c.confidential || !c.checkSecret(clientSecret) {
		return nil, errors.New("invalid client credentials")
	}
	return c, nil
}

func (s *Storage) ClientCredentialsTokenRequest(_ context.Context, clientID string, scopes []string) (op.TokenRequest, error) {
	return &clientCredentialsRequest{clientID: clientID, scopes: scopes}, nil
}

func (s *Storage) setUserinfo(info *oidc.UserInfo, subject string, scopes []string) {
	info.Subject = subject
	// ValidateAdminToken checks this claim in the ID token.
	u, known := s.users[subject]
	info.AppendClaims("admin", known && u.Admin)
	if known && slices.Contains(scopes, oidc.ScopeProfile) {
		info.PreferredUsername = u.Username
		info.Name = u.Username
	}
}

func (s *Storage) SetUserinfoFromScopes(_ context.Context, info *oidc.UserInfo, userID, _ string, scopes []string) error {
	s.setUserinfo(info, userID, scopes)
	return nil
}

func (s *Storage) SetUserinfoFromToken(_ context.Context, info *oidc.UserInfo, tokenID, _, _ string) error {
	t, err := s.getAccessToken(tokenID)
	if err != nil {
		return err
	}
	s.setUserinfo(info, t.Subject, t.Scopes)
	return nil
}

func (s *Storage) SetIntrospectionFromToken(_ context.Context, resp *oidc.IntrospectionResponse, tokenID, _, _ string) error {
	t, err := s.getAccessToken(tokenID)
	if err != nil {
		return err
	}
	resp.Scope = t.Scopes
	resp.ClientID = t.ClientID
	resp.Subject = t.Subject
	resp.Audience = t.Audience
	resp.Expiration = oidc.FromTime(t.ExpiresAt)
	resp.IssuedAt = oidc.FromTime(t.IssuedAt)
	resp.TokenType = oidc.BearerToken
	if u, ok := s.users[t.Subject]; ok {
		resp.Username = u.Username
	}
	return nil
}

// GetPrivateClaimsFromScopes is only used for JWT access tokens, which
// Etherpad does not issue.
func (s *Storage) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}

// The JWT profile grant and private_key_jwt client authentication are
// disabled, so these are never asked for a key.
func (s *Storage) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, errors.New("jwt profile is not supported")
}

func (s *Storage) ValidateJWTProfileScopes(context.Context, string, []string) ([]string, error) {
	return nil, errors.New("jwt profile is not supported")
}

func (s *Storage) Health(context.Context) error { return nil }

var (
	_ op.Storage                  = (*Storage)(nil)
	_ op.ClientCredentialsStorage = (*Storage)(nil)
)
