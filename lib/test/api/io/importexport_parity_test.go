package io

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apiio "github.com/ether/etherpad-go/lib/api/io"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/server"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's api/importexportGetPost and export specs: the
// /p/:pad/import and /p/:pad/export routes behind webaccess.
func TestImportExportParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "imports a txt file into a new pad", Test: testIEImportTxt},
		testutils.TestRunConfig{Name: "export from the read-only id", Test: testIEExportReadOnly},
		testutils.TestRunConfig{Name: "document formats", Test: testIEDocumentFormats},
		testutils.TestRunConfig{Name: "etherpad import and export", Test: testIEEtherpad},
		testutils.TestRunConfig{Name: "unsupported file type", Test: testIEUnsupported},
		testutils.TestRunConfig{Name: "malformed .etherpad files are rejected", Test: testIEMalformed},
		testutils.TestRunConfig{Name: "revisions in txt and html export", Test: testIERevisions},
		testutils.TestRunConfig{Name: "import authorization", Test: testIEImportAuthorization},
		testutils.TestRunConfig{Name: "a failing export answers 500 without download headers", Test: testIEExportError},
	)
	testDb.StartTestDBHandler()
}

const iePadText = "This is the text of a test pad.\nSecond line.\n"

func ieCreds() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("user:user-password"))
}

// ieApp serves the import and export routes behind webaccess, with the
// test's own settings.
func ieApp(t *testing.T, ds testutils.TestDataStore, authn bool) *fiber.App {
	t.Helper()
	password := "user-password"
	initStore := ds.ToInitStore()
	ds.Settings.LoadTest = false
	ds.Settings.RequireAuthentication = authn
	ds.Settings.Users = map[string]settings.User{"user": {Password: &password}}
	app := fiber.New(server.FiberConfig())
	app.Use(server.RecoverMiddleware(ds.Logger))
	app.Use(func(c fiber.Ctx) error {
		return pad.CheckAccessWithHooks(c, ds.Logger, ds.Settings, ds.ReadOnlyManager, ds.Hooks)
	})
	initStore.C = app
	apiio.Init(initStore)
	return app
}

func ieDo(t *testing.T, app *fiber.App, req *http.Request, auth string) (*http.Response, []byte) {
	t.Helper()
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func ieImport(t *testing.T, app *fiber.App, padId, filename string, content []byte, auth string) (*http.Response, []byte) {
	t.Helper()
	body, ct := buildImportMultipartBody(t, filename, content)
	req := httptest.NewRequest("POST", "/p/"+padId+"/import", body)
	req.Header.Set("Content-Type", ct)
	return ieDo(t, app, req, auth)
}

func ieExport(t *testing.T, app *fiber.App, path, auth string) (*http.Response, []byte) {
	t.Helper()
	return ieDo(t, app, httptest.NewRequest("GET", path, nil), auth)
}

func iePadText_(t *testing.T, ds testutils.TestDataStore, padId string) string {
	t.Helper()
	ds.PadManager.UnloadPad(padId)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	return p.Text()
}

func ieExists(t *testing.T, ds testutils.TestDataStore, padId string) bool {
	t.Helper()
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	return *exists
}

func testIEImportTxt(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	resp, body := ieImport(t, app, "ieTxt", "test.txt", []byte(iePadText), "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	var res apiio.ImportResponse
	require.NoError(t, json.Unmarshal(body, &res))
	assert.Equal(t, 0, res.Code)
	assert.Equal(t, iePadText, iePadText_(t, ds, "ieTxt"))
}

func testIEExportReadOnly(t *testing.T, ds testutils.TestDataStore) {
	for _, authn := range []bool{false, true} {
		app := ieApp(t, ds, authn)
		auth := ""
		if authn {
			auth = ieCreds()
		}
		padId := "ieReadOnly"
		if !ieExists(t, ds, padId) {
			resp, body := ieImport(t, app, padId, "test.txt", []byte(iePadText), auth)
			require.Equal(t, 200, resp.StatusCode, string(body))
		}
		readOnlyId := ds.ReadOnlyManager.GetReadOnlyId(padId)
		for _, exportType := range []string{"html", "txt", "etherpad"} {
			name := exportType
			if authn {
				name += " with requireAuthentication"
			}
			t.Run(name, func(t *testing.T) {
				resp, body := ieExport(t, app, "/p/"+readOnlyId+"/export/"+exportType, auth)
				require.Equal(t, 200, resp.StatusCode, string(body))
				text := string(body)
				assert.Contains(t, text, "This is the text")
				assert.NotContains(t, text, padId, "the writable pad id must not leak")

				resp, body = ieImport(t, app, readOnlyId, "test."+exportType, body, auth)
				assert.Equal(t, 403, resp.StatusCode, "re-import to the read-only id: %s", body)
				assert.Equal(t, iePadText, iePadText_(t, ds, padId))

				target := padId + "_import_" + exportType
				if authn {
					target += "_authn"
				}
				resp, out := ieImport(t, app, target, "test."+exportType, []byte(text), auth)
				assert.Equal(t, 200, resp.StatusCode, "re-import to a read-write id: %s", out)
				assert.Contains(t, iePadText_(t, ds, target), "This is the text")
			})
		}
	}
}

func zipEntry(t *testing.T, data []byte, name string) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			require.NoError(t, err)
			defer rc.Close()
			b, _ := io.ReadAll(rc)
			return string(b)
		}
	}
	t.Fatalf("%s not in the archive", name)
	return ""
}

