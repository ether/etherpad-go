package webaccess

import (
	"github.com/ether/etherpad-go/lib/settings"
)

type WebAccessType struct {
	User     *SocketClientRequest
	Users    map[string]settings.User
	Next     func() error
	Username *string
	Password *string
}
