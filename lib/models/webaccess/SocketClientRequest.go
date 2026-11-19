package webaccess

import "github.com/ether/etherpad-go/lib/accounts"

type SocketClientRequest struct {
	IsAdmin           bool
	username          string
	PadAuthorizations *map[string]string
	ReadOnly          *bool
	Username          *string
	// CanCreate is nil unless the user's settings say; only an explicit
	// false keeps the user from creating pads.
	CanCreate *bool
	// AccountID is the account of a person logged in with one; "" for a
	// person a plugin authenticated without an account.
	AccountID string
	// Grants are what the user may do (lib/accounts); nil when made by
	// code that does not resolve them.
	Grants *accounts.Grants
}