func testIEDocumentFormats(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	padId := "ieFormats"
	resp, body := ieImport(t, app, padId, "test.txt", []byte(iePadText), "")
	require.Equal(t, 200, resp.StatusCode, string(body))

	const docxType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	const odtType = "application/vnd.oasis.opendocument.text"
	for _, tc := range []struct {
		path, contentType, magic, reimportAs string
	}{
		{"word", docxType, "PK\x03\x04", "test.docx"},
		{"docx", docxType, "PK\x03\x04", "test.docx"},
		{"doc", docxType, "PK\x03\x04", ""},
		{"open", odtType, "PK\x03\x04", "test.odt"},
		{"odt", odtType, "PK\x03\x04", "test.odt"},
		{"pdf", "application/pdf", "%PDF-", "test.pdf"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			resp, body := ieExport(t, app, "/p/"+padId+"/export/"+tc.path, "")
			require.Equal(t, 200, resp.StatusCode, string(body))
			assert.Contains(t, resp.Header.Get("Content-Type"), tc.contentType)
			require.Greater(t, len(body), 200)
			assert.Equal(t, tc.magic, string(body[:len(tc.magic)]))
			if tc.contentType == odtType {
				assert.Equal(t, odtType, zipEntry(t, body, "mimetype"))
			}
			if tc.reimportAs == "" {
				return
			}
			target := padId + "_from_" + tc.path
			resp, out := ieImport(t, app, target, tc.reimportAs, body, "")
			require.Equal(t, 200, resp.StatusCode, string(out))
			text := iePadText_(t, ds, target)
			assert.Contains(t, text, "This is the text of a test pad.")
			assert.Contains(t, text, "Second line.")
		})
	}
}

