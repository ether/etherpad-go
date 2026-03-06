package oidc

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/url"
	"time"

	db2 "github.com/ether/etherpad-go/lib/db"
	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"
)

const (
	oidcStateStorageKey      = "fosite_state"
	oidcSigningKeyStorageKey = "signing_key"
)

type storedRequester struct {
	ID                string                `json:"id"`
	RequestedAt       time.Time             `json:"requestedAt"`
	ClientID          string                `json:"clientId"`
	RequestedScope    []string              `json:"requestedScope"`
	GrantedScope      []string              `json:"grantedScope"`
	Form              url.Values            `json:"form"`
	RequestedAudience []string              `json:"requestedAudience"`
	GrantedAudience   []string              `json:"grantedAudience"`
	Session           openid.DefaultSession `json:"session"`
}

type storedAuthorizeCode struct {
	Active    bool            `json:"active"`
	Requester storedRequester `json:"requester"`
}

type storedRefreshToken struct {
	Active               bool            `json:"active"`
	AccessTokenSignature string          `json:"accessTokenSignature"`
	Requester            storedRequester `json:"requester"`
}

type storeSnapshot struct {
	AuthorizeCodes         map[string]storedAuthorizeCode `json:"authorizeCodes"`
	IDSessions             map[string]storedRequester     `json:"idSessions"`
	AccessTokens           map[string]storedRequester     `json:"accessTokens"`
	RefreshTokens          map[string]storedRefreshToken  `json:"refreshTokens"`
	PKCES                  map[string]storedRequester     `json:"pkces"`
	BlacklistedJTIs        map[string]time.Time           `json:"blacklistedJtis"`
	AccessTokenRequestIDs  map[string]string              `json:"accessTokenRequestIds"`
	RefreshTokenRequestIDs map[string]string              `json:"refreshTokenRequestIds"`
	IssuerPublicKeys       map[string]IssuerPublicKeys    `json:"issuerPublicKeys"`
	PARSessions            map[string]storedRequester     `json:"parSessions"`
}

