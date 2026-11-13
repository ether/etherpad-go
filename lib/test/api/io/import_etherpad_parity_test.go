package io

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	libio "github.com/ether/etherpad-go/lib/io"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	modelpad "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of the remaining cases of etherpad-lite's ImportEtherpad spec, plus
// the atomicity of a replacing import.
func TestImportEtherpadParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "unknown records are ignored", Test: testImportIgnoresUnknownRecords},
		testutils.TestRunConfig{Name: "records of another pad are rejected, nothing is written", Test: testImportRejectsMixedPads},
		testutils.TestRunConfig{Name: "an existing author keeps name and colour", Test: testImportKeepsExistingAuthor},
		testutils.TestRunConfig{Name: "a new author is created from the file", Test: testImportCreatesNewAuthor},
		testutils.TestRunConfig{Name: "old files without revision authors", Test: testImportWithoutRevisionAuthors},
		testutils.TestRunConfig{Name: "a deletionToken record is ignored", Test: testImportIgnoresDeletionToken},
		testutils.TestRunConfig{Name: "pad-wide settings follow the creator rule", Test: testImportPadSettings},
		testutils.TestRunConfig{Name: "a failed write puts the previous pad back", Test: testImportRestoresOnFailure},
		testutils.TestRunConfig{Name: "authors list the imported pad", Test: testImportAuthorPads},
		testutils.TestRunConfig{Name: "order of records does not matter", Test: testImportRecordOrder},
		testutils.TestRunConfig{Name: "pad sub-keys other than revs and chat are ignored", Test: testImportIgnoresPadSubKeys},
	)
	testDb.StartTestDBHandler()
}

// liteExport is etherpad-lite's makeExport: one revision "foo", whose insert
// carries no author, and the author record of authorId.
func liteExport(authorId string) map[string]any {
	return map[string]any{
		"pad:testing": map[string]any{
			"atext":          map[string]string{"text": "foo\n", "attribs": "|1+4"},
			"pool":           map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
			"head":           0,
			"savedRevisions": []any{},
		},
		"globalAuthor:" + authorId: map[string]any{
			"colorId": "#000000", "name": "new", "timestamp": 1598747784631, "padIDs": "testing",
		},
		"pad:testing:revs:0": map[string]any{
			"changeset": "Z:1>3+3$foo",
			"meta": map[string]any{
				"author": "", "timestamp": 1597632398288,
				"pool":  map[string]any{"numToAttrib": map[string]any{}, "nextNum": 0},
				"atext": map[string]string{"text": "foo\n", "attribs": "|1+4"},
			},
		},
	}
}

