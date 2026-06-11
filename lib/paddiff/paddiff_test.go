package paddiff

import (
	"errors"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePad is an in-memory Pad implementation for unit tests.
type fakePad struct {
	atexts map[int]apool.AText
	revs   map[int]db2.PadSingleRevision
}

func (f *fakePad) GetInternalRevisionAText(targetRev int) *apool.AText {
	atext, ok := f.atexts[targetRev]
	if !ok {
		return nil
	}
	return &atext
}

func (f *fakePad) GetRevision(revNumber int) (*db2.PadSingleRevision, error) {
	rev, ok := f.revs[revNumber]
	if !ok {
		return nil, errors.New("revision not found")
	}
	return &rev, nil
}

// buildRevision creates a revision changeset by splicing the given text and
// returns the changeset plus the resulting text.
func buildRevision(t *testing.T, pool *apool.APool, orig string, start int, ndel int, ins string, author string) (string, string) {
	t.Helper()

	var attribs *string
	if author != "" {
		authorNum := pool.PutAttrib(apool.Attribute{Key: "author", Value: author}, nil)
		attribStr := "*" + utils.NumToString(authorNum)
		attribs = &attribStr
	}

	cs, err := changeset.MakeSplice(orig, start, ndel, ins, attribs, pool)
	require.NoError(t, err)

	newText := orig[:start] + ins + orig[start+ndel:]
	return cs, newText
}

func TestGetValidRevisionRange(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name     string
		startRev int
		endRev   *int
		head     int
		wantFrom int
		wantTo   int
		wantOk   bool
	}{
		{"full range", 0, intPtr(3), 3, 0, 3, true},
		{"endRev defaults to head", 1, nil, 3, 1, 3, true},
		{"endRev clamped to head", 0, intPtr(99), 3, 0, 3, true},
		{"start equals end", 2, intPtr(2), 3, 2, 2, true},
		{"negative start invalid", -1, intPtr(2), 3, 0, 0, false},
		{"start beyond head invalid", 4, nil, 3, 0, 0, false},
		{"end below start invalid", 2, intPtr(1), 3, 0, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			from, to, ok := GetValidRevisionRange(tc.startRev, tc.endRev, tc.head)
			assert.Equal(t, tc.wantOk, ok)
			if tc.wantOk {
				assert.Equal(t, tc.wantFrom, from)
				assert.Equal(t, tc.wantTo, to)
			}
		})
	}
}

func TestCreateDiffATextReinsertsDeletionsWithRemovedAttribute(t *testing.T) {
	pool := apool.NewAPool()

	text0 := "Hello World\n"
	atext0 := changeset.MakeAText(text0, nil)

	// Revision 1 (author a1) replaces "World" with "Etherpad"
	cs1, text1 := buildRevision(t, &pool, text0, 6, 5, "Etherpad", "a1")
	assert.Equal(t, "Hello Etherpad\n", text1)

	a1 := "a1"
	p := &fakePad{
		atexts: map[int]apool.AText{0: atext0},
		revs: map[int]db2.PadSingleRevision{
			1: {RevNum: 1, Changeset: cs1, AuthorId: &a1},
		},
	}

	atext, authors, err := CreateDiffAText(p, &pool, 0, 1)
	require.NoError(t, err)
	require.NotNil(t, atext)

	// The deleted text is re-inserted before the text that replaced it
	assert.Equal(t, "Hello WorldEtherpad\n", atext.Text)
	assert.Equal(t, []string{"a1"}, authors)

	// The re-inserted "World" (chars 6..11) must carry the 'removed' attribute
	end := 11
	attribsAtWorld, err := changeset.Subattribution(atext.Attribs, 6, &end)
	require.NoError(t, err)
	ops, err := changeset.DeserializeOps(*attribsAtWorld)
	require.NoError(t, err)
	require.NotEmpty(t, *ops)

	foundRemoved := false
	for _, op := range *ops {
		for _, attr := range changeset.AttribsFromString(op.Attribs, pool) {
			if attr.Key == "removed" && attr.Value == "true" {
				foundRemoved = true
			}
		}
	}
	assert.True(t, foundRemoved, "re-inserted deletion must carry the removed attribute")

	// The inserted "Etherpad" (chars 11..19) must be attributed to a1 and must
	// not be marked as removed
	end = 19
	attribsAtInsert, err := changeset.Subattribution(atext.Attribs, 11, &end)
	require.NoError(t, err)
	insertOps, err := changeset.DeserializeOps(*attribsAtInsert)
	require.NoError(t, err)

	foundAuthor := false
	for _, op := range *insertOps {
		for _, attr := range changeset.AttribsFromString(op.Attribs, pool) {
			assert.NotEqual(t, "removed", attr.Key, "inserted text must not be marked as removed")
			if attr.Key == "author" && attr.Value == "a1" {
				foundAuthor = true
			}
		}
	}
	assert.True(t, foundAuthor, "inserted text must be attributed to its author")
}

