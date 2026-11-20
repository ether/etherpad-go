package legacy

import (
	"strings"

	"github.com/ether/etherpad-go/lib/accounts"
)

// need is one right a call needs: scope on the pad named by the parameter
// pad, or for all pads where pad is "".
type need struct {
	scope string
	pad   string
}

// padRead, padWrite… are the functions on one pad (parameter padID) and
// what they need of it; everything else needs admin:pads for all pads
// (groups, authors, sessions, the lists of all pads, statistics), as
// before the tokens every key could. Read-only ids (r.…) are not pads
// here: a call with one needs admin:pads.
var padFunctions = map[string][]string{
	// reading
	"getText": {accounts.ScopePadRead}, "getHTML": {accounts.ScopePadRead},
	"getRevisionsCount": {accounts.ScopePadRead}, "getLastEdited": {accounts.ScopePadRead},
	"getReadOnlyID": {accounts.ScopePadRead}, "getPublicStatus": {accounts.ScopePadRead},
	"listAuthorsOfPad": {accounts.ScopePadRead}, "padUsersCount": {accounts.ScopePadRead},
	"padUsers": {accounts.ScopePadRead}, "createDiffHTML": {accounts.ScopePadRead},
	"getChatHistory": {accounts.ScopePadRead}, "getChatHead": {accounts.ScopePadRead},
	"getAttributePool": {accounts.ScopePadRead}, "getRevisionChangeset": {accounts.ScopePadRead},
	"getSavedRevisionsCount": {accounts.ScopePadRead}, "listSavedRevisions": {accounts.ScopePadRead},
	// writing
	"setText": {accounts.ScopePadWrite}, "setHTML": {accounts.ScopePadWrite},
	"appendText": {accounts.ScopePadWrite}, "saveRevision": {accounts.ScopePadWrite},
	"restoreRevision": {accounts.ScopePadWrite}, "sendClientsMessage": {accounts.ScopePadWrite},
	"appendChatMessage": {accounts.ScopeChatWrite},
	// settings of the pad
	"setPublicStatus": {accounts.ScopePadSettings},
	// creating and deleting
	"createPad":  {accounts.ScopePadCreate, accounts.ScopePadWrite},
	"deletePad":  {accounts.ScopePadDelete},
	"compactPad": {accounts.ScopePadDelete},
}

// copyFunctions take sourceID and destinationID.
var copyFunctions = map[string]bool{"copyPad": true, "copyPadWithoutHistory": true, "movePad": true}

// needsOf lists what calling function with args needs. ok is false where
// a pad id is missing or not a pad (read-only id): then the call needs
// admin:pads for all pads.
func needsOf(function string, args map[string]string) []need {
	admin := []need{{scope: accounts.ScopeAdminPads}}
	isPad := func(id string) bool { return id != "" && !strings.HasPrefix(id, "r.") }
	if scopes, ok := padFunctions[function]; ok {
		pad := args["padID"]
		if !isPad(pad) {
			return admin
		}
		out := make([]need, 0, len(scopes))
		for _, s := range scopes {
			out = append(out, need{scope: s, pad: pad})
		}
		return out
	}
	if copyFunctions[function] {
		src, dst := args["sourceID"], args["destinationID"]
		if !isPad(src) || !isPad(dst) {
			return admin
		}
		out := []need{{accounts.ScopePadRead, src}, {accounts.ScopePadCreate, dst}, {accounts.ScopePadWrite, dst}}
		if function == "movePad" {
			out = append(out, need{accounts.ScopePadDelete, src})
		}
		if boolArg(args["force"]) {
			// Overwriting the destination deletes what was there.
			out = append(out, need{accounts.ScopePadDelete, dst})
		}
		return out
	}
	if function == "checkToken" {
		return nil
	}
	return admin
}

// allowed reports whether grants allow every need.
func allowed(g accounts.Grants, needs []need) bool {
	for _, n := range needs {
		if n.pad == "" {
			if !g.HasGlobal(n.scope) {
				return false
			}
			continue
		}
		if !g.Has(n.scope, n.pad) {
			return false
		}
	}
	return true
}

// padParams are the parameters that name a pad.
var padParams = []string{"padID", "sourceID", "destinationID"}

// sanitizedArgs returns args with each pad id as the handlers turn it
// (SanitizePadId: whitespace and ":" replaced, lower-cased with
// lowerCasePadIds) for an id that names no pad; "" where that fails, so
// the call needs admin:pads. ok is false when nothing changed.
func sanitizedArgs(args map[string]string, sanitize func(string) (string, error)) (map[string]string, bool) {
	out := make(map[string]string, len(args))
	for k, v := range args {
		out[k] = v
	}
	changed := false
	for _, k := range padParams {
		v, ok := args[k]
		if !ok || v == "" {
			continue
		}
		id, err := sanitize(v)
		if err != nil {
			id = ""
		}
		if id != v {
			out[k], changed = id, true
		}
	}
	return out, changed
}
