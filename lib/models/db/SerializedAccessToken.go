package db

import (
	"encoding/json"
	"time"

	"github.com/ory/fosite"
)

type SerializedAccessToken struct {
	RequestID     string
	ClientID      string
	Scopes        []string
	GrantedScopes []string
	RequestedAt   time.Time
	SessionData   json.RawMessage
}

func FromFositeRequester(request fosite.Requester) (*SerializedAccessToken, error) {
	session := request.GetSession().(*fosite.DefaultSession)
	sessStr, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}

	return &SerializedAccessToken{
		RequestID:     request.GetID(),
		ClientID:      request.GetClient().GetID(),
		Scopes:        request.GetRequestedScopes(),
		GrantedScopes: request.GetGrantedScopes(),
		RequestedAt:   request.GetRequestedAt(),
		SessionData:   sessStr,
	}, nil
}
