package legacy

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// etherpad-lite serves the same functions under two more path styles:
//
//   - /rest/{version}/{resource}/{action}, e.g. /rest/1.3.1/group/create,
//     GET and POST like /api/{version}/{function} (openapi.ts);
//   - /api/2/..., one HTTP method and path per function, e.g. POST
//     /api/2/groups, documented at /api-docs.json (RestAPI.ts).
//
// Both end in the same functions, API keys and response format.

// restPaths are the /rest paths of the functions, etherpad-lite's resources.
var restPaths = map[string]string{
	"createGroup": "/group/create", "createGroupIfNotExistsFor": "/group/createIfNotExistsFor",
	"deleteGroup": "/group/delete", "listPads": "/group/listPads", "createGroupPad": "/group/createPad",
	"listSessionsOfGroup": "/group/listSessions", "listAllGroups": "/group/list",

	"createAuthor": "/author/create", "createAuthorIfNotExistsFor": "/author/createIfNotExistsFor",
	"listPadsOfAuthor": "/author/listPads", "listSessionsOfAuthor": "/author/listSessions",
	"getAuthorName": "/author/getName", "anonymizeAuthor": "/author/anonymize",

	"createSession": "/session/create", "deleteSession": "/session/delete", "getSessionInfo": "/session/info",

	"listAllPads": "/pad/listAll", "createDiffHTML": "/pad/createDiffHTML", "createPad": "/pad/create",
	"getText": "/pad/getText", "setText": "/pad/setText", "getHTML": "/pad/getHTML", "setHTML": "/pad/setHTML",
	"getRevisionsCount": "/pad/getRevisionsCount", "getLastEdited": "/pad/getLastEdited",
	"deletePad": "/pad/delete", "getReadOnlyID": "/pad/getReadOnlyID",
	"setPublicStatus": "/pad/setPublicStatus", "getPublicStatus": "/pad/getPublicStatus",
	"listAuthorsOfPad": "/pad/authors", "padUsersCount": "/pad/usersCount", "padUsers": "/pad/users",
	"sendClientsMessage": "/pad/sendClientsMessage", "checkToken": "/pad/checkToken",
	"getChatHistory": "/pad/getChatHistory", "getChatHead": "/pad/getChatHead",
	"appendChatMessage": "/pad/appendChatMessage", "getAttributePool": "/pad/getAttributePool",
	"getRevisionChangeset": "/pad/getRevisionChangeset", "copyPad": "/pad/copyPad", "movePad": "/pad/movePad",
	"getPadID": "/pad/getPadID", "getSavedRevisionsCount": "/pad/getSavedRevisionsCount",
	"listSavedRevisions": "/pad/listSavedRevisions", "saveRevision": "/pad/saveRevision",
	"restoreRevision": "/pad/restoreRevision", "appendText": "/pad/appendText",
	"copyPadWithoutHistory": "/pad/copyPadWithoutHistory", "compactPad": "/pad/compactPad",

	"getStats": "/server/getStats",
}

// restFunctions maps a /rest path back to its function.
var restFunctions = func() map[string]string {
	m := make(map[string]string, len(restPaths))
	for f, p := range restPaths {
		m[p] = f
	}
	return m
}()

// api2Route is a route of /api/2: the function and the API version whose
// parameters it takes, as in etherpad-lite's RestAPI mapping.
type api2Route struct {
	method, path, version, function, summary, tag string
}

