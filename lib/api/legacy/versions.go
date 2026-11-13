package legacy

// LatestVersion is the newest API version (etherpad-lite 1.3.1).
const LatestVersion = "1.3.1"

// versionOrder lists the API versions, oldest first. Each version has the
// functions of the one before it, plus its own.
var versionOrder = []string{
	"1", "1.1", "1.2", "1.2.1", "1.2.7", "1.2.8", "1.2.9", "1.2.10", "1.2.11",
	"1.2.12", "1.2.13", "1.2.14", "1.2.15", "1.3.0", "1.3.1",
}

// versionChanges are the functions each version adds or changes, with the
// parameters it takes, as in etherpad-lite's APIHandler. A parameter that is
// not listed for the called version is ignored, as there: before 1.3.0,
// setText takes no authorId.
var versionChanges = map[string]map[string][]string{
	"1": {
		"createGroup":                {},
		"createGroupIfNotExistsFor":  {"groupMapper"},
		"deleteGroup":                {"groupID"},
		"listPads":                   {"groupID"},
		"createPad":                  {"padID", "text"},
		"createGroupPad":             {"groupID", "padName", "text"},
		"createAuthor":               {"name"},
		"createAuthorIfNotExistsFor": {"authorMapper", "name"},
		"listPadsOfAuthor":           {"authorID"},
		"createSession":              {"groupID", "authorID", "validUntil"},
		"deleteSession":              {"sessionID"},
		"getSessionInfo":             {"sessionID"},
		"listSessionsOfGroup":        {"groupID"},
		"listSessionsOfAuthor":       {"authorID"},
		"getText":                    {"padID", "rev"},
		"setText":                    {"padID", "text"},
		"getHTML":                    {"padID", "rev"},
		"setHTML":                    {"padID", "html"},
		"getRevisionsCount":          {"padID"},
		"getLastEdited":              {"padID"},
		"deletePad":                  {"padID", "deletionToken"},
		"getReadOnlyID":              {"padID"},
		"setPublicStatus":            {"padID", "publicStatus"},
		"getPublicStatus":            {"padID"},
		"listAuthorsOfPad":           {"padID"},
		"padUsersCount":              {"padID"},
	},
	"1.1": {
		"getAuthorName":      {"authorID"},
		"padUsers":           {"padID"},
		"sendClientsMessage": {"padID", "msg"},
		"listAllGroups":      {},
	},
	"1.2":   {"checkToken": {}},
	"1.2.1": {"listAllPads": {}},
	"1.2.7": {
		"createDiffHTML": {"padID", "startRev", "endRev"},
		"getChatHistory": {"padID", "start", "end"},
		"getChatHead":    {"padID"},
	},
	"1.2.8": {
		"getAttributePool":     {"padID"},
		"getRevisionChangeset": {"padID", "rev"},
	},
	"1.2.9": {
		"copyPad": {"sourceID", "destinationID", "force"},
		"movePad": {"sourceID", "destinationID", "force"},
	},
	"1.2.10": {"getPadID": {"roID"}},
	"1.2.11": {
		"getSavedRevisionsCount": {"padID"},
		"listSavedRevisions":     {"padID"},
		"saveRevision":           {"padID", "rev"},
		"restoreRevision":        {"padID", "rev"},
	},
	"1.2.12": {"appendChatMessage": {"padID", "text", "authorID", "time"}},
	"1.2.13": {"appendText": {"padID", "text"}},
	"1.2.14": {"getStats": {}},
	"1.2.15": {"copyPadWithoutHistory": {"sourceID", "destinationID", "force"}},
	"1.3.0": {
		"appendText":            {"padID", "text", "authorId"},
		"copyPadWithoutHistory": {"sourceID", "destinationID", "force", "authorId"},
		"createGroupPad":        {"groupID", "padName", "text", "authorId"},
		"createPad":             {"padID", "text", "authorId"},
		"restoreRevision":       {"padID", "rev", "authorId"},
		"setHTML":               {"padID", "html", "authorId"},
		"setText":               {"padID", "text", "authorId"},
	},
	"1.3.1": {
		"compactPad":      {"padID", "keepRevisions"},
		"anonymizeAuthor": {"authorID"},
	},
}

// versions maps each version to its functions and their parameters.
var versions = func() map[string]map[string][]string {
	out := map[string]map[string][]string{}
	current := map[string][]string{}
	for _, v := range versionOrder {
		next := make(map[string][]string, len(current))
		for k, p := range current {
			next[k] = p
		}
		for k, p := range versionChanges[v] {
			next[k] = p
		}
		out[v] = next
		current = next
	}
	return out
}()
