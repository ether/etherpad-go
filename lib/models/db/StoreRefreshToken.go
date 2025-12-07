package db

import "github.com/ory/fosite"

type StoreRefreshToken struct {
	Active               bool
	AccessTokenSignature string
	fosite.Requester
}
