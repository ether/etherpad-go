// Package legacy serves etherpad-lite's HTTP API, /api/{version}/{function},
// for clients written against it (Moodle, Nextcloud, WordPress plugins and
// the like). Calls are checked the way etherpad-lite checks them and then
// run by the REST API under /admin/api, so both behave the same.
//
// Requests: GET or POST; parameters in the query string, a form body or a
// JSON body (body before query). Responses: {"code", "message", "data"},
// where code 0 is ok, 1 a wrong parameter (HTTP 200), 2 an internal error
// (500), 3 no such function (404) and 4 no or wrong API key (401).
package legacy

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/session"
	"github.com/ether/etherpad-go/lib/api/stats"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

// Response is the reply of every call.
type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// apiError is a failed call: code 1 (wrong parameter), 2 (internal),
// 3 (no such function) or 4 (not authorised).
type apiError struct {
	code    int
	message string
}

func wrongParam(format string, a ...any) *apiError {
	return &apiError{code: 1, message: fmt.Sprintf(format, a...)}
}

var internalError = &apiError{code: 2, message: "internal error"}

// Authenticator checks a bearer token when the API key is not used.
type Authenticator func(token string) bool

// API serves the legacy HTTP API.
type API struct {
	store *lib.InitStore
	inner fasthttp.RequestHandler
	// apiKey is the key of authenticationMethod "apikey"; empty otherwise.
	apiKey string
	bearer Authenticator
}

// APIKeyFile is where the API key is kept, like etherpad-lite's APIKEY.txt.
func APIKeyFile(root string) string {
	if p := os.Getenv("EP_APIKEY_FILE"); p != "" {
		return p
	}
	return filepath.Join(root, "APIKEY.txt")
}

// LoadAPIKey reads the API key, creating the file with a random key when it
// does not exist (etherpad-lite does the same).
func LoadAPIKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return strings.TrimSpace(string(b)), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	key := utils.RandomString(32)
	if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
		return "", err
	}
	return key, nil
}

// New builds the API. With authenticationMethod "apikey" calls need the key
// (apikey, api_key or authorization parameter); otherwise a bearer token
// accepted by bearer, as for /admin/api.
func New(store *lib.InitStore, apiKey string, bearer Authenticator) *API {
	inner := fiber.New(fiber.Config{Immutable: true})
	routes := *store
	routes.PrivateAPI = inner
	pad.Init(&routes)
	groups.Init(&routes)
	session.Init(&routes)
	author.Init(&routes)
	inner.Get("/stats", stats.GetStats(&routes))
	return &API{store: store, inner: inner.Handler(), apiKey: apiKey, bearer: bearer}
}

// Register adds the routes: /api (current version), /api/{version}/{function}.
func (a *API) Register(app *fiber.App) {
	app.Get("/api", a.currentVersion)
	app.Get("/api/", a.currentVersion)
	app.Get("/api/openapi.json", func(c fiber.Ctx) error { return a.serveDefinition(c, LatestVersion) })
	app.Get("/api/:version/:function", a.serve)
	app.Post("/api/:version/:function", a.serve)
}

func (a *API) currentVersion(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"currentVersion": LatestVersion})
}

func (a *API) serve(c fiber.Ctx) error {
	if c.Params("function") == "openapi.json" && c.Method() == fiber.MethodGet {
		return a.serveDefinition(c, c.Params("version"))
	}
	resp, status := a.handle(c)
	return c.Status(status).JSON(resp)
}

func (a *API) handle(c fiber.Ctx) (Response, int) {
	version := c.Params("version")
	function := c.Params("function")
	params, ok := versions[version]
	if !ok {
		return reply(&apiError{code: 3, message: "no such api version"})
	}
	names, ok := params[function]
	if !ok {
		return reply(&apiError{code: 3, message: "no such function"})
	}
	fields, err := requestFields(c)
	if err != nil {
		return reply(wrongParam("%s", err.Error()))
	}
	if e := a.authorise(c, fields); e != nil {
		return reply(e)
	}
	// Only the parameters of the called version count.
	args := args{}
	for _, n := range names {
		if v, ok := fields[n]; ok {
			args[n] = v
		}
	}
	data, e := a.call(function, args)
	if e != nil {
		return reply(e)
	}
	return Response{Code: 0, Message: "ok", Data: data}, fiber.StatusOK
}

func reply(e *apiError) (Response, int) {
	status := fiber.StatusOK
	switch e.code {
	case 2:
		status = fiber.StatusInternalServerError
	case 3:
		status = fiber.StatusNotFound
	case 4:
		status = fiber.StatusUnauthorized
	}
	return Response{Code: e.code, Message: e.message, Data: nil}, status
}

