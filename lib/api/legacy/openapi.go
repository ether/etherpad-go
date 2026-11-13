package legacy

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// functionTags are the OpenAPI tags of the functions, as in etherpad-lite.
var functionTags = map[string]string{
	"createGroup": "group", "createGroupIfNotExistsFor": "group", "deleteGroup": "group",
	"listPads": "group", "createGroupPad": "group", "listAllGroups": "group",
	"createAuthor": "author", "createAuthorIfNotExistsFor": "author", "listPadsOfAuthor": "author",
	"getAuthorName": "author", "anonymizeAuthor": "author",
	"createSession": "session", "deleteSession": "session", "getSessionInfo": "session",
	"listSessionsOfGroup": "session", "listSessionsOfAuthor": "session",
	"getChatHistory": "chat", "getChatHead": "chat", "appendChatMessage": "chat",
	"checkToken": "server", "getStats": "server",
}

var tagDescriptions = []struct{ name, description string }{
	{"pad", "Pads: text, HTML, revisions, copies"},
	{"author", "Authors"},
	{"session", "Sessions giving authors access to group pads"},
	{"group", "Groups of pads"},
	{"chat", "The chat of a pad"},
	{"server", "The server"},
}

var camelWord = regexp.MustCompile(`[A-Z][a-z]*|[a-z]+|[0-9]+`)

// summary turns "getRevisionsCount" into "Get revisions count".
func summary(function string) string {
	words := camelWord.FindAllString(function, -1)
	for i, w := range words {
		if i == 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		} else if w != "ID" && w != "HTML" {
			words[i] = strings.ToLower(w)
		}
	}
	return strings.Join(words, " ")
}

// definition is the OpenAPI document of a version. Every function answers
// GET and POST; the document names POST only, as etherpad-lite's.
func (a *API) definition(version, serverURL string) fiber.Map {
	paths := fiber.Map{}
	for _, function := range sortedFunctions(version) {
		tag := functionTags[function]
		if tag == "" {
			tag = "pad"
		}
		params := make([]fiber.Map, 0, len(versions[version][function]))
		for _, p := range versions[version][function] {
			params = append(params, fiber.Map{
				"name": p, "in": "query", "schema": fiber.Map{"type": "string"},
			})
		}
		paths["/"+function] = fiber.Map{"post": fiber.Map{
			"operationId": function,
			"summary":     summary(function),
			"tags":        []string{tag},
			"parameters":  params,
			"responses": fiber.Map{
				"200": fiber.Map{"$ref": "#/components/responses/Success"},
				"401": fiber.Map{"$ref": "#/components/responses/Unauthorized"},
				"404": fiber.Map{"$ref": "#/components/responses/NotFound"},
				"500": fiber.Map{"$ref": "#/components/responses/InternalError"},
			},
		}}
	}
	tags := make([]fiber.Map, 0, len(tagDescriptions))
	for _, t := range tagDescriptions {
		tags = append(tags, fiber.Map{"name": t.name, "description": t.description})
	}
	schemes := fiber.Map{
		"ApiKey":       fiber.Map{"type": "apiKey", "in": "query", "name": "apikey"},
		"ApiKeyAlias":  fiber.Map{"type": "apiKey", "in": "query", "name": "api_key"},
		"ApiKeyHeader": fiber.Map{"type": "apiKey", "in": "header", "name": "apikey"},
		"Bearer":       fiber.Map{"type": "http", "scheme": "bearer"},
	}
	security := []fiber.Map{{"ApiKey": []string{}}, {"ApiKeyAlias": []string{}}, {"ApiKeyHeader": []string{}}, {"Bearer": []string{}}}
	response := func(description string) fiber.Map {
		return fiber.Map{"description": description, "content": fiber.Map{"application/json": fiber.Map{
			"schema": fiber.Map{"$ref": "#/components/schemas/Response"},
		}}}
	}
	return fiber.Map{
		"openapi": "3.0.2",
		"info": fiber.Map{
			"title":       "Etherpad API",
			"description": "etherpad-lite's HTTP API. Every function answers GET and POST; parameters go in the query string or the body. code 0 is ok, 1 a wrong parameter, 2 an internal error, 3 no such function, 4 no or wrong API key.",
			"version":     version,
		},
		"servers": []fiber.Map{{"url": serverURL}},
		"tags":    tags,
		"paths":   paths,
		"components": fiber.Map{
			"securitySchemes": schemes,
			"schemas": fiber.Map{"Response": fiber.Map{
				"type": "object",
				"properties": fiber.Map{
					"code":    fiber.Map{"type": "integer", "enum": []int{0, 1, 2, 3, 4}},
					"message": fiber.Map{"type": "string"},
					"data":    fiber.Map{"type": "object", "nullable": true},
				},
			}},
			"responses": fiber.Map{
				"Success":       response("ok (code 0) or a wrong parameter (code 1)"),
				"Unauthorized":  response("no or wrong API key (code 4)"),
				"NotFound":      response("no such function (code 3)"),
				"InternalError": response("internal error (code 2)"),
			},
		},
		"security": security,
	}
}

func (a *API) serveDefinition(c fiber.Ctx, version string) error {
	if _, ok := versions[version]; !ok {
		r, status := reply(&apiError{code: 3, message: "no such api version"})
		return c.Status(status).JSON(r)
	}
	return c.JSON(a.definition(version, a.origin(c)+"/api/"+version))
}

var hostPattern = regexp.MustCompile(`^[A-Za-z0-9.\-]+(:[0-9]{1,5})?$|^\[[0-9A-Fa-f:.]+\](:[0-9]{1,5})?$`)

// origin is the server URL of the document.
func (a *API) origin(c fiber.Ctx) string {
	publicURL := ""
	if st := a.store.RetrievedSettings; st != nil {
		publicURL = st.PublicURL
	}
	return serverOrigin(publicURL, c.Scheme(), c.Host())
}

// serverOrigin is publicURL when it is a plain http(s) origin, else the
// request's scheme (http or https only) and host; a host header that is
// not a host name becomes localhost, so nothing a client sends is copied
// into the document (etherpad-lite's generateServerForApiVersion).
func serverOrigin(publicURL, scheme, host string) string {
	if u, err := url.Parse(strings.TrimRight(publicURL, "/")); err == nil && publicURL != "" &&
		(u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil &&
		hostPattern.MatchString(u.Host) && u.RawQuery == "" && u.Fragment == "" {
		return u.Scheme + "://" + u.Host + u.Path
	}
	if scheme != "https" {
		scheme = "http"
	}
	if !hostPattern.MatchString(host) {
		host = "localhost"
	}
	return scheme + "://" + host
}
