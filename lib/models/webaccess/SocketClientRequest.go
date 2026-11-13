package webaccess

type SocketClientRequest struct {
	IsAdmin           bool
	username          string
	PadAuthorizations *map[string]string
	ReadOnly          *bool
	Username          *string
	// CanCreate is nil unless the user's settings say; only an explicit
	// false keeps the user from creating pads.
	CanCreate *bool
}
