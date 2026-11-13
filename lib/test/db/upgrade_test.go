package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/db/migrations"
	"github.com/ether/etherpad-go/lib/hooks"
	dbmodels "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// An installation upgraded from an older release keeps its pads: a database
// left at schema version v (for every v from 1 on) with pads in it is
// migrated to the current schema, and the pads load and read the same.
//
// The data is written by the current code and copied into the old schema
// column by column, so it has the shape every release since 1 writes.
func TestUpgradeFromEveryOlderSchema(t *testing.T) {
	latest := 0
	for _, m := range migrations.GetMigrations() {
		latest = max(latest, m.Version)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.db")
	want := writeSourcePads(t, src)

	for v := 1; v < latest; v++ {
		t.Run(fmt.Sprintf("from schema %d", v), func(t *testing.T) {
			old := filepath.Join(dir, fmt.Sprintf("v%d.db", v))
			copyIntoOldSchema(t, src, old, v)

			// The release's start-up runs the remaining migrations.
			store, err := db.NewSQLiteDB(old)
			require.NoError(t, err)
			defer store.Close()
			checkPads(t, store, want)
		})
	}
}

type padWant struct {
	text, attribs, readOnly string
	head, chatHead          int
	chat                    []string
	savedRevs               int
}

const upgradePad = "upgradePad"

func writeSourcePads(t *testing.T, path string) padWant {
	t.Helper()
	store, err := db.NewSQLiteDB(path)
	require.NoError(t, err)
	defer store.Close()
	h := hooks.NewHook()
	pm := pad.NewManager(store, &h)
	am := author.NewManager(store)
	name := "Upgrade Author"
	a, err := am.CreateAuthor(&name)
	require.NoError(t, err)
	text := "Grüße 😀 first line"
	p, err := pm.GetPad(upgradePad, &text, &a.Id)
	require.NoError(t, err)
	require.NoError(t, p.SetText(p.Text()+"second line\n", &a.Id))
	bold := p.Pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, nil)
	authorNum := p.Pool.PutAttrib(apool.Attribute{Key: "author", Value: a.Id}, nil)
	_, err = p.AppendRevision(fmt.Sprintf("Z:%s>0=6*%s*%s=1$", base36(len([]rune(p.Text()))), base36(authorNum), base36(bold)), &a.Id)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = p.AppendChatMessage(&a.Id, int64(1000+i), fmt.Sprintf("chat %d", i))
		require.NoError(t, err)
	}
	require.NoError(t, p.AddSavedRevision(a.Id))
	ro := pad.NewReadOnlyManager(store).GetReadOnlyId(upgradePad)
	require.NoError(t, p.Check())
	return padWant{text: p.Text(), attribs: p.AText.Attribs, readOnly: ro, head: p.Head, chatHead: p.ChatHead,
		chat: []string{"chat 0", "chat 1", "chat 2"}, savedRevs: 1}
}

func base36(n int) string { return itoa36(n) }

func itoa36(n int) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if n == 0 {
		return "0"
	}
	var s []byte
	for n > 0 {
		s = append([]byte{digits[n%36]}, s...)
		n /= 36
	}
	return string(s)
}

func columns(t *testing.T, conn *sql.DB, table string) []string {
	t.Helper()
	rows, err := conn.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		cols = append(cols, name)
	}
	return cols
}

func tables(t *testing.T, conn *sql.DB) []string {
	t.Helper()
	rows, err := conn.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations'`)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out = append(out, n)
	}
	return out
}

// copyIntoOldSchema creates a database at schema version v and copies the
// source rows into it, only the columns that version has.
func copyIntoOldSchema(t *testing.T, srcPath, dstPath string, v int) {
	t.Helper()
	dst, err := sql.Open("sqlite", dstPath+"?_pragma=foreign_keys(OFF)")
	require.NoError(t, err)
	defer dst.Close()
	require.NoError(t, migrations.NewMigrationManagerUpTo(dst, migrations.DialectSQLite, v).Run())

	_, err = dst.Exec(`ATTACH DATABASE ? AS src`, srcPath)
	require.NoError(t, err)
	for _, table := range tables(t, dst) {
		cols := columns(t, dst, table)
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = `"` + c + `"`
		}
		list := strings.Join(quoted, ",")
		_, err := dst.Exec(fmt.Sprintf(`INSERT INTO main."%s" (%s) SELECT %s FROM src."%s"`, table, list, list, table))
		require.NoError(t, err, "copying %s", table)
	}
	var n int
	require.NoError(t, dst.QueryRow(`SELECT COUNT(*) FROM main.padRev WHERE id = ?`, upgradePad).Scan(&n))
	require.Greater(t, n, 0)
}

func checkPads(t *testing.T, store db.DataStore, want padWant) {
	t.Helper()
	h := hooks.NewHook()
	pm := pad.NewManager(store, &h)
	exists, err := pm.DoesPadExist(upgradePad)
	require.NoError(t, err)
	require.True(t, *exists)
	p, err := pm.GetPad(upgradePad, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
	assert.Equal(t, want.text, p.Text())
	assert.Equal(t, want.attribs, p.AText.Attribs)
	assert.Equal(t, want.head, p.Head)
	assert.Equal(t, want.chatHead, p.ChatHead)
	assert.Len(t, p.SavedRevisions, want.savedRevs)
	assert.Equal(t, want.readOnly, pad.NewReadOnlyManager(store).GetReadOnlyId(upgradePad))
	msgs, err := p.GetChatMessages(0, p.ChatHead)
	require.NoError(t, err)
	var texts []string
	for _, m := range *msgs {
		texts = append(texts, m.Message)
	}
	assert.Equal(t, want.chat, texts)

	// Still editable after the upgrade.
	editor := "a.upgradeEditor00001"
	require.NoError(t, store.SaveAuthor(dbAuthor(editor)))
	require.NoError(t, p.SetText(p.Text()+"after upgrade\n", &editor))
	pm.UnloadPad(upgradePad)
	p, err = pm.GetPad(upgradePad, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
	assert.Contains(t, p.Text(), "after upgrade")
}

func dbAuthor(id string) dbmodels.AuthorDB {
	return dbmodels.AuthorDB{ID: id, ColorId: "#123456", Timestamp: 1}
}