func marshalPrivateKey(key *rsa.PrivateKey) (string, error) {
	if key == nil {
		return "", errors.New("private key is nil")
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return string(pem.EncodeToMemory(block)), nil
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("invalid PEM private key")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func requesterToStored(requester fosite.Requester) (storedRequester, error) {
	req, ok := requester.(*fosite.Request)
	if !ok {
		return storedRequester{}, errors.New("unsupported fosite requester type")
	}
	session, ok := req.GetSession().(*openid.DefaultSession)
	if !ok {
		return storedRequester{}, errors.New("unsupported fosite session type")
	}
	return storedRequester{
		ID:                req.GetID(),
		RequestedAt:       req.GetRequestedAt(),
		ClientID:          req.GetClient().GetID(),
		RequestedScope:    append([]string(nil), req.GetRequestedScopes()...),
		GrantedScope:      append([]string(nil), req.GetGrantedScopes()...),
		Form:              req.GetRequestForm(),
		RequestedAudience: append([]string(nil), req.GetRequestedAudience()...),
		GrantedAudience:   append([]string(nil), req.GetGrantedAudience()...),
		Session:           *session,
	}, nil
}

func (s *MemoryStore) storedToRequester(value storedRequester) (fosite.Requester, error) {
	client, ok := s.Clients[value.ClientID]
	if !ok {
		return nil, fosite.ErrNotFound.WithDebugf("client %s not found while restoring oidc state", value.ClientID)
	}
	req := fosite.NewRequest()
	req.ID = value.ID
	req.RequestedAt = value.RequestedAt
	req.Client = client
	req.Form = value.Form
	req.SetRequestedScopes(value.RequestedScope)
	for _, scope := range value.GrantedScope {
		req.GrantScope(scope)
	}
	req.SetRequestedAudience(value.RequestedAudience)
	for _, audience := range value.GrantedAudience {
		req.GrantAudience(audience)
	}
	session := value.Session
	req.Session = &session
	return req, nil
}

func (s *MemoryStore) saveSnapshot() error {
	if s.persistence == nil {
		return nil
	}
	snapshot, err := s.snapshot()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	return s.persistence.SetOIDCStorageValue(oidcStateStorageKey, string(raw))
}

func (s *MemoryStore) snapshot() (*storeSnapshot, error) {
	snapshot := &storeSnapshot{
		AuthorizeCodes:         make(map[string]storedAuthorizeCode, len(s.AuthorizeCodes)),
		IDSessions:             make(map[string]storedRequester, len(s.IDSessions)),
		AccessTokens:           make(map[string]storedRequester, len(s.AccessTokens)),
		RefreshTokens:          make(map[string]storedRefreshToken, len(s.RefreshTokens)),
		PKCES:                  make(map[string]storedRequester, len(s.PKCES)),
		BlacklistedJTIs:        make(map[string]time.Time, len(s.BlacklistedJTIs)),
		AccessTokenRequestIDs:  make(map[string]string, len(s.AccessTokenRequestIDs)),
		RefreshTokenRequestIDs: make(map[string]string, len(s.RefreshTokenRequestIDs)),
		IssuerPublicKeys:       make(map[string]IssuerPublicKeys, len(s.IssuerPublicKeys)),
		PARSessions:            make(map[string]storedRequester, len(s.PARSessions)),
	}

	for key, value := range s.AuthorizeCodes {
		requester, err := requesterToStored(value.Requester)
		if err != nil {
			return nil, err
		}
		snapshot.AuthorizeCodes[key] = storedAuthorizeCode{Active: value.active, Requester: requester}
	}
	for key, value := range s.IDSessions {
		requester, err := requesterToStored(value)
		if err != nil {
			return nil, err
		}
		snapshot.IDSessions[key] = requester
	}
	for key, value := range s.AccessTokens {
		requester, err := requesterToStored(value)
		if err != nil {
			return nil, err
		}
		snapshot.AccessTokens[key] = requester
	}
	for key, value := range s.RefreshTokens {
		requester, err := requesterToStored(value.Requester)
		if err != nil {
			return nil, err
		}
		snapshot.RefreshTokens[key] = storedRefreshToken{
			Active:               value.active,
			AccessTokenSignature: value.accessTokenSignature,
			Requester:            requester,
		}
	}
	for key, value := range s.PKCES {
		requester, err := requesterToStored(value)
		if err != nil {
			return nil, err
		}
		snapshot.PKCES[key] = requester
	}
	for key, value := range s.PARSessions {
		requester, err := requesterToStored(value)
		if err != nil {
			return nil, err
		}
		snapshot.PARSessions[key] = requester
	}
	for key, value := range s.BlacklistedJTIs {
		snapshot.BlacklistedJTIs[key] = value
	}
	for key, value := range s.AccessTokenRequestIDs {
		snapshot.AccessTokenRequestIDs[key] = value
	}
	for key, value := range s.RefreshTokenRequestIDs {
		snapshot.RefreshTokenRequestIDs[key] = value
	}
	for key, value := range s.IssuerPublicKeys {
		snapshot.IssuerPublicKeys[key] = value
	}
	return snapshot, nil
}

func (s *MemoryStore) loadSnapshot() error {
	if s.persistence == nil {
		return nil
	}
	raw, err := s.persistence.GetOIDCStorageValue(oidcStateStorageKey)
	if err != nil || raw == nil || *raw == "" {
		return err
	}
	var snapshot storeSnapshot
	if err := json.Unmarshal([]byte(*raw), &snapshot); err != nil {
		return err
	}

	for key, value := range snapshot.AuthorizeCodes {
		requester, err := s.storedToRequester(value.Requester)
		if err != nil {
			return err
		}
		s.AuthorizeCodes[key] = StoreAuthorizeCode{active: value.Active, Requester: requester}
	}
	for key, value := range snapshot.IDSessions {
		requester, err := s.storedToRequester(value)
		if err != nil {
			return err
		}
		s.IDSessions[key] = requester
	}
	for key, value := range snapshot.AccessTokens {
		requester, err := s.storedToRequester(value)
		if err != nil {
			return err
		}
		s.AccessTokens[key] = requester
	}
	for key, value := range snapshot.RefreshTokens {
		requester, err := s.storedToRequester(value.Requester)
		if err != nil {
			return err
		}
		s.RefreshTokens[key] = StoreRefreshToken{
			active:               value.Active,
			accessTokenSignature: value.AccessTokenSignature,
			Requester:            requester,
		}
	}
	for key, value := range snapshot.PKCES {
		requester, err := s.storedToRequester(value)
		if err != nil {
			return err
		}
		s.PKCES[key] = requester
	}
	for key, value := range snapshot.PARSessions {
		requester, err := s.storedToRequester(value)
		if err != nil {
			return err
		}
		s.PARSessions[key] = requester.(fosite.AuthorizeRequester)
	}
	s.BlacklistedJTIs = snapshot.BlacklistedJTIs
	s.AccessTokenRequestIDs = snapshot.AccessTokenRequestIDs
	s.RefreshTokenRequestIDs = snapshot.RefreshTokenRequestIDs
	s.IssuerPublicKeys = snapshot.IssuerPublicKeys
	return nil
}

func loadOrCreatePrivateKey(persistence db2.DataStore, generated *rsa.PrivateKey) (*rsa.PrivateKey, error) {
	if persistence == nil {
		return generated, nil
	}
	raw, err := persistence.GetOIDCStorageValue(oidcSigningKeyStorageKey)
	if err != nil {
		return nil, err
	}
	if raw != nil && *raw != "" {
		return parsePrivateKey(*raw)
	}
	encoded, err := marshalPrivateKey(generated)
	if err != nil {
		return nil, err
	}
	if err := persistence.SetOIDCStorageValue(oidcSigningKeyStorageKey, encoded); err != nil {
		return nil, err
	}
	return generated, nil
}
