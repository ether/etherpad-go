package pad

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only every 100th revision stores the full text. Every revision's text
// must still be recoverable, and loading a pad must give its current text.
func TestEveryRevisionIsRecoverableWithKeyRevisions(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		m := newInstance(t, store)
		padID := "keyrev-pad"

		texts := map[int]string{}
		for i := 0; i < 250; i++ {
			unlock := mustLock(m.LockPad(padID))
			p, err := m.GetPad(padID, nil, nil)
			require.NoError(t, err)
			if i == 0 {
				texts[p.Head] = p.Text()
			}
			pos := utf8.RuneCountInString(p.Text()) - 1
			if i%7 == 3 && pos > 5 {
				require.NoError(t, p.SpliceText(2, 3, "", nil)) // deletions too
			} else {
				require.NoError(t, p.SpliceText(pos, 0, fmt.Sprintf("line %d\n", i), nil))
			}
			texts[p.Head] = p.Text()
			unlock()
		}

		p := m.globalPadCache.GetPad(padID)
		require.NotNil(t, p)
		require.GreaterOrEqual(t, p.Head, 200)

		for rev := 0; rev <= p.Head; rev++ {
			stored, err := store.GetRevision(padID, rev)
			require.NoError(t, err)
			if rev%100 == 0 {
				assert.Equal(t, texts[rev], stored.AText.Text, "key revision %d stores its text", rev)
			} else {
				assert.Empty(t, stored.AText.Text, "revision %d stores no text", rev)
			}
			rebuilt := p.GetInternalRevisionAText(rev)
			require.NotNil(t, rebuilt, "revision %d", rev)
			assert.Equal(t, texts[rev], rebuilt.Text, "text of revision %d", rev)
		}

		// A fresh instance loads the current text from key revision + changesets.
		fresh := newInstance(t, open(t))
		loaded, err := fresh.GetPad(padID, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, p.Head, loaded.Head)
		assert.Equal(t, p.Text(), loaded.Text())
		assert.Equal(t, p.AText.Attribs, loaded.AText.Attribs)

		// The pad row's head is current although its text is only
		// rewritten on key revisions (pad listings and retention read it).
		row, err := store.GetPad(padID)
		require.NoError(t, err)
		assert.Equal(t, p.Head, row.Head)
	})
}

// A pad row written by an older version may hold a text that is not the
// one of any revision; loading takes the key revision and changesets.
func TestLoadingIgnoresAStalePadRowText(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		store := open(t)
		m := newInstance(t, store)
		padID := "stale-row-text"
		for i := 0; i < 5; i++ {
			appendLine(t, m, padID, fmt.Sprintf("l%d", i))
		}
		want := m.globalPadCache.GetPad(padID).Text()

		row, err := store.GetPad(padID)
		require.NoError(t, err)
		row.Head = m.globalPadCache.GetPad(padID).Head
		row.ATextText = "wrong text\n"
		row.ATextAttribs = "|1+b"
		require.NoError(t, store.CreatePad(padID, *row))

		loaded, err := newInstance(t, open(t)).GetPad(padID, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, want, loaded.Text())
	})
}
