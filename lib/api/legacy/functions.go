package legacy

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// call runs one API function. The REST API does the work; the checks here
// give the answers etherpad-lite gives.
func (a *API) call(function string, p args) (any, *apiError) {
	switch function {
	// ── groups ──
	case "createGroup":
		return a.rest("POST", "/groups", nil, nil, "")
	case "createGroupIfNotExistsFor":
		if !p.has("groupMapper") {
			return nil, wrongParam("groupMapper is not a string")
		}
		return a.rest("POST", "/groups/createIfNotExistsFor", nil,
			map[string]string{"groupMapper": p["groupMapper"]}, "")
	case "deleteGroup":
		g, e := a.group(p)
		if e != nil {
			return nil, e
		}
		return nothing(a.rest("DELETE", "/groups/"+esc(g), nil, nil, "groupID does not exist"))
	case "listPads":
		g, e := a.group(p)
		if e != nil {
			return nil, e
		}
		return a.rest("GET", "/groups/"+esc(g)+"/pads", nil, nil, "groupID does not exist")
	case "listAllGroups":
		return a.rest("GET", "/groups", nil, nil, "")
	case "createGroupPad":
		g, e := a.group(p)
		if e != nil {
			return nil, e
		}
		name, err := a.store.PadManager.SanitizePadId(p["padName"])
		if err != nil {
			return nil, internalError
		}
		if exists, err := a.store.PadManager.DoesPadExist(g + "$" + name); err != nil {
			return nil, internalError
		} else if *exists {
			return nil, wrongParam("padName does already exist")
		}
		body := map[string]string{"padName": name, "text": p["text"], "authorId": p["authorId"]}
		return a.rest("POST", "/groups/"+esc(g)+"/pads", nil, body, "groupID does not exist")

	// ── authors ──
	case "createAuthor":
		var name *string
		if p.has("name") {
			n := p["name"]
			name = &n
		}
		created, err := a.store.AuthorManager.CreateAuthor(name)
		if err != nil {
			return nil, internalError
		}
		return map[string]string{"authorID": created.Id}, nil
	case "createAuthorIfNotExistsFor":
		if !p.has("authorMapper") {
			return nil, wrongParam("authorMapper is not a string")
		}
		data, e := a.rest("POST", "/author/createIfNotExistsFor", nil,
			map[string]string{"authorMapper": p["authorMapper"], "name": p["name"]}, "")
		if e != nil {
			return nil, e
		}
		return map[string]any{"authorID": field(data, "authorId")}, nil
	case "getAuthorName":
		data, e := a.rest("GET", "/author/"+esc(p["authorID"])+"/name", nil, nil, "authorID does not exist")
		if e != nil {
			return nil, e
		}
		return field(data, "authorName"), nil
	case "listPadsOfAuthor":
		data, e := a.rest("GET", "/author/"+esc(p["authorID"])+"/pads", nil, nil, "authorID does not exist")
		if e != nil {
			return nil, e
		}
		if data == nil {
			data = []any{}
		}
		return map[string]any{"padIDs": data}, nil
	case "anonymizeAuthor":
		if p["authorID"] == "" {
			return nil, wrongParam("authorID is required")
		}
		return a.rest("POST", "/author/"+esc(p["authorID"])+"/anonymize", nil, nil, "authorID does not exist")

	// ── sessions ──
	case "createSession":
		validUntil, e := checkValidUntil(p)
		if e != nil {
			return nil, e
		}
		if _, e := a.group(p); e != nil {
			return nil, e
		}
		if _, err := a.store.Store.GetAuthor(p["authorID"]); err != nil {
			return nil, wrongParam("authorID does not exist")
		}
		return a.rest("POST", "/sessions", nil, map[string]any{
			"groupID": p["groupID"], "authorID": p["authorID"], "validUntil": validUntil,
		}, "")
	case "deleteSession":
		return nothing(a.rest("DELETE", "/sessions/"+esc(p["sessionID"]), nil, nil, "sessionID does not exist"))
	case "getSessionInfo":
		return a.rest("GET", "/sessions/"+esc(p["sessionID"]), nil, nil, "sessionID does not exist")
	case "listSessionsOfGroup":
		g, e := a.group(p)
		if e != nil {
			return nil, e
		}
		return sessionMap(a.rest("GET", "/groups/"+esc(g)+"/sessions", nil, nil, "groupID does not exist"))
	case "listSessionsOfAuthor":
		return sessionMap(a.rest("GET", "/authors/"+esc(p["authorID"])+"/sessions", nil, nil, "authorID does not exist"))

	// ── pads ──
	case "listAllPads":
		return a.rest("GET", "/pads", nil, nil, "")
	case "createPad":
		id, e := a.newPadID(p)
		if e != nil {
			return nil, e
		}
		// Without text the pad gets the default text; "" makes it empty.
		body := map[string]any{"authorId": p["authorId"]}
		if p.has("text") {
			body["text"] = p["text"]
		}
		return a.rest("POST", "/pads/"+esc(id), nil, body, "")
	case "deletePad":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		q := url.Values{}
		if p.has("deletionToken") {
			q.Set("deletionToken", p["deletionToken"])
		}
		return nothing(a.rest("DELETE", "/pads/"+esc(id), q, nil, "padID does not exist"))
	case "getText", "getHTML":
		id, rev, e := a.padAndRev(p, false)
		if e != nil {
			return nil, e
		}
		q := url.Values{}
		if rev != nil {
			q.Set("rev", strconv.Itoa(*rev))
		}
		path := "/pads/" + esc(id) + "/text"
		if function == "getHTML" {
			path = "/pads/" + esc(id) + "/html"
		}
		return a.rest("GET", path, q, nil, "padID does not exist")
	case "setText", "appendText":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		if !p.has("text") {
			return nil, wrongParam("text is not a string")
		}
		path := "/pads/" + esc(id) + "/text"
		if function == "appendText" {
			path = "/pads/" + esc(id) + "/appendText"
		}
		return nothing(a.rest("POST", path, nil, map[string]string{"text": p["text"], "authorId": p["authorId"]}, "padID does not exist"))
	case "setHTML":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		if !p.has("html") {
			return nil, wrongParam("html is not a string")
		}
		return nothing(a.rest("POST", "/pads/"+esc(id)+"/html", nil, map[string]string{"html": p["html"], "authorId": p["authorId"]}, "padID does not exist"))
	case "getRevisionsCount", "getLastEdited", "getReadOnlyID", "getPublicStatus",
		"listAuthorsOfPad", "padUsersCount", "padUsers", "getChatHead", "getAttributePool",
		"getSavedRevisionsCount", "listSavedRevisions":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		return a.rest("GET", "/pads/"+esc(id)+"/"+padReadPaths[function], nil, nil, "padID does not exist")
	case "setPublicStatus":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		return nothing(a.rest("POST", "/pads/"+esc(id)+"/publicStatus", nil,
			map[string]bool{"publicStatus": boolArg(p["publicStatus"])}, "padID does not exist"))
	case "sendClientsMessage":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		return nothing(a.rest("POST", "/pads/"+esc(id)+"/sendClientsMessage", nil,
			map[string]string{"msg": p["msg"]}, "padID does not exist"))
	case "checkToken":
		return nil, nil
	case "createDiffHTML":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		q := url.Values{}
		for _, k := range []string{"startRev", "endRev"} {
			if p.has(k) {
				n, e := checkRev(p[k])
				if e != nil {
					return nil, e
				}
				q.Set(k, strconv.Itoa(n))
			}
		}
		return a.rest("GET", "/pads/"+esc(id)+"/diffHTML", q, nil, "padID does not exist")
	case "getChatHistory":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		q := url.Values{}
		if p.has("start") != p.has("end") {
			return nil, wrongParam("start and end must both be given or both be omitted")
		}
		if p.has("start") {
			start, ok1 := parseIntJS(p["start"])
			end, ok2 := parseIntJS(p["end"])
			if !ok1 || !ok2 {
				return nil, wrongParam("start and end must be numbers")
			}
			if start < 0 {
				return nil, wrongParam("start is below zero")
			}
			if start > end {
				return nil, wrongParam("start is higher than end")
			}
			q.Set("start", strconv.Itoa(start))
			q.Set("end", strconv.Itoa(end))
		}
		data, e := a.rest("GET", "/pads/"+esc(id)+"/chatHistory", q, nil, "padID does not exist")
		if e != nil {
			return nil, e
		}
		// etherpad-lite's messages: {text, userId, time, userName}.
		list, _ := field(data, "messages").([]any)
		messages := make([]any, 0, len(list))
		for _, m := range list {
			if mm, ok := m.(map[string]any); ok {
				messages = append(messages, map[string]any{
					"text": mm["text"], "userId": mm["authorID"], "time": mm["time"], "userName": mm["userName"],
				})
			}
		}
		return map[string]any{"messages": messages}, nil
	case "appendChatMessage":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		if !p.has("text") {
			return nil, wrongParam("text is not a string")
		}
		when := time.Now().UnixMilli()
		if p.has("time") {
			if n, ok := parseIntJS(p["time"]); ok {
				when = int64(n)
			}
		}
		return nothing(a.rest("POST", "/pads/"+esc(id)+"/chat", nil, map[string]any{
			"text": p["text"], "authorID": p["authorID"], "time": when,
		}, "padID does not exist"))
	case "getRevisionChangeset":
		id, rev, e := a.padAndRev(p, false)
		if e != nil {
			return nil, e
		}
		q := url.Values{}
		if rev != nil {
			q.Set("rev", strconv.Itoa(*rev))
		}
		data, e := a.rest("GET", "/pads/"+esc(id)+"/revisionChangeset", q, nil, "padID does not exist")
		if e != nil {
			return nil, e
		}
		return field(data, "changeset"), nil
	case "copyPad", "movePad", "copyPadWithoutHistory":
		src, e := a.pad(p, "sourceID")
		if e != nil {
			return nil, e
		}
		dst, err := a.store.PadManager.SanitizePadId(p["destinationID"])
		if err != nil {
			return nil, internalError
		}
		if !a.store.PadManager.IsValidPadId(dst) {
			return nil, wrongParam("destinationID did not match requirements")
		}
		if i := strings.Index(dst, "$"); i >= 0 {
			if found, err := a.store.Store.GetGroup(dst[:i]); err != nil || found == nil {
				return nil, wrongParam("groupID does not exist for destinationID")
			}
		}
		force := boolArg(p["force"])
		if !force {
			if exists, err := a.store.PadManager.DoesPadExist(dst); err != nil {
				return nil, internalError
			} else if *exists {
				return nil, wrongParam("destinationID does already exist")
			}
		}
		path := map[string]string{"copyPad": "copy", "movePad": "move", "copyPadWithoutHistory": "copyWithoutHistory"}[function]
		return a.rest("POST", "/pads/"+esc(src)+"/"+path, nil, map[string]any{
			"destinationID": dst, "force": force, "authorId": p["authorId"],
		}, "padID does not exist")
	case "getPadID":
		data, e := a.rest("GET", "/pads/readonly/"+esc(p["roID"]), nil, nil, "padID does not exist")
		if e != nil {
			return nil, e
		}
		return data, nil
	case "saveRevision":
		id, rev, e := a.padAndRev(p, false)
		if e != nil {
			return nil, e
		}
		var body map[string]int
		if rev != nil {
			body = map[string]int{"rev": *rev}
		}
		return nothing(a.rest("POST", "/pads/"+esc(id)+"/saveRevision", nil, body, "padID does not exist"))
	case "restoreRevision":
		if !p.has("rev") {
			return nil, wrongParam("rev is not defined")
		}
		id, rev, e := a.padAndRev(p, true)
		if e != nil {
			return nil, e
		}
		return nothing(a.rest("POST", "/pads/"+esc(id)+"/restoreRevision", nil,
			map[string]any{"rev": *rev, "authorId": p["authorId"]}, "padID does not exist"))
	case "compactPad":
		id, e := a.pad(p, "padID")
		if e != nil {
			return nil, e
		}
		body := map[string]any{}
		if p.has("keepRevisions") {
			n, ok := number(p["keepRevisions"])
			if !ok || n < 0 || n != float64(int(n)) {
				return nil, wrongParam("keepRevisions must be a non-negative integer")
			}
			body["keepRevisions"] = int(n)
		}
		return a.rest("POST", "/pads/"+esc(id)+"/compact", nil, body, "padID does not exist")
	case "getStats":
		return a.rest("GET", "/stats", nil, nil, "")
	}
	return nil, &apiError{code: 3, message: "no such function"}
}

