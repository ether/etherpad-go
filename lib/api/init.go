package api

import (
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/io"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/locales"
)

func InitAPI(store *lib.InitStore) *oidc.Authenticator {
	locales.Init(store)
	author.Init(store)
	pad.Init(store)
	groups.Init(store)
	static.Init(store)
	io.Init(store)
	return oidc.Init(store)
}