var api2Routes = []api2Route{
	{fiber.MethodPost, "/groups", "1", "createGroup", "Creates a new group", "group"},
	{fiber.MethodPost, "/groups/createIfNotExistsFor", "1", "createGroupIfNotExistsFor", "Creates a new group if it doesn't exist", "group"},
	{fiber.MethodGet, "/groups/pads", "1", "listPads", "Lists all pads in a group", "group"},
	{fiber.MethodDelete, "/groups", "1", "deleteGroup", "Deletes a group", "group"},
	{fiber.MethodPost, "/authors", "1", "createAuthor", "", "author"},
	{fiber.MethodPost, "/authors/createIfNotExistsFor", "1", "createAuthorIfNotExistsFor", "", "author"},
	{fiber.MethodGet, "/authors/pads", "1", "listPadsOfAuthor", "", "author"},
	{fiber.MethodPost, "/sessions", "1", "createSession", "", "session"},
	{fiber.MethodDelete, "/sessions", "1", "deleteSession", "", "session"},
	{fiber.MethodGet, "/sessions/info", "1", "getSessionInfo", "", "session"},
	{fiber.MethodGet, "/sessions/group", "1", "listSessionsOfGroup", "Lists all sessions in a group", "session"},
	{fiber.MethodGet, "/sessions/author", "1", "listSessionsOfAuthor", "Lists all sessions of an author", "session"},
	{fiber.MethodGet, "/pads/text", "1", "getText", "", "pad"},
	{fiber.MethodGet, "/pads/html", "1", "getHTML", "Get the HTML of a pad", "pad"},
	{fiber.MethodGet, "/pads/revisions", "1", "getRevisionsCount", "Get the number of revisions of a pad", "pad"},
	{fiber.MethodGet, "/pads/lastEdited", "1", "getLastEdited", "Get the timestamp of the last revision of a pad", "pad"},
	{fiber.MethodDelete, "/pads", "1", "deletePad", "Deletes a pad", "pad"},
	{fiber.MethodGet, "/pads/readonly", "1", "getReadOnlyID", "Get the read only id of a pad", "pad"},
	{fiber.MethodPost, "/pads/publicStatus", "1", "setPublicStatus", "Set the public status of a pad", "pad"},
	{fiber.MethodGet, "/pads/publicStatus", "1", "getPublicStatus", "Get the public status of a pad", "pad"},
	{fiber.MethodGet, "/pads/authors", "1", "listAuthorsOfPad", "Get the authors of a pad", "pad"},
	{fiber.MethodGet, "/pads/usersCount", "1", "padUsersCount", "Get the number of users currently editing a pad", "pad"},
	{fiber.MethodGet, "/authors/name", "1.1", "getAuthorName", "Get the name of an author", "author"},
	{fiber.MethodGet, "/pads/users", "1.1", "padUsers", "Get the users currently editing a pad", "pad"},
	{fiber.MethodPost, "/pads/clientsMessage", "1.1", "sendClientsMessage", "Send a message to all clients of a pad", "pad"},
	{fiber.MethodGet, "/groups", "1.1", "listAllGroups", "Lists all groups", "group"},
	{fiber.MethodGet, "/checkToken", "1.2", "checkToken", "Check if a token is valid", "token"},
	{fiber.MethodGet, "/pads", "1.2.1", "listAllPads", "Lists all pads", "pad"},
	{fiber.MethodPost, "/pads/diff", "1.2.7", "createDiffHTML", "Creates a diff of a pad", "pad"},
	{fiber.MethodGet, "/pads/chatHistory", "1.2.7", "getChatHistory", "Get the chat history of a pad", "pad"},
	{fiber.MethodGet, "/pads/chatHead", "1.2.7", "getChatHead", "Get the chat head of a pad", "pad"},
	{fiber.MethodGet, "/pads/attributePool", "1.2.8", "getAttributePool", "Get the attribute pool of a pad", "pad"},
	{fiber.MethodGet, "/pads/revisionChangeset", "1.2.8", "getRevisionChangeset", "Get the changeset of a revision of a pad", "pad"},
	{fiber.MethodPost, "/pads/copypad", "1.2.9", "copyPad", "Copies a pad", "pad"},
	{fiber.MethodPost, "/pads/movePad", "1.2.9", "movePad", "Moves a pad", "pad"},
	{fiber.MethodPost, "/pads/padId", "1.2.10", "getPadID", "Get the pad id of a pad", "pad"},
	{fiber.MethodGet, "/savedRevisions", "1.2.11", "listSavedRevisions", "Lists all saved revisions of a pad", "pad"},
	{fiber.MethodPost, "/savedRevisions", "1.2.11", "saveRevision", "Saves a revision of a pad", "pad"},
	{fiber.MethodGet, "/savedRevisions/revisionsCount", "1.2.11", "getSavedRevisionsCount", "Get the number of saved revisions of a pad", "pad"},
	{fiber.MethodPatch, "/chats/messages", "1.2.12", "appendChatMessage", "Appends a chat message to a pad", "pad"},
	{fiber.MethodGet, "/stats", "1.2.14", "getStats", "Get stats", "stats"},
	{fiber.MethodPatch, "/pads/text", "1.3.0", "appendText", "Appends text to a pad", "pad"},
	{fiber.MethodPost, "/pads/copyWithoutHistory", "1.3.0", "copyPadWithoutHistory", "Copies a pad without its history", "pad"},
	{fiber.MethodPost, "/pads/group", "1.3.0", "createGroupPad", "Creates a new pad in a group", "pad"},
	{fiber.MethodPost, "/pads", "1.3.0", "createPad", "Creates a new pad", "pad"},
	{fiber.MethodPatch, "/savedRevisions", "1.3.0", "restoreRevision", "Restores a revision of a pad", "pad"},
	{fiber.MethodPost, "/pads/html", "1.3.0", "setHTML", "Sets the HTML of a pad", "pad"},
	{fiber.MethodPost, "/pads/text", "1.3.0", "setText", "Sets the text of a pad", "pad"},
}