// ieEtherpadFile is an .etherpad export of a pad whose only line is the
// indented bullet "hello" (etherpad-lite's test.etherpad fixture), made by
// importing the HTML and exporting the pad.
func ieEtherpadFile(t *testing.T, app *fiber.App) []byte {
	t.Helper()
	html := `<html><body><ul class="bullet"><li><ul class="bullet"><li>hello</li></ul></li></ul></body></html>`
	resp, body := ieImport(t, app, "ieEtherpadSource", "test.html", []byte(html), "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	resp, body = ieExport(t, app, "/p/ieEtherpadSource/export/etherpad", "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	return body
}

func testIEEtherpad(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	padId := "ieEtherpad"
	file := ieEtherpadFile(t, app)
	resp, body := ieImport(t, app, padId, "test.etherpad", file, "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	var res apiio.ImportResponse
	require.NoError(t, json.Unmarshal(body, &res))
	assert.Equal(t, apiio.ImportResponse{Code: 0, Message: "ok", Data: apiio.ImportData{DirectDatabaseAccess: true}}, res)

	resp, body = ieExport(t, app, "/p/"+padId+"/export/etherpad", "")
	require.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, string(body), "hello")

	resp, body = ieExport(t, app, "/p/"+padId+"/export/html", "")
	require.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Contains(t, string(body), `<ul class="bullet"><li><ul class="bullet"><li>`)
	assert.Contains(t, string(body), `hello</span></ul></li></ul>`)

	t.Run("a file that changes the creator is refused", func(t *testing.T) {
		target := "ieEtherpadCreator"
		resp, body := ieImport(t, app, target, "test.txt", []byte(iePadText), "")
		require.Equal(t, 200, resp.StatusCode, string(body))
		resp, body = ieImport(t, app, target, "test.etherpad", file, "")
		assert.Equal(t, 400, resp.StatusCode)
		require.NoError(t, json.Unmarshal(body, &res))
		assert.Equal(t, "permission", res.Message)
		assert.Equal(t, iePadText, iePadText_(t, ds, target))
	})
}

func testIEUnsupported(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	ds.Settings.AllowUnknownFileEnds = false
	resp, body := ieImport(t, app, "ieUnsupported", "test.xasdasdxx", []byte(iePadText), "")
	assert.Equal(t, 400, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "json")
	var res apiio.ImportResponse
	require.NoError(t, json.Unmarshal(body, &res))
	assert.Equal(t, 1, res.Code)
	assert.Equal(t, "uploadFailed", res.Message)
}

func ieGoodExport() map[string]any {
	return map[string]any{
		"pad:testing": map[string]any{
			"atext":          map[string]any{"text": "foo\n", "attribs": "|1+4"},
			"pool":           map[string]any{"numToAttrib": map[string]any{"0": []string{"author", "a.foo"}}, "nextNum": 1},
			"chatHead":       0,
			"head":           0,
			"savedRevisions": []any{},
		},
		"globalAuthor:a.foo": map[string]any{"colorId": "#000000", "name": "author foo", "timestamp": 1598747784631, "padIDs": "testing"},
		"pad:testing:revs:0": map[string]any{
			"changeset": "Z:1>3+3$foo",
			"meta": map[string]any{
				"author": "a.foo", "timestamp": 1597632398288,
				"pool":  map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
				"atext": map[string]any{"text": "foo\n", "attribs": "|1+4"},
			},
		},
		"pad:testing:chat:0": map[string]any{"text": "this is a test", "authorId": "a.foo", "time": 1637966993265},
	}
}

func testIEMalformed(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	importRecords := func(t *testing.T, padId string, records map[string]any) (*http.Response, []byte) {
		b, err := json.Marshal(records)
		require.NoError(t, err)
		return ieImport(t, app, padId, "test.etherpad", b, "")
	}

	t.Run("the good file imports", func(t *testing.T) {
		resp, body := importRecords(t, "ieMalformedGood", ieGoodExport())
		require.Equal(t, 200, resp.StatusCode, string(body))
		resp, body = ieExport(t, app, "/p/ieMalformedGood/export/txt", "")
		require.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, string(body), "foo")
		chats, err := ds.DS.GetChatsOfPad("ieMalformedGood", 0, 0)
		require.NoError(t, err)
		require.Len(t, *chats, 1)
		require.NotNil(t, (*chats)[0].AuthorId, "etherpad-lite's authorId field is read")
		assert.Equal(t, "a.foo", *(*chats)[0].AuthorId)
	})

	for name, mutate := range map[string]func(r map[string]any){
		"missing rev": func(r map[string]any) { delete(r, "pad:testing:revs:0") },
		"bad changeset": func(r map[string]any) {
			r["pad:testing:revs:0"].(map[string]any)["changeset"] = "garbage"
		},
		"missing attrib in pool": func(r map[string]any) {
			r["pad:testing"].(map[string]any)["pool"].(map[string]any)["nextNum"] = 2
		},
		"extra attrib in pool": func(r map[string]any) {
			r["pad:testing"].(map[string]any)["pool"].(map[string]any)["numToAttrib"].(map[string]any)["1"] = []string{"key", "value"}
		},
		"changeset refers to non-existent attrib": func(r map[string]any) {
			r["pad:testing:revs:1"] = map[string]any{
				"changeset": "Z:4>4*1+4$asdf",
				"meta":      map[string]any{"author": "a.foo", "timestamp": 1597632398288},
			}
			p := r["pad:testing"].(map[string]any)
			p["head"] = 1
			p["atext"] = map[string]any{"text": "asdffoo\n", "attribs": "*1+4|1+4"}
		},
		"pad atext does not match": func(r map[string]any) {
			r["pad:testing"].(map[string]any)["atext"] = map[string]any{"text": "foo\n", "attribs": "*0|1+4"}
		},
		"missing chat message": func(r map[string]any) { delete(r, "pad:testing:chat:0") },
	} {
		t.Run(name, func(t *testing.T) {
			padId := "ieMalformed" + strings.ReplaceAll(name, " ", "")
			records := ieGoodExport()
			mutate(records)
			resp, body := importRecords(t, padId, records)
			assert.Equal(t, 400, resp.StatusCode, string(body))
			// Nothing of the file is kept: the pad stays as the import
			// handler created it (empty) or does not exist.
			if ieExists(t, ds, padId) {
				assert.NotContains(t, iePadText_(t, ds, padId), "foo")
			}
		})
	}
}