func TestCreateDiffATextComposesMultipleRevisionsAndCollectsAuthors(t *testing.T) {
	pool := apool.NewAPool()

	text0 := "Hello World\n"
	atext0 := changeset.MakeAText(text0, nil)

	// Revision 1 (author a1) replaces "World" with "Etherpad"
	cs1, text1 := buildRevision(t, &pool, text0, 6, 5, "Etherpad", "a1")
	// Revision 2 (author a2) appends " Rocks"
	cs2, text2 := buildRevision(t, &pool, text1, 14, 0, " Rocks", "a2")
	assert.Equal(t, "Hello Etherpad Rocks\n", text2)

	a1, a2 := "a1", "a2"
	p := &fakePad{
		atexts: map[int]apool.AText{0: atext0},
		revs: map[int]db2.PadSingleRevision{
			1: {RevNum: 1, Changeset: cs1, AuthorId: &a1},
			2: {RevNum: 2, Changeset: cs2, AuthorId: &a2},
		},
	}

	atext, authors, err := CreateDiffAText(p, &pool, 0, 2)
	require.NoError(t, err)
	require.NotNil(t, atext)

	assert.Equal(t, "Hello WorldEtherpad Rocks\n", atext.Text)
	assert.ElementsMatch(t, []string{"a1", "a2"}, authors)
}

func TestCreateDiffATextSkipsClearAuthorshipChangesets(t *testing.T) {
	pool := apool.NewAPool()

	text0 := "Hello World\n"
	atext0 := changeset.MakeAText(text0, nil)

	// Revision 1 only clears the authorship of the whole text
	clearCs := createClearAuthorship(atext0, &pool)
	assert.True(t, isClearAuthorship(clearCs, &pool))

	a1 := "a1"
	p := &fakePad{
		atexts: map[int]apool.AText{0: atext0},
		revs: map[int]db2.PadSingleRevision{
			1: {RevNum: 1, Changeset: clearCs, AuthorId: &a1},
		},
	}

	atext, authors, err := CreateDiffAText(p, &pool, 0, 1)
	require.NoError(t, err)
	require.NotNil(t, atext)

	// No diffable change happened, so the text stays the same and no author is
	// reported
	assert.Equal(t, text0, atext.Text)
	assert.Empty(t, authors)
}

func TestIsClearAuthorshipRejectsRegularChangesets(t *testing.T) {
	pool := apool.NewAPool()
	text0 := "Hello World\n"

	cs, _ := buildRevision(t, &pool, text0, 6, 5, "Etherpad", "a1")
	assert.False(t, isClearAuthorship(cs, &pool))
}

func TestCreateDiffATextEmptyRangeReturnsClearedStartAText(t *testing.T) {
	pool := apool.NewAPool()
	text0 := "Hello World\n"
	atext0 := changeset.MakeAText(text0, nil)

	p := &fakePad{
		atexts: map[int]apool.AText{0: atext0},
		revs:   map[int]db2.PadSingleRevision{},
	}

	atext, authors, err := CreateDiffAText(p, &pool, 0, 0)
	require.NoError(t, err)
	require.NotNil(t, atext)
	assert.Equal(t, text0, atext.Text)
	assert.Empty(t, authors)
}
