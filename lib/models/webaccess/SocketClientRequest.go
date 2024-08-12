package webaccess

type SocketClientRequest struct {
	IsAdmin           bool
	username          string
	PadAuthorizations *map[string]string
	ReadOnly          *bool
	Username          *string
	CanCreate         bool
}
