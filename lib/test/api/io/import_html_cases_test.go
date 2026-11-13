package io

import (
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite api/importexport: the text a pad has after setHTML. Where
// etherpad-go differs on purpose, the case says so:
//   - etherpad-lite adds an empty line at the end of every import (so an
//     export/import round trip grows the pad by a line); etherpad-go does not.
//   - a non-breaking space becomes a space (etherpad-lite keeps some as
//     U+00A0, depending on the neighbouring spaces).
func TestImportHTMLCases(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "cases", Test: testImportHTMLCases})
	testDb.StartTestDBHandler()
}

func testImportHTMLCases(t *testing.T, ds testutils.TestDataStore) {
	cases := []struct{ name, input, wantText string }{
		{"whitespaceinlist #3620", `<html><body><ul> <li>FOO</li></ul></body></html>`, "*FOO\n"},
		{"prefixcorrectlinenumber", `<html><body><ol><li>should be 1</li><li>should be 2</li></ol></body></html>`, "*should be 1\n*should be 2\n"},
		{"prefixcorrectlinenumbernested", `<html><body><ol><li>should be 1</li><ol><li>foo</li></ol><li>should be 2</li></ol></body></html>`, "*should be 1\n*foo\n*should be 2\n"},
		{"ignoreAnyTagsOutsideBody", `<html><head><title>title</title><style></style></head><body>empty<br></body></html>`, "empty\n"},
		{"indentedListsAreNotBullets", `<html><body><ul class="indent"><li>indent</li><li>indent</ul></body></html>`, "*indent\n*indent\n"},
		{"lineWithMultipleSpaces", `<html><body>Text with  more   than    one space.<br></body></html>`, "Text with more than one space.\n"},
		{"lineWithMultipleNonBreakingAndNormalSpaces", `<html><body>Text&nbsp;with&nbsp; more&nbsp;&nbsp;&nbsp;than   &nbsp;one space.<br></body></html>`, "Text with  more   than  one space.\n"},
		{"multiplenbsp", `<html><body>&nbsp;&nbsp;<br></body></html>`, "  \n"},
		{"multipleNonBreakingSpaceBetweenWords", `<html><body>&nbsp;&nbsp;word1&nbsp;&nbsp;word2&nbsp;&nbsp;&nbsp;word3<br></body></html>`, "  word1  word2   word3\n"},
		{"nonBreakingSpacePreceededBySpaceBetweenWords", `<html><body> &nbsp;word1 &nbsp;word2 &nbsp;word3<br></body></html>`, " word1  word2  word3\n"},
		{"nonBreakingSpaceFollowededBySpaceBetweenWords", `<html><body>&nbsp; word1&nbsp; word2&nbsp; word3<br></body></html>`, "  word1  word2  word3\n"},
		{"spacesAfterNewline", `<!doctype html><html><body>something<br>             something<br></body></html>`, "something\nsomething\n"},
		{"spacesAfterNewlineP", `<!doctype html><html><body>something<p></p>             something<br></body></html>`, "something\n\nsomething\n"},
		{"spacesAtEndOfLine", `<html><body>something            <br>             something<br></body></html>`, "something\nsomething\n"},
		{"spacesAtEndOfLineP", `<html><body>something            <p></p>             something<br></body></html>`, "something\n\nsomething\n"},
		{"nonBreakingSpacesAfterNewlines", `<html><body>something<br>&nbsp;&nbsp;&nbsp;something<br></body></html>`, "something\n   something\n"},
		{"nonBreakingSpacesAfterNewlinesP", `<html><body>something<p></p>&nbsp;&nbsp;&nbsp;something<br></body></html>`, "something\n\n   something\n"},
		{"collapseSpacesInsideElements", `<html><body>Need <span> more </span> space<i>  s </i> !<br></body></html>`, "Need more space s !\n"},
		{"collapseSpacesAcrossNewlines", "\n      <html><body>Need\n          <span> more </span>\n          space\n          <i>  s </i>\n          !<br></body></html>", "Need more space s !\n"},
		{"multipleNewLinesAtBeginning", `<html><body><br><br><p></p><p></p>first line<br><br>second line<br></body></html>`, "\n\n\n\nfirst line\n\nsecond line\n"},
		{"multiLineParagraph", "<html><body>\n    <p>\n      а б в г ґ д е є ж з и і ї й к л м н о\n      п р с т у ф х ц ч ш щ ю я ь\n    </p>\n</body></html>", "а б в г ґ д е є ж з и і ї й к л м н о п р с т у ф х ц ч ш щ ю я ь\n"},
		{"multiLineParagraphWithPre", "<html><body>\n    <p>\n        а б в г ґ д е є ж з и і ї й к л м н о<pre>multiple\n   lines\n in\n      pre\n</pre></p><p>п р с т у ф х ц ч ш щ ю я\nь</p>\n</body></html>", "а б в г ґ д е є ж з и і ї й к л м н о\nmultiple\n   lines\n in\n      pre\n\nп р с т у ф х ц ч ш щ ю я ь\n"},
		{"preIntroducesASpace", "<html><body><p>\n    1\n<pre>preline\n</pre></p></body></html>", "1\npreline\n"},
		{"dontDeleteSpaceInsideElements", `<html><body>Need<span> more </span>space<i> s </i>!<br></body></html>`, "Need more space s !\n"},
		{"dontDeleteSpaceOutsideElements", `<html><body>Need <span>more</span> space <i>s</i> !<br></body></html>`, "Need more space s !\n"},
		{"dontDeleteSpaceAtEndOfElement", `<html><body>Need <span>more </span>space <i>s </i>!<br></body></html>`, "Need more space s !\n"},
		{"dontDeleteSpaceAtBeginOfElements", `<html><body>Need<span> more</span> space<i> s</i> !<br></body></html>`, "Need more space s !\n"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			padId := "htmlCase" + strings.Repeat("x", i)
			importHTML(t, ds, padId, tc.input)
			ds.PadManager.UnloadPad(padId)
			p, err := ds.PadManager.GetPad(padId, nil, nil)
			require.NoError(t, err)
			require.NoError(t, p.Check())
			assert.Equal(t, tc.wantText, p.Text())
		})
	}
}