func testIERevisions(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	padId := "ieRevisions"
	records := map[string]any{
		"pad:testing": map[string]any{
			"atext":          map[string]any{"text": "oofoo\n", "attribs": "|1+6"},
			"pool":           map[string]any{"numToAttrib": map[string]any{"0": []string{"author", "a.foo"}}, "nextNum": 1},
			"head":           2,
			"savedRevisions": []any{},
		},
		"globalAuthor:a.foo": map[string]any{"colorId": "#000000", "name": "author foo", "timestamp": 1598747784631, "padIDs": "testing"},
		"pad:testing:revs:0": map[string]any{"changeset": "Z:1>3+3$foo", "meta": map[string]any{
			"author": "a.foo", "timestamp": 1597632398288,
			"pool":  map[string]any{"nextNum": 1, "numToAttrib": map[string]any{"0": []string{"author", "a.foo"}}},
			"atext": map[string]any{"text": "foo\n", "attribs": "|1+4"},
		}},
		"pad:testing:revs:1": map[string]any{"changeset": "Z:4>1+1$o", "meta": map[string]any{"author": "a.foo", "timestamp": 1597632398288}},
		"pad:testing:revs:2": map[string]any{"changeset": "Z:5>1+1$o", "meta": map[string]any{"author": "a.foo", "timestamp": 1597632398288}},
	}
	b, err := json.Marshal(records)
	require.NoError(t, err)
	resp, body := ieImport(t, app, padId, "test.etherpad", b, "")
	require.Equal(t, 200, resp.StatusCode, string(body))

	for _, tc := range []struct {
		rev, txt, html string
		status         int
	}{
		{"", "oofoo\n", "oofoo", 200},
		{"1", "ofoo\n", "ofoo", 200},
		{"2", "oofoo\n", "oofoo", 200},
		{"5", "oofoo\n", "oofoo", 200}, // above head: the head revision
		// etherpad-lite parses "1test" as 1 and answers 500 for "test1";
		// etherpad-go refuses both as invalid revisions.
		{"1test", "", "", 400},
		{"test1", "", "", 400},
	} {
		path := "/p/" + padId
		if tc.rev != "" {
			path += "/" + tc.rev
		}
		resp, body := ieExport(t, app, path+"/export/txt", "")
		require.Equal(t, tc.status, resp.StatusCode, "txt rev %q: %s", tc.rev, body)
		if tc.status != 200 {
			assert.Empty(t, resp.Header.Get("Content-Disposition"))
			continue
		}
		assert.Equal(t, tc.txt, string(body), "txt rev %q", tc.rev)
		resp, body = ieExport(t, app, path+"/export/html", "")
		require.Equal(t, 200, resp.StatusCode)
		assert.Contains(t, string(body), ">"+tc.html+"<", "html rev %q", tc.rev)
		if tc.html == "ofoo" {
			assert.NotContains(t, string(body), "oofoo")
		}
	}
}