func marshal(t *testing.T, v any) []byte {
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func padExists(t *testing.T, ds testutils.TestDataStore, padId string) bool {
	exists, err := ds.PadManager.DoesPadExist(padId)
	require.NoError(t, err)
	return *exists
}

func testImportIgnoresUnknownRecords(t *testing.T, ds testutils.TestDataStore) {
	data := liteExport("a.unknownRecords00001")
	data["maliciousDbKeyX"] = "value"
	data["sessionstorage:abc"] = map[string]any{"x": 1}
	require.NoError(t, ds.Importer.SetPadRaw("impUnknown", marshal(t, data), ""))
	text, _ := reload(t, ds, "impUnknown")
	assert.Equal(t, "foo\n", text)
}

func testImportRejectsMixedPads(t *testing.T, ds testutils.TestDataStore) {
	for name, mutate := range map[string]func(map[string]any){
		"pad record of another pad": func(d map[string]any) {
			d["pad:differentPadId"] = d["pad:testing"]
			delete(d, "pad:testing")
		},
		"revision of another pad": func(d map[string]any) {
			d["pad:differentPadId:revs:0"] = d["pad:testing:revs:0"]
			delete(d, "pad:testing:revs:0")
		},
		"author of another pad": func(d map[string]any) {
			for k, v := range d {
				if author, ok := v.(map[string]any); ok && k != "pad:testing" && author["padIDs"] != nil {
					author["padIDs"] = "differentPadId"
				}
			}
		},
		"author padIDs not a string": func(d map[string]any) {
			for k, v := range d {
				if author, ok := v.(map[string]any); ok && k != "pad:testing" && author["padIDs"] != nil {
					author["padIDs"] = map[string]any{"testing": 1}
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			authorId := "a.mixedPads" + string(rune('A'+len(name)%26)) + "0000000"
			data := liteExport(authorId)
			mutate(data)
			padId := "impMixed" + string(rune('A'+len(name)%26))
			err := ds.Importer.SetPadRaw(padId, marshal(t, data), "")
			require.Error(t, err)
			assert.True(t, errors.Is(err, libio.ErrInvalidEtherpadFile))
			assert.False(t, padExists(t, ds, padId), "all or nothing: no pad")
			existing, _ := ds.DS.GetAuthor(authorId)
			assert.Nil(t, existing, "all or nothing: no author")
		})
	}
}

func testImportKeepsExistingAuthor(t *testing.T, ds testutils.TestDataStore) {
	name := "existing"
	existing, err := ds.AuthorManager.CreateAuthor(&name)
	require.NoError(t, err)
	before, err := ds.AuthorManager.GetAuthor(existing.Id)
	require.NoError(t, err)
	require.NoError(t, ds.Importer.SetPadRaw("impExistingAuthor", marshal(t, liteExport(existing.Id)), ""))
	after, err := ds.AuthorManager.GetAuthor(existing.Id)
	require.NoError(t, err)
	require.NotNil(t, after.Name)
	assert.Equal(t, "existing", *after.Name, "a file cannot rename an existing author")
	assert.Equal(t, before.ColorId, after.ColorId, "nor change their colour")
	assert.NotEqual(t, "#000000", after.ColorId)
}

func testImportCreatesNewAuthor(t *testing.T, ds testutils.TestDataStore) {
	authorId := "a.newAuthorFromFile01"
	require.NoError(t, ds.Importer.SetPadRaw("impNewAuthor", marshal(t, liteExport(authorId)), ""))
	a, err := ds.AuthorManager.GetAuthor(authorId)
	require.NoError(t, err)
	require.NotNil(t, a.Name)
	assert.Equal(t, "new", *a.Name)
	assert.Equal(t, "#000000", a.ColorId)
}

func testImportWithoutRevisionAuthors(t *testing.T, ds testutils.TestDataStore) {
	data := liteExport("a.unused0000000000001")
	delete(data, "globalAuthor:a.unused0000000000001")
	rev := data["pad:testing:revs:0"].(map[string]any)
	delete(rev["meta"].(map[string]any), "author")
	require.NoError(t, ds.Importer.SetPadRaw("impNoMetaAuthor", marshal(t, data), ""))
	ds.PadManager.UnloadPad("impNoMetaAuthor")
	p, err := ds.PadManager.GetPad("impNoMetaAuthor", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "foo\n", p.Text())
	assert.Equal(t, 0, p.Head)
	author, err := p.GetRevisionAuthor(0)
	require.NoError(t, err)
	assert.Equal(t, "", *author)
}

func testImportIgnoresDeletionToken(t *testing.T, ds testutils.TestDataStore) {
	padId := "impDeletionToken"
	creator := createPadBy(t, ds, padId)
	tokens := pad.NewDeletionTokens(ds.DS)
	token, err := tokens.CreateIfAbsent(padId)
	require.NoError(t, err)
	data := map[string]any{}
	require.NoError(t, json.Unmarshal(etherpadFile(t, "pad:src", "imported\n", creator, false), &data))
	data["pad:src:deletionToken"] = map[string]any{"createdAt": 0, "hash": "0000"}
	require.NoError(t, ds.Importer.SetPadRaw(padId, marshal(t, data), "a.someImporter0000001"))
	assert.True(t, tokens.IsValid(padId, *token), "the pad keeps its own token")
}

func testImportPadSettings(t *testing.T, ds testutils.TestDataStore) {
	withSettings := func(t *testing.T, creator string) []byte {
		data := map[string]any{}
		require.NoError(t, json.Unmarshal(etherpadFile(t, "pad:src", "imported\n", creator, false), &data))
		data["pad:src"].(map[string]any)["padSettings"] = map[string]any{"enforceSettings": true}
		return marshal(t, data)
	}
	defaults := modelpad.PadSettingsDefaults{ShowChat: true}
	settingsOf := func(padId string) map[string]any {
		ds.PadManager.UnloadPad(padId)
		p, err := ds.PadManager.GetPad(padId, nil, nil)
		require.NoError(t, err)
		return p.GetPadSettings(defaults)
	}

	t.Run("not the creator: the target keeps its settings", func(t *testing.T) {
		padId := "impSettingsOther"
		creator := createPadBy(t, ds, padId)
		require.NoError(t, ds.Importer.SetPadRaw(padId, withSettings(t, creator), "a.notTheCreator000001"))
		assert.Equal(t, false, settingsOf(padId)["enforceSettings"])
	})

	t.Run("the creator: the file's settings", func(t *testing.T) {
		padId := "impSettingsCreator"
		creator := createPadBy(t, ds, padId)
		require.NoError(t, ds.Importer.SetPadRaw(padId, withSettings(t, creator), creator))
		assert.Equal(t, true, settingsOf(padId)["enforceSettings"])
	})

	t.Run("export carries them", func(t *testing.T) {
		padId := "impSettingsExport"
		createPadBy(t, ds, padId)
		p, err := ds.PadManager.GetPad(padId, nil, nil)
		require.NoError(t, err)
		_, err = p.SetPadSettings(map[string]any{"showChat": false}, defaults)
		require.NoError(t, err)
		exporter := libio.NewExportEtherpad(ds.Hooks, ds.PadManager, ds.DS, ds.Logger, testutils.GetTestAssets())
		export, err := exporter.GetPadRaw(padId, nil, nil)
		require.NoError(t, err)
		exported := marshal(t, export)
		assert.Contains(t, string(exported), `"padSettings"`)
		assert.Contains(t, string(exported), `showChat`)
	})
}

// failingRevisions fails storing a revision once, after it is armed (a
// transient database error in the middle of the import).
type failingRevisions struct {
	db.DataStore
	armed bool
}

func (f *failingRevisions) SaveRevision(padId string, rev int, changeset string, text db2.AText, pool db2.RevPool, authorId *string, timestamp int64) error {
	if f.armed {
		f.armed = false
		return errors.New("disk full")
	}
	return f.DataStore.SaveRevision(padId, rev, changeset, text, pool, authorId, timestamp)
}

func testImportRestoresOnFailure(t *testing.T, ds testutils.TestDataStore) {
	padId := "impRestore"
	creator := createPadBy(t, ds, padId)
	original, _ := reload(t, ds, padId)
	readOnly := ds.ReadOnlyManager.GetReadOnlyId(padId)

	store := &failingRevisions{DataStore: ds.DS}
	importer := libio.NewImporter(ds.PadManager, ds.AuthorManager, store, nil, ds.Hooks)
	store.armed = true
	err := importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "imported\n", creator, false), creator)
	require.Error(t, err)

	text, _ := reload(t, ds, padId)
	assert.Equal(t, original, text, "the pad is still there, unchanged")
	assert.Equal(t, readOnly, ds.ReadOnlyManager.GetReadOnlyId(padId))
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
}

