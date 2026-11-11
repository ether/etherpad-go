package pad

import (
	"sort"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	modelpad "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/require"
)

// Every insert must carry an author attribute, also for callers that do not
// come through the websocket (HTTP API, imports, plugins). Go counterpart of
// etherpad-lite's padInsertAuthorInvariant spec (upstream #7773).
func TestInsertsMustCarryAnAuthor(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		m := newInstance(t, store)
		author := newTestAuthor(t, store)
		emptyText := ""

		withNewPad := func(t *testing.T, padID string, fn func(p *modelpad.Pad)) {
			t.Helper()
			unlock := mustLock(m.LockPad(padID))
			defer unlock()
			p, err := m.GetPad(padID, &emptyText, nil)
			require.NoError(t, err)
			require.Equal(t, "\n", p.Text())
			fn(p)
		}

		t.Run("an insert without attributes is rejected", func(t *testing.T) {
			withNewPad(t, "insertNoAttribs", func(p *modelpad.Pad) {
				_, err := p.AppendRevision("Z:1>5+5$world", &author)
				require.ErrorIs(t, err, modelpad.ErrInsertWithoutAuthor)
				require.Equal(t, "\n", p.Text(), "nothing is applied")
				require.Equal(t, 0, p.Head)
			})
		})

		t.Run("one unattributed insert rejects the whole changeset", func(t *testing.T) {
			withNewPad(t, "insertPartly", func(p *modelpad.Pad) {
				num := p.Pool.PutAttrib(authorAttrib(author), nil)
				require.Equal(t, 0, num)
				// The first insert has no author, the second has one.
				_, err := p.AppendRevision("Z:1>a+5*0+5$worldhello", &author)
				require.ErrorIs(t, err, modelpad.ErrInsertWithoutAuthor)
				require.Equal(t, "\n", p.Text(), "no partial application")
			})
		})

		t.Run("an insert with an author is accepted", func(t *testing.T) {
			withNewPad(t, "insertWithAuthor", func(p *modelpad.Pad) {
				require.NoError(t, p.SetText("hello\n", &author))
				require.Equal(t, "hello\n", p.Text())
			})
		})

		t.Run("keeps and deletions need no author", func(t *testing.T) {
			withNewPad(t, "deleteNoAuthor", func(p *modelpad.Pad) {
				require.NoError(t, p.SetText("hello world\n", &author))
				// Delete "hello " with a raw changeset that has no attributes.
				_, err := p.AppendRevision("Z:c<6-6$", &author)
				require.NoError(t, err)
				require.Equal(t, "world\n", p.Text())
				// Keep without attributes, then delete.
				_, err = p.AppendRevision("Z:6<1=4-1$", &author)
				require.NoError(t, err)
				require.Equal(t, "worl\n", p.Text())
			})
		})

		t.Run("text set without an author goes to the system author", func(t *testing.T) {
			withNewPad(t, "insertSystemAuthor", func(p *modelpad.Pad) {
				require.NoError(t, p.SetText("unattributed\n", nil))
				requireEveryCharHasAuthor(t, p)
				require.Contains(t, authorsInPool(p), modelpad.SystemAuthorId)
			})
		})
	})
}

// The initial text of a new pad is an insert too.
func TestInitialPadTextCarriesAnAuthor(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		m := newInstance(t, store)
		creator := newTestAuthor(t, store)

		load := func(t *testing.T, padID string, text *string, author *string) *modelpad.Pad {
			t.Helper()
			unlock := mustLock(m.LockPad(padID))
			defer unlock()
			p, err := m.GetPad(padID, text, author)
			require.NoError(t, err)
			return p
		}

		t.Run("provided text belongs to the given author", func(t *testing.T) {
			text := "provided content"
			p := load(t, "initProvided", &text, &creator)
			require.Equal(t, text+"\n", p.Text())
			requireEveryCharHasAuthor(t, p)
			require.Equal(t, []string{creator}, authorsInPool(p))
		})

		t.Run("provided text without an author goes to the system author", func(t *testing.T) {
			text := "from the API"
			p := load(t, "initNoAuthor", &text, nil)
			requireEveryCharHasAuthor(t, p)
			require.Equal(t, []string{modelpad.SystemAuthorId}, authorsInPool(p))
			rev, err := store.GetRevision("initNoAuthor", 0)
			require.NoError(t, err)
			require.NotNil(t, rev.AuthorId)
			require.Equal(t, modelpad.SystemAuthorId, *rev.AuthorId)
		})

		t.Run("an empty pad has nothing to attribute", func(t *testing.T) {
			empty := ""
			p := load(t, "initEmpty", &empty, nil)
			require.Equal(t, "\n", p.Text())
			require.Empty(t, authorsInPool(p))
		})

		t.Run("a pad created this way loads again and passes its check", func(t *testing.T) {
			text := "to be loaded again"
			load(t, "initReload", &text, &creator)
			other := newInstanceB(store)
			unlock := mustLock(other.LockPad("initReload"))
			defer unlock()
			p, err := other.GetPad("initReload", nil, nil)
			require.NoError(t, err)
			require.NoError(t, p.Check())
			requireEveryCharHasAuthor(t, p)
		})
	})
}