var api2Index = func() map[string]api2Route {
	m := make(map[string]api2Route, len(api2Routes))
	for _, r := range api2Routes {
		m[r.method+" "+r.path] = r
	}
	return m
}()

func (a *API) registerREST(app *fiber.App) {
	latestREST := func(c fiber.Ctx) error { return a.serveRESTDefinition(c, LatestVersion) }
	app.Get("/rest/openapi.json", latestREST)
	app.Get("/rest/:version/openapi.json", func(c fiber.Ctx) error {
		return a.serveRESTDefinition(c, c.Params("version"))
	})
	app.Get("/rest/:version/:resource/:action", a.serveREST)
	app.Post("/rest/:version/:resource/:action", a.serveREST)

	app.Get("/api-docs.json", a.serveAPI2Definition)
	// Before /api/:version/:function, which would take /api/2/groups.
	methods := []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodDelete, fiber.MethodPatch}
	app.Add(methods, "/api/2", a.serveAPI2)
	app.Add(methods, "/api/2/*", a.serveAPI2)
}

func (a *API) serveREST(c fiber.Ctx) error {
	// etherpad-lite allows every origin here, for old Swagger clients.
	c.Set(fiber.HeaderAccessControlAllowOrigin, "*")
	function, ok := restFunctions["/"+c.Params("resource")+"/"+c.Params("action")]
	if !ok {
		r, status := reply(&apiError{code: 3, message: "no such function"})
		return c.Status(status).JSON(r)
	}
	resp, status := a.handleCall(c, c.Params("version"), function)
	return c.Status(status).JSON(resp)
}

func (a *API) serveRESTDefinition(c fiber.Ctx, version string) error {
	c.Set(fiber.HeaderAccessControlAllowOrigin, "*")
	if _, ok := versions[version]; !ok {
		r, status := reply(&apiError{code: 3, message: "no such api version"})
		return c.Status(status).JSON(r)
	}
	return c.JSON(a.definitionWith(version, a.origin(c)+"/rest/"+version, func(f string) string { return restPaths[f] }))
}

func (a *API) serveAPI2(c fiber.Ctx) error {
	path := strings.TrimPrefix(c.Path(), "/api/2")
	route, ok := api2Index[c.Method()+" "+path]
	if !ok {
		// etherpad-lite's answer to an unknown route.
		return c.JSON(fiber.Map{"code": 1, "message": "not found"})
	}
	resp, status := a.handleCall(c, route.version, route.function)
	return c.Status(status).JSON(resp)
}

// serveAPI2Definition is the OpenAPI document of /api/2.
func (a *API) serveAPI2Definition(c fiber.Ctx) error {
	doc := a.definition(LatestVersion, a.origin(c)+"/api/2")
	paths := fiber.Map{}
	for _, r := range api2Routes {
		op := fiber.Map{
			"operationId": r.function,
			"summary":     r.summary,
			"tags":        []string{r.tag},
			"responses": fiber.Map{
				"200": fiber.Map{"$ref": "#/components/responses/Success"},
				"401": fiber.Map{"$ref": "#/components/responses/Unauthorized"},
				"500": fiber.Map{"$ref": "#/components/responses/InternalError"},
			},
		}
		if r.summary == "" {
			op["summary"] = summary(r.function)
		}
		params := versions[r.version][r.function]
		if r.method == fiber.MethodGet {
			list := make([]fiber.Map, 0, len(params))
			for _, p := range params {
				list = append(list, fiber.Map{"name": p, "in": "query", "schema": fiber.Map{"type": "string"}})
			}
			op["parameters"] = list
		} else if len(params) > 0 {
			props := fiber.Map{}
			for _, p := range params {
				props[p] = fiber.Map{"type": "string"}
			}
			op["requestBody"] = fiber.Map{"content": fiber.Map{"application/json": fiber.Map{
				"schema": fiber.Map{"type": "object", "properties": props},
			}}}
		}
		item, _ := paths[r.path].(fiber.Map)
		if item == nil {
			item = fiber.Map{}
			paths[r.path] = item
		}
		item[strings.ToLower(r.method)] = op
	}
	doc["paths"] = paths
	doc["tags"] = append(doc["tags"].([]fiber.Map),
		fiber.Map{"name": "token", "description": "Tokens"},
		fiber.Map{"name": "stats", "description": "Statistics of the server"})
	info := doc["info"].(fiber.Map)
	info["version"] = "2"
	info["description"] = "etherpad-lite's REST API: one HTTP method and path per function. Parameters go in the query string (GET) or the body (JSON or form). code 0 is ok, 1 a wrong parameter, 2 an internal error, 4 no or wrong API key."
	return c.JSON(doc)
}