var padReadPaths = map[string]string{
	"getRevisionsCount":      "revisionsCount",
	"getLastEdited":          "lastEdited",
	"getReadOnlyID":          "readOnlyID",
	"getPublicStatus":        "publicStatus",
	"listAuthorsOfPad":       "authors",
	"padUsersCount":          "usersCount",
	"padUsers":               "users",
	"getChatHead":            "chatHead",
	"getAttributePool":       "attributePool",
	"getSavedRevisionsCount": "savedRevisionsCount",
	"listSavedRevisions":     "savedRevisions",
}

// nothing drops the data of calls that answer with data null.
func nothing(_ any, e *apiError) (any, *apiError) { return nil, e }

func field(data any, key string) any {
	if m, ok := data.(map[string]any); ok {
		return m[key]
	}
	return nil
}

// sessionMap turns the REST session list into etherpad-lite's
// {sessionID: info} map, or null when there are none.
func sessionMap(data any, e *apiError) (any, *apiError) {
	if e != nil {
		return nil, e
	}
	list, _ := field(data, "sessions").([]any)
	if len(list) == 0 {
		return nil, nil
	}
	out := map[string]any{}
	for _, s := range list {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["sessionID"].(string)
		out[id] = map[string]any{"groupID": m["groupID"], "authorID": m["authorID"], "validUntil": m["validUntil"]}
	}
	return out, nil
}