// requireEveryCharHasAuthor checks that the attributes cover the whole text
// and every character except the final newline carries an author.
func requireEveryCharHasAuthor(t *testing.T, p *modelpad.Pad) {
	t.Helper()
	ops, err := changeset.DeserializeOps(p.AText.Attribs)
	require.NoError(t, err)
	total, attributed := 0, 0
	for _, op := range *ops {
		total += op.Chars
		if a := changeset.FromString(op.Attribs, &p.Pool).Get("author"); a != nil && *a != "" {
			attributed += op.Chars
		}
	}
	textLen := len([]rune(p.Text()))
	require.Equal(t, textLen, total, "attributes must cover the text: %q vs %q", p.AText.Attribs, p.Text())
	require.GreaterOrEqual(t, attributed, textLen-1, "unattributed characters in %q", p.AText.Attribs)
}

func authorAttrib(author string) apool.Attribute {
	return apool.Attribute{Key: "author", Value: author}
}

// authorsInPool returns the non-empty author ids in the pad's pool, sorted.
func authorsInPool(p *modelpad.Pad) []string {
	var out []string
	p.Pool.EachAttrib(func(attr apool.Attribute) {
		if attr.Key == "author" && attr.Value != "" {
			out = append(out, attr.Value)
		}
	})
	sort.Strings(out)
	return out
}

// newTestAuthor creates an author in the store (revisions reference authors).
func newTestAuthor(t *testing.T, store db.DataStore) string {
	t.Helper()
	a, err := author.NewManager(store).CreateAuthor(nil)
	require.NoError(t, err)
	return a.Id
}

// The generated default text is nobody's contribution: it goes to the system
// author, while revision 0 still records who created the pad. Not parallel:
// it sets the global default pad text.
func TestDefaultPadTextBelongsToTheSystemAuthor(t *testing.T) {
	before := settings.Displayed.DefaultPadText
	settings.Displayed.DefaultPadText = "Welcome to this pad!"
	t.Cleanup(func() { settings.Displayed.DefaultPadText = before })

	store := db.NewMemoryDataStore()
	m := newInstance(t, store)
	creator := newTestAuthor(t, store)
	unlock := mustLock(m.LockPad("initDefault"))
	defer unlock()
	p, err := m.GetPad("initDefault", nil, &creator)
	require.NoError(t, err)
	require.Equal(t, "Welcome to this pad!\n", p.Text())
	requireEveryCharHasAuthor(t, p)
	require.Equal(t, []string{modelpad.SystemAuthorId}, authorsOfText(t, p),
		"the generated text is attributed to the system author, not the creator")
	rev, err := store.GetRevision("initDefault", 0)
	require.NoError(t, err)
	require.NotNil(t, rev.AuthorId)
	require.Equal(t, creator, *rev.AuthorId, "revision 0 records who created the pad")
}

// authorsOfText returns the authors the pad's text is attributed to, sorted.
// (The pool also lists authors of revisions that attribute no text.)
func authorsOfText(t *testing.T, p *modelpad.Pad) []string {
	t.Helper()
	ops, err := changeset.DeserializeOps(p.AText.Attribs)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, op := range *ops {
		if a := changeset.FromString(op.Attribs, &p.Pool).Get("author"); a != nil && *a != "" {
			seen[*a] = true
		}
	}
	out := make([]string, 0, len(seen))
	for a := range seen {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}