// etherpad-go derives an author's pads from the revisions they wrote, so an
// author of the file's revisions lists the pad, beside the pads they already
// had (etherpad-lite "author pad IDs").
func testImportAuthorPads(t *testing.T, ds testutils.TestDataStore) {
	t.Run("new author", func(t *testing.T) {
		authorId := "a.authorPadsNew000001"
		require.NoError(t, ds.Importer.SetPadRaw("impAuthorPadsNew", etherpadFile(t, "pad:src", "x\n", authorId, false), ""))
		pads, err := ds.AuthorManager.ListPadsOfAuthor(authorId)
		require.NoError(t, err)
		assert.Equal(t, []string{"impAuthorPadsNew"}, pads)
	})
	t.Run("existing author on a different pad", func(t *testing.T) {
		other := "impAuthorPadsOther"
		authorId := createPadBy(t, ds, other)
		require.NoError(t, ds.Importer.SetPadRaw("impAuthorPadsTarget", etherpadFile(t, "pad:src", "x\n", authorId, false), ""))
		pads, err := ds.AuthorManager.ListPadsOfAuthor(authorId)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{other, "impAuthorPadsTarget"}, pads)
	})
	t.Run("existing author on the same pad", func(t *testing.T) {
		padId := "impAuthorPadsSame"
		authorId := createPadBy(t, ds, padId)
		require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "x\n", authorId, false), authorId))
		pads, err := ds.AuthorManager.ListPadsOfAuthor(authorId)
		require.NoError(t, err)
		assert.Equal(t, []string{padId}, pads)
	})
}

func testImportRecordOrder(t *testing.T, ds testutils.TestDataStore) {
	authorId := "a.recordOrder00000001"
	var records map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(etherpadFile(t, "pad:src", "foo\n", authorId, false), &records))
	keys := []string{"pad:src", "pad:src:revs:0", "globalAuthor:" + authorId}
	for n, perm := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		// A JSON object written in the permuted order.
		raw := "{"
		for i, k := range perm {
			if i > 0 {
				raw += ","
			}
			key, _ := json.Marshal(keys[k])
			raw += string(key) + ":" + string(records[keys[k]])
		}
		raw += "}"
		padId := "impOrder" + string(rune('A'+n))
		require.NoError(t, ds.Importer.SetPadRaw(padId, []byte(raw), ""), "%v", perm)
		text, _ := reload(t, ds, padId)
		assert.Equal(t, "foo\n", text, "%v", perm)
	}
}

func testImportIgnoresPadSubKeys(t *testing.T, ds testutils.TestDataStore) {
	padId := "impSubKeys"
	creator := createPadBy(t, ds, padId)
	data := map[string]any{}
	require.NoError(t, json.Unmarshal(etherpadFile(t, "pad:src", "imported\n", creator, false), &data))
	data["pad:src"].(map[string]any)["chatHead"] = 0
	data["pad:src:foo"] = map[string]any{"x": 1}
	data["pad:src:revs:foo"] = map[string]any{"x": 1}
	data["pad:src:chat:0"] = map[string]any{"text": "hi", "userId": creator, "time": 1}
	require.NoError(t, ds.Importer.SetPadRaw(padId, marshal(t, data), creator))

	ds.PadManager.UnloadPad(padId)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "imported\n", p.Text())
	assert.Equal(t, 0, p.Head, "revs:foo is no revision")
	chats, err := ds.DS.GetChatsOfPad(padId, 0, 0)
	require.NoError(t, err)
	require.Len(t, *chats, 1)
	assert.Equal(t, "hi", (*chats)[0].Message)
}