// pad checks a pad id like etherpad-lite's getPadSafe(id, true).
func (a *API) pad(p args, key string) (string, *apiError) {
	if !p.has(key) {
		return "", wrongParam("padID is not a string")
	}
	id, err := a.store.PadManager.SanitizePadId(p[key])
	if err != nil {
		return "", internalError
	}
	ok, err := a.store.PadManager.CanOpenPadId(id)
	if err != nil {
		return "", internalError
	}
	if !ok {
		return "", wrongParam("padID did not match requirements")
	}
	exists, err := a.store.PadManager.DoesPadExist(id)
	if err != nil {
		return "", internalError
	}
	if !*exists {
		return "", wrongParam("padID does not exist")
	}
	return id, nil
}

// newPadID checks the id of a pad to create (etherpad-lite's createPad).
func (a *API) newPadID(p args) (string, *apiError) {
	if !p.has("padID") {
		return "", wrongParam("padID is not a string")
	}
	id, err := a.store.PadManager.SanitizePadId(p["padID"])
	if err != nil {
		return "", internalError
	}
	if strings.Contains(id, "$") {
		return "", wrongParam("createPad can't create group pads")
	}
	if strings.ContainsAny(id, "/?&#") {
		return "", wrongParam("malformed padID: Remove special characters")
	}
	if !a.store.PadManager.IsValidPadId(id) {
		return "", wrongParam("malformed padID: Remove special characters")
	}
	exists, err := a.store.PadManager.DoesPadExist(id)
	if err != nil {
		return "", internalError
	}
	if *exists {
		return "", wrongParam("padID does already exist")
	}
	return id, nil
}