func testIEImportAuthorization(t *testing.T, ds testutils.TestDataStore) {
	level := "create"
	hookId := ds.Hooks.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) {
		ctx.Grant(level)
	})
	defer ds.Hooks.DequeueHook(hooks.AuthorizeString, hookId)

	n := 0
	for _, tc := range []struct {
		name        string
		authn       bool
		user        bool
		level       string
		exists      bool
		wantStatus  int
		wantReplace bool
	}{
		{"!authn !exist -> create", false, false, "create", false, 200, true},
		{"!authn exist -> replace", false, false, "create", true, 200, true},
		{"authn anonymous !exist -> fail", true, false, "create", false, 401, false},
		{"authn anonymous exist -> fail", true, false, "create", true, 401, false},
		{"authn user create !exist -> create", true, true, "create", false, 200, true},
		{"authn user modify !exist -> fail", true, true, "modify", false, 403, false},
		{"authn user readonly !exist -> fail", true, true, "readOnly", false, 403, false},
		{"authn user create exist -> replace", true, true, "create", true, 200, true},
		{"authn user modify exist -> replace", true, true, "modify", true, 200, true},
		{"authn user readonly exist -> fail", true, true, "readOnly", true, 403, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n++
			padId := "ieAuthz" + string(rune('a'+n))
			app := ieApp(t, ds, tc.authn)
			ds.Settings.RequireAuthorization = true
			level = tc.level
			if tc.exists {
				text := "before import"
				_, err := ds.PadManager.GetPad(padId, &text, nil)
				require.NoError(t, err)
			}
			auth := ""
			if tc.user {
				auth = ieCreds()
			}
			resp, body := ieImport(t, app, padId, "test.txt", []byte(iePadText), auth)
			assert.Equal(t, tc.wantStatus, resp.StatusCode, string(body))
			switch {
			case tc.wantReplace:
				assert.Equal(t, iePadText, iePadText_(t, ds, padId))
			case tc.exists:
				assert.Equal(t, "before import\n", iePadText_(t, ds, padId))
			default:
				assert.False(t, ieExists(t, ds, padId))
			}
		})
	}
}

func testIEExportError(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	padId := "ieExportError"
	resp, body := ieImport(t, app, padId, "test.txt", []byte(iePadText), "")
	require.Equal(t, 200, resp.StatusCode, string(body))
	// A plugin that fails while the document is built.
	hookId := ds.Hooks.EnqueueHook("getLineHTMLForExport", func(ctx any) { panic("plugin failure") })
	defer ds.Hooks.DequeueHook("getLineHTMLForExport", hookId)

	resp, body = ieExport(t, app, "/p/"+padId+"/export/html", "")
	assert.Equal(t, 500, resp.StatusCode, string(body))
	assert.Empty(t, resp.Header.Get("Content-Disposition"), "the error must not be saved as the document")
	assert.NotContains(t, string(body), "plugin failure", "no internal details")
}