func (a *API) authorise(c fiber.Ctx, fields map[string]string) *apiError {
	if a.apiKey != "" {
		given := fields["apikey"]
		if given == "" {
			given = fields["api_key"]
		}
		if given == "" {
			given = fields["authorization"]
		}
		if given == "" {
			given = c.Get(fiber.HeaderAuthorization)
		}
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(given)), []byte(a.apiKey)) != 1 {
			return &apiError{code: 4, message: "no or wrong API Key"}
		}
		return nil
	}
	header := c.Get(fiber.HeaderAuthorization)
	if header == "" {
		return &apiError{code: 4, message: "no or wrong API Key"}
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if a.bearer == nil || !a.bearer(token) {
		return &apiError{code: 4, message: "no or wrong OAuth token"}
	}
	return nil
}

// requestFields merges the parameters: body before query.
func requestFields(c fiber.Ctx) (map[string]string, error) {
	fields := map[string]string{}
	c.RequestCtx().QueryArgs().VisitAll(func(k, v []byte) {
		fields[string(k)] = string(v)
	})
	if c.Method() != fiber.MethodPost {
		return fields, nil
	}
	ct := strings.ToLower(c.Get(fiber.HeaderContentType))
	switch {
	case strings.HasPrefix(ct, fiber.MIMEApplicationJSON):
		if len(c.Body()) == 0 {
			return fields, nil
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return nil, fmt.Errorf("invalid JSON body")
		}
		for k, raw := range body {
			if v, ok := jsonScalar(raw); ok {
				fields[k] = v
			}
		}
	case strings.HasPrefix(ct, fiber.MIMEMultipartForm):
		form, err := c.MultipartForm()
		if err != nil {
			return nil, fmt.Errorf("invalid form body")
		}
		for k, v := range form.Value {
			if len(v) > 0 {
				fields[k] = v[0]
			}
		}
	default:
		c.RequestCtx().PostArgs().VisitAll(func(k, v []byte) {
			fields[string(k)] = string(v)
		})
	}
	return fields, nil
}

// jsonScalar turns a JSON body value into the string a form would carry.
// null counts as absent.
func jsonScalar(raw json.RawMessage) (string, bool) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	default:
		return string(raw), true
	}
}

// args are the parameters of one call.
type args map[string]string

func (a args) has(k string) bool { _, ok := a[k]; return ok }

// restCall runs a REST API request in-process.
func (a *API) restCall(method, path string, query url.Values, body any) (int, []byte) {
	var req fasthttp.Request
	req.Header.SetMethod(method)
	uri := path
	if len(query) > 0 {
		uri += "?" + query.Encode()
	}
	req.SetRequestURI(uri)
	if body != nil {
		b, _ := json.Marshal(body)
		req.Header.SetContentType(fiber.MIMEApplicationJSON)
		req.SetBody(b)
	}
	var ctx fasthttp.RequestCtx
	ctx.Init(&req, nil, nil)
	a.inner(&ctx)
	return ctx.Response.StatusCode(), append([]byte(nil), ctx.Response.Body()...)
}

// rest runs a REST call and turns its answer into data or an error.
// notFound is the message of a 404.
func (a *API) rest(method, path string, query url.Values, body any, notFound string) (any, *apiError) {
	status, out := a.restCall(method, path, query, body)
	switch {
	case status >= 200 && status < 300:
		var data any
		if len(out) == 0 || json.Unmarshal(out, &data) != nil {
			return nil, nil
		}
		return data, nil
	case status == fiber.StatusNotFound && notFound != "":
		return nil, wrongParam("%s", notFound)
	case status >= 500:
		return nil, internalError
	default:
		var e struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(out, &e) != nil || e.Message == "" {
			e.Message = strings.TrimSpace(string(out))
		}
		for _, prefix := range []string{"Invalid parameter: ", "Missing parameter: "} {
			e.Message = strings.TrimPrefix(e.Message, prefix)
		}
		if e.Message == "" {
			e.Message = "bad request"
		}
		return nil, wrongParam("%s", e.Message)
	}
}

func esc(s string) string { return url.PathEscape(s) }

// parseIntJS parses like JavaScript's parseInt(s, 10): leading digits,
// anything after them ignored.
func parseIntJS(s string) (int, bool) {
	s = strings.TrimSpace(s)
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	start := end
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0, false
	}
	return n, true
}

// checkRev is etherpad-lite's checkValidRev.
func checkRev(s string) (int, *apiError) {
	n, ok := parseIntJS(s)
	if !ok {
		return 0, wrongParam("rev is not a number")
	}
	if n < 0 {
		return 0, wrongParam("rev is not a negative number")
	}
	return n, nil
}

// boolArg reads a boolean the way etherpad-lite does: the string "true"
// (any case) or a JSON true.
func boolArg(s string) bool { return strings.EqualFold(strings.TrimSpace(s), "true") }

// number parses a JSON-ish number.
func number(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}