// padAndRev checks padID and the optional (or required) rev.
func (a *API) padAndRev(p args, required bool) (string, *int, *apiError) {
	var rev *int
	if p.has("rev") {
		n, e := checkRev(p["rev"])
		if e != nil {
			return "", nil, e
		}
		rev = &n
	} else if required {
		return "", nil, wrongParam("rev is not defined")
	}
	id, e := a.pad(p, "padID")
	if e != nil {
		return "", nil, e
	}
	if rev != nil {
		loaded, err := a.store.PadManager.GetPad(id, nil, nil)
		if err != nil {
			return "", nil, internalError
		}
		if *rev > loaded.Head {
			return "", nil, wrongParam("rev is higher than the head revision of the pad")
		}
	}
	return id, rev, nil
}

func (a *API) group(p args) (string, *apiError) {
	g := p["groupID"]
	if g == "" {
		return "", wrongParam("groupID does not exist")
	}
	if found, err := a.store.Store.GetGroup(g); err != nil || found == nil {
		return "", wrongParam("groupID does not exist")
	}
	return g, nil
}

// checkValidUntil is etherpad-lite's createSession check of validUntil.
func checkValidUntil(p args) (int64, *apiError) {
	n, ok := number(p["validUntil"])
	if !ok {
		return 0, wrongParam("validUntil is not a number")
	}
	if n < 0 {
		return 0, wrongParam("validUntil is a negative number")
	}
	if n != float64(int64(n)) {
		return 0, wrongParam("validUntil is a float value")
	}
	if int64(n) < time.Now().Unix() {
		return 0, wrongParam("validUntil is in the past")
	}
	return int64(n), nil
}

// sortedFunctions lists the functions of a version, for documentation.
func sortedFunctions(version string) []string {
	names := make([]string, 0, len(versions[version]))
	for n := range versions[version] {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
